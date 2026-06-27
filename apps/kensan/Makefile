# kensan — build / push（gitops-workflow.md: multi-arch manifest list を default）
CONTAINER_RUNTIME ?= docker
IMAGE ?= ghcr.io/yu-min3/kensan
TAG ?= dev
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help test build k8s-build deploy

help:
	@echo "test       - backend go test + frontend tsc/vite build"
	@echo "build      - ローカル arch の image build（検証用、push なし）"
	@echo "k8s-build  - multi-arch build + GHCR push（要 docker login ghcr.io）"
	@echo "deploy     - 1発デプロイ: test→build/push→tag bump→PR（VERSION=v0.2.0 MERGE=1）"

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

# 入口は make、実ロジックは deploy.sh（分岐/arg/prompt は shell が素直なため）
deploy:
	./deploy.sh $(VERSION) $(if $(MERGE),--merge)
