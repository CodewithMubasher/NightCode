package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	ctxbuilder "github.com/CodewithMubasher/NightCode/backend/internal/context"
	"github.com/CodewithMubasher/NightCode/backend/internal/provider"
	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
	"github.com/google/uuid"
)

const (
	maxIterations          = 20
	maxConsecutiveFailures = 4
)

// Loop drives the real agent turn: build context, iterate over model + tools,
// and emit RuntimeEvents for the SSE stream.
type Loop struct {
	provider provider.Provider
	registry tools.ToolRegistry
	store    *store.Store
}

func NewLoop(p provider.Provider, registry tools.ToolRegistry, s *store.Store) *Loop {
	return &Loop{provider: p, registry: registry, store: s}
}

// streamDelay is a temporary debugging aid to reliably exercise cancellation.
// Set NIGHTCODE_STREAM_DELAY_MS to slow down text-delta forwarding.
var streamDelayMs = func() int {
	v := os.Getenv("NIGHTCODE_STREAM_DELAY_MS")
	n, _ := strconv.Atoi(v)
	return n
}()

// Run executes one turn. It emits events on `events` and closes it when done.
// `history` is the prior conversation (role/content), already loaded from the store.
func (l *Loop) Run(ctx context.Context, ws *tools.Workspace, chatID string, userMessage string, history []ctxbuilder.Message, events chan<- types.RuntimeEvent) {
	defer close(events)

	now := func() int64 { return time.Now().UnixMilli() }
	evtID := func() string { return fmt.Sprintf("evt-%d", time.Now().UnixNano()) }
	messageID := uuid.New().String()

	// Build context via Step 3
	ctxb, buildErr := ctxbuilder.Build(ctx, ctxbuilder.BuildRequest{
		Workspace: ws,
		ChatID:    chatID,
		History:   history,
	})
	if buildErr != nil {
		log.Printf("context build error: %v", buildErr)
	}
	for _, w := range ctxb.Warnings {
		log.Printf("context warning: %s", w)
	}

	// Emit turn.started
	events <- types.RuntimeEvent{
		Type:      "turn.started",
		ID:        evtID(),
		Timestamp: now(),
	}

	// Assemble provider messages: history + current user message
	messages := buildMessages(ctxb.Messages, userMessage)
	toolSpecs := buildToolSpecs(l.registry)

	var segments []types.Segment
	consecutiveFailures := 0

	// finalize persists the assistant message (with full segments) to the store.
	finalize := func() {
		if l.store == nil {
			return
		}
		b, err := json.Marshal(segments)
		if err != nil {
			log.Printf("marshal segments error: %v", err)
			return
		}
		if len(segments) == 0 {
			return
		}
		if err := l.store.InsertMessage(messageID, chatID, "assistant", b); err != nil {
			log.Printf("persist assistant segments error: %v", err)
		}
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		if ctx.Err() != nil {
			log.Printf("agent loop: context cancelled (%v); exiting", ctx.Err())
			return
		}

		seg := types.Segment{Type: "text", ID: fmt.Sprintf("seg-%d", time.Now().UnixNano())}

		req := provider.ChatRequest{
			SystemPrompt: ctxb.SystemPrompt,
			Messages:     messages,
			Tools:        toolSpecs,
		}

		eventCh, err := l.provider.StreamChat(ctx, req)
		if err != nil {
			events <- turnError(evtID, seg.ID, now, err, "internal_error", false)
			return
		}

		var textBuf string
		var toolCalls []provider.ToolCall
		var deltaCount int

	streamLoop:
		for event := range eventCh {
			switch event.Type {
			case provider.ChatEventDelta:
				textBuf += event.Text
				deltaCount++
				if streamDelayMs > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(streamDelayMs) * time.Millisecond):
					}
				}
				events <- types.RuntimeEvent{
					Type:      "assistant.delta",
					ID:        evtID(),
					SegmentID: seg.ID,
					Timestamp: now(),
					Text:      textBuf,
				}

			case provider.ChatEventToolCalls:
				toolCalls = event.ToolCalls
				break streamLoop

			case provider.ChatEventError:
				if ctx.Err() != nil {
					return
				}
				events <- l.classifiedError(evtID, seg.ID, now, event.Err)
				return

			case provider.ChatEventRetry:
				// Non-terminal: the provider hit a retryable error (rate
				// limit, transient 5xx) and is about to wait before trying
				// again. Surface this so the UI can show real progress
				// instead of going silent for the whole backoff window —
				// previously a 45s Gemini rate-limit backoff looked
				// identical to a hung request, and cancelling during that
				// window discarded the turn with no visible trace.
				events <- types.RuntimeEvent{
					Type:      "turn.retry",
					ID:        evtID(),
					SegmentID: seg.ID,
					Timestamp: now(),
					Error:     event.Err.Error(),
					Attempt:   event.Attempt,
					MaxAttempt: event.MaxAttempt,
					RetryAfterMs: event.RetryAfter.Milliseconds(),
				}
			}
		}

		log.Printf("agent loop: stream ended — deltas=%d textLen=%d toolCalls=%d", deltaCount, len(textBuf), len(toolCalls))

		// Guard against a provider silently returning nothing: no text, no
		// tool calls, no error. Surface this as a real error instead of
		// completing the turn as if it succeeded with an empty reply.
		if deltaCount == 0 && len(toolCalls) == 0 {
			events <- turnError(evtID, seg.ID, now, fmt.Errorf("model returned an empty response (no text, no tool calls)"), "empty_response", false)
			return
		}

		// Accumulate text segment (if any text was produced)
		if textBuf != "" {
			seg.Text = textBuf
			segments = append(segments, seg)
		}

		// No tool calls → final answer.
		if len(toolCalls) == 0 {
			emitDone(events, evtID, seg.ID, now)
			finalize()
			events <- types.RuntimeEvent{Type: "turn.completed", ID: evtID(), Timestamp: now()}
			return
		}

		// Execute tool calls (concurrently if multiple)
		toolSeg := types.Segment{Type: "tool-group", ID: fmt.Sprintf("seg-%d", time.Now().UnixNano())}
		results := l.executeTools(ctx, ws, chatID, messageID, toolCalls, toolSeg, &consecutiveFailures, events, evtID, now)

		// Consecutive failure cap
		if consecutiveFailures >= maxConsecutiveFailures {
			events <- types.RuntimeEvent{
				Type:      "turn.error",
				ID:        evtID(),
				SegmentID: toolSeg.ID,
				Timestamp: now(),
				Error:     fmt.Sprintf("too many consecutive tool failures (%d)", consecutiveFailures),
				Code:      "too_many_tool_failures",
			}
			return
		}

		if len(results) > 0 {
			toolSeg.Calls = results
			segments = append(segments, toolSeg)
		}

		// Feed tool results back into message history
		messages = appendMessagesWithToolResults(messages, toolCalls, results)
	}

	// Iteration cap hit — work was done, just can't continue further.
	finalize()
	emitDone(events, evtID, "", now)
	events <- types.RuntimeEvent{Type: "turn.completed", ID: evtID(), Timestamp: now()}
}

