"""
Silver Emotion Asset: LLM感情抽出
bronze.notes_raw + bronze.tasks_raw → silver.emotion_segments
"""

import os

from dagster import AssetExecutionContext, RetryPolicy, asset

from dagster_project.resources import IcebergCatalogResource
from pipelines.silver.emotion_extractor import extract_emotions_batch


@asset(
    deps=["bronze_notes_raw", "bronze_tasks_raw"],
    group_name="silver",
    compute_kind="llm_extraction",
    retry_policy=RetryPolicy(max_retries=2, delay=30),
)
def silver_emotion_segments(
    context: AssetExecutionContext, iceberg_catalog: IcebergCatalogResource
):
    from google import genai

    catalog = iceberg_catalog.get_catalog()
    api_key = os.environ.get("GOOGLE_API_KEY", "")
    if not api_key:
        context.log.warning("GOOGLE_API_KEY not set, skipping emotion extraction")
        return

    client = genai.Client(api_key=api_key)
    model = os.environ.get("GOOGLE_MODEL", "gemini-2.0-flash")
    count = extract_emotions_batch(catalog, client, model)
    context.log.info(f"Extracted emotions from {count} diary notes")
