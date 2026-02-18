# Build stage
FROM python:3.12-slim AS builder

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install dependencies only (cached when pyproject.toml unchanged)
COPY pyproject.toml README.md ./
RUN mkdir -p src/kensan_ai && touch src/kensan_ai/__init__.py
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --system .

# Copy actual source code (doesn't invalidate dependency layer)
COPY src/ ./src/

# Runtime stage
FROM python:3.12-slim

WORKDIR /app

# Install runtime dependencies (curl for healthcheck)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy installed packages from builder
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy application code
COPY src/ ./src/

# Set PYTHONPATH so the package can be found
ENV PYTHONPATH=/app/src

# Create non-root user
RUN useradd -m -u 1000 appuser
USER appuser

EXPOSE 8089

CMD ["uvicorn", "kensan_ai.main:app", "--host", "0.0.0.0", "--port", "8089"]
