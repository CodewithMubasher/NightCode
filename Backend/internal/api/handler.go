package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CodewithMubasher/NightCode/backend/internal/agent"
	ctxbuilder "github.com/CodewithMubasher/NightCode/backend/internal/context"
	"github.com/CodewithMubasher/NightCode/backend/internal/provider"
	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
	"github.com/google/uuid"
)

type Handler struct {
	store *store.Store
	// activeRuns tracks in-flight runs by chatID so they can be cancelled.
	activeRuns   map[string]context.CancelFunc
	activeRunsMu sync.Mutex

	registry tools.ToolRegistry
	provider provider.Provider
	// providerName/modelName describe the default provider wired at startup
	// (via NIGHTCODE_PROVIDER), used when a request doesn't specify one.
	providerName string
	modelName    string
	// workspacesRoot is the base dir under which per-workspace folders live.
	workspacesRoot string

	// providerCacheMu guards providerCache, a small per-(provider,model)
	// cache so switching models in the UI doesn't re-init a client every
	// single message.
	providerCacheMu sync.Mutex
	providerCache    map[string]provider.Provider
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{
		store:          s,
		activeRuns:     make(map[string]context.CancelFunc),
		registry:       tools.NewRegistry(),
		workspacesRoot: os.Getenv("NIGHTCODE_WORKSPACES_ROOT"),
		providerCache:  make(map[string]provider.Provider),
	}
}

// SetProvider wires the default (startup) provider into the handler.
func (h *Handler) SetProvider(p provider.Provider) {
	h.provider = p
}

// SetDefaultProviderInfo records which provider/model was configured at
// startup via NIGHTCODE_PROVIDER, so /api/models can report it and requests
// that omit a provider/model fall back to it.
func (h *Handler) SetDefaultProviderInfo(providerName, modelName string) {
	h.providerName = providerName
	h.modelName = modelName
}

// resolveProvider returns a Provider for the given (provider, model) pair,
// building and caching a new client if needed. Falls back to the default
// provider wired at startup when providerName is empty.
func (h *Handler) resolveProvider(ctx context.Context, providerName, modelName string) (provider.Provider, error) {
	if providerName == "" {
		// No explicit provider requested — use the server default as-is,
		// including its default model.
		return h.provider, nil
	}
	if providerName == h.providerName && (modelName == "" || modelName == h.modelName) {
		return h.provider, nil
	}

	cacheKey := providerName + ":" + modelName
	h.providerCacheMu.Lock()
	if p, ok := h.providerCache[cacheKey]; ok {
		h.providerCacheMu.Unlock()
		return p, nil
	}
	h.providerCacheMu.Unlock()

	var p provider.Provider
	var err error
	switch providerName {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY not set")
		}
		p, err = provider.NewGeminiProvider(ctx, apiKey, modelName)
	case "groq":
		apiKey := os.Getenv("GROQ_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GROQ_API_KEY not set")
		}
		p, err = provider.NewGroqProvider(ctx, apiKey, modelName, os.Getenv("GROQ_BASE_URL"))
	case "opencode-zen":
		apiKey := os.Getenv("OPENCODE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENCODE_API_KEY not set")
		}
		p, err = provider.NewOpenCodeProvider(ctx, apiKey, modelName, os.Getenv("OPENCODE_BASE_URL"))
	case "openrouter":
		keys := collectOpenRouterKeys()
		if len(keys) == 0 {
			return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
		}
		p, err = provider.NewOpenRouterProvider(ctx, keys, modelName, os.Getenv("OPENROUTER_BASE_URL"))
	case "web2api":
		p, err = provider.NewWeb2APIProvider(ctx, os.Getenv("WEB2API_API_KEY"), modelName, os.Getenv("WEB2API_BASE_URL"))
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
	if err != nil {
		return nil, err
	}

	h.providerCacheMu.Lock()
	h.providerCache[cacheKey] = p
	h.providerCacheMu.Unlock()

	return p, nil
}

// collectOpenRouterKeys gathers all OPENROUTER_API_KEY, OPENROUTER_API_KEY_2, etc.
func collectOpenRouterKeys() []string {
	var keys []string
	if k := os.Getenv("OPENROUTER_API_KEY"); k != "" {
		keys = append(keys, k)
	}
	if k := os.Getenv("OPENROUTER_API_KEY_2"); k != "" {
		keys = append(keys, k)
	}
	if k := os.Getenv("OPENROUTER_API_KEY_3"); k != "" {
		keys = append(keys, k)
	}
	return keys
}

