"""Situation detector for determining context based on explicit specification."""

from datetime import datetime
from enum import Enum
from zoneinfo import ZoneInfo


class Situation(str, Enum):
    """Possible situations for AI context."""

    CHAT = "chat"
    REVIEW = "review"
    DAILY_ADVICE = "daily_advice"
    PERSONA = "persona"


def detect_situation(
    explicit_situation: str | None = None,
    timezone: str = "Asia/Tokyo",
) -> Situation:
    """Detect the appropriate situation based on explicit specification.

    Args:
        explicit_situation: Explicitly specified situation
        timezone: User's timezone (reserved for future use)

    Returns:
        Specified Situation, or CHAT as default
    """
    if explicit_situation:
        try:
            return Situation(explicit_situation.lower())
        except ValueError:
            pass  # Unknown situation, fall through to default

    return Situation.CHAT


def is_weekend(timezone: str = "Asia/Tokyo") -> bool:
    """Check if current day is weekend (for weekly review suggestion).

    Args:
        timezone: User's timezone

    Returns:
        True if Saturday or Sunday
    """
    try:
        tz = ZoneInfo(timezone)
    except Exception:
        tz = ZoneInfo("Asia/Tokyo")

    now = datetime.now(tz)
    return now.weekday() >= 5  # Saturday = 5, Sunday = 6
