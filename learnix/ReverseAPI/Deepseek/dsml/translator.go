package dsml

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// TranslatedResponse is the NightCode-compatible output of parsing
// a raw DeepSeek DSML response. It carries both the assistant's text
// content and any tool calls the model requested.
type TranslatedResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

// ToolCall is a single tool invocation extracted from DSML.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Translator converts raw DeepSeek DSML output into a TranslatedResponse.
type Translator interface {
	Translate(input string) (*TranslatedResponse, error)
}

// dsmlTranslator is the production implementation.
type dsmlTranslator struct{}

// New returns a Translator that parses DeepSeek DSML format.
func New() Translator {
	return &dsmlTranslator{}
}

// Tags (using the literal full-width-bar delimiter).
var (
	tagCallsOpen  = "<｜｜DSML｜｜ calls>"
	tagCallsClose = "</｜｜DSML｜｜ calls>"

	reInvokeOpen  = regexp.MustCompile(`<｜｜DSML｜｜ invoke name="([^"]*)">`)
	tagInvokeClose = "</｜｜DSML｜｜ invoke>"

	reParamOpen  = regexp.MustCompile(`<｜｜DSML｜｜ parameter name="([^"]*)" string="([^"]*)">`)
	tagParamClose = "</｜｜DSML｜｜ parameter>"
)

// Translate parses raw DeepSeek output and returns the structured response.
func (d *dsmlTranslator) Translate(input string) (*TranslatedResponse, error) {
	if strings.TrimSpace(input) == "" {
		return &TranslatedResponse{}, nil
	}

	// Fast path: no DSML tags at all → pure text response.
	if !strings.Contains(input, "DSML") {
		return &TranslatedResponse{Content: strings.TrimSpace(input)}, nil
	}

	callsStart := strings.Index(input, tagCallsOpen)
	callsEnd := strings.Index(input, tagCallsClose)

	// Malformed: has "DSML" but not the full calls block.
	if callsStart == -1 || callsEnd == -1 {
		cleaned := stripPartialDSML(input)
		return &TranslatedResponse{Content: strings.TrimSpace(cleaned)}, nil
	}

	// Text before the DSML block.
	textBefore := strings.TrimSpace(input[:callsStart])
	// Text after the DSML block.
	textAfter := strings.TrimSpace(input[callsEnd+len(tagCallsClose):])

	// If there's text before AND after, join with newline.
	content := textBefore
	if textAfter != "" {
		if content != "" {
			content += "\n" + textAfter
		} else {
			content = textAfter
		}
	}

	// Extract the tool calls block.
	callsBlock := input[callsStart+len(tagCallsOpen) : callsEnd]

	toolCalls, err := parseToolCalls(callsBlock)
	if err != nil {
		return nil, fmt.Errorf("parse tool calls: %w", err)
	}

	return &TranslatedResponse{
		Content:   content,
		ToolCalls: toolCalls,
	}, nil
}

// parseToolCalls extracts all <invoke> blocks from the calls section.
func parseToolCalls(block string) ([]ToolCall, error) {
	var calls []ToolCall
	idCounter := 0

	for {
		// Find next invoke open tag.
		loc := reInvokeOpen.FindStringSubmatchIndex(block)
		if loc == nil {
			break
		}
		toolName := block[loc[2]:loc[3]]

		// Find matching invoke close tag.
		invokeStart := loc[1]
		closeIdx := strings.Index(block[invokeStart:], tagInvokeClose)
		if closeIdx == -1 {
			return nil, fmt.Errorf("unclosed invoke block for tool %q", toolName)
		}

		invokeBody := block[invokeStart : invokeStart+closeIdx]
		block = block[invokeStart+closeIdx+len(tagInvokeClose):]

		// Parse parameters from invoke body.
		params, err := parseParameters(invokeBody)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", toolName, err)
		}

		idCounter++
		calls = append(calls, ToolCall{
			ID:        fmt.Sprintf("call_%d", idCounter),
			Name:      toolName,
			Arguments: params,
		})
	}

	return calls, nil
}

// parseParameters extracts all <parameter> blocks from an invoke body.
func parseParameters(body string) (map[string]any, error) {
	params := make(map[string]any)

	for {
		loc := reParamOpen.FindStringSubmatchIndex(body)
		if loc == nil {
			break
		}
		paramName := body[loc[2]:loc[3]]
		isString := body[loc[4]:loc[5]] == "true"

		paramStart := loc[1]
		closeIdx := strings.Index(body[paramStart:], tagParamClose)
		if closeIdx == -1 {
			return nil, fmt.Errorf("unclosed parameter %q", paramName)
		}

		rawValue := body[paramStart : paramStart+closeIdx]
		body = body[paramStart+closeIdx+len(tagParamClose):]

		if isString {
			params[paramName] = rawValue
		} else {
			// Try to parse as JSON number/bool, fall back to string.
			params[paramName] = parseNonString(rawValue)
		}
	}

	return params, nil
}

// parseNonString attempts to interpret a parameter value as a non-string
// JSON type (number, bool, null). Falls back to the raw string.
func parseNonString(raw string) any {
	trimmed := strings.TrimSpace(raw)

	// Boolean.
	if trimmed == "true" {
		return true
	}
	if trimmed == "false" {
		return false
	}
	if trimmed == "null" {
		return nil
	}

	// Integer.
	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i
	}
	// Float.
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}

	return raw
}

// stripPartialDSML removes any partial DSML tags from text that didn't
// form a complete calls block. This handles truncated model output.
func stripPartialDSML(input string) string {
	// Remove any partial DSML tags.
	lines := strings.Split(input, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "DSML") {
			// Skip lines that are clearly DSML protocol.
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
