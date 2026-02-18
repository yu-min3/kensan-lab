"""Analytics queries."""

from datetime import datetime
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_analytics_summary(
    user_id: UUID,
    period: str,
    start_datetime: datetime,
    end_datetime: datetime,
) -> dict[str, Any]:
    """Get analytics summary for a period (weekly or monthly).

    Args:
        user_id: The user ID.
        period: Period label (e.g., "weekly", "monthly").
        start_datetime: Start of the period (inclusive, UTC).
        end_datetime: End of the period (exclusive, UTC).

    Returns total hours and goal-based breakdown.
    """
    async with get_connection() as conn:
        # Total time from time entries
        rows = await conn.fetch(
            """
            SELECT
                goal_name,
                goal_color,
                COUNT(*) as entry_count,
                SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 3600) as total_hours
            FROM time_entries
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
            GROUP BY goal_name, goal_color
            ORDER BY total_hours DESC
            """,
            user_id,
            start_datetime,
            end_datetime,
        )

        goal_breakdown = []
        total_hours = 0.0

        for row in rows:
            hours = float(row["total_hours"] or 0)
            total_hours += hours
            goal_breakdown.append({
                "goalName": row["goal_name"] or "未分類",
                "goalColor": row["goal_color"],
                "hours": round(hours, 1),
                "entryCount": row["entry_count"],
            })

        return {
            "period": period,
            "startDatetime": start_datetime.isoformat(),
            "endDatetime": end_datetime.isoformat(),
            "totalHours": round(total_hours, 1),
            "goalBreakdown": goal_breakdown,
        }


async def get_daily_summary(
    user_id: UUID,
    start_datetime: datetime,
    end_datetime: datetime,
) -> dict[str, Any]:
    """Get a summary of planned vs actual time for a datetime range.

    Args:
        user_id: The user ID.
        start_datetime: Start of the day (inclusive, UTC).
        end_datetime: End of the day (exclusive, UTC).
    """
    async with get_connection() as conn:
        # Planned time blocks
        blocks = await conn.fetch(
            """
            SELECT task_name, goal_name, start_datetime, end_datetime
            FROM time_blocks
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
            ORDER BY start_datetime
            """,
            user_id,
            start_datetime,
            end_datetime,
        )

        planned = []
        planned_hours = 0.0
        for b in blocks:
            hours = (b["end_datetime"] - b["start_datetime"]).total_seconds() / 3600.0
            planned_hours += hours
            planned.append({
                "taskName": b["task_name"],
                "goalName": b["goal_name"],
                "startDatetime": b["start_datetime"].isoformat(),
                "endDatetime": b["end_datetime"].isoformat(),
                "hours": round(hours, 1),
            })

        # Actual time entries
        entries = await conn.fetch(
            """
            SELECT task_name, goal_name, start_datetime, end_datetime
            FROM time_entries
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
            ORDER BY start_datetime
            """,
            user_id,
            start_datetime,
            end_datetime,
        )

        actual = []
        actual_hours = 0.0
        for e in entries:
            hours = (e["end_datetime"] - e["start_datetime"]).total_seconds() / 3600.0
            actual_hours += hours
            actual.append({
                "taskName": e["task_name"],
                "goalName": e["goal_name"],
                "startDatetime": e["start_datetime"].isoformat(),
                "endDatetime": e["end_datetime"].isoformat(),
                "hours": round(hours, 1),
            })

        return {
            "startDatetime": start_datetime.isoformat(),
            "endDatetime": end_datetime.isoformat(),
            "planned": planned,
            "actual": actual,
            "plannedHours": round(planned_hours, 1),
            "actualHours": round(actual_hours, 1),
        }
