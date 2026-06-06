"""
Bronze Ingestion: PostgreSQL → Iceberg Bronze層
差分取り込み: updated_at / created_at ベースの増分ロード

- mutable テーブル (updated_at): 変更があれば全件 overwrite
- append-only テーブル (since_column=created_at): 差分のみ append
- .ingestion_state.json に最終取り込み時刻を記録
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import TYPE_CHECKING, Any

import psycopg
import pyarrow as pa

from catalog.config import get_catalog, get_pg_dsn, setup_logging

if TYPE_CHECKING:
    from pyiceberg.catalog import Catalog

logger = setup_logging("bronze.ingest")

STATE_FILE = Path(__file__).parent.parent.parent / ".ingestion_state.json"


class IngestionError(Exception):
    """Ingestion処理中のエラー"""

    pass


def _is_uuid_like(value: object) -> bool:
    """値がUUID型かどうかを判定（psycopgのUUID型または文字列形式）"""
    if value is None:
        return False
    # psycopgはUUIDをuuid.UUIDとして返す
    type_name = type(value).__name__
    return type_name == "UUID" or (
        isinstance(value, str) and len(value) == 36 and value.count("-") == 4
    )

# 取り込み対象テーブルの定義
TABLES = {
    "bronze.time_entries_raw": {
        "query": """
            SELECT id, user_id, start_datetime, end_datetime,
                   task_id, task_name, milestone_id, milestone_name,
                   goal_id, goal_name, goal_color, description,
                   created_at, updated_at
            FROM time_entries
            WHERE updated_at > %(since)s
            ORDER BY updated_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("start_datetime", pa.timestamp("us", tz="UTC")),
            ("end_datetime", pa.timestamp("us", tz="UTC")),
            ("task_id", pa.string()),
            ("task_name", pa.string()),
            ("milestone_id", pa.string()),
            ("milestone_name", pa.string()),
            ("goal_id", pa.string()),
            ("goal_name", pa.string()),
            ("goal_color", pa.string()),
            ("description", pa.string()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("updated_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
    },
    "bronze.tasks_raw": {
        "query": """
            SELECT id, user_id, milestone_id, parent_task_id, name,
                   estimated_minutes, completed, due_date, frequency,
                   days_of_week, sort_order, created_at, updated_at
            FROM tasks
            WHERE updated_at > %(since)s
            ORDER BY updated_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("milestone_id", pa.string()),
            ("parent_task_id", pa.string()),
            ("name", pa.string()),
            ("estimated_minutes", pa.int32()),
            ("completed", pa.bool_()),
            ("due_date", pa.date32()),
            ("frequency", pa.string()),
            ("days_of_week", pa.list_(pa.int32())),
            ("sort_order", pa.int32()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("updated_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
    },
    "bronze.notes_raw": {
        "query": """
            SELECT id, user_id, type, title, content, format, "date",
                   task_id, milestone_id, goal_id, milestone_name,
                   goal_name, goal_color, archived, created_at, updated_at
            FROM notes
            WHERE updated_at > %(since)s
            ORDER BY updated_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("type", pa.string()),
            ("title", pa.string()),
            ("content", pa.string()),
            ("format", pa.string()),
            ("date", pa.date32()),
            ("task_id", pa.string()),
            ("milestone_id", pa.string()),
            ("goal_id", pa.string()),
            ("milestone_name", pa.string()),
            ("goal_name", pa.string()),
            ("goal_color", pa.string()),
            ("archived", pa.bool_()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("updated_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
    },
    # ===== Tag Tables =====
    "bronze.tags_raw": {
        "query": """
            SELECT id, user_id, name, color, type,
                   COALESCE(category, 'general') AS category,
                   pinned, usage_count, created_at, updated_at
            FROM tags
            WHERE updated_at > %(since)s
            ORDER BY updated_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("name", pa.string()),
            ("color", pa.string()),
            ("type", pa.string()),
            ("category", pa.string()),
            ("pinned", pa.bool_()),
            ("usage_count", pa.int32()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("updated_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
    },
    "bronze.note_tags_raw": {
        "query": """
            SELECT note_id, tag_id
            FROM note_tags
            WHERE %(since)s IS NOT NULL OR TRUE
            ORDER BY note_id
        """,
        "arrow_schema": pa.schema([
            ("note_id", pa.string()),
            ("tag_id", pa.string()),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
        "always_full": True,
    },
    # ===== AI Data Tables =====
    "bronze.ai_interactions_raw": {
        "query": """
            SELECT id, user_id, session_id, situation, context_id,
                   user_input, ai_output, tool_calls::text AS tool_calls_json,
                   tokens_input, tokens_output, latency_ms,
                   rating, feedback, conversation_id, created_at
            FROM ai_interactions
            WHERE created_at > %(since)s
            ORDER BY created_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("session_id", pa.string()),
            ("situation", pa.string()),
            ("context_id", pa.string()),
            ("user_input", pa.string()),
            ("ai_output", pa.string()),
            ("tool_calls_json", pa.string()),
            ("tokens_input", pa.int32()),
            ("tokens_output", pa.int32()),
            ("latency_ms", pa.int32()),
            ("rating", pa.int32()),
            ("feedback", pa.string()),
            ("conversation_id", pa.string()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
        "since_column": "created_at",
    },
    "bronze.ai_facts_raw": {
        "query": """
            SELECT id, user_id, fact_type, content, source,
                   confidence, source_interaction_id, created_at
            FROM user_facts
            WHERE created_at > %(since)s
            ORDER BY created_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("fact_type", pa.string()),
            ("content", pa.string()),
            ("source", pa.string()),
            ("confidence", pa.float32()),
            ("source_interaction_id", pa.string()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
        "since_column": "created_at",
    },
    "bronze.ai_reviews_raw": {
        "query": """
            SELECT id, user_id,
                   period_start AS week_start, period_end AS week_end,
                   summary,
                   array_to_json(good_points)::text AS good_points_json,
                   array_to_json(improvement_points)::text AS improvement_points_json,
                   array_to_json(advice)::text AS advice_json,
                   tokens_input, tokens_output, created_at
            FROM ai_review_reports
            WHERE created_at > %(since)s
            ORDER BY created_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("user_id", pa.string()),
            ("week_start", pa.date32()),
            ("week_end", pa.date32()),
            ("summary", pa.string()),
            ("good_points_json", pa.string()),
            ("improvement_points_json", pa.string()),
            ("advice_json", pa.string()),
            ("tokens_input", pa.int32()),
            ("tokens_output", pa.int32()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
        "since_column": "created_at",
    },
    "bronze.ai_contexts_raw": {
        "query": """
            SELECT id, name, situation, version, is_active,
                   system_prompt,
                   array_to_json(allowed_tools)::text AS allowed_tools_json,
                   max_turns, temperature, experiment_id,
                   created_at, updated_at
            FROM ai_contexts
            WHERE updated_at > %(since)s
            ORDER BY updated_at
        """,
        "arrow_schema": pa.schema([
            ("id", pa.string()),
            ("name", pa.string()),
            ("situation", pa.string()),
            ("version", pa.string()),
            ("is_active", pa.bool_()),
            ("system_prompt", pa.string()),
            ("allowed_tools_json", pa.string()),
            ("max_turns", pa.int32()),
            ("temperature", pa.float32()),
            ("experiment_id", pa.string()),
            ("created_at", pa.timestamp("us", tz="UTC")),
            ("updated_at", pa.timestamp("us", tz="UTC")),
            ("_ingested_at", pa.timestamp("us", tz="UTC")),
        ]),
    },
}

# 初回取り込み用の最小日時
EPOCH = datetime(2000, 1, 1, tzinfo=timezone.utc)


def load_state() -> dict[str, str]:
    """ステートファイルから前回取り込み時刻を読み込み"""
    if STATE_FILE.exists():
        try:
            return json.loads(STATE_FILE.read_text())
        except json.JSONDecodeError as e:
            logger.warning(f"State file corrupted, starting fresh: {e}")
            return {}
    return {}


def save_state(state: dict[str, str]) -> None:
    """ステートファイルに取り込み時刻を保存"""
    STATE_FILE.write_text(json.dumps(state, indent=2, default=str))
    logger.debug(f"State saved to {STATE_FILE}")


def get_last_ingested(state: dict[str, str], table_name: str) -> datetime:
    """テーブルの前回取り込み時刻を取得"""
    ts = state.get(table_name)
    if ts:
        return datetime.fromisoformat(ts)
    return EPOCH


def fetch_rows(dsn: str, query: str, since: datetime) -> list[dict[str, Any]]:
    """PostgreSQLからデータを取得

    Raises:
        IngestionError: PostgreSQL接続またはクエリ実行に失敗した場合
    """
    try:
        with psycopg.connect(dsn) as conn:
            with conn.cursor(row_factory=psycopg.rows.dict_row) as cur:
                cur.execute(query, {"since": since})
                return cur.fetchall()
    except psycopg.Error as e:
        raise IngestionError(f"PostgreSQL error: {e}") from e


def rows_to_arrow(rows: list[dict[str, Any]], arrow_schema: pa.Schema) -> pa.Table:
    """dict行リストをArrow Tableに変換

    UUID型のカラムは自動検出し、string型に変換する。

    Raises:
        IngestionError: Arrow Table変換に失敗した場合
    """
    now = datetime.now(timezone.utc)

    columns: dict[str, list[Any]] = {}
    try:
        for field in arrow_schema:
            if field.name == "_ingested_at":
                columns[field.name] = [now] * len(rows)
            else:
                # UUID型は動的に検出して文字列変換
                # 最初の非NULL値でUUID型かどうかを判定
                sample_value = next(
                    (row.get(field.name) for row in rows if row.get(field.name) is not None),
                    None
                )
                is_uuid_column = _is_uuid_like(sample_value)

                columns[field.name] = [
                    str(row[field.name]) if is_uuid_column and row.get(field.name) is not None
                    else row.get(field.name)
                    for row in rows
                ]

        return pa.table(columns, schema=arrow_schema)
    except (pa.ArrowInvalid, pa.ArrowTypeError) as e:
        raise IngestionError(f"Arrow conversion failed: {e}") from e


def get_max_timestamp(rows: list[dict[str, Any]], column: str) -> datetime:
    """行リストからタイムスタンプカラムの最大値を取得"""
    return max(row[column] for row in rows)


def ingest_table(
    catalog: Catalog,
    dsn: str,
    iceberg_table_name: str,
    config: dict[str, Any],
    state: dict[str, str],
) -> dict[str, str]:
    """1テーブル分のingestion。更新後のstateを返す。

    Raises:
        IngestionError: Ingestion処理に失敗した場合
    """
    try:
        table = catalog.load_table(iceberg_table_name)
    except Exception as e:
        raise IngestionError(f"Failed to load table {iceberg_table_name}: {e}") from e

    always_full = config.get("always_full", False)
    since_column = config.get("since_column", "updated_at")
    is_append_only = since_column == "created_at"
    since = get_last_ingested(state, iceberg_table_name)
    is_initial = since == EPOCH

    if always_full:
        # タイムスタンプカラムなしのテーブル: 毎回全件 overwrite
        mode = "overwrite"
        logger.info(f"Fetching: {iceberg_table_name} (full overwrite)...")
        rows = fetch_rows(dsn, config["query"], EPOCH)
    else:
        mode = "append" if is_append_only and not is_initial else "overwrite"
        if mode == "overwrite" and not is_initial:
            # mutable テーブル: 変更があるかチェックしてから全件取得
            check_rows = fetch_rows(dsn, config["query"], since)
            if not check_rows:
                logger.info(f"No changes for {iceberg_table_name} (since {since.isoformat()})")
                return state
            # 変更があったので全件再取得
            since = EPOCH

        logger.info(f"Fetching: {iceberg_table_name} ({mode}, since {since.isoformat()})...")
        rows = fetch_rows(dsn, config["query"], since)

    if not rows:
        logger.info(f"No new rows for {iceberg_table_name}")
        return state

    arrow_table = rows_to_arrow(rows, config["arrow_schema"])

    try:
        if mode == "append":
            table.append(arrow_table)
        else:
            table.overwrite(arrow_table)
    except Exception as e:
        raise IngestionError(f"Failed to write to {iceberg_table_name}: {e}") from e

    if not always_full:
        max_ts = get_max_timestamp(rows, since_column)
        state[iceberg_table_name] = max_ts.isoformat()
    logger.info(f"Ingested {len(rows)} rows into {iceberg_table_name} ({mode})")
    return state


def main() -> None:
    """メインエントリーポイント"""
    logger.info("Bronze ingestion started.")

    try:
        catalog = get_catalog()
        dsn = get_pg_dsn()
    except Exception as e:
        logger.error(f"Failed to initialize: {e}")
        raise SystemExit(1) from e

    state = load_state()
    errors: list[str] = []

    for table_name, config in TABLES.items():
        try:
            state = ingest_table(catalog, dsn, table_name, config, state)
        except IngestionError as e:
            logger.error(f"Failed to ingest {table_name}: {e}")
            errors.append(table_name)
            continue

    save_state(state)
    if errors:
        logger.warning(f"Bronze ingestion completed with errors: {errors}")
    else:
        logger.info(f"Bronze ingestion complete. State saved to {STATE_FILE.name}")


if __name__ == "__main__":
    main()
