"""Shared timezone conversion utilities."""

from __future__ import annotations

from datetime import date, datetime, time, timedelta
from zoneinfo import ZoneInfo

DEFAULT_TZ = ZoneInfo("Asia/Tokyo")
_UTC = ZoneInfo("UTC")


def local_today(tz: ZoneInfo = DEFAULT_TZ) -> date:
    """Return today's date in the given timezone (default: Asia/Tokyo).

    Unlike ``date.today()`` which uses the server's system timezone,
    this always returns the correct local date regardless of the host TZ.
    """
    return datetime.now(tz).date()


def local_date_to_utc_range(
    target_date: date,
    tz: ZoneInfo = DEFAULT_TZ,
) -> tuple[datetime, datetime]:
    """Convert a local date to a UTC datetime range (start inclusive, end exclusive)."""
    start_local = datetime(target_date.year, target_date.month, target_date.day, tzinfo=tz)
    end_local = start_local + timedelta(days=1)
    return start_local.astimezone(_UTC), end_local.astimezone(_UTC)


def combine_to_utc(
    target_date: date,
    local_time: time,
    tz: ZoneInfo = DEFAULT_TZ,
) -> datetime:
    """Combine a local date and time into a UTC datetime."""
    local_dt = datetime.combine(target_date, local_time, tzinfo=tz)
    return local_dt.astimezone(_UTC)


def to_local_str(utc_iso: str | None, tz: ZoneInfo = DEFAULT_TZ) -> str | None:
    """Convert a UTC ISO 8601 string to a readable local datetime string.

    Returns a human-readable format like '2026-02-08 09:00' in the given timezone.
    """
    if not utc_iso:
        return utc_iso
    dt = datetime.fromisoformat(utc_iso)
    return dt.astimezone(tz).strftime("%Y-%m-%d %H:%M")
