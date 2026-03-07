"""
Silver Explorer Assets: Bronze AI Explorer → Silver AI Explorer
dev/prod 両カタログに対して変換を実行
"""

from dagster import AssetExecutionContext, MaterializeResult, MetadataValue, asset

from dagster_project.resources import IcebergCatalogResource
from pipelines.silver.explorer_transform import (
    transform_explorer_events,
    transform_explorer_interactions,
)


@asset(
    deps=["bronze_ai_explorer_events_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_ai_explorer_interactions(
    context: AssetExecutionContext,
    iceberg_catalog: IcebergCatalogResource,
):
    total = 0
    for env, catalog in iceberg_catalog.get_catalogs().items():
        count = transform_explorer_interactions(catalog)
        context.log.info(f"[{env}] Transformed {count} explorer interactions")
        total += count
    return MaterializeResult(
        metadata={"interaction_count": MetadataValue.int(total)}
    )


@asset(
    deps=["bronze_ai_explorer_events_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_ai_explorer_events(
    context: AssetExecutionContext,
    iceberg_catalog: IcebergCatalogResource,
):
    total = 0
    for env, catalog in iceberg_catalog.get_catalogs().items():
        count = transform_explorer_events(catalog)
        context.log.info(f"[{env}] Transformed {count} explorer events")
        total += count
    return MaterializeResult(
        metadata={"event_count": MetadataValue.int(total)}
    )
