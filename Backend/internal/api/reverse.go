package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
	"github.com/CodewithMubasher/NightCode/backend/internal/types"
)

type reverseSummary struct {
	Reversed []reverseAction `json:"reversed"`
	Skipped  []reverseAction `json:"skipped"`
}

type reverseAction struct {
	ToolName string `json:"toolName"`
	Path     string `json:"path"`
	Status   string `json:"status"`
}

// HandleReverseMessage reverses all file changes from an assistant message
// and deletes the message. POST /api/workspaces/{workspaceId}/chats/{chatId}/messages/{messageId}/reverse
func (h *Handler) HandleReverseMessage(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatId")
	messageID := r.PathValue("messageId")

	msg, err := h.store.GetMessage(messageID)
	if err != nil {
		http.Error(w, `{"error":"message not found"}`, http.StatusNotFound)
		return
	}
	if msg.ChatID != chatID {
		http.Error(w, `{"error":"message does not belong to chat"}`, http.StatusBadRequest)
		return
	}

	var segments []types.Segment
	if err := json.Unmarshal(msg.Segments, &segments); err != nil {
		http.Error(w, `{"error":"invalid segments"}`, http.StatusInternalServerError)
		return
	}

	workspaceID := r.PathValue("workspaceId")
	workspacePath := h.resolveWorkspacePath(workspaceID)

	ws, wsErr := tools.NewWorkspace(workspacePath)
	if wsErr != nil {
		// Don't silently fall back to the process CWD — reversing files
		// outside the workspace (or nil-dereferencing a failed fallback)
		// is worse than returning an error.
		log.Printf("reverse: workspace resolve error: %v", wsErr)
		http.Error(w, `{"error":"failed to resolve workspace"}`, http.StatusInternalServerError)
		return
	}

	var summary reverseSummary

	// Process tool-group segments in reverse order
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if seg.Type != "tool-group" {
			continue
		}
		// Process calls within each group in reverse order
		for j := len(seg.Calls) - 1; j >= 0; j-- {
			call := seg.Calls[j]
			if call.Status != "completed" || len(call.ReversalData) == 0 {
				continue
			}

			action := reverseAction{ToolName: call.Name}

			switch call.Name {
			case "write_file":
				action.Status = h.reverseWriteFile(call.ReversalData, ws)
			case "edit_file":
				action.Status = h.reverseEditFile(call.ReversalData, ws)
			default:
				action.Status = "skipped"
				summary.Skipped = append(summary.Skipped, reverseAction{ToolName: call.Name, Status: "not_undoable"})
				continue
			}

			// Extract path from reversal data
			var pathData struct {
				Path string `json:"path"`
			}
			if json.Unmarshal(call.ReversalData, &pathData) == nil {
				action.Path = pathData.Path
			}

			if action.Status == "ok" {
				summary.Reversed = append(summary.Reversed, action)
			} else {
				summary.Skipped = append(summary.Skipped, action)
			}
		}
	}

	// Delete the message
	if err := h.store.DeleteMessage(messageID); err != nil {
		log.Printf("reverse: delete message error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, summary)
}

func (h *Handler) reverseWriteFile(data json.RawMessage, ws *tools.Workspace) string {
	var reversal struct {
		OldContent string `json:"oldContent"`
		NewFile    bool   `json:"newFile"`
		Path       string `json:"path"`
	}
	if err := json.Unmarshal(data, &reversal); err != nil {
		return "error"
	}

	resolved, err := ws.ResolvePath(reversal.Path)
	if err != nil {
		return "error"
	}

	if reversal.NewFile {
		// File didn't exist before — delete it
		if err := os.Remove(resolved); err != nil && !os.IsNotExist(err) {
			log.Printf("reverse: delete new file error: %v", err)
			return "error"
		}
		return "ok"
	}

	// Restore old content
	if err := os.WriteFile(resolved, []byte(reversal.OldContent), 0o644); err != nil {
		log.Printf("reverse: restore file error: %v", err)
		return "error"
	}
	return "ok"
}

func (h *Handler) reverseEditFile(data json.RawMessage, ws *tools.Workspace) string {
	var reversal struct {
		Path      string `json:"path"`
		OldString string `json:"oldString"`
		NewString string `json:"newString"`
	}
	if err := json.Unmarshal(data, &reversal); err != nil {
		return "error"
	}

	resolved, err := ws.ResolvePath(reversal.Path)
	if err != nil {
		return "error"
	}

	fileData, err := os.ReadFile(resolved)
	if err != nil {
		log.Printf("reverse: read file error: %v", err)
		return "error"
	}

	content := string(fileData)
	count := strings.Count(content, reversal.OldString)

	if count == 0 {
		// The new_string may have already been edited — try swapping
		count2 := strings.Count(content, reversal.NewString)
		if count2 == 1 {
			newContent := strings.Replace(content, reversal.NewString, reversal.OldString, 1)
			if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
				log.Printf("reverse: write (swap) error: %v", err)
				return "error"
			}
			return "ok"
		}
		log.Printf("reverse: old_string not found (%d) and new_string count=%d in %s", count, count2, reversal.Path)
		return "error"
	}
	if count > 1 {
		log.Printf("reverse: old_string appears %d times (expected 1) in %s", count, reversal.Path)
		return "error"
	}

	newContent := strings.Replace(content, reversal.OldString, reversal.NewString, 1)
	if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
		log.Printf("reverse: write error: %v", err)
		return "error"
	}
	return "ok"
}
