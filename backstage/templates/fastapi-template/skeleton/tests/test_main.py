"""
Tests for the FastAPI application.
"""
import os
import sys

from fastapi.testclient import TestClient

# Add app directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'app'))

from main import app

client = TestClient(app)


def test_health_check():
    """Test health endpoint."""
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "healthy"


def test_metrics():
    """Test Prometheus metrics endpoint."""
    response = client.get("/metrics")
    assert response.status_code == 200
    assert "http_requests_total" in response.text


def test_config_defaults():
    """The page falls back to a usable configuration when nothing is set."""
    response = client.get("/api/config")
    assert response.status_code == 200
    data = response.json()
    assert data["theme"] in ("day", "night")
    assert data["message"]
    assert data["appName"]


def test_config_reads_the_environment(monkeypatch):
    """deploy/values.yaml reaches the page through these variables."""
    monkeypatch.setenv("APP_THEME", "night")
    monkeypatch.setenv("APP_MESSAGE", "good evening")
    data = client.get("/api/config").json()
    assert data["theme"] == "night"
    assert data["message"] == "good evening"


def test_config_rejects_an_unknown_theme(monkeypatch):
    """A typo in values.yaml must not render an unstyled page."""
    monkeypatch.setenv("APP_THEME", "twilight")
    assert client.get("/api/config").json()["theme"] == "day"


def test_whoami_without_the_gateway():
    """Reaching the pod directly is legitimate and must not error."""
    data = client.get("/api/whoami").json()
    assert data["authenticated"] is False
    assert data["groups"] == []


def test_whoami_reads_the_gateway_headers():
    """The application owns no auth code; it reports what it was handed."""
    data = client.get(
        "/api/whoami",
        headers={
            "x-auth-request-user": "demo",
            "x-auth-request-email": "demo-admin@example.com",
            "x-auth-request-groups": "platform-admin, developers",
        },
    ).json()
    assert data["authenticated"] is True
    assert data["user"] == "demo"
    assert data["email"] == "demo-admin@example.com"
    assert data["groups"] == ["platform-admin", "developers"]


def test_example_endpoint():
    """Test example API endpoint."""
    response = client.get("/api/v1/example")
    assert response.status_code == 200
    data = response.json()
    assert "data" in data
    assert "timestamp" in data
