package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

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
		ws, _ = tools.NewWorkspace(".")
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
	json.NewEncoder(w).Encode(summary)
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
	count := 0
	for i := 0; i < len(content); {
		idx := indexOf(content[i:], reversal.OldString)
		if idx < 0 {
			break
		}
		count++
		i += idx + len(reversal.OldString)
	}

	if count == 0 {
		// The new_string may have already been edited — try swapping
		count2 := 0
		for i := 0; i < len(content); {
			idx := indexOf(content[i:], reversal.NewString)
			if idx < 0 {
				break
			}
			count2++
			i += idx + len(reversal.NewString)
		}
		if count2 == 1 {
			newContent := replaceOnce(content, reversal.NewString, reversal.OldString)
			if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
				return "error"
			}
			return "ok"
		}
		return "error"
	}
	if count > 1 {
		return "error"
	}

	newContent := replaceOnce(content, reversal.OldString, reversal.NewString)
	if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
		return "error"
	}
	return "ok"
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func replaceOnce(s, old, new string) string {
	idx := indexOf(s, old)
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}
