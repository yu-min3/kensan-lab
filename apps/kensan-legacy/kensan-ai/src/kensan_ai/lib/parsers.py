"""Shared parser utilities for common data types."""

from datetime import date, time
from uuid import UUID


def parse_uuid(value: str | None) -> UUID | None:
    """Parse a string to UUID, returning None if invalid or empty.

    Args:
        value: The string value to parse.

    Returns:
        UUID if valid, None otherwise.
    """
    if not value:
        return None
    try:
        return UUID(value)
    except (ValueError, TypeError):
        return None


def parse_date(value: str | None) -> date | None:
    """Parse an ISO date string (YYYY-MM-DD), returning None if invalid or empty.

    Args:
        value: The date string to parse.

    Returns:
        date if valid, None otherwise.
    """
    if not value:
        return None
    try:
        return date.fromisoformat(value)
    except (ValueError, TypeError):
        return None


def parse_time(value: str | None) -> time | None:
    """Parse a time string (HH:MM or HH:MM:SS), returning None if invalid or empty.

    Supports both HH:MM and HH:MM:SS formats.

    Args:
        value: The time string to parse.

    Returns:
        time if valid, None otherwise.
    """
    if not value:
        return None
    try:
        # Try standard ISO format first (HH:MM:SS)
        return time.fromisoformat(value)
    except (ValueError, TypeError):
        pass
    try:
        # Try HH:MM format
        parts = value.split(":")
        if len(parts) >= 2:
            return time(int(parts[0]), int(parts[1]))
    except (ValueError, TypeError, IndexError):
        pass
    return None


def require_uuid(value: str | None, field_name: str = "id") -> UUID:
    """Parse a string to UUID, raising ValueError if invalid or missing.

    Args:
        value: The string value to parse.
        field_name: The field name for error messages.

    Returns:
        UUID if valid.

    Raises:
        ValueError: If the value is invalid or missing.
    """
    result = parse_uuid(value)
    if result is None:
        raise ValueError(f"Invalid or missing {field_name}")
    return result


def require_date(value: str | None, field_name: str = "date") -> date:
    """Parse a date string, raising ValueError if invalid or missing.

    Args:
        value: The date string to parse.
        field_name: The field name for error messages.

    Returns:
        date if valid.

    Raises:
        ValueError: If the value is invalid or missing.
    """
    result = parse_date(value)
    if result is None:
        raise ValueError(f"Invalid or missing {field_name}")
    return result
