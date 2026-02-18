"""Review tools for AI agent."""

from typing import Any

from kensan_ai.tools.base import tool
from kensan_ai.db.queries import ai_reviews
from kensan_ai.lib.parsers import parse_uuid, parse_date


@tool(
    category="review",
    name="get_reviews",
    description="過去のAI週次レビュー一覧を取得する。",
    input_schema={
        "properties": {
            "limit": {"type": "integer", "description": "取得件数（デフォルト10）"},
        },
        "required": [],
    },
)
async def get_reviews(args: dict[str, Any]) -> dict[str, Any]:
    """Get list of AI review reports."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}
    reviews = await ai_reviews.list_reviews(user_id)
    limit = args.get("limit", 10)
    return {"reviews": reviews[:limit]}


@tool(
    category="review",
    name="get_review",
    description="特定のAI週次レビューを取得する。事前に get_reviews でIDを確認すること。",
    input_schema={
        "properties": {
            "review_id": {"type": "string", "description": "レビューID"},
        },
        "required": ["review_id"],
    },
)
async def get_review(args: dict[str, Any]) -> dict[str, Any]:
    """Get a specific AI review report."""
    user_id = parse_uuid(args.get("user_id"))
    review_id = parse_uuid(args.get("review_id"))
    if not user_id or not review_id:
        return {"error": "Invalid or missing user_id or review_id"}
    review = await ai_reviews.get_review(review_id, user_id)
    if review is None:
        return {"error": "Review not found"}
    return {"review": review}


@tool(
    category="review",
    name="generate_review",
    description="指定した期間のレビューを生成しDBに保存する。事前に get_analytics_summary と get_time_entries で期間のデータを取得してから生成すること。",
    readonly=False,
    input_schema={
        "properties": {
            "period_start": {"type": "string", "description": "期間の開始日 (YYYY-MM-DD)"},
            "period_end": {"type": "string", "description": "期間の終了日 (YYYY-MM-DD)"},
            "summary": {"type": "string", "description": "期間の概要"},
            "good_points": {"type": "array", "items": {"type": "string"}, "description": "良かった点"},
            "improvement_points": {"type": "array", "items": {"type": "string"}, "description": "改善点"},
            "advice": {"type": "array", "items": {"type": "string"}, "description": "次の期間へのアドバイス"},
        },
        "required": ["period_start", "period_end", "summary", "good_points", "improvement_points", "advice"],
    },
)
async def generate_review(args: dict[str, Any]) -> dict[str, Any]:
    """Generate and save a review report for the specified period."""
    user_id = parse_uuid(args.get("user_id"))
    period_start = parse_date(args.get("period_start"))
    period_end = parse_date(args.get("period_end"))

    if not user_id or not period_start or not period_end:
        return {"error": "Invalid or missing user_id, period_start, or period_end"}

    review = await ai_reviews.create_review(
        user_id=user_id,
        period_start=period_start,
        period_end=period_end,
        summary=args.get("summary", ""),
        good_points=args.get("good_points", []),
        improvement_points=args.get("improvement_points", []),
        advice=args.get("advice", []),
    )
    return {"review": review}


ALL_REVIEW_TOOLS = [
    get_reviews,
    get_review,
    generate_review,
]
