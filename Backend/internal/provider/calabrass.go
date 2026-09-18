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
	calabrassDefaultModel   = "gemini-3.6-flash"
	calabrassDefaultBaseURL = "http://127.0.0.1:8081/v1"
)

// CalabrassProvider implements Provider using Eino's OpenAI-compatible ChatModel
// adapter pointed at a local Calabrass server.
type CalabrassProvider struct {
	cm    *einoopenai.ChatModel
	model string
}

// NewCalabrassProvider builds a Calabrass-backed provider. Calabrass exposes
// an OpenAI-compatible API with no authentication required.
func NewCalabrassProvider(ctx context.Context, model, baseURL string) (*CalabrassProvider, error) {
	if model == "" {
		model = calabrassDefaultModel
	}
	if baseURL == "" {
		baseURL = calabrassDefaultBaseURL
	}

	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  "no-key",
		Model:   model,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("calabrass chat model: %w", err)
	}

	log.Printf("calabrass provider initialized (model=%s, baseURL=%s)", model, baseURL)

	return &CalabrassProvider{cm: cm, model: model}, nil
}

func (p *CalabrassProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *CalabrassProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
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

		log.Printf("calabrass provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
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

func (p *CalabrassProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
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
				return nil
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
