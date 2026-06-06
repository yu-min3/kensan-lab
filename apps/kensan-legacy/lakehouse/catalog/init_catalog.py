"""
Polaris Catalog にIcebergテーブルを作成する。
冪等: 既存テーブルはスキップ。
"""

from pyiceberg.catalog import load_catalog
from pyiceberg.schema import Schema
from pyiceberg.types import (
    BooleanType,
    DateType,
    FloatType,
    IntegerType,
    ListType,
    LongType,
    NestedField,
    StringType,
    TimestamptzType,
)
from pyiceberg.partitioning import PartitionSpec, PartitionField
from pyiceberg.transforms import MonthTransform

from catalog.config import get_catalog


def create_namespaces(catalog):
    """Bronze / Silver / Gold ネームスペースを作成"""
    for ns in ["bronze", "silver", "gold"]:
        try:
            catalog.create_namespace(ns)
            print(f"  Created namespace: {ns}")
        except Exception:
            print(f"  Namespace already exists: {ns}")


def create_bronze_tables(catalog):
    """Bronze層: PostgreSQLの生データをそのまま格納"""

    # bronze.time_entries_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "start_datetime", TimestamptzType()),
        NestedField(4, "end_datetime", TimestamptzType()),
        NestedField(5, "task_id", StringType()),
        NestedField(6, "task_name", StringType()),
        NestedField(7, "milestone_id", StringType()),
        NestedField(8, "milestone_name", StringType()),
        NestedField(9, "goal_id", StringType()),
        NestedField(10, "goal_name", StringType()),
        NestedField(11, "goal_color", StringType()),
        NestedField(12, "description", StringType()),
        NestedField(13, "created_at", TimestamptzType()),
        NestedField(14, "updated_at", TimestamptzType()),
        NestedField(15, "_ingested_at", TimestamptzType()),
    )
    partition_spec = PartitionSpec(
        PartitionField(3, 1000, MonthTransform(), "start_datetime_month")
    )
    _create_table(catalog, "bronze.time_entries_raw", schema, partition_spec)

    # bronze.tasks_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "milestone_id", StringType()),
        NestedField(4, "parent_task_id", StringType()),
        NestedField(5, "name", StringType()),
        NestedField(6, "estimated_minutes", IntegerType()),
        NestedField(7, "completed", BooleanType()),
        NestedField(8, "due_date", DateType()),
        NestedField(9, "frequency", StringType()),
        NestedField(10, "days_of_week", ListType(100, IntegerType(), element_required=False)),
        NestedField(11, "sort_order", IntegerType()),
        NestedField(12, "created_at", TimestamptzType()),
        NestedField(13, "updated_at", TimestamptzType()),
        NestedField(14, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.tasks_raw", schema)

    # bronze.notes_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "type", StringType()),
        NestedField(4, "title", StringType()),
        NestedField(5, "content", StringType()),
        NestedField(6, "format", StringType()),
        NestedField(7, "date", DateType()),
        NestedField(8, "task_id", StringType()),
        NestedField(9, "milestone_id", StringType()),
        NestedField(10, "goal_id", StringType()),
        NestedField(11, "milestone_name", StringType()),
        NestedField(12, "goal_name", StringType()),
        NestedField(13, "goal_color", StringType()),
        NestedField(14, "archived", BooleanType()),
        NestedField(15, "created_at", TimestamptzType()),
        NestedField(16, "updated_at", TimestamptzType()),
        NestedField(17, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.notes_raw", schema)

    # bronze.tags_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "name", StringType()),
        NestedField(4, "color", StringType()),
        NestedField(5, "type", StringType()),
        NestedField(6, "category", StringType()),
        NestedField(7, "pinned", BooleanType()),
        NestedField(8, "usage_count", IntegerType()),
        NestedField(9, "created_at", TimestamptzType()),
        NestedField(10, "updated_at", TimestamptzType()),
        NestedField(11, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.tags_raw", schema)

    # bronze.note_tags_raw (note_id <-> tag_id mapping)
    schema = Schema(
        NestedField(1, "note_id", StringType()),
        NestedField(2, "tag_id", StringType()),
        NestedField(3, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.note_tags_raw", schema)


def create_bronze_ai_tables(catalog):
    """Bronze層: AI関連データをPostgreSQLから格納"""

    # bronze.ai_interactions_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "session_id", StringType()),
        NestedField(4, "situation", StringType()),
        NestedField(5, "context_id", StringType()),
        NestedField(6, "user_input", StringType()),
        NestedField(7, "ai_output", StringType()),
        NestedField(8, "tool_calls_json", StringType()),
        NestedField(9, "tokens_input", IntegerType()),
        NestedField(10, "tokens_output", IntegerType()),
        NestedField(11, "latency_ms", IntegerType()),
        NestedField(12, "rating", IntegerType()),
        NestedField(13, "feedback", StringType()),
        NestedField(14, "conversation_id", StringType()),
        NestedField(15, "created_at", TimestamptzType()),
        NestedField(16, "_ingested_at", TimestamptzType()),
    )
    partition_spec = PartitionSpec(
        PartitionField(15, 1001, MonthTransform(), "created_at_month")
    )
    _create_table(catalog, "bronze.ai_interactions_raw", schema, partition_spec)

    # bronze.ai_facts_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "fact_type", StringType()),
        NestedField(4, "content", StringType()),
        NestedField(5, "source", StringType()),
        NestedField(6, "confidence", FloatType()),
        NestedField(7, "source_interaction_id", StringType()),
        NestedField(8, "created_at", TimestamptzType()),
        NestedField(9, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.ai_facts_raw", schema)

    # bronze.ai_reviews_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "week_start", DateType()),
        NestedField(4, "week_end", DateType()),
        NestedField(5, "summary", StringType()),
        NestedField(6, "good_points_json", StringType()),
        NestedField(7, "improvement_points_json", StringType()),
        NestedField(8, "advice_json", StringType()),
        NestedField(9, "tokens_input", IntegerType()),
        NestedField(10, "tokens_output", IntegerType()),
        NestedField(11, "created_at", TimestamptzType()),
        NestedField(12, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.ai_reviews_raw", schema)

    # bronze.ai_contexts_raw
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "name", StringType()),
        NestedField(3, "situation", StringType()),
        NestedField(4, "version", StringType()),
        NestedField(5, "is_active", BooleanType()),
        NestedField(6, "system_prompt", StringType()),
        NestedField(7, "allowed_tools_json", StringType()),
        NestedField(8, "max_turns", IntegerType()),
        NestedField(9, "temperature", FloatType()),
        NestedField(10, "experiment_id", StringType()),
        NestedField(11, "created_at", TimestamptzType()),
        NestedField(12, "updated_at", TimestamptzType()),
        NestedField(13, "_ingested_at", TimestamptzType()),
    )
    _create_table(catalog, "bronze.ai_contexts_raw", schema)

    # bronze.external_tool_results_raw (kensan-ai direct write)
    schema = Schema(
        NestedField(1, "tool_name", StringType()),
        NestedField(2, "input_data", StringType()),
        NestedField(3, "result_json", StringType()),
        NestedField(4, "result_count", IntegerType()),
        NestedField(5, "metadata_json", StringType()),
        NestedField(6, "executed_at", TimestamptzType()),
        NestedField(7, "_ingested_at", TimestamptzType()),
    )
    partition_spec = PartitionSpec(
        PartitionField(6, 1002, MonthTransform(), "executed_at_month")
    )
    _create_table(catalog, "bronze.external_tool_results_raw", schema, partition_spec)


def create_silver_tables(catalog):
    """Silver層: クリーニング・正規化済みデータ"""

    # silver.time_entries
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "date", DateType()),
        NestedField(4, "start_datetime", TimestamptzType()),
        NestedField(5, "end_datetime", TimestamptzType()),
        NestedField(6, "duration_minutes", IntegerType()),  # 算出値
        NestedField(7, "task_id", StringType()),
        NestedField(8, "task_name", StringType()),
        NestedField(9, "goal_name", StringType()),
        NestedField(10, "goal_color", StringType()),
        NestedField(11, "description", StringType()),
        NestedField(12, "created_at", TimestamptzType()),
        NestedField(13, "updated_at", TimestamptzType()),
    )
    partition_spec = PartitionSpec(
        PartitionField(3, 1000, MonthTransform(), "date_month")
    )
    _create_table(catalog, "silver.time_entries", schema, partition_spec)

    # silver.tasks
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "name", StringType()),
        NestedField(4, "completed", BooleanType()),
        NestedField(5, "milestone_id", StringType()),
        NestedField(6, "parent_task_id", StringType()),
        NestedField(7, "is_subtask", BooleanType()),  # parent有無
        NestedField(8, "estimated_minutes", IntegerType()),
        NestedField(9, "due_date", DateType()),
        NestedField(10, "frequency", StringType()),
        NestedField(11, "created_at", TimestamptzType()),
        NestedField(12, "updated_at", TimestamptzType()),
    )
    _create_table(catalog, "silver.tasks", schema)

    # silver.notes
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "type", StringType()),
        NestedField(4, "title", StringType()),
        NestedField(5, "content_length", IntegerType()),  # 本文長
        NestedField(6, "format", StringType()),
        NestedField(7, "date", DateType()),
        NestedField(8, "goal_name", StringType()),
        NestedField(9, "archived", BooleanType()),
        NestedField(10, "created_at", TimestamptzType()),
        NestedField(11, "updated_at", TimestamptzType()),
    )
    _create_table(catalog, "silver.notes", schema)


