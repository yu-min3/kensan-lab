# syntax=docker/dockerfile:1
# Multi-stage build: a React frontend built to static files, served by a
# FastAPI backend.
#
# The frontend is built from each generated repository. What somebody chose in
# the scaffolder form is still runtime configuration, but edits to either the
# Python or React source now produce a new immutable image.

FROM docker.io/library/node:22-slim AS frontend

WORKDIR /frontend

# Dependencies first, so a change to the source does not re-resolve them.
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund

COPY frontend/ ./
RUN npm run build

FROM docker.io/library/python:3.11-slim AS builder

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# The builder works in the same directory the runtime image uses. uv writes an
# absolute shebang into every console script in the virtualenv, so a venv built
# under a different prefix and copied to /app leaves those entry points pointing
# at an interpreter that no longer exists — the container then exits with
# "exec /app/.venv/bin/opentelemetry-instrument: no such file or directory".
WORKDIR /app

# Copy dependency files
COPY pyproject.toml uv.lock ./

# Install dependencies into .venv
RUN uv sync --frozen --no-install-project --no-dev

# Install OpenTelemetry auto-instrumentation libraries
RUN /app/.venv/bin/opentelemetry-bootstrap -a install

# CI asks Docker to build this target first. Keeping tests in the same build
# graph as the runtime image means the disposable runner needs only Docker and
# the exact dependency lock the service owns.
FROM builder AS test
COPY app/ /app/app/
COPY tests/ /app/tests/
RUN uv sync --frozen --no-install-project
RUN uv run ruff check app/ && uv run ruff format --check app/
RUN uv run pytest tests/ -v

# Final stage
FROM docker.io/library/python:3.11-slim

WORKDIR /app

# Copy uv for runtime (if needed for future extensions)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/uv

# Copy virtual environment from builder
COPY --from=builder /app/.venv /app/.venv

# Copy application code
COPY app/ /app/

# The built frontend. main.py mounts it only if the directory exists, so a
# checkout without a frontend build still starts — that is what `make dev`
# relies on.
COPY --from=frontend /frontend/dist /app/static

# Create non-root user
RUN useradd -m -u 1000 appuser && \
    chown -R appuser:appuser /app

USER appuser

# Activate virtual environment
ENV PATH="/app/.venv/bin:$PATH"
ENV VIRTUAL_ENV="/app/.venv"

# Expose port
EXPOSE 8000

# Run the application with OpenTelemetry auto-instrumentation
CMD ["opentelemetry-instrument", "python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
