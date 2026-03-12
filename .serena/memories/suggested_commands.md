# Suggested commands

## Build / run
- `make build` build frontend + unified binary
- `make build-quick` build CLI-only binary
- `./bin/arc-server` run server (default :7432)
- `./bin/arc-server -addr :8080` run server on custom port
- `arc server start` run server in background
- `arc server run` run server in foreground

## Tests / quality
- `make test` Go tests with race + coverage
- `make lint` golangci-lint
- `make vet` go vet
- `make check` lint + test + frontend check

## Formatting
- `make fmt` Go formatting (go fmt + gofumpt)

## Code generation
- `make gen` run all codegen
- `make gen-api` OpenAPI server code
- `make gen-sqlc` sqlc DB code
- `make gen-types` OpenAPI TS types for web

## Frontend (SvelteKit)
- `make web-install` install deps (bun)
- `make web-dev` run dev server
- `make web-build` production build
- `make web-check` type checking

## Docker
- `make docker-up`
- `make docker-down`
- `make docker-logs`

## Arc CLI essentials
- `arc ready`
- `arc create "title" --type=task -p 2`
- `arc update <id> --status in_progress`
- `arc close <id>`
- `arc show <id>`
- `arc dep add <issue> <depends-on>`

## Docs lookup
- `arc docs search "query"`
- `arc docs <topic>`

## System utilities (Linux)
- `git`, `ls`, `cd`, `pwd`, `rg`, `find`, `sed`, `awk`
