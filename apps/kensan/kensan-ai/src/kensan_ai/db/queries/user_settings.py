"""User settings queries."""

from uuid import UUID
from zoneinfo import ZoneInfo

from kensan_ai.db.connection import get_connection

# Fallback timezone when user has no setting
_FALLBACK_TZ = "Asia/Tokyo"


async def get_user_timezone(user_id: UUID) -> ZoneInfo:
    """Get the user's timezone from user_settings, falling back to Asia/Tokyo."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            "SELECT timezone FROM user_settings WHERE user_id = $1",
            user_id,
        )
        tz_name = row["timezone"] if row and row["timezone"] else _FALLBACK_TZ
        try:
            return ZoneInfo(tz_name)
        except (KeyError, ValueError):
            return ZoneInfo(_FALLBACK_TZ)
