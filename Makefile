.PHONY: build test lint clean run deps release release-snapshot docker docker-multiarch nix-build nix-run check install-tools staticcheck gosec

# Add GOPATH/bin and Docker Desktop to PATH for tools
export PATH := /Applications/Docker.app/Contents/Resources/bin:$(shell go env GOPATH)/bin:$(PATH)

VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
BINARY := agent-collab

# Default target
all: deps build

# Install dependencies
deps:
	go mod tidy
	go mod download

# Production build
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY) ./src

# Development build (fast compile)
build-dev:
	go build -o bin/$(BINARY) ./src

# Run
run: build-dev
	./bin/$(BINARY)

# Run dashboard
dashboard: build-dev
	./bin/$(BINARY) dashboard

# Run tests
test:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run E2E tests (in-memory)
test-e2e:
	go test -v -race -tags=e2e ./src/e2e/...

# Lint
lint:
	@echo "🔍 Running golangci-lint..."
	golangci-lint run ./...

# Format check (for CI)
fmt:
	@echo "📝 Checking format..."
	@test -z "$$(gofmt -l . 2>&1)" || (echo "gofmt issues:" && gofmt -l . && exit 1)
	@echo "✓ Format OK"

# Format fix
fmt-fix:
	go fmt ./...
	goimports -w .

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.out coverage.html

# Cross compile all platforms
build-all:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./src
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./src
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./src
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 ./src
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe ./src

# Install locally
install: build
	cp bin/$(BINARY) $(GOPATH)/bin/

# Release (requires goreleaser)
release:
	@if [ -z "$(GITHUB_TOKEN)" ]; then echo "GITHUB_TOKEN not set"; exit 1; fi
	goreleaser release --clean

# Snapshot release (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

# Check goreleaser config
release-check:
	goreleaser check

# Generate (if any go:generate directives exist)
generate:
	go generate ./...

# Docker build
docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-t $(BINARY):$(VERSION) \
		-t $(BINARY):latest \
		.

# Docker multi-arch build
docker-multiarch:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-t $(BINARY):$(VERSION) \
		-t $(BINARY):latest \
		--push \
		.

# Nix build
nix-build:
	nix build .#agent-collab

# Nix run
nix-run:
	nix run .#agent-collab -- --help

# Nix develop shell
nix-develop:
	nix develop

# Staticcheck
staticcheck:
	@echo "🔍 Running staticcheck..."
	staticcheck -checks='all,-ST1000,-ST1003,-ST1005,-ST1020,-ST1021,-ST1022,-SA1019,-QF1003,-U1000' ./...

# Gosec
gosec:
	@echo "🔐 Running gosec..."
	gosec -exclude=G104,G115,G204,G304,G301,G302,G306,G112 -exclude-generated -quiet ./...

# Security scan
security: gosec
	govulncheck ./... || true

# CI check - run all checks before push
check: fmt lint staticcheck gosec
	@echo ""
	@echo "✅ All CI checks passed! Ready to push."

# Install development tools
install-tools:
	@echo "📦 Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "✓ Tools installed"

# ===========================================
# Docker Compose E2E Test Targets
# ===========================================

.PHONY: e2e-up e2e-down e2e-test e2e-logs e2e-clean

# Build Linux binary for Docker
build-linux:
	@echo "Building Linux AMD64 binary..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./src

# Start E2E test cluster (3 nodes)
e2e-up: build-linux
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for cluster to form..."
	@sleep 3
	@docker compose -f docker-compose.test.yml ps

# Stop E2E test cluster
e2e-down:
	docker compose -f docker-compose.test.yml down

# Run E2E tests against Docker cluster
e2e-test: e2e-up
	@echo "Running E2E tests..."
	go test -v -race -tags=e2e ./src/e2e/...
	@$(MAKE) e2e-down

# Show E2E cluster logs
e2e-logs:
	docker compose -f docker-compose.test.yml logs -f

# Clean E2E test artifacts
e2e-clean: e2e-down
	docker compose -f docker-compose.test.yml down -v --rmi local
	rm -f bin/$(BINARY)-linux-amd64

# ===========================================
# Claude Code Integration Test Targets
# ===========================================

.PHONY: claude-up claude-down claude-alice claude-bob claude-charlie claude-all claude-clean

# Start Claude test cluster (requires CLAUDE_OAUTH_TOKEN)
claude-up:
	@if [ -z "$(CLAUDE_OAUTH_TOKEN)" ]; then \
		echo "Error: CLAUDE_OAUTH_TOKEN not set"; \
		echo ""; \
		echo "Get token by running: claude setup-token"; \
		echo "Then: export CLAUDE_OAUTH_TOKEN=sk-ant-oat01-..."; \
		exit 1; \
	fi
	@chmod +x scripts/docker-entrypoint.sh 2>/dev/null || true
	docker compose -f docker-compose.claude.yml build --no-cache
	docker compose -f docker-compose.claude.yml up -d
	@echo "Waiting for cluster to form..."
	@sleep 5
	@docker compose -f docker-compose.claude.yml ps
	@echo ""
	@echo "Ready! Run agents:"
	@echo "  make claude-alice   # First: authentication"
	@echo "  make claude-bob     # Second: database"
	@echo "  make claude-charlie # Third: API"

# Stop Claude test cluster
claude-down:
	docker compose -f docker-compose.claude.yml down

# Setup Claude authentication (OAuth)
claude-auth:
	@echo "Setting up Claude authentication..."
	@echo "This will open a URL - copy it to your browser and complete authentication."
	@echo ""
	docker exec -it claude-alice docker-entrypoint setup

# Run Alice (authentication)
claude-alice:
	docker exec -it claude-alice docker-entrypoint run

