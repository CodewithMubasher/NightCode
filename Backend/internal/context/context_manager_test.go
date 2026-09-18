package context

import (
	"context"
	"path/filepath"
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

	// Create a tool result message that exceeds the 2000-char compaction threshold
	largeOutput := strings.Repeat("line of output\n", 200) // ~3000 chars, exceeds 2000
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

// TestCompactMessage_ThresholdBelow2000 ensures messages under 2000 chars
// are NOT compacted, preserving meaningful context.
func TestCompactMessage_ThresholdBelow2000(t *testing.T) {
	// 1500 chars — below threshold, should NOT be compacted
	content := strings.Repeat("a", 1500)
	msg := Message{Role: "tool", Content: content, ToolCallID: "c1", ToolName: "shell"}
	compacted := compactMessage(msg)

	if compacted.Content != content {
		t.Errorf("message under 2000 chars should not be compacted, got %d chars", len(compacted.Content))
	}
}

// TestCompactMessage_ThresholdAbove2000 ensures messages over 2000 chars
// ARE compacted to 2000 chars.
func TestCompactMessage_ThresholdAbove2000(t *testing.T) {
	content := strings.Repeat("b", 3000)
	msg := Message{Role: "tool", Content: content, ToolCallID: "c1", ToolName: "shell"}
	compacted := compactMessage(msg)

	if len(compacted.Content) >= 3000 {
		t.Errorf("message over 2000 chars should be compacted, still %d chars", len(compacted.Content))
	}
	if !strings.Contains(compacted.Content, "truncated") {
		t.Error("expected truncation notice in compacted output")
	}
	if !strings.Contains(compacted.Content, "3000") {
		t.Error("expected original length in truncation notice")
	}
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

func TestContextManager_SystemPromptBudget_WithinBudget(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Tiny budget but no extra sections — should work fine
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
	// No truncation warnings expected for a normal workspace
	for _, w := range result.Warnings {
		if strings.Contains(w, "over budget") {
			t.Errorf("unexpected budget warning: %s", w)
		}
	}
}

func TestContextManager_SystemPromptBudget_EnforcedWithWarning(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create a very large instructions file to blow the budget
	instructions := strings.Repeat("# Large Instruction\n" + strings.Repeat("Details here. ", 50) + "\n", 200)
	writeFile(t, filepath.Join(ws.Root, ".nightcode", "instructions.md"), instructions)

	// Use a tiny budget so the system prompt exceeds it
	cm := NewContextManager(ContextBudget{TotalTokens: 500})
	result, err := cm.Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "test",
		History:   nil,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have at least one "over budget" warning
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "over budget") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an 'over budget' warning but none was emitted")
	}
}

// TestCompactMessage_EmptyContent verifies empty content passes through.
func TestCompactMessage_EmptyContent(t *testing.T) {
	msg := Message{Role: "tool", Content: ""}
	compacted := compactMessage(msg)
	if compacted.Content != "" {
		t.Errorf("expected empty content unchanged, got %q", compacted.Content)
	}
}

// TestCompactMessage_UserNeverTruncated verifies user messages are never compacted.
func TestCompactMessage_UserNeverTruncated(t *testing.T) {
	content := strings.Repeat("u", 5000)
	msg := Message{Role: "user", Content: content}
	compacted := compactMessage(msg)
	if compacted.Content != content {
		t.Error("user messages should never be truncated")
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
