"""
Silver Assets: Bronze → Silver 変換
dev/prod 両カタログに対して変換を実行
"""

from dagster import AssetExecutionContext, asset

from dagster_project.resources import IcebergCatalogResource
from pipelines.silver.transform import (
    transform_ai_facts,
    transform_ai_interactions,
    transform_ai_reviews,
    transform_ai_token_usage,
    transform_notes,
    transform_tasks,
    transform_time_entries,
)


@asset(
    deps=["bronze_time_entries_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_time_entries(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_time_entries(catalog)
        context.log.info(f"[{env}] Transformed time_entries")


@asset(
    deps=["bronze_tasks_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_tasks(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_tasks(catalog)
        context.log.info(f"[{env}] Transformed tasks")


@asset(
    deps=["bronze_notes_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_notes(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_notes(catalog)
        context.log.info(f"[{env}] Transformed notes")


@asset(
    deps=["bronze_ai_interactions_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_ai_interactions(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_ai_interactions(catalog)
        context.log.info(f"[{env}] Transformed ai_interactions")


@asset(
    deps=["bronze_ai_interactions_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_ai_token_usage(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_ai_token_usage(catalog)
        context.log.info(f"[{env}] Transformed ai_token_usage")


@asset(
    deps=["bronze_ai_facts_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_ai_facts(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_ai_facts(catalog)
        context.log.info(f"[{env}] Transformed ai_facts")


@asset(
    deps=["bronze_ai_reviews_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_ai_reviews(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    for env, catalog in iceberg_catalog.get_catalogs().items():
        transform_ai_reviews(catalog)
        context.log.info(f"[{env}] Transformed ai_reviews")