def create_silver_ai_tables(catalog):
    """Silver層: AI分析用の整形済みデータ"""

    # silver.ai_interactions
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "date", DateType()),
        NestedField(4, "situation", StringType()),
        NestedField(5, "user_input", StringType()),
        NestedField(6, "ai_output_length", IntegerType()),
        NestedField(7, "tool_count", IntegerType()),
        NestedField(8, "tool_names_json", StringType()),
        NestedField(9, "tokens_input", IntegerType()),
        NestedField(10, "tokens_output", IntegerType()),
        NestedField(11, "tokens_total", IntegerType()),
        NestedField(12, "latency_ms", IntegerType()),
        NestedField(13, "has_feedback", BooleanType()),
        NestedField(14, "rating", IntegerType()),
        NestedField(15, "conversation_id", StringType()),
        NestedField(16, "created_at", TimestamptzType()),
    )
    partition_spec = PartitionSpec(
        PartitionField(3, 1003, MonthTransform(), "date_month")
    )
    _create_table(catalog, "silver.ai_interactions", schema, partition_spec)

    # silver.ai_token_usage
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "date", DateType()),
        NestedField(3, "situation", StringType()),
        NestedField(4, "interaction_count", IntegerType()),
        NestedField(5, "tokens_input_total", LongType()),
        NestedField(6, "tokens_output_total", LongType()),
        NestedField(7, "tokens_total", LongType()),
        NestedField(8, "avg_latency_ms", IntegerType()),
    )
    _create_table(catalog, "silver.ai_token_usage", schema)

    # silver.ai_facts - Bronze ai_facts_raw から変換
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "date", DateType()),  # created_at から抽出
        NestedField(4, "fact_type", StringType()),
        NestedField(5, "content_length", IntegerType()),  # 本文長
        NestedField(6, "source", StringType()),
        NestedField(7, "confidence", FloatType()),
        NestedField(8, "created_at", TimestamptzType()),
    )
    _create_table(catalog, "silver.ai_facts", schema)

    # silver.ai_reviews - Bronze ai_reviews_raw から変換
    schema = Schema(
        NestedField(1, "id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "week_start", DateType()),
        NestedField(4, "week_end", DateType()),
        NestedField(5, "summary_length", IntegerType()),  # サマリー長
        NestedField(6, "good_points_count", IntegerType()),  # 良い点の数
        NestedField(7, "improvement_points_count", IntegerType()),  # 改善点の数
        NestedField(8, "advice_count", IntegerType()),  # アドバイスの数
        NestedField(9, "tokens_input", IntegerType()),
        NestedField(10, "tokens_output", IntegerType()),
        NestedField(11, "tokens_total", IntegerType()),  # 算出値
        NestedField(12, "created_at", TimestamptzType()),
    )
    _create_table(catalog, "silver.ai_reviews", schema)

    # silver.tag_usage_profile — タグ使用統計
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "tag_id", StringType()),
        NestedField(3, "tag_name", StringType()),
        NestedField(4, "note_count", IntegerType()),
        NestedField(5, "note_types_json", StringType()),
        NestedField(6, "first_used", DateType()),
        NestedField(7, "last_used", DateType()),
        NestedField(8, "co_tags_json", StringType()),
        NestedField(9, "monthly_trend", StringType()),
    )
    _create_table(catalog, "silver.tag_usage_profile", schema)

    # silver.user_trait_segments — LLMで抽出したユーザー性質セグメント
    schema = Schema(
        NestedField(1, "note_id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "date", DateType()),
        NestedField(4, "trait_category", StringType()),
        NestedField(5, "trait_value", StringType()),
        NestedField(6, "evidence", StringType()),
        NestedField(7, "confidence", FloatType()),
        NestedField(8, "source_tags_json", StringType()),
        NestedField(9, "extracted_at", TimestamptzType()),
    )
    _create_table(catalog, "silver.user_trait_segments", schema)

    # silver.emotion_segments — 日記ノートからLLMで抽出した感情セグメント
    schema = Schema(
        NestedField(1, "note_id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "date", DateType()),
        NestedField(4, "time_hint", StringType()),  # morning|afternoon|evening|unknown
        NestedField(5, "valence", FloatType()),  # -1.0〜1.0
        NestedField(6, "energy", FloatType()),  # 0.0〜1.0
        NestedField(7, "stress", FloatType()),  # 0.0〜1.0
        NestedField(8, "dominant_emotion", StringType()),  # joy, focus, frustration等
        NestedField(9, "keywords_json", StringType()),  # JSON配列
        NestedField(10, "related_tasks_json", StringType()),  # JSON配列 [{task_id, task_name}]
        NestedField(11, "confidence", FloatType()),  # 0.0〜1.0
        NestedField(12, "extracted_at", TimestamptzType()),
    )
    _create_table(catalog, "silver.emotion_segments", schema)


def create_gold_tables(catalog):
    """Gold層: 分析用集計テーブル"""

    # gold.weekly_summary
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "week_start", DateType()),
        NestedField(3, "total_minutes", LongType()),
        NestedField(4, "task_count", IntegerType()),
        NestedField(5, "completed_task_count", IntegerType()),
        NestedField(6, "note_count", IntegerType()),
        NestedField(7, "diary_count", IntegerType()),
        NestedField(8, "learning_count", IntegerType()),
    )
    _create_table(catalog, "gold.weekly_summary", schema)

    # gold.goal_progress
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "goal_name", StringType()),
        NestedField(3, "week_start", DateType()),
        NestedField(4, "total_minutes", LongType()),
        NestedField(5, "entry_count", IntegerType()),
    )
    _create_table(catalog, "gold.goal_progress", schema)


