package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodewithMubasher/NightCode/backend/internal/tools"
)

// buildSkillManifest lists all .nightcode/skills/*.md files and extracts a
// lightweight per-skill description. Conventions (documented):
//   - If the file has YAML-ish frontmatter delimited by a leading "---" line,
//     a "name:" and/or "description:" key is preferred.
//   - Otherwise the first "## " heading is used.
//   - Otherwise the first non-empty line is used.
//
// Only the manifest (name + description) is emitted here; full content on-demand
// loading is Step 4+ work.
func buildSkillManifest(ws *tools.Workspace) (string, []string) {
	skillsDirFile, err := ws.ResolvePath(filepath.Join(".nightcode", "skills"))
	if err != nil {
		return "", nil // skills dir outside sandbox or missing — not worth warning
	}

	entries, err := os.ReadDir(skillsDirFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // no skills directory — normal case, not a warning
		}
		return "", []string{"skills dir read error: " + err.Error()}
	}

	var warnings []string
	type skillInfo struct {
		name        string
		description string
	}
	var skills []skillInfo

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		full := filepath.Join(skillsDirFile, e.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %s read error: %v", e.Name(), err))
			continue
		}

		name, desc := extractSkillInfo(e.Name(), string(data))
		skills = append(skills, skillInfo{name: name, description: desc})
	}

	if len(skills) == 0 {
		return "", warnings
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].name < skills[j].name })

	var sb strings.Builder
	for _, s := range skills {
		if s.description != "" {
			fmt.Fprintf(&sb, "- **%s**: %s\n", s.name, s.description)
		} else {
			fmt.Fprintf(&sb, "- **%s**\n", s.name)
		}
	}

	return strings.TrimRight(sb.String(), "\n"), warnings
}

// extractSkillInfo derives a human name and description from a skill markdown file.
func extractSkillInfo(filename, content string) (string, string) {
	name := strings.TrimSuffix(filename, ".md")

	lines := strings.Split(content, "\n")

	// Try frontmatter
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "---" {
				break
			}
			if strings.HasPrefix(line, "name:") {
				if v := strings.TrimSpace(strings.TrimPrefix(line, "name:")); v != "" {
					name = v
				}
			}
		}
	}

	description := ""
	// Frontmatter description
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "---" {
				break
			}
			if strings.HasPrefix(line, "description:") {
				description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			}
		}
	}

	// Fallback: first "## " heading
	if description == "" {
		for _, line := range lines {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "##") {
				description = strings.TrimSpace(strings.TrimPrefix(t, "##"))
				break
			}
		}
	}

	// Fallback: first non-empty line (skip frontmatter)
	if description == "" {
		inFrontmatter := false
		for _, line := range lines {
			t := strings.TrimSpace(line)
			if t == "---" {
				inFrontmatter = !inFrontmatter
				continue
			}
			if !inFrontmatter && t != "" && !strings.HasPrefix(t, "#") {
				description = t
				break
			}
		}
	}

	return name, description
}