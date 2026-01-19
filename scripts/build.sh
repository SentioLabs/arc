#!/bin/bash
# Build script for Arc - builds frontend and embeds in Go binary
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Building Arc ==="

# Build frontend
echo "Building frontend..."
cd "$PROJECT_ROOT/web"
bun install --frozen-lockfile
bun run build

# Build Go binary
echo "Building Go binary..."
cd "$PROJECT_ROOT"
go build -o arc-server ./cmd/server
go build -o arc ./cmd/arc

echo ""
echo "Build complete!"
echo "  Server binary: arc-server"
echo "  CLI binary:    arc"
echo ""
echo "Run with: ./arc-server"
