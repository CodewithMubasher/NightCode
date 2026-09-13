package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupGrepFixture creates a small tree of text files for grep testing.
func setupGrepFixture(t *testing.T) *Workspace {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src", "pkg"), 0o755)
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)

	files := map[string]string{
		"src/main.go":      "package main\n\nfunc main() {\n\tfmt.Println(\"TODO: wire up\")\n}\n",
		"src/pkg/utils.go": "package pkg\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n",
		"docs/notes.md":    "# Notes\n\nTODO: write docs\n\nfunc is not code here\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func runGrep(t *testing.T, ws *Workspace, in GrepInput) GrepOutput {
	t.Helper()
	tool := &GrepTool{}
	raw, _ := json.Marshal(in)
	result, err := tool.Execute(context.Background(), raw, ws)
	if err != nil {
		t.Fatalf("grep error: %v", err)
	}
	var out GrepOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("unmarshal grep output: %v", err)
	}
	return out
}

// TestGrepNativeFallbackMatchesRg forces the native fallback (by overriding
// rgLookPath to always fail) and compares results against the rg-backed path
// for the same queries. This proves the untested fallback produces equivalent
// output to ripgrep.
func TestGrepNativeFallbackMatchesRg(t *testing.T) {
	ws := setupGrepFixture(t)

	queries := []GrepInput{
		{Pattern: "func "},
		{Pattern: "TODO"},
		{Pattern: "func ", PathGlob: "**/*.go"},
		{Pattern: "Add"},
	}

	// Save the real rg lookup
	origLookPath := rgLookPath
	t.Cleanup(func() { rgLookPath = origLookPath })

	for _, q := range queries {
		// rg-backed result
		rgLookPath = origLookPath
		rgOut := runGrep(t, ws, q)

		// native fallback result
		rgLookPath = func(string) (string, error) {
			return "", os.ErrNotExist
		}
		nativeOut := runGrep(t, ws, q)

		if rgOut.Total != nativeOut.Total {
			t.Errorf("query %+v: match count differs: rg=%d native=%d", q, rgOut.Total, nativeOut.Total)
			continue
		}
		if rgOut.Truncated != nativeOut.Truncated {
			t.Errorf("query %+v: truncated differs: rg=%v native=%v", q, rgOut.Truncated, nativeOut.Truncated)
		}

		// Compare matches exactly (order should match too since both sort by file walk order)
		for i := range rgOut.Matches {
			rg := rgOut.Matches[i]
			nt := nativeOut.Matches[i]
			if rg.File != nt.File || rg.LineNumber != nt.LineNumber {
				t.Errorf("query %+v: match[%d] differs: rg={%s:%d} native={%s:%d}",
					q, i, rg.File, rg.LineNumber, nt.File, nt.LineNumber)
			}
		}
	}
}

// TestGrepNativeFallbackExplicitlyForced verifies the fallback is genuinely
// exercised (not just that both empty) by checking a known match is found.
func TestGrepNativeFallbackExplicitlyForced(t *testing.T) {
	ws := setupGrepFixture(t)

	origLookPath := rgLookPath
	t.Cleanup(func() { rgLookPath = origLookPath })
	rgLookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}

	out := runGrep(t, ws, GrepInput{Pattern: "func "})
	// matches across src/main.go, src/pkg/utils.go, and docs/notes.md
	if out.Total != 3 {
		t.Fatalf("expected 3 matches in native fallback, got %d", out.Total)
	}

	found := map[string]bool{}
	for _, m := range out.Matches {
		found[m.File] = true
	}
	want := []string{"src/main.go", "src/pkg/utils.go", "docs/notes.md"}
	for _, w := range want {
		if !found[filepath.FromSlash(w)] {
			t.Errorf("native fallback missing expected file %s: %+v", w, out.Matches)
		}
	}
}