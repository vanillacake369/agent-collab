.PHONY: build test lint clean run deps

VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY := agent-collab

# 기본 타겟
all: deps build

# 의존성 설치
deps:
	go mod tidy
	go mod download

# 빌드
build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/agent-collab

# 개발용 빌드 (빠른 컴파일)
build-dev:
	go build -o bin/$(BINARY) ./cmd/agent-collab

# 실행
run: build-dev
	./bin/$(BINARY)

# 대시보드 실행
dashboard: build-dev
	./bin/$(BINARY) dashboard

# 테스트
test:
	go test -v -race ./...

# 테스트 커버리지
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# 린트
lint:
	golangci-lint run

# 포맷
fmt:
	go fmt ./...
	goimports -w .

# 클린
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# 크로스 컴파일
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./cmd/agent-collab
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./cmd/agent-collab
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/agent-collab
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe ./cmd/agent-collab

# 설치 (로컬)
install: build
	cp bin/$(BINARY) $(GOPATH)/bin/

# 도움말
help:
	@echo "사용 가능한 타겟:"
	@echo "  deps        - 의존성 설치"
	@echo "  build       - 프로덕션 빌드"
	@echo "  build-dev   - 개발용 빌드"
	@echo "  run         - 빌드 후 실행"
	@echo "  dashboard   - TUI 대시보드 실행"
	@echo "  test        - 테스트 실행"
	@echo "  lint        - 린트 실행"
	@echo "  clean       - 빌드 결과물 삭제"
	@echo "  build-all   - 크로스 컴파일"
