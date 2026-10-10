package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
			relPath, relErr := filepath.Rel(searchRoot, path)
			if relErr != nil {
				relPath = path
			}
			relPathSlash := filepath.ToSlash(relPath)
			matched, matchErr := doublestar.Match(in.PathGlob, relPathSlash)
			if matchErr != nil {
				log.Printf("grep native: glob match error for %s: %v", relPathSlash, matchErr)
			}
			if !matched {
				matched, matchErr = doublestar.Match(in.PathGlob, filepath.Base(path))
				if matchErr != nil {
					log.Printf("grep native: glob match error for %s: %v", filepath.Base(path), matchErr)
				}
				if !matched {
					return nil
				}
			}
		}

		// Open once: peek the first 512 bytes for a NUL byte (binary
		// heuristic), then reuse the same handle for line scanning — avoids
		// opening every file twice.
		f, err := os.Open(path)
		if err != nil {
			return nil
		}

		peek := make([]byte, 512)
		n, readErr := f.Read(peek)
		if readErr != nil && readErr != io.EOF {
			f.Close()
			return nil
		}
		isBinary := false
		for i := 0; i < n; i++ {
			if peek[i] == 0 {
				isBinary = true
				break
			}
		}
		if isBinary {
			f.Close()
			return nil
		}

		// Rewind to the start for the scanner.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return nil
		}

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
					f.Close()
					return filepath.SkipAll
				}
			}
		}
		f.Close()
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
