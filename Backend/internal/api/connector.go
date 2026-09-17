package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/store"
	"github.com/google/uuid"
)

type connectorRequest struct {
	Name    string `json:"name"`
	Command string `json:"command"` // full command like: python "C:\path\to\server.py"
}

// parseCommand splits a full command string into executable and args.
// Handles quoted arguments: python "C:\path\to\server.py" --flag
func parseCommand(cmd string) (string, []string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", nil
	}

	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		switch {
		case inQuotes:
			if ch == quoteChar {
				inQuotes = false
			} else {
				current.WriteByte(ch)
			}
		case ch == '"' || ch == '\'':
			inQuotes = true
			quoteChar = ch
		case ch == ' ' || ch == '\t':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if len(parts) == 0 {
		return "", nil
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return parts[0], parts[1:]
}

// GET /api/connectors
func (h *Handler) HandleListConnectors(w http.ResponseWriter, r *http.Request) {
	connectors, err := h.store.GetConnectors()
	if err != nil {
		http.Error(w, `{"error":"failed to list connectors"}`, http.StatusInternalServerError)
		return
	}
	if connectors == nil {
		connectors = []store.ConnectorRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectors)
}

// POST /api/connectors
func (h *Handler) HandleCreateConnector(w http.ResponseWriter, r *http.Request) {
	var req connectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		http.Error(w, `{"error":"command required"}`, http.StatusBadRequest)
		return
	}

	// Parse the full command into executable + args.
	executable, args := parseCommand(req.Command)
	argsJSON, _ := json.Marshal(args)

	id := uuid.New().String()
	if err := h.store.InsertConnector(id, req.Name, "stdio", executable, string(argsJSON), true); err != nil {
		log.Printf("create connector error: %v", err)
		http.Error(w, `{"error":"failed to create connector"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// DELETE /api/connectors/{connectorId}
func (h *Handler) HandleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	connectorID := r.PathValue("connectorId")
	if err := h.store.DeleteConnector(connectorID); err != nil {
		log.Printf("delete connector error: %v", err)
		http.Error(w, `{"error":"failed to delete connector"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// PATCH /api/connectors/{connectorId}/toggle
func (h *Handler) HandleToggleConnector(w http.ResponseWriter, r *http.Request) {
	connectorID := r.PathValue("connectorId")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateConnectorEnabled(connectorID, req.Enabled); err != nil {
		log.Printf("toggle connector error: %v", err)
		http.Error(w, `{"error":"failed to toggle connector"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
