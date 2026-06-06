"""Analytics tools for AI agent."""

from datetime import date, datetime, timedelta
from typing import Any
from zoneinfo import ZoneInfo

from kensan_ai.tools.base import tool
from kensan_ai.db.queries.analytics import (
    get_analytics_summary as db_get_analytics_summary,
    get_daily_summary as db_get_daily_summary,
)
from kensan_ai.db.queries.user_settings import get_user_timezone
from kensan_ai.lib.parsers import parse_uuid, parse_date
from kensan_ai.lib.timezone_utils import local_today


def _local_date_to_utc_range(
    target_date: date,
    tz: ZoneInfo,
) -> tuple[datetime, datetime]:
    """Convert a local date to a UTC datetime range (start inclusive, end exclusive)."""
    start_local = datetime(target_date.year, target_date.month, target_date.day, tzinfo=tz)
    end_local = start_local + timedelta(days=1)
    return start_local.astimezone(ZoneInfo("UTC")), end_local.astimezone(ZoneInfo("UTC"))


def _to_local(utc_iso: str | None, tz: ZoneInfo) -> str | None:
    """Convert a UTC ISO 8601 string to a readable local datetime string."""
    if not utc_iso:
        return utc_iso
    dt = datetime.fromisoformat(utc_iso)
    return dt.astimezone(tz).strftime("%Y-%m-%d %H:%M")


def _localize_summary(summary: dict, tz: ZoneInfo) -> dict:
    """Convert all datetime fields in a summary dict to local timezone."""
    result = dict(summary)
    for key in ("startDatetime", "endDatetime"):
        if key in result:
            result[key] = _to_local(result[key], tz)
    for list_key in ("planned", "actual"):
        if list_key in result and isinstance(result[list_key], list):
            result[list_key] = [
                {**item, "startDatetime": _to_local(item.get("startDatetime"), tz),
                 "endDatetime": _to_local(item.get("endDatetime"), tz)}
                if "startDatetime" in item else item
                for item in result[list_key]
            ]
    return result


@tool(
    category="analytics",
    name="get_analytics_summary",
    description="週次または月次の稼働サマリーを取得する。目標別の時間配分を確認する。",
    input_schema={
        "properties": {
            "period": {"type": "string", "enum": ["weekly", "monthly"], "description": "集計期間"},
            "start_date": {"type": "string", "description": "開始日 (YYYY-MM-DD)"},
            "end_date": {"type": "string", "description": "終了日 (YYYY-MM-DD)"},
        },
        "required": ["period", "start_date", "end_date"],
    },
)
async def get_analytics_summary(args: dict[str, Any]) -> dict[str, Any]:
    """Get analytics summary for a period."""
    user_id = parse_uuid(args.get("user_id"))
    start = parse_date(args.get("start_date"))
    end = parse_date(args.get("end_date"))
    if not user_id or not start or not end:
        return {"error": "Invalid or missing user_id, start_date, or end_date"}

    user_tz = await get_user_timezone(user_id)

    # Convert local dates to UTC datetime range
    start_dt, _ = _local_date_to_utc_range(start, user_tz)
    _, end_dt = _local_date_to_utc_range(end, user_tz)

    summary = await db_get_analytics_summary(
        user_id=user_id,
        period=args.get("period", "weekly"),
        start_datetime=start_dt,
        end_datetime=end_dt,
    )
    return {"summary": _localize_summary(summary, user_tz)}


@tool(
    category="analytics",
    name="get_daily_summary",
    description="特定日の時間配分サマリーを取得する。計画vs実績の比較に使う。",
    input_schema={
        "properties": {
            "date": {"type": "string", "description": "日付 (YYYY-MM-DD)。省略時は今日"},
        },
        "required": [],
    },
)
async def get_daily_summary(args: dict[str, Any]) -> dict[str, Any]:
    """Get daily summary with planned vs actual comparison."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    user_tz = await get_user_timezone(user_id)

    target_date = parse_date(args.get("date"))
    if not target_date:
        target_date = local_today(user_tz)

    # Convert local date to UTC datetime range
    start_dt, end_dt = _local_date_to_utc_range(target_date, user_tz)

    summary = await db_get_daily_summary(
        user_id=user_id,
        start_datetime=start_dt,
        end_datetime=end_dt,
    )
    return {"summary": _localize_summary(summary, user_tz)}


ALL_ANALYTICS_TOOLS = [
    get_analytics_summary,
    get_daily_summary,
]
