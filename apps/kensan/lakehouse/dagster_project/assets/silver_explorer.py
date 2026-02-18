"""
Silver Explorer Assets: Bronze AI Explorer → Silver AI Explorer
インタラクション単位サマリーとイベント単位テーブルを作成
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
    catalog = iceberg_catalog.get_catalog()
    count = transform_explorer_interactions(catalog)
    context.log.info(f"Transformed {count} explorer interactions")
    return MaterializeResult(
        metadata={"interaction_count": MetadataValue.int(count)}
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
    catalog = iceberg_catalog.get_catalog()
    count = transform_explorer_events(catalog)
    context.log.info(f"Transformed {count} explorer events")
    return MaterializeResult(
        metadata={"event_count": MetadataValue.int(count)}
    )
