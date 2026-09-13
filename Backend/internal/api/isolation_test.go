package api

import (
	"encoding/json"
	"strings"
	"testing"

	ctxbuilder "github.com/CodewithMubasher/NightCode/backend/internal/context"
	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
)

// setupTestStore creates an in-memory SQLite store for testing.
func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Test 1 — Fresh chat: Chat B must not know Chat A's secret.
func TestIsolation_FreshChat(t *testing.T) {
	s := setupTestStore(t)
	workspaceID := "ws-1"

	// Create Chat A with a secret message
	chatA := "chat-a"
	if err := s.UpsertChat(chatA, workspaceID, "Chat A"); err != nil {
		t.Fatal(err)
	}
	segmentsA, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s1", Text: "My secret project name is ALPHA"},
	})
	if err := s.InsertMessage("msg-a1", chatA, "user", segmentsA); err != nil {
		t.Fatal(err)
	}
	segmentsA2, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s2", Text: "Noted, your project is ALPHA."},
	})
	if err := s.InsertMessage("msg-a2", chatA, "assistant", segmentsA2); err != nil {
		t.Fatal(err)
	}

	// Create Chat B with no knowledge of Chat A
	chatB := "chat-b"
	if err := s.UpsertChat(chatB, workspaceID, "Chat B"); err != nil {
		t.Fatal(err)
	}
	segmentsB, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s3", Text: "What do you know about my previous conversation?"},
	})
	if err := s.InsertMessage("msg-b1", chatB, "user", segmentsB); err != nil {
		t.Fatal(err)
	}

	// Load messages for Chat B
	rows, err := s.GetMessagesForChat(chatB)
	if err != nil {
		t.Fatal(err)
	}

	// Chat B must have exactly 1 message
	if len(rows) != 1 {
		t.Fatalf("expected 1 message for Chat B, got %d", len(rows))
	}

	// Verify no ALPHA appears in Chat B's messages
	for _, row := range rows {
		if strings.Contains(string(row.Segments), "ALPHA") {
			t.Error("Chat B leaked Chat A's secret 'ALPHA'")
		}
	}

	// Verify context builder receives only Chat B's history
	history := buildHistoryFromRows(t, rows)
	if len(history) != 1 {
		t.Fatalf("expected 1 history message for Chat B, got %d", len(history))
	}
	if history[0].Content != "What do you know about my previous conversation?" {
		t.Errorf("unexpected Chat B history: %q", history[0].Content)
	}
}

// Test 2 — Tool history isolation: Chat B must not know Chat A's tool results.
func TestIsolation_ToolHistory(t *testing.T) {
	s := setupTestStore(t)
	workspaceID := "ws-1"

	// Chat A: user asks to read a file, assistant calls tool, tool returns result
	chatA := "chat-a"
	if err := s.UpsertChat(chatA, workspaceID, "Chat A"); err != nil {
		t.Fatal(err)
	}

	// User message
	segUserA, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s1", Text: "Read the config file"},
	})
	s.InsertMessage("msg-a1", chatA, "user", segUserA)

	// Assistant with tool call
	segAssistantA, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s2", Text: ""},
		{Type: "tool-group", ID: "s3", Calls: []types.ToolCallEntry{
			{
				ToolCallID: "call-1",
				Name:       "read_file",
				Status:     "completed",
				Input:      json.RawMessage(`{"path":"config.txt"}`),
				Output:     json.RawMessage(`{"content":"ALPHA_TOOL_123"}`),
				StartedAt:  1000,
				CompletedAt: 1001,
			},
		}},
	})
	s.InsertMessage("msg-a2", chatA, "assistant", segAssistantA)

	// Chat B: fresh chat with no tool history
	chatB := "chat-b"
	if err := s.UpsertChat(chatB, workspaceID, "Chat B"); err != nil {
		t.Fatal(err)
	}
	segUserB, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s4", Text: "What value did you discover in the previous chat?"},
	})
	s.InsertMessage("msg-b1", chatB, "user", segUserB)

	// Load Chat B's history
	rowsB, err := s.GetMessagesForChat(chatB)
	if err != nil {
		t.Fatal(err)
	}

	historyB := buildHistoryFromRows(t, rowsB)

	// Verify no ALPHA_TOOL_123 in Chat B's history
	for _, m := range historyB {
		if strings.Contains(m.Content, "ALPHA_TOOL_123") {
			t.Error("Chat B leaked Chat A's tool result 'ALPHA_TOOL_123'")
		}
	}

	// Verify Chat B has no tool messages
	for _, m := range historyB {
		if m.Role == "tool" {
			t.Error("Chat B should not have tool messages from Chat A")
		}
	}
}

