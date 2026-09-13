package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func (t *GrepTool) execNativeImpl(ctx context.Context, in GrepInput, searchRoot string, ws *Workspace) (ToolResult, error) {
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return ToolResult{}, &ToolError{Code: "invalid_input", Message: fmt.Sprintf("invalid regex: %v", err)}
	}

	var matches []GrepMatch
	truncated := false

	err = filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != searchRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Apply glob filter
		if in.PathGlob != "" {
			relPath, _ := filepath.Rel(searchRoot, path)
			relPathSlash := filepath.ToSlash(relPath)
			matched, _ := doublestar.Match(in.PathGlob, relPathSlash)
			if !matched {
				matched, _ = doublestar.Match(in.PathGlob, filepath.Base(path))
				if !matched {
					return nil
				}
			}
		}

		// Skip binary files
		if isBinaryFile(path) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				relPath, _ := filepath.Rel(searchRoot, path)
				matches = append(matches, GrepMatch{
					File:       relPath,
					LineNumber: lineNum,
					Line:       strings.TrimRight(line, "\r"),
				})
				if len(matches) >= grepMaxMatches {
					truncated = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil {
		return ToolResult{}, &ToolError{Code: "search_error", Message: fmt.Sprintf("walk error: %v", err)}
	}

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

func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}