# Run Bob (database)
claude-bob:
	docker exec -it claude-bob docker-entrypoint run

# Run Charlie (API)
claude-charlie:
	docker exec -it claude-charlie docker-entrypoint run

# Run all agents sequentially
claude-all: claude-alice claude-bob claude-charlie

# Interactive Claude session in container
claude-interactive-%:
	docker exec -it claude-$* claude

# Interactive shell into agent container
claude-shell-%:
	docker exec -it claude-$* /bin/bash

# Show Claude cluster logs
claude-logs:
	docker compose -f docker-compose.claude.yml logs -f

# Check MCP status across all peers
claude-status:
	@echo "=== Locks ==="
	@docker exec claude-alice agent-collab mcp call list_locks '{}' 2>/dev/null || echo "No locks"
	@echo ""
	@echo "=== Recent Events ==="
	@docker exec claude-alice agent-collab mcp call get_events '{"limit": 10}' 2>/dev/null || echo "No events"
	@echo ""
	@echo "=== Shared Contexts ==="
	@docker exec claude-alice agent-collab mcp call search_similar '{"query": "authentication database API"}' 2>/dev/null || echo "No contexts"

# Verify collaboration results
claude-verify:
	@echo "=== Modified Files ==="
	@git diff --stat test-workspace/ 2>/dev/null || echo "No git changes"
	@echo ""
	@echo "=== main.go Content ==="
	@cat test-workspace/main.go
	@echo ""
	@echo "=== Context Count per Peer ==="
	@echo -n "Alice: "; docker exec claude-alice agent-collab mcp call search_similar '{"query": "*"}' 2>/dev/null | grep -c "content" || echo "0"
	@echo -n "Bob: "; docker exec claude-bob agent-collab mcp call search_similar '{"query": "*"}' 2>/dev/null | grep -c "content" || echo "0"
	@echo -n "Charlie: "; docker exec claude-charlie agent-collab mcp call search_similar '{"query": "*"}' 2>/dev/null | grep -c "content" || echo "0"

# Full test: rebuild, run all agents, verify results
claude-test: claude-clean
	@echo "============================================"
	@echo "  CLAUDE COLLABORATION TEST"
	@echo "============================================"
	@echo ""
	@echo "[1/5] Building cluster (fresh)..."
	@git checkout test-workspace/main.go 2>/dev/null || true
	@$(MAKE) claude-up
	@echo ""
	@echo "[2/5] Running Alice (authentication)..."
	@$(MAKE) claude-alice || true
	@sleep 2
	@echo ""
	@echo "[3/5] Running Bob (database)..."
	@$(MAKE) claude-bob || true
	@sleep 2
	@echo ""
	@echo "[4/5] Running Charlie (API)..."
	@$(MAKE) claude-charlie || true
	@sleep 2
	@echo ""
	@echo "[5/5] Verifying results..."
	@$(MAKE) claude-verify
	@echo ""
	@echo "============================================"
	@echo "  TEST COMPLETE"
	@echo "============================================"

# Clean Claude test environment
claude-clean: claude-down
	docker compose -f docker-compose.claude.yml down -v --rmi local
	rm -rf test-workspace/*.go.bak

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  Build:"
	@echo "    build           - Production build with optimizations"
	@echo "    build-dev       - Development build (fast compile)"
	@echo "    build-all       - Cross compile for all platforms"
	@echo ""
	@echo "  Run:"
	@echo "    run             - Build and run"
	@echo "    dashboard       - Run TUI dashboard"
	@echo ""
	@echo "  Test:"
	@echo "    test            - Run unit tests"
	@echo "    test-coverage   - Run tests with coverage report"
	@echo "    test-e2e        - Run E2E tests"
	@echo ""
	@echo "  Quality:"
	@echo "    lint            - Run linter"
	@echo "    fmt             - Format code"
	@echo "    security        - Run security scans"
	@echo ""
	@echo "  Release:"
	@echo "    release         - Create release (requires GITHUB_TOKEN)"
	@echo "    release-snapshot- Create local snapshot release"
	@echo "    release-check   - Validate goreleaser config"
	@echo ""
	@echo "  Docker:"
	@echo "    docker          - Build Docker image"
	@echo "    docker-multiarch- Build multi-arch Docker image"
	@echo ""
	@echo "  Nix:"
	@echo "    nix-build       - Build with Nix"
	@echo "    nix-run         - Run with Nix"
	@echo "    nix-develop     - Enter Nix development shell"
	@echo ""
	@echo "  E2E Testing (Docker):"
	@echo "    e2e-up          - Start 3-node test cluster"
	@echo "    e2e-down        - Stop test cluster"
	@echo "    e2e-test        - Run E2E tests against cluster"
	@echo "    e2e-logs        - Show cluster logs"
	@echo "    e2e-clean       - Clean test artifacts"
	@echo ""
	@echo "  Claude Integration (OAuth):"
	@echo "    claude-test     - Full test: rebuild + run all + verify"
	@echo "    claude-up       - Start 3-agent cluster"
	@echo "    claude-alice    - Run Alice (authentication)"
	@echo "    claude-bob      - Run Bob (database)"
	@echo "    claude-charlie  - Run Charlie (API)"
	@echo "    claude-status   - Check MCP status (locks, events)"
	@echo "    claude-verify   - Verify collaboration results"
	@echo "    claude-logs     - Show cluster logs"
	@echo "    claude-clean    - Clean environment"
	@echo ""
	@echo "  Other:"
	@echo "    deps            - Install dependencies"
	@echo "    install         - Install binary to GOPATH"
	@echo "    clean           - Clean build artifacts"
	@echo "    generate        - Run go generate"