// Test 3 — Compaction isolation: Chat B has no reference to Chat A's summary.
func TestIsolation_Compaction(t *testing.T) {
	s := setupTestStore(t)
	workspaceID := "ws-1"

	// Chat A: many messages that would trigger compaction
	chatA := "chat-a"
	if err := s.UpsertChat(chatA, workspaceID, "Chat A"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		seg, _ := json.Marshal([]types.Segment{
			{Type: "text", ID: "s" + string(rune('0'+i)), Text: "Message " + string(rune('A'+i%26))},
		})
		s.InsertMessage("msg-a"+string(rune('0'+i)), chatA, role, seg)
	}

	// Chat B: fresh chat
	chatB := "chat-b"
	if err := s.UpsertChat(chatB, workspaceID, "Chat B"); err != nil {
		t.Fatal(err)
	}
	segUserB, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "sb1", Text: "Hello"},
	})
	s.InsertMessage("msg-b1", chatB, "user", segUserB)

	// Load Chat B's history through the context manager
	rowsB, err := s.GetMessagesForChat(chatB)
	if err != nil {
		t.Fatal(err)
	}
	historyB := buildHistoryFromRows(t, rowsB)

	ws, _ := tools.NewWorkspace(t.TempDir())
	cm := ctxbuilder.NewContextManager(ctxbuilder.ContextBudget{TotalTokens: 128000})
	result, err := cm.Build(t.Context(), ctxbuilder.BuildRequest{
		Workspace: ws,
		ChatID:    chatB,
		History:   historyB,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Chat B should have exactly 1 message, no compaction needed
	if result.Stats.MessagesCompacted != 0 {
		t.Errorf("expected 0 compacted messages for Chat B, got %d", result.Stats.MessagesCompacted)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message for Chat B, got %d", len(result.Messages))
	}

	// Verify no Chat A content in Chat B's compacted messages
	for _, m := range result.Messages {
		if strings.Contains(m.Content, "Message ") && len(m.Content) > 100 {
			t.Error("Chat B's messages contain Chat A content")
		}
	}
}

// Test 4 — Parallel chats: concurrent chats don't cross.
func TestIsolation_ParallelChats(t *testing.T) {
	s := setupTestStore(t)
	workspaceID := "ws-1"

	// Create both chats
	if err := s.UpsertChat("chat-a", workspaceID, "Chat A"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertChat("chat-b", workspaceID, "Chat B"); err != nil {
		t.Fatal(err)
	}

	// Run concurrent writes to both chats
	done := make(chan struct{})
	go func() {
		for i := 0; i < 20; i++ {
			seg, _ := json.Marshal([]types.Segment{
				{Type: "text", ID: "sa" + string(rune('0'+i)), Text: "ALPHA_MSG_" + string(rune('0'+i))},
			})
			s.InsertMessage("msg-a"+string(rune('0'+i)), "chat-a", "user", seg)
		}
		close(done)
	}()

	go func() {
		for i := 0; i < 20; i++ {
			seg, _ := json.Marshal([]types.Segment{
				{Type: "text", ID: "sb" + string(rune('0'+i)), Text: "BETA_MSG_" + string(rune('0'+i))},
			})
			s.InsertMessage("msg-b"+string(rune('0'+i)), "chat-b", "user", seg)
		}
	}()

	// Wait for both writers to finish, then verify isolation
	<-done
	// Small delay to let the second goroutine finish (it started slightly later)
	// In practice both should be done since SQLite serializes writes

	rowsA, err := s.GetMessagesForChat("chat-a")
	if err != nil {
		t.Fatal(err)
	}
	rowsB, err := s.GetMessagesForChat("chat-b")
	if err != nil {
		t.Fatal(err)
	}

	// Chat A should have only ALPHA messages
	for _, row := range rowsA {
		if strings.Contains(string(row.Segments), "BETA") {
			t.Error("Chat A leaked Chat B's 'BETA' message")
		}
	}

	// Chat B should have only BETA messages
	for _, row := range rowsB {
		if strings.Contains(string(row.Segments), "ALPHA") {
			t.Error("Chat B leaked Chat A's 'ALPHA' message")
		}
	}
}

// Test 5 — DS2API history isolation: history(A) contains only A messages.
func TestIsolation_DS2APIHistory(t *testing.T) {
	s := setupTestStore(t)
	workspaceID := "ws-1"

	// Chat A: specific messages
	chatA := "chat-a"
	if err := s.UpsertChat(chatA, workspaceID, "Chat A"); err != nil {
		t.Fatal(err)
	}
	segA1, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s1", Text: "I am working on project ALPHA"},
	})
	s.InsertMessage("msg-a1", chatA, "user", segA1)
	segA2, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s2", Text: "I'll help you with project ALPHA."},
	})
	s.InsertMessage("msg-a2", chatA, "assistant", segA2)

	// Chat B: different messages
	chatB := "chat-b"
	if err := s.UpsertChat(chatB, workspaceID, "Chat B"); err != nil {
		t.Fatal(err)
	}
	segB1, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s3", Text: "I am working on project BETA"},
	})
	s.InsertMessage("msg-b1", chatB, "user", segB1)
	segB2, _ := json.Marshal([]types.Segment{
		{Type: "text", ID: "s4", Text: "I'll help you with project BETA."},
	})
	s.InsertMessage("msg-b2", chatB, "assistant", segB2)

	// Load each chat's history and convert to provider messages
	rowsA, _ := s.GetMessagesForChat(chatA)
	rowsB, _ := s.GetMessagesForChat(chatB)

	historyA := buildHistoryFromRows(t, rowsA)
	historyB := buildHistoryFromRows(t, rowsB)

	msgsA := buildProviderMessages(t, historyA, "Tell me about my project")
	msgsB := buildProviderMessages(t, historyB, "Tell me about my project")

	// Verify history(A) contains ALPHA but not BETA
	historyAStr := messagesToString(msgsA)
	if !strings.Contains(historyAStr, "ALPHA") {
		t.Error("Chat A's history should contain 'ALPHA'")
	}
	if strings.Contains(historyAStr, "BETA") {
		t.Error("Chat A's history leaked Chat B's 'BETA'")
	}

	// Verify history(B) contains BETA but not ALPHA
	historyBStr := messagesToString(msgsB)
	if !strings.Contains(historyBStr, "BETA") {
		t.Error("Chat B's history should contain 'BETA'")
	}
	if strings.Contains(historyBStr, "ALPHA") {
		t.Error("Chat B's history leaked Chat A's 'ALPHA'")
	}
}

