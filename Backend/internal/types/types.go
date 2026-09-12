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
}
