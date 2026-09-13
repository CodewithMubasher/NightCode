package context

import (
	"context"
	"strings"
	"testing"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

func TestContextManager_BasicBuild(t *testing.T) {
	ws := setupTestWorkspace(t)

	cm := NewContextManager(ContextBudget{TotalTokens: 128000})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   nil,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if result.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if result.Stats.BudgetTokens <= 0 {
		t.Error("expected positive budget tokens")
	}
}

func TestContextManager_HistoryWithinBudget(t *testing.T) {
	ws := setupTestWorkspace(t)

	history := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you"},
		{Role: "assistant", Content: "doing well"},
	}

	cm := NewContextManager(ContextBudget{TotalTokens: 128000})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   history,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Stats.MessagesCompacted != 0 {
		t.Errorf("expected 0 compacted messages, got %d", result.Stats.MessagesCompacted)
	}
	if result.Stats.MessagesKept != 4 {
		t.Errorf("expected 4 kept messages, got %d", result.Stats.MessagesKept)
	}
	if len(result.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(result.Messages))
	}
}

func TestContextManager_HistoryExceedsBudget(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create enough history to exceed a very small budget
	history := make([]Message, 50)
	for i := range history {
		if i%2 == 0 {
			history[i] = Message{Role: "user", Content: strings.Repeat("x", 200)}
		} else {
			history[i] = Message{Role: "assistant", Content: strings.Repeat("y", 200)}
		}
	}

	// Tiny budget: 1000 tokens total, ~250 for history
	cm := NewContextManager(ContextBudget{TotalTokens: 1000})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   history,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Stats.MessagesCompacted == 0 {
		t.Error("expected some messages to be compacted")
	}
	if len(result.Messages) != 50 {
		t.Errorf("expected all 50 messages preserved (some compacted), got %d", len(result.Messages))
	}
}

func TestContextManager_ToolResultCompaction(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create a tool result message with very large output
	largeOutput := strings.Repeat("line of output\n", 500) // ~7500 chars
	history := []Message{
		{Role: "user", Content: "run something"},
		{Role: "assistant", Content: "", ToolCalls: []ToolCallInfo{
			{ID: "c1", Name: "shell", Arguments: `{"command":"ls"}`},
		}},
		{Role: "tool", Content: largeOutput, ToolCallID: "c1", ToolName: "shell"},
		{Role: "assistant", Content: "done"},
	}

	// Tiny budget forces compaction
	cm := NewContextManager(ContextBudget{TotalTokens: 500})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   history,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Find the tool message
	for _, m := range result.Messages {
		if m.Role == "tool" {
			if len(m.Content) >= len(largeOutput) {
				t.Error("expected tool result to be compacted")
			}
			if !strings.Contains(m.Content, "truncated") {
				t.Error("expected truncation notice in compacted tool result")
			}
			if m.ToolCallID != "c1" {
				t.Errorf("expected ToolCallID preserved, got %q", m.ToolCallID)
			}
			if m.ToolName != "shell" {
				t.Errorf("expected ToolName preserved, got %q", m.ToolName)
			}
			return
		}
	}
	t.Error("expected to find a tool message")
}

func TestContextManager_ToolCallsPreserved(t *testing.T) {
	ws := setupTestWorkspace(t)

	history := []Message{
		{Role: "assistant", Content: "let me check", ToolCalls: []ToolCallInfo{
			{ID: "c1", Name: "read_file", Arguments: `{"path":"main.go"}`},
			{ID: "c2", Name: "shell", Arguments: `{"command":"go test"}`},
		}},
		{Role: "tool", Content: "file content here", ToolCallID: "c1", ToolName: "read_file"},
		{Role: "tool", Content: "ok", ToolCallID: "c2", ToolName: "shell"},
	}

	cm := NewContextManager(ContextBudget{TotalTokens: 128000})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   history,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Find the assistant message with tool calls
	for _, m := range result.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			if len(m.ToolCalls) != 2 {
				t.Errorf("expected 2 tool calls preserved, got %d", len(m.ToolCalls))
			}
			if m.ToolCalls[0].ID != "c1" || m.ToolCalls[0].Name != "read_file" {
				t.Errorf("unexpected first tool call: %+v", m.ToolCalls[0])
			}
			if m.ToolCalls[1].ID != "c2" || m.ToolCalls[1].Name != "shell" {
				t.Errorf("unexpected second tool call: %+v", m.ToolCalls[1])
			}
			return
		}
	}
	t.Error("expected to find assistant message with tool calls")
}

func TestContextManager_KeepsRecentMessages(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create 10 messages, all within budget
	history := make([]Message, 10)
	for i := range history {
		if i%2 == 0 {
			history[i] = Message{Role: "user", Content: "msg"}
		} else {
			history[i] = Message{Role: "assistant", Content: "reply"}
		}
	}

	cm := NewContextManager(ContextBudget{TotalTokens: 128000})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   history,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// All messages should be kept unchanged
	if len(result.Messages) != 10 {
		t.Errorf("expected 10 messages, got %d", len(result.Messages))
	}
	for i, m := range result.Messages {
		if m.Content != history[i].Content {
			t.Errorf("message %d content changed: expected %q, got %q", i, history[i].Content, m.Content)
		}
	}
}

func TestContextManager_TokenEstimation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minTokens int
		maxTokens int
	}{
		{"empty", "", 0, 1},
		{"short", "hello", 1, 2},
		{"medium", "this is a test string", 5, 7},
		{"code", "func main() { fmt.Println(\"hello\") }", 9, 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := estimateTokens(tt.input)
			if tokens < tt.minTokens || tokens > tt.maxTokens {
				t.Errorf("estimateTokens(%q) = %d, want [%d, %d]", tt.input, tokens, tt.minTokens, tt.maxTokens)
			}
		})
	}
}

func TestContextManager_BudgetCalculations(t *testing.T) {
	budget := ContextBudget{TotalTokens: 128000}

	usable := budget.usableTokens() // 128000 * 0.75 = 96000
	if usable != 96000 {
		t.Errorf("usableTokens = %d, want 96000", usable)
	}

	sysLimit := budget.systemTokenLimit() // 96000 * 0.30 = 28800
	if sysLimit != 28800 {
		t.Errorf("systemTokenLimit = %d, want 28800", sysLimit)
	}

	histBudget := budget.historyTokenBudget() // 96000 - 28800 = 67200
	if histBudget != 67200 {
		t.Errorf("historyTokenBudget = %d, want 67200", histBudget)
	}
}

func setupTestWorkspace(t *testing.T) *tools.Workspace {
	t.Helper()
	ws, _ := tools.NewWorkspace(t.TempDir())
	return ws
}
