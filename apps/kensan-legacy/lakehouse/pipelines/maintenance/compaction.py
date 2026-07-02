"""
Iceberg Maintenance: Compaction & Snapshot Expiry

小ファイル統合と古いスナップショットの削除を行うメンテナンスジョブ。
週次 Dagster ジョブとして実行。
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import TYPE_CHECKING

from catalog.config import setup_logging

if TYPE_CHECKING:
    from pyiceberg.catalog import Catalog

logger = setup_logging("maintenance.compaction")

# Compaction 対象テーブル (append-only で小ファイルが溜まりやすいもの)
COMPACTION_TABLES = [
    "bronze.ai_interactions_raw",
    "bronze.ai_facts_raw",
    "bronze.ai_reviews_raw",
    "bronze.ai_explorer_events_raw",
]

# 全テーブル (snapshot expiry 対象)
ALL_TABLES = [
    "bronze.time_entries_raw",
    "bronze.tasks_raw",
    "bronze.notes_raw",
    "bronze.tags_raw",
    "bronze.note_tags_raw",
    "bronze.ai_interactions_raw",
    "bronze.ai_facts_raw",
    "bronze.ai_reviews_raw",
    "bronze.ai_contexts_raw",
    "bronze.ai_explorer_events_raw",
    "silver.time_entries",
    "silver.tasks",
    "silver.notes",
    "silver.ai_interactions",
    "silver.ai_token_usage",
    "silver.ai_facts",
    "silver.ai_reviews",
    "silver.emotion_segments",
    "silver.tag_usage_profile",
    "silver.user_trait_segments",
    "silver.ai_explorer_interactions",
    "silver.ai_explorer_events",
    "gold.weekly_summary",
    "gold.goal_progress",
    "gold.ai_usage_weekly",
    "gold.ai_quality_weekly",
    "gold.user_interest_profile",
    "gold.user_trait_profile",
    "gold.emotion_weekly",
]


def compact_tables(catalog: Catalog) -> dict[str, int]:
    """Append-only テーブルの小ファイルを統合.

    Returns:
        テーブル名 → rewrite されたファイル数
    """
    results: dict[str, int] = {}
    for table_name in COMPACTION_TABLES:
        try:
            table = catalog.load_table(table_name)
            # PyIceberg の rewrite_data_files は target_size_bytes で制御
            result = table.rewrite_data_files(
                target_size_bytes=128 * 1024 * 1024,  # 128MB
            )
            rewritten = getattr(result, "rewritten_data_files_count", 0)
            results[table_name] = rewritten
            if rewritten > 0:
                logger.info(f"Compacted {table_name}: {rewritten} files rewritten")
        except Exception as e:
            logger.warning(f"Compaction failed for {table_name}: {e}")
            results[table_name] = 0

    return results


def expire_old_snapshots(catalog: Catalog, days: int = 7) -> dict[str, int]:
    """古いスナップショットを削除.

    Args:
        days: この日数より古いスナップショットを削除

    Returns:
        テーブル名 → 削除されたスナップショット数
    """
    cutoff = datetime.now(timezone.utc) - timedelta(days=days)
    cutoff_ms = int(cutoff.timestamp() * 1000)
    results: dict[str, int] = {}

    for table_name in ALL_TABLES:
        try:
            table = catalog.load_table(table_name)
            snapshots_before = len(list(table.metadata.snapshots))
            table.manage_snapshots().expire_snapshots_older_than(cutoff_ms).commit()
            snapshots_after = len(list(table.metadata.snapshots))
            expired = snapshots_before - snapshots_after
            results[table_name] = expired
            if expired > 0:
                logger.info(f"Expired {expired} snapshots from {table_name}")
        except Exception as e:
            logger.warning(f"Snapshot expiry failed for {table_name}: {e}")
            results[table_name] = 0

    return results
