# Local verification only; publication belongs to the host Release Gate.
IMAGE ?= sense-canary:local

.PHONY: build test lint
build:
	docker build -t $(IMAGE) .

test:
	uv sync --frozen --all-extras --no-install-project
	uv run --frozen --no-sync pytest tests/ -v

lint:
	uv sync --frozen --all-extras --no-install-project
	uv run --frozen --no-sync ruff check app/
	uv run --frozen --no-sync ruff format --check app/
