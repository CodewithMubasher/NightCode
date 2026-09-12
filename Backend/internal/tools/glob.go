package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type GlobInput struct {
	Pattern string `json:"pattern"`
}

type GlobOutput struct {
	Files     []string `json:"files"`
	Truncated bool     `json:"truncated"`
}

type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Find files matching a glob pattern. Supports ** for recursive matching." }
func (t *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern (supports ** for recursive matching, e.g. **/*.go)"}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in GlobInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	if in.Pattern == "" {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "pattern is required"}
	}

	// Use doublestar for ** support, operating on the workspace root as an fs.FS
	fsys := os.DirFS(ws.Root)
	matches, err := doublestar.Glob(fsys, in.Pattern)
	if err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_glob", Message: fmt.Sprintf("invalid glob pattern: %v", err)}
	}

	var files []string
	for _, match := range matches {
		// Skip hidden files/dirs
		parts := strings.Split(match, string(filepath.Separator))
		skip := false
		for _, part := range parts {
			if strings.HasPrefix(part, ".") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		files = append(files, filepath.ToSlash(match))
	}

	truncated := false
	if len(files) > 500 {
		files = files[:500]
		truncated = true
	}

	if files == nil {
		files = []string{}
	}

	out := GlobOutput{
		Files:     files,
		Truncated: truncated,
	}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}
