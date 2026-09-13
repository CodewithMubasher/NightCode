package context

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

func newWS(t *testing.T, dir string) *tools.Workspace {
	t.Helper()
	ws, err := tools.NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	return ws
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Scenario 1: instructions present and populated — assert verbatim in SystemPrompt.
func TestBuild_InstructionsPresent(t *testing.T) {
	dir := t.TempDir()
	ins := "# Project Rules\n\n- Use tabs, not spaces.\n- Do not delete tests.\n"
	writeFile(t, filepath.Join(dir, ".nightcode", "instructions.md"), ins)

	bc, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, dir)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(bc.SystemPrompt, ins) {
		t.Errorf("SystemPrompt missing instructions verbatim.\nGot prompt:\n%s", bc.SystemPrompt)
	}
	// No "instructions not found" warning when present
	for _, w := range bc.Warnings {
		if strings.Contains(w, "instructions.md not found") {
			t.Errorf("unexpected missing-instructions warning: %s", w)
		}
	}
}

// Scenario 2: no .nightcode at all — Build succeeds, warning notes no instructions,
// prompt still coherent (not empty/broken).
func TestBuild_NoNightcodeDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")

	bc, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, dir)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found := false
	for _, w := range bc.Warnings {
		if strings.Contains(w, "instructions.md not found") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'instructions.md not found' warning, got: %v", bc.Warnings)
	}

	// Prompt must still have the file-tree section referencing main.go
	if !strings.Contains(bc.SystemPrompt, "main.go") {
		t.Errorf("SystemPrompt lacks file-tree (missing main.go):\n%s", bc.SystemPrompt)
	}
	if strings.TrimSpace(bc.SystemPrompt) == "" {
		t.Errorf("SystemPrompt is empty — should still contain workspace structure")
	}
}

// Scenario 3: multiple skills — every file represented, manifest content matches
// actual file content (not templated/guessed).
func TestBuild_SkillsManifest(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, ".nightcode", "skills")
	a := "# deploy\n\n## Deploy the app to production\n\nBody A\n"
	b := "---\nname: reviewer\ndescription: Review code for security issues\n---\n\nBody B\n"
	c := "## Run tests\n\nBody C\n"
	writeFile(t, filepath.Join(skillsDir, "deploy.md"), a)
	writeFile(t, filepath.Join(skillsDir, "reviewer.md"), b)
	writeFile(t, filepath.Join(skillsDir, "test.md"), c)
	// A non-.md file must be ignored
	writeFile(t, filepath.Join(skillsDir, "README.txt"), "ignored")

	bc, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, dir)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, want := range []string{"deploy", "reviewer", "test"} {
		if !strings.Contains(bc.SystemPrompt, "**"+want+"**") {
			t.Errorf("skills manifest missing %s:\n%s", want, bc.SystemPrompt)
		}
	}
	if strings.Contains(bc.SystemPrompt, "README.txt") {
		t.Errorf("non-.md file leaked into skills manifest")
	}

	// Frontmatter name must override filename (reviewer.md -> name "reviewer" via frontmatter)
	if !strings.Contains(bc.SystemPrompt, "reviewer") {
		t.Errorf("expected frontmatter name 'reviewer'")
	}
	// Frontmatter description from file b must appear verbatim
	if !strings.Contains(bc.SystemPrompt, "Review code for security issues") {
		t.Errorf("frontmatter description not extracted verbatim:\n%s", bc.SystemPrompt)
	}
	// Heading-derived description from file c
	if !strings.Contains(bc.SystemPrompt, "Run tests") {
		t.Errorf("heading description not extracted:\n%s", bc.SystemPrompt)
	}
}

// Scenario 4: git repo with changes vs non-git-repo.
func TestBuild_GitContext(t *testing.T) {
	// --- git repo with uncommitted change
	repo := t.TempDir()
	gitInit(t, repo)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "v1\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "init")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "v2\n") // now modified

	bc, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, repo)})
	if err != nil {
		t.Fatalf("Build git repo: %v", err)
	}
	if !strings.Contains(bc.SystemPrompt, "On branch") {
		t.Errorf("git context missing branch line:\n%s", bc.SystemPrompt)
	}
	if !strings.Contains(bc.SystemPrompt, "tracked.txt") {
		t.Errorf("git context missing modified file:\n%s", bc.SystemPrompt)
	}
	if !strings.Contains(bc.SystemPrompt, "file(s) changed") {
		t.Errorf("git context missing change-count:\n%s", bc.SystemPrompt)
	}

	// --- non-git repo: section cleanly absent, no error/warning
	notRepo := t.TempDir()
	writeFile(t, filepath.Join(notRepo, "a.txt"), "x\n")
	bc2, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, notRepo)})
	if err != nil {
		t.Fatalf("Build non-git: %v", err)
	}
	if strings.Contains(bc2.SystemPrompt, "On branch") || strings.Contains(bc2.SystemPrompt, "Git Status") {
		t.Errorf("git section should be absent for non-repo:\n%s", bc2.SystemPrompt)
	}
	for _, w := range bc2.Warnings {
		if strings.Contains(strings.ToLower(w), "git") {
			t.Errorf("unexpected git warning for non-repo: %s", w)
		}
	}
}

// Scenario 5: deep/large tree exceeding cap — truncation triggers and is noted.
func TestBuild_FileTreeTruncation(t *testing.T) {
	dir := t.TempDir()
	// Create > fileTreeMaxEntries (200) files at one level
	for i := 0; i < 250; i++ {
		writeFile(t, filepath.Join(dir, "f", "file"+pad(i)+".txt"), "x\n")
	}

	bc, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, dir)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(bc.SystemPrompt, "truncated at 200 entries") {
		t.Errorf("expected truncation note, got:\n%s", bc.SystemPrompt)
	}
}

func pad(i int) string {
	s := intToStr(i)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

// Scenario 6: history passthrough — order and role attribution preserved.
func TestBuild_HistoryPassthrough(t *testing.T) {
	dir := t.TempDir()
	history := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "second question"},
	}

	bc, err := Build(context.Background(), BuildRequest{Workspace: newWS(t, dir), History: history})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(bc.Messages) != len(history) {
		t.Fatalf("history length changed: got %d want %d", len(bc.Messages), len(history))
	}
	for i := range history {
		if bc.Messages[i].Role != history[i].Role || bc.Messages[i].Content != history[i].Content {
			t.Errorf("history order/role mismatch at %d: got {%s,%q} want {%s,%q}",
				i, bc.Messages[i].Role, bc.Messages[i].Content, history[i].Role, history[i].Content)
		}
	}
}

// --- helpers ---

func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}