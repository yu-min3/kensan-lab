"""Prompt evaluation queries (extracted from prompt_experiments.py)."""

import json
import logging
from datetime import date
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection

logger = logging.getLogger(__name__)


async def create_evaluation(
    context_id: UUID,
    period_start: date,
    period_end: date,
    interaction_count: int,
    avg_rating: float | None,
    rated_count: int,
    tool_success_rate: float | None,
    avg_turns: float | None,
    avg_tokens: float | None,
    strengths: list[str],
    weaknesses: list[str],
    improvement_suggestions: list[str],
    sample_analysis: dict | None = None,
    user_id: UUID | None = None,
) -> UUID | None:
    """Create a prompt evaluation. Returns None if duplicate (ON CONFLICT DO NOTHING)."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO prompt_evaluations (
                context_id, period_start, period_end,
                interaction_count, avg_rating, rated_count,
                tool_success_rate, avg_turns, avg_tokens,
                strengths, weaknesses, improvement_suggestions,
                sample_analysis, user_id
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
            ON CONFLICT DO NOTHING
            RETURNING id
            """,
            context_id,
            period_start,
            period_end,
            interaction_count,
            avg_rating,
            rated_count,
            tool_success_rate,
            avg_turns,
            avg_tokens,
            strengths,
            weaknesses,
            improvement_suggestions,
            json.dumps(sample_analysis or {}, ensure_ascii=False),
            user_id,
        )
        if row is None:
            return None
        return row["id"]


async def get_evaluation(evaluation_id: UUID) -> dict[str, Any] | None:
    """Get a single evaluation by ID."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            SELECT id, context_id, period_start, period_end,
                   interaction_count, avg_rating, rated_count,
                   tool_success_rate, avg_turns, avg_tokens,
                   strengths, weaknesses, improvement_suggestions,
                   sample_analysis, created_at
            FROM prompt_evaluations
            WHERE id = $1
            """,
            evaluation_id,
        )
        if row is None:
            return None
        return _evaluation_row_to_dict(row)


def _evaluation_row_to_dict(row: Any) -> dict[str, Any]:
    """Convert an evaluation DB row to a dictionary."""
    sample_analysis = row["sample_analysis"]
    if isinstance(sample_analysis, str):
        sample_analysis = json.loads(sample_analysis)

    return {
        "id": str(row["id"]),
        "context_id": str(row["context_id"]),
        "period_start": row["period_start"].isoformat(),
        "period_end": row["period_end"].isoformat(),
        "interaction_count": row["interaction_count"],
        "avg_rating": row["avg_rating"],
        "rated_count": row["rated_count"],
        "tool_success_rate": row["tool_success_rate"],
        "avg_turns": row["avg_turns"],
        "avg_tokens": row["avg_tokens"],
        "strengths": list(row["strengths"]) if row["strengths"] else [],
        "weaknesses": list(row["weaknesses"]) if row["weaknesses"] else [],
        "improvement_suggestions": list(row["improvement_suggestions"]) if row["improvement_suggestions"] else [],
        "sample_analysis": sample_analysis,
        "created_at": row["created_at"].isoformat(),
    }
