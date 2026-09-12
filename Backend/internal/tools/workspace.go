package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace represents a sandboxed directory for tool operations.
type Workspace struct {
	Root string // absolute, symlink-resolved path to the workspace root
}

// NewWorkspace creates a Workspace after validating the root path exists and is a directory.
func NewWorkspace(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		resolved = filepath.Clean(abs)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat workspace root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace root is not a directory: %s", resolved)
	}
	return &Workspace{Root: resolved}, nil
}

// ResolvePath canonicalizes userPath relative to ws.Root and verifies the result
// is within the workspace boundary. For existing files/dirs, resolves symlinks.
// For non-existent paths (e.g., write_file creating new files), validates the
// directory structure up to the first existing ancestor.
func (ws *Workspace) ResolvePath(userPath string) (string, error) {
	if userPath == "" {
		return "", &SandboxError{Code: "sandbox_violation", Msg: "empty path"}
	}
	if strings.ContainsRune(userPath, 0) {
		return "", &SandboxError{Code: "sandbox_violation", Msg: "path contains null byte"}
	}

	// Join against root and clean to resolve . and ..
	// If userPath is absolute, filepath.Join discards previous elements,
	// so we must reject absolute paths to prevent sandbox escape.
	if filepath.IsAbs(userPath) {
		return "", &SandboxError{
			Code: "sandbox_violation",
			Msg:  fmt.Sprintf("absolute path %q is not allowed", userPath),
		}
	}
	joined := filepath.Join(ws.Root, userPath)
	cleaned := filepath.Clean(joined)

	// Verify the cleaned path is within root BEFORE resolving symlinks.
	// This catches traversal attacks even if symlinks don't exist.
	if cleaned == ws.Root {
		return cleaned, nil
	}
	if !strings.HasPrefix(cleaned, ws.Root+string(filepath.Separator)) {
		return "", &SandboxError{
			Code: "sandbox_violation",
			Msg:  fmt.Sprintf("path %q resolves outside workspace root %q", userPath, ws.Root),
		}
	}

	// Try EvalSymlinks on the full path (handles existing files with symlinks)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err == nil {
		// Re-verify after symlink resolution (symlink could point outside root)
		if resolved == ws.Root {
			return resolved, nil
		}
		if !strings.HasPrefix(resolved, ws.Root+string(filepath.Separator)) {
			return "", &SandboxError{
				Code: "sandbox_violation",
				Msg:  fmt.Sprintf("symlink in %q escapes workspace root", userPath),
			}
		}
		return resolved, nil
	}

	// Path doesn't exist yet (e.g., write_file). Walk up to find the deepest
	// existing ancestor, resolve that, then append remaining segments.
	dir := cleaned
	var tail []string
	for {
		base := filepath.Base(dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		// Add current segment to tail BEFORE checking parent
		tail = append(tail, base)
		if info, statErr := os.Stat(parent); statErr == nil && info.IsDir() {
			parentResolved, parentErr := filepath.EvalSymlinks(parent)
			if parentErr != nil {
				parentResolved = parent
			}
			// Build resolved path: parentResolved + reversed tail segments
			segments := make([]string, 0, 1+len(tail))
			segments = append(segments, parentResolved)
			for i := len(tail) - 1; i >= 0; i-- {
				segments = append(segments, tail[i])
			}
			return filepath.Join(segments...), nil
		}
		dir = parent
	}

	// Fallback: return cleaned path (validation already passed above)
	return cleaned, nil
}

// SandboxError is a typed error for sandbox boundary violations.
type SandboxError struct {
	Code string
	Msg  string
}

func (e *SandboxError) Error() string {
	return e.Msg
}
