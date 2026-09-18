package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestConvertOpenAICompatMessages_BasicRoles(t *testing.T) {
	req := ChatRequest{
		SystemPrompt: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
			{Role: "user", Content: "What's 2+2?"},
		},
	}

	msgs := ConvertOpenAICompatMessages(req)

	if len(msgs) != 4 { // system + 3 messages
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != schema.System || msgs[0].Content != "You are helpful." {
		t.Errorf("expected system message, got role=%v content=%q", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != schema.User || msgs[1].Content != "Hi" {
		t.Errorf("expected user message, got role=%v content=%q", msgs[1].Role, msgs[1].Content)
	}
	if msgs[2].Role != schema.Assistant || msgs[2].Content != "Hello!" {
		t.Errorf("expected assistant message, got role=%v content=%q", msgs[2].Role, msgs[2].Content)
	}
}

func TestConvertOpenAICompatMessages_ToolCallsPreserved(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{
				{ID: "call-1", Name: "read_file", Arguments: `{"path":"foo"}`},
			}},
			{Role: "tool", Content: "file contents", ToolCallID: "call-1", ToolName: "read_file"},
		},
	}

	msgs := ConvertOpenAICompatMessages(req)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].ID != "call-1" {
		t.Errorf("expected tool call preserved, got %v", msgs[0].ToolCalls)
	}
	if msgs[1].Role != schema.Tool || msgs[1].ToolCallID != "call-1" {
		t.Errorf("expected tool response message, got role=%v toolCallID=%q", msgs[1].Role, msgs[1].ToolCallID)
	}
}

func TestConvertOpenAICompatMessages_SkipsUnknownRoles(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "A"},
			{Role: "system", Content: "injected"}, // should be skipped
			{Role: "assistant", Content: "B"},
		},
	}

	msgs := ConvertOpenAICompatMessages(req)

	// system messages from history are skipped (only SystemPrompt is used)
	for _, m := range msgs {
		if m.Role == schema.System && m.Content == "injected" {
			t.Error("system message in history should be skipped")
		}
	}
}

func TestClassifyOpenAICompatError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantKind  ErrorKind
		wantRetry bool
	}{
		{"auth 401", &fakeClassifyErr{msg: "401 Unauthorized"}, ErrorKindAuth, false},
		{"auth invalid key", &fakeClassifyErr{msg: "invalid api key"}, ErrorKindAuth, false},
		{"auth forbidden", &fakeClassifyErr{msg: "403 Forbidden"}, ErrorKindAuth, false},
		{"rate limit 429", &fakeClassifyErr{msg: "429 Too Many Requests"}, ErrorKindRateLimit, true},
		{"rate limit text", &fakeClassifyErr{msg: "rate limit exceeded"}, ErrorKindRateLimit, true},
		{"rate limit many", &fakeClassifyErr{msg: "too many requests"}, ErrorKindRateLimit, true},
		{"transient 500", &fakeClassifyErr{msg: "500 Internal Server Error"}, ErrorKindTransient, true},
		{"transient 502", &fakeClassifyErr{msg: "502 Bad Gateway"}, ErrorKindTransient, true},
		{"transient 503", &fakeClassifyErr{msg: "503 Service Unavailable"}, ErrorKindTransient, true},
		{"transient 504", &fakeClassifyErr{msg: "504 Gateway Timeout"}, ErrorKindTransient, true},
		{"transient timeout", &fakeClassifyErr{err: context.DeadlineExceeded}, ErrorKindTransient, true},
		{"context canceled", &fakeClassifyErr{err: context.Canceled}, ErrorKindOther, false},
		{"other", &fakeClassifyErr{msg: "something weird"}, ErrorKindOther, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perr := ClassifyOpenAICompatError(tt.err)
			if perr.Kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", perr.Kind, tt.wantKind)
			}
			if perr.Retryable != tt.wantRetry {
				t.Errorf("retryable = %v, want %v", perr.Retryable, tt.wantRetry)
			}
		})
	}
}

func TestConvertOpenAICompatToolSpecs(t *testing.T) {
	specs := []ToolSpec{
		{
			Name:        "read_file",
			Description: "Read a file",
			Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
		},
	}

	infos := ConvertOpenAICompatToolSpecs(specs)
	if len(infos) != 1 {
		t.Fatalf("expected 1 tool info, got %d", len(infos))
	}
	if infos[0].Name != "read_file" {
		t.Errorf("expected name=read_file, got %q", infos[0].Name)
	}
	if infos[0].Desc != "Read a file" {
		t.Errorf("expected desc='Read a file', got %q", infos[0].Desc)
	}
}

func TestConvertOpenAICompatToolSpecs_Empty(t *testing.T) {
	if result := ConvertOpenAICompatToolSpecs(nil); result != nil {
		t.Errorf("expected nil for empty specs, got %v", result)
	}
}

// fakeClassifyErr is a minimal error implementation for testing classifyError.
type fakeClassifyErr struct {
	msg string
	err error
}

func (e *fakeClassifyErr) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.msg
}

func (e *fakeClassifyErr) Unwrap() error {
	return e.err
}
