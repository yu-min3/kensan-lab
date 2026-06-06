"""
Silver Transform: Bronze → Silver
クリーニング・正規化・算出カラム追加
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pyarrow as pa
import pyarrow.compute as pc

from catalog.config import get_catalog, setup_logging

if TYPE_CHECKING:
    from pyiceberg.catalog import Catalog

logger = setup_logging("silver.transform")


class TransformError(Exception):
    """Transform処理中のエラー"""

    pass


def transform_time_entries(catalog: Catalog) -> int:
    """time_entries: date抽出、duration_minutes算出、不要カラム除去

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.time_entries_raw")
        silver = catalog.load_table("silver.time_entries")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load time_entries tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.time_entries_raw")
        return 0

    # Vectorized: extract date from start_datetime
    dates = pc.cast(df.column("start_datetime"), pa.date32())

    # Vectorized: duration = (end - start) in minutes
    start_us = pc.cast(df.column("start_datetime"), pa.int64())
    end_us = pc.cast(df.column("end_datetime"), pa.int64())
    diff_us = pc.subtract(end_us, start_us)
    # microseconds → minutes: divide by 60_000_000
    duration_minutes = pc.cast(
        pc.divide(diff_us, 60_000_000), pa.int32()
    )
    # Clamp negatives to 0
    duration_minutes = pc.max_element_wise(duration_minutes, pa.scalar(0, pa.int32()))

    try:
        silver_table = pa.table({
            "id": df.column("id"),
            "user_id": df.column("user_id"),
            "date": dates,
            "start_datetime": df.column("start_datetime"),
            "end_datetime": df.column("end_datetime"),
            "duration_minutes": duration_minutes,
            "task_id": df.column("task_id"),
            "task_name": df.column("task_name"),
            "goal_name": df.column("goal_name"),
            "goal_color": df.column("goal_color"),
            "description": df.column("description"),
            "created_at": df.column("created_at"),
            "updated_at": df.column("updated_at"),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write time_entries: {e}") from e

    logger.info(f"Transformed {len(silver_table)} time entries to Silver")
    return len(silver_table)


def transform_tasks(catalog: Catalog) -> int:
    """tasks: is_subtaskフラグ追加、不要カラム除去

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.tasks_raw")
        silver = catalog.load_table("silver.tasks")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load tasks tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.tasks_raw")
        return 0

    # is_subtask: parent_task_id がnullでなければTrue
    is_subtask = pc.is_valid(df.column("parent_task_id"))

    try:
        silver_table = pa.table({
            "id": df.column("id"),
            "user_id": df.column("user_id"),
            "name": df.column("name"),
            "completed": df.column("completed"),
            "milestone_id": df.column("milestone_id"),
            "parent_task_id": df.column("parent_task_id"),
            "is_subtask": is_subtask,
            "estimated_minutes": df.column("estimated_minutes"),
            "due_date": df.column("due_date"),
            "frequency": df.column("frequency"),
            "created_at": df.column("created_at"),
            "updated_at": df.column("updated_at"),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write tasks: {e}") from e

    logger.info(f"Transformed {len(silver_table)} tasks to Silver")
    return len(silver_table)


def transform_notes(catalog: Catalog) -> int:
    """notes: content_length算出、本文除去

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.notes_raw")
        silver = catalog.load_table("silver.notes")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load notes tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.notes_raw")
        return 0

    # Vectorized: content_length via PyArrow utf8_length
    content_col = df.column("content")
    # Replace nulls with empty string for length calc
    content_filled = pc.if_else(pc.is_null(content_col), "", content_col)
    content_length = pc.cast(pc.utf8_length(content_filled), pa.int32())

    try:
        silver_table = pa.table({
            "id": df.column("id"),
            "user_id": df.column("user_id"),
            "type": df.column("type"),
            "title": df.column("title"),
            "content_length": content_length,
            "format": df.column("format"),
            "date": df.column("date"),
            "goal_name": df.column("goal_name"),
            "archived": df.column("archived"),
            "created_at": df.column("created_at"),
            "updated_at": df.column("updated_at"),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write notes: {e}") from e

    logger.info(f"Transformed {len(silver_table)} notes to Silver")
    return len(silver_table)


def transform_ai_interactions(catalog: Catalog) -> int:
    """ai_interactions: date抽出、ツール情報解析、トークン合計算出

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.ai_interactions_raw")
        silver = catalog.load_table("silver.ai_interactions")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load ai_interactions tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.ai_interactions_raw")
        return 0

    dates: list = []
    ai_output_lengths: list[int] = []
    tool_counts: list[int] = []
    tool_names_list: list[str] = []
    tokens_totals: list[int] = []
    has_feedbacks: list[bool] = []

    for i in range(len(df)):
        created = df.column("created_at")[i].as_py()
        ai_output = df.column("ai_output")[i].as_py()
        tool_calls_json = df.column("tool_calls_json")[i].as_py()
        tokens_in = df.column("tokens_input")[i].as_py() or 0
        tokens_out = df.column("tokens_output")[i].as_py() or 0
        rating = df.column("rating")[i].as_py()

        # date
        dates.append(created.date() if created else None)

        # ai_output_length
        ai_output_lengths.append(len(ai_output) if ai_output else 0)

        # tool_count and tool_names
        if tool_calls_json:
            try:
                calls = json.loads(tool_calls_json)
                tool_counts.append(len(calls))
                names = list({c.get("name", "") for c in calls if isinstance(c, dict)})
                tool_names_list.append(json.dumps(names, ensure_ascii=False))
            except (json.JSONDecodeError, TypeError):
                tool_counts.append(0)
                tool_names_list.append("[]")
        else:
            tool_counts.append(0)
            tool_names_list.append("[]")

        # tokens_total
        tokens_totals.append(tokens_in + tokens_out)

        # has_feedback
        has_feedbacks.append(rating is not None)

    try:
        silver_table = pa.table({
            "id": df.column("id"),
            "user_id": df.column("user_id"),
            "date": pa.array(dates, type=pa.date32()),
            "situation": df.column("situation"),
            "user_input": df.column("user_input"),
            "ai_output_length": pa.array(ai_output_lengths, type=pa.int32()),
            "tool_count": pa.array(tool_counts, type=pa.int32()),
            "tool_names_json": pa.array(tool_names_list, type=pa.string()),
            "tokens_input": df.column("tokens_input"),
            "tokens_output": df.column("tokens_output"),
            "tokens_total": pa.array(tokens_totals, type=pa.int32()),
            "latency_ms": df.column("latency_ms"),
            "has_feedback": pa.array(has_feedbacks, type=pa.bool_()),
            "rating": df.column("rating"),
            "conversation_id": df.column("conversation_id"),
            "created_at": df.column("created_at"),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write ai_interactions: {e}") from e

    logger.info(f"Transformed {len(silver_table)} AI interactions to Silver")
    return len(silver_table)


def transform_ai_token_usage(catalog: Catalog) -> int:
    """ai_token_usage: 日別×situation別のトークン使用量集計

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.ai_interactions_raw")
        silver = catalog.load_table("silver.ai_token_usage")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load ai_token_usage tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.ai_interactions_raw for token usage")
        return 0

    # user_id × date × situation ごとに集計
    usage: dict[tuple[str, object, str], dict[str, int]] = {}

    for i in range(len(df)):
        user_id = df.column("user_id")[i].as_py()
        created = df.column("created_at")[i].as_py()
        situation = df.column("situation")[i].as_py() or "chat"
        tokens_in = df.column("tokens_input")[i].as_py() or 0
        tokens_out = df.column("tokens_output")[i].as_py() or 0
        latency = df.column("latency_ms")[i].as_py() or 0

        if created is None:
            continue

        date_val = created.date()
        key = (user_id, date_val, situation)
        if key not in usage:
            usage[key] = {
                "interaction_count": 0,
                "tokens_input_total": 0,
                "tokens_output_total": 0,
                "tokens_total": 0,
                "latency_sum": 0,
            }
        usage[key]["interaction_count"] += 1
        usage[key]["tokens_input_total"] += tokens_in
        usage[key]["tokens_output_total"] += tokens_out
        usage[key]["tokens_total"] += tokens_in + tokens_out
        usage[key]["latency_sum"] += latency

    if not usage:
        logger.info("No data to aggregate for ai_token_usage")
        return 0

    try:
        silver_table = pa.table({
            "user_id": pa.array([k[0] for k in usage], type=pa.string()),
            "date": pa.array([k[1] for k in usage], type=pa.date32()),
            "situation": pa.array([k[2] for k in usage], type=pa.string()),
            "interaction_count": pa.array(
                [v["interaction_count"] for v in usage.values()], type=pa.int32()
            ),
            "tokens_input_total": pa.array(
                [v["tokens_input_total"] for v in usage.values()], type=pa.int64()
            ),
            "tokens_output_total": pa.array(
                [v["tokens_output_total"] for v in usage.values()], type=pa.int64()
            ),
            "tokens_total": pa.array(
                [v["tokens_total"] for v in usage.values()], type=pa.int64()
            ),
            "avg_latency_ms": pa.array(
                [
                    v["latency_sum"] // v["interaction_count"] if v["interaction_count"] > 0 else 0
                    for v in usage.values()
                ],
                type=pa.int32(),
            ),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write ai_token_usage: {e}") from e

    logger.info(f"Transformed {len(silver_table)} AI token usage records to Silver")
    return len(silver_table)


def transform_ai_facts(catalog: Catalog) -> int:
    """ai_facts: Bronze ai_facts_raw から Silver ai_facts へ変換

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.ai_facts_raw")
        silver = catalog.load_table("silver.ai_facts")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load ai_facts tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.ai_facts_raw")
        return 0

    # Vectorized: date extraction and content_length
    dates = pc.cast(df.column("created_at"), pa.date32())
    content_col = df.column("content")
    content_filled = pc.if_else(pc.is_null(content_col), "", content_col)
    content_lengths = pc.cast(pc.utf8_length(content_filled), pa.int32())

    try:
        silver_table = pa.table({
            "id": df.column("id"),
            "user_id": df.column("user_id"),
            "date": dates,
            "fact_type": df.column("fact_type"),
            "content_length": content_lengths,
            "source": df.column("source"),
            "confidence": df.column("confidence"),
            "created_at": df.column("created_at"),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write ai_facts: {e}") from e

    logger.info(f"Transformed {len(silver_table)} AI facts to Silver")
    return len(silver_table)


def transform_ai_reviews(catalog: Catalog) -> int:
    """ai_reviews: Bronze ai_reviews_raw から Silver ai_reviews へ変換

    Returns:
        変換した行数

    Raises:
        TransformError: 変換処理に失敗した場合
    """
    try:
        bronze = catalog.load_table("bronze.ai_reviews_raw")
        silver = catalog.load_table("silver.ai_reviews")
        df = bronze.scan().to_arrow()
    except Exception as e:
        raise TransformError(f"Failed to load ai_reviews tables: {e}") from e

    if len(df) == 0:
        logger.info("No data in bronze.ai_reviews_raw")
        return 0

    summary_lengths: list[int] = []
    good_points_counts: list[int] = []
    improvement_points_counts: list[int] = []
    advice_counts: list[int] = []
    tokens_totals: list[int] = []

    for i in range(len(df)):
        summary = df.column("summary")[i].as_py()
        good_points_json = df.column("good_points_json")[i].as_py()
        improvement_points_json = df.column("improvement_points_json")[i].as_py()
        advice_json = df.column("advice_json")[i].as_py()
        tokens_in = df.column("tokens_input")[i].as_py() or 0
        tokens_out = df.column("tokens_output")[i].as_py() or 0

        summary_lengths.append(len(summary) if summary else 0)
        tokens_totals.append(tokens_in + tokens_out)

        # JSON配列の要素数をカウント
        for json_str, target_list in [
            (good_points_json, good_points_counts),
            (improvement_points_json, improvement_points_counts),
            (advice_json, advice_counts),
        ]:
            if json_str:
                try:
                    items = json.loads(json_str)
                    target_list.append(len(items) if isinstance(items, list) else 0)
                except (json.JSONDecodeError, TypeError):
                    target_list.append(0)
            else:
                target_list.append(0)

    try:
        silver_table = pa.table({
            "id": df.column("id"),
            "user_id": df.column("user_id"),
            "week_start": df.column("week_start"),
            "week_end": df.column("week_end"),
            "summary_length": pa.array(summary_lengths, type=pa.int32()),
            "good_points_count": pa.array(good_points_counts, type=pa.int32()),
            "improvement_points_count": pa.array(improvement_points_counts, type=pa.int32()),
            "advice_count": pa.array(advice_counts, type=pa.int32()),
            "tokens_input": df.column("tokens_input"),
            "tokens_output": df.column("tokens_output"),
            "tokens_total": pa.array(tokens_totals, type=pa.int32()),
            "created_at": df.column("created_at"),
        })
        silver.overwrite(silver_table)
    except Exception as e:
        raise TransformError(f"Failed to write ai_reviews: {e}") from e

    logger.info(f"Transformed {len(silver_table)} AI reviews to Silver")
    return len(silver_table)


def main() -> None:
    """メインエントリーポイント"""
    logger.info("Silver transformation started.")

    try:
        catalog = get_catalog()
    except Exception as e:
        logger.error(f"Failed to initialize catalog: {e}")
        raise SystemExit(1) from e

    errors: list[str] = []
    transforms = [
        ("time_entries", transform_time_entries),
        ("tasks", transform_tasks),
        ("notes", transform_notes),
        ("ai_interactions", transform_ai_interactions),
        ("ai_token_usage", transform_ai_token_usage),
        ("ai_facts", transform_ai_facts),
        ("ai_reviews", transform_ai_reviews),
    ]

    for name, func in transforms:
        try:
            func(catalog)
        except TransformError as e:
            logger.error(f"Failed to transform {name}: {e}")
            errors.append(name)
            continue

    if errors:
        logger.warning(f"Silver transformation completed with errors: {errors}")
    else:
        logger.info("Silver transformation complete.")


if __name__ == "__main__":
    main()
