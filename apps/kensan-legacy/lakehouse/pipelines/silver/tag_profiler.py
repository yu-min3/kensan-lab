"""
Silver Tag Profiler: bronze.notes_raw + bronze.tags_raw + bronze.note_tags_raw
→ silver.tag_usage_profile

タグ使用統計を集計: note_count, note_types分布, 共起タグ, monthly_trend
"""

from __future__ import annotations

import json
from collections import defaultdict
from datetime import date, timedelta
from typing import TYPE_CHECKING

import pyarrow as pa

from catalog.config import setup_logging

if TYPE_CHECKING:
    from pyiceberg.catalog import Catalog

logger = setup_logging("silver.tag_profiler")


class TagProfileError(Exception):
    """タグプロファイル処理のエラー"""

    pass


def build_tag_usage_profile(catalog: Catalog) -> int:
    """タグ使用統計を集計して silver.tag_usage_profile に書き込む

    Args:
        catalog: Iceberg catalog

    Returns:
        集計した行数

    Raises:
        TagProfileError: 処理に失敗した場合
    """
    try:
        notes_table = catalog.load_table("bronze.notes_raw")
        tags_table = catalog.load_table("bronze.tags_raw")
        note_tags_table = catalog.load_table("bronze.note_tags_raw")
        silver_table = catalog.load_table("silver.tag_usage_profile")

        notes_df = notes_table.scan().to_arrow()
        tags_df = tags_table.scan().to_arrow()
        note_tags_df = note_tags_table.scan().to_arrow()
    except Exception as e:
        raise TagProfileError(f"Failed to load tables: {e}") from e

    if len(tags_df) == 0 or len(note_tags_df) == 0:
        logger.info("No tags or note_tags data to process")
        return 0

    # Build lookup maps
    # note_id -> {type, date, user_id}
    note_info: dict[str, dict] = {}
    for i in range(len(notes_df)):
        nid = notes_df.column("id")[i].as_py()
        note_info[nid] = {
            "type": notes_df.column("type")[i].as_py(),
            "date": notes_df.column("date")[i].as_py(),
            "user_id": notes_df.column("user_id")[i].as_py(),
        }

    # tag_id -> {name, user_id}
    tag_info: dict[str, dict] = {}
    for i in range(len(tags_df)):
        tid = tags_df.column("id")[i].as_py()
        tag_info[tid] = {
            "name": tags_df.column("name")[i].as_py(),
            "user_id": tags_df.column("user_id")[i].as_py(),
        }

    # note_id -> [tag_id] mapping
    note_to_tags: dict[str, list[str]] = defaultdict(list)
    for i in range(len(note_tags_df)):
        nid = note_tags_df.column("note_id")[i].as_py()
        tid = note_tags_df.column("tag_id")[i].as_py()
        note_to_tags[nid].append(tid)

    # Aggregate per (user_id, tag_id)
    ProfileKey = tuple[str, str]  # (user_id, tag_id)
    profiles: dict[ProfileKey, dict] = {}

    today = date.today()
    three_months_ago = today - timedelta(days=90)
    six_months_ago = today - timedelta(days=180)

    for note_id, tag_ids in note_to_tags.items():
        note = note_info.get(note_id)
        if not note:
            continue

        note_date = note["date"]
        note_type = note["type"]

        for tid in tag_ids:
            tag = tag_info.get(tid)
            if not tag:
                continue

            key: ProfileKey = (tag["user_id"], tid)
            if key not in profiles:
                profiles[key] = {
                    "tag_name": tag["name"],
                    "note_count": 0,
                    "note_types": defaultdict(int),
                    "first_used": note_date,
                    "last_used": note_date,
                    "co_tags": defaultdict(int),
                    "recent_count": 0,  # last 3 months
                    "older_count": 0,  # 3-6 months ago
                }

            p = profiles[key]
            p["note_count"] += 1
            if note_type:
                p["note_types"][note_type] += 1

            if note_date:
                if p["first_used"] is None or note_date < p["first_used"]:
                    p["first_used"] = note_date
                if p["last_used"] is None or note_date > p["last_used"]:
                    p["last_used"] = note_date

                if note_date >= three_months_ago:
                    p["recent_count"] += 1
                elif note_date >= six_months_ago:
                    p["older_count"] += 1

            # Co-occurring tags (other tags on the same note)
            for other_tid in tag_ids:
                if other_tid != tid:
                    other_tag = tag_info.get(other_tid)
                    if other_tag:
                        p["co_tags"][other_tag["name"]] += 1

    if not profiles:
        logger.info("No tag usage data to aggregate")
        return 0

    # Determine monthly_trend
    def calc_trend(recent: int, older: int) -> str:
        if older == 0 and recent > 0:
            return "growing"
        if recent == 0 and older > 0:
            return "fading"
        if older == 0 and recent == 0:
            return "stable"
        ratio = recent / older
        if ratio > 1.5:
            return "growing"
        elif ratio < 0.5:
            return "fading"
        return "stable"

    # Build Arrow table
    try:
        keys = list(profiles.keys())
        vals = list(profiles.values())

        # Top 10 co-tags per profile
        co_tags_arrays = []
        for v in vals:
            top_co = sorted(v["co_tags"].items(), key=lambda x: x[1], reverse=True)[:10]
            co_tags_arrays.append(
                json.dumps([{"tag": t, "count": c} for t, c in top_co], ensure_ascii=False)
            )

        arrow_table = pa.table({
            "user_id": pa.array([k[0] for k in keys], type=pa.string()),
            "tag_id": pa.array([k[1] for k in keys], type=pa.string()),
            "tag_name": pa.array([v["tag_name"] for v in vals], type=pa.string()),
            "note_count": pa.array([v["note_count"] for v in vals], type=pa.int32()),
            "note_types_json": pa.array(
                [json.dumps(dict(v["note_types"]), ensure_ascii=False) for v in vals],
                type=pa.string(),
            ),
            "first_used": pa.array([v["first_used"] for v in vals], type=pa.date32()),
            "last_used": pa.array([v["last_used"] for v in vals], type=pa.date32()),
            "co_tags_json": pa.array(co_tags_arrays, type=pa.string()),
            "monthly_trend": pa.array(
                [calc_trend(v["recent_count"], v["older_count"]) for v in vals],
                type=pa.string(),
            ),
        })
        silver_table.overwrite(arrow_table)
    except Exception as e:
        raise TagProfileError(f"Failed to write tag_usage_profile: {e}") from e

    logger.info(f"Built tag usage profiles for {len(arrow_table)} (user, tag) pairs")
    return len(arrow_table)
