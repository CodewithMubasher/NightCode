package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func setupTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	dir := t.TempDir()
	// Create a symlink that points outside the workspace
	outsideDir := filepath.Join(t.TempDir(), "escaped")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(dir, "sneaky_link")
	if runtime.GOOS == "windows" {
		// On Windows, symlinks for directories require elevated privileges.
		// Use a junction or skip symlink tests.
		t.Skip("skipping symlink test on Windows (requires elevated privileges)")
	}
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatal(err)
	}
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestResolvePath_Valid(t *testing.T) {
	dir := t.TempDir()
	// Create nested structure
	os.MkdirAll(filepath.Join(dir, "src", "pkg"), 0o755)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644)

	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"relative simple", "src/main.go", filepath.Join(ws.Root, "src", "main.go")},
		{"relative nested", "src/pkg/foo.go", filepath.Join(ws.Root, "src", "pkg", "foo.go")},
		{"dot current", "./src/main.go", filepath.Join(ws.Root, "src", "main.go")},
		{"dotdot within", "src/../src/main.go", filepath.Join(ws.Root, "src", "main.go")},
		{"workspace root", ".", ws.Root},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ws.ResolvePath(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePath_SandboxViolation(t *testing.T) {
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"relative traversal double dotdot", "../../etc/passwd"},
		{"triple dotdot", "../../../etc/passwd"},
		{"dotdot then deeper traversal", "src/../../etc/passwd"},
		{"mixed valid and traversal", "src/../../etc/shadow"},
		{"trailing dotdot", "foo/../../etc/passwd"},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests,
			struct{ name, path string }{"windows absolute C:", "C:\\Windows\\System32\\config\\sam"},
			struct{ name, path string }{"windows UNC path", "\\\\server\\share\\file"},
			struct{ name, path string }{"windows forward slash absolute", "C:/Windows/System32/config/sam"},
		)
	} else {
		tests = append(tests,
			struct{ name, path string }{"unix absolute path outside root", "/etc/passwd"},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ws.ResolvePath(tt.path)
			if err == nil {
				t.Fatal("expected sandbox violation error, got nil")
			}
			var sbErr *SandboxError
			if stdErr, ok := err.(*SandboxError); ok {
				sbErr = stdErr
			}
			if sbErr == nil {
				t.Errorf("expected SandboxError, got %T: %v", err, err)
			}
		})
	}
}

func TestResolvePath_EmptyAndNull(t *testing.T) {
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ws.ResolvePath("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}

	_, err = ws.ResolvePath("foo\x00bar")
	if err == nil {
		t.Fatal("expected error for null byte in path")
	}
}

func TestResolvePath_SymlinkEscape(t *testing.T) {
	ws := setupTestWorkspace(t)

	_, err := ws.ResolvePath("sneaky_link/etc/passwd")
	if err == nil {
		t.Fatal("expected sandbox violation for symlink escape")
	}
}

func TestResolvePath_SymlinkWithin(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows")
	}
	subDir := filepath.Join(dir, "real_subdir")
	os.MkdirAll(subDir, 0o755)
	linkPath := filepath.Join(dir, "mylink")
	if err := os.Symlink(subDir, linkPath); err != nil {
		t.Fatal(err)
	}
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ws.ResolvePath("mylink/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(subDir, "file.txt")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestResolvePath_DotDotMixedWithValid(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}

	// a/b/../b/c should resolve to a/b/c
	got, err := ws.ResolvePath("a/b/../b/c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(ws.Root, "a", "b", "c")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
