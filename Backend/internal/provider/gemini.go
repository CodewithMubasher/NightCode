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

	"google.golang.org/genai"

	einogemini "github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultModel      = "gemini-3.6-flash"
	maxRetryAttempts  = 3
	initialRetryDelay = time.Second
)

// GeminiProvider implements Provider using Eino's Gemini ChatModel adapter.
type GeminiProvider struct {
	cm    *einogemini.ChatModel
	model string
}

// NewGeminiProvider builds a Gemini-backed provider.
func NewGeminiProvider(ctx context.Context, apiKey, model string) (*GeminiProvider, error) {
	if model == "" {
		model = defaultModel
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("gemini client: %w", err)
	}

	cm, err := einogemini.NewChatModel(ctx, &einogemini.Config{
		Client: client,
		Model:  model,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini chat model: %w", err)
	}

	return &GeminiProvider{cm: cm, model: model}, nil
}

func (p *GeminiProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *GeminiProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
	delay := initialRetryDelay
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return
		}

		perr := p.streamOnce(ctx, req, events)
		if perr == nil {
			return
		}

		// Non-retryable (auth / unknown / cancellation) — surface immediately.
		if !perr.Retryable || attempt >= maxRetryAttempts-1 {
			events <- ChatEvent{Type: ChatEventError, Err: perr}
			return
		}

		log.Printf("gemini provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
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

// streamOnce performs a single model stream. It returns a *ProviderError if
	// the model call failed before or during streaming in a way that should abort
	// (or be retried). A nil return means the stream completed cleanly.
func (p *GeminiProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
	messages := convertMessages(req)
	toolInfos := convertToolSpecs(req.Tools)

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
		return classifyError(err)
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
			return classifyError(err)
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

// convertMessages maps provider Message to Eino schema.Message.
func convertMessages(req ChatRequest) []*schema.Message {
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

// classifyError maps a genai API error (or generic error) to a ProviderError.
func classifyError(err error) *ProviderError {
	var code int
	var status, message string

	var v genai.APIError
	var p *genai.APIError
	switch {
	case errors.As(err, &p) && p != nil:
		code = p.Code
		status = p.Status
		message = p.Message
	case errors.As(err, &v):
		code = v.Code
		status = v.Status
		message = v.Message
	}

	switch {
	case code == 401 || code == 403:
		return &ProviderError{Kind: ErrorKindAuth, Retryable: false, Err: err}
	case code == 429 || status == "RESOURCE_EXHAUSTED":
		return &ProviderError{Kind: ErrorKindRateLimit, Retryable: true, Err: err}
	case code >= 500:
		return &ProviderError{Kind: ErrorKindTransient, Retryable: true, Err: err}
	}

	// Gemini returns HTTP 400 (INVALID_ARGUMENT) for an invalid API key.
	if strings.Contains(message, "API key not valid") ||
		strings.Contains(message, "API_KEY_INVALID") ||
		strings.Contains(message, "API key expired") ||
		strings.Contains(status, "PERMISSION_DENIED") ||
		strings.Contains(status, "UNAUTHENTICATED") {
		return &ProviderError{Kind: ErrorKindAuth, Retryable: false, Err: err}
	}

	return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: err}
}

// jsonSchemaNode is a minimal JSON Schema representation for converting tool
// input schemas into Eino ParameterInfo.
type jsonSchemaNode struct {
	Type        string                     `json:"type"`
	Description string                     `json:"description"`
	Enum        []string                   `json:"enum"`
	Properties  map[string]*jsonSchemaNode `json:"properties"`
	Items       *jsonSchemaNode            `json:"items"`
	Required    []string                   `json:"required"`
}

func nodeToParamInfo(n *jsonSchemaNode) *schema.ParameterInfo {
	if n == nil {
		return &schema.ParameterInfo{Type: schema.String}
	}
	p := &schema.ParameterInfo{Desc: n.Description, Enum: n.Enum}
	switch n.Type {
	case "integer":
		p.Type = schema.Integer
	case "number":
		p.Type = schema.Number
	case "boolean":
		p.Type = schema.Boolean
	case "array":
		p.Type = schema.Array
		if n.Items != nil {
			p.ElemInfo = nodeToParamInfo(n.Items)
		}
	case "object":
		p.Type = schema.Object
		if n.Properties != nil {
			p.SubParams = make(map[string]*schema.ParameterInfo)
			for k, v := range n.Properties {
				p.SubParams[k] = nodeToParamInfo(v)
			}
		}
	default:
		p.Type = schema.String
	}
	return p
}

func convertToolSpecs(specs []ToolSpec) []*schema.ToolInfo {
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