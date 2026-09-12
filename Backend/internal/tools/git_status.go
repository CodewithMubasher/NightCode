package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

type GitStatusInput struct{}

type GitStatusOutput struct {
	Branch      string `json:"branch"`
	IsRepo      bool   `json:"isRepo"`
	StatusShort string `json:"statusShort"`
}

type GitStatusTool struct{}

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string { return "Get the current git branch and status (read-only). Returns isRepo: false if not a git repo." }
func (t *GitStatusTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *GitStatusTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	// Check if git is available
	gitPath, err := exec.LookPath("git")
	if err != nil {
		out := GitStatusOutput{IsRepo: false}
		resultBytes, _ := json.Marshal(out)
		return ToolResult{Output: resultBytes}, nil
	}

	// Check if this is a git repo
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, gitPath, "rev-parse", "--git-dir")
	cmd.Dir = ws.Root
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		out := GitStatusOutput{IsRepo: false}
		resultBytes, _ := json.Marshal(out)
		return ToolResult{Output: resultBytes}, nil
	}

	// Get branch name
	branch := ""
	cmd = exec.CommandContext(ctx, gitPath, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = ws.Root
	if branchOut, err := cmd.Output(); err == nil {
		branch = strings.TrimSpace(string(branchOut))
	}

	// Get short status
	statusShort := ""
	cmd = exec.CommandContext(ctx, gitPath, "status", "--short")
	cmd.Dir = ws.Root
	if statusOut, err := cmd.Output(); err == nil {
		statusShort = strings.TrimSpace(string(statusOut))
	}

	out := GitStatusOutput{
		Branch:      branch,
		IsRepo:      true,
		StatusShort: statusShort,
	}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}
