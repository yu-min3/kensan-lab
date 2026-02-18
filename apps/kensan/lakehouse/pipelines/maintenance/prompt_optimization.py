"""
Prompt Optimization Trigger: kensan-ai の /admin/run-prompt-optimization を呼び出す

全アクティブコンテキストの会話品質を評価し、改善が必要なプロンプトに対して
最適化版を生成する。Dagster スケジュールジョブから利用。
"""

from __future__ import annotations

from catalog.config import setup_logging
from pipelines.maintenance.common import trigger_kensan_ai_endpoint

logger = setup_logging("maintenance.prompt_optimization")


def trigger_prompt_optimization(base_url: str) -> dict:
    """kensan-ai の run-prompt-optimization エンドポイントを呼び出す.

    Args:
        base_url: kensan-ai の base URL (例: http://localhost:8089)

    Returns:
        {"contexts_evaluated", "experiments_created", "errors"}
    """
    result = trigger_kensan_ai_endpoint(
        "/admin/run-prompt-optimization",
        base_url,
        timeout=600,
    )
    logger.info(
        "Prompt optimization complete: %d evaluated, %d experiments, %d errors",
        result.get("contexts_evaluated", 0),
        result.get("experiments_created", 0),
        len(result.get("errors", [])),
    )
    return result
