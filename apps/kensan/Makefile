# kensan — build / push（gitops-workflow.md: multi-arch manifest list を default）
CONTAINER_RUNTIME ?= docker
IMAGE ?= ghcr.io/yu-min3/kensan
TAG ?= dev
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help test build k8s-build

help:
	@echo "test       - backend go test + frontend tsc/vite build"
	@echo "build      - ローカル arch の image build（検証用、push なし）"
	@echo "k8s-build  - multi-arch build + GHCR push（要 docker login ghcr.io）"

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