def create_gold_ai_tables(catalog):
    """Gold層: AI分析用の週次集計テーブル"""

    # gold.ai_usage_weekly
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "week_start", DateType()),
        NestedField(3, "interaction_count", IntegerType()),
        NestedField(4, "tokens_input_total", LongType()),
        NestedField(5, "tokens_output_total", LongType()),
        NestedField(6, "tokens_total", LongType()),
        NestedField(7, "avg_latency_ms", IntegerType()),
        NestedField(8, "situation_distribution_json", StringType()),
        NestedField(9, "tool_usage_json", StringType()),
        NestedField(10, "web_search_count", IntegerType()),
    )
    _create_table(catalog, "gold.ai_usage_weekly", schema)

    # gold.ai_quality_weekly
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "week_start", DateType()),
        NestedField(3, "rated_count", IntegerType()),
        NestedField(4, "avg_rating", FloatType()),
        NestedField(5, "fact_count", IntegerType()),
        NestedField(6, "review_generated", BooleanType()),
    )
    _create_table(catalog, "gold.ai_quality_weekly", schema)

    # gold.user_interest_profile — ユーザー関心プロファイル
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "week_start", DateType()),
        NestedField(3, "top_tags_json", StringType()),
        NestedField(4, "emerging_tags_json", StringType()),
        NestedField(5, "fading_tags_json", StringType()),
        NestedField(6, "tag_clusters_json", StringType()),
        NestedField(7, "total_tagged_notes", IntegerType()),
    )
    _create_table(catalog, "gold.user_interest_profile", schema)

    # gold.user_trait_profile — ユーザー性格プロファイル
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "updated_at", TimestamptzType()),
        NestedField(3, "work_style", StringType()),
        NestedField(4, "learning_style", StringType()),
        NestedField(5, "collaboration", StringType()),
        NestedField(6, "strengths_json", StringType()),
        NestedField(7, "challenges_json", StringType()),
        NestedField(8, "triggers_json", StringType()),
        NestedField(9, "trait_count", IntegerType()),
        NestedField(10, "avg_confidence", FloatType()),
    )
    _create_table(catalog, "gold.user_trait_profile", schema)

    # gold.emotion_weekly — 感情データの週次集計
    schema = Schema(
        NestedField(1, "user_id", StringType()),
        NestedField(2, "week_start", DateType()),
        NestedField(3, "avg_valence", FloatType()),
        NestedField(4, "avg_energy", FloatType()),
        NestedField(5, "avg_stress", FloatType()),
        NestedField(6, "dominant_emotion", StringType()),  # 最頻値
        NestedField(7, "emotion_distribution_json", StringType()),  # {"joy":3,"focus":5,...}
        NestedField(8, "diary_count", IntegerType()),
        NestedField(9, "task_correlation_json", StringType()),  # [{task_name, avg_valence, count}]
        NestedField(10, "valence_trend", StringType()),  # improving|stable|declining
    )
    _create_table(catalog, "gold.emotion_weekly", schema)


