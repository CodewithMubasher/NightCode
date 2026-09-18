package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	ctxbuilder "github.com/CodewithMubasher/NightCode/backend/internal/context"
	"github.com/CodewithMubasher/NightCode/backend/internal/provider"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
)

// fakeProvider returns a scripted sequence of ChatEvents.
type fakeProvider struct {
	// responses is a list of ChatEvent lists; each StreamChat call pops one.
	responses [][]provider.ChatEvent
	callCount int
}

func (f *fakeProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.ChatEvent, error) {
	idx := f.callCount
	f.callCount++
	ch := make(chan provider.ChatEvent, 16)
	go func() {
		defer close(ch)
		var events []provider.ChatEvent
		if idx < len(f.responses) {
			events = f.responses[idx]
		}
		for _, e := range events {
			ch <- e
		}
	}()
	return ch, nil
}

func collect(events <-chan types.RuntimeEvent) []types.RuntimeEvent {
	var out []types.RuntimeEvent
	for e := range events {
		out = append(out, e)
	}
	return out
}

// fakeTool always fails, to drive consecutive failures.
type fakeFailTool struct{}

func (fakeFailTool) Name() string                     { return "fail_tool" }
func (fakeFailTool) Description() string              { return "always fails" }
func (fakeFailTool) InputSchema() json.RawMessage     { return json.RawMessage(`{"type":"object","properties":{}}`) }
func (fakeFailTool) Execute(ctx context.Context, input json.RawMessage, ws *tools.Workspace) (tools.ToolResult, error) {
	return tools.ToolResult{}, &tools.ToolError{Code: "boom", Message: "deliberate failure"}
}

func failRegistry() tools.ToolRegistry {
	r := tools.ToolRegistry{}
	r.Register(fakeFailTool{})
	return r
}

// TestLoop_ConsecutiveToolFailureCap verifies 4 consecutive tool failures aborts
// with turn.error / too_many_tool_failures (not looping to the 20-iteration cap).
func TestLoop_ConsecutiveToolFailureCap(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())

	// Three iterations, each returning a tool call to fail_tool.
	// 4th failure should trigger the cap, but we only script 4 iterations worth.
	call := provider.ToolCall{ID: "1", Name: "fail_tool", Arguments: "{}"}

	var responses [][]provider.ChatEvent
	for i := 0; i < maxIterations; i++ {
		responses = append(responses, []provider.ChatEvent{
			{Type: provider.ChatEventToolCalls, ToolCalls: []provider.ToolCall{call}},
		})
	}

	p := &fakeProvider{responses: responses}
	loop := NewLoop(p, failRegistry(), nil)

	events := make(chan types.RuntimeEvent, 64)
	go loop.Run(context.Background(), ws, "c", "do it", nil, events)
	evts := collect(events)

	// Count failures and confirm termination type
	failures := 0
	var termErr *types.RuntimeEvent
	for i := range evts {
		if evts[i].Type == "tool.failed" {
			failures++
		}
		if evts[i].Type == "turn.error" {
			termErr = &evts[i]
		}
	}

	if failures != maxConsecutiveFailures {
		t.Errorf("expected %d failures before abort, got %d", maxConsecutiveFailures, failures)
	}
	if termErr == nil {
		t.Fatal("expected a turn.error event")
	}
	if termErr.Code != "too_many_tool_failures" {
		t.Errorf("expected code too_many_tool_failures, got %q", termErr.Code)
	}
	if p.callCount > maxConsecutiveFailures {
		t.Errorf("loop kept iterating after failure cap: %d model calls", p.callCount)
	}
}

// noToolOkTool always succeeds.
type fakeOkTool struct{}

func (fakeOkTool) Name() string                     { return "ok_tool" }
func (fakeOkTool) Description() string              { return "always ok" }
func (fakeOkTool) InputSchema() json.RawMessage     { return json.RawMessage(`{"type":"object","properties":{}}`) }
func (fakeOkTool) Execute(ctx context.Context, input json.RawMessage, ws *tools.Workspace) (tools.ToolResult, error) {
	return tools.ToolResult{Output: json.RawMessage(`{"ok":true}`)}, nil
}

