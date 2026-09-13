package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

const (
	openrouterDefaultModel   = "google/gemma-4-31b-it:free"
	openrouterDefaultBaseURL = "https://openrouter.ai/api/v1"
)

// OpenRouterProvider implements Provider using Eino's OpenAI-compatible
// ChatModel adapter pointed at OpenRouter's OpenAI-compatible endpoint.
// Supports multiple API keys with automatic rotation on rate limits.
type OpenRouterProvider struct {
	mu        sync.Mutex
	keys      []string
	keyIndex  int
	model     string
	baseURL   string
	cm        *einoopenai.ChatModel
}

// NewOpenRouterProvider builds an OpenRouter-backed provider.
// apiKeys is a list of API keys; the first is used initially, others are
// fallbacks tried automatically on rate-limit (429) errors.
func NewOpenRouterProvider(ctx context.Context, apiKeys []string, model, baseURL string) (*OpenRouterProvider, error) {
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("openrouter: at least one API key required")
	}
	if model == "" {
		model = openrouterDefaultModel
	}
	if baseURL == "" {
		baseURL = openrouterDefaultBaseURL
	}

	cm, err := buildOpenRouterModel(ctx, apiKeys[0], model, baseURL)
	if err != nil {
		return nil, fmt.Errorf("openrouter chat model: %w", err)
	}

	log.Printf("openrouter provider initialized (model=%s, baseURL=%s, keys=%d)", model, baseURL, len(apiKeys))

	return &OpenRouterProvider{
		keys:    apiKeys,
		model:   model,
		baseURL: baseURL,
		cm:      cm,
	}, nil
}

func buildOpenRouterModel(ctx context.Context, apiKey, model, baseURL string) (*einoopenai.ChatModel, error) {
	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
	})
}

func (p *OpenRouterProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *OpenRouterProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
	delay := initialRetryDelay
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return
		}

		perr := p.streamOnce(ctx, req, events)
		if perr == nil {
			return
		}

		// Rate limit: rotate to next API key before retrying
		if perr.Kind == ErrorKindRateLimit {
			if p.rotateKey() {
				log.Printf("openrouter: rate limited, rotated to next API key (key %d/%d)", p.keyIndex+1, len(p.keys))
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
				continue
			}
		}

		if !perr.Retryable || attempt >= maxRetryAttempts-1 {
			events <- ChatEvent{Type: ChatEventError, Err: perr}
			return
		}

		log.Printf("openrouter provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
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

// rotateKey switches to the next API key and rebuilds the chat model.
// Returns false if there are no more keys to try.
func (p *OpenRouterProvider) rotateKey() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	nextIndex := p.keyIndex + 1
	if nextIndex >= len(p.keys) {
		return false
	}

	p.keyIndex = nextIndex
	cm, err := buildOpenRouterModel(context.Background(), p.keys[nextIndex], p.model, p.baseURL)
	if err != nil {
		log.Printf("openrouter: failed to rebuild model with key %d: %v", nextIndex+1, err)
		return false
	}
	p.cm = cm
	return true
}

func (p *OpenRouterProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
	messages := convertOpenRouterMessages(req)
	toolInfos := convertOpenRouterToolSpecs(req.Tools)

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
		return classifyOpenRouterError(err)
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
				return nil
			}
			return classifyOpenRouterError(err)
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

func convertOpenRouterMessages(req ChatRequest) []*schema.Message {
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

func classifyOpenRouterError(err error) *ProviderError {
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "invalid api key") || strings.Contains(lower, "invalid_api_key") ||
		strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		return &ProviderError{Kind: ErrorKindAuth, Retryable: false, Err: err}
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "rate_limit") ||
		strings.Contains(lower, "too many requests"):
		return &ProviderError{Kind: ErrorKindRateLimit, Retryable: true, Err: err}
	case strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") ||
		strings.Contains(lower, "timeout") || errors.Is(err, context.DeadlineExceeded):
		return &ProviderError{Kind: ErrorKindTransient, Retryable: true, Err: err}
	}

	return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: err}
}

func convertOpenRouterToolSpecs(specs []ToolSpec) []*schema.ToolInfo {
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
