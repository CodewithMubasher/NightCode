package provider

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/genai"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantKind  ErrorKind
		wantRetry bool
	}{
		{
			name:      "auth 401 value",
			err:       genai.APIError{Code: 401, Message: "unauthorized"},
			wantKind:  ErrorKindAuth,
			wantRetry: false,
		},
		{
			name:      "auth 403 pointer",
			err:       &genai.APIError{Code: 403, Message: "forbidden"},
			wantKind:  ErrorKindAuth,
			wantRetry: false,
		},
		{
			name:      "rate limit 429",
			err:       genai.APIError{Code: 429, Message: "quota exceeded"},
			wantKind:  ErrorKindRateLimit,
			wantRetry: true,
		},
		{
			name:      "transient 500",
			err:       genai.APIError{Code: 500, Message: "internal"},
			wantKind:  ErrorKindTransient,
			wantRetry: true,
		},
		{
			name:      "transient 503 pointer",
			err:       &genai.APIError{Code: 503, Message: "unavailable"},
			wantKind:  ErrorKindTransient,
			wantRetry: true,
		},
		{
			name:      "wrapped rate limit",
			err:       fmt.Errorf("stream failed: %w", genai.APIError{Code: 429}),
			wantKind:  ErrorKindRateLimit,
			wantRetry: true,
		},
		{
			name:      "generic error",
			err:       errors.New("boom"),
			wantKind:  ErrorKindOther,
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			if got.Retryable != tt.wantRetry {
				t.Errorf("Retryable = %v, want %v", got.Retryable, tt.wantRetry)
			}
		})
	}
}