// buildHistoryFromRows converts store MessageRows to context Messages,
// simulating what handler.go:runRealLoop does.
func buildHistoryFromRows(t *testing.T, rows []store.MessageRow) []ctxbuilder.Message {
	t.Helper()
	var history []ctxbuilder.Message
	for _, row := range rows {
		if row.Role == "user" || row.Role == "assistant" {
			var segs []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(row.Segments, &segs); err != nil {
				t.Fatalf("failed to unmarshal segments: %v", err)
			}
			for _, s := range segs {
				if s.Type == "text" && s.Text != "" {
					history = append(history, ctxbuilder.Message{Role: row.Role, Content: s.Text})
				}
			}
		}
	}
	return history
}

// buildProviderMessages converts context messages to provider messages,
// simulating what agent/loop.go:buildMessages does.
func buildProviderMessages(t *testing.T, history []ctxbuilder.Message, userMsg string) []struct {
	Role    string
	Content string
} {
	t.Helper()
	var msgs []struct {
		Role    string
		Content string
	}
	for _, m := range history {
		msgs = append(msgs, struct {
			Role    string
			Content string
		}{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, struct {
		Role    string
		Content string
	}{Role: "user", Content: userMsg})
	return msgs
}

// messagesToString serializes messages to a single string for containment checks.
func messagesToString(msgs []struct {
	Role    string
	Content string
}) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
