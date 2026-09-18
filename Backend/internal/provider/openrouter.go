package provider

import (
	"context"
	"fmt"
	"io"
	"log"
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
