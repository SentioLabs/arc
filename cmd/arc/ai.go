// Package main provides CLI commands for managing AI sessions and agents.
// These commands integrate with Claude Code hooks (SessionStart, PostToolUse)
// to track AI coding sessions and their spawned subagents.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/sentiolabs/arc/internal/types"
	"github.com/spf13/cobra"
)

// postToolUsePayload is the JSON structure sent by Claude Code's PostToolUse hook.
type postToolUsePayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	ToolName       string `json:"tool_name"`
	ToolInput      struct {
		Description  string `json:"description"`
		Prompt       string `json:"prompt"`
		SubagentType string `json:"subagent_type"`
		Model        string `json:"model"`
	} `json:"tool_input"`
	ToolResponse struct {
		Status            string `json:"status"`
		AgentID           string `json:"agentId"`           //nolint:tagliatelle // matches Claude Code payload
		TotalDurationMs   int    `json:"totalDurationMs"`   //nolint:tagliatelle // matches Claude Code payload
		TotalTokens       int    `json:"totalTokens"`       //nolint:tagliatelle // matches Claude Code payload
		TotalToolUseCount int    `json:"totalToolUseCount"` //nolint:tagliatelle // matches Claude Code payload
	} `json:"tool_response"`
}

// parsePostToolUsePayload reads and parses a PostToolUse JSON payload from r.
func parsePostToolUsePayload(r io.Reader) (*postToolUsePayload, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("empty input")
	}

	var p postToolUsePayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	return &p, nil
}

// aiCmd is the parent command for AI session and agent management.
var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "Manage AI sessions and agents",
}

// aiSessionCmd is the parent command for AI session operations.
var aiSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage AI sessions",
}

// aiSessionStartCmd creates a new AI session. It requires --id to identify the
// session and optionally accepts --transcript-path and --cwd. The create is
// idempotent: if the session already exists, the existing record is returned.
//
// With --stdin, reads a Claude Code hook JSON payload from stdin instead of
// flags, extracting session_id, transcript_path, and cwd.
var aiSessionStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new AI session",
	RunE: func(cmd *cobra.Command, args []string) error {
		useStdin, _ := cmd.Flags().GetBool("stdin")

		err := runSessionStart(cmd, useStdin)
		if err != nil && useStdin {
			// Hook mode: log and suppress — never block session startup.
			_, _ = fmt.Fprintf(os.Stderr, "arc: session not tracked: %v\n", err)
			return nil
		}
		return err
	},
}

// runSessionStart contains the session-start logic. It returns an error on any
// failure; the caller decides whether to propagate or suppress it.
func runSessionStart(cmd *cobra.Command, useStdin bool) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	var id, transcriptPath, cwd string

	if useStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		if len(data) == 0 {
			return errors.New("empty stdin payload")
		}
		var input hookInput
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("parse hook payload: %w", err)
		}
		id = input.SessionID
		transcriptPath = input.TranscriptPath
		cwd = input.CWD
	} else {
		id, _ = cmd.Flags().GetString("id")
		transcriptPath, _ = cmd.Flags().GetString("transcript-path")
		cwd, _ = cmd.Flags().GetString("cwd")
	}

	if id == "" {
		return errors.New("--id is required (or session_id in stdin payload)")
	}

	resolvedProjectID, err := resolveFromServer(cwd)
	if err != nil {
		return fmt.Errorf("CWD does not map to a registered arc project: %w", err)
	}

	session := &types.AISession{
		ID:             id,
		TranscriptPath: transcriptPath,
		CWD:            cwd,
	}

	created, err := c.CreateAISession(resolvedProjectID, session)
	if err != nil {
		return err
	}

	if outputJSON {
		outputResult(created)
		return nil
	}

	fmt.Printf("Started session: %s\n", created.ID)
	return nil
}

// aiSessionListCmd lists AI sessions sorted by start time (newest first).
// Supports --json for machine-readable output.
var aiSessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List AI sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		projID, err := getProjectID()
		if err != nil {
			return err
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		sessions, err := c.ListAISessions(projID, defaultListLimit, 0)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(sessions)
			return nil
		}

		if len(sessions) == 0 {
			fmt.Println("No AI sessions found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tSTARTED\tTRANSCRIPT")
		_, _ = fmt.Fprintln(w, "──\t───────\t──────────")
		for _, s := range sessions {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
				s.ID,
				s.StartedAt.Format("2006-01-02 15:04"),
				s.TranscriptPath)
		}
		_ = w.Flush()
		return nil
	},
}

// aiSessionShowCmd displays details for a single AI session, including
// its registered agents. Supports --json for machine-readable output.
var aiSessionShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show AI session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projID, err := getProjectID()
		if err != nil {
			return err
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		session, err := c.GetAISession(projID, args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(session)
			return nil
		}

		fmt.Printf("ID:         %s\n", session.ID)
		fmt.Printf("Started:    %s\n", session.StartedAt.Format("2006-01-02 15:04:05"))
		if session.TranscriptPath != "" {
			fmt.Printf("Transcript: %s\n", session.TranscriptPath)
		}
		if session.CWD != "" {
			fmt.Printf("CWD:        %s\n", session.CWD)
		}

		if len(session.Agents) > 0 {
			fmt.Printf("\nAgents (%d):\n", len(session.Agents))
			for _, a := range session.Agents {
				status := a.Status
				if status == "" {
					status = "unknown"
				}
				fmt.Printf("  %s  %s  %s\n", a.ID, status, a.Description)
			}
		}

		return nil
	},
}

// aiAgentCmd is the parent command for AI agent operations.
var aiAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI agents",
}