// SetWorkspacesRoot sets the base directory for workspace folders.
func (h *Handler) SetWorkspacesRoot(dir string) {
	h.workspacesRoot = dir
}

type sendMessageRequest struct {
	Message  string `json:"message"`
	Provider string `json:"provider,omitempty"` // "gemini" | "groq" | "opencode-zen" | "openrouter" | "web2api"; empty = server default
	Model    string `json:"model,omitempty"`    // e.g. "gemini-2.5-flash" | "llama-3.3-70b-versatile"
}

// POST /api/workspaces/{workspaceId}/chats/{chatId}/messages
func (h *Handler) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	chatID := r.PathValue("chatId")

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, `{"error":"message required"}`, http.StatusBadRequest)
		return
	}

	log.Printf("HandleSendMessage: workspace=%s chat=%s msg=%q", workspaceID, chatID, req.Message)

	// Ensure chat exists
	chat, err := h.store.GetChat(chatID)
	if err != nil {
		title := req.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		if err := h.store.UpsertChat(chatID, workspaceID, title); err != nil {
			log.Printf("upsert chat error: %v", err)
			http.Error(w, `{"error":"failed to create chat"}`, http.StatusInternalServerError)
			return
		}
	} else if chat.WorkspaceID == "" && workspaceID != "" {
		_ = h.store.UpsertChat(chatID, workspaceID, chat.Title)
	}

	// Persist user message
	if err := h.store.InsertMessage(
		uuid.New().String(),
		chatID,
		"user",
		json.RawMessage("[]"),
	); err != nil {
		log.Printf("insert user message error: %v", err)
	}

	// Register the run BEFORE writing any response so cancel can find it immediately
	ctx, cancel := context.WithCancel(r.Context())

	h.activeRunsMu.Lock()
	if oldCancel, ok := h.activeRuns[chatID]; ok {
		oldCancel() // cancel any in-flight run for this chat
	}
	h.activeRuns[chatID] = cancel
	h.activeRunsMu.Unlock()

	defer func() {
		h.activeRunsMu.Lock()
		delete(h.activeRuns, chatID)
		h.activeRunsMu.Unlock()
	}()

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	events := make(chan types.RuntimeEvent, 32)

	// Resolve which provider/model to use for this request (falls back to
	// the server default configured via NIGHTCODE_PROVIDER when the request
	// doesn't specify one).
	resolvedProvider, provErr := h.resolveProvider(ctx, req.Provider, req.Model)
	log.Printf("HandleSendMessage: resolved provider request=%q/%q -> nil=%v err=%v", req.Provider, req.Model, resolvedProvider == nil, provErr)

	switch {
	case provErr != nil:
		// An explicit provider/model was requested but couldn't be resolved
		// (missing API key, bad model, etc). Surface this as a real error
		// instead of silently falling back to the echo agent, which would
		// look like a successful-but-empty reply to the user.
		go func() {
			defer close(events)
			events <- types.RuntimeEvent{
				Type:      "turn.error",
				ID:        uuid.New().String(),
				Timestamp: time.Now().UnixMilli(),
				Error:     provErr.Error(),
				Code:      "provider_unavailable",
				Retryable: false,
			}
		}()
	case resolvedProvider == nil:
		go agent.Run(ctx, req.Message, events)
	default:
		go h.runRealLoop(ctx, resolvedProvider, workspaceID, chatID, req.Message, events)
	}

	// Accumulate assistant response for persistence
	var finalText string
	var lastSegmentID string

	for event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			log.Printf("marshal event error: %v", err)
			return
		}
		if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			log.Printf("write event error: %v", err)
			return
		}
		flusher.Flush()

		if event.Type == "assistant.delta" && event.Text != "__DONE__" {
			if event.Text == "" {
				continue
			}
			finalText = event.Text // each delta contains the full accumulated text
			lastSegmentID = event.SegmentID
		}
	}

	// The real loop persists its own message; only the echo agent needs this fallback.
	if resolvedProvider == nil && finalText != "" && lastSegmentID != "" {
		segment := map[string]any{
			"type": "text",
			"id":   lastSegmentID,
			"text": finalText,
		}
		segmentsJSON, _ := json.Marshal([]any{segment})
		msgID := uuid.New().String()
		if err := h.store.InsertMessage(msgID, chatID, "assistant", segmentsJSON); err != nil {
			log.Printf("persist assistant message error: %v", err)
		} else {
			log.Printf("persisted assistant message for chat=%s", chatID)
		}
	}

	log.Printf("HandleSendMessage: stream completed for chat=%s", chatID)
}

