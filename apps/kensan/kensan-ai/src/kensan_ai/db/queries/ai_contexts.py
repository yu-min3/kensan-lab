"""AI context and version queries."""

import json
import logging
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection

logger = logging.getLogger(__name__)


async def ensure_user_contexts(user_id: UUID) -> None:
    """Lazily copy system template contexts to the user if they have none.

    On first access, copies all active default system templates (user_id IS NULL)
    into per-user rows with source_template_id set to the original.
    Also copies initial versions into ai_context_versions.
    """
    async with get_connection() as conn:
        # Check if user already has contexts
        exists = await conn.fetchval(
            "SELECT 1 FROM ai_contexts WHERE user_id = $1 LIMIT 1",
            user_id,
        )
        if exists:
            return

        # Copy all active default system templates
        rows = await conn.fetch(
            """
            INSERT INTO ai_contexts (
                name, situation, version, is_active, is_default,
                system_prompt, allowed_tools, max_turns, temperature,
                description, user_id, source_template_id, active_version
            )
            SELECT name, situation, version, is_active, is_default,
                   system_prompt, allowed_tools, max_turns, temperature,
                   description, $1, id, 1
            FROM ai_contexts
            WHERE user_id IS NULL AND is_default = true AND is_active = true
            ON CONFLICT DO NOTHING
            RETURNING id, system_prompt, allowed_tools, max_turns, temperature
            """,
            user_id,
        )

        # Create initial version records for each copied context
        for row in rows:
            await conn.execute(
                """
                INSERT INTO ai_context_versions (
                    context_id, version_number, system_prompt,
                    allowed_tools, max_turns, temperature, changelog,
                    source, candidate_status
                )
                VALUES ($1, 1, $2, $3, $4, $5, 'テンプレートからコピー', 'manual', NULL)
                """,
                row["id"],
                row["system_prompt"],
                list(row["allowed_tools"]),
                row["max_turns"],
                row["temperature"],
            )

        logger.info("Copied %d system template contexts for user %s", len(rows), user_id)


async def list_contexts(
    situation: str | None = None,
    user_id: UUID | None = None,
) -> list[dict[str, Any]]:
    """List all active AI contexts, optionally filtered by situation.

    If user_id is provided, ensures user contexts exist (lazy copy)
    and returns only that user's contexts.
    If user_id is None, returns system templates.
    """
    if user_id:
        await ensure_user_contexts(user_id)

    async with get_connection() as conn:
        conditions = ["c.is_active = true"]
        params: list[Any] = []
        idx = 1

        if user_id:
            conditions.append(f"c.user_id = ${idx}")
            params.append(user_id)
            idx += 1
        else:
            conditions.append("c.user_id IS NULL")

        if situation:
            conditions.append(f"c.situation = ${idx}")
            params.append(situation)
            idx += 1

        where_clause = " AND ".join(conditions)

        rows = await conn.fetch(
            f"""
            SELECT c.id, c.name, c.situation, c.version, c.is_active, c.is_default,
                   c.system_prompt, c.allowed_tools, c.max_turns, c.temperature,
                   c.description, c.created_at, c.updated_at, c.active_version,
                   (SELECT MAX(version_number) FROM ai_context_versions WHERE context_id = c.id) AS current_version_number,
                   pc.pending_candidate_count
            FROM ai_contexts c
            LEFT JOIN LATERAL (
                SELECT COUNT(*) AS pending_candidate_count
                FROM ai_context_versions
                WHERE context_id = c.id AND candidate_status = 'pending'
            ) pc ON true
            WHERE {where_clause}
            ORDER BY c.situation, c.is_default DESC, c.created_at DESC
            """,
            *params,
        )

        return [_context_row_to_dict(row) for row in rows]


async def get_context(context_id: UUID) -> dict[str, Any] | None:
    """Get a single AI context by ID."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            SELECT c.id, c.name, c.situation, c.version, c.is_active, c.is_default,
                   c.system_prompt, c.allowed_tools, c.max_turns, c.temperature,
                   c.description, c.created_at, c.updated_at, c.active_version,
                   (SELECT MAX(version_number) FROM ai_context_versions WHERE context_id = c.id) AS current_version_number
            FROM ai_contexts c
            WHERE c.id = $1
            """,
            context_id,
        )
        if row is None:
            return None
        return _context_row_to_dict(row)


