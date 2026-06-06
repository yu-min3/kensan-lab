"""Task queries."""

from datetime import date
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_tasks(
    user_id: UUID,
    milestone_id: UUID | None = None,
    completed: bool | None = None,
    due_date: date | None = None,
) -> list[dict[str, Any]]:
    """Get tasks for a user with optional filters."""
    async with get_connection() as conn:
        conditions = ["t.user_id = $1"]
        params: list[Any] = [user_id]
        param_idx = 2

        if milestone_id is not None:
            conditions.append(f"t.milestone_id = ${param_idx}")
            params.append(milestone_id)
            param_idx += 1

        if completed is not None:
            conditions.append(f"t.completed = ${param_idx}")
            params.append(completed)
            param_idx += 1

        if due_date is not None:
            conditions.append(f"t.due_date = ${param_idx}")
            params.append(due_date)
            param_idx += 1

        where_clause = " AND ".join(conditions)

        tasks = await conn.fetch(
            f"""
            SELECT
                t.id, t.name, t.estimated_minutes, t.completed, t.due_date,
                t.milestone_id, m.name as milestone_name,
                g.id as goal_id, g.name as goal_name, g.color as goal_color
            FROM tasks t
            LEFT JOIN milestones m ON t.milestone_id = m.id
            LEFT JOIN goals g ON m.goal_id = g.id
            WHERE {where_clause}
            ORDER BY t.completed, t.due_date NULLS LAST, t.created_at DESC
            """,
            *params,
        )

        return [
            {
                "id": str(task["id"]),
                "name": task["name"],
                "estimatedMinutes": task["estimated_minutes"],
                "completed": task["completed"],
                "dueDate": task["due_date"].isoformat() if task["due_date"] else None,
                "milestone": {
                    "id": str(task["milestone_id"]),
                    "name": task["milestone_name"],
                } if task["milestone_id"] else None,
                "goal": {
                    "id": str(task["goal_id"]),
                    "name": task["goal_name"],
                    "color": task["goal_color"],
                } if task["goal_id"] else None,
            }
            for task in tasks
        ]


async def create_task(
    user_id: UUID,
    name: str,
    milestone_id: UUID | None = None,
    estimated_minutes: int | None = None,
    due_date: date | None = None,
) -> dict[str, Any]:
    """Create a new task."""
    async with get_connection() as conn:
        task = await conn.fetchrow(
            """
            INSERT INTO tasks (user_id, name, milestone_id, estimated_minutes, due_date)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING id, name, milestone_id, estimated_minutes, completed, due_date
            """,
            user_id,
            name,
            milestone_id,
            estimated_minutes,
            due_date,
        )

        return {
            "id": str(task["id"]),
            "name": task["name"],
            "milestoneId": str(task["milestone_id"]) if task["milestone_id"] else None,
            "estimatedMinutes": task["estimated_minutes"],
            "completed": task["completed"],
            "dueDate": task["due_date"].isoformat() if task["due_date"] else None,
        }


async def update_task(
    task_id: UUID,
    user_id: UUID,
    name: str | None = None,
    completed: bool | None = None,
    due_date: date | None = None,
) -> dict[str, Any] | None:
    """Update an existing task."""
    async with get_connection() as conn:
        updates = []
        params: list[Any] = []
        param_idx = 1

        if name is not None:
            updates.append(f"name = ${param_idx}")
            params.append(name)
            param_idx += 1

        if completed is not None:
            updates.append(f"completed = ${param_idx}")
            params.append(completed)
            param_idx += 1

        if due_date is not None:
            updates.append(f"due_date = ${param_idx}")
            params.append(due_date)
            param_idx += 1

        if not updates:
            return None

        params.extend([task_id, user_id])
        set_clause = ", ".join(updates)

        task = await conn.fetchrow(
            f"""
            UPDATE tasks
            SET {set_clause}
            WHERE id = ${param_idx} AND user_id = ${param_idx + 1}
            RETURNING id, name, milestone_id, estimated_minutes, completed, due_date
            """,
            *params,
        )

        if task is None:
            return None

        return {
            "id": str(task["id"]),
            "name": task["name"],
            "milestoneId": str(task["milestone_id"]) if task["milestone_id"] else None,
            "estimatedMinutes": task["estimated_minutes"],
            "completed": task["completed"],
            "dueDate": task["due_date"].isoformat() if task["due_date"] else None,
        }


async def delete_task(task_id: UUID, user_id: UUID) -> bool:
    """Delete a task. Returns True if deleted."""
    async with get_connection() as conn:
        result = await conn.execute(
            "DELETE FROM tasks WHERE id = $1 AND user_id = $2",
            task_id,
            user_id,
        )
        return result == "DELETE 1"
