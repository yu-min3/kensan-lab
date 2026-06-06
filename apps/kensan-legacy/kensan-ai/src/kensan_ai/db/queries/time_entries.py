"""Time entry queries."""

from datetime import datetime
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_time_entries(
    user_id: UUID,
    start_datetime: datetime | None = None,
    end_datetime: datetime | None = None,
) -> list[dict[str, Any]]:
    """Get time entries for a user with optional datetime filters.

    Args:
        user_id: The user ID.
        start_datetime: Filter entries starting from this datetime (inclusive, UTC).
        end_datetime: Filter entries starting before this datetime (exclusive, UTC).
    """
    async with get_connection() as conn:
        conditions = ["user_id = $1"]
        params: list[Any] = [user_id]
        param_idx = 2

        if start_datetime is not None and end_datetime is not None:
            conditions.append(
                f"start_datetime >= ${param_idx} AND start_datetime < ${param_idx + 1}"
            )
            params.extend([start_datetime, end_datetime])
            param_idx += 2

        where_clause = " AND ".join(conditions)

        entries = await conn.fetch(
            f"""
            SELECT
                id, start_datetime, end_datetime, task_id, task_name,
                milestone_id, milestone_name, goal_id, goal_name, goal_color,
                description
            FROM time_entries
            WHERE {where_clause}
            ORDER BY start_datetime DESC
            """,
            *params,
        )

        return [
            {
                "id": str(entry["id"]),
                "startDatetime": entry["start_datetime"].isoformat(),
                "endDatetime": entry["end_datetime"].isoformat(),
                "taskId": str(entry["task_id"]) if entry["task_id"] else None,
                "taskName": entry["task_name"],
                "milestone": {
                    "id": str(entry["milestone_id"]),
                    "name": entry["milestone_name"],
                } if entry["milestone_id"] else None,
                "goal": {
                    "id": str(entry["goal_id"]),
                    "name": entry["goal_name"],
                    "color": entry["goal_color"],
                } if entry["goal_id"] else None,
                "description": entry["description"],
            }
            for entry in entries
        ]
