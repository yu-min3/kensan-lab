"""Search tools for semantic and keyword-based search on note content chunks."""

import logging
from typing import Any
from uuid import UUID

from kensan_ai.tools.base import tool
from kensan_ai.db.connection import get_connection
from kensan_ai.embeddings.service import get_embedding_service

logger = logging.getLogger(__name__)


def _parse_uuid(value: str | None) -> UUID | None:
    """Parse a string to UUID, returning None if invalid or empty."""
    if not value:
        return None
    try:
        return UUID(value)
    except ValueError:
        return None


@tool(
    category="search",
    name="semantic_search",
    description="ベクトル類似度を使用してノートのチャンクを検索します。意味的に類似したコンテンツを見つけるのに適しています。",
    input_schema={
        "properties": {
            "query": {
                "type": "string",
                "description": "検索クエリ",
            },
            "limit": {
                "type": "integer",
                "description": "結果の最大件数 (デフォルト: 5)",
            },
            "content_type": {
                "type": "string",
                "description": "コンテンツタイプでフィルタ (例: 'markdown', 'code', 'drawio')",
            },
        },
        "required": ["query"],
    },
)
async def semantic_search(args: dict[str, Any]) -> dict[str, Any]:
    """Perform semantic search on note_content_chunks using vector similarity."""
    user_id = _parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    query = args.get("query", "").strip()
    if not query:
        return {"error": "Query cannot be empty"}

    limit = args.get("limit", 5)
    content_type = args.get("content_type")

    try:
        embedding_service = get_embedding_service()
        query_embedding = await embedding_service.generate_embedding(
            query, task_type="RETRIEVAL_QUERY"
        )

        async with get_connection() as conn:
            if content_type:
                rows = await conn.fetch(
                    """
                    SELECT ncc.id, ncc.chunk_text, ncc.chunk_index, ncc.content_type,
                           ncc.note_id, n.title as note_title, n.type as note_type,
                           1 - (ncc.embedding <=> $1::vector) as similarity
                    FROM note_content_chunks ncc
                    JOIN notes n ON ncc.note_id = n.id
                    WHERE ncc.user_id = $2 AND ncc.content_type = $3 AND ncc.embedding IS NOT NULL
                    ORDER BY ncc.embedding <=> $1::vector
                    LIMIT $4
                    """,
                    query_embedding,
                    user_id,
                    content_type,
                    limit,
                )
            else:
                rows = await conn.fetch(
                    """
                    SELECT ncc.id, ncc.chunk_text, ncc.chunk_index, ncc.content_type,
                           ncc.note_id, n.title as note_title, n.type as note_type,
                           1 - (ncc.embedding <=> $1::vector) as similarity
                    FROM note_content_chunks ncc
                    JOIN notes n ON ncc.note_id = n.id
                    WHERE ncc.user_id = $2 AND ncc.embedding IS NOT NULL
                    ORDER BY ncc.embedding <=> $1::vector
                    LIMIT $3
                    """,
                    query_embedding,
                    user_id,
                    limit,
                )

            results = [
                {
                    "id": str(row["id"]),
                    "noteId": str(row["note_id"]),
                    "noteTitle": row["note_title"],
                    "noteType": row["note_type"],
                    "chunkIndex": row["chunk_index"],
                    "contentType": row["content_type"],
                    "content": row["chunk_text"][:500] + "..." if row["chunk_text"] and len(row["chunk_text"]) > 500 else row["chunk_text"],
                    "similarity": round(row["similarity"], 4),
                }
                for row in rows
            ]

            return {"results": results, "count": len(results)}

    except Exception as e:
        logger.error(f"Semantic search failed: {e}")
        return {"error": f"Search failed: {str(e)}"}


