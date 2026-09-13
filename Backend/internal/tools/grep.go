package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const grepMaxMatches = 200

// rgLookPath is injectable so tests can force the native fallback path.
var rgLookPath = exec.LookPath

type GrepInput struct {
	Pattern  string `json:"pattern"`
	PathGlob string `json:"path_glob,omitempty"`
	Query    string `json:"query,omitempty"`
	Path     string `json:"path,omitempty"`
}

type GrepMatch struct {
	File       string `json:"file"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line_content"`
}

type GrepOutput struct {
	Matches   []GrepMatch `json:"matches"`
	Truncated bool        `json:"truncated"`
	Total     int         `json:"total"`
}

type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search file contents. Use query for simple text search, pattern for regex." }
func (t *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Plain text to search for (simple, no regex)"},
			"pattern": {"type": "string", "description": "Regex pattern to search for (advanced)"},
			"path": {"type": "string", "description": "Directory to search in (defaults to workspace root)"},
			"path_glob": {"type": "string", "description": "Glob pattern to filter files (e.g. **/*.go)"}
		},
		"required": []
	}`)
}

func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in GrepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	// query is a convenience alias for pattern (auto-escape regex)
	if in.Query != "" && in.Pattern == "" {
		in.Pattern = regexp.QuoteMeta(in.Query)
	}
	if in.Pattern == "" {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "query or pattern is required"}
	}

	// If path is set, prepend it to path_glob
	if in.Path != "" && in.PathGlob == "" {
		in.PathGlob = in.Path + "/**"
	}

	// Determine search root
	searchRoot := ws.Root
	if in.Path != "" {
		resolved, err := ws.ResolvePath(in.Path)
		if err != nil {
			return ToolResult{}, err
		}
		searchRoot = resolved
	}

	// Try rg (ripgrep) first — it's faster and handles binary/encoding automatically
	if rgPath, err := rgLookPath("rg"); err == nil {
		return t.execRg(ctx, rgPath, in, searchRoot, ws)
	}

	// Fallback: native Go regex walk
	return t.execNative(ctx, in, searchRoot, ws)
}

func (t *GrepTool) execRg(ctx context.Context, rgPath string, in GrepInput, searchRoot string, ws *Workspace) (ToolResult, error) {
	args := []string{
		"--no-heading",
		"--line-number",
		"--color=never",
		"--max-count=" + strconv.Itoa(grepMaxMatches),
		"-n",
		in.Pattern,
	}

	if in.PathGlob != "" {
		args = append(args, "--glob", in.PathGlob)
	}

	args = append(args, searchRoot)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// rg returns exit code 1 when no matches found — that's not an error
	if err != nil && err.Error() != "exit status 1" {
		return ToolResult{}, &ToolError{Code: "search_error", Message: fmt.Sprintf("rg failed: %v — %s", err, stderr.String())}
	}

	var matches []GrepMatch
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		// rg output format: filepath:linenum:content
		// filepath can contain colons on Windows (e.g., C:\path), so split from the right
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		relPath := parts[0]
		lineNum, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil {
			continue
		}
		content := parts[2]

		// Make path relative to workspace root
		if strings.HasPrefix(relPath, ws.Root) {
			relPath = relPath[len(ws.Root)+1:]
		}

		matches = append(matches, GrepMatch{
			File:       relPath,
			LineNumber: lineNum,
			Line:       content,
		})
	}

	truncated := len(matches) >= grepMaxMatches
	if matches == nil {
		matches = []GrepMatch{}
	}

	out := GrepOutput{
		Matches:   matches,
		Truncated: truncated,
		Total:     len(matches),
	}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}

func (t *GrepTool) execNative(ctx context.Context, in GrepInput, searchRoot string, ws *Workspace) (ToolResult, error) {
	// Native fallback using filepath.WalkDir + regexp
	return t.execNativeImpl(ctx, in, searchRoot, ws)
}
