"""
Dagster Definitions の構造テスト
アセット数、依存関係、ジョブ、スケジュールが正しく定義されているかを検証
"""

from dagster import AssetKey

from dagster_project import all_assets, defs, daily_schedule, ai_explorer_schedule, reindex_schedule, weekly_review_schedule, prompt_optimization_schedule, full_pipeline, ai_explorer_pipeline, reindex_pipeline, weekly_review_pipeline, prompt_optimization_pipeline
from dagster_project.assets.bronze import bronze_assets


# ---------------------------------------------------------------------------
# アセット登録
# ---------------------------------------------------------------------------

def test_total_asset_count():
    """Bronze 10 + Silver 12 + Gold 7 + Maintenance 3 = 32 アセットが登録されている"""
    assert len(all_assets) == 32


def test_bronze_asset_count():
    """Bronze レイヤーは 9 テーブル分 (PG) + 1 (Loki) = 10 アセットを持つ"""
    bronze = [a for a in all_assets if hasattr(a, 'key') and a.key.to_user_string().startswith("bronze_")]
    assert len(bronze) == 10


def test_bronze_pg_asset_names():
    """Bronze PG アセット名が期待通り"""
    names = {a.key.to_user_string() for a in bronze_assets}
    expected = {
        "bronze_time_entries_raw",
        "bronze_tasks_raw",
        "bronze_notes_raw",
        "bronze_tags_raw",
        "bronze_note_tags_raw",
        "bronze_ai_interactions_raw",
        "bronze_ai_facts_raw",
        "bronze_ai_reviews_raw",
        "bronze_ai_contexts_raw",
    }
    assert names == expected


def test_bronze_loki_asset_exists():
    """Bronze Loki アセットが存在する"""
    bronze = [a for a in all_assets if hasattr(a, 'key') and a.key.to_user_string().startswith("bronze_")]
    names = {a.key.to_user_string() for a in bronze}
    assert "bronze_ai_explorer_events_raw" in names


def test_silver_asset_names():
    """Silver アセット名が期待通り"""
    silver = [a for a in all_assets if hasattr(a, 'key') and a.key.to_user_string().startswith("silver_")]
    names = {a.key.to_user_string() for a in silver}
    expected = {
        "silver_time_entries",
        "silver_tasks",
        "silver_notes",
        "silver_ai_interactions",
        "silver_ai_token_usage",
        "silver_ai_facts",
        "silver_ai_reviews",
        "silver_emotion_segments",
        "silver_tag_usage_profile",
        "silver_user_trait_segments",
        "silver_ai_explorer_interactions",
        "silver_ai_explorer_events",
    }
    assert names == expected


def test_gold_asset_names():
    """Gold アセット名が期待通り"""
    gold = [a for a in all_assets if hasattr(a, 'key') and a.key.to_user_string().startswith("gold_")]
    names = {a.key.to_user_string() for a in gold}
    expected = {
        "gold_weekly_summary",
        "gold_goal_progress",
        "gold_ai_usage_weekly",
        "gold_ai_quality_weekly",
        "gold_user_interest_profile",
        "gold_user_trait_profile",
        "gold_emotion_weekly",
    }
    assert names == expected


# ---------------------------------------------------------------------------
# 依存関係
# ---------------------------------------------------------------------------

def _get_asset_dep_keys(asset_def) -> set[str]:
    """アセットの依存先キーを文字列 set で返す"""
    spec = next(iter(asset_def.specs))
    return {dep.asset_key.to_user_string() for dep in spec.deps}


def _find_asset(name: str):
    return next(a for a in all_assets if a.key == AssetKey(name))


def test_silver_time_entries_depends_on_bronze():
    asset = _find_asset("silver_time_entries")
    assert "bronze_time_entries_raw" in _get_asset_dep_keys(asset)


def test_silver_ai_interactions_depends_on_bronze():
    asset = _find_asset("silver_ai_interactions")
    assert "bronze_ai_interactions_raw" in _get_asset_dep_keys(asset)


