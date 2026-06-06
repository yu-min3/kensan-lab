.PHONY: build test run clean lint fmt help

# Service name
SERVICE_NAME=analytics-service
SERVICE_DIR=services/analytics

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean

# Build directory (relative to backend/)
BIN_DIR=../../bin

# Build the service
build:
	@echo "Building $(SERVICE_NAME)..."
	@cd ../.. && $(GOBUILD) -o $(BIN_DIR)/$(SERVICE_NAME) ./$(SERVICE_DIR)/cmd

# Run tests for this service only
test:
	@cd ../.. && $(GOTEST) -v ./$(SERVICE_DIR)/...

# Run the service
run: build
	@$(BIN_DIR)/$(SERVICE_NAME)

# Clean build artifacts
clean:
	@rm -f $(BIN_DIR)/$(SERVICE_NAME)

# Lint this service
lint:
	@cd ../.. && golangci-lint run ./$(SERVICE_DIR)/...

# Format this service
fmt:
	@gofmt -s -w .

# Docker build
docker-build:
	@cd ../.. && docker build -f $(SERVICE_DIR)/Dockerfile -t kensan-$(SERVICE_NAME):latest .

# Help
help:
	@echo "$(SERVICE_NAME) Makefile commands:"
	@echo "  make build        - Build the service"
	@echo "  make test         - Run tests"
	@echo "  make run          - Build and run the service"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make lint         - Run linter"
	@echo "  make fmt          - Format code"
	@echo "  make docker-build - Build Docker image"