// aiAgentRegisterCmd registers a new AI agent from a PostToolUse stdin payload.
// It parses the JSON payload piped via --stdin, extracts agent metadata from
// tool_input/tool_response, and calls the server API. Non-Agent tool payloads
// are silently ignored (exit 0). If the session doesn't exist, the server
// auto-creates it via lazy fallback.
var aiAgentRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register an AI agent from PostToolUse hook payload",
	RunE: func(cmd *cobra.Command, args []string) error {
		useStdin, _ := cmd.Flags().GetBool("stdin")
		if !useStdin {
			return errors.New("--stdin is required")
		}

		payload, err := parsePostToolUsePayload(os.Stdin)
		if err != nil {
			return fmt.Errorf("parse payload: %w", err)
		}

		if payload.ToolName != "Agent" {
			// Not an Agent tool use; silently succeed.
			return nil
		}

		if payload.SessionID == "" {
			return errors.New("session_id is required in payload")
		}

		// Resolve project from payload CWD; silently skip if unresolvable
		resolvedProjectID, err := resolveFromServer(payload.CWD)
		if err != nil {
			return nil
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		durationMs := payload.ToolResponse.TotalDurationMs
		totalTokens := payload.ToolResponse.TotalTokens
		toolUseCount := payload.ToolResponse.TotalToolUseCount

		agent := &types.AIAgent{
			ID:           payload.ToolResponse.AgentID,
			SessionID:    payload.SessionID,
			Description:  payload.ToolInput.Description,
			Prompt:       payload.ToolInput.Prompt,
			AgentType:    payload.ToolInput.SubagentType,
			Model:        payload.ToolInput.Model,
			Status:       payload.ToolResponse.Status,
			DurationMs:   &durationMs,
			TotalTokens:  &totalTokens,
			ToolUseCount: &toolUseCount,
		}

		created, err := c.CreateAIAgent(resolvedProjectID, payload.SessionID, agent)
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(created)
			return nil
		}

		fmt.Printf("Registered agent: %s (session %s)\n", created.ID, created.SessionID)
		return nil
	},
}

// aiAgentShowCmd displays details for a single AI agent including description,
// type, model, status, duration, tokens, and tool use count.
// Requires --session to identify the parent session.
var aiAgentShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show AI agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session")
		if sessionID == "" {
			return errors.New("--session is required")
		}

		projID, err := getProjectID()
		if err != nil {
			return err
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		agent, err := c.GetAIAgent(projID, sessionID, args[0])
		if err != nil {
			return err
		}

		if outputJSON {
			outputResult(agent)
			return nil
		}

		fmt.Printf("ID:          %s\n", agent.ID)
		fmt.Printf("Session:     %s\n", agent.SessionID)
		fmt.Printf("Status:      %s\n", agent.Status)
		if agent.Description != "" {
			fmt.Printf("Description: %s\n", agent.Description)
		}
		if agent.AgentType != "" {
			fmt.Printf("Type:        %s\n", agent.AgentType)
		}
		if agent.Model != "" {
			fmt.Printf("Model:       %s\n", agent.Model)
		}
		if agent.DurationMs != nil {
			fmt.Printf("Duration:    %dms\n", *agent.DurationMs)
		}
		if agent.TotalTokens != nil {
			fmt.Printf("Tokens:      %d\n", *agent.TotalTokens)
		}
		if agent.ToolUseCount != nil {
			fmt.Printf("Tool Uses:   %d\n", *agent.ToolUseCount)
		}

		return nil
	},
}

// aiAgentTranscriptCmd dumps the transcript for an AI agent to stdout.
// The transcript is derived from the session's transcript path on disk.
// Requires --session to identify the parent session.
var aiAgentTranscriptCmd = &cobra.Command{
	Use:   "transcript <id>",
	Short: "Show AI agent transcript",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session")
		if sessionID == "" {
			return errors.New("--session is required")
		}

		projID, err := getProjectID()
		if err != nil {
			return err
		}

		c, err := getClient()
		if err != nil {
			return err
		}

		entries, err := c.GetAgentTranscript(projID, sessionID, args[0])
		if err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		for _, entry := range entries {
			if err := enc.Encode(entry); err != nil {
				return fmt.Errorf("write transcript entry: %w", err)
			}
		}

		return nil
	},
}

func init() {
	// Wire ai command to root
	rootCmd.AddCommand(aiCmd)

	// Wire session subcommands
	aiCmd.AddCommand(aiSessionCmd)
	aiSessionCmd.AddCommand(aiSessionStartCmd)
	aiSessionCmd.AddCommand(aiSessionListCmd)
	aiSessionCmd.AddCommand(aiSessionShowCmd)

	// Wire agent subcommands
	aiCmd.AddCommand(aiAgentCmd)
	aiAgentCmd.AddCommand(aiAgentRegisterCmd)
	aiAgentCmd.AddCommand(aiAgentShowCmd)
	aiAgentCmd.AddCommand(aiAgentTranscriptCmd)

	// Flags
	aiSessionStartCmd.Flags().String("id", "", "Session ID")
	aiSessionStartCmd.Flags().String("transcript-path", "", "Path to transcript file")
	aiSessionStartCmd.Flags().String("cwd", "", "Working directory")
	aiSessionStartCmd.Flags().Bool("stdin", false, "Read SessionStart hook payload from stdin")

	aiAgentRegisterCmd.Flags().Bool("stdin", false, "Read PostToolUse payload from stdin")

	aiAgentShowCmd.Flags().String("session", "", "Session ID")
	aiAgentTranscriptCmd.Flags().String("session", "", "Session ID")
}
