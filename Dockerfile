# ===========================================================================
# Stage 1: Build frontend
# ===========================================================================
FROM oven/bun:1 AS frontend-builder

WORKDIR /build/web

# Copy package files first for better layer caching
COPY web/package.json web/bun.lock* ./
RUN bun install --frozen-lockfile

# Copy frontend source and build
COPY web/ ./
RUN bun run build

# ===========================================================================
# Stage 2: Build Go binaries
# ===========================================================================
FROM golang:1.25-alpine AS go-builder

WORKDIR /build

# Install git (needed for go mod download with git dependencies)
RUN apk add --no-cache git

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /build/web/build ./web/build

# Build both binaries
# CGO_ENABLED=0 for static binaries (modernc.org/sqlite is pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o arc-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o arc ./cmd/arc

# ===========================================================================
# Stage 3: Runtime
# ===========================================================================
FROM alpine:3.21

# Add ca-certificates for HTTPS and tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 arc && \
    adduser -u 1000 -G arc -s /bin/sh -D arc

# Create data directory
RUN mkdir -p /data && chown arc:arc /data

# Copy binaries from builder
COPY --from=go-builder /build/arc-server /usr/local/bin/
COPY --from=go-builder /build/arc /usr/local/bin/

# Switch to non-root user
USER arc

# Expose the default port
EXPOSE 7432

# Data volume for SQLite database
VOLUME ["/data"]

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:7432/health || exit 1

# Default command
ENTRYPOINT ["arc-server"]
CMD ["-addr", ":7432", "-db", "/data/arc.db"]
