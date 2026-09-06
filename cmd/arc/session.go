package main

import "os"

// resolveSessionID uses an explicit flag or hook identity first, then Arc's
// environment override, then the Codex thread identity. Codex SessionStart
// hooks register their session_id unchanged, matching CODEX_THREAD_ID in the
// session's commands. This only resolves identity; it does not register a session.
func resolveSessionID(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if id := os.Getenv("ARC_SESSION_ID"); id != "" {
		return id
	}
	return os.Getenv("CODEX_THREAD_ID")
}
