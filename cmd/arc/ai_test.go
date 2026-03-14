package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePostToolUsePayload(t *testing.T) {
	t.Run("valid full payload", func(t *testing.T) {
		input := `{
			"session_id": "test-uuid",
			"transcript_path": "/tmp/test.jsonl",
			"cwd": "/tmp",
			"tool_name": "Agent",
			"tool_input": {
				"description": "Check Go version",
				"prompt": "Run go version",
				"subagent_type": "general-purpose",
				"model": "haiku"
			},
			"tool_response": {
				"status": "completed",
				"agentId": "abc123",
				"totalDurationMs": 3919,
				"totalTokens": 44763,
				"totalToolUseCount": 1
			}
		}`
		p, err := parsePostToolUsePayload(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "test-uuid", p.SessionID)
		assert.Equal(t, "/tmp/test.jsonl", p.TranscriptPath)
		assert.Equal(t, "/tmp", p.CWD)
		assert.Equal(t, "Agent", p.ToolName)
		assert.Equal(t, "Check Go version", p.ToolInput.Description)
		assert.Equal(t, "Run go version", p.ToolInput.Prompt)
		assert.Equal(t, "general-purpose", p.ToolInput.SubagentType)
		assert.Equal(t, "haiku", p.ToolInput.Model)
		assert.Equal(t, "completed", p.ToolResponse.Status)
		assert.Equal(t, "abc123", p.ToolResponse.AgentID)
		assert.Equal(t, 3919, p.ToolResponse.TotalDurationMs)
		assert.Equal(t, 44763, p.ToolResponse.TotalTokens)
		assert.Equal(t, 1, p.ToolResponse.TotalToolUseCount)
	})

	t.Run("empty stdin", func(t *testing.T) {
		_, err := parsePostToolUsePayload(strings.NewReader(""))
		assert.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parsePostToolUsePayload(strings.NewReader("not json at all"))
		assert.Error(t, err)
	})

	t.Run("missing fields uses zero values", func(t *testing.T) {
		input := `{"session_id": "s1", "tool_name": "Agent"}`
		p, err := parsePostToolUsePayload(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "s1", p.SessionID)
		assert.Equal(t, "Agent", p.ToolName)
		assert.Empty(t, p.TranscriptPath)
		assert.Empty(t, p.ToolInput.Description)
		assert.Equal(t, 0, p.ToolResponse.TotalDurationMs)
	})

	t.Run("non-Agent tool_name parses successfully", func(t *testing.T) {
		input := `{"session_id": "s2", "tool_name": "Bash", "tool_input": {}, "tool_response": {}}`
		p, err := parsePostToolUsePayload(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "Bash", p.ToolName)
	})
}

// hasSubcommand checks whether parent has a subcommand with the given name.
func hasSubcommand(parent *cobra.Command, name string) bool {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return true
		}
	}
	return false
}

func TestAiCmdTree(t *testing.T) {
	assert.True(t, hasSubcommand(rootCmd, "ai"), "rootCmd should have an 'ai' subcommand")
	assert.True(t, hasSubcommand(aiCmd, "session"), "aiCmd should have a 'session' subcommand")
	assert.True(t, hasSubcommand(aiCmd, "agent"), "aiCmd should have an 'agent' subcommand")
	assert.True(t, hasSubcommand(aiSessionCmd, "start"), "aiSessionCmd should have a 'start' subcommand")
	assert.True(t, hasSubcommand(aiSessionCmd, "list"), "aiSessionCmd should have a 'list' subcommand")
	assert.True(t, hasSubcommand(aiSessionCmd, "show"), "aiSessionCmd should have a 'show' subcommand")
	assert.True(t, hasSubcommand(aiAgentCmd, "register"), "aiAgentCmd should have a 'register' subcommand")
	assert.True(t, hasSubcommand(aiAgentCmd, "show"), "aiAgentCmd should have a 'show' subcommand")
	assert.True(t, hasSubcommand(aiAgentCmd, "transcript"), "aiAgentCmd should have a 'transcript' subcommand")
}

func TestAiSessionStartFlags(t *testing.T) {
	flag := aiSessionStartCmd.Flags().Lookup("id")
	require.NotNil(t, flag, "--id flag should exist on session start")

	flag = aiSessionStartCmd.Flags().Lookup("transcript-path")
	require.NotNil(t, flag, "--transcript-path flag should exist on session start")
}

func TestAiAgentRegisterStdinFlag(t *testing.T) {
	flag := aiAgentRegisterCmd.Flags().Lookup("stdin")
	require.NotNil(t, flag, "--stdin flag should exist on agent register")
}
