"""
Maintenance Assets: 定期メンテナンスジョブ
"""

from dagster import AssetExecutionContext, MaterializeResult, MetadataValue, RetryPolicy, asset

from dagster_project.resources import KensanAiResource
from pipelines.maintenance.reindex_chunks import reindex_pending_chunks
from pipelines.maintenance.weekly_review import trigger_weekly_reviews
from pipelines.maintenance.prompt_optimization import trigger_prompt_optimization


@asset(
    name="reindex_note_chunks",
    group_name="maintenance",
    compute_kind="http_trigger",
    retry_policy=RetryPolicy(max_retries=2, delay=30),
)
def reindex_note_chunks(
    context: AssetExecutionContext,
    kensan_ai: KensanAiResource,
):
    """index_status='pending' のノートをチャンク分割＋embedding生成."""
    base_url = kensan_ai.get_base_url()
    result = reindex_pending_chunks(base_url, batch_size=50)

    context.log.info(
        "Reindex result: %d users, %d notes, %d chunks",
        result["users_processed"],
        result["total_processed"],
        result["total_chunks"],
    )

    return MaterializeResult(
        metadata={
            "users_processed": MetadataValue.int(result["users_processed"]),
            "total_processed": MetadataValue.int(result["total_processed"]),
            "total_chunks": MetadataValue.int(result["total_chunks"]),
        }
    )


@asset(
    name="generate_weekly_reviews",
    group_name="maintenance",
    compute_kind="http_trigger",
    retry_policy=RetryPolicy(max_retries=1, delay=60),
)
def generate_weekly_reviews(
    context: AssetExecutionContext,
    kensan_ai: KensanAiResource,
):
    """全アクティブユーザーの週次レビューを自動生成."""
    base_url = kensan_ai.get_base_url()
    result = trigger_weekly_reviews(base_url)

    context.log.info(
        "Weekly review result: %d users, %d generated, %d skipped, %d errors",
        result["users_processed"],
        result["reviews_generated"],
        result["skipped"],
        result["errors"],
    )

    return MaterializeResult(
        metadata={
            "period_start": MetadataValue.text(result.get("period_start", "")),
            "period_end": MetadataValue.text(result.get("period_end", "")),
            "users_processed": MetadataValue.int(result["users_processed"]),
            "reviews_generated": MetadataValue.int(result["reviews_generated"]),
            "skipped": MetadataValue.int(result["skipped"]),
            "errors": MetadataValue.int(result["errors"]),
        }
    )


@asset(
    name="run_prompt_optimization",
    group_name="maintenance",
    compute_kind="http_trigger",
    retry_policy=RetryPolicy(max_retries=1, delay=60),
)
def run_prompt_optimization(
    context: AssetExecutionContext,
    kensan_ai: KensanAiResource,
):
    """プロンプト品質評価＋自動最適化バッチ."""
    base_url = kensan_ai.get_base_url()
    result = trigger_prompt_optimization(base_url)

    context.log.info(
        "Prompt optimization result: %d evaluated, %d experiments, %d errors",
        result["contexts_evaluated"],
        result["experiments_created"],
        len(result.get("errors", [])),
    )

    return MaterializeResult(
        metadata={
            "period_start": MetadataValue.text(result.get("period_start", "")),
            "period_end": MetadataValue.text(result.get("period_end", "")),
            "contexts_evaluated": MetadataValue.int(result["contexts_evaluated"]),
            "experiments_created": MetadataValue.int(result["experiments_created"]),
            "errors": MetadataValue.int(len(result.get("errors", []))),
        }
    )
