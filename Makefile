.PHONY: build test lint clean run deps release release-snapshot docker docker-multiarch nix-build nix-run check install-tools staticcheck gosec

# Add GOPATH/bin to PATH for tools
export PATH := $(shell go env GOPATH)/bin:$(PATH)

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
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/agent-collab

# Development build (fast compile)
build-dev:
	go build -o bin/$(BINARY) ./cmd/agent-collab

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

# Run E2E tests
test-e2e:
	go test -v -race -tags=e2e ./tests/e2e/...

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
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./cmd/agent-collab
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./cmd/agent-collab
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/agent-collab
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 ./cmd/agent-collab
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe ./cmd/agent-collab

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
# Multipass E2E Test Targets
# ===========================================

.PHONY: multipass-setup multipass-init multipass-test multipass-test-lock multipass-test-context multipass-cleanup multipass-status

# Build Linux binary for Multipass VMs
build-linux:
	@echo "Building Linux AMD64 binary for Multipass..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o agent-collab-linux ./cmd/agent-collab
	@echo "Binary: ./agent-collab-linux"

# Set script permissions
multipass-permissions:
	chmod +x scripts/*.sh

# Setup Multipass VMs
multipass-setup: multipass-permissions build-linux
	scripts/setup.sh

# Initialize cluster
multipass-init:
	scripts/cluster-init.sh

# Run lock propagation test
multipass-test-lock:
	scripts/test-lock.sh

# Run context sync test
multipass-test-context:
	scripts/test-context.sh

# Run all Multipass tests
multipass-test: multipass-permissions build-linux
	scripts/run-all.sh

# Run Tier 1 tests only
multipass-test-tier1: multipass-permissions build-linux
	scripts/run-all.sh --tier1

# Run tests but keep VMs
multipass-test-keep: multipass-permissions build-linux
	scripts/run-all.sh --skip-cleanup

# Cleanup Multipass VMs
multipass-cleanup:
	scripts/cleanup.sh

# Check VM status
multipass-status:
	@multipass list
	@echo ""
	@for vm in peer1 peer2 peer3; do \
		echo "=== $$vm ==="; \
		multipass exec $$vm -- /home/ubuntu/agent-collab status 2>/dev/null || echo "Not running"; \
	done

# Show test results
multipass-results:
	@echo "Test Results:"
	@ls -la results/ 2>/dev/null || echo "No results yet"
	@echo ""
	@if ls results/summary-*.json 1> /dev/null 2>&1; then \
		cat $$(ls -t results/summary-*.json | head -1); \
	fi

# ===========================================
# Original Targets
# ===========================================

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
	@echo "  Multipass E2E:"
	@echo "    multipass-setup      - Create VMs and setup environment"
	@echo "    multipass-init       - Initialize cluster"
	@echo "    multipass-test       - Run all Multipass tests"
	@echo "    multipass-test-tier1 - Run Tier 1 tests only (lock)"
	@echo "    multipass-test-keep  - Run tests, keep VMs"
	@echo "    multipass-cleanup    - Delete VMs"
	@echo "    multipass-status     - Check VM status"
	@echo "    multipass-results    - Show test results"
	@echo ""
	@echo "  Other:"
	@echo "    deps            - Install dependencies"
	@echo "    install         - Install binary to GOPATH"
	@echo "    clean           - Clean build artifacts"
	@echo "    generate        - Run go generate"
