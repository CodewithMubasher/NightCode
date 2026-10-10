package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const grepMaxMatches = 200

// splitRgLine parses ripgrep's "path:line:content" output line.
// The line-number field is the first purely-numeric field at index >= 1,
// which correctly handles Windows drive-letter colons (C:\path\file.go:12:content)
// and content that itself contains colons. Returns ok=false when no valid
// line-number field is found.
func splitRgLine(line string) (path, lineNum, content string, ok bool) {
	parts := strings.Split(line, ":")
	// Need at least path:line:content (3 fields), or path:line (2 fields).
	if len(parts) < 2 {
		return "", "", "", false
	}
	// Find the line-number field: first numeric field starting at index 1
	// (index 0 may be a bare Windows drive letter like "C").
	lineIdx := -1
	for i := 1; i < len(parts)-1; i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil && n > 0 {
			lineIdx = i
			break
		}
	}
	if lineIdx == -1 {
		return "", "", "", false
	}
	path = strings.Join(parts[:lineIdx], ":")
	lineNum = parts[lineIdx]
	content = strings.Join(parts[lineIdx+1:], ":")
	return path, lineNum, content, true
}

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
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return ToolResult{}, &ToolError{Code: "search_error", Message: fmt.Sprintf("rg failed: %v — %s", err, stderr.String())}
		}
	}

	var matches []GrepMatch
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		// rg output format: filepath:linenum:content
		// On Windows the filepath contains a drive-letter colon (C:\...),
		// and the content may contain colons too, so identify the numeric
		// line-number field instead of splitting naively from the left.
		relPath, lineNumStr, content, ok := splitRgLine(line)
		if !ok {
			continue
		}
		lineNum, parseErr := strconv.Atoi(lineNumStr)
		if parseErr != nil {
			continue
		}

		// Make path relative to workspace root (case-insensitive on Windows)
		rootSlash := filepath.ToSlash(ws.Root)
		pathSlash := filepath.ToSlash(relPath)
		if len(pathSlash) > len(rootSlash) && strings.EqualFold(pathSlash[:len(rootSlash)], rootSlash) {
			relPath = filepath.FromSlash(pathSlash[len(rootSlash)+1:])
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
