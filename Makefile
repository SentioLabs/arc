# Arc Issue Tracker - Makefile
# ============================================================================
# Self-documenting Makefile for development tasks
# Run `make` or `make help` to see available targets
# ============================================================================

# Default target
.DEFAULT_GOAL := help

# Binary output
BIN_DIR := ./bin
SERVER_BIN := $(BIN_DIR)/arc-server
CLI_BIN := $(BIN_DIR)/arc

# Build settings
GO := go
GOFLAGS := -ldflags="-s -w"
CGO_ENABLED := 0

# Frontend settings
WEB_DIR := web
NODE_PACKAGE_MANAGER := bun

# Docker settings
DOCKER_IMAGE := arc-server
DOCKER_TAG := latest
COMPOSE_FILE := docker-compose.yml

# ===========================================================================
# Help
# ===========================================================================

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ===========================================================================
# Code Generation
# ===========================================================================

.PHONY: gen
gen: gen-api gen-sqlc gen-types ## Run all code generation

.PHONY: gen-api
gen-api: ## Generate OpenAPI server code (oapi-codegen)
	@echo "==> Generating OpenAPI server code..."
	$(GO) generate ./internal/api

.PHONY: gen-sqlc
gen-sqlc: ## Generate sqlc database code
	@echo "==> Generating sqlc code..."
	sqlc generate

.PHONY: gen-types
gen-types: ## Generate TypeScript types from OpenAPI spec
	@echo "==> Generating TypeScript types..."
	cd $(WEB_DIR) && $(NODE_PACKAGE_MANAGER) run generate

# ===========================================================================
# Frontend
# ===========================================================================

.PHONY: web-install
web-install: ## Install frontend dependencies
	@echo "==> Installing frontend dependencies..."
	cd $(WEB_DIR) && $(NODE_PACKAGE_MANAGER) install

.PHONY: web-build
web-build: web-install ## Build frontend for production
	@echo "==> Building frontend..."
	cd $(WEB_DIR) && $(NODE_PACKAGE_MANAGER) run build

.PHONY: web-dev
web-dev: ## Run frontend dev server
	@echo "==> Starting frontend dev server..."
	cd $(WEB_DIR) && $(NODE_PACKAGE_MANAGER) run dev

.PHONY: web-check
web-check: ## Run frontend type checking
	@echo "==> Checking frontend types..."
	cd $(WEB_DIR) && $(NODE_PACKAGE_MANAGER) run check

.PHONY: web-clean
web-clean: ## Clean frontend build artifacts
	@echo "==> Cleaning frontend build..."
	rm -rf $(WEB_DIR)/build $(WEB_DIR)/.svelte-kit

# ===========================================================================
# Build
# ===========================================================================

.PHONY: build
build: web-build build-server build-cli ## Build frontend and all binaries

.PHONY: build-server
build-server: ## Build the arc-server binary (requires frontend built first)
	@echo "==> Building $(SERVER_BIN)..."
	@if [ ! -d "$(WEB_DIR)/build" ]; then \
		echo "Warning: Frontend not built. Run 'make web-build' first for embedded UI."; \
	fi
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o $(SERVER_BIN) ./cmd/server

.PHONY: build-cli
build-cli: ## Build the arc CLI binary
	@echo "==> Building $(CLI_BIN)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o $(CLI_BIN) ./cmd/arc

.PHONY: build-quick
build-quick: build-server build-cli ## Build binaries without rebuilding frontend

.PHONY: install
install: ## Install binaries to GOPATH/bin
	@echo "==> Installing binaries..."
	$(GO) install ./cmd/server
	$(GO) install ./cmd/arc

# ===========================================================================
# Testing & Quality
# ===========================================================================

.PHONY: test
test: ## Run all tests
	@echo "==> Running tests..."
	$(GO) test -race -cover ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	@echo "==> Running tests (verbose)..."
	$(GO) test -race -cover -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests and generate coverage report
	@echo "==> Running tests with coverage..."
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: lint
lint: ## Run golangci-lint
	@echo "==> Running linter..."
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run linter and fix issues
	@echo "==> Running linter with auto-fix..."
	golangci-lint run --fix

.PHONY: vet
vet: ## Run go vet
	@echo "==> Running go vet..."
	$(GO) vet ./...

.PHONY: check
check: lint test web-check ## Run all checks (lint, test, frontend)

# ===========================================================================
# Dependencies
# ===========================================================================

.PHONY: tidy
tidy: ## Run go mod tidy
	@echo "==> Tidying modules..."
	$(GO) mod tidy

.PHONY: deps
deps: ## Download dependencies
	@echo "==> Downloading dependencies..."
	$(GO) mod download

.PHONY: deps-update
deps-update: ## Update all dependencies
	@echo "==> Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

# ===========================================================================
# Docker
# ===========================================================================

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "==> Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: docker-up
docker-up: ## Start services with docker-compose
	@echo "==> Starting services..."
	docker compose -f $(COMPOSE_FILE) up -d

.PHONY: docker-down
docker-down: ## Stop services with docker-compose
	@echo "==> Stopping services..."
	docker compose -f $(COMPOSE_FILE) down

.PHONY: docker-logs
docker-logs: ## Show docker-compose logs
	docker compose -f $(COMPOSE_FILE) logs -f

.PHONY: docker-restart
docker-restart: docker-down docker-up ## Restart docker-compose services

.PHONY: docker-clean
docker-clean: ## Remove Docker image and volumes
	@echo "==> Cleaning Docker resources..."
	docker compose -f $(COMPOSE_FILE) down -v
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true

# ===========================================================================
# Development
# ===========================================================================

.PHONY: run
run: build-server ## Build and run the server locally
	@echo "==> Starting server..."
	./$(SERVER_BIN)

.PHONY: dev
dev: ## Run server with live reload (requires air)
	@echo "==> Starting development server..."
	@command -v air >/dev/null 2>&1 || { echo "air not found. Install with: go install github.com/air-verse/air@latest"; exit 1; }
	air

.PHONY: mocks
mocks: ## Generate mocks (requires mockery)
	@echo "==> Generating mocks..."
	mockery

# ===========================================================================
# Cleanup
# ===========================================================================

.PHONY: clean
clean: ## Remove build artifacts
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	rm -f *.test

.PHONY: clean-all
clean-all: clean web-clean docker-clean ## Remove all artifacts including frontend and Docker

# ===========================================================================
# Shortcuts
# ===========================================================================

.PHONY: all
all: tidy gen lint test build ## Full build pipeline: tidy, generate, lint, test, build

.PHONY: fmt
fmt: ## Format Go code
	@echo "==> Formatting code..."
	$(GO) fmt ./...
	gofumpt -l -w .
