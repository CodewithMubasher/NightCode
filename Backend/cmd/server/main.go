package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/api"
	"github.com/CodewithMubasher/NightCode/backend/internal/provider"
	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present (no error if missing)
	_ = godotenv.Load()

	port := os.Getenv("NIGHTCODE_PORT")
	if port == "" {
		port = "3001"
	}

	dbPath := os.Getenv("NIGHTCODE_DB_PATH")
	if dbPath == "" {
		dbPath = "nightcode.db"
	}

	corsOrigin := os.Getenv("NIGHTCODE_CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:5173"
	}

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	h := api.NewHandler(s)
	registry := tools.NewRegistry()

	// Provider selection: NIGHTCODE_PROVIDER=echo|gemini|groq (default gemini)
	providerMode := os.Getenv("NIGHTCODE_PROVIDER")
	if providerMode == "" {
		providerMode = "gemini"
	}

	switch providerMode {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		model := os.Getenv("GEMINI_MODEL")
		if apiKey == "" {
			log.Println("WARNING: GEMINI_API_KEY not set; falling back to echo agent")
		} else {
			p, err := provider.NewGeminiProvider(context.Background(), apiKey, model)
			if err != nil {
				log.Fatalf("failed to create gemini provider: %v", err)
			}
			h.SetProvider(p)
			h.SetDefaultProviderInfo("gemini", modelOr(model))
			log.Printf("using gemini provider (model=%s)", modelOr(model))
		}
	case "groq":
		apiKey := os.Getenv("GROQ_API_KEY")
		model := os.Getenv("GROQ_MODEL")
		baseURL := os.Getenv("GROQ_BASE_URL")
		if apiKey == "" {
			log.Println("WARNING: GROQ_API_KEY not set; falling back to echo agent")
		} else {
			p, err := provider.NewGroqProvider(context.Background(), apiKey, model, baseURL)
			if err != nil {
				log.Fatalf("failed to create groq provider: %v", err)
			}
			h.SetProvider(p)
			h.SetDefaultProviderInfo("groq", groqModelOr(model))
			log.Printf("using groq provider (model=%s)", groqModelOr(model))
		}
	case "opencode-zen":
		apiKey := os.Getenv("OPENCODE_API_KEY")
		model := os.Getenv("OPENCODE_MODEL")
		baseURL := os.Getenv("OPENCODE_BASE_URL")
		if apiKey == "" {
			log.Println("WARNING: OPENCODE_API_KEY not set; falling back to echo agent")
		} else {
			p, err := provider.NewOpenCodeProvider(context.Background(), apiKey, model, baseURL)
			if err != nil {
				log.Fatalf("failed to create opencode provider: %v", err)
			}
			h.SetProvider(p)
			h.SetDefaultProviderInfo("opencode-zen", opencodeModelOr(model))
			log.Printf("using opencode-zen provider (model=%s)", opencodeModelOr(model))
		}
	case "openrouter":
		model := os.Getenv("OPENROUTER_MODEL")
		baseURL := os.Getenv("OPENROUTER_BASE_URL")
		var keys []string
		if k := os.Getenv("OPENROUTER_API_KEY"); k != "" {
			keys = append(keys, k)
		}
		if k := os.Getenv("OPENROUTER_API_KEY_2"); k != "" {
			keys = append(keys, k)
		}
		if k := os.Getenv("OPENROUTER_API_KEY_3"); k != "" {
			keys = append(keys, k)
		}
		if len(keys) == 0 {
			log.Println("WARNING: OPENROUTER_API_KEY not set; falling back to echo agent")
		} else {
			p, err := provider.NewOpenRouterProvider(context.Background(), keys, model, baseURL)
			if err != nil {
				log.Fatalf("failed to create openrouter provider: %v", err)
			}
			h.SetProvider(p)
			h.SetDefaultProviderInfo("openrouter", openrouterModelOr(model))
			log.Printf("using openrouter provider (model=%s, keys=%d)", openrouterModelOr(model), len(keys))
		}
	case "calabrass":
		model := os.Getenv("CALABRASS_MODEL")
		baseURL := os.Getenv("CALABRASS_BASE_URL")
		p, err := provider.NewCalabrassProvider(context.Background(), model, baseURL)
		if err != nil {
			log.Fatalf("failed to create calabrass provider: %v", err)
		}
		h.SetProvider(p)
		h.SetDefaultProviderInfo("calabrass", calabrassModelOr(model))
		log.Printf("using calabrass provider (model=%s)", calabrassModelOr(model))
	default:
		log.Println("using echo agent (NIGHTCODE_PROVIDER=echo)")
	}

	// Workspace root base dir
	if wr := os.Getenv("NIGHTCODE_WORKSPACES_ROOT"); wr != "" {
		h.SetWorkspacesRoot(wr)
	}

	mux := http.NewServeMux()

	// SSE endpoint — POST streams RuntimeEvents
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/chats/{chatId}/messages", h.HandleSendMessage)

	// Cancel endpoint — DELETE cancels an in-flight run
	mux.HandleFunc("DELETE /api/workspaces/{workspaceId}/chats/{chatId}/runs/current", h.HandleCancelRun)
	mux.HandleFunc("DELETE /api/workspaces/{workspaceId}/chats/{chatId}/runs/{runId}", h.HandleCancelRun)

	// Models endpoint — lists usable models based on which provider API keys
	// are configured, so the frontend's model picker reflects reality.
	mux.HandleFunc("GET /api/models", h.HandleListModels)

	// Workspace endpoints
	mux.HandleFunc("GET /api/workspaces", h.HandleListWorkspaces)
	mux.HandleFunc("POST /api/workspaces", h.HandleCreateWorkspace)
	mux.HandleFunc("DELETE /api/workspaces/{workspaceId}", h.HandleDeleteWorkspace)

	// History endpoints
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/chats", h.HandleListChats)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/chats/{chatId}/messages", h.HandleListMessages)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/chats/{chatId}/artifacts", h.HandleListArtifacts)

	// Debug tools endpoint — gated behind NIGHTCODE_DEBUG_TOOLS=true
	if os.Getenv("NIGHTCODE_DEBUG_TOOLS") == "true" {
		log.Println("DEBUG: tool debug endpoint enabled at POST /api/debug/tools/:toolName?workspace=<path>")
		mux.HandleFunc("POST /api/debug/tools/{toolName}", makeDebugToolHandler(registry))
		mux.HandleFunc("GET /api/debug/tools", makeDebugToolListHandler(registry))
	}

	handler := corsMiddleware(mux, corsOrigin)

	log.Printf("NightCode backend listening on :%s (cors: %s)", port, corsOrigin)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func makeDebugToolHandler(registry tools.ToolRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		toolName := r.PathValue("toolName")
		workspacePath := r.URL.Query().Get("workspace")
		if workspacePath == "" {
			http.Error(w, `{"error":"missing ?workspace= query parameter"}`, http.StatusBadRequest)
			return
		}

		tool := registry.Get(toolName)
		if tool == nil {
			http.Error(w, fmt.Sprintf(`{"error":"unknown tool: %s"}`, toolName), http.StatusNotFound)
			return
		}

		var input json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}

		// Create workspace from the query parameter path.
		ws, err := tools.NewWorkspace(workspacePath)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid workspace: %v"}`, err), http.StatusBadRequest)
			return
		}

		result, err := tool.Execute(r.Context(), input, ws)
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			w.WriteHeader(http.StatusOK) // Return 200 with error in body for easier debugging
			resp := map[string]any{
				"error": err.Error(),
			}
			var toolErr *tools.ToolError
			if stdErr, ok := err.(*tools.ToolError); ok {
				toolErr = stdErr
			}
			var sbErr *tools.SandboxError
			if stdErr, ok := err.(*tools.SandboxError); ok {
				sbErr = stdErr
			}
			if toolErr != nil {
				resp["errorCode"] = toolErr.Code
			} else if sbErr != nil {
				resp["errorCode"] = sbErr.Code
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"output": json.RawMessage(result.Output),
		}
		if result.Artifact != nil {
			resp["artifact"] = result.Artifact
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func makeDebugToolListHandler(registry tools.ToolRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type toolInfo struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		}
		var toolsList []toolInfo
		for _, name := range registry.Names() {
			t := registry.Get(name)
			toolsList = append(toolsList, toolInfo{
				Name:        t.Name(),
				Description: t.Description(),
				InputSchema: t.InputSchema(),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toolsList)
	}
}

func modelOr(model string) string {
	if model == "" {
		return "gemini-3.6-flash"
	}
	return model
}

func groqModelOr(model string) string {
	if model == "" {
		return "openai/gpt-oss-20b"
	}
	return model
}

func opencodeModelOr(model string) string {
	if model == "" {
		return "mimo-v2.5-free"
	}
	return model
}

func openrouterModelOr(model string) string {
	if model == "" {
		return "google/gemma-4-31b-it:free"
	}
	return model
}

func calabrassModelOr(model string) string {
	if model == "" {
		return "gemini-3.6-flash"
	}
	return model
}

func corsMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin || strings.HasPrefix(origin, "http://localhost") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
