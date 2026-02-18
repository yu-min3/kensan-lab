"""
Kensan Lakehouse - Dagster Definitions
全32アセット (Bronze 10 + Silver 12 + Gold 7 + Maintenance 3) をオーケストレーション
"""

from dagster import AssetSelection, DefaultScheduleStatus, Definitions, ScheduleDefinition, define_asset_job

from dagster_project.assets.bronze import bronze_assets
from dagster_project.assets.bronze_loki import bronze_ai_explorer_events_raw
from dagster_project.assets.gold import (
    gold_ai_quality_weekly,
    gold_ai_usage_weekly,
    gold_emotion_weekly,
    gold_goal_progress,
    gold_user_interest_profile,
    gold_user_trait_profile,
    gold_weekly_summary,
)
from dagster_project.assets.silver import (
    silver_ai_facts,
    silver_ai_interactions,
    silver_ai_reviews,
    silver_ai_token_usage,
    silver_notes,
    silver_tasks,
    silver_time_entries,
)
from dagster_project.assets.silver_emotion import silver_emotion_segments
from dagster_project.assets.silver_explorer import (
    silver_ai_explorer_events,
    silver_ai_explorer_interactions,
)
from dagster_project.assets.silver_tags import silver_tag_usage_profile
from dagster_project.assets.silver_traits import silver_user_trait_segments
from dagster_project.assets.maintenance import reindex_note_chunks, generate_weekly_reviews, run_prompt_optimization
from dagster_project.resources import IcebergCatalogResource, KensanAiResource, LokiResource, PostgresDsnResource

all_assets = [
    *bronze_assets,
    bronze_ai_explorer_events_raw,
    silver_time_entries,
    silver_tasks,
    silver_notes,
    silver_ai_interactions,
    silver_ai_token_usage,
    silver_ai_facts,
    silver_ai_reviews,
    silver_emotion_segments,
    silver_tag_usage_profile,
    silver_user_trait_segments,
    silver_ai_explorer_interactions,
    silver_ai_explorer_events,
    gold_weekly_summary,
    gold_goal_progress,
    gold_ai_usage_weekly,
    gold_ai_quality_weekly,
    gold_user_interest_profile,
    gold_user_trait_profile,
    gold_emotion_weekly,
    reindex_note_chunks,
    generate_weekly_reviews,
    run_prompt_optimization,
]

full_pipeline = define_asset_job(
    name="full_pipeline",
    selection="*",
    description="Bronze → Silver → Gold 全アセット実行",
)

ai_explorer_pipeline = define_asset_job(
    name="ai_explorer_pipeline",
    selection=AssetSelection.assets(
        bronze_ai_explorer_events_raw,
        silver_ai_explorer_interactions,
        silver_ai_explorer_events,
    ),
    description="Loki → Bronze → Silver AI Explorer パイプライン",
)

reindex_pipeline = define_asset_job(
    name="reindex_pipeline",
    selection=AssetSelection.assets(reindex_note_chunks),
    description="pending ノートのチャンクインデックス再構築",
)

weekly_review_pipeline = define_asset_job(
    name="weekly_review_pipeline",
    selection=AssetSelection.assets(generate_weekly_reviews),
    description="週次レビュー自動生成",
)

prompt_optimization_pipeline = define_asset_job(
    name="prompt_optimization_pipeline",
    selection=AssetSelection.assets(run_prompt_optimization),
    description="プロンプト品質評価＋自動最適化",
)

daily_schedule = ScheduleDefinition(
    name="daily_schedule",
    job=full_pipeline,
    cron_schedule="0 2 * * *",
    default_status=DefaultScheduleStatus.RUNNING,
)

ai_explorer_schedule = ScheduleDefinition(
    name="ai_explorer_schedule",
    job=ai_explorer_pipeline,
    cron_schedule="*/5 * * * *",
    default_status=DefaultScheduleStatus.RUNNING,
)

reindex_schedule = ScheduleDefinition(
    name="reindex_schedule",
    job=reindex_pipeline,
    cron_schedule="*/10 * * * *",
    default_status=DefaultScheduleStatus.RUNNING,
)

weekly_review_schedule = ScheduleDefinition(
    name="weekly_review_schedule",
    job=weekly_review_pipeline,
    cron_schedule="0 3 * * 1",
    default_status=DefaultScheduleStatus.RUNNING,
)

prompt_optimization_schedule = ScheduleDefinition(
    name="prompt_optimization_schedule",
    job=prompt_optimization_pipeline,
    cron_schedule="10 3 * * 1",
    default_status=DefaultScheduleStatus.RUNNING,
)

defs = Definitions(
    assets=all_assets,
    jobs=[full_pipeline, ai_explorer_pipeline, reindex_pipeline, weekly_review_pipeline, prompt_optimization_pipeline],
    schedules=[daily_schedule, ai_explorer_schedule, reindex_schedule, weekly_review_schedule, prompt_optimization_schedule],
    resources={
        "iceberg_catalog": IcebergCatalogResource(),
        "pg_dsn": PostgresDsnResource(),
        "loki": LokiResource(),
        "kensan_ai": KensanAiResource(),
    },
)
