# Style and conventions

## Go
- Follow standard Go conventions (gofmt).
- Formatting: `make fmt` runs `go fmt ./...` and `gofumpt -l -w .`.
- Linting: `make lint` uses golangci-lint.
- Vetting: `make vet` runs `go vet ./...`.

## Frontend (SvelteKit/TypeScript)
- Linting: `bun run lint` (via `web/` scripts).
- Formatting: `bun run format` (Prettier + Svelte plugin).
- Type checking: `bun run check` (svelte-check).

## General
- Keep API JSON request/response consistent.
- Maintain workspace isolation patterns in storage layer.
