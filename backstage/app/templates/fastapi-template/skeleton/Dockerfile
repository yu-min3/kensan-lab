# syntax=docker/dockerfile:1
# Multi-stage build for optimized FastAPI application with uv

FROM docker.io/library/python:3.11-slim AS builder

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

WORKDIR /build

# Copy dependency files
COPY pyproject.toml uv.lock ./

# Install dependencies into .venv
RUN uv sync --frozen --no-install-project --no-dev

# Install OpenTelemetry auto-instrumentation libraries
RUN /build/.venv/bin/opentelemetry-bootstrap -a install

# Final stage
FROM docker.io/library/python:3.11-slim

WORKDIR /app

# Copy uv for runtime (if needed for future extensions)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/uv

# Copy virtual environment from builder
COPY --from=builder /build/.venv /app/.venv

# Copy application code
COPY app/ /app/

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
