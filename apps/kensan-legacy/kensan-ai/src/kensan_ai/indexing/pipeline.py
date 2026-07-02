"""Indexing pipeline for note content chunks.

Polls for notes with index_status='pending', fetches their content,
chunks it, generates embeddings, and stores the results in note_content_chunks.
"""

import logging
from uuid import UUID

import numpy as np

from kensan_ai.db.connection import get_connection
from kensan_ai.embeddings.service import get_embedding_service
from kensan_ai.indexing.chunker import chunk_content
from kensan_ai.storage.minio_client import get_minio_client

logger = logging.getLogger(__name__)


async def reindex_pending_notes(user_id: UUID, batch_size: int = 10) -> dict:
    """Reindex notes that have index_status='pending'.

    Pipeline per note:
    1. Set index_status = 'processing'
    2. Fetch note_contents rows (content_type, content, storage_key)
    3. For each content: get text (inline from DB, or external from MinIO)
    4. Chunk according to content_type
    5. Generate embeddings in batch
    6. Delete old chunks, insert new ones
    7. Set index_status = 'indexed'

    On error: set index_status = 'failed'.

    Returns:
        Dict with 'processed' and 'chunks_created' counts.
    """
    processed = 0
    chunks_created = 0

    async with get_connection() as conn:
        # 1. Fetch pending notes
        pending_notes = await conn.fetch(
            """
            SELECT id, user_id
            FROM notes
            WHERE user_id = $1 AND index_status = 'pending'
            ORDER BY updated_at DESC
            LIMIT $2
            """,
            user_id,
            batch_size,
        )

    if not pending_notes:
        return {"processed": 0, "chunks_created": 0}

    embedding_service = get_embedding_service()
    minio_client = get_minio_client()

    for note_row in pending_notes:
        note_id = note_row["id"]
        try:
            # 2. Mark as processing
            async with get_connection() as conn:
                await conn.execute(
                    "UPDATE notes SET index_status = 'processing' WHERE id = $1",
                    note_id,
                )

            # 3. Fetch note_contents
            async with get_connection() as conn:
                contents = await conn.fetch(
                    """
                    SELECT id, content_type, content, storage_key
                    FROM note_contents
                    WHERE note_id = $1
                    ORDER BY sort_order
                    """,
                    note_id,
                )

            # 4. Collect all chunks across all contents
            all_chunks = []  # list of (chunk, content_type, note_content_id)
            for content_row in contents:
                content_type = content_row["content_type"]
                note_content_id = content_row["id"]

                # Get text: inline or from MinIO
                text = content_row["content"]
                if not text and content_row["storage_key"]:
                    try:
                        text = minio_client.download_text(content_row["storage_key"])
                    except RuntimeError:
                        logger.warning(
                            "Failed to download content for note %s, content %s",
                            note_id,
                            note_content_id,
                        )
                        continue

                if not text:
                    continue

                chunks = chunk_content(text, content_type)
                for chunk in chunks:
                    all_chunks.append((chunk, content_type, note_content_id))

            # 5. Generate embeddings in batch
            if all_chunks:
                texts = [c[0].text for c in all_chunks]
                embeddings = await embedding_service.generate_embeddings(texts)

                # 6. Delete old chunks and insert new ones
                async with get_connection() as conn:
                    await conn.execute(
                        "DELETE FROM note_content_chunks WHERE note_id = $1",
                        note_id,
                    )

                    for (chunk, content_type, note_content_id), embedding in zip(all_chunks, embeddings):
                        vector = np.array(embedding, dtype=np.float32)
                        await conn.execute(
                            """
                            INSERT INTO note_content_chunks
                                (note_id, note_content_id, chunk_index, chunk_text,
                                 token_count, embedding, embedding_model, processed_at,
                                 user_id, content_type)
                            VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8, $9)
                            """,
                            note_id,
                            note_content_id,
                            chunk.index,
                            chunk.text,
                            chunk.token_count,
                            vector,
                            embedding_service.model,
                            user_id,
                            content_type,
                        )

                    chunks_created += len(all_chunks)
            else:
                # No content to index — still clear old chunks
                async with get_connection() as conn:
                    await conn.execute(
                        "DELETE FROM note_content_chunks WHERE note_id = $1",
                        note_id,
                    )

            # 7. Mark as indexed
            async with get_connection() as conn:
                await conn.execute(
                    "UPDATE notes SET index_status = 'indexed', indexed_at = NOW() WHERE id = $1",
                    note_id,
                )

            processed += 1

        except Exception:
            logger.error("Failed to index note %s", note_id, exc_info=True)
            async with get_connection() as conn:
                await conn.execute(
                    "UPDATE notes SET index_status = 'failed' WHERE id = $1",
                    note_id,
                )

    logger.info(
        "Reindex complete: %d notes processed, %d chunks created (user %s)",
        processed,
        chunks_created,
        user_id,
    )
    return {"processed": processed, "chunks_created": chunks_created}
