package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

const (
	web2apiDefaultModel   = "DeepSeekV4.1"
	web2apiDefaultBaseURL = "http://127.0.0.1:8080/v1"
)

// chatIDKey is the context key for passing the NightCode chat ID to the
// HTTP transport, which adds it as X-NightCode-Chat-ID header.
type chatIDKey struct{}

// ChatIDFromContext extracts the chat ID from context.
func ChatIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(chatIDKey{}).(string); ok {
		return v
	}
	return ""
}

// chatIDTransport is a custom RoundTripper that injects the X-NightCode-Chat-ID
// header from the context into every HTTP request.
type chatIDTransport struct {
	base http.RoundTripper
}

func (t *chatIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if chatID := ChatIDFromContext(req.Context()); chatID != "" {
		req.Header.Set("X-NightCode-Chat-ID", chatID)
	}
	return t.base.RoundTrip(req)
}

// Web2APIProvider implements Provider using Eino's OpenAI-compatible ChatModel
// adapter pointed at a local Web2API server (e.g. deepseek-tui). The Web2API
// server handles all the DeepSeek Web browser automation, tool call parsing,
// and session management. NightCode just sees a standard OpenAI-compatible API.
type Web2APIProvider struct {
	cm    *einoopenai.ChatModel
	model string
}

// NewWeb2APIProvider builds a Web2API-backed provider.
func NewWeb2APIProvider(ctx context.Context, apiKey, model, baseURL string) (*Web2APIProvider, error) {
	if model == "" {
		model = web2apiDefaultModel
	}
	if baseURL == "" {
		baseURL = web2apiDefaultBaseURL
	}

	// Use a custom transport that injects X-NightCode-Chat-ID header per request.
	httpClient := &http.Client{
		Transport: &chatIDTransport{base: http.DefaultTransport},
	}

	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("web2api chat model: %w", err)
	}

	log.Printf("web2api provider initialized (model=%s, baseURL=%s)", model, baseURL)

	return &Web2APIProvider{cm: cm, model: model}, nil
}

func (p *Web2APIProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *Web2APIProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
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

		log.Printf("web2api provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
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

func (p *Web2APIProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
	messages := convertWeb2APIMessages(req)
	toolInfos := convertWeb2APIToolSpecs(req.Tools)

	// Inject chat ID into context so the custom transport adds the header.
	if req.ChatID != "" {
		ctx = context.WithValue(ctx, chatIDKey{}, req.ChatID)
	}

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
		return classifyWeb2APIError(err)
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
			return classifyWeb2APIError(err)
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

func convertWeb2APIMessages(req ChatRequest) []*schema.Message {
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

func classifyWeb2APIError(err error) *ProviderError {
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
		strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") ||
		errors.Is(err, context.DeadlineExceeded):
		return &ProviderError{Kind: ErrorKindTransient, Retryable: true, Err: err}
	case strings.Contains(lower, "deepseek web chat error") || strings.Contains(lower, "browser"):
		return &ProviderError{Kind: ErrorKindTransient, Retryable: true, Err: err}
	}

	return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: err}
}

func convertWeb2APIToolSpecs(specs []ToolSpec) []*schema.ToolInfo {
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
