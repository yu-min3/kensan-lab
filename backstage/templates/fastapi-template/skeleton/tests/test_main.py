"""
Tests for the FastAPI application.
"""
from fastapi.testclient import TestClient
import sys
import os

# Add app directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'app'))

from main import app

client = TestClient(app)


def test_read_root():
    """Test root endpoint."""
    response = client.get("/")
    assert response.status_code == 200
    data = response.json()
    assert "message" in data
    assert "status" in data
    assert data["status"] == "running"


def test_health_check():
    """Test health check endpoint."""
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "healthy"


def test_metrics():
    """Test metrics endpoint."""
    response = client.get("/metrics")
    assert response.status_code == 200
    assert "http_requests_total" in response.text


def test_example_endpoint():
    """Test example API endpoint."""
    response = client.get("/api/v1/example")
    assert response.status_code == 200
    data = response.json()
    assert "data" in data
    assert "timestamp" in data
