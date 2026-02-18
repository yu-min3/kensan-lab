"""Common utilities for maintenance pipeline HTTP triggers."""

from __future__ import annotations

import requests

from catalog.config import setup_logging

logger = setup_logging("maintenance.common")


def trigger_kensan_ai_endpoint(
    endpoint: str,
    base_url: str,
    params: dict | None = None,
    timeout: int = 600,
) -> dict:
    """Generic HTTP trigger for kensan-ai admin endpoints.

    Args:
        endpoint: API path (e.g. "/admin/reindex-pending")
        base_url: kensan-ai base URL (e.g. "http://localhost:8089")
        params: Optional query parameters
        timeout: Request timeout in seconds

    Returns:
        JSON response dict from kensan-ai

    Raises:
        requests.HTTPError: If the request fails
    """
    url = f"{base_url}/api/v1{endpoint}"

    logger.info(f"Calling {endpoint}: {url}")
    resp = requests.post(url, params=params, timeout=timeout)
    resp.raise_for_status()

    return resp.json()