func (l *Loop) executeTools(
	ctx context.Context,
	ws *tools.Workspace,
	chatID string,
	messageID string,
	toolCalls []provider.ToolCall,
	toolSeg types.Segment,
	consecutiveFailures *int,
	events chan<- types.RuntimeEvent,
	evtID func() string,
	now func() int64,
) []types.ToolCallEntry {
	results := make([]types.ToolCallEntry, len(toolCalls))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(i int, tc provider.ToolCall) {
			defer wg.Done()

			started := now()
			input := json.RawMessage(tc.Arguments)
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}

			events <- types.RuntimeEvent{
				Type:       "tool.started",
				ID:         evtID(),
				SegmentID:  toolSeg.ID,
				Timestamp:  started,
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Input:      input,
			}

			t, ok := l.registry[tc.Name]
			var result tools.ToolResult
			var err error
			if !ok {
				err = fmt.Errorf("unknown tool: %s", tc.Name)
			} else {
				result, err = t.Execute(ctx, input, ws)
			}

			duration := now() - started
			entry := types.ToolCallEntry{
				ToolCallID:  tc.ID,
				Name:        tc.Name,
				StartedAt:   started,
				Input:       input,
				CompletedAt: now(),
			}

			if err != nil {
				entry.Status = "failed"
				entry.Error = err.Error()
				mu.Lock()
				*consecutiveFailures++
				mu.Unlock()

				if l.store != nil {
					_ = l.store.InsertToolCall(uuid.New().String(), chatID, messageID, tc.ID, tc.Name, "failed", string(input), "", err.Error(), started, now())
				}

				events <- types.RuntimeEvent{
					Type:       "tool.failed",
					ID:         evtID(),
					SegmentID:  toolSeg.ID,
					Timestamp:  now(),
					ToolCallID: tc.ID,
					Name:       tc.Name,
					Error:      err.Error(),
					Duration:   duration,
				}
			} else {
				entry.Status = "completed"
				entry.Output = result.Output
				mu.Lock()
				*consecutiveFailures = 0
				mu.Unlock()

				if l.store != nil {
					_ = l.store.InsertToolCall(uuid.New().String(), chatID, messageID, tc.ID, tc.Name, "completed", string(input), string(result.Output), "", started, now())
				}

				events <- types.RuntimeEvent{
					Type:       "tool.completed",
					ID:         evtID(),
					SegmentID:  toolSeg.ID,
					Timestamp:  now(),
					ToolCallID: tc.ID,
					Name:       tc.Name,
					Output:     result.Output,
					Duration:   duration,
				}

				if result.Artifact != nil {
					artifactID := fmt.Sprintf("artifact-%d", time.Now().UnixNano())
					if l.store != nil {
						_ = l.store.InsertArtifact(artifactID, chatID, result.Artifact.Name, result.Artifact.ArtifactType, result.Artifact.Language, result.Artifact.Content)
					}
					events <- types.RuntimeEvent{
						Type:         "artifact.created",
						ID:           evtID(),
						SegmentID:    toolSeg.ID,
						Timestamp:    now(),
						ArtifactID:   artifactID,
						ToolCallID:   tc.ID,
						Name:         result.Artifact.Name,
						Content:      result.Artifact.Content,
						ArtifactType: result.Artifact.ArtifactType,
						Language:     result.Artifact.Language,
					}
				}
			}

			mu.Lock()
			results[i] = entry
			mu.Unlock()
		}(i, tc)
	}

	wg.Wait()
	return results
}

