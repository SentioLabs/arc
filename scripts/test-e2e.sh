#!/usr/bin/env bash
# Every invocation owns a private Docker Compose project. See --help.
set -euo pipefail
exec python3 "$(dirname "${BASH_SOURCE[0]}")/test_e2e.py" "$@"