// runRealLoop loads conversation history, resolves the workspace folder, and
// drives the real agent loop using the given (already-resolved) provider.
func (h *Handler) runRealLoop(ctx context.Context, p provider.Provider, workspaceID, chatID, userMessage string, events chan<- types.RuntimeEvent) {
	// Load history from store into context.Message
	var history []ctxbuilder.Message
	if h.store != nil {
		rows, err := h.store.GetMessagesForChat(chatID)
		if err != nil {
			log.Printf("load history error: %v", err)
		} else {
			history = make([]ctxbuilder.Message, 0, len(rows)*2)
			for _, row := range rows {
				if row.Role == "user" {
					content := extractTextFromSegments(row.Segments)
					if content == "" {
						continue
					}
					history = append(history, ctxbuilder.Message{Role: "user", Content: content})
				} else if row.Role == "assistant" {
					msgs := expandAssistantSegments(row.Segments)
					history = append(history, msgs...)
				}
			}
		}
	}

	// Resolve workspace folder. If it doesn't exist yet, create it rather
	// than silently falling back to the OS temp dir — that fallback meant
	// every file the agent created ended up in a throwaway, unpredictable
	// location (e.g. C:\Users\...\AppData\Local\Temp on Windows) with no
	// indication to the user that's where it went.
	workspacePath := h.resolveWorkspacePath(workspaceID)
	if mkErr := os.MkdirAll(workspacePath, 0o755); mkErr != nil {
		log.Printf("workspace mkdir error: %v", mkErr)
	}
	ws, err := tools.NewWorkspace(workspacePath)
	if err != nil {
		log.Printf("workspace resolve error: %v", err)
		// Last-resort fallback so tools still function even if the
		// workspace directory genuinely can't be created (e.g. permissions).
		ws, _ = tools.NewWorkspace(os.TempDir())
		log.Printf("WARNING: using OS temp dir as workspace fallback for chat=%s — files created this turn will not be in a stable location", chatID)
	}

	loop := agent.NewLoop(p, h.registry, h.store)
	loop.Run(ctx, ws, chatID, userMessage, history, events)
}

// extractTextFromSegments pulls assistant/user text out of stored TurnSegment JSON.
// For now, returns the concatenated text segments; tool segments are skipped.
func extractTextFromSegments(segments json.RawMessage) string {
	if len(segments) == 0 {
		return ""
	}
	var segs []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(segments, &segs); err != nil {
		return ""
	}
	out := ""
	for _, s := range segs {
		if s.Type == "text" {
			out += s.Text
		}
	}
	return out
}

// expandAssistantSegments converts stored assistant message segments into
// the full sequence of provider messages: alternating assistant (with
// tool_calls) and tool (with results) messages. This preserves tool
// call/result pairs across turns so the model retains context about
// what tools were invoked and what they returned.
//
// Segment format (from types.Segment):
//
//	{"type": "text", "text": "..."}
//	{"type": "tool-group", "calls": [{"toolCallId": "...", "name": "...", "status": "...", "input": ..., "output": ...}]}
func expandAssistantSegments(segments json.RawMessage) []ctxbuilder.Message {
	if len(segments) == 0 {
		return nil
	}

	var segs []types.Segment
	if err := json.Unmarshal(segments, &segs); err != nil {
		return nil
	}

	var msgs []ctxbuilder.Message

	for _, seg := range segs {
		switch seg.Type {
		case "text":
			if seg.Text != "" {
				msgs = append(msgs, ctxbuilder.Message{
					Role:    "assistant",
					Content: seg.Text,
				})
			}

		case "tool-group":
			if len(seg.Calls) == 0 {
				continue
			}

			// Build the assistant message with tool_calls.
			assistantMsg := ctxbuilder.Message{
				Role:      "assistant",
				Content:   "", // assistant messages with tool calls have empty content
				ToolCalls: make([]ctxbuilder.ToolCallInfo, 0, len(seg.Calls)),
			}
			for _, call := range seg.Calls {
				// Marshal input back to JSON string for provider.ToolCall.Arguments.
				argsBytes, _ := json.Marshal(call.Input)
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, ctxbuilder.ToolCallInfo{
					ID:        call.ToolCallID,
					Name:      call.Name,
					Arguments: string(argsBytes),
				})
			}
			msgs = append(msgs, assistantMsg)

			// Build tool result messages for each call.
			for _, call := range seg.Calls {
				content := ""
				if call.Status == "failed" {
					content = call.Error
					if content == "" {
						content = "tool failed with no error message"
					}
				} else if len(call.Output) > 0 {
					content = string(call.Output)
				}
				msgs = append(msgs, ctxbuilder.Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: call.ToolCallID,
					ToolName:   call.Name,
				})
			}
		}
	}

	return msgs
}

