#!/bin/bash
# E2E test orchestration script
# Usage: ./scripts/test-e2e.sh [integration|playwright|all]
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.test.yml"
MODE="${1:-all}"

cleanup() { docker compose -f "$COMPOSE_FILE" down -v 2>/dev/null; }
trap cleanup EXIT

echo "==> Building arc binary..."
"$PROJECT_ROOT/scripts/build.sh"

echo "==> Starting test server..."
docker compose -f "$COMPOSE_FILE" up -d --build --wait

case "$MODE" in
  integration)
    echo "==> Running Go integration tests..."
    cd "$PROJECT_ROOT"
    ARC_BINARY="$PROJECT_ROOT/bin/arc" go test -tags integration -v -count=1 ./tests/integration/...
    ;;
  playwright)
    echo "==> Running Playwright E2E tests..."
    cd "$PROJECT_ROOT/web"
    bun playwright test --config playwright.e2e.config.ts
    ;;
  all)
    echo "==> Running Go integration tests..."
    cd "$PROJECT_ROOT"
    ARC_BINARY="$PROJECT_ROOT/bin/arc" go test -tags integration -v -count=1 ./tests/integration/...
    echo "==> Running Playwright E2E tests..."
    cd "$PROJECT_ROOT/web"
    bun playwright test --config playwright.e2e.config.ts
    ;;
  *)
    echo "Usage: $0 [integration|playwright|all]"
    exit 1
    ;;
esac

echo "==> All tests passed!"
