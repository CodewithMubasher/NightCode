package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

const (
	fileTreeMaxDepth  = 3
	fileTreeMaxEntries = 200
)

// excludedDirs are directory names skipped during the file-tree walk.
var excludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".nightcode":   true,
}

// buildFileTree produces a depth-limited directory summary. It is genuinely
// bounded: max depth 3, max entries 200 (with a truncation note past that).
// Notes on simplicity: hidden dotfiles/dirs (other than the explicit excludes)
// are shown, and .gitignore is NOT parsed this step (kept simple).
func buildFileTree(ws *tools.Workspace) (string, []string) {
	var warnings []string
	var entries []string
	truncated := false

	var walk func(path string, depth int)
	walk = func(path string, depth int) {
		if truncated {
			return
		}
		rel, _ := filepath.Rel(ws.Root, path)
		if rel == "." {
			rel = ""
		}

		dirEntries, err := os.ReadDir(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("file-tree: cannot read %s: %v", rel, err))
			return
		}

		// Stable, predictable ordering: dirs first, then files, each alphabetical.
		sort.Slice(dirEntries, func(i, j int) bool {
			if dirEntries[i].IsDir() != dirEntries[j].IsDir() {
				return dirEntries[i].IsDir()
			}
			return dirEntries[i].Name() < dirEntries[j].Name()
		})

		for _, de := range dirEntries {
			if truncated {
				return
			}
			name := de.Name()
			if de.IsDir() && excludedDirs[name] {
				continue
			}
			childRel := name
			if rel != "" {
				childRel = filepath.ToSlash(filepath.Join(rel, name))
			}
			indent := strings.Repeat("  ", depth)
			suffix := ""
			if de.IsDir() {
				suffix = "/"
			}
			entries = append(entries, fmt.Sprintf("%s%s%s", indent, childRel, suffix))
			if len(entries) >= fileTreeMaxEntries {
				truncated = true
				return
			}
			if de.IsDir() && depth+1 < fileTreeMaxDepth {
				walk(filepath.Join(path, name), depth+1)
			}
		}
	}

	walk(ws.Root, 0)

	if len(entries) == 0 && !truncated {
		// Empty workspace (or only excluded dirs)
		return "(empty workspace)", warnings
	}

	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e)
		sb.WriteString("\n")
	}
	if truncated {
		sb.WriteString(fmt.Sprintf("... (truncated at %d entries)\n", fileTreeMaxEntries))
	}

	return strings.TrimRight(sb.String(), "\n"), warnings
}