package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/api"
	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

func main() {
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

	mux := http.NewServeMux()

	// SSE endpoint — POST streams RuntimeEvents
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/chats/{chatId}/messages", h.HandleSendMessage)

	// Cancel endpoint — DELETE cancels an in-flight run
	mux.HandleFunc("DELETE /api/workspaces/{workspaceId}/chats/{chatId}/runs/{runId}", h.HandleCancelRun)

	// History endpoints
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/chats", h.HandleListChats)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/chats/{chatId}/messages", h.HandleListMessages)

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
