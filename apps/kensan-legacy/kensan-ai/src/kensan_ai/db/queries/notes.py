"""Note queries."""

import asyncio
import logging
from typing import Any
from uuid import UUID

from kensan_ai.db.connection import get_connection

logger = logging.getLogger(__name__)


async def _store_embedding(note_id: UUID, text: str) -> None:
    """Generate and store an embedding for a note (fire-and-forget safe)."""
    try:
        from kensan_ai.embeddings.service import get_embedding_service

        import numpy as np

        embedding = await get_embedding_service().generate_embedding(text)
        vector = np.array(embedding, dtype=np.float32)
        async with get_connection() as conn:
            await conn.execute(
                "UPDATE notes SET embedding = $1 WHERE id = $2",
                vector,
                note_id,
            )
        logger.info("Stored embedding for note %s", note_id)
    except Exception:
        logger.warning("Failed to generate embedding for note %s", note_id, exc_info=True)


async def backfill_embeddings(user_id: UUID, batch_size: int = 20) -> int:
    """Generate embeddings for notes that don't have one yet.

    Returns the number of notes processed.
    """
    from kensan_ai.embeddings.service import get_embedding_service

    async with get_connection() as conn:
        rows = await conn.fetch(
            """
            SELECT id, title, content
            FROM notes
            WHERE user_id = $1 AND embedding IS NULL
              AND (title IS NOT NULL OR content IS NOT NULL)
            ORDER BY created_at DESC
            LIMIT $2
            """,
            user_id,
            batch_size,
        )

    if not rows:
        return 0

    service = get_embedding_service()
    texts = [
        f"{r['title']}\n{r['content']}" if r["title"] and r["content"]
        else (r["title"] or r["content"] or "")
        for r in rows
    ]

    import numpy as np

    embeddings = await service.generate_embeddings(texts)

    async with get_connection() as conn:
        for row, emb in zip(rows, embeddings):
            vector = np.array(emb, dtype=np.float32)
            await conn.execute(
                "UPDATE notes SET embedding = $1, index_status = 'pending' WHERE id = $2",
                vector,
                row["id"],
            )

    logger.info("Backfilled embeddings for %d notes (user %s)", len(rows), user_id)
    return len(rows)


async def get_notes(
    user_id: UUID,
    note_type: str | None = None,
    start_date: "date | None" = None,
    end_date: "date | None" = None,
    limit: int = 20,
) -> list[dict[str, Any]]:
    """Get notes for a user with optional type and date filters.

    Args:
        user_id: The user ID.
        note_type: Filter by note type (e.g., 'diary', 'learning').
        start_date: Filter notes with date >= start_date (uses note's date column).
        end_date: Filter notes with date <= end_date (uses note's date column).
        limit: Max number of notes to return.
    """
    async with get_connection() as conn:
        conditions = ["user_id = $1"]
        params: list[Any] = [user_id]
        param_idx = 2

        if note_type is not None:
            conditions.append(f"type = ${param_idx}")
            params.append(note_type)
            param_idx += 1

        if start_date is not None:
            conditions.append(f"COALESCE(date, created_at::date) >= ${param_idx}")
            params.append(start_date)
            param_idx += 1

        if end_date is not None:
            conditions.append(f"COALESCE(date, created_at::date) <= ${param_idx}")
            params.append(end_date)
            param_idx += 1

        params.append(limit)
        where_clause = " AND ".join(conditions)

        # Sort by note date when available, fall back to created_at
        rows = await conn.fetch(
            f"""
            SELECT id, title, type, content, date, created_at, updated_at
            FROM notes
            WHERE {where_clause}
            ORDER BY COALESCE(date, created_at::date) DESC, created_at DESC
            LIMIT ${param_idx}
            """,
            *params,
        )

        return [
            {
                "id": str(row["id"]),
                "title": row["title"],
                "type": row["type"],
                "content": row["content"],
                "date": row["date"].isoformat() if row["date"] else None,
                "createdAt": row["created_at"].isoformat(),
                "updatedAt": row["updated_at"].isoformat() if row["updated_at"] else None,
            }
            for row in rows
        ]


async def create_note(
    user_id: UUID,
    title: str,
    content: str,
    note_type: str,
) -> dict[str, Any]:
    """Create a new note."""
    async with get_connection() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO notes (user_id, title, content, type)
            VALUES ($1, $2, $3, $4)
            RETURNING id, title, type, content, created_at
            """,
            user_id,
            title,
            content,
            note_type,
        )

        result = {
            "id": str(row["id"]),
            "title": row["title"],
            "type": row["type"],
            "content": row["content"],
            "createdAt": row["created_at"].isoformat(),
        }

        # Fire-and-forget embedding generation
        text = f"{title}\n{content}" if title and content else (title or content or "")
        if text.strip():
            asyncio.create_task(_store_embedding(row["id"], text))

        return result


async def update_note(
    note_id: UUID,
    user_id: UUID,
    title: str | None = None,
    content: str | None = None,
) -> dict[str, Any] | None:
    """Update an existing note."""
    async with get_connection() as conn:
        updates = []
        params: list[Any] = []
        param_idx = 1

        if title is not None:
            updates.append(f"title = ${param_idx}")
            params.append(title)
            param_idx += 1

        if content is not None:
            updates.append(f"content = ${param_idx}")
            params.append(content)
            param_idx += 1

        if not updates:
            return None

        params.extend([note_id, user_id])
        set_clause = ", ".join(updates)

        row = await conn.fetchrow(
            f"""
            UPDATE notes
            SET {set_clause}
            WHERE id = ${param_idx} AND user_id = ${param_idx + 1}
            RETURNING id, title, type, content, created_at, updated_at
            """,
            *params,
        )

        if row is None:
            return None

        result = {
            "id": str(row["id"]),
            "title": row["title"],
            "type": row["type"],
            "content": row["content"],
            "createdAt": row["created_at"].isoformat(),
            "updatedAt": row["updated_at"].isoformat() if row["updated_at"] else None,
        }

        # Re-generate embedding if title or content changed
        if title is not None or content is not None:
            text = f"{row['title']}\n{row['content']}" if row["title"] and row["content"] else (row["title"] or row["content"] or "")
            if text.strip():
                asyncio.create_task(_store_embedding(row["id"], text))

        return result
