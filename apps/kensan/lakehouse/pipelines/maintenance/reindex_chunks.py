"""
Chunk Reindex: kensan-ai の /admin/reindex-pending を呼び出す

index_status='pending' のノートに対してチャンク分割＋embedding生成を
kensan-ai 側で一括実行する。Dagster スケジュールジョブから利用。
"""

from __future__ import annotations

from catalog.config import setup_logging
from pipelines.maintenance.common import trigger_kensan_ai_endpoint

logger = setup_logging("maintenance.reindex_chunks")


def reindex_pending_chunks(base_url: str, batch_size: int = 50) -> dict:
    """kensan-ai の reindex-pending エンドポイントを呼び出す.

    Args:
        base_url: kensan-ai の base URL (例: http://localhost:8089)
        batch_size: 各ユーザーの1回あたり処理件数

    Returns:
        {"users_processed": N, "total_processed": N, "total_chunks": N}
    """
    result = trigger_kensan_ai_endpoint(
        "/admin/reindex-pending",
        base_url,
        params={"batch_size": batch_size},
        timeout=300,
    )
    logger.info(
        "Reindex complete: %d users, %d notes, %d chunks",
        result.get("users_processed", 0),
        result.get("total_processed", 0),
        result.get("total_chunks", 0),
    )
    return result
