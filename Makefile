# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=animate-server
BINARY_UNIX=$(BINARY_NAME)_unix

# Build the application
.PHONY: build
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/animate-server

# Build for Linux
.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(BINARY_UNIX) ./cmd/animate-server

# Run the application
.PHONY: run
run: build
	./bin/$(BINARY_NAME)

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f bin/$(BINARY_NAME)
	rm -f bin/$(BINARY_UNIX)

# Run tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Install the application
.PHONY: install
install:
	$(GOCMD) install ./cmd/animate-server

# Development server with hot reload (requires air)
.PHONY: dev
dev:
	air

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build the application"
	@echo "  build-linux - Build for Linux"
	@echo "  run         - Build and run the application"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linter"
	@echo "  install     - Install the application"
	@echo "  dev         - Run development server with hot reload"
	@echo "  help        - Show this help message" 