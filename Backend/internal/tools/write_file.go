package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteFileOutput struct {
	Written  int    `json:"written"`
	AbsPath  string `json:"absPath"`
	NewFile  bool   `json:"newFile"`
}

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Create or overwrite a file with the given content. Creates parent directories as needed." }
func (t *WriteFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path relative to workspace root"},
			"content": {"type": "string", "description": "File content to write"}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteFileTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in WriteFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	if in.Path == "" {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "path is required"}
	}

	resolved, err := ws.ResolvePath(in.Path)
	if err != nil {
		return ToolResult{}, err
	}

	// Check if the file already exists
	newFile := true
	if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
		newFile = false
	}

	// Verify parent directory is within sandbox
	parentDir := filepath.Dir(resolved)
	if _, err := ws.ResolvePath(filepath.Join(in.Path, "..")); err != nil {
		// Fallback: check the parent dir directly
		if !isWithinRoot(parentDir, ws.Root) {
			return ToolResult{}, &SandboxError{Code: "sandbox_violation", Msg: "parent directory escapes sandbox"}
		}
	}

	// Create parent directories
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return ToolResult{}, &ToolError{Code: "write_error", Message: fmt.Sprintf("cannot create directories: %v", err)}
	}

	if err := os.WriteFile(resolved, []byte(in.Content), 0o644); err != nil {
		return ToolResult{}, &ToolError{Code: "write_error", Message: fmt.Sprintf("cannot write file: %v", err)}
	}

	out := WriteFileOutput{
		Written: len(in.Content),
		AbsPath: resolved,
		NewFile: newFile,
	}
	resultBytes, _ := json.Marshal(out)

	artifact := &ArtifactRef{
		Name:         in.Path,
		ArtifactType: "code",
		Language:     langFromExt(in.Path),
		Content:      in.Content,
	}

	return ToolResult{Output: resultBytes, Artifact: artifact}, nil
}

func isWithinRoot(path, root string) bool {
	if path == root {
		return true
	}
	return len(path) > len(root) && path[:len(root)] == root && path[len(root)] == filepath.Separator
}

func langFromExt(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".cs":
		return "csharp"
	case ".sh", ".bash":
		return "bash"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".md", ".markdown":
		return "markdown"
	case ".sql":
		return "sql"
	case ".env":
		return "bash"
	case ".txt":
		return "plaintext"
	default:
		return "plaintext"
	}
}
