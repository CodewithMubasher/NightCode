package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type ListDirInput struct {
	Path string `json:"path"`
}

type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type ListDirOutput struct {
	Entries []DirEntry `json:"entries"`
}

type ListDirTool struct{}

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return "List the immediate children of a directory (non-recursive)." }
func (t *ListDirTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path relative to workspace root (defaults to workspace root)"}
		}
	}`)
}

func (t *ListDirTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in ListDirInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	target := ws.Root
	if in.Path != "" {
		var err error
		target, err = ws.ResolvePath(in.Path)
		if err != nil {
			return ToolResult{}, err
		}
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{}, &ToolError{Code: "not_found", Message: fmt.Sprintf("directory not found: %s", in.Path)}
		}
		return ToolResult{}, &ToolError{Code: "read_error", Message: fmt.Sprintf("cannot stat directory: %v", err)}
	}
	if !info.IsDir() {
		return ToolResult{}, &ToolError{Code: "not_a_directory", Message: fmt.Sprintf("path is not a directory: %s", in.Path)}
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return ToolResult{}, &ToolError{Code: "read_error", Message: fmt.Sprintf("cannot read directory: %v", err)}
	}

	var result []DirEntry
	for _, entry := range entries {
		// Skip hidden files
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, DirEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	// Sort: directories first, then alphabetical
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	if result == nil {
		result = []DirEntry{}
	}

	out := ListDirOutput{Entries: result}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}