async def update_context(
    context_id: UUID,
    system_prompt: str | None = None,
    allowed_tools: list[str] | None = None,
    max_turns: int | None = None,
    temperature: float | None = None,
    changelog: str | None = None,
) -> dict[str, Any] | None:
    """Update an AI context and create a new version.

    Flow:
    1. Save current state as a new version in ai_context_versions
    2. Update ai_contexts with new values
    3. Return updated context
    """
    async with get_connection() as conn:
        # Get current state
        current = await conn.fetchrow(
            """
            SELECT id, system_prompt, allowed_tools, max_turns, temperature
            FROM ai_contexts WHERE id = $1
            """,
            context_id,
        )
        if current is None:
            return None

        # Determine next version number
        latest_version = await conn.fetchval(
            "SELECT COALESCE(MAX(version_number), 0) FROM ai_context_versions WHERE context_id = $1",
            context_id,
        )
        next_version = latest_version + 1

        # Save current state as new version (before applying changes)
        await conn.execute(
            """
            INSERT INTO ai_context_versions (
                context_id, version_number, system_prompt, allowed_tools,
                max_turns, temperature, changelog, source, candidate_status
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, 'manual', NULL)
            """,
            context_id,
            next_version,
            system_prompt if system_prompt is not None else current["system_prompt"],
            allowed_tools if allowed_tools is not None else list(current["allowed_tools"]),
            max_turns if max_turns is not None else current["max_turns"],
            temperature if temperature is not None else current["temperature"],
            changelog,
        )

        # Build UPDATE query dynamically
        updates = [f"active_version = {next_version}"]
        params: list[Any] = []
        param_idx = 1

        if system_prompt is not None:
            updates.append(f"system_prompt = ${param_idx}")
            params.append(system_prompt)
            param_idx += 1

        if allowed_tools is not None:
            updates.append(f"allowed_tools = ${param_idx}")
            params.append(allowed_tools)
            param_idx += 1

        if max_turns is not None:
            updates.append(f"max_turns = ${param_idx}")
            params.append(max_turns)
            param_idx += 1

        if temperature is not None:
            updates.append(f"temperature = ${param_idx}")
            params.append(temperature)
            param_idx += 1

        params.append(context_id)
        set_clause = ", ".join(updates)
        await conn.execute(
            f"UPDATE ai_contexts SET {set_clause} WHERE id = ${param_idx}",
            *params,
        )

        # Return updated context
        return await get_context(context_id)


async def list_versions(context_id: UUID) -> list[dict[str, Any]]:
    """List all versions for a context, newest first."""
    async with get_connection() as conn:
        rows = await conn.fetch(
            """
            SELECT id, context_id, version_number, system_prompt, allowed_tools,
                   max_turns, temperature, changelog, created_at,
                   source, eval_summary, candidate_status
            FROM ai_context_versions
            WHERE context_id = $1
            ORDER BY version_number DESC
            """,
            context_id,
        )
        return [_version_row_to_dict(row) for row in rows]


async def get_version(context_id: UUID, version_number: int) -> dict[str, Any] | None:
    """Get a specific version of a context."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            SELECT id, context_id, version_number, system_prompt, allowed_tools,
                   max_turns, temperature, changelog, created_at,
                   source, eval_summary, candidate_status
            FROM ai_context_versions
            WHERE context_id = $1 AND version_number = $2
            """,
            context_id,
            version_number,
        )
        if row is None:
            return None
        return _version_row_to_dict(row)


async def rollback_to_version(context_id: UUID, version_number: int) -> dict[str, Any] | None:
    """Rollback a context to a specific version.

    Flow:
    1. Get the target version data
    2. Save current state as a new version (with rollback changelog)
    3. Update ai_contexts with the target version data
    """
    async with get_connection() as conn:
        # Get target version
        target = await conn.fetchrow(
            """
            SELECT system_prompt, allowed_tools, max_turns, temperature
            FROM ai_context_versions
            WHERE context_id = $1 AND version_number = $2
            """,
            context_id,
            version_number,
        )
        if target is None:
            return None

        # Determine next version number
        latest_version = await conn.fetchval(
            "SELECT COALESCE(MAX(version_number), 0) FROM ai_context_versions WHERE context_id = $1",
            context_id,
        )
        next_version = latest_version + 1

        # Save rollback as a new version
        await conn.execute(
            """
            INSERT INTO ai_context_versions (
                context_id, version_number, system_prompt, allowed_tools,
                max_turns, temperature, changelog, source, candidate_status
            )
            VALUES ($1, $2, $3, $4, $5, $6, $7, 'rollback', NULL)
            """,
            context_id,
            next_version,
            target["system_prompt"],
            list(target["allowed_tools"]),
            target["max_turns"],
            target["temperature"],
            f"ロールバック from v{version_number}",
        )

        # Update ai_contexts
        await conn.execute(
            """
            UPDATE ai_contexts
            SET system_prompt = $1, allowed_tools = $2, max_turns = $3, temperature = $4,
                active_version = $5
            WHERE id = $6
            """,
            target["system_prompt"],
            list(target["allowed_tools"]),
            target["max_turns"],
            target["temperature"],
            next_version,
            context_id,
        )

        return await get_context(context_id)


