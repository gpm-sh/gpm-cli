# GPM CLI Testing and Development Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS = -ldflags "-X gpm.sh/gpm/gpm-cli/cmd.Version=$(VERSION) -X gpm.sh/gpm/gpm-cli/cmd.Commit=$(COMMIT) -X gpm.sh/gpm/gpm-cli/cmd.Date=$(DATE)"
BUILD_FLAGS = $(LDFLAGS) -trimpath

.PHONY: help test test-unit test-integration test-e2e test-coverage build build-all install clean lint fmt deps version setup-hooks release-major release-minor release-patch

help:
	@echo "Available targets:"
	@echo "  deps              Install dependencies"
	@echo "  fmt               Format code"
	@echo "  lint              Run linter"
	@echo "  test              Run all tests"
	@echo "  test-unit         Run unit tests only"
	@echo "  test-integration  Run integration tests only"
	@echo "  test-e2e          Run end-to-end tests only"
	@echo "  test-coverage     Run tests with coverage report"
	@echo "  build             Build the CLI binary"
	@echo "  build-all         Build for all platforms"
	@echo "  install           Install the binary to GOPATH/bin"
	@echo "  version           Show version information"
	@echo "  setup-hooks       Setup Git hooks for development"
	@echo "  release-major     Create a major release"
	@echo "  release-minor     Create a minor release"
	@echo "  release-patch     Create a patch release"
	@echo "  clean             Clean build artifacts"

deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run

build:
	@echo "Building gpm CLI..."
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
	go build $(BUILD_FLAGS) -o gpm .

build-all: clean
	@echo "Building for all platforms..."
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/gpm-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/gpm-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/gpm-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/gpm-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/gpm-windows-amd64.exe .
	@echo "Built binaries are in dist/"

install:
	go install $(BUILD_FLAGS) .

version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

clean:
	rm -f gpm gpm-cli
	rm -rf coverage dist
	find . -name "*.tgz" -delete
	find . -name "coverage.out" -delete

test: test-unit test-integration

test-unit:
	@echo "Running unit tests..."
	go test -v -race -timeout 30s ./internal/... ./cmd/... -run "^Test[^I]"

test-integration:
	@echo "Running integration tests..."
	go test -v -race -timeout 60s ./test/integration/...

test-e2e:
	@echo "Running end-to-end tests..."
	go test -v -timeout 120s ./test/e2e/... -run "E2E"

test-coverage:
	@echo "Running tests with coverage..."
	mkdir -p coverage
	go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	go tool cover -func=coverage/coverage.out

test-pkg:
	@read -p "Enter package path: " pkg; \
	go test -v -race $$pkg

test-watch:
	find . -name "*.go" | entr -c make test-unit

bench:
	go test -bench=. -benchmem ./...

smoke:
	@echo "Running smoke tests..."
	go build $(BUILD_FLAGS) -o gpm-test .
	./gpm-test --version
	./gpm-test --help
	rm -f gpm-test

release:
	@echo "Building release binary..."
	go build $(BUILD_FLAGS) -o gpm .
	strip gpm 2>/dev/null || true
	@echo "Release binary built: gpm"

setup-hooks:
	@echo "Setting up Git hooks..."
	./scripts/setup-git-hooks.sh

release-major:
	@echo "Creating major release..."
	./scripts/release.sh major

release-minor:
	@echo "Creating minor release..."
	./scripts/release.sh minor

release-patch:
	@echo "Creating patch release..."
	./scripts/release.sh patch
