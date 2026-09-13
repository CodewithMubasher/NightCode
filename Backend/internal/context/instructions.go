package context

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

// buildInstructions reads .nightcode/instructions.md verbatim if present.
// Missing file is non-fatal: returns empty prompt + a warning so the absence
// is visible rather than silently swallowed.
func buildInstructions(ws *tools.Workspace) (string, []string) {
	resolved, err := ws.ResolvePath(filepath.Join(".nightcode", "instructions.md"))
	if err != nil {
		// Sandbox violation or unresolvable path — treat as missing.
		return "", []string{"instructions.md readable but unresolved: " + err.Error()}
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", []string{".nightcode/instructions.md not found"}
		}
		return "", []string{"instructions.md read error: " + err.Error()}
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", []string{".nightcode/instructions.md is empty"}
	}

	return content, nil
}