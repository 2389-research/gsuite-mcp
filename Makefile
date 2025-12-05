# GSuite MCP Server Makefile

# Variables
BINARY_NAME=gsuite-mcp
BUILD_DIR=.
CMD_DIR=./cmd/gsuite-mcp
GO=go
GOFLAGS=
LDFLAGS=-ldflags "-s -w"

# Build targets
.PHONY: all build clean test lint install run help

all: clean lint test build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -rf dist/
	@$(GO) clean
	@echo "Clean complete"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-short: Run tests (skip long-running tests)
test-short:
	@echo "Running short tests..."
	$(GO) test -v -short ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run
	@echo "Lint complete"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "Format complete"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...
	@echo "Vet complete"

## install: Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install $(GOFLAGS) $(CMD_DIR)
	@echo "Install complete"

## run: Build and run the server
run: build
	@echo "Starting $(BINARY_NAME)..."
	./$(BINARY_NAME)

## run-ish: Run server in ish mode (testing without real credentials)
run-ish: build
	@echo "Starting $(BINARY_NAME) in ish mode..."
	ISH_MODE=true ISH_BASE_URL=http://localhost:9000 ./$(BINARY_NAME)

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies updated"

## release: Build release binaries for multiple platforms
release:
	@echo "Building release binaries..."
	goreleaser release --snapshot --clean
	@echo "Release build complete"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
