package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const gitDiffMaxBytes = 100 * 1024 // 100KB

type GitDiffInput struct {
	Path string `json:"path,omitempty"`
}

type GitDiffOutput struct {
	Diff      string `json:"diff"`
	Truncated bool   `json:"truncated"`
}

type GitDiffTool struct{}

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string { return "Get the unified diff for a file or the entire working tree (read-only)." }
func (t *GitDiffTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path to diff (omit for entire working tree)"}
		}
	}`)
}

func (t *GitDiffTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in GitDiffInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		return ToolResult{}, &ToolError{Code: "git_not_found", Message: "git is not installed or not in PATH"}
	}

	// Verify this is a git repo
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, gitPath, "rev-parse", "--git-dir")
	cmd.Dir = ws.Root
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return ToolResult{}, &ToolError{Code: "not_a_git_repo", Message: "workspace is not a git repository"}
	}

	args := []string{"diff"}
	if in.Path != "" {
		// Validate path is within sandbox
		if _, err := ws.ResolvePath(in.Path); err != nil {
			return ToolResult{}, err
		}
		// Pass the path relative to workspace root to git
		args = append(args, "--", in.Path)
	}

	cmd = exec.CommandContext(ctx, gitPath, args...)
	cmd.Dir = ws.Root
	out, err := cmd.Output()
	if err != nil {
		return ToolResult{}, &ToolError{Code: "git_error", Message: fmt.Sprintf("git diff failed: %v", err)}
	}

	diff := string(out)
	truncated := false
	if len(diff) > gitDiffMaxBytes {
		diff = diff[:gitDiffMaxBytes]
		truncated = true
	}

	result := GitDiffOutput{
		Diff:      strings.TrimRight(diff, "\n"),
		Truncated: truncated,
	}
	resultBytes, _ := json.Marshal(result)
	return ToolResult{Output: resultBytes}, nil
}
