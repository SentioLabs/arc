#!/bin/sh
# Entrypoint for the self-contained integration test container.
# Starts the arc server, waits for health, runs the compiled test binary.
set -e

DB_PATH="${ARC_DB:-/tmp/arc-test.db}"
PORT="${ARC_PORT:-7432}"

# Start the server in the background
arc server start --foreground --port "$PORT" --db "$DB_PATH" &
SERVER_PID=$!

# Wait for the server to become healthy
echo "Waiting for server on port $PORT..."
for i in $(seq 1 30); do
    if wget -q --spider "http://127.0.0.1:${PORT}/health" 2>/dev/null; then
        echo "Server ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "Server failed to start within 30s"
        kill "$SERVER_PID" 2>/dev/null
        exit 1
    fi
    sleep 1
done

# Run the integration test binary
ARC_BINARY=/usr/local/bin/arc /usr/local/bin/integration.test -test.v -test.count=1
EXIT_CODE=$?

kill "$SERVER_PID" 2>/dev/null
exit "$EXIT_CODE"
