"""
Gold Aggregation: Silver → Gold
週次サマリーとゴール別進捗の集計
"""

from __future__ import annotations

import json
from datetime import date, timedelta
from typing import TYPE_CHECKING, Any

import pyarrow as pa

from catalog.config import get_catalog, setup_logging

if TYPE_CHECKING:
    from pyiceberg.catalog import Catalog

logger = setup_logging("gold.aggregate")


class AggregationError(Exception):
    """Aggregation処理中のエラー"""

    pass


def _iso_week_start(date_val: date | None) -> date | None:
    """日付からISO週の月曜日を算出"""
    if date_val is None:
        return None
    weekday = date_val.weekday()  # 0=月曜
    return date_val - timedelta(days=weekday)


def aggregate_weekly_summary(catalog: Catalog) -> int:
    """週次サマリー: 時間、タスク、ノートの集計

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_time = catalog.load_table("silver.time_entries")
        silver_tasks = catalog.load_table("silver.tasks")
        silver_notes = catalog.load_table("silver.notes")
        gold = catalog.load_table("gold.weekly_summary")

        time_df = silver_time.scan().to_arrow()
        tasks_df = silver_tasks.scan().to_arrow()
        notes_df = silver_notes.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load weekly_summary tables: {e}") from e

    # user_id × week_start ごとに集計
    summaries: dict[tuple[str, date | None], dict[str, int]] = {}

    # 時間集計
    for i in range(len(time_df)):
        user_id = time_df.column("user_id")[i].as_py()
        date_val = time_df.column("date")[i].as_py()
        minutes = time_df.column("duration_minutes")[i].as_py() or 0

        if date_val is None:
            continue

        week_start = _iso_week_start(date_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "total_minutes": 0, "task_count": 0,
                "completed_task_count": 0, "note_count": 0,
                "diary_count": 0, "learning_count": 0,
            }
        summaries[key]["total_minutes"] += minutes

    # タスク集計 (created_atの週で集計)
    for i in range(len(tasks_df)):
        user_id = tasks_df.column("user_id")[i].as_py()
        created = tasks_df.column("created_at")[i].as_py()
        completed = tasks_df.column("completed")[i].as_py()

        if created is None:
            continue

        week_start = _iso_week_start(created.date())
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "total_minutes": 0, "task_count": 0,
                "completed_task_count": 0, "note_count": 0,
                "diary_count": 0, "learning_count": 0,
            }
        summaries[key]["task_count"] += 1
        if completed:
            summaries[key]["completed_task_count"] += 1

    # ノート集計
    for i in range(len(notes_df)):
        user_id = notes_df.column("user_id")[i].as_py()
        date_val = notes_df.column("date")[i].as_py()
        note_type = notes_df.column("type")[i].as_py()

        if date_val is None:
            created = notes_df.column("created_at")[i].as_py()
            if created:
                date_val = created.date()
            else:
                continue

        week_start = _iso_week_start(date_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "total_minutes": 0, "task_count": 0,
                "completed_task_count": 0, "note_count": 0,
                "diary_count": 0, "learning_count": 0,
            }
        summaries[key]["note_count"] += 1
        if note_type == "diary":
            summaries[key]["diary_count"] += 1
        elif note_type == "learning":
            summaries[key]["learning_count"] += 1

    if not summaries:
        logger.info("No data to aggregate for weekly_summary")
        return 0

    try:
        # Arrow Tableに変換
        gold_table = pa.table({
            "user_id": pa.array([k[0] for k in summaries], type=pa.string()),
            "week_start": pa.array([k[1] for k in summaries], type=pa.date32()),
            "total_minutes": pa.array([v["total_minutes"] for v in summaries.values()], type=pa.int64()),
            "task_count": pa.array([v["task_count"] for v in summaries.values()], type=pa.int32()),
            "completed_task_count": pa.array([v["completed_task_count"] for v in summaries.values()], type=pa.int32()),
            "note_count": pa.array([v["note_count"] for v in summaries.values()], type=pa.int32()),
            "diary_count": pa.array([v["diary_count"] for v in summaries.values()], type=pa.int32()),
            "learning_count": pa.array([v["learning_count"] for v in summaries.values()], type=pa.int32()),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write weekly_summary: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} weekly summaries to Gold")
    return len(gold_table)


def aggregate_goal_progress(catalog: Catalog) -> int:
    """ゴール別の週次進捗

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_time = catalog.load_table("silver.time_entries")
        gold = catalog.load_table("gold.goal_progress")
        time_df = silver_time.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load goal_progress tables: {e}") from e

    progress: dict[tuple[str, str, date | None], dict[str, int]] = {}

    for i in range(len(time_df)):
        user_id = time_df.column("user_id")[i].as_py()
        date_val = time_df.column("date")[i].as_py()
        goal_name = time_df.column("goal_name")[i].as_py()
        minutes = time_df.column("duration_minutes")[i].as_py() or 0

        if date_val is None or not goal_name:
            continue

        week_start = _iso_week_start(date_val)
        key = (user_id, goal_name, week_start)
        if key not in progress:
            progress[key] = {"total_minutes": 0, "entry_count": 0}
        progress[key]["total_minutes"] += minutes
        progress[key]["entry_count"] += 1

    if not progress:
        logger.info("No data to aggregate for goal_progress")
        return 0

    try:
        gold_table = pa.table({
            "user_id": pa.array([k[0] for k in progress], type=pa.string()),
            "goal_name": pa.array([k[1] for k in progress], type=pa.string()),
            "week_start": pa.array([k[2] for k in progress], type=pa.date32()),
            "total_minutes": pa.array([v["total_minutes"] for v in progress.values()], type=pa.int64()),
            "entry_count": pa.array([v["entry_count"] for v in progress.values()], type=pa.int32()),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write goal_progress: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} goal progress entries to Gold")
    return len(gold_table)


def aggregate_ai_usage_weekly(catalog: Catalog) -> int:
    """AI使用量の週次集計

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_interactions = catalog.load_table("silver.ai_interactions")
        gold = catalog.load_table("gold.ai_usage_weekly")
        df = silver_interactions.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load ai_usage_weekly tables: {e}") from e

    if len(df) == 0:
        logger.info("No data to aggregate for ai_usage_weekly")
        return 0

    summaries: dict[tuple[str, date | None], dict[str, Any]] = {}

    for i in range(len(df)):
        user_id = df.column("user_id")[i].as_py()
        date_val = df.column("date")[i].as_py()
        situation = df.column("situation")[i].as_py() or "chat"
        tokens_in = df.column("tokens_input")[i].as_py() or 0
        tokens_out = df.column("tokens_output")[i].as_py() or 0
        latency = df.column("latency_ms")[i].as_py() or 0
        tool_names_json = df.column("tool_names_json")[i].as_py()

        if date_val is None:
            continue

        week_start = _iso_week_start(date_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "interaction_count": 0,
                "tokens_input_total": 0,
                "tokens_output_total": 0,
                "tokens_total": 0,
                "latency_sum": 0,
                "situation_dist": {},
                "tool_usage": {},
                "web_search_count": 0,
            }

        s = summaries[key]
        s["interaction_count"] += 1
        s["tokens_input_total"] += tokens_in
        s["tokens_output_total"] += tokens_out
        s["tokens_total"] += tokens_in + tokens_out
        s["latency_sum"] += latency

        # situation distribution
        s["situation_dist"][situation] = s["situation_dist"].get(situation, 0) + 1

        # tool usage
        if tool_names_json:
            try:
                names = json.loads(tool_names_json)
                for name in names:
                    s["tool_usage"][name] = s["tool_usage"].get(name, 0) + 1
                    if name == "web_search":
                        s["web_search_count"] += 1
            except (json.JSONDecodeError, TypeError):
                pass

    if not summaries:
        logger.info("No data to aggregate for ai_usage_weekly")
        return 0

    try:
        gold_table = pa.table({
            "user_id": pa.array([k[0] for k in summaries], type=pa.string()),
            "week_start": pa.array([k[1] for k in summaries], type=pa.date32()),
            "interaction_count": pa.array(
                [v["interaction_count"] for v in summaries.values()], type=pa.int32()
            ),
            "tokens_input_total": pa.array(
                [v["tokens_input_total"] for v in summaries.values()], type=pa.int64()
            ),
            "tokens_output_total": pa.array(
                [v["tokens_output_total"] for v in summaries.values()], type=pa.int64()
            ),
            "tokens_total": pa.array(
                [v["tokens_total"] for v in summaries.values()], type=pa.int64()
            ),
            "avg_latency_ms": pa.array(
                [
                    v["latency_sum"] // v["interaction_count"] if v["interaction_count"] > 0 else 0
                    for v in summaries.values()
                ],
                type=pa.int32(),
            ),
            "situation_distribution_json": pa.array(
                [json.dumps(v["situation_dist"], ensure_ascii=False) for v in summaries.values()],
                type=pa.string(),
            ),
            "tool_usage_json": pa.array(
                [json.dumps(v["tool_usage"], ensure_ascii=False) for v in summaries.values()],
                type=pa.string(),
            ),
            "web_search_count": pa.array(
                [v["web_search_count"] for v in summaries.values()], type=pa.int32()
            ),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write ai_usage_weekly: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} AI usage weekly records to Gold")
    return len(gold_table)


def aggregate_ai_quality_weekly(catalog: Catalog) -> int:
    """AI品質の週次集計

    Silver層のai_interactions, ai_facts, ai_reviewsから集計。
    Medallionパターンに準拠（Gold → Silver のみ）。

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_interactions = catalog.load_table("silver.ai_interactions")
        silver_facts = catalog.load_table("silver.ai_facts")
        silver_reviews = catalog.load_table("silver.ai_reviews")
        gold = catalog.load_table("gold.ai_quality_weekly")

        interactions_df = silver_interactions.scan().to_arrow()
        facts_df = silver_facts.scan().to_arrow()
        reviews_df = silver_reviews.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load ai_quality_weekly tables: {e}") from e

    summaries: dict[tuple[str, date | None], dict[str, Any]] = {}

    # Rating集計 (silver.ai_interactions)
    for i in range(len(interactions_df)):
        user_id = interactions_df.column("user_id")[i].as_py()
        date_val = interactions_df.column("date")[i].as_py()
        rating = interactions_df.column("rating")[i].as_py()

        if date_val is None:
            continue

        week_start = _iso_week_start(date_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "rated_count": 0,
                "rating_sum": 0,
                "fact_count": 0,
                "review_generated": False,
            }

        if rating is not None:
            summaries[key]["rated_count"] += 1
            summaries[key]["rating_sum"] += rating

    # Fact集計 (silver.ai_facts)
    for i in range(len(facts_df)):
        user_id = facts_df.column("user_id")[i].as_py()
        date_val = facts_df.column("date")[i].as_py()

        if date_val is None:
            continue

        week_start = _iso_week_start(date_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "rated_count": 0,
                "rating_sum": 0,
                "fact_count": 0,
                "review_generated": False,
            }
        summaries[key]["fact_count"] += 1

    # Review集計 (silver.ai_reviews)
    for i in range(len(reviews_df)):
        user_id = reviews_df.column("user_id")[i].as_py()
        week_start_val = reviews_df.column("week_start")[i].as_py()

        if week_start_val is None:
            continue

        week_start = _iso_week_start(week_start_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "rated_count": 0,
                "rating_sum": 0,
                "fact_count": 0,
                "review_generated": False,
            }
        summaries[key]["review_generated"] = True

    if not summaries:
        logger.info("No data to aggregate for ai_quality_weekly")
        return 0

    try:
        gold_table = pa.table({
            "user_id": pa.array([k[0] for k in summaries], type=pa.string()),
            "week_start": pa.array([k[1] for k in summaries], type=pa.date32()),
            "rated_count": pa.array(
                [v["rated_count"] for v in summaries.values()], type=pa.int32()
            ),
            "avg_rating": pa.array(
                [
                    v["rating_sum"] / v["rated_count"] if v["rated_count"] > 0 else None
                    for v in summaries.values()
                ],
                type=pa.float32(),
            ),
            "fact_count": pa.array(
                [v["fact_count"] for v in summaries.values()], type=pa.int32()
            ),
            "review_generated": pa.array(
                [v["review_generated"] for v in summaries.values()], type=pa.bool_()
            ),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write ai_quality_weekly: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} AI quality weekly records to Gold")
    return len(gold_table)


def aggregate_interest_profile(catalog: Catalog) -> int:
    """ユーザー関心プロファイル集約

    silver.tag_usage_profile を user_id でグループ化し、
    top_tags, emerging_tags, fading_tags, クラスタを算出。

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_tags = catalog.load_table("silver.tag_usage_profile")
        gold = catalog.load_table("gold.user_interest_profile")
        df = silver_tags.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load interest_profile tables: {e}") from e

    if len(df) == 0:
        logger.info("No data to aggregate for interest_profile")
        return 0

    # user_id ごとに集約
    user_profiles: dict[str, list[dict[str, Any]]] = {}
    for i in range(len(df)):
        user_id = df.column("user_id")[i].as_py()
        tag_name = df.column("tag_name")[i].as_py()
        note_count = df.column("note_count")[i].as_py() or 0
        trend = df.column("monthly_trend")[i].as_py() or "stable"
        co_tags_json = df.column("co_tags_json")[i].as_py()

        if user_id not in user_profiles:
            user_profiles[user_id] = []

        co_tags = []
        if co_tags_json:
            try:
                co_tags = json.loads(co_tags_json)
            except (json.JSONDecodeError, TypeError):
                pass

        user_profiles[user_id].append({
            "tag_name": tag_name,
            "note_count": note_count,
            "trend": trend,
            "co_tags": co_tags,
        })

    if not user_profiles:
        logger.info("No user profiles to aggregate")
        return 0

    today = date.today()
    week_start = _iso_week_start(today)

    try:
        user_ids = []
        week_starts = []
        top_tags_list = []
        emerging_list = []
        fading_list = []
        clusters_list = []
        total_notes_list = []

        for user_id, tags in user_profiles.items():
            # Sort by note_count descending
            sorted_tags = sorted(tags, key=lambda x: x["note_count"], reverse=True)

            # Top 10 tags
            top_tags = [
                {"name": t["tag_name"], "count": t["note_count"], "trend": t["trend"]}
                for t in sorted_tags[:10]
            ]

            # Emerging: growing trend
            emerging = [
                {"name": t["tag_name"], "count": t["note_count"]}
                for t in sorted_tags if t["trend"] == "growing"
            ]

            # Fading: fading trend
            fading = [
                {"name": t["tag_name"], "count": t["note_count"]}
                for t in sorted_tags if t["trend"] == "fading"
            ]

            # Clusters: group tags that frequently co-occur
            # Simple approach: for each top tag, list its top co-tags
            clusters = []
            seen = set()
            for t in sorted_tags[:5]:
                if t["tag_name"] in seen:
                    continue
                cluster_tags = [t["tag_name"]]
                seen.add(t["tag_name"])
                for co in t["co_tags"][:3]:
                    co_name = co.get("tag") or co.get("name", "")
                    if co_name and co_name not in seen:
                        cluster_tags.append(co_name)
                        seen.add(co_name)
                if len(cluster_tags) > 1:
                    clusters.append(cluster_tags)

            total_notes = sum(t["note_count"] for t in tags)

            user_ids.append(user_id)
            week_starts.append(week_start)
            top_tags_list.append(json.dumps(top_tags, ensure_ascii=False))
            emerging_list.append(json.dumps(emerging, ensure_ascii=False))
            fading_list.append(json.dumps(fading, ensure_ascii=False))
            clusters_list.append(json.dumps(clusters, ensure_ascii=False))
            total_notes_list.append(total_notes)

        gold_table = pa.table({
            "user_id": pa.array(user_ids, type=pa.string()),
            "week_start": pa.array(week_starts, type=pa.date32()),
            "top_tags_json": pa.array(top_tags_list, type=pa.string()),
            "emerging_tags_json": pa.array(emerging_list, type=pa.string()),
            "fading_tags_json": pa.array(fading_list, type=pa.string()),
            "tag_clusters_json": pa.array(clusters_list, type=pa.string()),
            "total_tagged_notes": pa.array(total_notes_list, type=pa.int32()),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write interest_profile: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} user interest profiles to Gold")
    return len(gold_table)


def aggregate_trait_profile(catalog: Catalog) -> int:
    """ユーザー性格プロファイル集約

    silver.user_trait_segments を user_id でグループ化し、
    confidence 加重・最新優先で work_style, learning_style 等を決定。

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_traits = catalog.load_table("silver.user_trait_segments")
        gold = catalog.load_table("gold.user_trait_profile")
        df = silver_traits.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load trait_profile tables: {e}") from e

    if len(df) == 0:
        logger.info("No data to aggregate for trait_profile")
        return 0

    from datetime import datetime as dt, timezone as tz

    # user_id ごとにトレイトを収集
    user_traits: dict[str, list[dict[str, Any]]] = {}
    for i in range(len(df)):
        user_id = df.column("user_id")[i].as_py()
        category = df.column("trait_category")[i].as_py()
        value = df.column("trait_value")[i].as_py()
        confidence = df.column("confidence")[i].as_py() or 0.5
        extracted_at = df.column("extracted_at")[i].as_py()

        user_traits.setdefault(user_id, []).append({
            "category": category,
            "value": value,
            "confidence": confidence,
            "extracted_at": extracted_at,
        })

    if not user_traits:
        return 0

    now = dt.now(tz.utc)

    def _best_value(traits: list[dict], category: str) -> str | None:
        """指定カテゴリの最も確信度が高い値を取得（最新優先）"""
        matching = [t for t in traits if t["category"] == category]
        if not matching:
            return None
        # confidence * recency で最良を選択
        matching.sort(key=lambda t: (t["confidence"], t["extracted_at"] or now), reverse=True)
        return matching[0]["value"]

    def _collect_values(traits: list[dict], category: str) -> list[str]:
        """指定カテゴリの全値をconfidence順で収集"""
        matching = [t for t in traits if t["category"] == category]
        matching.sort(key=lambda t: t["confidence"], reverse=True)
        return [t["value"] for t in matching[:5]]

    try:
        user_ids = []
        updated_ats = []
        work_styles = []
        learning_styles = []
        collaborations = []
        strengths_list = []
        challenges_list = []
        triggers_list = []
        trait_counts = []
        avg_confidences = []

        for user_id, traits in user_traits.items():
            user_ids.append(user_id)
            updated_ats.append(now)
            work_styles.append(_best_value(traits, "work_style"))
            learning_styles.append(_best_value(traits, "learning_style"))
            collaborations.append(_best_value(traits, "collaboration"))
            strengths_list.append(json.dumps(_collect_values(traits, "strengths"), ensure_ascii=False))
            challenges_list.append(json.dumps(_collect_values(traits, "challenges"), ensure_ascii=False))
            triggers_list.append(json.dumps(_collect_values(traits, "triggers"), ensure_ascii=False))
            trait_counts.append(len(traits))
            avg_conf = sum(t["confidence"] for t in traits) / len(traits) if traits else 0.0
            avg_confidences.append(avg_conf)

        gold_table = pa.table({
            "user_id": pa.array(user_ids, type=pa.string()),
            "updated_at": pa.array(updated_ats, type=pa.timestamp("us", tz="UTC")),
            "work_style": pa.array(work_styles, type=pa.string()),
            "learning_style": pa.array(learning_styles, type=pa.string()),
            "collaboration": pa.array(collaborations, type=pa.string()),
            "strengths_json": pa.array(strengths_list, type=pa.string()),
            "challenges_json": pa.array(challenges_list, type=pa.string()),
            "triggers_json": pa.array(triggers_list, type=pa.string()),
            "trait_count": pa.array(trait_counts, type=pa.int32()),
            "avg_confidence": pa.array(avg_confidences, type=pa.float32()),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write trait_profile: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} user trait profiles to Gold")
    return len(gold_table)


def aggregate_emotion_weekly(catalog: Catalog) -> int:
    """感情データの週次集計

    silver.emotion_segments を (user_id, week_start) でグループ化し、
    平均valence/energy/stress、最頻感情、タスク相関、トレンドを算出。

    Returns:
        集計した行数

    Raises:
        AggregationError: 集計処理に失敗した場合
    """
    try:
        silver_emotion = catalog.load_table("silver.emotion_segments")
        gold = catalog.load_table("gold.emotion_weekly")
        df = silver_emotion.scan().to_arrow()
    except Exception as e:
        raise AggregationError(f"Failed to load emotion_weekly tables: {e}") from e

    if len(df) == 0:
        logger.info("No data to aggregate for emotion_weekly")
        return 0

    summaries: dict[tuple[str, date | None], dict[str, Any]] = {}

    for i in range(len(df)):
        user_id = df.column("user_id")[i].as_py()
        date_val = df.column("date")[i].as_py()
        valence = df.column("valence")[i].as_py()
        energy = df.column("energy")[i].as_py()
        stress = df.column("stress")[i].as_py()
        emotion = df.column("dominant_emotion")[i].as_py()
        related_tasks_json = df.column("related_tasks_json")[i].as_py()

        if date_val is None:
            continue

        week_start = _iso_week_start(date_val)
        key = (user_id, week_start)
        if key not in summaries:
            summaries[key] = {
                "valence_sum": 0.0,
                "energy_sum": 0.0,
                "stress_sum": 0.0,
                "count": 0,
                "emotion_dist": {},
                "task_valence": {},  # task_name -> {sum, count}
            }

        s = summaries[key]
        s["valence_sum"] += valence or 0.0
        s["energy_sum"] += energy or 0.0
        s["stress_sum"] += stress or 0.0
        s["count"] += 1

        if emotion:
            s["emotion_dist"][emotion] = s["emotion_dist"].get(emotion, 0) + 1

        # タスク相関
        if related_tasks_json:
            try:
                tasks = json.loads(related_tasks_json)
                for t in tasks:
                    tname = t.get("task_name")
                    if tname and valence is not None:
                        if tname not in s["task_valence"]:
                            s["task_valence"][tname] = {"sum": 0.0, "count": 0}
                        s["task_valence"][tname]["sum"] += valence
                        s["task_valence"][tname]["count"] += 1
            except (json.JSONDecodeError, TypeError):
                pass

    if not summaries:
        logger.info("No data to aggregate for emotion_weekly")
        return 0

    # 前週比のトレンド算出用: user_id → [(week_start, avg_valence)]
    user_weekly: dict[str, list[tuple[date, float]]] = {}
    for (user_id, week_start), s in summaries.items():
        if week_start is None:
            continue
        avg_v = s["valence_sum"] / s["count"] if s["count"] > 0 else 0.0
        user_weekly.setdefault(user_id, []).append((week_start, avg_v))

    for uid in user_weekly:
        user_weekly[uid].sort(key=lambda x: x[0])

    def _calc_trend(user_id: str, week_start: date | None) -> str:
        if week_start is None:
            return "stable"
        weeks = user_weekly.get(user_id, [])
        if len(weeks) < 2:
            return "stable"
        # この週のインデックスを探す
        idx = next((i for i, w in enumerate(weeks) if w[0] == week_start), -1)
        if idx <= 0:
            return "stable"
        prev_val = weeks[idx - 1][1]
        curr_val = weeks[idx][1]
        diff = curr_val - prev_val
        if diff > 0.1:
            return "improving"
        elif diff < -0.1:
            return "declining"
        return "stable"

    try:
        keys = list(summaries.keys())
        vals = list(summaries.values())

        gold_table = pa.table({
            "user_id": pa.array([k[0] for k in keys], type=pa.string()),
            "week_start": pa.array([k[1] for k in keys], type=pa.date32()),
            "avg_valence": pa.array(
                [v["valence_sum"] / v["count"] if v["count"] > 0 else None for v in vals],
                type=pa.float32(),
            ),
            "avg_energy": pa.array(
                [v["energy_sum"] / v["count"] if v["count"] > 0 else None for v in vals],
                type=pa.float32(),
            ),
            "avg_stress": pa.array(
                [v["stress_sum"] / v["count"] if v["count"] > 0 else None for v in vals],
                type=pa.float32(),
            ),
            "dominant_emotion": pa.array(
                [
                    max(v["emotion_dist"], key=v["emotion_dist"].get) if v["emotion_dist"] else None
                    for v in vals
                ],
                type=pa.string(),
            ),
            "emotion_distribution_json": pa.array(
                [json.dumps(v["emotion_dist"], ensure_ascii=False) for v in vals],
                type=pa.string(),
            ),
            "diary_count": pa.array([v["count"] for v in vals], type=pa.int32()),
            "task_correlation_json": pa.array(
                [
                    json.dumps(
                        [
                            {
                                "task_name": tn,
                                "avg_valence": round(tv["sum"] / tv["count"], 2),
                                "count": tv["count"],
                            }
                            for tn, tv in v["task_valence"].items()
                        ],
                        ensure_ascii=False,
                    )
                    for v in vals
                ],
                type=pa.string(),
            ),
            "valence_trend": pa.array(
                [_calc_trend(k[0], k[1]) for k in keys],
                type=pa.string(),
            ),
        })
        gold.overwrite(gold_table)
    except Exception as e:
        raise AggregationError(f"Failed to write emotion_weekly: {e}") from e

    logger.info(f"Aggregated {len(gold_table)} emotion weekly records to Gold")
    return len(gold_table)


def main() -> None:
    """メインエントリーポイント"""
    logger.info("Gold aggregation started.")

    try:
        catalog = get_catalog()
    except Exception as e:
        logger.error(f"Failed to initialize catalog: {e}")
        raise SystemExit(1) from e

    errors: list[str] = []
    aggregations = [
        ("weekly_summary", aggregate_weekly_summary),
        ("goal_progress", aggregate_goal_progress),
        ("ai_usage_weekly", aggregate_ai_usage_weekly),
        ("ai_quality_weekly", aggregate_ai_quality_weekly),
        ("interest_profile", aggregate_interest_profile),
        ("trait_profile", aggregate_trait_profile),
        ("emotion_weekly", aggregate_emotion_weekly),
    ]

    for name, func in aggregations:
        try:
            func(catalog)
        except AggregationError as e:
            logger.error(f"Failed to aggregate {name}: {e}")
            errors.append(name)
            continue

    if errors:
        logger.warning(f"Gold aggregation completed with errors: {errors}")
    else:
        logger.info("Gold aggregation complete.")


if __name__ == "__main__":
    main()
