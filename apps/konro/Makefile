# konro — build / push（apps/kensan/Makefile と同型）
-include ../../.env    # repo root .env（WRITE_GHCR_PAT / GITHUB_USER）
CONTAINER_RUNTIME ?= docker
IMAGE ?= ghcr.io/yu-min3/konro
TAG ?= dev
PLATFORMS ?= linux/amd64,linux/arm64
WRITE_GHCR_PAT ?= $(GITHUB_GHCR_PAT)

.PHONY: help login test build k8s-build

help:
	@echo "test       - backend go test + frontend tsc/vite build"
	@echo "build      - ローカル arch の image build（検証用、push なし）"
	@echo "login      - GHCR login（WRITE_GHCR_PAT / write:packages を使用）"
	@echo "k8s-build  - multi-arch build + GHCR push（要 make login か docker login）"

login:
	@echo "$(WRITE_GHCR_PAT)" | $(CONTAINER_RUNTIME) login ghcr.io -u $${GITHUB_USER:-yu-min3} --password-stdin

test:
	cd backend && go vet ./... && go test ./...
	cd frontend && npm run build

build:
	$(CONTAINER_RUNTIME) build -f Dockerfile -t $(IMAGE):$(TAG) ../..

k8s-build:
	$(CONTAINER_RUNTIME) buildx build -f Dockerfile \
		--platform=$(PLATFORMS) \
		-t $(IMAGE):$(TAG) \
		--push ../..
