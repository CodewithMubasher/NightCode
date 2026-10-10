package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

const (
	cloudflareDefaultModel   = "@cf/openai/gpt-oss-20b"
	// cloudflareDefaultAccountID is the fallback account ID kept for
	// backwards compatibility; override with CLOUDFLARE_ACCOUNT_ID.
	cloudflareDefaultAccountID = "4bf1720eea8954695027af95c83b7600"
)

// cloudflareBaseURL builds the Workers AI base URL from the account ID,
// honouring CLOUDFLARE_BASE_URL when set, otherwise constructing it from
// CLOUDFLARE_ACCOUNT_ID (or the built-in default).
func cloudflareBaseURL(baseURL string) string {
	if baseURL != "" {
		return baseURL
	}
	acct := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if acct == "" {
		acct = cloudflareDefaultAccountID
	}
	return "https://api.cloudflare.com/client/v4/accounts/" + acct + "/ai/v1"
}

// CloudflareProvider implements Provider using Eino's OpenAI-compatible ChatModel
// adapter pointed at Cloudflare Workers AI.
type CloudflareProvider struct {
	cm    *einoopenai.ChatModel
	model string
}

// NewCloudflareProvider builds a Cloudflare-backed provider.
func NewCloudflareProvider(ctx context.Context, apiKey, model, baseURL string) (*CloudflareProvider, error) {
	if model == "" {
		model = cloudflareDefaultModel
	}
	baseURL = cloudflareBaseURL(baseURL)

	cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("cloudflare chat model: %w", err)
	}

	log.Printf("cloudflare provider initialized (model=%s, baseURL=%s)", model, baseURL)

	return &CloudflareProvider{cm: cm, model: model}, nil
}

func (p *CloudflareProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)

	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()

	return events, nil
}

func (p *CloudflareProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
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
			if !sendChatEvent(ctx, events, ChatEvent{Type: ChatEventError, Err: perr}) {
				return
			}
			return
		}

		log.Printf("cloudflare provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
		if !sendChatEvent(ctx, events, ChatEvent{
			Type:       ChatEventRetry,
			Err:        perr.Err,
			Attempt:    attempt + 1,
			MaxAttempt: maxRetryAttempts,
			RetryAfter: delay,
		}) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay *= 2
	}
}

func (p *CloudflareProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
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
			if !sendChatEvent(ctx, events, ChatEvent{Type: ChatEventDelta, Text: msg.Content}) {
				return nil
			}
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
		if !sendChatEvent(ctx, events, ChatEvent{Type: ChatEventToolCalls, ToolCalls: toolCalls}) {
			return nil
		}
	}
	return nil
}
