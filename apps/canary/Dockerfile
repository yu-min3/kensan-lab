# syntax=docker/dockerfile@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
# Multi-stage build: a React frontend built to static files, served by a
# FastAPI backend.
#
# The frontend is built from each generated repository. What somebody chose in
# the scaffolder form is still runtime configuration, but edits to either the
# Python or React source now produce a new immutable image.

FROM docker.io/library/node@sha256:43ac6c60b8f89723f746e8a92ce91abd5017e627ce1ddfe4238355d3a30b772c AS frontend

WORKDIR /frontend

# Dependencies first, so a change to the source does not re-resolve them.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY frontend/ ./
RUN npm run build

FROM docker.io/library/python@sha256:6f31d6e9ba2b0a787a3f81c37b004155b87b9efa1b771182bd550c1615745be5 AS builder

# Install uv
COPY --from=ghcr.io/astral-sh/uv@sha256:61d393e44e249f2e4b526b6c7ddcecce245946826e608e11c93ad4f5bba55b21 /uv /uvx /bin/

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

# Private canary disables OTEL; avoid installing instrumentation outside uv.lock.

# CI asks Docker to build this target first. Keeping tests in the same build
# graph as the runtime image means the disposable runner needs only Docker and
# the exact dependency lock the service owns.
FROM builder AS test
COPY README.md /app/README.md
COPY app/ /app/app/
COPY tests/ /app/tests/
RUN uv sync --frozen --no-install-project --all-extras
RUN uv run --frozen --no-sync ruff check app/ && uv run --frozen --no-sync ruff format --check app/
RUN uv run --frozen --no-sync pytest tests/ -v

# Final stage
FROM docker.io/library/python@sha256:6f31d6e9ba2b0a787a3f81c37b004155b87b9efa1b771182bd550c1615745be5

WORKDIR /app

# Copy uv for runtime (if needed for future extensions)
COPY --from=ghcr.io/astral-sh/uv@sha256:61d393e44e249f2e4b526b6c7ddcecce245946826e608e11c93ad4f5bba55b21 /uv /bin/uv

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
