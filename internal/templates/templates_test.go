package templates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sentiolabs/arc/internal/templates"
)

func TestRenderClaudeMdReference(t *testing.T) {
	tests := []struct {
		name     string
		data     templates.ClaudeMdReferenceData
		contains []string
	}{
		{
			name: "default agents file",
			data: templates.ClaudeMdReferenceData{
				AgentsFile: "AGENTS.md",
			},
			contains: []string{
				"## Session Completion",
				"AGENTS.md",
				"Landing the Plane",
				"#landing-the-plane-session-completion",
			},
		},
		{
			name: "custom agents file",
			data: templates.ClaudeMdReferenceData{
				AgentsFile: "CUSTOM_AGENTS.md",
			},
			contains: []string{
				"CUSTOM_AGENTS.md",
				"## Session Completion",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := templates.RenderClaudeMdReference(tt.data)
			require.NoError(t, err)

			for _, want := range tt.contains {
				assert.Contains(t, result, want,
					"result should contain %q\ngot: %s", want, result)
			}
		})
	}
}

func TestRenderClaudeMdReference_Format(t *testing.T) {
	result, err := templates.RenderClaudeMdReference(templates.ClaudeMdReferenceData{
		AgentsFile: "AGENTS.md",
	})
	require.NoError(t, err)

	// Should start with markdown header
	assert.Contains(t, result, "## Session Completion",
		"should contain '## Session Completion'\ngot: %s", result)

	// Should contain a markdown link
	assert.Contains(t, result, "[AGENTS.md",
		"should contain markdown link")
}

func TestRenderAgentsMd(t *testing.T) {
	result, err := templates.RenderAgentsMd()
	require.NoError(t, err)

	// Should contain key sections
	expectedContents := []string{
		"# Agent Instructions",
		"## Quick Reference",
		"## Landing the Plane",
		"arc ready",
		"arc close",
		"--reason",
		"git push",
		"CRITICAL RULES",
	}

	for _, want := range expectedContents {
		assert.Contains(t, result, want, "AGENTS.md should contain %q", want)
	}
}
