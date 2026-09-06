package main

import (
	"errors"
	"os"
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
