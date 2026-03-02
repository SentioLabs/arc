#!/usr/bin/env bash
# test-agentic-teams.sh - Synthetic E2E test for the agentic teams pipeline.
#
# Validates the full workflow: workspace creation, epic with children (--parent),
# teammate labels, team context, prime role filtering, ready state, and lifecycle.
#
# Prerequisites:
#   - arc server running on localhost:7432
#   - arc CLI in PATH

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────

ARC_SERVER="${ARC_SERVER:-http://localhost:7432}"
API="${ARC_SERVER}/api/v1"
PASS_COUNT=0
FAIL_COUNT=0
WS_ID=""

# ── Helpers ────────────────────────────────────────────────────────────────────

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  echo "  PASS: $1"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  echo "  FAIL: $1"
  if [ -n "${2:-}" ]; then
    echo "        $2"
  fi
}

summary() {
  echo ""
  echo "══════════════════════════════════════════"
  echo "  Results: ${PASS_COUNT} passed, ${FAIL_COUNT} failed"
  echo "══════════════════════════════════════════"
  if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
  fi
  exit 0
}

cleanup() {
  echo ""
  echo "── Cleanup ──────────────────────────────"
  if [ -n "$WS_ID" ]; then
    if arc workspace delete "$WS_ID" >/dev/null 2>&1; then
      echo "  Deleted test workspace $WS_ID"
    else
      echo "  Warning: could not delete workspace $WS_ID"
    fi
  fi
  # Clean up labels we created (global, not workspace-scoped)
  curl -sf -X DELETE "${API}/labels/teammate:backend" >/dev/null 2>&1 || true
  curl -sf -X DELETE "${API}/labels/teammate:tests" >/dev/null 2>&1 || true
  echo "  Cleaned up teammate labels"
}

# Always clean up, even on failure
trap cleanup EXIT

# ── Pre-flight checks ─────────────────────────────────────────────────────────

echo "── Pre-flight checks ──────────────────────"

# Check arc CLI is available
if ! command -v arc >/dev/null 2>&1; then
  echo "FATAL: arc CLI not found in PATH"
  exit 2
fi
pass "arc CLI found in PATH"

# Check server is reachable
if ! curl -sf "${API}/workspaces" >/dev/null 2>&1; then
  echo "FATAL: arc server not reachable at ${ARC_SERVER}"
  echo "       Start it with: arc server start"
  exit 2
fi
pass "arc server reachable at ${ARC_SERVER}"

# ── Step 1: Create temporary test workspace ────────────────────────────────────

echo ""
echo "── Step 1: Create test workspace ──────────"

WS_JSON=$(arc workspace create "e2e-agentic-teams-test" --json 2>&1)
WS_ID=$(echo "$WS_JSON" | jq -r '.id')

if [ -z "$WS_ID" ] || [ "$WS_ID" = "null" ]; then
  echo "FATAL: Could not create test workspace"
  echo "       Response: $WS_JSON"
  exit 2
fi
pass "Created workspace: $WS_ID"

# ── Step 2: Create teammate labels ────────────────────────────────────────────

echo ""
echo "── Step 2: Create teammate labels ─────────"

# Labels are global (not workspace-scoped), created via REST API.
# Ignore errors if they already exist.
LABEL_BACKEND=$(curl -sf -X POST "${API}/labels" \
  -H "Content-Type: application/json" \
  -d '{"name":"teammate:backend","color":"#4A90D9","description":"Backend teammate"}' 2>&1) || true

LABEL_TESTS=$(curl -sf -X POST "${API}/labels" \
  -H "Content-Type: application/json" \
  -d '{"name":"teammate:tests","color":"#50C878","description":"Tests teammate"}' 2>&1) || true

# Verify labels exist
LABELS_LIST=$(curl -sf "${API}/labels" 2>&1)
if echo "$LABELS_LIST" | jq -e '.[] | select(.name == "teammate:backend")' >/dev/null 2>&1; then
  pass "Label teammate:backend exists"
else
  fail "Label teammate:backend not found" "$LABELS_LIST"
fi

if echo "$LABELS_LIST" | jq -e '.[] | select(.name == "teammate:tests")' >/dev/null 2>&1; then
  pass "Label teammate:tests exists"
else
  fail "Label teammate:tests not found" "$LABELS_LIST"
fi

# ── Step 3: Create epic with plan ─────────────────────────────────────────────

echo ""
echo "── Step 3: Create epic with plan ──────────"

EPIC_JSON=$(arc create "Calculator API" --type=epic --priority=1 -w "$WS_ID" --json 2>&1)
EPIC_ID=$(echo "$EPIC_JSON" | jq -r '.id')

if [ -z "$EPIC_ID" ] || [ "$EPIC_ID" = "null" ]; then
  fail "Could not create epic" "$EPIC_JSON"
else
  pass "Created epic: $EPIC_ID"
fi

