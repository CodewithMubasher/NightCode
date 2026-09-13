package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

// Message is a single conversation message ready to hand to a Provider.
// The agent loop (Step 4) is responsible for converting stored message rows
// (with segments) into this minimal role/content shape.
type Message struct {
	Role    string // "system" | "user" | "assistant" | "tool"
	Content string
}

// BuildRequest is the input to Build.
type BuildRequest struct {
	Workspace *tools.Workspace
	ChatID    string
	History   []Message // prior turns in this chat, already loaded from the store
}

// BuiltContext is the fully-assembled context for one turn.
type BuiltContext struct {
	SystemPrompt string    // instructions + skills + file-tree + git, assembled
	Messages     []Message // history + current turn, ready to hand to a Provider
	Warnings     []string  // non-fatal issues surfaced (e.g. "instructions.md not found")
}

// baseSystemPrompt is always included, ahead of any workspace-specific
// .nightcode/instructions.md content. It defines core agent behavior that
// should hold regardless of workspace.
const baseSystemPrompt = `When you're about to use a tool, it's often helpful to say a brief, natural sentence first — but only when it actually adds something. Skip it for small, obvious, or rapid-fire steps (e.g. reading a file you clearly need, or running a couple of related commands back to back). When you do say something, sound like a person thinking out loud, not a status log:
- "Let me check what's already in this file."
- "Found it — looks like the bug is in the import path."
- "Two files need updating for this."
- "Let me run the tests to confirm."

Avoid mechanical, repetitive phrasing like "I am going to..." or "I will now..." before every single action — that gets tedious fast and reads like a script, not a person. Vary your phrasing, and stay quiet when there's nothing worth narrating. It's fine to run several related tool calls in a row without a sentence between each one.

When using the shell tool on Windows, write commands in PowerShell syntax (Get-ChildItem, Remove-Item, Copy-Item, $env:VAR, Where-Object, etc.) — not bash/sh syntax. The shell tool runs real PowerShell on Windows, so bash-isms like ls, rm -rf, cat, or && between commands will fail or behave unexpectedly.`

// Build assembles the complete model context for a turn. It is a pure function
// of the workspace + history: no LLM, no agent loop.
func Build(ctx context.Context, req BuildRequest) (BuiltContext, error) {
	if req.Workspace == nil {
		return BuiltContext{}, fmt.Errorf("context: workspace is nil")
	}

	var bc BuiltContext
	var warnings []string

	// 1. Instructions (.nightcode/instructions.md)
	instructions, warn := buildInstructions(req.Workspace)
	warnings = append(warnings, warn...)

	prompt := baseSystemPrompt
	if instructions != "" {
		prompt += "\n\n" + instructions
	}

	// 2. Skills manifest (.nightcode/skills/*.md)
	skills, skillWarn := buildSkillManifest(req.Workspace)
	warnings = append(warnings, skillWarn...)

	// 3. File-tree summary
	tree, treeWarn := buildFileTree(req.Workspace)
	warnings = append(warnings, treeWarn...)

	// 4. Git context
	git := buildGitContext(ctx, req.Workspace)

	// Assemble system prompt in order
	var sb strings.Builder
	sb.WriteString(prompt)

	if skills != "" {
		sb.WriteString("\n\n## Available Skills\n")
		sb.WriteString(skills)
	}

	if tree != "" {
		sb.WriteString("\n\n## Workspace Structure\n")
		sb.WriteString(tree)
	}

	if git != "" {
		sb.WriteString("\n\n## Git Status\n")
		sb.WriteString(git)
	}

	bc.SystemPrompt = sb.String()
	bc.Warnings = warnings

	// 5. History passthrough (order and roles preserved verbatim)
	bc.Messages = append([]Message(nil), req.History...)

	return bc, nil
}