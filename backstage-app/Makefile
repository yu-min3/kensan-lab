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
	@echo "  make install       - Install dependencies"
	@echo "  make build         - Build backend and create Docker image"
	@echo "  make push          - Login to GHCR and push image"
	@echo "  make all           - Install, build, and push image"
	@echo "  make update-image  - Build, push, and update kustomization.yaml (no git commit)"
	@echo "  make deploy        - GitOps deployment: build, push, update, commit, push, sync"
	@echo "  make commit-deploy - Commit kustomization.yaml changes only (after update-image)"
	@echo "  make preview       - Preview Kustomize manifests without deploying"
	@echo "  make clean         - Remove Docker image locally"
	@echo ""
	@echo "Development Commands:"
	@echo "  make dev           - Run Backstage locally (production mode, port 7007)"
	@echo "  make dev-watch     - Run with hot reload (frontend:3000, backend:7007)"
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
	@echo "Build context: parent directory (to include app-templates)"
	cd .. && $(CONTAINER_RUNTIME) build --format=docker -f backstage-app/packages/backend/Dockerfile -t $(FULL_IMAGE) .

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

.PHONY: update-image
update-image: build push
	@echo "Updating Kustomize overlay with tag: $(TAG)..."
	cd ../base-infra/backstage/overlays/prod && \
		kustomize edit set image ghcr.io/$(GITHUB_USER)/backstage:$(TAG)
	@echo ""
	@echo "✅ Image built, pushed, and kustomization.yaml updated"
	@echo "   Image: $(FULL_IMAGE)"
	@echo ""
	@echo "Next steps:"
	@echo "  - Review changes: git diff ../base-infra/backstage/overlays/prod/kustomization.yaml"
	@echo "  - Deploy via GitOps: make deploy TAG=$(TAG)"
	@echo "  - Or commit manually: make commit-deploy TAG=$(TAG)"

.PHONY: deploy
deploy: build push
	@echo "Updating Kustomize overlay with tag: $(TAG)..."
	cd ../base-infra/backstage/overlays/prod && \
		kustomize edit set image ghcr.io/$(GITHUB_USER)/backstage:$(TAG)
	@echo "Committing change to Git..."
	cd .. && git add base-infra/backstage/overlays/prod/kustomization.yaml
	cd .. && git commit -m "Deploy Backstage $(TAG)"
	cd .. && git push
	@echo "Triggering Argo CD sync..."
	argocd app sync backstage --async || echo "Warning: argocd CLI not available, please sync manually"
	@echo ""
	@echo "✅ GitOps deployment triggered for $(FULL_IMAGE)"
	@echo ""
	@echo "Monitor deployment:"
	@echo "  argocd app get backstage"
	@echo "  kubectl get pods -n backstage -w"
	@echo "  kubectl logs -n backstage deployment/backstage -f"

.PHONY: commit-deploy
commit-deploy:
	@echo "Committing Kustomize overlay change..."
	cd .. && git add base-infra/backstage/overlays/prod/kustomization.yaml
	cd .. && git commit -m "Deploy Backstage $(TAG)"
	cd .. && git push
	@echo "✅ Committed and pushed"
	@echo "Argo CD will sync automatically or run: argocd app sync backstage"

.PHONY: preview
preview:
	@echo "Previewing deployment with tag: $(TAG)..."
	@cd ../base-infra/backstage/overlays/prod && \
		kustomize edit set image ghcr.io/$(GITHUB_USER)/backstage:$(TAG)
	@echo ""
	@echo "Generated Deployment (image section):"
	@kubectl kustomize ../base-infra/backstage/overlays/prod | grep -A10 "kind: Deployment" | grep -A5 "containers:"
	@echo ""
	@echo "Full manifests:"
	@echo "  kubectl kustomize ../base-infra/backstage/overlays/prod"
