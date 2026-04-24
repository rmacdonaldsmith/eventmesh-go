# EventMesh Makefile
# Simple build automation for development and CI/CD

.PHONY: build build-server build-cli test clean fmt vet help all

# Variables
BIN_DIR=bin
SERVER_BINARY=eventmesh
CLI_BINARY=eventmesh-cli
SERVER_CMD_DIR=cmd/eventmesh
CLI_CMD_DIR=cmd/eventmesh-cli
TEST_TIMEOUT?=2m
GO_FILES=$(shell find . -type f -name '*.go' -not -path './vendor/*')

# Default target
all: fmt vet test build

# Build all binaries
build: build-server build-cli
	@echo "✅ Build complete: $(BIN_DIR)/$(SERVER_BINARY), $(BIN_DIR)/$(CLI_BINARY)"

# Build the server binary
build-server:
	@echo "🔨 Building EventMesh server..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(SERVER_BINARY) ./$(SERVER_CMD_DIR)
	@echo "✅ Server build complete: $(BIN_DIR)/$(SERVER_BINARY)"

# Build the CLI binary
build-cli:
	@echo "🔨 Building EventMesh CLI..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(CLI_BINARY) ./$(CLI_CMD_DIR)
	@echo "✅ CLI build complete: $(BIN_DIR)/$(CLI_BINARY)"

# Run all tests with coverage
test:
	@echo "🧪 Running tests..."
	go test ./... -timeout $(TEST_TIMEOUT) -v -cover
	@echo "✅ Tests complete"

# Run tests with detailed coverage report
test-coverage:
	@echo "📊 Running tests with detailed coverage..."
	go test ./... -timeout $(TEST_TIMEOUT) -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Format Go code
fmt:
	@echo "🎨 Formatting Go code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Run Go vet for static analysis
vet:
	@echo "🔍 Running go vet..."
	go vet ./...
	@echo "✅ Static analysis complete"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	go clean ./...
	@echo "✅ Clean complete"

# Tidy Go modules
mod-tidy:
	@echo "📦 Tidying Go modules..."
	go mod tidy
	go mod verify
	@echo "✅ Modules tidied"

# Run the server (requires build first)
run: build
	@echo "🚀 Starting EventMesh server..."
	./$(BIN_DIR)/$(SERVER_BINARY) --node-id dev-node --listen :8080 --peer-listen :9090

# Development setup target
dev-setup:
	@echo "⚙️  Setting up development environment..."
	go mod download
	@mkdir -p $(BIN_DIR)
	@echo "✅ Development setup complete"

# Show help
help:
	@echo "EventMesh Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  all           - Run fmt, vet, test, and build (default)"
	@echo "  build         - Build server and CLI binaries"
	@echo "  build-server  - Build the EventMesh server binary"
	@echo "  build-cli     - Build the EventMesh CLI binary"
	@echo "  test          - Run all tests with basic coverage"
	@echo "  test-coverage - Run tests with detailed HTML coverage report"
	@echo "  fmt           - Format Go code with go fmt"
	@echo "  vet           - Run go vet static analysis"
	@echo "  clean         - Remove build artifacts and clean cache"
	@echo "  mod-tidy      - Tidy and verify Go modules"
	@echo "  run           - Build and run the server with dev settings"
	@echo "  dev-setup     - Set up development environment"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make          - Run full build pipeline"
	@echo "  make test     - Run tests only"
	@echo "  make run      - Start development server"
