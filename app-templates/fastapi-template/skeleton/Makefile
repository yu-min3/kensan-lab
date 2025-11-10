# Makefile for building and deploying ${{ values.name }}

# Variables
APP_NAME := ${{ values.name }}
REGISTRY := ghcr.io/yu-min3
IMAGE := $(REGISTRY)/$(APP_NAME)

# TAG is required - must be provided via command line
TAG ?=

# Container runtime (podman or docker) - auto-detect or override via RUNTIME env var
RUNTIME ?= $(shell which podman >/dev/null 2>&1 && echo podman || echo docker)

# Build command - use buildx for docker, standard build for podman
ifeq ($(RUNTIME),docker)
    BUILD_CMD := $(RUNTIME) buildx build
else
    BUILD_CMD := $(RUNTIME) build
endif

PUSH_CMD := $(RUNTIME) push
TAG_CMD := $(RUNTIME) tag

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

.PHONY: help build push update-dev update-prod deploy-dev deploy-prod clean validate

# Default target
help: ## Show this help message
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Container Runtime:$(NC)"
	@echo "  Current: $(YELLOW)$(RUNTIME)$(NC)"
	@echo "  Override: RUNTIME=docker make build TAG=v1.0"
	@echo ""
	@echo "$(GREEN)TAG Format Requirements:$(NC)"
	@echo "  $(YELLOW)Dev:$(NC)  Any tag format (e.g., dev-latest, dev-v1.0, feature-branch)"
	@echo "  $(YELLOW)Prod:$(NC) Must be vX.Y format (e.g., v1.0, v1.2, v10.5, v12.34)"
	@echo ""
	@echo "$(GREEN)Usage examples:$(NC)"
	@echo "  make build TAG=v1.0             # Build image with tag v1.0"
	@echo "  make deploy-dev TAG=dev-latest  # Build, push, and deploy to Dev"
	@echo "  make deploy-prod TAG=v1.2       # Build, push, and deploy to Prod (vX.Y format required)"
	@echo "  RUNTIME=docker make build TAG=v1.0  # Override runtime to use docker"

check-tag:
	@if [ -z "$(TAG)" ]; then \
		echo "$(RED)ERROR: TAG is required$(NC)"; \
		echo "Usage: make <target> TAG=<version>"; \
		echo "Example: make build TAG=v1.0"; \
		exit 1; \
	fi

check-tag-format-prod:
	@if ! echo "$(TAG)" | grep -qE '^v[0-9]+\.[0-9]+$$'; then \
		echo "$(RED)ERROR: Production TAG must be in format vX.Y (e.g., v1.0, v1.2, v10.5)$(NC)"; \
		echo "Current TAG: $(TAG)"; \
		echo "Valid examples:"; \
		echo "  v1.0"; \
		echo "  v1.2"; \
		echo "  v10.5"; \
		echo "  v12.34"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ TAG format is valid: $(TAG)$(NC)"

build: check-tag ## Build container image (requires TAG=<version>)
	@echo "$(GREEN)Building container image with $(RUNTIME): $(IMAGE):$(TAG)$(NC)"
	$(BUILD_CMD) --platform linux/amd64,linux/arm64 \
		-t $(IMAGE):$(TAG) \
		--load \
		.
	@echo "$(GREEN)✓ Build complete: $(IMAGE):$(TAG)$(NC)"

push: check-tag ## Push container image to registry (requires TAG=<version>)
	@echo "$(GREEN)Pushing container image with $(RUNTIME): $(IMAGE):$(TAG)$(NC)"
	$(PUSH_CMD) $(IMAGE):$(TAG)
	@echo "$(GREEN)✓ Push complete: $(IMAGE):$(TAG)$(NC)"

update-dev: check-tag ## Update Dev environment image tag in overlays/dev/kustomization.yaml
	@echo "$(GREEN)Updating Dev environment to tag: $(TAG)$(NC)"
	@sed -i.bak 's|newTag: .*|newTag: $(TAG)|' overlays/dev/kustomization.yaml && rm overlays/dev/kustomization.yaml.bak || \
		sed -i 's|newTag: .*|newTag: $(TAG)|' overlays/dev/kustomization.yaml
	@echo "$(YELLOW)Updated overlays/dev/kustomization.yaml$(NC)"
	@echo "$(RED)Don't forget to commit and push the changes!$(NC)"
	@echo "  git add overlays/dev/kustomization.yaml"
	@echo "  git commit -m 'Update Dev image to $(TAG)'"
	@echo "  git push"

update-prod: check-tag check-tag-format-prod ## Update Prod environment image tag in overlays/prod/kustomization.yaml
	@echo "$(GREEN)Updating Prod environment to tag: $(TAG)$(NC)"
	@sed -i.bak 's|newTag: .*|newTag: $(TAG)|' overlays/prod/kustomization.yaml && rm overlays/prod/kustomization.yaml.bak || \
		sed -i 's|newTag: .*|newTag: $(TAG)|' overlays/prod/kustomization.yaml
	@echo "$(YELLOW)Updated overlays/prod/kustomization.yaml$(NC)"
	@echo "$(RED)Don't forget to commit and push the changes!$(NC)"
	@echo "  git add overlays/prod/kustomization.yaml"
	@echo "  git commit -m 'Update Prod image to $(TAG)'"
	@echo "  git push"

deploy-dev: build push update-dev ## Build, push, and update Dev environment (requires TAG=<version>)
	@echo "$(GREEN)✓ Dev deployment complete!$(NC)"
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Review changes: git diff overlays/dev/kustomization.yaml"
	@echo "  2. Commit changes: git add overlays/dev/kustomization.yaml && git commit -m 'Deploy $(TAG) to Dev'"
	@echo "  3. Push to trigger Argo CD: git push"

deploy-prod: build push update-prod ## Build, push, and update Prod environment (requires TAG=<version>)
	@echo "$(GREEN)✓ Prod deployment complete!$(NC)"
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Review changes: git diff overlays/prod/kustomization.yaml"
	@echo "  2. Commit changes: git add overlays/prod/kustomization.yaml && git commit -m 'Deploy $(TAG) to Prod'"
	@echo "  3. Push to trigger Argo CD: git push"

validate: ## Validate Kustomize manifests
	@echo "$(GREEN)Validating Kustomize manifests...$(NC)"
	@kustomize build overlays/dev > /dev/null && echo "$(GREEN)✓ Dev manifests are valid$(NC)" || echo "$(RED)✗ Dev manifests have errors$(NC)"
	@kustomize build overlays/prod > /dev/null && echo "$(GREEN)✓ Prod manifests are valid$(NC)" || echo "$(RED)✗ Prod manifests have errors$(NC)"

clean: ## Clean up local container images
	@echo "$(YELLOW)Cleaning up container images for $(APP_NAME)...$(NC)"
	@$(RUNTIME) images $(IMAGE) -q | xargs -r $(RUNTIME) rmi -f || true
	@echo "$(GREEN)✓ Cleanup complete$(NC)"

# Development helpers
.PHONY: run-local install sync test lint format format-check

install: ## Install uv (if not already installed)
	@which uv > /dev/null || (echo "$(RED)uv is not installed$(NC)" && echo "Install with: curl -LsSf https://astral.sh/uv/install.sh | sh" && exit 1)
	@echo "$(GREEN)✓ uv is installed$(NC)"

sync: install ## Sync dependencies with uv
	@echo "$(GREEN)Syncing dependencies...$(NC)"
	uv sync
	@echo "$(GREEN)✓ Dependencies synced$(NC)"

run-local: sync ## Run application locally (without Docker)
	@echo "$(GREEN)Running application locally...$(NC)"
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