@tool(
    category="search",
    name="keyword_search",
    description="キーワードベースの全文検索をノートチャンクに対して行います。特定の単語やフレーズを含むコンテンツを見つけるのに適しています。",
    input_schema={
        "properties": {
            "query": {
                "type": "string",
                "description": "検索キーワード（スペース区切りでAND検索）",
            },
            "limit": {
                "type": "integer",
                "description": "結果の最大件数 (デフォルト: 10)",
            },
            "content_type": {
                "type": "string",
                "description": "コンテンツタイプでフィルタ (例: 'markdown', 'code', 'drawio')",
            },
        },
        "required": ["query"],
    },
)
async def keyword_search(args: dict[str, Any]) -> dict[str, Any]:
    """Perform keyword-based full-text search on note_content_chunks."""
    user_id = _parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    query = args.get("query", "").strip()
    if not query:
        return {"error": "Query cannot be empty"}

    limit = args.get("limit", 10)
    content_type = args.get("content_type")

    try:
        keywords = query.split()
        tsquery = " & ".join(keywords)

        async with get_connection() as conn:
            if content_type:
                rows = await conn.fetch(
                    """
                    SELECT ncc.id, ncc.chunk_text, ncc.chunk_index, ncc.content_type,
                           ncc.note_id, n.title as note_title, n.type as note_type,
                           ts_rank(to_tsvector('simple', ncc.chunk_text), to_tsquery('simple', $1)) as rank
                    FROM note_content_chunks ncc
                    JOIN notes n ON ncc.note_id = n.id
                    WHERE ncc.user_id = $2
                      AND ncc.content_type = $3
                      AND to_tsvector('simple', ncc.chunk_text) @@ to_tsquery('simple', $1)
                    ORDER BY rank DESC
                    LIMIT $4
                    """,
                    tsquery,
                    user_id,
                    content_type,
                    limit,
                )
            else:
                rows = await conn.fetch(
                    """
                    SELECT ncc.id, ncc.chunk_text, ncc.chunk_index, ncc.content_type,
                           ncc.note_id, n.title as note_title, n.type as note_type,
                           ts_rank(to_tsvector('simple', ncc.chunk_text), to_tsquery('simple', $1)) as rank
                    FROM note_content_chunks ncc
                    JOIN notes n ON ncc.note_id = n.id
                    WHERE ncc.user_id = $2
                      AND to_tsvector('simple', ncc.chunk_text) @@ to_tsquery('simple', $1)
                    ORDER BY rank DESC
                    LIMIT $3
                    """,
                    tsquery,
                    user_id,
                    limit,
                )

            results = [
                {
                    "id": str(row["id"]),
                    "noteId": str(row["note_id"]),
                    "noteTitle": row["note_title"],
                    "noteType": row["note_type"],
                    "chunkIndex": row["chunk_index"],
                    "contentType": row["content_type"],
                    "content": row["chunk_text"][:500] + "..." if row["chunk_text"] and len(row["chunk_text"]) > 500 else row["chunk_text"],
                    "rank": round(row["rank"], 4),
                }
                for row in rows
            ]

            return {"results": results, "count": len(results)}

    except Exception as e:
        logger.error(f"Keyword search failed: {e}")
        return {"error": f"Search failed: {str(e)}"}


