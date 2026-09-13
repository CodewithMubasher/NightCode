package context

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

// buildGitContext reuses the Step 2 git_status tool directly (no reimplementation).
// Returns "" when the workspace is not a git repo (normal, expected case — no
// warning, no error).
func buildGitContext(ctx context.Context, ws *tools.Workspace) string {
	tool := &tools.GitStatusTool{}
	result, err := tool.Execute(ctx, json.RawMessage(`{}`), ws)
	if err != nil {
		return ""
	}

	var out tools.GitStatusOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		return ""
	}
	if !out.IsRepo {
		return ""
	}

	var sb strings.Builder
	status := strings.TrimSpace(out.StatusShort)
	if status == "" {
		sb.WriteString("On branch " + out.Branch + ", working tree clean\n")
		return sb.String()
	}

	// Count changed files from git status --short output (one entry per line)
	lineCount := len(strings.Split(strings.TrimSpace(status), "\n"))
	sb.WriteString("On branch " + out.Branch + ", " + strconv.Itoa(lineCount) + " file(s) changed:\n")
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if strings.TrimSpace(line) != "" {
			sb.WriteString("  " + line + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}