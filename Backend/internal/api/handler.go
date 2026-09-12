package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/CodewithMubasher/NightCode/backend/internal/agent"
	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
	"github.com/google/uuid"
)

type Handler struct {
	store *store.Store
	// activeRuns tracks in-flight runs by chatID so they can be cancelled.
	activeRuns   map[string]context.CancelFunc
	activeRunsMu sync.Mutex
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{
		store:      s,
		activeRuns: make(map[string]context.CancelFunc),
	}
}

type sendMessageRequest struct {
	Message string `json:"message"`
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

	ctx, cancel := context.WithCancel(r.Context())

	h.activeRunsMu.Lock()
	h.activeRuns[chatID] = cancel
	h.activeRunsMu.Unlock()

	defer func() {
		h.activeRunsMu.Lock()
		delete(h.activeRuns, chatID)
		h.activeRunsMu.Unlock()
	}()

	events := make(chan types.RuntimeEvent, 32)
	go agent.Run(ctx, req.Message, events)

	// Accumulate assistant response for persistence
	var finalText string
	var lastSegmentID string

	encoder := json.NewEncoder(w)
	for event := range events {
		if err := encoder.Encode(event); err != nil {
			log.Printf("write event error: %v", err)
			return
		}
		flusher.Flush()

		if event.Type == "assistant.delta" && event.Text != "__DONE__" {
			finalText = event.Text // each delta contains the full accumulated text
			lastSegmentID = event.SegmentID
		}
	}

	// Persist assistant message with proper TurnSegment[] shape
	if finalText != "" && lastSegmentID != "" {
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
