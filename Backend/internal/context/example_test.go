package context

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

// TestRenderExampleSystemPrompt builds a realistic fixture and logs the full
// assembled SystemPrompt. This is a manual-inspection aid (the concrete output
// is copied into the step report, not asserted here beyond coherence).
func TestRenderExampleSystemPrompt(t *testing.T) {
	dir := t.TempDir()

	// .nightcode
	writeFile(t, filepath.Join(dir, ".nightcode", "instructions.md"),
		"# NightCode Agent Instructions\n\nYou are a coding agent.\n- Prefer small, focused edits.\n- Run tests before claiming success.\n")

	skills := filepath.Join(dir, ".nightcode", "skills")
	writeFile(t, filepath.Join(skills, "deploy.md"), "# deploy\n\n## Deploy the app to production\n")
	writeFile(t, filepath.Join(skills, "review.md"), "---\nname: reviewer\ndescription: Review code for security issues\n---\n")

	// source tree
	writeFile(t, filepath.Join(dir, "README.md"), "# Demo Project\n")
	writeFile(t, filepath.Join(dir, "src", "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "src", "util", "fmt.go"), "package util\n")
	writeFile(t, filepath.Join(dir, "docs", "guide.md"), "# Guide\n")

	// git
	gitInit(t, dir)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")
	writeFile(t, filepath.Join(dir, "src", "main.go"), "package main\n\nfunc main() { println(\"changed\") }\n")

	ws, err := tools.NewWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}

	bc, err := Build(context.Background(), BuildRequest{
		Workspace: ws,
		ChatID:    "chat-1",
		History: []Message{
			{Role: "user", Content: "Add a hello endpoint"},
			{Role: "assistant", Content: "Done."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("=== WARNINGS ===\n%v\n", bc.Warnings)
	t.Logf("=== SYSTEM PROMPT ===\n%s\n", bc.SystemPrompt)
	t.Logf("=== MESSAGES ===\n%+v\n", bc.Messages)
}