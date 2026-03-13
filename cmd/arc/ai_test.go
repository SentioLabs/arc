package main

import (
	"strings"
	"testing"

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
		assert.Equal(t, "", p.TranscriptPath)
		assert.Equal(t, "", p.ToolInput.Description)
		assert.Equal(t, 0, p.ToolResponse.TotalDurationMs)
	})

	t.Run("non-Agent tool_name parses successfully", func(t *testing.T) {
		input := `{"session_id": "s2", "tool_name": "Bash", "tool_input": {}, "tool_response": {}}`
		p, err := parsePostToolUsePayload(strings.NewReader(input))
		require.NoError(t, err)
		assert.Equal(t, "Bash", p.ToolName)
	})
}

func TestAiCmdTree(t *testing.T) {
	t.Run("ai command exists on root", func(t *testing.T) {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "ai" {
				found = true
				break
			}
		}
		assert.True(t, found, "rootCmd should have an 'ai' subcommand")
	})

	t.Run("ai has session subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiCmd.Commands() {
			if cmd.Name() == "session" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiCmd should have a 'session' subcommand")
	})

	t.Run("ai has agent subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiCmd.Commands() {
			if cmd.Name() == "agent" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiCmd should have an 'agent' subcommand")
	})

	t.Run("session has start subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiSessionCmd.Commands() {
			if cmd.Name() == "start" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiSessionCmd should have a 'start' subcommand")
	})

	t.Run("session has list subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiSessionCmd.Commands() {
			if cmd.Name() == "list" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiSessionCmd should have a 'list' subcommand")
	})

	t.Run("session has show subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiSessionCmd.Commands() {
			if cmd.Name() == "show" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiSessionCmd should have a 'show' subcommand")
	})

	t.Run("agent has register subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiAgentCmd.Commands() {
			if cmd.Name() == "register" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiAgentCmd should have a 'register' subcommand")
	})

	t.Run("agent has show subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiAgentCmd.Commands() {
			if cmd.Name() == "show" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiAgentCmd should have a 'show' subcommand")
	})

	t.Run("agent has transcript subcommand", func(t *testing.T) {
		found := false
		for _, cmd := range aiAgentCmd.Commands() {
			if cmd.Name() == "transcript" {
				found = true
				break
			}
		}
		assert.True(t, found, "aiAgentCmd should have a 'transcript' subcommand")
	})
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
