"""Interaction logger for recording all AI conversations."""

import json
import logging
from dataclasses import asdict
from typing import Any
from uuid import UUID

from kensan_ai.agents.base import ToolCall
from kensan_ai.db.connection import get_connection

logger = logging.getLogger("kensan_ai.interaction")


class InteractionLogger:
    """Logger for AI interactions."""

    @staticmethod
    async def log(
        user_id: UUID,
        session_id: UUID,
        situation: str,
        user_input: str,
        ai_output: str,
        tool_calls: list[ToolCall] | None = None,
        tokens_input: int | None = None,
        tokens_output: int | None = None,
        latency_ms: int | None = None,
        context_id: UUID | None = None,
        conversation_id: UUID | None = None,
        persona_context_id: UUID | None = None,
    ) -> UUID:
        """Log an AI interaction to the database.

        Args:
            user_id: The user's ID
            session_id: The conversation session ID
            situation: The situation type (chat, review, daily_advice)
            user_input: The user's input message
            ai_output: The AI's response
            tool_calls: List of tool calls made during the interaction
            tokens_input: Number of input tokens used
            tokens_output: Number of output tokens used
            latency_ms: Response latency in milliseconds
            context_id: The context ID used for this interaction
            conversation_id: The conversation ID for grouping messages
            persona_context_id: The persona context ID active during this interaction

        Returns:
            The interaction ID
        """
        tool_calls_json = []
        if tool_calls:
            for tc in tool_calls:
                tool_calls_json.append({
                    "id": tc.id,
                    "name": tc.name,
                    "input": tc.input,
                    "output": tc.output if isinstance(tc.output, (dict, list, str, int, float, bool, type(None))) else str(tc.output),
                })

        async with get_connection() as conn:
            row = await conn.fetchrow(
                """
                INSERT INTO ai_interactions (
                    user_id, session_id, situation, user_input, ai_output,
                    tool_calls, tokens_input, tokens_output, latency_ms, context_id,
                    conversation_id, persona_context_id
                )
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
                RETURNING id
                """,
                user_id,
                session_id,
                situation,
                user_input,
                ai_output,
                json.dumps(tool_calls_json, ensure_ascii=False),
                tokens_input,
                tokens_output,
                latency_ms,
                context_id,
                conversation_id,
                persona_context_id,
            )
            interaction_id = row["id"]

            logger.info(json.dumps({
                "event": "interaction.logged",
                "interaction_id": str(interaction_id),
                "user_id": str(user_id),
                "session_id": str(session_id),
                "situation": situation,
                "tokens_input": tokens_input,
                "tokens_output": tokens_output,
                "latency_ms": latency_ms,
                "context_id": str(context_id) if context_id else None,
                "conversation_id": str(conversation_id) if conversation_id else None,
                "tool_call_count": len(tool_calls_json),
            }, ensure_ascii=False))

            return interaction_id

    @staticmethod
    async def add_feedback(
        interaction_id: UUID,
        rating: int,
        feedback: str | None = None,
    ) -> bool:
        """Add feedback to an existing interaction.

        Args:
            interaction_id: The interaction to add feedback to
            rating: Rating from 1-5
            feedback: Optional text feedback

        Returns:
            True if successful, False if interaction not found
        """
        if not 1 <= rating <= 5:
            raise ValueError("Rating must be between 1 and 5")

        async with get_connection() as conn:
            result = await conn.execute(
                """
                UPDATE ai_interactions
                SET rating = $1, feedback = $2
                WHERE id = $3
                """,
                rating,
                feedback,
                interaction_id,
            )
            return result == "UPDATE 1"

    @staticmethod
    async def get_interaction(interaction_id: UUID) -> dict[str, Any] | None:
        """Get an interaction by ID.

        Args:
            interaction_id: The interaction ID

        Returns:
            Interaction data or None if not found
        """
        async with get_connection() as conn:
            row = await conn.fetchrow(
                """
                SELECT id, user_id, session_id, situation, user_input, ai_output,
                       tool_calls, tokens_input, tokens_output, latency_ms,
                       rating, feedback, context_id, created_at
                FROM ai_interactions
                WHERE id = $1
                """,
                interaction_id,
            )

            if row:
                return {
                    "id": str(row["id"]),
                    "userId": str(row["user_id"]),
                    "sessionId": str(row["session_id"]),
                    "situation": row["situation"],
                    "userInput": row["user_input"],
                    "aiOutput": row["ai_output"],
                    "toolCalls": json.loads(row["tool_calls"]) if row["tool_calls"] else [],
                    "tokensInput": row["tokens_input"],
                    "tokensOutput": row["tokens_output"],
                    "latencyMs": row["latency_ms"],
                    "rating": row["rating"],
                    "feedback": row["feedback"],
                    "contextId": str(row["context_id"]) if row["context_id"] else None,
                    "createdAt": row["created_at"].isoformat(),
                }
            return None

    @staticmethod
    async def get_user_interactions(
        user_id: UUID,
        limit: int = 10,
        situation: str | None = None,
    ) -> list[dict[str, Any]]:
        """Get recent interactions for a user.

        Args:
            user_id: The user's ID
            limit: Maximum number of interactions to return (max 100)
            situation: Filter by situation type

        Returns:
            List of interaction summaries
        """
        # Cap limit to prevent abuse (max 100)
        limit = min(limit, 100)

        async with get_connection() as conn:
            conditions = ["user_id = $1"]
            params: list[Any] = [user_id]
            param_idx = 2

            if situation:
                conditions.append(f"situation = ${param_idx}")
                params.append(situation)
                param_idx += 1

            # Add limit as a parameterized value
            params.append(limit)
            limit_param = f"${param_idx}"

            where_clause = " AND ".join(conditions)

            rows = await conn.fetch(
                f"""
                SELECT id, session_id, situation, user_input, ai_output,
                       tokens_input, tokens_output, rating, created_at
                FROM ai_interactions
                WHERE {where_clause}
                ORDER BY created_at DESC
                LIMIT {limit_param}
                """,
                *params,
            )

            return [
                {
                    "id": str(row["id"]),
                    "sessionId": str(row["session_id"]),
                    "situation": row["situation"],
                    "userInput": row["user_input"][:100] + "..." if len(row["user_input"]) > 100 else row["user_input"],
                    "aiOutput": row["ai_output"][:200] + "..." if len(row["ai_output"]) > 200 else row["ai_output"],
                    "tokensInput": row["tokens_input"],
                    "tokensOutput": row["tokens_output"],
                    "rating": row["rating"],
                    "createdAt": row["created_at"].isoformat(),
                }
                for row in rows
            ]