async def adopt_version(context_id: UUID, version_number: int) -> dict[str, Any] | None:
    """Adopt a candidate version.

    Sets candidate_status to 'adopted', updates ai_contexts system_prompt
    and active_version to the adopted version.
    """
    async with get_connection() as conn:
        # Get the target version
        target = await conn.fetchrow(
            """
            SELECT system_prompt, allowed_tools, max_turns, temperature, candidate_status
            FROM ai_context_versions
            WHERE context_id = $1 AND version_number = $2
            """,
            context_id,
            version_number,
        )
        if target is None:
            return None

        # Update candidate_status to 'adopted'
        await conn.execute(
            """
            UPDATE ai_context_versions
            SET candidate_status = 'adopted'
            WHERE context_id = $1 AND version_number = $2
            """,
            context_id,
            version_number,
        )

        # Update ai_contexts with adopted version data
        await conn.execute(
            """
            UPDATE ai_contexts
            SET system_prompt = $1, allowed_tools = $2, max_turns = $3, temperature = $4,
                active_version = $5
            WHERE id = $6
            """,
            target["system_prompt"],
            list(target["allowed_tools"]),
            target["max_turns"],
            target["temperature"],
            version_number,
            context_id,
        )

        return await get_context(context_id)


async def reject_version(context_id: UUID, version_number: int) -> dict[str, Any] | None:
    """Reject a candidate version.

    Sets candidate_status to 'rejected'. Does not change ai_contexts.
    """
    async with get_connection() as conn:
        result = await conn.execute(
            """
            UPDATE ai_context_versions
            SET candidate_status = 'rejected'
            WHERE context_id = $1 AND version_number = $2
            """,
            context_id,
            version_number,
        )
        if result == "UPDATE 0":
            return None

        return await get_context(context_id)


def _context_row_to_dict(row: Any) -> dict[str, Any]:
    """Convert a context DB row to a dictionary."""
    result = {
        "id": str(row["id"]),
        "name": row["name"],
        "situation": row["situation"],
        "version": row["version"],
        "is_active": row["is_active"],
        "is_default": row["is_default"],
        "system_prompt": row["system_prompt"],
        "allowed_tools": list(row["allowed_tools"]) if row["allowed_tools"] else [],
        "max_turns": row["max_turns"],
        "temperature": row["temperature"],
        "description": row["description"],
        "created_at": row["created_at"].isoformat(),
        "updated_at": row["updated_at"].isoformat(),
        "current_version_number": row["current_version_number"],
        "active_version": row.get("active_version"),
    }

    # Add pending candidate count if present (from list_contexts LEFT JOIN)
    if "pending_candidate_count" in row.keys():
        result["pending_candidate_count"] = row["pending_candidate_count"] or 0
    else:
        result["pending_candidate_count"] = 0

    return result


def _version_row_to_dict(row: Any) -> dict[str, Any]:
    """Convert a version DB row to a dictionary."""
    eval_summary = row.get("eval_summary")
    if isinstance(eval_summary, str):
        eval_summary = json.loads(eval_summary)

    return {
        "id": str(row["id"]),
        "context_id": str(row["context_id"]),
        "version_number": row["version_number"],
        "system_prompt": row["system_prompt"],
        "allowed_tools": list(row["allowed_tools"]) if row["allowed_tools"] else [],
        "max_turns": row["max_turns"],
        "temperature": row["temperature"],
        "changelog": row["changelog"],
        "created_at": row["created_at"].isoformat(),
        "source": row.get("source", "manual"),
        "eval_summary": eval_summary,
        "candidate_status": row.get("candidate_status"),
    }
