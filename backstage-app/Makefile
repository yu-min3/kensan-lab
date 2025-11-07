# Backstage Image Build and Push Makefile
# Load environment variables from .env file if it exists
-include ../.env
export

# Default values
REGISTRY ?= ghcr.io
IMAGE_NAME ?= $(GITHUB_USER)/backstage
TAG ?= latest
FULL_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(TAG)

.PHONY: help
help:
	@echo "Backstage Build and Push Commands:"
	@echo "  make install      - Install dependencies"
	@echo "  make build        - Build backend and create Docker image"
	@echo "  make push         - Login to GHCR and push image"
	@echo "  make all          - Install, build, and push image"
	@echo "  make clean        - Remove Docker image locally"
	@echo "  make dev          - Run Backstage locally for development"
	@echo ""
	@echo "Environment Variables (set in ../.env):"
	@echo "  GITHUB_USER       - GitHub username"
	@echo "  GITHUB_GHCR_PAT   - GitHub Personal Access Token for GHCR"
	@echo "  TAG               - Image tag (default: latest)"

.PHONY: install
install:
	@echo "Installing dependencies..."
	node .yarn/releases/yarn-4.4.1.cjs install

.PHONY: build-backend
build-backend:
	@echo "Building backend package..."
	node .yarn/releases/yarn-4.4.1.cjs workspace backend build

.PHONY: build
build: build-backend
	@echo "Building Docker image: $(FULL_IMAGE)"
	DOCKER_BUILDKIT=1 docker build -f packages/backend/Dockerfile -t $(FULL_IMAGE) .

.PHONY: login
login:
	@echo "Logging in to GHCR..."
	@echo "$(GITHUB_GHCR_PAT)" | docker login ghcr.io -u $(GITHUB_USER) --password-stdin

.PHONY: push
push: login
	@echo "Pushing image: $(FULL_IMAGE)"
	docker push $(FULL_IMAGE)

.PHONY: all
all: install build push
	@echo "Successfully built and pushed $(FULL_IMAGE)"

.PHONY: clean
clean:
	@echo "Removing local image: $(FULL_IMAGE)"
	docker rmi $(FULL_IMAGE) || true

.PHONY: dev
dev:
	@echo "Starting Backstage in development mode..."
	node .yarn/releases/yarn-4.4.1.cjs start

.PHONY: tsc
tsc:
	@echo "Running TypeScript compiler..."
	node .yarn/releases/yarn-4.4.1.cjs tsc
