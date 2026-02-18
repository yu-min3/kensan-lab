"""Time block queries."""

from datetime import datetime
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_time_blocks(
    user_id: UUID,
    start_datetime: datetime | None = None,
    end_datetime: datetime | None = None,
) -> list[dict[str, Any]]:
    """Get time blocks for a user with optional datetime filters.

    Args:
        user_id: The user ID.
        start_datetime: Filter blocks starting from this datetime (inclusive, UTC).
        end_datetime: Filter blocks starting before this datetime (exclusive, UTC).
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

        blocks = await conn.fetch(
            f"""
            SELECT
                id, start_datetime, end_datetime, task_id, task_name,
                milestone_id, milestone_name, goal_id, goal_name, goal_color
            FROM time_blocks
            WHERE {where_clause}
            ORDER BY start_datetime
            """,
            *params,
        )

        return [
            {
                "id": str(block["id"]),
                "startDatetime": block["start_datetime"].isoformat(),
                "endDatetime": block["end_datetime"].isoformat(),
                "taskId": str(block["task_id"]) if block["task_id"] else None,
                "taskName": block["task_name"],
                "milestone": {
                    "id": str(block["milestone_id"]),
                    "name": block["milestone_name"],
                } if block["milestone_id"] else None,
                "goal": {
                    "id": str(block["goal_id"]),
                    "name": block["goal_name"],
                    "color": block["goal_color"],
                } if block["goal_id"] else None,
            }
            for block in blocks
        ]


async def create_time_block(
    user_id: UUID,
    start_datetime: datetime,
    end_datetime: datetime,
    task_name: str,
    task_id: UUID | None = None,
    milestone_id: UUID | None = None,
    goal_id: UUID | None = None,
) -> dict[str, Any]:
    """Create a new time block.

    Denormalized fields (goal_name, goal_color, milestone_name) are
    automatically resolved from goal_id/milestone_id via subqueries.
    """
    async with get_connection() as conn:
        block = await conn.fetchrow(
            """
            INSERT INTO time_blocks (
                user_id, start_datetime, end_datetime, task_id, task_name,
                milestone_id, milestone_name, goal_id, goal_name, goal_color
            )
            VALUES (
                $1, $2, $3, $4, $5,
                $6, (SELECT name FROM milestones WHERE id = $6),
                $7, (SELECT name FROM goals WHERE id = $7), (SELECT color FROM goals WHERE id = $7)
            )
            RETURNING id, start_datetime, end_datetime, task_id, task_name,
                milestone_id, milestone_name, goal_id, goal_name, goal_color
            """,
            user_id,
            start_datetime,
            end_datetime,
            task_id,
            task_name,
            milestone_id,
            goal_id,
        )

        return {
            "id": str(block["id"]),
            "startDatetime": block["start_datetime"].isoformat(),
            "endDatetime": block["end_datetime"].isoformat(),
            "taskId": str(block["task_id"]) if block["task_id"] else None,
            "taskName": block["task_name"],
            "milestone": {
                "id": str(block["milestone_id"]),
                "name": block["milestone_name"],
            } if block["milestone_id"] else None,
            "goal": {
                "id": str(block["goal_id"]),
                "name": block["goal_name"],
                "color": block["goal_color"],
            } if block["goal_id"] else None,
        }


async def update_time_block(
    time_block_id: UUID,
    user_id: UUID,
    start_datetime: datetime | None = None,
    end_datetime: datetime | None = None,
    task_name: str | None = None,
) -> dict[str, Any] | None:
    """Update an existing time block."""
    async with get_connection() as conn:
        updates = []
        params: list[Any] = []
        param_idx = 1

        if start_datetime is not None:
            updates.append(f"start_datetime = ${param_idx}")
            params.append(start_datetime)
            param_idx += 1

        if end_datetime is not None:
            updates.append(f"end_datetime = ${param_idx}")
            params.append(end_datetime)
            param_idx += 1

        if task_name is not None:
            updates.append(f"task_name = ${param_idx}")
            params.append(task_name)
            param_idx += 1

        if not updates:
            return None

        params.extend([time_block_id, user_id])
        set_clause = ", ".join(updates)

        block = await conn.fetchrow(
            f"""
            UPDATE time_blocks
            SET {set_clause}
            WHERE id = ${param_idx} AND user_id = ${param_idx + 1}
            RETURNING id, start_datetime, end_datetime, task_id, task_name,
                milestone_id, milestone_name, goal_id, goal_name, goal_color
            """,
            *params,
        )

        if block is None:
            return None

        return {
            "id": str(block["id"]),
            "startDatetime": block["start_datetime"].isoformat(),
            "endDatetime": block["end_datetime"].isoformat(),
            "taskId": str(block["task_id"]) if block["task_id"] else None,
            "taskName": block["task_name"],
        }


async def delete_time_block(time_block_id: UUID, user_id: UUID) -> bool:
    """Delete a time block. Returns True if deleted."""
    async with get_connection() as conn:
        result = await conn.execute(
            "DELETE FROM time_blocks WHERE id = $1 AND user_id = $2",
            time_block_id,
            user_id,
        )
        return result == "DELETE 1"
