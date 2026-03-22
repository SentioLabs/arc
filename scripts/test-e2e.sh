#!/bin/bash
# E2E test orchestration script
# Usage: ./scripts/test-e2e.sh [integration|playwright|all]
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.test.yml"
MODE="${1:-all}"

cleanup() { docker compose -f "$COMPOSE_FILE" --profile integration --profile playwright down -v 2>/dev/null; }
trap cleanup EXIT

case "$MODE" in
  integration)
    echo "==> Running self-contained integration tests in Docker..."
    docker compose -f "$COMPOSE_FILE" --profile integration up --build --exit-code-from integration-tests
    ;;
  playwright)
    echo "==> Starting test server for Playwright..."
    docker compose -f "$COMPOSE_FILE" --profile playwright up -d --build --wait
    echo "==> Ensuring Playwright browsers are installed..."
    cd "$PROJECT_ROOT/web"
    bunx playwright install --with-deps chromium
    echo "==> Running Playwright E2E tests..."
    bun playwright test --config playwright.e2e.config.ts
    ;;
  all)
    echo "==> Running self-contained integration tests in Docker..."
    docker compose -f "$COMPOSE_FILE" --profile integration up --build --exit-code-from integration-tests

    echo "==> Starting test server for Playwright..."
    docker compose -f "$COMPOSE_FILE" --profile playwright up -d --build --wait
    echo "==> Ensuring Playwright browsers are installed..."
    cd "$PROJECT_ROOT/web"
    bunx playwright install --with-deps chromium
    echo "==> Running Playwright E2E tests..."
    bun playwright test --config playwright.e2e.config.ts
    ;;
  *)
    echo "Usage: $0 [integration|playwright|all]"
    exit 1
    ;;
esac

echo "==> All tests passed!"
