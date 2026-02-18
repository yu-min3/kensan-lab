"""
Silver Tag Asset: タグ使用統計集計
bronze.notes_raw + bronze.tags_raw + bronze.note_tags_raw → silver.tag_usage_profile
"""

from dagster import AssetExecutionContext, asset

from dagster_project.resources import IcebergCatalogResource
from pipelines.silver.tag_profiler import build_tag_usage_profile


@asset(
    deps=["bronze_notes_raw", "bronze_tags_raw", "bronze_note_tags_raw"],
    group_name="silver",
    compute_kind="pyarrow",
)
def silver_tag_usage_profile(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    catalog = iceberg_catalog.get_catalog()
    count = build_tag_usage_profile(catalog)
    context.log.info(f"Built tag usage profiles for {count} (user, tag) pairs")
