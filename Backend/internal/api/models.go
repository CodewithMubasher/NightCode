package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

// modelInfo describes one selectable model for the frontend's model picker.
type modelInfo struct {
	Provider string `json:"provider"` // "gemini" | "groq" | "opencode-zen" | "openrouter"
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

var opencodeModels = []string{
	"deepseek-v4-flash-free",
	"mimo-v2-pro-free",
	"mimo-v2-omni-free",
	"mimo-v2.5-free",
	"minimax-m2.5-free",
	"nemotron-3-super-free",
	"big-pickle",
	"laguna-s-2.1-free",
	"ling-3.0-tiny-free",
	"longcat-2.0-free",
}

var openrouterModels = []string{
	"google/gemma-4-31b-it:free",
	"google/gemma-4-26b-a4b-it:free",
	"nvidia/nemotron-3-super-120b-a12b:free",
	"nvidia/nemotron-3.5-lightning:free",
	"nvidia/nemotron-3-ultra-550b-a55b:free",
	"poolside/laguna-s-2.1:free",
	"cohere/north-mini-code:free",
	"thinkingmachines/inkling:free",
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

	if os.Getenv("OPENCODE_API_KEY") != "" {
		for _, id := range opencodeModels {
			out = append(out, modelInfo{
				Provider: "opencode-zen",
				ID:       id,
				Label:    labelForModel(id),
				Default:  h.providerName == "opencode-zen" && h.modelName == id,
			})
		}
	}

	if os.Getenv("OPENROUTER_API_KEY") != "" {
		for _, id := range openrouterModels {
			out = append(out, modelInfo{
				Provider: "openrouter",
				ID:       id,
				Label:    labelForModel(id),
				Default:  h.providerName == "openrouter" && h.modelName == id,
			})
		}
	}

	// Calabrass: local server, no API key needed. Fetch models dynamically.
	if calabrassModels := calabrassModels(); len(calabrassModels) > 0 {
		for _, id := range calabrassModels {
			out = append(out, modelInfo{
				Provider: "calabrass",
				ID:       id,
				Label:    labelForModel(id),
				Default:  h.providerName == "calabrass" && h.modelName == id,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if out == nil {
		out = []modelInfo{}
	}
	json.NewEncoder(w).Encode(out)
}

// calabrassModels fetches available models from the local Calabrass server
// at http://127.0.0.1:8081/v1/models. Returns empty list if unreachable.
func calabrassModels() []string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8081/v1/models")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}

	var models []string
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models
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
		"deepseek-v4-flash-free": "DeepSeek V4 Flash Free",
		"mimo-v2-pro-free":      "MiMo V2 Pro Free",
		"mimo-v2-omni-free":     "MiMo V2 Omni Free",
		"mimo-v2.5-free":        "MiMo V2.5 Free",
		"minimax-m2.5-free":     "MiniMax M2.5 Free",
		"nemotron-3-super-free":  "Nemotron 3 Super Free",
		"big-pickle":             "Big Pickle",
		"laguna-s-2.1-free":      "Laguna S 2.1 Free",
		"ling-3.0-tiny-free":     "Ling 3.0 Tiny Free",
		"longcat-2.0-free":       "LongCat 2.0 Free",
		"google/gemma-4-31b-it:free":            "Gemma 4 31B",
		"google/gemma-4-26b-a4b-it:free":        "Gemma 4 26B A4B",
		"nvidia/nemotron-3-super-120b-a12b:free": "Nemotron 3 Super 120B",
		"nvidia/nemotron-3.5-lightning:free":     "Nemotron 3.5 Lightning",
		"nvidia/nemotron-3-ultra-550b-a55b:free": "Nemotron 3 Ultra 550B",
		"poolside/laguna-s-2.1:free":             "Laguna S 2.1",
		"cohere/north-mini-code:free":            "North Mini Code",
		"thinkingmachines/inkling:free":          "Inkling",
	}
	if l, ok := labels[id]; ok {
		return l
	}
	return id
}
