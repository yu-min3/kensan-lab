"""Pattern analysis tools for AI agent."""

from typing import Any

from kensan_ai.tools.base import tool
from kensan_ai.db.queries.patterns import get_user_patterns as db_get_user_patterns
from kensan_ai.db.queries.user_settings import get_user_timezone
from kensan_ai.lib.parsers import parse_uuid


@tool(
    category="pattern",
    name="get_user_patterns",
    description="ユーザーの行動パターンを取得する（生産性ピーク時間、計画精度、目標トレンド等）。計画提案の根拠として使う。",
    input_schema={
        "properties": {
            "lookback_weeks": {
                "type": "integer",
                "description": "分析期間（週数）。デフォルト4、最大12",
            },
        },
        "required": [],
    },
)
async def get_user_patterns(args: dict[str, Any]) -> dict[str, Any]:
    """Get user behavior patterns for planning recommendations."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    user_tz = await get_user_timezone(user_id)
    weeks = min(args.get("lookback_weeks", 4), 12)

    patterns = await db_get_user_patterns(user_id, weeks, user_tz)
    return {"patterns": patterns}


ALL_PATTERN_TOOLS = [
    get_user_patterns,
]
