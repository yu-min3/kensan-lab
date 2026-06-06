"""Database queries for AI interaction history."""

import json
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection


async def get_conversations(
    user_id: UUID,
    limit: int = 20,
    offset: int = 0,
) -> list[dict[str, Any]]:
    """Get list of conversations grouped by conversation_id.

    Returns conversations with their last message and message count,
    ordered by most recent activity.
    """
    limit = min(limit, 100)

    async with get_connection() as conn:
        rows = await conn.fetch(
            """
            SELECT
                conversation_id,
                MAX(created_at) AS last_message_at,
                COUNT(*) AS message_count,
                (
                    SELECT user_input
                    FROM ai_interactions AS inner_ai
                    WHERE inner_ai.conversation_id = ai.conversation_id
                      AND inner_ai.user_id = $1
                    ORDER BY created_at DESC
                    LIMIT 1
                ) AS last_message
            FROM ai_interactions AS ai
            WHERE user_id = $1
              AND conversation_id IS NOT NULL
            GROUP BY conversation_id
            ORDER BY last_message_at DESC
            LIMIT $2 OFFSET $3
            """,
            user_id,
            limit,
            offset,
        )

        return [
            {
                "id": str(row["conversation_id"]),
                "lastMessage": (
                    row["last_message"][:100] + "..."
                    if row["last_message"] and len(row["last_message"]) > 100
                    else row["last_message"] or ""
                ),
                "lastMessageAt": row["last_message_at"].isoformat(),
                "messageCount": row["message_count"],
            }
            for row in rows
        ]


async def get_conversation_messages(
    user_id: UUID,
    conversation_id: UUID,
) -> list[dict[str, Any]]:
    """Get all messages for a specific conversation."""
    async with get_connection() as conn:
        rows = await conn.fetch(
            """
            SELECT id, situation, user_input, ai_output, tool_calls,
                   tokens_input, tokens_output, created_at
            FROM ai_interactions
            WHERE user_id = $1
              AND conversation_id = $2
            ORDER BY created_at ASC
            """,
            user_id,
            conversation_id,
        )

        messages: list[dict[str, Any]] = []
        for row in rows:
            # Each interaction has a user message and an assistant response
            messages.append({
                "id": f"{row['id']}_user",
                "role": "user",
                "content": row["user_input"],
                "situation": row["situation"],
                "toolCalls": [],
                "createdAt": row["created_at"].isoformat(),
            })
            messages.append({
                "id": f"{row['id']}_assistant",
                "role": "assistant",
                "content": row["ai_output"],
                "situation": row["situation"],
                "toolCalls": json.loads(row["tool_calls"]) if row["tool_calls"] else [],
                "createdAt": row["created_at"].isoformat(),
            })

        return messages
