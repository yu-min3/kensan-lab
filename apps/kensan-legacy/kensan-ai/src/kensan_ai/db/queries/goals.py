"""Goal and milestone queries."""

from datetime import date
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_goals_and_milestones(user_id: UUID) -> list[dict[str, Any]]:
    """Get all goals with their milestones and task counts for a user."""
    async with get_connection() as conn:
        # Get goals
        goals = await conn.fetch(
            """
            SELECT id, name, description, color, status, created_at, updated_at
            FROM goals
            WHERE user_id = $1 AND status != 'archived'
            ORDER BY created_at DESC
            """,
            user_id,
        )

        result = []
        for goal in goals:
            # Get milestones for this goal
            milestones = await conn.fetch(
                """
                SELECT id, name, description, target_date, status, created_at, updated_at
                FROM milestones
                WHERE goal_id = $1 AND status != 'archived'
                ORDER BY target_date NULLS LAST, created_at
                """,
                goal["id"],
            )

            milestone_list = []
            for milestone in milestones:
                # Get task counts for this milestone
                task_counts = await conn.fetchrow(
                    """
                    SELECT
                        COUNT(*) as total,
                        COUNT(*) FILTER (WHERE completed = true) as completed
                    FROM tasks
                    WHERE milestone_id = $1
                    """,
                    milestone["id"],
                )

                milestone_list.append({
                    "id": str(milestone["id"]),
                    "name": milestone["name"],
                    "description": milestone["description"],
                    "targetDate": milestone["target_date"].isoformat() if milestone["target_date"] else None,
                    "status": milestone["status"],
                    "taskCount": {
                        "total": task_counts["total"],
                        "completed": task_counts["completed"],
                    },
                })

            result.append({
                "id": str(goal["id"]),
                "name": goal["name"],
                "description": goal["description"],
                "color": goal["color"],
                "milestones": milestone_list,
            })

        return result


async def create_goal(
    user_id: UUID,
    name: str,
    description: str | None = None,
    color: str | None = None,
) -> dict[str, Any]:
    """Create a new goal."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO goals (user_id, name, description, color)
            VALUES ($1, $2, $3, $4)
            RETURNING id, name, description, color
            """,
            user_id,
            name,
            description,
            color,
        )
        return {
            "id": str(row["id"]),
            "name": row["name"],
            "description": row["description"],
            "color": row["color"],
        }


async def update_goal(
    goal_id: UUID,
    user_id: UUID,
    name: str | None = None,
    description: str | None = None,
) -> dict[str, Any] | None:
    """Update an existing goal."""
    async with get_connection() as conn:
        updates = []
        params: list[Any] = []
        param_idx = 1

        if name is not None:
            updates.append(f"name = ${param_idx}")
            params.append(name)
            param_idx += 1

        if description is not None:
            updates.append(f"description = ${param_idx}")
            params.append(description)
            param_idx += 1

        if not updates:
            return None

        params.extend([goal_id, user_id])
        set_clause = ", ".join(updates)

        row = await conn.fetchrow(
            f"""
            UPDATE goals SET {set_clause}
            WHERE id = ${param_idx} AND user_id = ${param_idx + 1}
            RETURNING id, name, description, color
            """,
            *params,
        )

        if row is None:
            return None

        return {
            "id": str(row["id"]),
            "name": row["name"],
            "description": row["description"],
            "color": row["color"],
        }


async def delete_goal(goal_id: UUID, user_id: UUID) -> bool:
    """Delete a goal. Returns True if deleted."""
    async with get_connection() as conn:
        result = await conn.execute(
            "DELETE FROM goals WHERE id = $1 AND user_id = $2",
            goal_id,
            user_id,
        )
        return result == "DELETE 1"


async def create_milestone(
    goal_id: UUID,
    user_id: UUID,
    name: str,
    due_date: date | None = None,
) -> dict[str, Any]:
    """Create a new milestone under a goal (validates goal ownership)."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO milestones (goal_id, user_id, name, target_date)
            SELECT $1, $4, $2, $3
            FROM goals WHERE id = $1 AND user_id = $4
            RETURNING id, name, target_date, status
            """,
            goal_id,
            name,
            due_date,
            user_id,
        )
        if row is None:
            return {"error": "Goal not found or access denied"}
        return {
            "id": str(row["id"]),
            "name": row["name"],
            "targetDate": row["target_date"].isoformat() if row["target_date"] else None,
            "status": row["status"],
        }


async def update_milestone(
    milestone_id: UUID,
    user_id: UUID,
    name: str | None = None,
    due_date: date | None = None,
) -> dict[str, Any] | None:
    """Update an existing milestone (validates goal ownership via user_id)."""
    async with get_connection() as conn:
        updates = []
        params: list[Any] = []
        param_idx = 1

        if name is not None:
            updates.append(f"name = ${param_idx}")
            params.append(name)
            param_idx += 1

        if due_date is not None:
            updates.append(f"target_date = ${param_idx}")
            params.append(due_date)
            param_idx += 1

        if not updates:
            return None

        params.extend([milestone_id, user_id])

        row = await conn.fetchrow(
            f"""
            UPDATE milestones SET {", ".join(updates)}
            WHERE id = ${param_idx}
              AND goal_id IN (SELECT id FROM goals WHERE user_id = ${param_idx + 1})
            RETURNING id, name, target_date, status
            """,
            *params,
        )

        if row is None:
            return None

        return {
            "id": str(row["id"]),
            "name": row["name"],
            "targetDate": row["target_date"].isoformat() if row["target_date"] else None,
            "status": row["status"],
        }


async def delete_milestone(milestone_id: UUID, user_id: UUID) -> bool:
    """Delete a milestone (validates goal ownership via user_id). Returns True if deleted."""
    async with get_connection() as conn:
        result = await conn.execute(
            """
            DELETE FROM milestones
            WHERE id = $1
              AND goal_id IN (SELECT id FROM goals WHERE user_id = $2)
            """,
            milestone_id,
            user_id,
        )
        return result == "DELETE 1"