def create_bronze_explorer_tables(catalog):
    """Bronze層: Loki AI Explorer イベント生データ"""

    # bronze.ai_explorer_events_raw
    schema = Schema(
        NestedField(1, "trace_id", StringType()),
        NestedField(2, "event_type", StringType()),
        NestedField(3, "user_id", StringType()),
        NestedField(4, "conversation_id", StringType()),
        NestedField(5, "timestamp", TimestamptzType()),
        NestedField(6, "payload", StringType()),
        NestedField(7, "_ingested_at", TimestamptzType()),
    )
    partition_spec = PartitionSpec(
        PartitionField(5, 1003, MonthTransform(), "timestamp_month")
    )
    _create_table(catalog, "bronze.ai_explorer_events_raw", schema, partition_spec)


def create_silver_explorer_tables(catalog):
    """Silver層: AI Explorer インタラクション・イベント"""

    # silver.ai_explorer_interactions
    schema = Schema(
        NestedField(1, "trace_id", StringType()),
        NestedField(2, "user_id", StringType()),
        NestedField(3, "timestamp", TimestamptzType()),
        NestedField(4, "outcome", StringType()),
        NestedField(5, "model", StringType()),
        NestedField(6, "total_turns", IntegerType()),
        NestedField(7, "total_input_tokens", IntegerType()),
        NestedField(8, "total_output_tokens", IntegerType()),
        NestedField(9, "pending_action_count", IntegerType()),
        NestedField(10, "user_message", StringType()),
        NestedField(11, "context_id", StringType()),
        NestedField(12, "context_name", StringType()),
        NestedField(13, "context_version", StringType()),
        NestedField(14, "experiment_id", StringType()),
        NestedField(15, "system_prompt_length", IntegerType()),
        NestedField(16, "system_prompt_sections_json", StringType()),
        NestedField(17, "tool_count", IntegerType()),
        NestedField(18, "tool_names_json", StringType()),
        NestedField(19, "tool_definitions_length", IntegerType()),
        NestedField(20, "event_count", IntegerType()),
    )
    _create_table(catalog, "silver.ai_explorer_interactions", schema)

    # silver.ai_explorer_events
    schema = Schema(
        NestedField(1, "trace_id", StringType()),
        NestedField(2, "event_type", StringType()),
        NestedField(3, "user_id", StringType()),
        NestedField(4, "conversation_id", StringType()),
        NestedField(5, "timestamp", TimestamptzType()),
        NestedField(6, "payload", StringType()),
        NestedField(7, "event_order", IntegerType()),
    )
    _create_table(catalog, "silver.ai_explorer_events", schema)