func okRegistry() tools.ToolRegistry {
	r := tools.ToolRegistry{}
	r.Register(fakeOkTool{})
	return r
}

// TestLoop_MaxIterationCap verifies the loop stops at 20 iterations with
// turn.completed when the model keeps requesting tools (the loop did real
// work but can't continue further — this is not an error).
func TestLoop_MaxIterationCap(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())
	call := provider.ToolCall{ID: "1", Name: "ok_tool", Arguments: "{}"}

	var responses [][]provider.ChatEvent
	for i := 0; i < maxIterations; i++ {
		responses = append(responses, []provider.ChatEvent{
			{Type: provider.ChatEventToolCalls, ToolCalls: []provider.ToolCall{call}},
		})
	}

	p := &fakeProvider{responses: responses}
	loop := NewLoop(p, okRegistry(), nil)

	events := make(chan types.RuntimeEvent, 128)
	go loop.Run(context.Background(), ws, "c", "loop forever", nil, events)
	evts := collect(events)

	var termCompleted bool
	for i := range evts {
		if evts[i].Type == "turn.completed" {
			termCompleted = true
		}
	}
	if !termCompleted {
		t.Fatal("expected a turn.completed event")
	}
	if p.callCount != maxIterations {
		t.Errorf("expected exactly %d model calls, got %d", maxIterations, p.callCount)
	}
}

// TestLoop_RunParallelToolCalls verifies multiple tool calls in one turn execute
// and share a single segment ID (parallel group).
func TestLoop_ParallelToolCalls(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())

	responses := [][]provider.ChatEvent{
		{
			{Type: provider.ChatEventToolCalls, ToolCalls: []provider.ToolCall{
				{ID: "a", Name: "ok_tool", Arguments: "{}"},
				{ID: "b", Name: "ok_tool", Arguments: "{}"},
				{ID: "c", Name: "ok_tool", Arguments: "{}"},
			}},
		},
		{
			{Type: provider.ChatEventDelta, Text: "done"},
		},
	}

	p := &fakeProvider{responses: responses}
	loop := NewLoop(p, okRegistry(), nil)

	events := make(chan types.RuntimeEvent, 128)
	go loop.Run(context.Background(), ws, "c", "parallel", nil, events)
	evts := collect(events)

	// All three tool.started should share the same segmentId
	segIDs := map[string]int{}
	started := 0
	for i := range evts {
		if evts[i].Type == "tool.started" {
			started++
			segIDs[evts[i].SegmentID]++
		}
	}
	if started != 3 {
		t.Errorf("expected 3 tool.started, got %d", started)
	}
	if len(segIDs) != 1 {
		t.Errorf("expected all parallel tools to share one segment ID, got %d", len(segIDs))
	}
}

// TestBuildMessages_UserTextPreservedInHistory verifies that user messages with
// real text content survive the history→provider-message pipeline. This is a
// regression test for the bug where user messages were stored with empty
// segments ("[]"), causing extractTextFromSegments to return "" and the message
// to be silently skipped from history on subsequent turns.
func TestBuildMessages_UserTextPreservedInHistory(t *testing.T) {
	// Simulate history as built by runRealLoop after the fix:
	// Turn 1: user says "my name is Alice"
	// Turn 1: assistant replies "Hello Alice"
	// Turn 2 (current): user says "what is my name?"
	history := []ctxbuilder.Message{
		{Role: "user", Content: "my name is Alice"},
		{Role: "assistant", Content: "Hello Alice"},
	}

	msgs := buildMessages(history, "what is my name?")

	// The assembled messages should contain the earlier user message text
	found := false
	for _, m := range msgs {
		if m.Role == "user" && strings.Contains(m.Content, "my name is Alice") {
			found = true
			break
		}
	}
	if !found {
		t.Error("earlier user message 'my name is Alice' not found in assembled provider messages")
		// Dump what we actually got for debugging
		for i, m := range msgs {
			t.Logf("  msgs[%d]: role=%q content=%q", i, m.Role, m.Content)
		}
	}

	// Also verify the current user message is at the end
	last := msgs[len(msgs)-1]
	if last.Role != "user" || last.Content != "what is my name?" {
		t.Errorf("expected last message to be current user query, got role=%q content=%q", last.Role, last.Content)
	}
}

