"""
Gold Assets: Silver → Gold 週次集計
dev/prod 両カタログに対して集計を実行
"""

from dagster import AssetExecutionContext, asset

from dagster_project.resources import IcebergCatalogResource
from pipelines.gold.aggregate import (
    aggregate_ai_quality_weekly,
    aggregate_ai_usage_weekly,
    aggregate_emotion_weekly,
    aggregate_goal_progress,
    aggregate_interest_profile,
    aggregate_trait_profile,
    aggregate_weekly_summary,
)


@asset(
    deps=["silver_time_entries", "silver_tasks", "silver_notes"],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_weekly_summary(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_weekly_summary(catalog)
        context.log.info(f"[{env}] Aggregated weekly_summary")


@asset(
    deps=["silver_time_entries"],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_goal_progress(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_goal_progress(catalog)
        context.log.info(f"[{env}] Aggregated goal_progress")


@asset(
    deps=["silver_ai_interactions"],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_ai_usage_weekly(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_ai_usage_weekly(catalog)
        context.log.info(f"[{env}] Aggregated ai_usage_weekly")


@asset(
    deps=[
        "silver_ai_interactions",
        "silver_ai_facts",
        "silver_ai_reviews",
    ],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_ai_quality_weekly(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_ai_quality_weekly(catalog)
        context.log.info(f"[{env}] Aggregated ai_quality_weekly")


@asset(
    deps=["silver_tag_usage_profile"],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_user_interest_profile(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_interest_profile(catalog)
        context.log.info(f"[{env}] Aggregated interest_profile")


@asset(
    deps=["silver_user_trait_segments"],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_user_trait_profile(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_trait_profile(catalog)
        context.log.info(f"[{env}] Aggregated trait_profile")


@asset(
    deps=["silver_emotion_segments"],
    group_name="gold",
    compute_kind="aggregation",
)
def gold_emotion_weekly(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        aggregate_emotion_weekly(catalog)
        context.log.info(f"[{env}] Aggregated emotion_weekly")
