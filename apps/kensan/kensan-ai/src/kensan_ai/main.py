"""FastAPI application for Kensan AI service."""

from contextlib import asynccontextmanager
from typing import AsyncIterator

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from kensan_ai.api import router
from kensan_ai.config import get_settings
from kensan_ai.db import get_pool, close_pool
from kensan_ai.errors import ToolError
from kensan_ai.telemetry import (
    TelemetryConfig,
    initialize_telemetry,
    instrument_asyncpg,
    instrument_fastapi,
    instrument_httpx,
    shutdown_telemetry,
)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    """Manage application lifecycle."""
    settings = get_settings()

    # Initialize OpenTelemetry before other components
    initialize_telemetry(TelemetryConfig(
        enabled=settings.otel_enabled,
        collector_url=settings.otel_collector_url,
        service_name="kensan-ai",
    ))
    if settings.otel_enabled:
        instrument_asyncpg()
        instrument_httpx()

    # Startup: Initialize DB pool
    await get_pool()
    yield
    # Shutdown: Close DB pool
    await close_pool()
    shutdown_telemetry()


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    settings = get_settings()

    app = FastAPI(
        title="Kensan AI",
        description="AI service for Kensan learning management app",
        version="0.1.0",
        lifespan=lifespan,
    )

    # CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],  # Configure appropriately for production
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
        expose_headers=["traceparent", "tracestate"],
    )

    # Instrument FastAPI with OpenTelemetry
    if settings.otel_enabled:
        instrument_fastapi(app)

    # Include API routes with /api/v1 prefix for frontend compatibility
    app.include_router(router, prefix="/api/v1")

    # Root-level health endpoint for Docker healthcheck
    @app.get("/health")
    async def root_health():
        return {"status": "ok", "version": "0.1.0"}

    # Exception handler for ToolError
    @app.exception_handler(ToolError)
    async def tool_error_handler(request: Request, exc: ToolError) -> JSONResponse:
        """Handle ToolError exceptions with structured error response."""
        status_code = 500
        if exc.code == "VALIDATION_ERROR":
            status_code = 400
        elif exc.code == "NOT_FOUND":
            status_code = 404
        elif exc.code == "AUTHENTICATION_ERROR":
            status_code = 401
        elif exc.code == "AUTHORIZATION_ERROR":
            status_code = 403

        return JSONResponse(
            status_code=status_code,
            content={"error": exc.to_dict()},
        )

    return app


# Create the app instance
app = create_app()


if __name__ == "__main__":
    import uvicorn

    settings = get_settings()
    uvicorn.run(
        "kensan_ai.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
    )
