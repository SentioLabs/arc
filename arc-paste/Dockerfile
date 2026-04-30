# syntax=docker/dockerfile:1.7
#
# Three-stage build for arc-paste:
#   1. `web` builds the SvelteKit SPA with bun (matches local toolchain)
#   2. `go-build` compiles a fully-static Go binary with the embedded SPA.
#      `-tags webui` is REQUIRED — without it `web.RegisterSPA` is a no-op
#      stub and `/share/<id>` returns Echo's default JSON 404.
#   3. Final image is `scratch` — no shell, no libc, just the binary.
#
# Build from the repo root, not arc-paste/, since stage 2 needs the Go module.
#   docker build -f arc-paste/Dockerfile -t arc-paste:latest .

# ─── Stage 1: SvelteKit SPA build ────────────────────────────────────────────
FROM oven/bun:1-alpine AS web
WORKDIR /web

# Install deps in their own layer so source-only changes don't bust the cache.
COPY web/package.json web/bun.lock* ./
RUN bun install --frozen-lockfile

COPY web/ ./
RUN bun run build

# ─── Stage 2: static Go binary ───────────────────────────────────────────────
FROM golang:1.26-alpine AS go-build
WORKDIR /build

# Module download is its own cache layer too.
COPY go.mod go.sum ./
RUN go mod download

# Bring in source + the SPA bundle the build tag will embed.
COPY . .
COPY --from=web /web/build ./web/build

# CGO_ENABLED=0 → fully static, no glibc dependency, runs on scratch.
# -tags webui   → embed the SPA (without it, the SPA returns 404).
# -trimpath     → strip absolute paths from the binary (smaller + reproducible).
# -ldflags '-s -w' → drop DWARF + symbol tables (~20% smaller binary).
RUN CGO_ENABLED=0 GOOS=linux go build \
    -tags webui \
    -trimpath \
    -ldflags='-s -w' \
    -o /out/arc-paste \
    ./arc-paste

# ─── Stage 3: minimal runtime ────────────────────────────────────────────────
FROM scratch

COPY --from=go-build /out/arc-paste /arc-paste

# Default DB lives under /data; mount a volume here to persist across restarts.
# arc-paste creates the directory itself, so this works even without a volume.
ENV ARC_PASTE_DB=/data/arc-paste.db
ENV ARC_PASTE_ADDR=:7433

EXPOSE 7433

ENTRYPOINT ["/arc-paste"]
