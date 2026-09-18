package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

const (
	groqDefaultModel   = "openai/gpt-oss-20b"
	groqDefaultBaseURL = "https://api.groq.com/openai/v1"
)

// GroqProvider implements Provider using Eino's OpenAI-compatible ChatModel
// adapter pointed at Groq's OpenAI-compatible endpoint.
type GroqProvider struct {
	cm    *einoopenai.ChatModel
	model string
}

// NewGroqProvider builds a Groq-backed provider. Groq exposes an
// OpenAI-compatible API, so we reuse Eino's openai component with a custom
// BaseURL rather than needing a dedicated Groq adapter.
func NewGroqProvider(ctx context.Context, apiKey, model, baseURL string) (*GroqProvider, error) {
	if model == "" {
		model = groqDefaultModel
	}
	if baseURL == "" {
		baseURL = groqDefaultBaseURL
	}

	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("groq chat model: %w", err)
	}

	log.Printf("groq provider initialized (model=%s, baseURL=%s)", model, baseURL)

	return &GroqProvider{cm: cm, model: model}, nil
}

func (p *GroqProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *GroqProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
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

		log.Printf("groq provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
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

func (p *GroqProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
	messages := ConvertOpenAICompatMessages(req)
	toolInfos := ConvertOpenAICompatToolSpecs(req.Tools)

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
		return ClassifyOpenAICompatError(err)
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
			return ClassifyOpenAICompatError(err)
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
