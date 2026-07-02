"""SSE (Server-Sent Events) response helpers."""

import json
from typing import Any


def sse_event(event_type: str, data: dict[str, Any]) -> str:
    """Format a single SSE event block.

    Args:
        event_type: The event type (e.g., "text", "tool_call", "action_proposal", "done")
        data: The event data to serialize as JSON

    Returns:
        Formatted SSE event string
    """
    return f"event: {event_type}\ndata: {json.dumps(data, ensure_ascii=False, default=str)}\n\n"


def sse_keepalive() -> str:
    """Format a keepalive SSE event to prevent client timeouts."""
    return "event: keepalive\ndata: {}\n\n"