# Set a plan on the epic
PLAN_TEXT="Build a calculator API with add/subtract/multiply/divide endpoints. Backend implements the API server. Tests teammate writes integration tests."
echo "$PLAN_TEXT" | arc plan set "$EPIC_ID" --stdin -w "$WS_ID" 2>&1
PLAN_OUT=$(arc plan show "$EPIC_ID" -w "$WS_ID" 2>&1)

if echo "$PLAN_OUT" | grep -q "calculator" 2>/dev/null; then
  pass "Epic plan set successfully"
else
  fail "Epic plan not found" "$PLAN_OUT"
fi

# ── Step 4: Create child tasks with teammate labels ───────────────────────────

echo ""
echo "── Step 4: Create child tasks ─────────────"

# Child 1: Backend task
BACKEND_JSON=$(arc create "Implement calculator endpoints" --type=task --parent="$EPIC_ID" -w "$WS_ID" --json 2>&1)
BACKEND_ID=$(echo "$BACKEND_JSON" | jq -r '.id')

if [ -z "$BACKEND_ID" ] || [ "$BACKEND_ID" = "null" ]; then
  fail "Could not create backend task" "$BACKEND_JSON"
else
  pass "Created backend task: $BACKEND_ID"
fi

# Child 2: Tests task
TESTS_JSON=$(arc create "Write integration tests" --type=task --parent="$EPIC_ID" -w "$WS_ID" --json 2>&1)
TESTS_ID=$(echo "$TESTS_JSON" | jq -r '.id')

if [ -z "$TESTS_ID" ] || [ "$TESTS_ID" = "null" ]; then
  fail "Could not create tests task" "$TESTS_JSON"
else
  pass "Created tests task: $TESTS_ID"
fi

# Add teammate labels to issues via REST API
curl -sf -X POST "${API}/workspaces/${WS_ID}/issues/${BACKEND_ID}/labels" \
  -H "Content-Type: application/json" \
  -d '{"label":"teammate:backend"}' >/dev/null 2>&1
pass "Added teammate:backend label to $BACKEND_ID"

curl -sf -X POST "${API}/workspaces/${WS_ID}/issues/${TESTS_ID}/labels" \
  -H "Content-Type: application/json" \
  -d '{"label":"teammate:tests"}' >/dev/null 2>&1
pass "Added teammate:tests label to $TESTS_ID"

# ── Step 5: Set dependency (tests blocked by backend) ─────────────────────────

echo ""
echo "── Step 5: Set dependency ─────────────────"

arc dep add "$TESTS_ID" "$BACKEND_ID" --type=blocks -w "$WS_ID" 2>&1
SHOW_TESTS=$(arc show "$TESTS_ID" -w "$WS_ID" --json 2>&1)

if echo "$SHOW_TESTS" | jq -e '.dependencies[]? | select(.depends_on_id == "'"$BACKEND_ID"'" and .type == "blocks")' >/dev/null 2>&1; then
  pass "Dependency set: $TESTS_ID blocked by $BACKEND_ID"
else
  fail "Dependency not found on $TESTS_ID" "$SHOW_TESTS"
fi

# ── Step 6: Verify arc list --parent returns only children ────────────────────

echo ""
echo "── Step 6: Verify list --parent ───────────"

CHILDREN_JSON=$(arc list --parent="$EPIC_ID" -w "$WS_ID" --json 2>&1)
CHILDREN_COUNT=$(echo "$CHILDREN_JSON" | jq 'length')
CHILDREN_IDS=$(echo "$CHILDREN_JSON" | jq -r '.[].id' | sort)

EXPECTED_IDS=$(printf "%s\n%s" "$BACKEND_ID" "$TESTS_ID" | sort)

if [ "$CHILDREN_COUNT" -eq 2 ]; then
  pass "list --parent returned exactly 2 children"
else
  fail "list --parent returned $CHILDREN_COUNT children, expected 2" "$CHILDREN_JSON"
fi

if [ "$CHILDREN_IDS" = "$EXPECTED_IDS" ]; then
  pass "list --parent returned correct child IDs"
else
  fail "list --parent returned wrong IDs" "got: $CHILDREN_IDS, expected: $EXPECTED_IDS"
fi

# ── Step 7: Verify team context groups by role ────────────────────────────────

echo ""
echo "── Step 7: Verify team context ────────────"

# Use the API endpoint directly for team context verification.
# The server-side endpoint batch-fetches labels correctly, while the CLI
# implementation has a known issue where individually-fetched children
# via GetIssue don't include labels (tracked separately).
TEAM_JSON=$(curl -sf "${API}/workspaces/${WS_ID}/team-context?epic_id=${EPIC_ID}" 2>&1)

# Check epic info is present
TEAM_EPIC_ID=$(echo "$TEAM_JSON" | jq -r '.epic.id')
if [ "$TEAM_EPIC_ID" = "$EPIC_ID" ]; then
  pass "team context includes epic info"
else
  fail "team context missing epic" "$TEAM_JSON"
fi

# Check backend role exists with our issue
BACKEND_ROLE_COUNT=$(echo "$TEAM_JSON" | jq '.roles.backend.issues | length')
if [ "$BACKEND_ROLE_COUNT" -eq 1 ]; then
  pass "team context has 1 issue under backend role"