def test_gold_weekly_summary_depends_on_silver():
    asset = _find_asset("gold_weekly_summary")
    deps = _get_asset_dep_keys(asset)
    assert "silver_time_entries" in deps
    assert "silver_tasks" in deps
    assert "silver_notes" in deps


def test_gold_ai_quality_depends_on_silver_only():
    """gold_ai_quality_weekly は Silver のみに依存 (Medallionパターン準拠)"""
    asset = _find_asset("gold_ai_quality_weekly")
    deps = _get_asset_dep_keys(asset)
    assert "silver_ai_interactions" in deps
    assert "silver_ai_facts" in deps
    assert "silver_ai_reviews" in deps
    # Bronze直接依存がないことを確認
    assert not any(d.startswith("bronze_") for d in deps)


def test_silver_emotion_depends_on_bronze():
    """silver_emotion_segments は bronze_notes_raw と bronze_tasks_raw に依存"""
    asset = _find_asset("silver_emotion_segments")
    deps = _get_asset_dep_keys(asset)
    assert "bronze_notes_raw" in deps
    assert "bronze_tasks_raw" in deps


def test_silver_tag_usage_depends_on_bronze():
    """silver_tag_usage_profile は 3つの Bronze テーブルに依存"""
    asset = _find_asset("silver_tag_usage_profile")
    deps = _get_asset_dep_keys(asset)
    assert "bronze_notes_raw" in deps
    assert "bronze_tags_raw" in deps
    assert "bronze_note_tags_raw" in deps


def test_gold_interest_profile_depends_on_silver():
    """gold_user_interest_profile は silver_tag_usage_profile に依存"""
    asset = _find_asset("gold_user_interest_profile")
    deps = _get_asset_dep_keys(asset)
    assert "silver_tag_usage_profile" in deps
    assert not any(d.startswith("bronze_") for d in deps)


def test_silver_trait_segments_depends_on_bronze():
    """silver_user_trait_segments は 3つの Bronze テーブルに依存"""
    asset = _find_asset("silver_user_trait_segments")
    deps = _get_asset_dep_keys(asset)
    assert "bronze_notes_raw" in deps
    assert "bronze_tags_raw" in deps
    assert "bronze_note_tags_raw" in deps


def test_gold_trait_profile_depends_on_silver():
    """gold_user_trait_profile は silver_user_trait_segments に依存"""
    asset = _find_asset("gold_user_trait_profile")
    deps = _get_asset_dep_keys(asset)
    assert "silver_user_trait_segments" in deps
    assert not any(d.startswith("bronze_") for d in deps)


def test_gold_emotion_depends_on_silver():
    """gold_emotion_weekly は silver_emotion_segments に依存"""
    asset = _find_asset("gold_emotion_weekly")
    deps = _get_asset_dep_keys(asset)
    assert "silver_emotion_segments" in deps
    assert not any(d.startswith("bronze_") for d in deps)


def test_silver_explorer_interactions_depends_on_bronze():
    """silver_ai_explorer_interactions は bronze_ai_explorer_events_raw に依存"""
    asset = _find_asset("silver_ai_explorer_interactions")
    deps = _get_asset_dep_keys(asset)
    assert "bronze_ai_explorer_events_raw" in deps


def test_silver_explorer_events_depends_on_bronze():
    """silver_ai_explorer_events は bronze_ai_explorer_events_raw に依存"""
    asset = _find_asset("silver_ai_explorer_events")
    deps = _get_asset_dep_keys(asset)
    assert "bronze_ai_explorer_events_raw" in deps


# ---------------------------------------------------------------------------
# ジョブ / スケジュール
# ---------------------------------------------------------------------------

def test_full_pipeline_job():
    """full_pipeline ジョブが存在する"""
    assert full_pipeline.name == "full_pipeline"


def test_ai_explorer_pipeline_job():
    """ai_explorer_pipeline ジョブが存在する"""
    assert ai_explorer_pipeline.name == "ai_explorer_pipeline"


