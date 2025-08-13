# ==============================================================================
# Makefile for the code-prompt-core project
# ==============================================================================

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run

# Binary name
BINARY_NAME=code-prompt-core
BINARY_UNIX=$(BINARY_NAME)
BINARY_WINDOWS=$(BINARY_NAME).exe

# Build flags
# -s: Omit the symbol table
# -w: Omit the DWARF symbol table (debugging information)
# These flags significantly reduce the binary size.
LDFLAGS = -ldflags="-s -w"

# Default target executed when you just run `make`
all: build docs

.PHONY: all build docs clean help run test

# --- Build Targets ---

build: ## Build the binary for the current OS/ARCH
	@echo "==> Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "==> Done."

build-linux: ## Build the binary for Linux (amd64)
	@echo "==> Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) .
	@echo "==> Done."

build-windows: ## Build the binary for Windows (amd64)
	@echo "==> Building for Windows..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_WINDOWS) .
	@echo "==> Done."

build-darwin: ## Build the binary for macOS (amd64)
	@echo "==> Building for macOS..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) .
	@echo "==> Done."

build-all: build-linux build-windows build-darwin ## Build for all target platforms

# --- Documentation Target ---

docs: ## Generate the API documentation
	@echo "==> Generating API documentation..."
	$(GORUN) main.go docs export
	@echo "==> APIDocumentation.md has been generated."

# --- Utility Targets ---

test: ## Run tests
	@echo "==> Running tests..."
	$(GOTEST) -v ./...

run: ## Run the application
	@echo "==> Running application..."
	$(GORUN) main.go $(ARGS)

clean: ## Remove previous build artifacts
	@echo "==> Cleaning up..."
	$(GOCLEAN)
	rm -f $(BINARY_UNIX) $(BINARY_WINDOWS)
	@echo "==> Done."

help: ## Show this help message
	@echo "Usage: make <target>"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'