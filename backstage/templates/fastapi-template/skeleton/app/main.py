"""Generated from the Backstage template.

The service is a small FastAPI backend
that also serves a React frontend built into the image.

Two endpoints carry the interesting part:

  /api/config — the values somebody typed into the scaffolder form. They are
                read from the environment, which the platform fills from
                deploy/values.yaml. Changing the look of this service is a pull
                request, not a rebuild.

  /api/whoami — who is calling. The application has no authentication code: the
                gateway checks with the identity provider first and forwards the
                answer in x-auth-request-* headers.
"""

import os
import time
from pathlib import Path

from fastapi import FastAPI, Request
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest
from starlette.responses import Response

app = FastAPI(
    title=os.getenv("APP_NAME", "${{ values.name }}"),
    description=os.getenv("APP_DESCRIPTION", "${{ values.description }}"),
    version="1.0.0",
)

# Prometheus metrics
REQUEST_COUNT = Counter(
    "http_requests_total", "Total HTTP requests", ["method", "endpoint", "status"]
)
REQUEST_LATENCY = Histogram(
    "http_request_duration_seconds", "HTTP request latency", ["method", "endpoint"]
)

# The identity the gateway forwards once oauth2-proxy has verified a session.
# Listed rather than globbed so the page shows exactly what the platform
# promises and nothing else that happens to be on the request.
IDENTITY_HEADERS = (
    "x-auth-request-user",
    "x-auth-request-email",
    "x-auth-request-groups",
)

STATIC_DIR = Path(__file__).parent / "static"


@app.middleware("http")
async def add_metrics(request, call_next):
    """Middleware to collect Prometheus metrics."""
    start_time = time.time()
    response = await call_next(request)
    duration = time.time() - start_time

    REQUEST_COUNT.labels(
        method=request.method, endpoint=request.url.path, status=response.status_code
    ).inc()
    REQUEST_LATENCY.labels(method=request.method, endpoint=request.url.path).observe(duration)

    return response


@app.get("/health")
async def health_check():
    """Health check endpoint for Kubernetes liveness/readiness probes."""
    return {"status": "healthy"}


@app.get("/metrics")
async def metrics():
    """Prometheus metrics endpoint."""
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.get("/api/config")
async def config():
    """What the scaffolder form asked for, as it stands in the cluster right now.

    APP_THEME and APP_MESSAGE come from deploy/values.yaml by way of the
    Deployment's environment. Editing that file and letting Argo CD sync is the
    whole update path — the image is not involved.
    """
    theme = os.getenv("APP_THEME", "day")
    if theme not in ("day", "night"):
        theme = "day"
    return {
        "appName": os.getenv("APP_NAME", "${{ values.name }}"),
        "theme": theme,
        "message": os.getenv("APP_MESSAGE", "Hello from ${{ values.name }}"),
    }


@app.get("/api/whoami")
async def whoami(request: Request):
    """The caller's identity, as the gateway reported it.

    Absent headers are not an error: reaching the pod directly, or running with
    the gateway gate switched off, is a legitimate way to see this service.
    """
    present = {h: request.headers[h] for h in IDENTITY_HEADERS if h in request.headers}
    groups = present.get("x-auth-request-groups", "")
    return {
        "authenticated": bool(present),
        "user": present.get("x-auth-request-user"),
        "email": present.get("x-auth-request-email"),
        "groups": [g.strip() for g in groups.split(",") if g.strip()],
        "headers": present,
    }


@app.get("/api/v1/example")
async def example_endpoint():
    """Example API endpoint."""
    return {"data": "This is an example response", "timestamp": time.time()}


# Mounted last so the routes above win. The directory is absent when running
# from a checkout without building the frontend, and the service still starts —
# only the page is missing, which is the right failure for `make dev`.
if STATIC_DIR.is_dir():
    app.mount("/assets", StaticFiles(directory=STATIC_DIR / "assets"), name="assets")

    @app.get("/")
    async def index():
        return FileResponse(STATIC_DIR / "index.html")


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)
