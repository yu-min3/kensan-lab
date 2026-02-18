"""Context resolver for loading AI context from database."""

import re
from dataclasses import dataclass, field
from typing import Any
from uuid import UUID

from kensan_ai.context.detector import Situation
from kensan_ai.context.variable_replacer import VariableReplacer
from kensan_ai.db.connection import get_connection
from kensan_ai.db.queries.ai_contexts import ensure_user_contexts
from kensan_ai.db.queries import ai_contexts as ai_contexts_queries

_VARIABLE_PATTERN = re.compile(r"\{(\w+)\}")


@dataclass
class AIContext:
    """AI context configuration loaded from database."""

    id: UUID
    name: str
    situation: Situation
    version: str
    system_prompt: str
    allowed_tools: list[str]
    max_turns: int
    temperature: float
    persona_context_id: UUID | None = None
    prompt_variables: list[str] = field(default_factory=list)


class ContextResolver:
    """Resolves AI context from database based on situation."""

    @staticmethod
    async def get_context(
        situation: Situation,
        user_id: UUID | None = None,
    ) -> AIContext | None:
        """Get the appropriate context for a situation.

        Resolves context from DB, applies variable replacement,
        and prepends shared persona prompt.

        Args:
            situation: The detected or specified situation
            user_id: Optional user ID for variable replacement

        Returns:
            AIContext if found, None otherwise
        """
        # Ensure per-user contexts exist (lazy copy from system templates)
        if user_id:
            await ensure_user_contexts(user_id)

        async with get_connection() as conn:
            context = await ContextResolver._resolve_context(
                conn, situation, user_id
            )
            if not context:
                return None

            # Variable replacement on task-specific prompt
            if user_id:
                context.prompt_variables = _VARIABLE_PATTERN.findall(context.system_prompt)
                context.system_prompt = await VariableReplacer.replace(
                    context.system_prompt, user_id
                )

            # Prepend shared persona (skip for persona row itself)
            if context.situation != Situation.PERSONA:
                persona = await ContextResolver._get_persona(conn, user_id)
                if persona:
                    context.persona_context_id = persona.id
                    context.prompt_variables = list(set(
                        persona.prompt_variables + context.prompt_variables
                    ))
                    context.system_prompt = persona.system_prompt + "\n\n" + context.system_prompt

            return context

    @staticmethod
    async def _resolve_context(
        conn: Any,
        situation: Situation,
        user_id: UUID | None,
    ) -> AIContext | None:
        """Resolve context from DB without variable replacement."""
        # Get default context for situation (prefer user-specific, fallback to system)
        if user_id:
            row = await conn.fetchrow(
                """
                SELECT id, name, situation, version, system_prompt,
                       allowed_tools, max_turns, temperature
                FROM ai_contexts
                WHERE situation = $1 AND is_default = true AND is_active = true
                  AND user_id = $2
                LIMIT 1
                """,
                situation.value,
                user_id,
            )
            if row:
                return ContextResolver._row_to_context(row)

        # Fallback to system template (user_id IS NULL)
        row = await conn.fetchrow(
            """
            SELECT id, name, situation, version, system_prompt,
                   allowed_tools, max_turns, temperature
            FROM ai_contexts
            WHERE situation = $1 AND is_default = true AND is_active = true
              AND user_id IS NULL
            LIMIT 1
            """,
            situation.value,
        )

        if row:
            return ContextResolver._row_to_context(row)

        # Last resort: any active context for this situation
        row = await conn.fetchrow(
            """
            SELECT id, name, situation, version, system_prompt,
                   allowed_tools, max_turns, temperature
            FROM ai_contexts
            WHERE situation = $1 AND is_active = true
            ORDER BY created_at DESC
            LIMIT 1
            """,
            situation.value,
        )

        if row:
            return ContextResolver._row_to_context(row)

        return None

    @staticmethod
    async def _get_persona(conn: Any, user_id: UUID | None) -> AIContext | None:
        """Fetch the persona context and apply variable replacement.

        Prefers user-specific persona, falls back to system template.
        """
        row = None
        if user_id:
            row = await conn.fetchrow(
                """SELECT id, name, situation, version, system_prompt,
                          allowed_tools, max_turns, temperature
                   FROM ai_contexts
                   WHERE situation = 'persona' AND is_active = true AND user_id = $1
                   LIMIT 1""",
                user_id,
            )
        if not row:
            row = await conn.fetchrow(
                """SELECT id, name, situation, version, system_prompt,
                          allowed_tools, max_turns, temperature
                   FROM ai_contexts
                   WHERE situation = 'persona' AND is_active = true AND user_id IS NULL
                   LIMIT 1""",
            )
        if not row:
            return None
        ctx = ContextResolver._row_to_context(row)
        if user_id:
            ctx.prompt_variables = _VARIABLE_PATTERN.findall(ctx.system_prompt)
            ctx.system_prompt = await VariableReplacer.replace(ctx.system_prompt, user_id)
        return ctx

    @staticmethod
    def _row_to_context(row: Any) -> AIContext:
        """Convert a database row to AIContext."""
        return AIContext(
            id=row["id"],
            name=row["name"],
            situation=Situation(row["situation"]),
            version=row["version"],
            system_prompt=row["system_prompt"],
            allowed_tools=list(row["allowed_tools"]) if row["allowed_tools"] else [],
            max_turns=row["max_turns"],
            temperature=row["temperature"],
        )

    @staticmethod
    async def get_context_by_id(
        context_id: UUID,
        user_id: UUID | None = None,
        version_number: int | None = None,
    ) -> AIContext | None:
        """Get a context directly by its ID.

        Loads context by ID, applies variable replacement and persona prepend.
        Used by A/B comparison to load specific contexts.

        Args:
            context_id: The context ID to load
            user_id: Optional user ID for variable replacement
            version_number: Optional version number to load a specific version's prompt

        Returns:
            AIContext if found, None otherwise
        """
        async with get_connection() as conn:
            row = await conn.fetchrow(
                """
                SELECT id, name, situation, version, system_prompt,
                       allowed_tools, max_turns, temperature
                FROM ai_contexts
                WHERE id = $1
                """,
                context_id,
            )
            if not row:
                return None

            context = ContextResolver._row_to_context(row)

            # Override with specific version's settings if requested
            if version_number is not None:
                version = await ai_contexts_queries.get_version(context_id, version_number)
                if version:
                    context.system_prompt = version["system_prompt"]
                    if version.get("allowed_tools"):
                        context.allowed_tools = version["allowed_tools"]
                    if version.get("max_turns") is not None:
                        context.max_turns = version["max_turns"]
                    if version.get("temperature") is not None:
                        context.temperature = version["temperature"]

            # Variable replacement on task-specific prompt
            if user_id:
                context.prompt_variables = _VARIABLE_PATTERN.findall(context.system_prompt)
                context.system_prompt = await VariableReplacer.replace(
                    context.system_prompt, user_id
                )

            # Prepend shared persona (skip for persona row itself)
            if context.situation != Situation.PERSONA:
                persona = await ContextResolver._get_persona(conn, user_id)
                if persona:
                    context.persona_context_id = persona.id
                    context.prompt_variables = list(set(
                        persona.prompt_variables + context.prompt_variables
                    ))
                    context.system_prompt = persona.system_prompt + "\n\n" + context.system_prompt

            return context

    @staticmethod
    async def list_contexts(
        situation: Situation | None = None,
        active_only: bool = True,
    ) -> list[AIContext]:
        """List all contexts, optionally filtered by situation."""
        async with get_connection() as conn:
            conditions = []
            params: list[Any] = []
            param_idx = 1

            if situation:
                conditions.append(f"situation = ${param_idx}")
                params.append(situation.value)
                param_idx += 1

            if active_only:
                conditions.append("is_active = true")

            where_clause = " AND ".join(conditions) if conditions else "1=1"

            rows = await conn.fetch(
                f"""
                SELECT id, name, situation, version, system_prompt,
                       allowed_tools, max_turns, temperature
                FROM ai_contexts
                WHERE {where_clause}
                ORDER BY situation, is_default DESC, created_at DESC
                """,
                *params,
            )

            return [ContextResolver._row_to_context(row) for row in rows]
