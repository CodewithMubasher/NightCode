package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	readFileMaxLines = 2000
	readFileMaxBytes = 100 * 1024 // 100KB
)

type ReadFileInput struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

type ReadFileOutput struct {
	Content    string `json:"content"`
	LineCount  int    `json:"lineCount"`
	Truncated  bool   `json:"truncated"`
	TotalLines int    `json:"totalLines"`
}

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file, with optional line range." }
func (t *ReadFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path relative to workspace root"},
			"start_line": {"type": "integer", "description": "Start line (1-indexed, inclusive)"},
			"end_line": {"type": "integer", "description": "End line (1-indexed, inclusive)"}
		},
		"required": ["path"]
	}`)
}

func (t *ReadFileTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in ReadFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	resolved, err := ws.ResolvePath(in.Path)
	if err != nil {
		return ToolResult{}, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{}, &ToolError{Code: "file_not_found", Message: fmt.Sprintf("file not found: %s", in.Path)}
		}
		return ToolResult{}, &ToolError{Code: "read_error", Message: fmt.Sprintf("cannot read file: %v", err)}
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return ToolResult{}, &ToolError{Code: "read_error", Message: fmt.Sprintf("error reading file: %v", err)}
	}

	totalLines := len(lines)
	truncated := false

	startLine := in.StartLine
	if startLine < 1 {
		startLine = 1
	}
	endLine := in.EndLine
	if endLine < 1 || endLine > totalLines {
		endLine = totalLines
	}

	// Apply cap
	if endLine-startLine+1 > readFileMaxLines {
		endLine = startLine + readFileMaxLines - 1
		truncated = true
	}

	// Also cap by byte size
	selected := lines[startLine-1 : endLine]
	if !truncated {
		totalBytes := 0
		for i, line := range selected {
			totalBytes += len(line) + 1 // +1 for newline
			if totalBytes > readFileMaxBytes {
				selected = selected[:i+1]
				endLine = startLine + i
				truncated = true
				break
			}
		}
	}

	var sb strings.Builder
	for i, line := range selected {
		fmt.Fprintf(&sb, "%d: %s", startLine+i, line)
		if i < len(selected)-1 {
			sb.WriteByte('\n')
		}
	}

	out := ReadFileOutput{
		Content:    sb.String(),
		LineCount:  len(selected),
		Truncated:  truncated,
		TotalLines: totalLines,
	}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}


