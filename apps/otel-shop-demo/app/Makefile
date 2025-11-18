# Container runtime (podman or docker)
CONTAINER_RUNTIME ?= podman
COMPOSE_CMD := $(CONTAINER_RUNTIME)-compose

.PHONY: help build up down logs test clean backend-build frontend-build

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build all Docker images
	@echo "Building images with $(CONTAINER_RUNTIME)..."
	$(COMPOSE_CMD) build

up: ## Start all services
	@echo "Starting services with $(CONTAINER_RUNTIME)..."
	$(COMPOSE_CMD) up -d
	@echo ""
	@echo "Services started!"
	@echo "  Frontend:  http://localhost:3000"
	@echo "  Backend:   http://localhost:8080"
	@echo "  Database:  postgresql://postgres:password@localhost:5432/shopdb"
	@echo "  OTel Collector: http://localhost:8888/metrics"

down: ## Stop all services
	@echo "Stopping services..."
	$(COMPOSE_CMD) down

logs: ## Show logs from all services
	$(COMPOSE_CMD) logs -f

logs-backend: ## Show backend logs
	$(COMPOSE_CMD) logs -f backend

logs-frontend: ## Show frontend logs
	$(COMPOSE_CMD) logs -f frontend

logs-db: ## Show database logs
	$(COMPOSE_CMD) logs -f postgresql

logs-collector: ## Show OTel collector logs
	$(COMPOSE_CMD) logs -f otel-collector

ps: ## Show running services
	$(COMPOSE_CMD) ps

restart: ## Restart all services
	$(COMPOSE_CMD) restart

clean: ## Stop and remove all containers, volumes, and images
	@echo "Cleaning up..."
	$(COMPOSE_CMD) down -v --rmi all
	@echo "Cleanup complete!"

test: ## Run a quick test of the API
	@echo "Testing backend health..."
	@curl -s http://localhost:8080/health
	@echo ""
	@echo "Testing products API..."
	@curl -s http://localhost:8080/api/products | head -c 200
	@echo "..."
	@echo ""
	@echo "Testing frontend..."
	@curl -s http://localhost:3000/health
	@echo ""

backend-shell: ## Open a shell in the backend container
	$(COMPOSE_CMD) exec backend sh

frontend-shell: ## Open a shell in the frontend container
	$(COMPOSE_CMD) exec frontend sh

db-shell: ## Open a PostgreSQL shell
	$(COMPOSE_CMD) exec postgresql psql -U postgres -d shopdb

# Development helpers
dev-backend: ## Run backend locally (without Docker)
	cd backend && go run main.go

dev-frontend: ## Run frontend locally (without Docker)
	cd frontend && npm run dev

build-backend: ## Build backend Docker image
	$(CONTAINER_RUNTIME) build -t ghcr.io/yu-min3/otel-shop-backend:latest ./backend

build-frontend: ## Build frontend Docker image
	$(CONTAINER_RUNTIME) build -t ghcr.io/yu-min3/otel-shop-frontend:latest ./frontend

push-backend: ## Push backend image to GHCR
	$(CONTAINER_RUNTIME) push ghcr.io/yu-min3/otel-shop-backend:latest

push-frontend: ## Push frontend image to GHCR
	$(CONTAINER_RUNTIME) push ghcr.io/yu-min3/otel-shop-frontend:latest

push-all: build-backend build-frontend push-backend push-frontend ## Build and push all images