// resolveWorkspacePath maps a workspaceID to a directory under workspacesRoot.
// If NIGHTCODE_WORKSPACES_ROOT was never configured, defaults to a
// "workspaces" folder next to the backend binary's working directory rather
// than the OS temp dir — so files created by the agent land somewhere
// predictable and persistent instead of a throwaway location the user has
// no reason to think to check.
func (h *Handler) resolveWorkspacePath(workspaceID string) string {
	root := h.workspacesRoot
	if root == "" {
		root = "workspaces"
	}
	return filepath.Join(root, workspaceID)
}

// DELETE /api/workspaces/{workspaceId}/chats/{chatId}/runs/{runId}
func (h *Handler) HandleCancelRun(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatId")

	h.activeRunsMu.Lock()
	cancel, ok := h.activeRuns[chatID]
	h.activeRunsMu.Unlock()

	if !ok {
		http.Error(w, `{"error":"no active run"}`, http.StatusNotFound)
		return
	}

	cancel()
	log.Printf("HandleCancelRun: cancelled run for chat=%s", chatID)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// GET /api/workspaces/{workspaceId}/chats
func (h *Handler) HandleListChats(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	chats, err := h.store.GetChatsForWorkspace(workspaceID)
	if err != nil {
		http.Error(w, `{"error":"failed to list chats"}`, http.StatusInternalServerError)
		return
	}
	if chats == nil {
		chats = []store.ChatRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}

// GET /api/workspaces/{workspaceId}/chats/{chatId}/messages
func (h *Handler) HandleListMessages(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatId")
	log.Printf("HandleListMessages: chatID=%s", chatID)
	msgs, err := h.store.GetMessagesForChat(chatID)
	if err != nil {
		log.Printf("HandleListMessages error: %v", err)
		http.Error(w, `{"error":"failed to list messages"}`, http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []store.MessageRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

// GET /api/workspaces/{workspaceId}/chats/{chatId}/artifacts
func (h *Handler) HandleListArtifacts(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatId")
	log.Printf("HandleListArtifacts: chatID=%s", chatID)
	artifacts, err := h.store.GetArtifactsForChat(chatID)
	if err != nil {
		log.Printf("HandleListArtifacts error: %v", err)
		http.Error(w, `{"error":"failed to list artifacts"}`, http.StatusInternalServerError)
		return
	}
	if artifacts == nil {
		artifacts = []store.ArtifactRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(artifacts)
}

// GET /api/workspaces
func (h *Handler) HandleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleListWorkspaces")
	workspaces, err := h.store.GetWorkspaces()
	if err != nil {
		log.Printf("HandleListWorkspaces error: %v", err)
		http.Error(w, `{"error":"failed to list workspaces"}`, http.StatusInternalServerError)
		return
	}
	if workspaces == nil {
		workspaces = []store.WorkspaceRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspaces)
}

type createWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// POST /api/workspaces
func (h *Handler) HandleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	if err := h.store.UpsertWorkspace(id, req.Name, req.Description); err != nil {
		log.Printf("create workspace error: %v", err)
		http.Error(w, `{"error":"failed to create workspace"}`, http.StatusInternalServerError)
		return
	}

	// Also create the workspace directory
	if h.workspacesRoot != "" {
		workspacePath := filepath.Join(h.workspacesRoot, id)
		_ = os.MkdirAll(workspacePath, 0o755)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// DELETE /api/workspaces/{workspaceId}
func (h *Handler) HandleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if err := h.store.DeleteWorkspace(workspaceID); err != nil {
		log.Printf("delete workspace error: %v", err)
		http.Error(w, `{"error":"failed to delete workspace"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
