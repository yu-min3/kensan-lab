.PHONY: all build run test clean docker-build docker-up docker-down migrate

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

# Services
SERVICES=user-service task-service timeblock-service analytics-service memo-service note-service

# Build all services
all: build

build:
	@for service in $(SERVICES); do \
		svcname=$${service%-service}; \
		echo "Building $$service..."; \
		$(GOBUILD) -o bin/$$service ./services/$$svcname/cmd; \
	done

# Run a specific service (usage: make run SERVICE=user-service)
run:
	@svcname=$$(echo $(SERVICE) | sed 's/-service$$//'); \
	$(GOBUILD) -o bin/$(SERVICE) ./services/$$svcname/cmd
	./bin/$(SERVICE)

# Run tests
test:
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf bin/

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Generate (protobuf, etc.)
generate:
	$(GOCMD) generate ./...

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Database migrations
migrate-up:
	@echo "Running migrations..."
	docker-compose exec postgres psql -U kensan -d kensan -f /docker-entrypoint-initdb.d/001_init.sql

# Development: start all services with hot reload (requires air)
dev:
	@echo "Starting development environment..."
	docker-compose up -d postgres
	@echo "Waiting for PostgreSQL..."
	@sleep 3
	@echo "PostgreSQL is ready. Start services manually or use 'make run SERVICE=<service-name>'"

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	gofmt -s -w .

# Help
help:
	@echo "Available commands:"
	@echo "  make build         - Build all services"
	@echo "  make run SERVICE=x - Run a specific service"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make deps          - Download dependencies"
	@echo "  make docker-build  - Build Docker images"
	@echo "  make docker-up     - Start Docker containers"
	@echo "  make docker-down   - Stop Docker containers"
	@echo "  make dev           - Start development environment"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
