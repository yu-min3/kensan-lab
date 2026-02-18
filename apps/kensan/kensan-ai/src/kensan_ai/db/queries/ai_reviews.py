"""AI review report queries."""

from datetime import date
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def create_review(
    user_id: UUID,
    period_start: date,
    period_end: date,
    summary: str,
    good_points: list[str],
    improvement_points: list[str],
    advice: list[str],
    tokens_input: int | None = None,
    tokens_output: int | None = None,
) -> dict[str, Any]:
    """Create a new AI review report."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO ai_review_reports (
                user_id, period_start, period_end, summary,
                good_points, improvement_points, advice,
                tokens_input, tokens_output
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
            RETURNING id, user_id, period_start, period_end, summary,
                good_points, improvement_points, advice, created_at
            """,
            user_id,
            period_start,
            period_end,
            summary,
            good_points,
            improvement_points,
            advice,
            tokens_input,
            tokens_output,
        )
        return _row_to_dict(row)


async def list_reviews(
    user_id: UUID,
    start_date: date | None = None,
    end_date: date | None = None,
) -> list[dict[str, Any]]:
    """List AI review reports for a user."""
    async with get_connection() as conn:
        conditions = ["user_id = $1"]
        params: list[Any] = [user_id]
        param_idx = 2

        if start_date:
            conditions.append(f"period_start >= ${param_idx}")
            params.append(start_date)
            param_idx += 1

        if end_date:
            conditions.append(f"period_end <= ${param_idx}")
            params.append(end_date)
            param_idx += 1

        where_clause = " AND ".join(conditions)

        rows = await conn.fetch(
            f"""
            SELECT id, user_id, period_start, period_end, summary,
                good_points, improvement_points, advice, created_at
            FROM ai_review_reports
            WHERE {where_clause}
            ORDER BY period_start DESC
            """,
            *params,
        )
        return [_row_to_dict(row) for row in rows]


async def get_review(review_id: UUID, user_id: UUID) -> dict[str, Any] | None:
    """Get a specific AI review report."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            SELECT id, user_id, period_start, period_end, summary,
                good_points, improvement_points, advice, created_at
            FROM ai_review_reports
            WHERE id = $1 AND user_id = $2
            """,
            review_id,
            user_id,
        )
        if row:
            return _row_to_dict(row)
        return None


def _row_to_dict(row: Any) -> dict[str, Any]:
    """Convert a database row to a dictionary."""
    return {
        "id": str(row["id"]),
        "userId": str(row["user_id"]),
        "periodStart": row["period_start"].isoformat(),
        "periodEnd": row["period_end"].isoformat(),
        "summary": row["summary"] or "",
        "goodPoints": list(row["good_points"]) if row["good_points"] else [],
        "improvementPoints": list(row["improvement_points"]) if row["improvement_points"] else [],
        "advice": list(row["advice"]) if row["advice"] else [],
        "createdAt": row["created_at"].isoformat(),
    }
