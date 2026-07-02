"""
Silver Trait Asset: LLM性質抽出
bronze.notes_raw + bronze.tags_raw + bronze.note_tags_raw → silver.user_trait_segments
dev/prod 両カタログに対して実行
"""

import os

from dagster import AssetExecutionContext, RetryPolicy, asset

from dagster_project.resources import IcebergCatalogResource
from pipelines.silver.trait_extractor import extract_traits_batch


@asset(
    deps=["bronze_notes_raw", "bronze_tags_raw", "bronze_note_tags_raw"],
    group_name="silver",
    compute_kind="llm_extraction",
    retry_policy=RetryPolicy(max_retries=2, delay=30),
)
def silver_user_trait_segments(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    from google import genai

    api_key = os.environ.get("GOOGLE_API_KEY", "")
    if not api_key:
        context.log.warning("GOOGLE_API_KEY not set, skipping trait extraction")
        return

    client = genai.Client(api_key=api_key)
    model = os.environ.get("GOOGLE_MODEL", "gemini-2.0-flash")

    for env, catalog in iceberg_catalog.get_catalogs().items():
        count = extract_traits_batch(catalog, client, model)
        context.log.info(f"[{env}] Extracted {count} trait segments")
