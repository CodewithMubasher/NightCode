package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// ConvertOpenAICompatMessages maps provider Message to Eino schema.Message.
// Used by all OpenAI-compatible providers (Groq, OpenRouter, OpenCode,
// Calabrass, Cloudflare, AshnaAI). Gemini uses a different converter.
func ConvertOpenAICompatMessages(req ChatRequest) []*schema.Message {
	msgs := make([]*schema.Message, 0, len(req.Messages)+1)

	if req.SystemPrompt != "" {
		msgs = append(msgs, &schema.Message{Role: schema.System, Content: req.SystemPrompt})
	}

	for _, m := range req.Messages {
		msg := &schema.Message{Content: m.Content}
		switch m.Role {
		case "user":
			msg.Role = schema.User
		case "assistant":
			msg.Role = schema.Assistant
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: schema.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
					Extra: tc.Extra,
				})
			}
		case "tool":
			msg.Role = schema.Tool
			msg.ToolCallID = m.ToolCallID
			msg.ToolName = m.ToolName
		default:
			continue
		}
		msgs = append(msgs, msg)
	}

	return msgs
}

// ConvertOpenAICompatToolSpecs converts provider ToolSpec to Eino ToolInfo.
// Used by all OpenAI-compatible providers. Reuses the jsonSchemaNode /
// nodeToParamInfo helpers defined in gemini.go.
func ConvertOpenAICompatToolSpecs(specs []ToolSpec) []*schema.ToolInfo {
	if len(specs) == 0 {
		return nil
	}
	out := make([]*schema.ToolInfo, 0, len(specs))
	for _, s := range specs {
		ti := &schema.ToolInfo{Name: s.Name, Desc: s.Description}
		if len(s.Schema) > 0 {
			var root jsonSchemaNode
			if err := json.Unmarshal(s.Schema, &root); err == nil && root.Properties != nil {
				params := make(map[string]*schema.ParameterInfo, len(root.Properties))
				required := make(map[string]bool, len(root.Required))
				for _, r := range root.Required {
					required[r] = true
				}
				for name, prop := range root.Properties {
					pi := nodeToParamInfo(prop)
					if required[name] {
						pi.Required = true
					}
					params[name] = pi
				}
				ti.ParamsOneOf = schema.NewParamsOneOfByParams(params)
			}
		}
		out = append(out, ti)
	}
	return out
}

// ClassifyOpenAICompatError maps an OpenAI-compatible API error to a
// ProviderError. Handles the union of error patterns across all providers:
// auth (401/403), rate limit (429 + "too many requests"), transient (5xx).
func ClassifyOpenAICompatError(err error) *ProviderError {
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "invalid api key") || strings.Contains(lower, "invalid_api_key") ||
		strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		return &ProviderError{Kind: ErrorKindAuth, Retryable: false, Err: err}
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "rate_limit") || strings.Contains(lower, "too many requests"):
		return &ProviderError{Kind: ErrorKindRateLimit, Retryable: true, Err: err}
	case strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") ||
		strings.Contains(lower, "timeout") || errors.Is(err, context.DeadlineExceeded):
		return &ProviderError{Kind: ErrorKindTransient, Retryable: true, Err: err}
	}

	return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: err}
}
