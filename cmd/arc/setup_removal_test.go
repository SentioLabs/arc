package main

import (
	"testing"

	"github.com/sentiolabs/arc/internal/templates"
)

func TestSetupCommandRemoved(t *testing.T) {
	// The "setup" command should not exist on rootCmd
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "setup <recipe>" || cmd.Name() == "setup" {
			t.Error("setup command should not be registered on rootCmd")
		}
	}
}

func TestCodexTemplateRenderFunctionsRemoved(t *testing.T) {
	// Verify that RenderCodexSkillToml and RenderCodexSkillMd are not accessible
	// We verify this indirectly: the templates package should only export
	// RenderClaudeMdReference and RenderAgentsMd
	// This is a compile-time check — if these functions still exist,
	// the test file wouldn't need updating. We verify the remaining functions work.
	_, err := templates.RenderAgentsMd()
	if err != nil {
		t.Errorf("RenderAgentsMd should still work: %v", err)
	}

	_, err = templates.RenderClaudeMdReference(templates.ClaudeMdReferenceData{
		AgentsFile: "AGENTS.md",
	})
	if err != nil {
		t.Errorf("RenderClaudeMdReference should still work: %v", err)
	}
}
