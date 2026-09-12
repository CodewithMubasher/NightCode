package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error)
}

// ToolResult is returned by Tool.Execute on success.
type ToolResult struct {
	Output   json.RawMessage `json:"output"`
	Artifact *ArtifactRef    `json:"artifact,omitempty"`
}

// ArtifactRef signals that a tool call should create an artifact in the frontend.
type ArtifactRef struct {
	Name         string `json:"name"`
	ArtifactType string `json:"artifactType"` // "document" | "code"
	Language     string `json:"language"`
	Content      string `json:"content"`
}

// ToolError is a typed recoverable error from a tool execution.
type ToolError struct {
	Code    string `json:"code"`    // e.g. "file_not_found", "ambiguous_edit", "invalid_input"
	Message string `json:"message"`
}

func (e *ToolError) Error() string {
	return e.Message
}
