# Makefile for building and deploying ${{ values.name }}
# Load environment variables from .env file if it exists
-include .env

# For GHCR push credential
GITHUB_TOKEN ?= $(GITHUB_GHCR_PAT)
export GITHUB_TOKEN

# Variables
APP_NAME := ${{ values.name }}
REGISTRY := ghcr.io/yu-min3
IMAGE := $(REGISTRY)/$(APP_NAME)

# TAG is required - must be provided via command line
TAG ?=

# Full image name
FULL_IMAGE := $(IMAGE):$(TAG)

# Container runtime (podman or docker) - auto-detect or override via CONTAINER_RUNTIME env var
CONTAINER_RUNTIME ?= $(shell which podman >/dev/null 2>&1 && echo podman || echo docker)

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

.PHONY: help build push update-image deploy clean validate

# Default target
help: ## Show this help message
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Container Runtime:$(NC)"
	@echo "  Current: $(YELLOW)$(CONTAINER_RUNTIME)$(NC)"
	@echo "  Override: CONTAINER_RUNTIME=docker make build TAG=v1.0"
	@echo ""
	@echo "$(GREEN)TAG Format Requirements:$(NC)"
	@echo "  Must be vX.Y format (e.g., v1.0, v1.2, v10.5, v12.34)"
	@echo ""
	@echo "$(GREEN)Usage examples:$(NC)"
	@echo "  make build TAG=v1.0   # Build image with tag v1.0"
	@echo "  make deploy TAG=v1.2  # Build, push, and deploy"
	@echo "  CONTAINER_RUNTIME=docker make build TAG=v1.0  # Override runtime to use docker"

check-tag:
	@if [ -z "$(TAG)" ]; then \
		echo "$(RED)ERROR: TAG is required$(NC)"; \
		echo "Usage: make <target> TAG=<version>"; \
		echo "Example: make build TAG=v1.0"; \
		exit 1; \
	fi

check-tag-format:
	@if ! echo "$(TAG)" | grep -qE '^v[0-9]+\.[0-9]+$$'; then \
		echo "$(RED)ERROR: TAG must be in format vX.Y (e.g., v1.0, v1.2, v10.5)$(NC)"; \
		echo "Current TAG: $(TAG)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ TAG format is valid: $(TAG)$(NC)"

build: check-tag ## Build container image (requires TAG=<version>)
	@echo "$(GREEN)Building container image with $(CONTAINER_RUNTIME): $(FULL_IMAGE)$(NC)"
	$(CONTAINER_RUNTIME) build --format=docker -t $(FULL_IMAGE) .
	@echo "$(GREEN)✓ Build complete: $(FULL_IMAGE)$(NC)"

login: ## Login to GHCR
	@echo "$(GREEN)Logging in to GHCR...$(NC)"
	@echo "$(YELLOW)Note: Set GITHUB_TOKEN or GITHUB_GHCR_PAT environment variable$(NC)"
	@if [ -z "$$GITHUB_TOKEN" ] && [ -z "$$GITHUB_GHCR_PAT" ]; then \
		echo "$(RED)ERROR: GITHUB_TOKEN or GITHUB_GHCR_PAT not set$(NC)"; \
		exit 1; \
	fi
	@echo "$${GITHUB_TOKEN:-$$GITHUB_GHCR_PAT}" | $(CONTAINER_RUNTIME) login ghcr.io -u $${GITHUB_USER:-yu-min3} --password-stdin

push: check-tag login ## Push container image to registry (requires TAG=<version>)
	@echo "$(GREEN)Pushing image: $(FULL_IMAGE)$(NC)"
	$(CONTAINER_RUNTIME) push $(FULL_IMAGE)
	@echo "$(GREEN)✓ Push complete: $(FULL_IMAGE)$(NC)"

update-image: check-tag check-tag-format ## Update image tag in manifests/deployment.yaml
	@echo "$(GREEN)Updating image tag to: $(TAG)$(NC)"
	sed -i.bak -E 's|(image: $(REGISTRY)/$(APP_NAME)):.*|\1:$(TAG)|' manifests/deployment.yaml && rm manifests/deployment.yaml.bak
	@echo "$(YELLOW)Updated manifests/deployment.yaml$(NC)"
	@echo "$(RED)Commit and push to deploy:$(NC)"
	@echo "  git add manifests/deployment.yaml && git commit -m 'Deploy $(TAG)' && git push"

deploy: build push update-image ## Build, push, and update image tag (requires TAG=vX.Y)
	@echo "$(GREEN)✓ Build + push + manifest update complete!$(NC)"
	@echo "$(YELLOW)Next: review and commit$(NC)"
	@echo "  git diff manifests/deployment.yaml"
	@echo "  git add manifests/deployment.yaml && git commit -m 'Deploy $(TAG)' && git push"

validate: ## Validate manifests with kubectl dry-run
	@echo "$(GREEN)Validating manifests...$(NC)"
	@kubectl apply --dry-run=client -f manifests/ > /dev/null && echo "$(GREEN)✓ Manifests are valid$(NC)" || echo "$(RED)✗ Manifests have errors$(NC)"

clean: ## Clean up local container images
	@echo "$(YELLOW)Cleaning up container images for $(APP_NAME)...$(NC)"
	@$(CONTAINER_RUNTIME) images $(IMAGE) -q | xargs -r $(CONTAINER_RUNTIME) rmi -f || true
	@echo "$(GREEN)✓ Cleanup complete$(NC)"

# Development helpers
.PHONY: run run-local install sync test lint format format-check

install: ## Install uv (if not already installed)
	@which uv > /dev/null || (echo "$(RED)uv is not installed$(NC)" && echo "Install with: curl -LsSf https://astral.sh/uv/install.sh | sh" && exit 1)
	@echo "$(GREEN)✓ uv is installed$(NC)"

sync: install ## Sync dependencies with uv
	@echo "$(GREEN)Syncing dependencies...$(NC)"
	uv sync
	@echo "$(GREEN)✓ Dependencies synced$(NC)"

run: build ## Run the built container image locally for testing
	@echo "$(GREEN)Running container image for local testing...$(NC)"
	@echo "Access at: http://localhost:8000"
	@echo ""
	$(CONTAINER_RUNTIME) run --rm -it -p 8000:8000 $(FULL_IMAGE)

run-local: sync ## Run application locally (without container)
	@echo "$(GREEN)Running application locally with uv...$(NC)"
	@echo "Access at: http://localhost:8000"
	@echo ""
	uv run python app/main.py

test: sync ## Run tests with pytest
	@echo "$(GREEN)Running tests...$(NC)"
	uv run pytest tests/ -v

lint: sync ## Run ruff linter
	@echo "$(GREEN)Running ruff linter...$(NC)"
	uv run ruff check app/
	@echo "$(GREEN)✓ Linting complete$(NC)"

format: sync ## Format code with ruff
	@echo "$(GREEN)Formatting code with ruff...$(NC)"
	uv run ruff format app/
	uv run ruff check --fix app/
	@echo "$(GREEN)✓ Code formatted$(NC)"

format-check: sync ## Check code formatting without making changes
	@echo "$(GREEN)Checking code formatting...$(NC)"
	uv run ruff format --check app/
	uv run ruff check app/
	@echo "$(GREEN)✓ Format check complete$(NC)"
