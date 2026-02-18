"""
Weekly Review Trigger: kensan-ai の /admin/generate-weekly-reviews を呼び出す

全アクティブユーザーの週次レビューを kensan-ai 側で自動生成する。
Dagster スケジュールジョブから利用。
"""

from __future__ import annotations

from catalog.config import setup_logging
from pipelines.maintenance.common import trigger_kensan_ai_endpoint

logger = setup_logging("maintenance.weekly_review")


def trigger_weekly_reviews(base_url: str) -> dict:
    """kensan-ai の generate-weekly-reviews エンドポイントを呼び出す.

    Args:
        base_url: kensan-ai の base URL (例: http://localhost:8089)

    Returns:
        {"period_start", "period_end", "users_processed", "reviews_generated", "skipped", "errors"}
    """
    result = trigger_kensan_ai_endpoint(
        "/admin/generate-weekly-reviews",
        base_url,
        timeout=600,
    )
    logger.info(
        "Weekly review complete: %d users, %d generated, %d skipped, %d errors",
        result.get("users_processed", 0),
        result.get("reviews_generated", 0),
        result.get("skipped", 0),
        result.get("errors", 0),
    )
    return result
