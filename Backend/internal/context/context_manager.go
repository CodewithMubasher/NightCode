package context

import (
	"context"
	"fmt"
	"strings"
)

const (
	// DefaultContextTokens is the default context window size for models
	// that don't specify one. 128K covers Llama 3.3, Gemini Flash, etc.
	DefaultContextTokens = 128000

	// charsPerToken is a rough heuristic for English code/text. Real tokenizers
	// vary (BPE ~3.5-4.5 chars/token), but this is good enough for budgeting.
	charsPerToken = 4

	// reserveFraction is the fraction of the context window reserved for the
	// model's response. 25% gives the model room to think.
	reserveFraction = 0.25

	// systemBudgetFraction is the max fraction of usable tokens the system
	// prompt can consume before we start trimming it.
	systemBudgetFraction = 0.30

	// compactionThreshold is the minimum number of chars a message must have
	// before we consider truncating it during compaction. 2000 chars (~500
	// tokens) is generous enough to preserve meaningful context while still
	// allowing budget reclamation from very large tool outputs.
	// Cross-reference: loop.go:maxToolOutputBytes (8 KB) truncates tool
	// output at the source, so stored tool results are already ≤ 8 KB.
	compactionThreshold = 2000
)

// ContextBudget defines token limits for a single turn.
type ContextBudget struct {
	TotalTokens int // full context window (e.g. 128000)
}

// usableTokens returns tokens available after reserving space for the response.
func (b ContextBudget) usableTokens() int {
	if b.TotalTokens <= 0 {
		b.TotalTokens = DefaultContextTokens
	}
	return int(float64(b.TotalTokens) * (1 - reserveFraction))
}

// systemTokenLimit returns the max tokens we'll allow the system prompt to use.
func (b ContextBudget) systemTokenLimit() int {
	return int(float64(b.usableTokens()) * systemBudgetFraction)
}

// historyTokenBudget returns tokens available for conversation history.
func (b ContextBudget) historyTokenBudget() int {
	return b.usableTokens() - b.systemTokenLimit()
}

// ContextManager assembles the full model context within a token budget.
// It replaces the raw context.Build() call in the agent loop.
type ContextManager struct {
	budget ContextBudget
}

// NewContextManager creates a ContextManager with the given budget.
// If budget.TotalTokens is 0, DefaultContextTokens is used.
func NewContextManager(budget ContextBudget) *ContextManager {
	if budget.TotalTokens <= 0 {
		budget.TotalTokens = DefaultContextTokens
	}
	return &ContextManager{budget: budget}
}

// ManagedContext is the output of ContextManager.Build.
type ManagedContext struct {
	SystemPrompt string
	Messages     []Message
	Warnings     []string
	Stats        ContextStats
}

// ContextStats reports what happened during context assembly.
type ContextStats struct {
	SystemTokens   int
	HistoryTokens  int
	TotalTokens    int
	MessagesKept   int
	MessagesCompacted int
	BudgetTokens   int
}

// Build assembles the complete model context within the token budget.
// It builds the system prompt, then windows/compacts history to fit.
func (cm *ContextManager) Build(ctx context.Context, req BuildRequest) (ManagedContext, error) {
	if req.Workspace == nil {
		return ManagedContext{}, fmt.Errorf("context: workspace is nil")
	}

	var mc ManagedContext
	var warnings []string

	// 1. Build system prompt (same as context.Build)
	systemPrompt, sysWarns := cm.buildSystemPrompt(ctx, req)
	warnings = append(warnings, sysWarns...)
	mc.SystemPrompt = systemPrompt

	// 2. Window/compact history to fit budget
	compacted, stats := cm.compactHistory(req.History, systemPrompt)
	mc.Messages = compacted
	mc.Stats = stats
	mc.Stats.SystemTokens = estimateTokens(systemPrompt)
	mc.Stats.BudgetTokens = cm.budget.historyTokenBudget()
	mc.Warnings = warnings

	return mc, nil
}

// buildSystemPrompt assembles the system prompt (instructions + skills + file tree + git).
// If the assembled prompt exceeds systemTokenLimit(), it is truncated in priority
// order: file tree first, then skills, then instructions. baseSystemPrompt is
// never truncated. Warnings are emitted for any truncation.
func (cm *ContextManager) buildSystemPrompt(ctx context.Context, req BuildRequest) (string, []string) {
	var warnings []string

	instructions, warn := buildInstructions(req.Workspace)
	warnings = append(warnings, warn...)

	base := baseSystemPrompt
	if instructions != "" {
		base += "\n\n" + instructions
	}

	skills, skillWarn := buildSkillManifest(req.Workspace)
	warnings = append(warnings, skillWarn...)

	tree, treeWarn := buildFileTree(req.Workspace)
	warnings = append(warnings, treeWarn...)

	git := buildGitContext(ctx, req.Workspace)

	// Assemble sections so we can drop them in priority order if over budget.
	type section struct {
		name    string
		content string
	}
	sections := []section{}
	if skills != "" {
		sections = append(sections, section{name: "skills", content: "\n\n## Available Skills\n" + skills})
	}
	if tree != "" {
		sections = append(sections, section{name: "file tree", content: "\n\n## Workspace Structure\n" + tree})
	}
	if git != "" {
		sections = append(sections, section{name: "git status", content: "\n\n## Git Status\n" + git})
	}

	// Build full prompt and check budget.
	sb := strings.Builder{}
	sb.WriteString(base)
	for _, s := range sections {
		sb.WriteString(s.content)
	}
	fullPrompt := sb.String()

	budget := cm.budget.systemTokenLimit()
	promptTokens := estimateTokens(fullPrompt)

	if promptTokens <= budget {
		return fullPrompt, warnings
	}

	// Over budget — drop sections in reverse priority order:
	// 1. file tree (least important), 2. skills, 3. instructions (most important)
	// Never truncate baseSystemPrompt.
	for i := len(sections) - 1; i >= 0; i-- {
		dropped := sections[i]
		// Remove this section and rebuild
		newSections := make([]section, 0, len(sections)-1)
		newSections = append(newSections, sections[:i]...)
		newSections = append(newSections, sections[i+1:]...)

		sb.Reset()
		sb.WriteString(base)
		for _, s := range newSections {
			sb.WriteString(s.content)
		}
		candidate := sb.String()
		candidateTokens := estimateTokens(candidate)

		warnings = append(warnings, fmt.Sprintf("system prompt over budget (%d/%d tokens), dropped %s section",
			promptTokens, budget, dropped.name))
		sections = newSections
		fullPrompt = candidate
		promptTokens = candidateTokens

		if promptTokens <= budget {
			return fullPrompt, warnings
		}
	}

	// Still over budget even after dropping all optional sections.
	// The base prompt + instructions must be kept; emit a warning.
	if promptTokens > budget {
		warnings = append(warnings, fmt.Sprintf("system prompt still over budget (%d/%d tokens) after dropping optional sections; base prompt was not truncated",
			promptTokens, budget))
	}

	return fullPrompt, warnings
}

