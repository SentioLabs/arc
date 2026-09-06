package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

// resolveSessionID honors explicit and Arc identities before native fallbacks.
// Native IDs can be inherited across nested runtimes, so conflicting values
// need an explicit choice instead of silently attributing work to the wrong one.
func resolveSessionID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if id := os.Getenv("ARC_SESSION_ID"); id != "" {
		return id, nil
	}
	codexID, piID := os.Getenv("CODEX_THREAD_ID"), os.Getenv("PI_SESSION_ID")
	if codexID != "" && piID != "" && codexID != piID {
		return "", errors.New("conflicting runtime session IDs: set ARC_SESSION_ID or pass --session-id")
	}
	if piID != "" {
		return piID, nil
	}
	return codexID, nil
}

// resolveClaimSessionID ensures the selected identity is registered in the
// issue's project before updating ownership, regardless of the runtime or hook.
func resolveClaimSessionID(c *client.Client, issueID, explicit string) (string, error) {
	id, err := resolveSessionID(explicit)
	if err != nil || id == "" {
		return id, err
	}

	issue, err := c.GetIssueByID(issueID)
	if err != nil {
		return "", fmt.Errorf("register AI session: resolve issue: %w", err)
	}
	// Reuse an existing hook registration without changing its metadata. A
	// worker may be executing from a different cwd than its registered parent.
	if existing, err := c.GetAISession(issue.ProjectID, id); err == nil {
		return validateClaimSession(&existing.AISession, id, issue.ProjectID)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("register AI session: working directory: %w", err)
	}
	// The issue's project is authoritative. The server validates the actual cwd
	// and preserves existing session metadata on an idempotent registration.
	session, err := c.CreateAISession(issue.ProjectID, &types.AISession{ID: id, CWD: cwd})
	if err != nil {
		return "", fmt.Errorf("register AI session: %w", err)
	}
	return validateClaimSession(session, id, issue.ProjectID)
}

func validateClaimSession(session *types.AISession, id, projectID string) (string, error) {
	// Older servers can return a same-ID session from a different project.
	if session == nil || session.ID != id || session.ProjectID != projectID {
		return "", fmt.Errorf("register AI session: response does not match session %q and issue project %q",
			id, projectID)
	}
	return id, nil
}
