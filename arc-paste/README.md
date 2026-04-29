# arc-paste

A tiny standalone binary that exposes the arc paste API and serves the embedded SvelteKit SPA. Designed for public deployment as a zero-knowledge paste service for sharing arc plan reviews.

## Building

```bash
make build-paste
```

Produces `./bin/arc-paste`.

## Running

```bash
./bin/arc-paste
```

Starts the server on port 7433 by default.

## Configuration

- `ARC_PASTE_ADDR`: Listen address (default: `:7433`)
- `ARC_PASTE_DB`: SQLite database path (default: `./arc-paste.db`)

## API

The binary serves:
- `/api/paste/*` — Paste HTTP handlers (create, retrieve, update, delete pastes)
- `/` — Embedded SPA (with index.html fallback for SPA routing)

CORS is enabled for all origins.

## Docker

```bash
make docker-build
docker run -p 7433:7433 arc-paste:latest
```

## License

See the main arc repository.