// classifiedError maps a ProviderError to a turn.error RuntimeEvent.
func (l *Loop) classifiedError(evtID func() string, segID string, now func() int64, err error) types.RuntimeEvent {
	code := "internal_error"
	retryable := false

	var perr *provider.ProviderError
	if pe, ok := err.(*provider.ProviderError); ok {
		perr = pe
		switch pe.Kind {
		case provider.ErrorKindAuth:
			code = "auth_error"
			retryable = false
		case provider.ErrorKindRateLimit:
			code = "rate_limit_exceeded"
			retryable = true
		case provider.ErrorKindTransient:
			code = "provider_error"
			retryable = true
		}
	}
	_ = perr

	msg := err.Error()
	return types.RuntimeEvent{
		Type:      "turn.error",
		ID:        evtID(),
		SegmentID: segID,
		Timestamp: now(),
		Error:     msg,
		Code:      code,
		Retryable: retryable,
	}
}

func turnError(evtID func() string, segID string, now func() int64, err error, code string, retryable bool) types.RuntimeEvent {
	return types.RuntimeEvent{
		Type:      "turn.error",
		ID:        evtID(),
		SegmentID: segID,
		Timestamp: now(),
		Error:     err.Error(),
		Code:      code,
		Retryable: retryable,
	}
}

func buildMessages(history []ctxbuilder.Message, userMessage string) []provider.Message {
	msgs := make([]provider.Message, 0, len(history)+1)
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" && m.Role != "tool" {
			continue
		}
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: "user", Content: userMessage})
	return msgs
}

func buildToolSpecs(registry tools.ToolRegistry) []provider.ToolSpec {
	names := registry.Names()
	specs := make([]provider.ToolSpec, 0, len(names))
	for _, name := range names {
		t := registry[name]
		specs = append(specs, provider.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.InputSchema(),
		})
	}
	return specs
}

func appendMessagesWithToolResults(msgs []provider.Message, toolCalls []provider.ToolCall, results []types.ToolCallEntry) []provider.Message {
	msgs = append(msgs, provider.Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCalls,
	})

	for i, tc := range toolCalls {
		var output string
		if i < len(results) {
			entry := results[i]
			if entry.Status == "failed" {
				output = entry.Error
			} else if len(entry.Output) > 0 {
				output = string(entry.Output)
			}
		}
		msgs = append(msgs, provider.Message{
			Role:       "tool",
			Content:    output,
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
		})
	}
	return msgs
}

func emitDone(events chan<- types.RuntimeEvent, evtID func() string, segID string, now func() int64) {
	events <- types.RuntimeEvent{
		Type:      "assistant.delta",
		ID:        evtID(),
		SegmentID: segID,
		Timestamp: now(),
		Text:      "__DONE__",
	}
}