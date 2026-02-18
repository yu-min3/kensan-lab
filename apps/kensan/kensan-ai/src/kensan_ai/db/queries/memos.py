"""Memo queries."""

from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_memos(
    user_id: UUID,
    limit: int = 20,
) -> list[dict[str, Any]]:
    """Get memos for a user."""
    async with get_connection() as conn:
        rows = await conn.fetch(
            """
            SELECT id, content, created_at, updated_at
            FROM memos
            WHERE user_id = $1
            ORDER BY created_at DESC
            LIMIT $2
            """,
            user_id,
            limit,
        )

        return [
            {
                "id": str(row["id"]),
                "content": row["content"],
                "createdAt": row["created_at"].isoformat(),
                "updatedAt": row["updated_at"].isoformat() if row["updated_at"] else None,
            }
            for row in rows
        ]


async def create_memo(
    user_id: UUID,
    content: str,
) -> dict[str, Any]:
    """Create a new memo."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO memos (user_id, content)
            VALUES ($1, $2)
            RETURNING id, content, created_at
            """,
            user_id,
            content,
        )

        return {
            "id": str(row["id"]),
            "content": row["content"],
            "createdAt": row["created_at"].isoformat(),
        }