def test_daily_schedule():
    """daily_schedule が毎日 02:00、初期 RUNNING"""
    assert daily_schedule.cron_schedule == "0 2 * * *"
    assert daily_schedule.default_status.value == "RUNNING"


def test_ai_explorer_schedule():
    """ai_explorer_schedule が5分ごと、初期 RUNNING"""
    assert ai_explorer_schedule.cron_schedule == "*/5 * * * *"
    assert ai_explorer_schedule.default_status.value == "RUNNING"


# ---------------------------------------------------------------------------
# Definitions 統合
# ---------------------------------------------------------------------------

def test_reindex_note_chunks_asset():
    """reindex_note_chunks アセットが存在する"""
    asset = _find_asset("reindex_note_chunks")
    assert asset.key.to_user_string() == "reindex_note_chunks"


def test_reindex_note_chunks_has_no_deps():
    """reindex_note_chunks は他のアセットに依存しない"""
    asset = _find_asset("reindex_note_chunks")
    deps = _get_asset_dep_keys(asset)
    assert len(deps) == 0


def test_reindex_pipeline_job():
    """reindex_pipeline ジョブが存在する"""
    assert reindex_pipeline.name == "reindex_pipeline"


def test_reindex_schedule():
    """reindex_schedule が10分ごと、初期 RUNNING"""
    assert reindex_schedule.cron_schedule == "*/10 * * * *"
    assert reindex_schedule.default_status.value == "RUNNING"


def test_generate_weekly_reviews_asset():
    """generate_weekly_reviews アセットが存在する"""
    asset = _find_asset("generate_weekly_reviews")
    assert asset.key.to_user_string() == "generate_weekly_reviews"


def test_generate_weekly_reviews_has_no_deps():
    """generate_weekly_reviews は他のアセットに依存しない"""
    asset = _find_asset("generate_weekly_reviews")
    deps = _get_asset_dep_keys(asset)
    assert len(deps) == 0


def test_weekly_review_pipeline_job():
    """weekly_review_pipeline ジョブが存在する"""
    assert weekly_review_pipeline.name == "weekly_review_pipeline"


def test_weekly_review_schedule():
    """weekly_review_schedule が毎週月曜 03:00、初期 RUNNING"""
    assert weekly_review_schedule.cron_schedule == "0 3 * * 1"
    assert weekly_review_schedule.default_status.value == "RUNNING"


def test_run_prompt_optimization_asset():
    """run_prompt_optimization アセットが存在する"""
    asset = _find_asset("run_prompt_optimization")
    assert asset.key.to_user_string() == "run_prompt_optimization"


def test_run_prompt_optimization_has_no_deps():
    """run_prompt_optimization は他のアセットに依存しない"""
    asset = _find_asset("run_prompt_optimization")
    deps = _get_asset_dep_keys(asset)
    assert len(deps) == 0


def test_prompt_optimization_pipeline_job():
    """prompt_optimization_pipeline ジョブが存在する"""
    assert prompt_optimization_pipeline.name == "prompt_optimization_pipeline"


def test_prompt_optimization_schedule():
    """prompt_optimization_schedule が毎週月曜 03:10、初期 RUNNING"""
    assert prompt_optimization_schedule.cron_schedule == "10 3 * * 1"
    assert prompt_optimization_schedule.default_status.value == "RUNNING"


# ---------------------------------------------------------------------------
# Definitions 統合
# ---------------------------------------------------------------------------

def test_definitions_resolve():
    """Definitions が矛盾なく解決される"""
    repo = defs.get_repository_def()
    asset_keys = repo.asset_graph.get_all_asset_keys()
    assert len(asset_keys) == 32


def test_definitions_resources():
    """iceberg_catalog, pg_dsn, loki, kensan_ai リソースが登録されている"""
    repo = defs.get_repository_def()
    resource_defs = repo.get_top_level_resources()
    assert "iceberg_catalog" in resource_defs
    assert "pg_dsn" in resource_defs
    assert "loki" in resource_defs
    assert "kensan_ai" in resource_defs
