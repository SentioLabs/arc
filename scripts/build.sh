#!/bin/bash
# Build script for Arc binary
# Usage: ./build.sh [-o GOOS] [-a GOARCH] [-d OUTPUT_DIR] [--webui]
#
# Examples:
#   ./build.sh                           # Native build, CLI-only
#   ./build.sh --webui                   # Native build with embedded web UI
#   ./build.sh -o darwin -a arm64        # Cross-compile for macOS ARM64
#   ./build.sh -o linux -a amd64 --webui # Cross-compile Linux with web UI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Defaults
GOOS=""
GOARCH=""
OUTPUT_DIR="$PROJECT_ROOT/bin"
WEBUI=false
CGO_ENABLED=0

# Version info
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

usage() {
    echo "Usage: $0 [-o GOOS] [-a GOARCH] [-d OUTPUT_DIR] [--webui]"
    echo ""
    echo "Options:"
    echo "  -o GOOS       Target OS (darwin, linux, windows)"
    echo "  -a GOARCH     Target architecture (amd64, arm64)"
    echo "  -d DIR        Output directory (default: ./bin)"
    echo "  --webui       Include embedded web UI (requires frontend built)"
    echo "  -h, --help    Show this help"
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -o)
            GOOS="$2"
            shift 2
            ;;
        -a)
            GOARCH="$2"
            shift 2
            ;;
        -d)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --webui)
            WEBUI=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Determine output binary name
BINARY_NAME="arc"
if [[ -n "$GOOS" && -n "$GOARCH" ]]; then
    BINARY_NAME="arc-${GOOS}-${GOARCH}"
fi

OUTPUT_PATH="$OUTPUT_DIR/$BINARY_NAME"

# Build environment
BUILD_ENV="CGO_ENABLED=$CGO_ENABLED"
if [[ -n "$GOOS" ]]; then
    BUILD_ENV="$BUILD_ENV GOOS=$GOOS"
fi
if [[ -n "$GOARCH" ]]; then
    BUILD_ENV="$BUILD_ENV GOARCH=$GOARCH"
fi

# Build tags
BUILD_TAGS=""
if [[ "$WEBUI" == "true" ]]; then
    BUILD_TAGS="-tags webui"
    # Verify frontend is built
    if [[ ! -d "$PROJECT_ROOT/web/build" ]]; then
        echo "Error: Frontend not built. Run 'make web-build' first."
        exit 1
    fi
fi

# Linker flags
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X github.com/sentiolabs/arc/internal/version.Version=$VERSION"
LDFLAGS="$LDFLAGS -X github.com/sentiolabs/arc/internal/version.GitCommit=$GIT_COMMIT"
LDFLAGS="$LDFLAGS -X github.com/sentiolabs/arc/internal/version.BuildDate=$BUILD_DATE"

# Build
mkdir -p "$OUTPUT_DIR"
cd "$PROJECT_ROOT"

echo "Building $OUTPUT_PATH..."
if [[ -n "$GOOS" ]]; then
    echo "  Target: $GOOS/$GOARCH"
fi
if [[ "$WEBUI" == "true" ]]; then
    echo "  Web UI: embedded"
else
    echo "  Web UI: none (CLI-only)"
fi

eval "$BUILD_ENV go build $BUILD_TAGS -ldflags \"$LDFLAGS\" -o $OUTPUT_PATH ./cmd/arc"

echo "Done: $OUTPUT_PATH"