// compactHistory windows and compacts history to fit the token budget.
// Strategy:
//  1. Estimate tokens for each message.
//  2. If total fits in budget, return as-is.
//  3. Otherwise, keep recent messages in full, compact oldest first.
//  4. Compaction = truncate content, preserve metadata (role, tool calls).
func (cm *ContextManager) compactHistory(history []Message, systemPrompt string) ([]Message, ContextStats) {
	if len(history) == 0 {
		return nil, ContextStats{}
	}

	// Estimate tokens for each message
	type msgTokens struct {
		msg    Message
		tokens int
	}
	perMsg := make([]msgTokens, len(history))
	totalHistoryTokens := 0
	for i, m := range history {
		t := estimateMessageTokens(m)
		perMsg[i] = msgTokens{msg: m, tokens: t}
		totalHistoryTokens += t
	}

	budget := cm.budget.historyTokenBudget()

	// If within budget, return as-is
	if totalHistoryTokens <= budget {
		result := make([]Message, len(history))
		for i, m := range history {
			result[i] = m
		}
		return result, ContextStats{
			HistoryTokens:    totalHistoryTokens,
			MessagesKept:     len(history),
			MessagesCompacted: 0,
		}
	}

	// Over budget — compact from oldest to newest, purely budget-driven.
	// Keep the last message in full for continuity.
	result := make([]Message, 0, len(history))
	compactedCount := 0
	runningTokens := 0

	for i, pm := range perMsg {
		isLast := i == len(history)-1

		if isLast || runningTokens+pm.tokens <= budget {
			// Keep in full
			result = append(result, pm.msg)
			runningTokens += pm.tokens
		} else {
			// Compact this message
			compacted := compactMessage(pm.msg)
			compactedTokens := estimateMessageTokens(compacted)
			result = append(result, compacted)
			runningTokens += compactedTokens
			compactedCount++
		}
	}

	return result, ContextStats{
		HistoryTokens:     runningTokens,
		MessagesKept:      len(history) - compactedCount,
		MessagesCompacted: compactedCount,
	}
}

// compactMessage truncates a message's content while preserving metadata.
// Truncation limit is 2000 chars to preserve meaningful context. Tool outputs
// are already capped at 8 KB by loop.go:maxToolOutputBytes at the source.
func compactMessage(m Message) Message {
	if m.Content == "" || len(m.Content) <= compactionThreshold {
		return m
	}

	// For tool results: keep first 2000 chars + truncation notice
	if m.Role == "tool" {
		truncated := m.Content[:2000] + "\n\n...[output truncated, original " + fmt.Sprintf("%d", len(m.Content)) + " chars]"
		return Message{
			Role:       m.Role,
			Content:    truncated,
			ToolCallID: m.ToolCallID,
			ToolName:   m.ToolName,
		}
	}

	// For assistant text: keep first 2000 chars + truncation notice
	if m.Role == "assistant" {
		truncated := m.Content[:2000] + "\n\n...[response truncated]"
		return Message{
			Role:      m.Role,
			Content:   truncated,
			ToolCalls: m.ToolCalls, // preserve tool calls
		}
	}

	// For user messages: keep in full (usually short)
	return m
}

// estimateTokens estimates the token count for a string.
func estimateTokens(s string) int {
	return (len(s) + charsPerToken - 1) / charsPerToken
}

// estimateMessageTokens estimates the total tokens for a message including
// overhead for role, tool calls, etc.
func estimateMessageTokens(m Message) int {
	tokens := estimateTokens(m.Content)

	// Add overhead for role marker
	tokens += 4

	// Add overhead for tool calls
	for _, tc := range m.ToolCalls {
		tokens += estimateTokens(tc.ID) + estimateTokens(tc.Name) + estimateTokens(tc.Arguments) + 10
	}

	// Add overhead for tool result metadata
	if m.ToolCallID != "" {
		tokens += estimateTokens(m.ToolCallID) + estimateTokens(m.ToolName) + 5
	}

	return tokens
}