def _create_table(catalog, name, schema, partition_spec=None):
    """テーブル作成（冪等・stale metadata自動修復）

    1. list_tables で存在確認
    2. 存在すれば load_table でメタデータ検証
    3. メタデータ破損なら drop → 再作成
    """
    ns, tbl = name.split(".")
    try:
        existing = {t[1] for t in catalog.list_tables(ns)}
        if tbl in existing:
            try:
                catalog.load_table(name)
                print(f"  Table already exists: {name}")
                return
            except Exception:
                print(f"  Stale metadata detected, recreating: {name}")
                catalog.drop_table(name)
    except Exception:
        pass

    try:
        if partition_spec:
            catalog.create_table(name, schema=schema, partition_spec=partition_spec)
        else:
            catalog.create_table(name, schema=schema)
        print(f"  Created table: {name}")
    except Exception as e:
        if "already exists" in str(e).lower():
            print(f"  Table already exists: {name}")
        else:
            raise


def main():
    catalog = get_catalog()

    print("Creating namespaces...")
    create_namespaces(catalog)

    print("Creating Bronze tables...")
    create_bronze_tables(catalog)

    print("Creating Bronze AI tables...")
    create_bronze_ai_tables(catalog)

    print("Creating Silver tables...")
    create_silver_tables(catalog)

    print("Creating Silver AI tables...")
    create_silver_ai_tables(catalog)

    print("Creating Bronze Explorer tables...")
    create_bronze_explorer_tables(catalog)

    print("Creating Silver Explorer tables...")
    create_silver_explorer_tables(catalog)

    print("Creating Gold tables...")
    create_gold_tables(catalog)

    print("Creating Gold AI tables...")
    create_gold_ai_tables(catalog)

    print("Done.")


if __name__ == "__main__":
    main()