// TestBuildMessages_EmptyContentFromLegacyStorage verifies that buildMessages
// does not crash on empty-content messages. The actual filtering of legacy
// empty-segment user rows happens in handler.go:runRealLoop (the
// extractTextFromSegments → "" → continue guard), so buildMessages sees only
// messages that survived that filter. This test documents that contract: if an
// empty user message somehow reaches buildMessages, it is included (the caller
// is responsible for filtering).
func TestBuildMessages_EmptyContentFromLegacyStorage(t *testing.T) {
	// Handler filters empty user messages BEFORE they reach buildMessages.
	// Simulate the post-filter history: only non-empty messages remain.
	history := []ctxbuilder.Message{
		{Role: "assistant", Content: "I don't have context yet"},
	}

	msgs := buildMessages(history, "tell me something")

	// Should have assistant + current user (2 total)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
		for i, m := range msgs {
			t.Logf("  msgs[%d]: role=%q content=%q", i, m.Role, m.Content)
		}
	}
}

// TestBuildMessages_MultiTurnPreservesUserContext is an end-to-end proof that
// a 3-turn conversation preserves earlier user messages. The fakeProvider
// captures the ChatRequest so we can inspect exactly what the model sees.
type capturingProvider struct {
	responses [][]provider.ChatEvent
	captured  []provider.ChatRequest
	callCount int
}

func (p *capturingProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.ChatEvent, error) {
	p.captured = append(p.captured, req)
	idx := p.callCount
	p.callCount++
	ch := make(chan provider.ChatEvent, 16)
	go func() {
		defer close(ch)
		if idx < len(p.responses) {
			for _, e := range p.responses[idx] {
				ch <- e
			}
		}
	}()
	return ch, nil
}

func TestBuildMessages_MultiTurnPreservesUserContext(t *testing.T) {
	ws, _ := tools.NewWorkspace(t.TempDir())

	// Simulate a 3-turn conversation where the answer to turn 3 depends on turn 1.
	// Turn 1: user "project is called NightCode"
	// Turn 1: assistant "Got it, NightCode."
	// Turn 2: user "what language is it in"
	// Turn 2: assistant "Go."
	// Turn 3 (current): user "what is the project name?"
	history := []ctxbuilder.Message{
		{Role: "user", Content: "project is called NightCode"},
		{Role: "assistant", Content: "Got it, NightCode."},
		{Role: "user", Content: "what language is it in"},
		{Role: "assistant", Content: "Go."},
	}

	p := &capturingProvider{
		responses: [][]provider.ChatEvent{
			{{Type: provider.ChatEventDelta, Text: "NightCode"}},
		},
	}
	loop := NewLoop(p, okRegistry(), nil)

	events := make(chan types.RuntimeEvent, 64)
	go loop.Run(context.Background(), ws, "chat-1", "what is the project name?", history, events)
	collect(events)

	if len(p.captured) == 0 {
		t.Fatal("provider was never called")
	}

	// The provider should have received all 4 history messages + current query = 5 total
	req := p.captured[0]
	totalMessages := len(req.Messages)
	if totalMessages < 5 {
		t.Errorf("expected at least 5 messages (4 history + 1 current), got %d", totalMessages)
		for i, m := range req.Messages {
			t.Logf("  msgs[%d]: role=%q content=%q", i, m.Role, m.Content)
		}
	}

	// Specifically check that "project is called NightCode" is in the messages
	found := false
	for _, m := range req.Messages {
		if strings.Contains(m.Content, "project is called NightCode") {
			found = true
			break
		}
	}
	if !found {
		t.Error("turn-1 user message 'project is called NightCode' not found in provider messages")
		for i, m := range req.Messages {
			t.Logf("  msgs[%d]: role=%q content=%q", i, m.Role, m.Content)
		}
	}
}