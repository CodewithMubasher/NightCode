package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Directories to always skip in tree view.
var treeIgnoreDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"target":       true,
	"vendor":       true,
	".cache":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".idea":        true,
	".vscode":      true,
}

type ListDirInput struct {
	Path string `json:"path"`
	Tree bool   `json:"tree"`
}

type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type ListDirOutput struct {
	Entries []DirEntry `json:"entries,omitempty"`
	Tree    string     `json:"tree,omitempty"`
}

type ListDirTool struct{}

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return "List directory contents. Use tree:true for recursive tree view." }
func (t *ListDirTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path relative to workspace root (defaults to workspace root)"},
			"tree": {"type": "boolean", "description": "If true, return a recursive tree view (default false)"}
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

	if in.Tree {
		return t.executeTree(target, ws)
	}
	return t.executeFlat(target)
}

func (t *ListDirTool) executeFlat(target string) (ToolResult, error) {
	entries, err := os.ReadDir(target)
	if err != nil {
		return ToolResult{}, &ToolError{Code: "read_error", Message: fmt.Sprintf("cannot read directory: %v", err)}
	}

	var result []DirEntry
	for _, entry := range entries {
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

func (t *ListDirTool) executeTree(target string, ws *Workspace) (ToolResult, error) {
	var sb strings.Builder
	relRoot, _ := filepath.Rel(ws.Root, target)
	if relRoot == "." {
		relRoot = filepath.Base(ws.Root)
	}
	sb.WriteString(relRoot + "/")

	var walk func(dir string, prefix string)
	walk = func(dir string, prefix string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		// Sort: dirs first, then alphabetical
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})

		// Filter out hidden and ignored
		var filtered []os.DirEntry
		for _, e := range entries {
			name := e.Name()
			if len(name) > 0 && name[0] == '.' {
				continue
			}
			if e.IsDir() && treeIgnoreDirs[name] {
				continue
			}
			filtered = append(filtered, e)
		}

		for i, entry := range filtered {
			isLast := i == len(filtered)-1
			connector := "├── "
			if isLast {
				connector = "└── "
			}

			if entry.IsDir() {
				sb.WriteString(prefix + connector + entry.Name() + "/\n")
				newPrefix := prefix + "│   "
				if isLast {
					newPrefix = prefix + "    "
				}
				walk(filepath.Join(dir, entry.Name()), newPrefix)
			} else {
				sb.WriteString(prefix + connector + entry.Name() + "\n")
			}
		}
	}

	walk(target, "")

	out := ListDirOutput{Tree: sb.String()}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}
