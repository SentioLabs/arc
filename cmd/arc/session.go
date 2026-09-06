package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

var errEmptySessionID = errors.New("--session-id cannot be empty")

// resolveSessionID uses an explicitly supplied session ID or Arc's generic
// environment fallback. Harness integrations are responsible for passing their
// native identity through one of these provider-neutral boundaries.
func resolveSessionID(explicit string, explicitSet bool) (string, error) {
	if explicitSet {
		if explicit == "" {
			return "", errEmptySessionID
		}
		return explicit, nil
	}
	return os.Getenv("ARC_SESSION_ID"), nil
}

// resolveClaimSessionID ensures the selected identity is registered in the
// issue's project before updating ownership, regardless of the runtime or hook.
func resolveClaimSessionID(c *client.Client, issueID, explicit string, explicitSet bool) (string, error) {
	id, err := resolveSessionID(explicit, explicitSet)
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

// resolvePrimeSessionID gives a validated Claude hook identity precedence over
// ARC_SESSION_ID unless prime was given an explicit session ID.
func resolvePrimeSessionID(explicit string, explicitSet bool, hookID string) (string, error) {
	if explicitSet {
		return resolveSessionID(explicit, true)
	}
	if hookID != "" {
		return hookID, nil
	}
	return resolveSessionID("", false)
}

func validateClaimSession(session *types.AISession, id, projectID string) (string, error) {
	// Older servers can return a same-ID session from a different project.
	if session == nil || session.ID != id || session.ProjectID != projectID {
		return "", fmt.Errorf("register AI session: response does not match session %q and issue project %q",
			id, projectID)
	}
	return id, nil
}