else
  fail "team context backend role has $BACKEND_ROLE_COUNT issues, expected 1" "$TEAM_JSON"
fi

BACKEND_ROLE_ID=$(echo "$TEAM_JSON" | jq -r '.roles.backend.issues[0].id')
if [ "$BACKEND_ROLE_ID" = "$BACKEND_ID" ]; then
  pass "team context backend role has correct issue ID"
else
  fail "team context backend role has wrong ID: $BACKEND_ROLE_ID" ""
fi

# Check tests role exists with our issue
TESTS_ROLE_COUNT=$(echo "$TEAM_JSON" | jq '.roles.tests.issues | length')
if [ "$TESTS_ROLE_COUNT" -eq 1 ]; then
  pass "team context has 1 issue under tests role"
else
  fail "team context tests role has $TESTS_ROLE_COUNT issues, expected 1" "$TEAM_JSON"
fi

TESTS_ROLE_ID=$(echo "$TEAM_JSON" | jq -r '.roles.tests.issues[0].id')
if [ "$TESTS_ROLE_ID" = "$TESTS_ID" ]; then
  pass "team context tests role has correct issue ID"
else
  fail "team context tests role has wrong ID: $TESTS_ROLE_ID" ""
fi

# ── Step 8: Verify prime --role produces teammate-filtered output ─────────────

echo ""
echo "── Step 8: Verify prime --role output ─────"

# prime requires .arc.json in cwd; create a temp dir to simulate
TMPDIR_PRIME=$(mktemp -d)
echo '{}' > "${TMPDIR_PRIME}/.arc.json"

PRIME_OUT=$(cd "$TMPDIR_PRIME" && ARC_TEAMMATE_ROLE=backend arc prime -w "$WS_ID" 2>&1)
rm -rf "$TMPDIR_PRIME"

if echo "$PRIME_OUT" | grep -qi "backend"; then
  pass "prime with role=backend mentions backend"
else
  fail "prime with role=backend does not mention backend" "$PRIME_OUT"
fi

if echo "$PRIME_OUT" | grep -qi "teammate"; then
  pass "prime with role=backend mentions teammate context"
else
  fail "prime with role=backend missing teammate context" "$PRIME_OUT"
fi

# ── Step 9: Verify ready state ────────────────────────────────────────────────

echo ""
echo "── Step 9: Verify ready state ─────────────"

READY_JSON=$(arc ready -w "$WS_ID" --json 2>&1)

# Backend task should be ready (no blockers)
if echo "$READY_JSON" | jq -e '.[] | select(.id == "'"$BACKEND_ID"'")' >/dev/null 2>&1; then
  pass "Backend task ($BACKEND_ID) is ready"
else
  fail "Backend task ($BACKEND_ID) not in ready list" "$READY_JSON"
fi

# Tests task should NOT be ready (blocked by backend)
if echo "$READY_JSON" | jq -e '.[] | select(.id == "'"$TESTS_ID"'")' >/dev/null 2>&1; then
  fail "Tests task ($TESTS_ID) should NOT be ready (blocked)" "$READY_JSON"
else
  pass "Tests task ($TESTS_ID) correctly NOT in ready list (blocked)"
fi

# ── Step 10: Test lifecycle ───────────────────────────────────────────────────

echo ""
echo "── Step 10: Test lifecycle ────────────────"

# Close the backend task
arc close "$BACKEND_ID" -r "Implementation complete" -w "$WS_ID" >/dev/null 2>&1
BACKEND_STATUS=$(arc show "$BACKEND_ID" -w "$WS_ID" --json 2>&1 | jq -r '.status')

if [ "$BACKEND_STATUS" = "closed" ]; then
  pass "Backend task closed successfully"
else
  fail "Backend task status is '$BACKEND_STATUS', expected 'closed'" ""
fi

# Now the tests task should be unblocked and appear in ready
READY_AFTER=$(arc ready -w "$WS_ID" --json 2>&1)

if echo "$READY_AFTER" | jq -e '.[] | select(.id == "'"$TESTS_ID"'")' >/dev/null 2>&1; then
  pass "Tests task ($TESTS_ID) now ready after backend closed"
else
  fail "Tests task ($TESTS_ID) still not ready after backend closed" "$READY_AFTER"
fi

# Verify team context shows the closed backend task with status=closed.
# The epic-scoped team context intentionally includes all children
# (including closed ones) so the team lead can see full progress.
TEAM_AFTER=$(curl -sf "${API}/workspaces/${WS_ID}/team-context?epic_id=${EPIC_ID}" 2>&1)
BACKEND_AFTER_STATUS=$(echo "$TEAM_AFTER" | jq -r '.roles.backend.issues[0].status' 2>/dev/null || echo "missing")

if [ "$BACKEND_AFTER_STATUS" = "closed" ]; then
  pass "Closed backend task shows status=closed in team context"
else
  fail "Backend task status in team context is '$BACKEND_AFTER_STATUS', expected 'closed'" "$TEAM_AFTER"
fi

# ── Summary ───────────────────────────────────────────────────────────────────

summary
