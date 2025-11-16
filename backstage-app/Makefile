# Backstage Build and Deployment Makefile
# Load environment variables from .env file if it exists
-include ../.env
export

# For Backstage compatibility: use GITHUB_GHCR_PAT as GITHUB_TOKEN if not set
GITHUB_TOKEN ?= $(GITHUB_GHCR_PAT)
export GITHUB_TOKEN

# Default values
REGISTRY ?= ghcr.io
GITHUB_USER ?= your-org
IMAGE_NAME ?= $(GITHUB_USER)/backstage
TAG ?= latest
FULL_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(TAG)

# Container runtime (podman or docker)
CONTAINER_RUNTIME ?= podman

# Yarn wrapper
YARN := node .yarn/releases/yarn-4.4.1.cjs

#==============================================================================
# Help
#==============================================================================

.PHONY: help
help:
	@echo "Backstage Development Commands:"
	@echo "  make install       - Install dependencies"
	@echo "  make dev           - Run Backstage locally (port 7007)"
	@echo ""
	@echo "Testing & Code Quality:"
	@echo "  make tsc           - TypeScript type checking"
	@echo "  make lint          - Run ESLint"
	@echo "  make test          - Run unit tests"
	@echo "  make test-e2e      - Run Playwright E2E tests"
	@echo ""
	@echo "Build & Deploy:"
	@echo "  make build         - Build Docker image"
	@echo "  make push          - Build and Push image to GHCR"
	@echo "  make run           - Run built image locally"
	@echo "  make clean         - Remove local Docker image"
	@echo ""
	@echo "GitOps Deployment (for goldship-platform):"
	@echo "  make update-image  - Build, push, and update kustomization.yaml"
	@echo "  make deploy        - Create PR for deployment (build, push, branch, commit)"
	@echo "  make commit-deploy - Commit kustomization.yaml changes only"
	@echo "  make preview       - Preview Kustomize manifests"
	@echo ""
	@echo "Environment Variables (set in ../.env):"
	@echo "  GITHUB_USER        - GitHub username"
	@echo "  GITHUB_GHCR_PAT    - GitHub Personal Access Token for GHCR"
	@echo "  TAG                - Image tag (default: latest)"

#==============================================================================
# Development
#==============================================================================

.PHONY: install
install:
	@echo "Installing dependencies..."
	$(YARN) install

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
		NODE_OPTIONS=--no-node-snapshot node dist/index.cjs.js \
		--config $(CURDIR)/app-config.yaml \
		--config $(CURDIR)/app-config.local.yaml

#==============================================================================
# Testing & Code Quality
#==============================================================================

.PHONY: tsc
tsc:
	@echo "Running TypeScript type checking..."
	$(YARN) tsc

.PHONY: lint
lint:
	@echo "Running ESLint..."
	$(YARN) lint:all

.PHONY: test
test:
	@echo "Running unit tests..."
	$(YARN) test:all

.PHONY: test-e2e
test-e2e:
	@echo "Running E2E tests with Playwright..."
	$(YARN) test-e2e

#==============================================================================
# Build & Deploy
#==============================================================================

.PHONY: build-backend
build-backend:
	@echo "Building backend package..."
	$(YARN) workspace backend build

.PHONY: build
build: build-backend
	@echo "Building container image: $(FULL_IMAGE)"
	@echo "Using container runtime: $(CONTAINER_RUNTIME)"
	@echo "Build context: parent directory (to include templates/)"
	@echo "Note: This may take several minutes and use significant memory"
	cd .. && $(CONTAINER_RUNTIME) build \
		--format=docker \
		-f backstage-app/packages/backend/Dockerfile \
		-t $(FULL_IMAGE) \
		.

.PHONY: login
login:
	@echo "Logging in to GHCR..."
	@echo "$(GITHUB_GHCR_PAT)" | $(CONTAINER_RUNTIME) login ghcr.io -u $(GITHUB_USER) --password-stdin

.PHONY: push
push: login build
	@echo "Pushing image: $(FULL_IMAGE)"
	$(CONTAINER_RUNTIME) push $(FULL_IMAGE)

.PHONY: run
run: build
	@echo "Running container image for local testing..."
	@echo "Access at: http://localhost:7007"
	@echo ""
	$(CONTAINER_RUNTIME) run --rm -it -p 7007:7007 \
		-v $(CURDIR)/app-config.local.yaml:/app/app-config.local.yaml \
		-e GITHUB_TOKEN="$(GITHUB_TOKEN)" \
		$(FULL_IMAGE) \
		node packages/backend --config app-config.yaml --config app-config.kubernetes.yaml --config app-config.local.yaml

.PHONY: clean
clean:
	@echo "Removing local image: $(FULL_IMAGE)"
	$(CONTAINER_RUNTIME) rmi $(FULL_IMAGE) || true

#==============================================================================
# GitOps Deployment (goldship-platform specific)
#==============================================================================

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
deploy: update-image
	@echo "Creating deployment branch..."
	cd .. && git checkout -b deploy-backstage-$(TAG)
	@echo "Committing change to Git..."
	cd .. && git add base-infra/backstage/overlays/prod/kustomization.yaml
	cd .. && git commit -m "Deploy Backstage $(TAG)"
	@echo "Pushing to remote..."
	cd .. && git push origin deploy-backstage-$(TAG)
	@echo ""
	@echo "✅ Deployment branch created and pushed: deploy-backstage-$(TAG)"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Create a Pull Request:"
	@echo "     gh pr create --base main --head deploy-backstage-$(TAG) --title 'Deploy Backstage $(TAG)' --body 'Update Backstage image to $(TAG)'"
	@echo ""
	@echo "  2. Merge the PR:"
	@echo "     - Review and approve the PR on GitHub"
	@echo "     - Merge to main"
	@echo "     - Argo CD will automatically sync the changes"
	@echo ""
	@echo "  3. Monitor deployment after merge:"
	@echo "     kubectl get pods -n backstage -w"

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
	@kubectl kustomize ../base-infra/backstage/overlays/prod | grep -A10 "kind: Deployment" | grep -A5 "containers:" || echo "  (Could not extract container section)"
	@echo ""
	@echo "Full manifests:"
	@echo "  kubectl kustomize ../base-infra/backstage/overlays/prod"