@tool(
    category="search",
    name="hybrid_search",
    description="セマンティック検索とキーワード検索を組み合わせたハイブリッド検索を行います。より精度の高い検索結果を得るのに適しています。",
    input_schema={
        "properties": {
            "query": {
                "type": "string",
                "description": "検索クエリ",
            },
            "limit": {
                "type": "integer",
                "description": "結果の最大件数 (デフォルト: 5)",
            },
            "content_type": {
                "type": "string",
                "description": "コンテンツタイプでフィルタ (例: 'markdown', 'code', 'drawio')",
            },
            "semantic_weight": {
                "type": "number",
                "description": "セマンティックスコアの重み (0.0-1.0、デフォルト: 0.7)",
            },
        },
        "required": ["query"],
    },
)
async def hybrid_search(args: dict[str, Any]) -> dict[str, Any]:
    """Perform hybrid search combining semantic and keyword search on note_content_chunks."""
    user_id = _parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    query = args.get("query", "").strip()
    if not query:
        return {"error": "Query cannot be empty"}

    limit = args.get("limit", 5)
    content_type = args.get("content_type")
    semantic_weight = args.get("semantic_weight", 0.7)

    semantic_weight = max(0.0, min(1.0, semantic_weight))
    keyword_weight = 1.0 - semantic_weight

    try:
        embedding_service = get_embedding_service()
        query_embedding = await embedding_service.generate_embedding(
            query, task_type="RETRIEVAL_QUERY"
        )

        keywords = query.split()
        tsquery = " | ".join(keywords)  # OR for broader matching

        async with get_connection() as conn:
            if content_type:
                rows = await conn.fetch(
                    """
                    WITH semantic AS (
                        SELECT id, 1 - (embedding <=> $1::vector) as semantic_score
                        FROM note_content_chunks
                        WHERE user_id = $2 AND content_type = $3 AND embedding IS NOT NULL
                    ),
                    keyword AS (
                        SELECT id,
                               ts_rank(to_tsvector('simple', chunk_text), to_tsquery('simple', $4)) as keyword_score
                        FROM note_content_chunks
                        WHERE user_id = $2 AND content_type = $3
                    )
                    SELECT ncc.id, ncc.chunk_text, ncc.chunk_index, ncc.content_type,
                           ncc.note_id, n.title as note_title, n.type as note_type,
                           COALESCE(s.semantic_score, 0) as semantic_score,
                           COALESCE(k.keyword_score, 0) as keyword_score,
                           (COALESCE(s.semantic_score, 0) * $5 + COALESCE(k.keyword_score, 0) * $6) as combined_score
                    FROM note_content_chunks ncc
                    JOIN notes n ON ncc.note_id = n.id
                    LEFT JOIN semantic s ON ncc.id = s.id
                    LEFT JOIN keyword k ON ncc.id = k.id
                    WHERE ncc.user_id = $2 AND ncc.content_type = $3
                    ORDER BY combined_score DESC
                    LIMIT $7
                    """,
                    query_embedding,
                    user_id,
                    content_type,
                    tsquery,
                    semantic_weight,
                    keyword_weight,
                    limit,
                )
            else:
                rows = await conn.fetch(
                    """
                    WITH semantic AS (
                        SELECT id, 1 - (embedding <=> $1::vector) as semantic_score
                        FROM note_content_chunks
                        WHERE user_id = $2 AND embedding IS NOT NULL
                    ),
                    keyword AS (
                        SELECT id,
                               ts_rank(to_tsvector('simple', chunk_text), to_tsquery('simple', $3)) as keyword_score
                        FROM note_content_chunks
                        WHERE user_id = $2
                    )
                    SELECT ncc.id, ncc.chunk_text, ncc.chunk_index, ncc.content_type,
                           ncc.note_id, n.title as note_title, n.type as note_type,
                           COALESCE(s.semantic_score, 0) as semantic_score,
                           COALESCE(k.keyword_score, 0) as keyword_score,
                           (COALESCE(s.semantic_score, 0) * $4 + COALESCE(k.keyword_score, 0) * $5) as combined_score
                    FROM note_content_chunks ncc
                    JOIN notes n ON ncc.note_id = n.id
                    LEFT JOIN semantic s ON ncc.id = s.id
                    LEFT JOIN keyword k ON ncc.id = k.id
                    WHERE ncc.user_id = $2
                    ORDER BY combined_score DESC
                    LIMIT $6
                    """,
                    query_embedding,
                    user_id,
                    tsquery,
                    semantic_weight,
                    keyword_weight,
                    limit,
                )

            results = [
                {
                    "id": str(row["id"]),
                    "noteId": str(row["note_id"]),
                    "noteTitle": row["note_title"],
                    "noteType": row["note_type"],
                    "chunkIndex": row["chunk_index"],
                    "contentType": row["content_type"],
                    "content": row["chunk_text"][:500] + "..." if row["chunk_text"] and len(row["chunk_text"]) > 500 else row["chunk_text"],
                    "semanticScore": round(row["semantic_score"], 4),
                    "keywordScore": round(row["keyword_score"], 4),
                    "combinedScore": round(row["combined_score"], 4),
                }
                for row in rows
            ]

            return {"results": results, "count": len(results)}

    except Exception as e:
        logger.error(f"Hybrid search failed: {e}")
        return {"error": f"Search failed: {str(e)}"}


@tool(
    category="search",
    name="search_notes",
    description="ノート（学習記録・日記）をキーワードで検索します。タイトルと本文を全文検索します。",
    input_schema={
        "properties": {
            "query": {
                "type": "string",
                "description": "検索キーワード（スペース区切りでAND検索）",
            },
            "note_type": {
                "type": "string",
                "description": "ノート種別でフィルタ (例: 'diary', 'learning', 'general')",
            },
            "limit": {
                "type": "integer",
                "description": "結果の最大件数 (デフォルト: 10)",
            },
        },
        "required": ["query"],
    },
)
async def search_notes(args: dict[str, Any]) -> dict[str, Any]:
    """Search notes using full-text search on title + content."""
    user_id = _parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    query = args.get("query", "").strip()
    if not query:
        return {"error": "Query cannot be empty"}

    limit = args.get("limit", 10)
    note_type = args.get("note_type")

    try:
        keywords = query.split()
        tsquery = " & ".join(keywords)

        async with get_connection() as conn:
            if note_type:
                rows = await conn.fetch(
                    """
                    SELECT id, title, type, content, created_at,
                           ts_rank(
                               to_tsvector('simple', COALESCE(title, '') || ' ' || COALESCE(content, '')),
                               to_tsquery('simple', $1)
                           ) as rank
                    FROM notes
                    WHERE user_id = $2
                      AND type = $3
                      AND to_tsvector('simple', COALESCE(title, '') || ' ' || COALESCE(content, ''))
                          @@ to_tsquery('simple', $1)
                    ORDER BY rank DESC
                    LIMIT $4
                    """,
                    tsquery,
                    user_id,
                    note_type,
                    limit,
                )
            else:
                rows = await conn.fetch(
                    """
                    SELECT id, title, type, content, created_at,
                           ts_rank(
                               to_tsvector('simple', COALESCE(title, '') || ' ' || COALESCE(content, '')),
                               to_tsquery('simple', $1)
                           ) as rank
                    FROM notes
                    WHERE user_id = $2
                      AND to_tsvector('simple', COALESCE(title, '') || ' ' || COALESCE(content, ''))
                          @@ to_tsquery('simple', $1)
                    ORDER BY rank DESC
                    LIMIT $3
                    """,
                    tsquery,
                    user_id,
                    limit,
                )

            results = [
                {
                    "id": str(row["id"]),
                    "title": row["title"],
                    "type": row["type"],
                    "content": row["content"][:500] + "..." if row["content"] and len(row["content"]) > 500 else row["content"],
                    "createdAt": row["created_at"].isoformat() if row["created_at"] else None,
                    "rank": round(row["rank"], 4),
                }
                for row in rows
            ]

            return {"results": results, "count": len(results)}

    except Exception as e:
        logger.error(f"Note search failed: {e}")
        return {"error": f"Search failed: {str(e)}"}


