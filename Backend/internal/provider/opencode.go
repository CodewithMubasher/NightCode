package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

const (
	opencodeDefaultModel   = "mimo-v2.5-free"
	opencodeDefaultBaseURL = "https://opencode.ai/zen/v1"
)

// OpenCodeProvider implements Provider using Eino's OpenAI-compatible ChatModel
// adapter pointed at Opencode Zen's OpenAI-compatible endpoint.
type OpenCodeProvider struct {
	cm    *einoopenai.ChatModel
	model string
}

// NewOpenCodeProvider builds an Opencode Zen-backed provider. Opencode Zen
// exposes an OpenAI-compatible API, so we reuse Eino's openai component with
// a custom BaseURL.
func NewOpenCodeProvider(ctx context.Context, apiKey, model, baseURL string) (*OpenCodeProvider, error) {
	if model == "" {
		model = opencodeDefaultModel
	}
	if baseURL == "" {
		baseURL = opencodeDefaultBaseURL
	}

	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("opencode chat model: %w", err)
	}

	log.Printf("opencode provider initialized (model=%s, baseURL=%s)", model, baseURL)

	return &OpenCodeProvider{cm: cm, model: model}, nil
}

func (p *OpenCodeProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *OpenCodeProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
	delay := initialRetryDelay
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return
		}

		perr := p.streamOnce(ctx, req, events)
		if perr == nil {
			return
		}

		if !perr.Retryable || attempt >= maxRetryAttempts-1 {
			events <- ChatEvent{Type: ChatEventError, Err: perr}
			return
		}

		log.Printf("opencode provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
		events <- ChatEvent{
			Type:       ChatEventRetry,
			Err:        perr.Err,
			Attempt:    attempt + 1,
			MaxAttempt: maxRetryAttempts,
			RetryAfter: delay,
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay *= 2
	}
}

func (p *OpenCodeProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
	messages := convertOpenCodeMessages(req)
	toolInfos := convertOpenCodeToolSpecs(req.Tools)

	var (
		sr  *schema.StreamReader[*schema.Message]
		err error
	)

	if len(toolInfos) > 0 {
		tcm, bindErr := p.cm.WithTools(toolInfos)
		if bindErr != nil {
			return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: fmt.Errorf("bind tools: %w", bindErr)}
		}
		sr, err = tcm.Stream(ctx, messages)
	} else {
		sr, err = p.cm.Stream(ctx, messages)
	}

	if err != nil {
		return classifyOpenCodeError(err)
	}
	defer sr.Close()

	var toolCalls []ToolCall
	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled — not an error to surface
			}
			return classifyOpenCodeError(err)
		}

		if msg.Content != "" {
			events <- ChatEvent{Type: ChatEventDelta, Text: msg.Content}
		}
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Extra:     tc.Extra,
			})
		}
	}

	if len(toolCalls) > 0 {
		events <- ChatEvent{Type: ChatEventToolCalls, ToolCalls: toolCalls}
	}
	return nil
}

func convertOpenCodeMessages(req ChatRequest) []*schema.Message {
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

func classifyOpenCodeError(err error) *ProviderError {
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "invalid api key") || strings.Contains(lower, "invalid_api_key") ||
		strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		return &ProviderError{Kind: ErrorKindAuth, Retryable: false, Err: err}
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "rate_limit"):
		return &ProviderError{Kind: ErrorKindRateLimit, Retryable: true, Err: err}
	case strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") ||
		strings.Contains(lower, "timeout") || errors.Is(err, context.DeadlineExceeded):
		return &ProviderError{Kind: ErrorKindTransient, Retryable: true, Err: err}
	}

	return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: err}
}

func convertOpenCodeToolSpecs(specs []ToolSpec) []*schema.ToolInfo {
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
