package api

import (
	"encoding/json"
	"net/http"
	"os"
)

// modelInfo describes one selectable model for the frontend's model picker.
type modelInfo struct {
	Provider string `json:"provider"` // "gemini" | "groq"
	ID       string `json:"id"`       // model identifier sent back on send
	Label    string `json:"label"`    // display name
	Default  bool   `json:"default"`  // true for the server's startup default
}

// geminiModels is a curated list of current Gemini models. Kept in code
// (rather than calling a live ListModels endpoint) so the picker stays fast
// and doesn't depend on an extra round trip per page load.
//
// NOTE: which models are actually available depends on the API key's
// account/tier — Google rolls out new model names over time and older ones
// get retired. If a listed model 400s as unknown, check what's actually
// available for your key at https://ai.google.dev/gemini-api/docs/models
// and update this list to match.
var geminiModels = []string{
	"gemini-3.6-flash",
}

// groqModels is a curated list of models currently available on Groq's free
// tier. Verified directly against GET https://api.groq.com/openai/v1/models
// rather than assumed — Groq deprecates/renames models fairly often, and a
// stale hardcoded list here causes silent-looking failures (the request
// gets rejected by Groq, e.g. 400 model_not_found or 413 too-large-for-TPM,
// and previously that error had nowhere to surface in the UI).
var groqModels = []string{
	"openai/gpt-oss-20b",
	"qwen/qwen3.8-27b",
	"qwen/qwen3.6-27b",
	"groq/compound",
	"groq/compound-mini",
	"allam-2-7b",
}

// GET /api/models
// Returns only models for providers that actually have an API key configured
// in the environment, so the frontend never offers a model the backend can't
// serve. Also marks the server's startup-default (provider, model) pair.
func (h *Handler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	var out []modelInfo

	if os.Getenv("GEMINI_API_KEY") != "" {
		for _, id := range geminiModels {
			out = append(out, modelInfo{
				Provider: "gemini",
				ID:       id,
				Label:    labelForModel(id),
				Default:  h.providerName == "gemini" && h.modelName == id,
			})
		}
	}

	if os.Getenv("GROQ_API_KEY") != "" {
		for _, id := range groqModels {
			out = append(out, modelInfo{
				Provider: "groq",
				ID:       id,
				Label:    labelForModel(id),
				Default:  h.providerName == "groq" && h.modelName == id,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if out == nil {
		out = []modelInfo{}
	}
	json.NewEncoder(w).Encode(out)
}

// labelForModel turns a raw model ID into a friendlier display label.
func labelForModel(id string) string {
	labels := map[string]string{
		"gemini-3.6-flash":     "Gemini 3.6 Flash",
		"openai/gpt-oss-20b":   "GPT-OSS 20B",
		"qwen/qwen3.8-27b":     "Qwen3 8x27B",
		"qwen/qwen3.6-27b":     "Qwen3 6x27B",
		"groq/compound":        "Groq Compound",
		"groq/compound-mini":   "Groq Compound Mini",
		"allam-2-7b":           "Allam 2 7B",
	}
	if l, ok := labels[id]; ok {
		return l
	}
	return id
}