@tool(
    category="search",
    name="semantic_search_notes",
    description="ノート（学習記録・日記）をベクトル類似度で検索します。意味的に類似した内容を見つけるのに適しています。embedding列がセットされたノートのみ対象。",
    input_schema={
        "properties": {
            "query": {
                "type": "string",
                "description": "検索クエリ",
            },
            "note_type": {
                "type": "string",
                "description": "ノート種別でフィルタ (例: 'diary', 'learning')",
            },
            "limit": {
                "type": "integer",
                "description": "結果の最大件数 (デフォルト: 5)",
            },
        },
        "required": ["query"],
    },
)
async def semantic_search_notes(args: dict[str, Any]) -> dict[str, Any]:
    """Perform semantic search on notes using vector similarity."""
    user_id = _parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    query = args.get("query", "").strip()
    if not query:
        return {"error": "Query cannot be empty"}

    limit = args.get("limit", 5)
    note_type = args.get("note_type")

    try:
        embedding_service = get_embedding_service()
        query_embedding = await embedding_service.generate_embedding(
            query, task_type="RETRIEVAL_QUERY"
        )

        async with get_connection() as conn:
            if note_type:
                rows = await conn.fetch(
                    """
                    SELECT id, title, type, content, created_at,
                           1 - (embedding <=> $1::vector) as similarity
                    FROM notes
                    WHERE user_id = $2 AND type = $3 AND embedding IS NOT NULL
                    ORDER BY embedding <=> $1::vector
                    LIMIT $4
                    """,
                    query_embedding,
                    user_id,
                    note_type,
                    limit,
                )
            else:
                rows = await conn.fetch(
                    """
                    SELECT id, title, type, content, created_at,
                           1 - (embedding <=> $1::vector) as similarity
                    FROM notes
                    WHERE user_id = $2 AND embedding IS NOT NULL
                    ORDER BY embedding <=> $1::vector
                    LIMIT $3
                    """,
                    query_embedding,
                    user_id,
                    limit,
                )

            results = [
                {
                    "id": str(row["id"]),
                    "title": row["title"],
                    "type": row["type"],
                    "content": row["content"][:500] + "..." if row["content"] and len(row["content"]) > 500 else row["content"],
                    "createdAt": row["created_at"].isoformat() if row["created_at"] else None,
                    "similarity": round(row["similarity"], 4),
                }
                for row in rows
            ]

            return {"results": results, "count": len(results)}

    except Exception as e:
        logger.error(f"Semantic note search failed: {e}")
        return {"error": f"Search failed: {str(e)}"}


@tool(
    category="search",
    name="reindex_notes",
    description="インデックス未生成のノートに対してチャンク分割とembedding生成を一括実行します。検索精度を高めるために使用します。",
    input_schema={
        "properties": {
            "batch_size": {
                "type": "integer",
                "description": "一度に処理する最大件数 (デフォルト: 10)",
            },
        },
        "required": [],
    },
)
async def reindex_notes(args: dict[str, Any]) -> dict[str, Any]:
    """Reindex pending notes: chunk content and generate embeddings."""
    from kensan_ai.indexing.pipeline import reindex_pending_notes

    user_id = _parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    batch_size = args.get("batch_size", 10)

    try:
        result = await reindex_pending_notes(user_id, batch_size=batch_size)
        return {
            **result,
            "message": f"{result['processed']}件のノートをインデックス化し、{result['chunks_created']}個のチャンクを生成しました",
        }
    except Exception as e:
        logger.error(f"Reindex notes failed: {e}")
        return {"error": f"Reindex failed: {str(e)}"}


# All search tools for export
ALL_SEARCH_TOOLS = [
    semantic_search,
    keyword_search,
    hybrid_search,
    search_notes,
    semantic_search_notes,
    reindex_notes,
]
