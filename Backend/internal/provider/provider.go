package provider

import (
	"context"
	"encoding/json"
	"time"
)

// Message is a provider-neutral conversation message. No Eino types leak here.
type Message struct {
	Role string // "system" | "user" | "assistant" | "tool"
	// Content is the text content (user/assistant/system text, or the tool
	// result string for tool-role messages).
	Content string
	// ToolCalls is set on assistant messages that requested tool calls.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// ToolCallID and ToolName are set on tool-role result messages.
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments string         `json:"arguments"` // raw JSON string
	Extra     map[string]any `json:"extra,omitempty"`
}

// ToolSpec describes a tool's schema for model tool-binding.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"` // JSON schema of input params
}

// ChatRequest is a single model call request (one ReAct iteration).
type ChatRequest struct {
	SystemPrompt string
	Messages     []Message // system + history + current turn + tool results, in order
	Tools        []ToolSpec
	ChatID       string // NightCode chat ID
}

// ChatEventType classifies a ChatEvent.
type ChatEventType int

const (
	// ChatEventDelta is a streaming text delta. Text carries the delta content.
	ChatEventDelta ChatEventType = iota
	// ChatEventToolCalls is emitted when the model finished and requested tool calls.
	ChatEventToolCalls
	// ChatEventError is a terminal error.
	ChatEventError
	// ChatEventRetry is emitted when a retryable error occurred and the
	// provider is about to wait and try again. Non-terminal — the caller
	// should keep listening on the channel. Carries the error that
	// triggered the retry (so the UI can show *why*, e.g. rate limit) and
	// how long until the next attempt.
	ChatEventRetry
)

// ChatEvent is a provider-level streaming event for a single model call.
type ChatEvent struct {
	Type       ChatEventType
	Text       string      // for ChatEventDelta
	ToolCalls  []ToolCall  // for ChatEventToolCalls
	Err        error       // for ChatEventError, ChatEventRetry
	Attempt    int         // for ChatEventRetry: which attempt is about to run (1-based)
	MaxAttempt int         // for ChatEventRetry: total attempts allowed
	RetryAfter time.Duration // for ChatEventRetry: delay before the next attempt
}

// ErrorKind classifies provider errors for the taxonomy.
type ErrorKind int

const (
	ErrorKindAuth ErrorKind = iota
	ErrorKindRateLimit
	ErrorKindTransient
	ErrorKindOther
)

// ProviderError is a typed provider error. The loop maps it to the taxonomy.
type ProviderError struct {
	Kind      ErrorKind
	Retryable bool
	Err       error
}

func (e *ProviderError) Error() string { return e.Err.Error() }
func (e *ProviderError) Unwrap() error { return e.Err }

// Provider streams a single model call's results as ChatEvents.
type Provider interface {
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
}