# Backstage Image Build and Push Makefile
# Load environment variables from .env file if it exists
-include ../.env
export

# Default values
REGISTRY ?= ghcr.io
IMAGE_NAME ?= $(GITHUB_USER)/backstage
TAG ?= latest
FULL_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(TAG)

# Container runtime (podman or docker)
CONTAINER_RUNTIME ?= podman

.PHONY: help
help:
	@echo "Backstage Build and Push Commands:"
	@echo "  make install      - Install dependencies"
	@echo "  make build        - Build backend and create Docker image"
	@echo "  make push         - Login to GHCR and push image"
	@echo "  make all          - Install, build, and push image"
	@echo "  make deploy       - Build, push, and deploy to Kubernetes via Kustomize"
	@echo "  make commit-deploy - Commit Kustomize overlay change for GitOps"
	@echo "  make preview      - Preview Kustomize manifests without deploying"
	@echo "  make clean        - Remove Docker image locally"
	@echo ""
	@echo "Development Commands:"
	@echo "  make dev          - Run Backstage locally (production mode, port 7007)"
	@echo "  make dev-watch    - Run with hot reload (frontend:3000, backend:7007)"
	@echo ""
	@echo "Environment Variables (set in ../.env):"
	@echo "  GITHUB_USER          - GitHub username"
	@echo "  GITHUB_GHCR_PAT      - GitHub Personal Access Token for GHCR"
	@echo "  TAG                  - Image tag (default: latest)"
	@echo "  CONTAINER_RUNTIME    - Container runtime (default: podman, can use: docker)"

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
	@echo "Building container image: $(FULL_IMAGE)"
	@echo "Using container runtime: $(CONTAINER_RUNTIME)"
	$(CONTAINER_RUNTIME) build --format=docker -f packages/backend/Dockerfile -t $(FULL_IMAGE) .

.PHONY: login
login:
	@echo "Logging in to GHCR..."
	@echo "$(GITHUB_GHCR_PAT)" | $(CONTAINER_RUNTIME) login ghcr.io -u $(GITHUB_USER) --password-stdin

.PHONY: push
push: login
	@echo "Pushing image: $(FULL_IMAGE)"
	$(CONTAINER_RUNTIME) push $(FULL_IMAGE)

.PHONY: all
all: install build push
	@echo "Successfully built and pushed $(FULL_IMAGE)"

.PHONY: clean
clean:
	@echo "Removing local image: $(FULL_IMAGE)"
	$(CONTAINER_RUNTIME) rmi $(FULL_IMAGE) || true

.PHONY: dev
dev: build-backend
	@echo "Extracting backend bundle..."
	@cd packages/backend/dist && tar -xzf bundle.tar.gz 2>/dev/null
	@echo "Starting Backstage backend (with frontend on port 7007)..."
	@echo "Access at: http://localhost:7007"
	@echo ""
	@echo "Press Ctrl+C to stop"
	@echo ""
	cd packages/backend/dist/packages/backend && \
		node dist/index.cjs.js \
		--config $(CURDIR)/app-config.yaml \
		--config $(CURDIR)/app-config.local.yaml

.PHONY: dev-watch
dev-watch:
	@echo "Starting Backstage in development mode with hot reload..."
	@echo "Frontend: http://localhost:3000 (Webpack Dev Server)"
	@echo "Backend:  http://localhost:7007 (API)"
	@echo ""
	node .yarn/releases/yarn-4.4.1.cjs start

.PHONY: tsc
tsc:
	@echo "Running TypeScript compiler..."
	node .yarn/releases/yarn-4.4.1.cjs tsc

.PHONY: deploy
deploy: build push
	@echo "Updating Kustomize overlay with tag: $(TAG)..."
	cd ../base-infra/backstage/overlays/current && \
		kustomize edit set image ghcr.io/$(GITHUB_USER)/backstage:$(TAG)
	@echo "Deploying via Kustomize..."
	kubectl apply -k ../base-infra/backstage/overlays/current
	@echo "Waiting for rollout to complete..."
	kubectl rollout status deployment/backstage -n backstage --timeout=5m
	@echo ""
	@echo "✅ Deployed $(FULL_IMAGE)"
	@echo ""
	@echo "Check status:"
	@echo "  kubectl get pods -n backstage"
	@echo "  kubectl logs -n backstage deployment/backstage -f"
	@echo ""
	@echo "To commit this change (GitOps):"
	@echo "  make commit-deploy"

.PHONY: commit-deploy
commit-deploy:
	@echo "Committing Kustomize overlay change..."
	cd .. && git add base-infra/backstage/overlays/current/kustomization.yaml
	cd .. && git commit -m "Deploy Backstage $(TAG)"
	cd .. && git push
	@echo "✅ Committed and pushed"
	@echo "Argo CD will sync automatically or run: argocd app sync backstage"

.PHONY: preview
preview:
	@echo "Previewing deployment with tag: $(TAG)..."
	@cd ../base-infra/backstage/overlays/current && \
		kustomize edit set image ghcr.io/$(GITHUB_USER)/backstage:$(TAG)
	@echo ""
	@echo "Generated Deployment (image section):"
	@kubectl kustomize ../base-infra/backstage/overlays/current | grep -A10 "kind: Deployment" | grep -A5 "containers:"
	@echo ""
	@echo "Full manifests:"
	@echo "  kubectl kustomize ../base-infra/backstage/overlays/current"
