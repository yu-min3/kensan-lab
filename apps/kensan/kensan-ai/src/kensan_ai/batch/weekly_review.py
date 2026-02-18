"""Weekly review auto-generation batch job.

Generates weekly reviews for all active users by invoking the agent with
the REVIEW context. Follows the same admin-endpoint pattern as reindex_pending.
"""

from __future__ import annotations

import logging
from datetime import date, timedelta

from kensan_ai.lib.timezone_utils import local_today
from uuid import UUID

from kensan_ai.agents import create_agent_runner
from kensan_ai.agents.chat import select_tools, TOOL_GROUPS
from kensan_ai.context import Situation, ContextResolver
from kensan_ai.db.connection import get_connection

logger = logging.getLogger(__name__)


def _last_week_range() -> tuple[date, date]:
    """Return (monday, sunday) of the previous week."""
    today = local_today()
    # Monday of this week
    monday_this_week = today - timedelta(days=today.weekday())
    # Last week
    period_start = monday_this_week - timedelta(days=7)
    period_end = period_start + timedelta(days=6)
    return period_start, period_end


async def _get_active_user_ids(period_start: date, period_end: date) -> list[UUID]:
    """Get user IDs that had ai_interactions during the given period."""
    async with get_connection() as conn:
        rows = await conn.fetch(
            """
            SELECT DISTINCT user_id
            FROM ai_interactions
            WHERE created_at >= $1::date
              AND created_at < ($2::date + INTERVAL '1 day')
            """,
            period_start,
            period_end,
        )
    return [row["user_id"] for row in rows]


async def _has_existing_review(user_id: UUID, period_start: date) -> bool:
    """Check if a review already exists for this user and period."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            SELECT id FROM ai_review_reports
            WHERE user_id = $1 AND period_start = $2
            LIMIT 1
            """,
            user_id,
            period_start,
        )
    return row is not None


async def generate_review_for_user(
    user_id: UUID,
    period_start: date,
    period_end: date,
) -> dict:
    """Generate a weekly review for a single user.

    Returns:
        {"user_id": ..., "status": "generated"|"skipped", ...}
    """
    # 1. Skip if review already exists
    if await _has_existing_review(user_id, period_start):
        logger.info("Review already exists for user=%s period_start=%s, skipping", user_id, period_start)
        return {
            "user_id": str(user_id),
            "status": "skipped",
            "reason": "already_exists",
        }

    # 2. Resolve REVIEW context
    context = await ContextResolver.get_context(Situation.REVIEW, user_id=user_id)
    if not context:
        logger.warning("No REVIEW context found for user=%s", user_id)
        return {
            "user_id": str(user_id),
            "status": "skipped",
            "reason": "no_context",
        }

    # 3. Select tools — add analytics group for batch (no frontend context)
    allowed_tools = select_tools(
        "週次レビュー生成",
        context.allowed_tools,
        "review",
        prompt_variables=context.prompt_variables,
    )
    # Ensure analytics tools are available (batch has no frontend-provided data)
    for tool_name in TOOL_GROUPS.get("analytics", []):
        if tool_name in context.allowed_tools and tool_name not in allowed_tools:
            allowed_tools.append(tool_name)

    # 4. Create agent
    agent = create_agent_runner(
        system_prompt=context.system_prompt,
        allowed_tools=allowed_tools,
        max_turns=context.max_turns,
        temperature=context.temperature,
        context_id=str(context.id),
        context_name=context.name,
        context_version=context.version,
    )

    # 5. Run agent (non-streaming)
    start_str = period_start.isoformat()
    end_str = period_end.isoformat()
    message = (
        f"{start_str}〜{end_str}の週次振り返りレビューを生成してください。\n\n"
        f"## 手順\n"
        f"1. get_notes(type=\"diary\", start_date=\"{start_str}\", end_date=\"{end_str}\") と "
        f"get_notes(type=\"learning\", start_date=\"{start_str}\", end_date=\"{end_str}\") で記録を取得\n"
        f"2. get_analytics_summary(period=\"weekly\", start_date=\"{start_str}\", end_date=\"{end_str}\") で稼働状況を確認\n"
        f"3. データを分析し、generate_review ツールで保存\n\n"
        f"## generate_review の各フィールド要件\n"
        f"- period_start: \"{start_str}\"\n"
        f"- period_end: \"{end_str}\"\n"
        f"- summary: 週全体の概要を3-5文で（必須。空にしない。成果・課題・来週への展望に加え、学習記録や日記があればその内容にも自然に織り込むこと。改行で段落分けしてよい）\n"
        f"- good_points: 具体的なデータに基づいた良かった点（3項目程度。学習内容や日記の気づきにも触れてよい）\n"
        f"- improvement_points: 具体的な改善点（3項目程度）\n"
        f"- advice: 来週への具体的なアクション提案（3項目程度。学習の方向性や生活面のアドバイスも含めてよい）"
    )

    result = await agent.run(message, user_id=str(user_id))

    logger.info(
        "Review generated for user=%s: tokens_in=%d tokens_out=%d tool_calls=%d",
        user_id,
        result.tokens_input,
        result.tokens_output,
        len(result.tool_calls),
    )

    return {
        "user_id": str(user_id),
        "status": "generated",
        "tokens_input": result.tokens_input,
        "tokens_output": result.tokens_output,
        "tool_calls": len(result.tool_calls),
    }


async def generate_weekly_reviews() -> dict:
    """Generate weekly reviews for all active users.

    Returns:
        Summary dict with counts of processed/generated/skipped/errors.
    """
    period_start, period_end = _last_week_range()
    logger.info("Generating weekly reviews for %s ~ %s", period_start, period_end)

    user_ids = await _get_active_user_ids(period_start, period_end)
    if not user_ids:
        logger.info("No active users found for the period")
        return {
            "period_start": period_start.isoformat(),
            "period_end": period_end.isoformat(),
            "users_processed": 0,
            "reviews_generated": 0,
            "skipped": 0,
            "errors": 0,
        }

    logger.info("Found %d active users", len(user_ids))

    generated = 0
    skipped = 0
    errors = 0

    for uid in user_ids:
        try:
            result = await generate_review_for_user(uid, period_start, period_end)
            if result["status"] == "generated":
                generated += 1
            else:
                skipped += 1
        except Exception:
            logger.exception("Failed to generate review for user=%s", uid)
            errors += 1

    summary = {
        "period_start": period_start.isoformat(),
        "period_end": period_end.isoformat(),
        "users_processed": len(user_ids),
        "reviews_generated": generated,
        "skipped": skipped,
        "errors": errors,
    }
    logger.info("Weekly review batch complete: %s", summary)
    return summary
