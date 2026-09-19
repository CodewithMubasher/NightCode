package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	nightcodeDefaultModel   = "gemini-3.6-flash"
	nightcodeDefaultBaseURL = "http://127.0.0.1:31415/v1"
)

// NightcodeProvider implements Provider using raw HTTP streaming against
// a local Nightcode LLM API gateway (OpenAI-compatible).
type NightcodeProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewNightcodeProvider builds a Nightcode-backed provider.
func NewNightcodeProvider(_ context.Context, apiKey, model, baseURL string) (*NightcodeProvider, error) {
	if model == "" {
		model = nightcodeDefaultModel
	}
	if baseURL == "" {
		baseURL = nightcodeDefaultBaseURL
	}

	log.Printf("nightcode provider initialized (model=%s, baseURL=%s)", model, baseURL)

	return &NightcodeProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

func (p *NightcodeProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	events := make(chan ChatEvent, 8)
	go func() {
		defer close(events)
		p.streamWithRetry(ctx, req, events)
	}()
	return events, nil
}

func (p *NightcodeProvider) streamWithRetry(ctx context.Context, req ChatRequest, events chan<- ChatEvent) {
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
		log.Printf("nightcode provider: retryable error (attempt %d/%d): %v", attempt+1, maxRetryAttempts, perr.Err)
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

type nightcodeRequest struct {
	Model    string           `json:"model"`
	Messages []nightcodeMsg   `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []nightcodeTool  `json:"tools,omitempty"`
}

type nightcodeMsg struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []nightcodeTC     `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type nightcodeTool struct {
	Type     string            `json:"type"`
	Function nightcodeFunction `json:"function"`
}

type nightcodeFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type nightcodeTC struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function nightcodeTCFunc  `json:"function"`
}

type nightcodeTCFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (p *NightcodeProvider) streamOnce(ctx context.Context, req ChatRequest, events chan<- ChatEvent) *ProviderError {
	body := p.buildRequest(req)
	log.Printf("nightcode streamOnce: model=%s msgs=%d tools=%d", p.model, len(body.Messages), len(body.Tools))

	payload, err := json.Marshal(body)
	if err != nil {
		return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: fmt.Errorf("marshal request: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: fmt.Errorf("create request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		log.Printf("nightcode streamOnce: HTTP error: %v", err)
		return ClassifyOpenAICompatError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Printf("nightcode streamOnce: HTTP %d: %s", resp.StatusCode, string(respBody))
		return &ProviderError{
			Kind:      classifyNightcodeHTTP(resp.StatusCode),
			Retryable: resp.StatusCode == 429 || resp.StatusCode >= 500,
			Err:       fmt.Errorf("nightcode API error (status %d): %s", resp.StatusCode, string(respBody)),
		}
	}

	// Peek first byte to detect JSON error vs SSE stream.
	prefix := make([]byte, 1)
	if _, err := io.ReadFull(resp.Body, prefix); err != nil {
		return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: fmt.Errorf("read response: %w", err)}
	}
	if prefix[0] == '{' {
		rest, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		full := append(prefix, rest...)
		log.Printf("nightcode streamOnce: non-SSE response: %s", string(full))
		return &ProviderError{Kind: ErrorKindOther, Retryable: false, Err: fmt.Errorf("nightcode API error: %s", string(full))}
	}

	combined := io.MultiReader(bytes.NewReader(prefix), resp.Body)
	var toolCalls []ToolCall
	chunkCount := 0
	scanner := bufio.NewScanner(combined)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk nightcodeChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("nightcode streamOnce: chunk parse error: %v", err)
			continue
		}

		chunkCount++
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				events <- ChatEvent{Type: ChatEventDelta, Text: delta.Content}
			}
			for _, tc := range delta.ToolCalls {
				toolCalls = append(toolCalls, ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("nightcode streamOnce: scanner error after %d chunks: %v", chunkCount, err)
		if ctx.Err() != nil {
			return nil
		}
		return ClassifyOpenAICompatError(err)
	}

	log.Printf("nightcode streamOnce: done, %d chunks, %d tool calls", chunkCount, len(toolCalls))

	if len(toolCalls) > 0 {
		events <- ChatEvent{Type: ChatEventToolCalls, ToolCalls: toolCalls}
	}
	return nil
}

func (p *NightcodeProvider) buildRequest(req ChatRequest) *nightcodeRequest {
	msgs := make([]nightcodeMsg, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, nightcodeMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, nightcodeMsg{Role: "user", Content: m.Content})
		case "assistant":
			msg := nightcodeMsg{Role: "assistant", Content: m.Content}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, nightcodeTC{
					ID:   tc.ID,
					Type: "function",
					Function: nightcodeTCFunc{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			msgs = append(msgs, msg)
		case "tool":
			msgs = append(msgs, nightcodeMsg{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
			})
		}
	}

	body := &nightcodeRequest{
		Model:    p.model,
		Messages: msgs,
		Stream:   true,
	}
	if len(req.Tools) > 0 {
		tools := make([]nightcodeTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, nightcodeTool{
				Type: "function",
				Function: nightcodeFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Schema,
				},
			})
		}
		body.Tools = tools
	}
	return body
}

type nightcodeChunk struct {
	Choices []nightcodeChoice `json:"choices"`
}

type nightcodeChoice struct {
	Delta nightcodeDelta `json:"delta"`
}

type nightcodeDelta struct {
	Content   string        `json:"content"`
	ToolCalls []nightcodeTC `json:"tool_calls"`
}

func classifyNightcodeHTTP(code int) ErrorKind {
	switch {
	case code == 401 || code == 403:
		return ErrorKindAuth
	case code == 429:
		return ErrorKindRateLimit
	case code >= 500:
		return ErrorKindTransient
	default:
		return ErrorKindOther
	}
}
