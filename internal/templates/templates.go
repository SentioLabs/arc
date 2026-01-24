package templates

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed *.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "*.tmpl"))

// ClaudeMdReferenceData holds the data for the CLAUDE.md reference template
type ClaudeMdReferenceData struct {
	AgentsFile string
}

// RenderClaudeMdReference renders the CLAUDE.md session completion reference
func RenderClaudeMdReference(data ClaudeMdReferenceData) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "claude_md_reference.tmpl", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderAgentsMd renders the full AGENTS.md content
func RenderAgentsMd() (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "agents_md.tmpl", nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderCodexSkillToml renders the Codex skill.toml content
func RenderCodexSkillToml() (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "codex_skill_toml.tmpl", nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderCodexSkillMd renders the Codex SKILL.md content
func RenderCodexSkillMd() (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "codex_skill_md.tmpl", nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}
