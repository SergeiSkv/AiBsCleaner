# AiBsCleaner Makefile

.PHONY: build install clean test analyze fix help setup docker-up docker-down

# Variables
BINARY_NAME=aibscleaner
VERSION=0.10.0
BUILD_FLAGS=-ldflags="-X 'github.com/SergeiSkv/AiBsCleaner/cmd.version=$(VERSION)'"
INSTALL_PATH=${GOPATH}/bin

# Default target
all: build

## help: Show this help message
help:
	@echo "AiBsCleaner - Stop AI bullshit, write performant Go"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' Makefile | sed 's/## /  /'

## setup: Setup development environment
setup:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	@echo "Setup complete!"

## init: Create default configuration file
init: build
	./$(BINARY_NAME) init
	@echo "Configuration created: .aibscleaner.yaml"

## list: List all available analyzers
list: build
	./$(BINARY_NAME) list-analyzers

## version: Show version information
version: build
	./$(BINARY_NAME) version

## build: Build the binary
build:
	go build $(BUILD_FLAGS) -o $(BINARY_NAME) .
	@echo "Built $(BINARY_NAME) successfully"

## install: Install AiBsCleaner globally
install: build
	@cp $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "Installed to $(INSTALL_PATH)/$(BINARY_NAME)"

## clean: Remove built binaries
clean:
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out
	@echo "Cleaned"

## test: Run tests
test:
	go test -v -cover ./...

## coverage: Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## analyze: Run AiBsCleaner on current directory
analyze: build
	./$(BINARY_NAME) .

## analyze-json: Run AiBsCleaner with JSON output
analyze-json: build
	./$(BINARY_NAME) --json .

## lint: Run additional linters
lint:
	golangci-lint run
	go vet ./...

## bench: Run benchmarks
bench:
	go test -bench=. -benchmem ./analyzer

## docker: Build Docker image
docker:
	docker build -t aibscleaner:$(VERSION) .

## release: Build for multiple platforms
release:
	@mkdir -p dist
	@echo "Building for multiple platforms..."
	# macOS
	GOOS=darwin GOARCH=amd64 go build -o dist/$(BINARY_NAME)-darwin-amd64 $(BUILD_FLAGS)
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY_NAME)-darwin-arm64 $(BUILD_FLAGS)
	# Linux
	GOOS=linux GOARCH=amd64 go build -o dist/$(BINARY_NAME)-linux-amd64 $(BUILD_FLAGS)
	GOOS=linux GOARCH=arm64 go build -o dist/$(BINARY_NAME)-linux-arm64 $(BUILD_FLAGS)
	# Windows
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY_NAME)-windows-amd64.exe $(BUILD_FLAGS)
	@echo "Release builds completed in dist/"

## compact: Run AiBsCleaner in compact IDE-friendly mode
compact: build
	./$(BINARY_NAME) --compact .

## check-all: Run all checks
check-all: test lint analyze
	@echo "All checks passed!"

## examples: Run analyzer on example files
examples: build
	./$(BINARY_NAME) examples/

# CI targets
ci-test:
	go test -v -race -coverprofile=coverage.out ./...

ci-lint:
	golangci-lint run --timeout=5m

ci-build:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $(BINARY_NAME) .