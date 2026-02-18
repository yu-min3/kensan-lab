"""
Bronze Loki Asset: Loki → Iceberg Bronze層
AI Explorer イベントを Loki から取得して Bronze に append
"""

import os

from dagster import AssetExecutionContext, MaterializeResult, MetadataValue, asset

from dagster_project.resources import IcebergCatalogResource, LokiResource
from pipelines.bronze.ingest_loki import TABLE_NAME, ingest_loki_events, load_state, save_state


@asset(
    name="bronze_ai_explorer_events_raw",
    group_name="bronze",
    compute_kind="loki_ingestion",
)
def bronze_ai_explorer_events_raw(
    context: AssetExecutionContext,
    iceberg_catalog: IcebergCatalogResource,
    loki: LokiResource,
):
    catalog = iceberg_catalog.get_catalog()
    loki_url = loki.get_base_url()
    state = load_state()

    prev_ns = state.get(TABLE_NAME, "0")
    state = ingest_loki_events(catalog, loki_url, state)
    save_state(state)

    new_ns = state.get(TABLE_NAME, "0")

    return MaterializeResult(
        metadata={
            "table": MetadataValue.text(TABLE_NAME),
            "mode": MetadataValue.text("append"),
            "prev_timestamp_ns": MetadataValue.text(prev_ns),
            "new_timestamp_ns": MetadataValue.text(new_ns),
        }
    )
