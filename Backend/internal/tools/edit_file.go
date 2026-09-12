package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type EditFileInput struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type EditFileOutput struct {
	LinesAdded   int    `json:"linesAdded"`
	LinesRemoved int    `json:"linesRemoved"`
	NewContent   string `json:"newContent"`
	Diff         string `json:"diff"`
}

type EditFileTool struct{}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Find and replace a string in a file. The old_string must appear exactly once." }
func (t *EditFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path relative to workspace root"},
			"old_string": {"type": "string", "description": "Exact string to find and replace"},
			"new_string": {"type": "string", "description": "Replacement string"}
		},
		"required": ["path", "old_string", "new_string"]
	}`)
}

func (t *EditFileTool) Execute(ctx context.Context, input json.RawMessage, ws *Workspace) (ToolResult, error) {
	var in EditFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	if in.Path == "" {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "path is required"}
	}
	if in.OldString == "" {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "old_string is required"}
	}
	if in.OldString == in.NewString {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: "old_string and new_string are identical"}
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

	// Count occurrences
	count := bytes.Count(data, []byte(in.OldString))
	if count == 0 {
		return ToolResult{}, &ToolError{Code: "edit_not_found", Message: "old_string not found in file"}
	}
	if count > 1 {
		return ToolResult{}, &ToolError{Code: "ambiguous_edit", Message: fmt.Sprintf("old_string found %d times in file; must be unique", count)}
	}

	// Compute diff stats from actual before/after
	beforeLines := strings.Split(string(data), "\n")
	newData := bytes.Replace(data, []byte(in.OldString), []byte(in.NewString), 1)
	afterLines := strings.Split(string(newData), "\n")

	linesAdded := len(afterLines) - len(beforeLines)
	if linesAdded < 0 {
		linesAdded = 0
	}
	linesRemoved := len(beforeLines) - len(afterLines)
	if linesRemoved < 0 {
		linesRemoved = 0
	}

	// Build a simple unified diff
	diff := computeUnifiedDiff(in.Path, beforeLines, afterLines)

	// Write the file
	if err := os.WriteFile(resolved, newData, 0o644); err != nil {
		return ToolResult{}, &ToolError{Code: "write_error", Message: fmt.Sprintf("cannot write file: %v", err)}
	}

	out := EditFileOutput{
		LinesAdded:   linesAdded,
		LinesRemoved: linesRemoved,
		NewContent:   string(newData),
		Diff:         diff,
	}
	resultBytes, _ := json.Marshal(out)
	return ToolResult{Output: resultBytes}, nil
}

func computeUnifiedDiff(path string, before, after []string) string {
	// Simple diff: find changed region
	maxLen := len(before)
	if len(after) > maxLen {
		maxLen = len(after)
	}

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- a/%s\n", path))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", path))

	// Find first difference
	changeStart := 0
	for changeStart < len(before) && changeStart < len(after) && before[changeStart] == after[changeStart] {
		changeStart++
	}

	// Find last difference
	changeEndBefore := len(before) - 1
	changeEndAfter := len(after) - 1
	for changeEndBefore > changeStart && changeEndAfter > changeStart &&
		before[changeEndBefore] == after[changeEndAfter] {
		changeEndBefore--
		changeEndAfter--
	}

	if changeStart > 0 {
		contextStart := changeStart - 1
		if contextStart < 0 {
			contextStart = 0
		}
		diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			contextStart+1, changeEndBefore-contextStart+1,
			contextStart+1, changeEndAfter-contextStart+1))

		// Context before
		for i := contextStart; i < changeStart; i++ {
			diff.WriteString(fmt.Sprintf(" %s\n", before[i]))
		}
	} else {
		diff.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n",
			changeEndBefore+1, changeEndAfter+1))
	}

	// Removed lines
	for i := changeStart; i <= changeEndBefore; i++ {
		diff.WriteString(fmt.Sprintf("-%s\n", before[i]))
	}

	// Added lines
	for i := changeStart; i <= changeEndAfter; i++ {
		diff.WriteString(fmt.Sprintf("+%s\n", after[i]))
	}

	// Context after
	if changeEndBefore < len(before)-1 || changeEndAfter < len(after)-1 {
		contextEndBefore := changeEndBefore + 1
		contextEndAfter := changeEndAfter + 1
		endBefore := len(before)
		endAfter := len(after)
		if endBefore-contextEndBefore > 3 {
			endBefore = contextEndBefore + 3
		}
		if endAfter-contextEndAfter > 3 {
			endAfter = contextEndAfter + 3
		}
		for i := contextEndBefore; i < endBefore && i < len(before); i++ {
			diff.WriteString(fmt.Sprintf(" %s\n", before[i]))
		}
	}

	return diff.String()
}
