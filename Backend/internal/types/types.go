package types

import "encoding/json"

// RuntimeEvent is the wire format for SSE events sent to the frontend.
// Flat structure matches Frontend/src/types/events.ts exactly.
type RuntimeEvent struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	SegmentID string          `json:"segmentId"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`

	// assistant.delta
	Text string `json:"text,omitempty"`

	// tool.started / tool.completed / tool.failed
	ToolCallID string `json:"toolCallId,omitempty"`
	Name       string `json:"name,omitempty"`
	Input      any    `json:"input,omitempty"`
	Output     any    `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	Duration   int64  `json:"duration,omitempty"`

	// artifact.created
	ArtifactID   string `json:"artifactId,omitempty"`
	Content      string `json:"content,omitempty"`
	ArtifactType string `json:"artifactType,omitempty"`
	Language     string `json:"language,omitempty"`

	// turn.error
	Code      string `json:"code,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`

	// turn.retry
	Attempt      int   `json:"attempt,omitempty"`
	MaxAttempt   int   `json:"maxAttempt,omitempty"`
	RetryAfterMs int64 `json:"retryAfterMs,omitempty"`
}

// Segment is a single turn segment for persistence. Matches the frontend
// TurnSegment model (text | tool-group).
type Segment struct {
	Type  string          `json:"type"`            // "text" | "tool-group"
	ID    string          `json:"id"`              //
	Text  string          `json:"text,omitempty"`  // for text segments
	Calls []ToolCallEntry `json:"calls,omitempty"` // for tool-group segments
}

// ToolCallEntry tracks a single tool invocation within a tool-group segment.
// Matches the frontend ToolCallEntry model.
type ToolCallEntry struct {
	ToolCallID   string          `json:"toolCallId"`
	Name         string          `json:"name"`
	Status       string          `json:"status"` // "running" | "completed" | "failed"
	Input        json.RawMessage `json:"input,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	StartedAt    int64           `json:"startedAt"`
	CompletedAt  int64           `json:"completedAt,omitempty"`
	ReversalData json.RawMessage `json:"reversalData,omitempty"`
}
