# Backstage Build and Deployment Makefile
# Load environment variables from .env file if it exists
-include .env
export

# GHCR token split (.env で使い分ける):
#   WRITE_GHCR_PAT (write:packages) — image push / GHCR login
#   READ_GHCR_PAT  (read:packages / repo read) — Backstage 実行時 GITHUB_TOKEN
#                  （github url reader / catalog-info fetch / TechDocs）
# 旧 GITHUB_GHCR_PAT は後方互換の fallback として両方に流用する。
WRITE_GHCR_PAT ?= $(GITHUB_GHCR_PAT)
READ_GHCR_PAT  ?= $(GITHUB_GHCR_PAT)
GITHUB_TOKEN   ?= $(READ_GHCR_PAT)
export GITHUB_TOKEN

# Default values
REGISTRY ?= ghcr.io
GITHUB_USER ?= your-org
IMAGE_NAME ?= $(GITHUB_USER)/backstage
TAG ?= latest
FULL_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(TAG)

# Container runtime (podman or docker)
CONTAINER_RUNTIME ?= docker

# Platform for cross-build (e.g., linux/amd64)
PLATFORM ?= linux/amd64

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
	@echo "GitOps Deployment (for kensan-lab):"
	@echo "  make update-image  - Build, push, and update kubernetes/backstage/backstage-deployment.yaml"
	@echo "  make commit-deploy - Commit the manifest change (run inside a worktree)"
	@echo "  make preview       - Show current image in the manifest"
	@echo "  (deploy 手順: worktree を切る → update-image → commit-deploy → gh pr create)"
	@echo ""
	@echo "Environment Variables (set in .env):"
	@echo "  GITHUB_USER        - GitHub username"
	@echo "  WRITE_GHCR_PAT     - PAT (write:packages) for image push / login"
	@echo "  READ_GHCR_PAT      - PAT (read) for runtime GITHUB_TOKEN (dev/run)"
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
	@echo "Build context: backstage/ (this directory)"
	@echo "Note: This may take several minutes and use significant memory"
	$(CONTAINER_RUNTIME) build \
		--platform $(PLATFORM) \
		-f packages/backend/Dockerfile \
		-t $(FULL_IMAGE) \
		.

.PHONY: login
login:
	@echo "Logging in to GHCR..."
	@echo "$(WRITE_GHCR_PAT)" | $(CONTAINER_RUNTIME) login ghcr.io -u $(GITHUB_USER) --password-stdin

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
# GitOps Deployment (kensan-lab specific)
#==============================================================================

## Flat manifest 運用 (kustomize overlays は retired)。image tag は
## kubernetes/backstage/backstage-deployment.yaml を直接書き換える (ADR-018 で移設)。
MANIFEST := ../kubernetes/backstage/backstage-deployment.yaml

.PHONY: update-image
update-image: build push
	@echo "Updating manifest with tag: $(TAG)..."
	sed -i.bak -E 's|(image: ghcr\.io/$(GITHUB_USER)/backstage:).*|\1$(TAG)|' $(MANIFEST)
	rm -f $(MANIFEST).bak
	@echo ""
	@echo "✅ Image built, pushed, and manifest updated"
	@echo "   Image: $(FULL_IMAGE)"
	@echo ""
	@echo "Next steps:"
	@echo "  - Review changes: git diff $(MANIFEST)"
	@echo "  - Commit & push (inside a worktree): make commit-deploy TAG=$(TAG)"

## NOTE: 旧 deploy target は inner repo で直接 `git checkout -b` していたが、
## worktree 必須ルール (.claude/rules/gitops-workflow.md) と衝突するため廃止。
## build/push + manifest 更新は update-image、commit/push は worktree 内で commit-deploy を使う。
.PHONY: deploy
deploy:
	@echo "❌ 'make deploy' は廃止 (inner repo での直接 checkout は worktree ルール違反)"
	@echo ""
	@echo "代わりに:"
	@echo "  1. worktree を切る:  git worktree add ~/kensan-lab.worktrees/deploy-backstage-\$$TAG -b deploy-backstage-\$$TAG origin/main"
	@echo "  2. その worktree の backstage で:  make update-image TAG=<tag>"
	@echo "  3. 同 worktree で:  make commit-deploy TAG=<tag> && gh pr create"
	@exit 1

.PHONY: commit-deploy
commit-deploy:
	@echo "Committing manifest change..."
	git add $(MANIFEST)
	git commit -m "Deploy Backstage $(TAG)"
	## fresh worktree branch には upstream が無いため -u + HEAD で明示 push
	git push -u origin HEAD
	@echo "✅ Committed and pushed"
	@echo "Argo CD will sync automatically or run: argocd app sync backstage"

.PHONY: preview
preview:
	@echo "Current image in manifest:"
	@grep -n "image: ghcr.io/$(GITHUB_USER)/backstage" $(MANIFEST)
	@echo ""
	@echo "With TAG=$(TAG): ghcr.io/$(GITHUB_USER)/backstage:$(TAG)"
