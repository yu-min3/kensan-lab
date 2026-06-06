"""
Silver Trait Extractor: bronze.notes_raw + bronze.tags_raw + bronze.note_tags_raw
→ silver.user_trait_segments

LLMで日記・traitタグ付きノートからユーザー性質を抽出。
emotion_extractor.py と同じ差分処理パターン。
"""

from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Any

import pyarrow as pa

from catalog.config import setup_logging

if TYPE_CHECKING:
    from google.genai import Client as GenAIClient
    from pyiceberg.catalog import Catalog

logger = setup_logging("silver.trait_extractor")


class TraitExtractionError(Exception):
    """性質抽出処理の全体エラー"""

    pass


EXTRACTION_PROMPT = """\
以下のノートからユーザーの性格特性・行動傾向を分析してJSON配列で出力してください。

## ノート情報
- 日付: {date}
- タイトル: {title}
- タグ: {tags}

## 本文
{content}

## 出力形式（JSON配列）
```json
[
  {{
    "trait_category": "work_style|learning_style|collaboration|strengths|challenges|triggers",
    "trait_value": "具体的な特性の説明",
    "evidence": "ノートから読み取れる根拠（引用や要約）",
    "confidence": 0.0
  }}
]
```

## ルール
- trait_category は以下のいずれか:
  - work_style: 仕事の進め方、集中パターン、時間管理の傾向
  - learning_style: 学習方法の好み、情報収集の傾向
  - collaboration: 他者との関わり方、コミュニケーションスタイル
  - strengths: 強み、得意なこと
  - challenges: 課題、苦手なこと
  - triggers: モチベーションや感情のトリガー
- trait_value: 具体的で簡潔な記述（1-2文）
- evidence: ノート中の根拠（1-2文）
- confidence: 0.0〜1.0（明確な記述なら高く、推測なら低く）
- 特性が読み取れない場合は空配列 [] を返す
- JSON以外のテキストは出力しない
- 最大5つまで
"""


def _parse_llm_response(response_text: str) -> list[dict[str, Any]]:
    """LLM応答からJSON配列をパースしバリデーション"""
    text = response_text.strip()
    if text.startswith("```"):
        lines = text.split("\n")
        lines = [line for line in lines if not line.strip().startswith("```")]
        text = "\n".join(lines)

    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        logger.warning(f"Failed to parse LLM response as JSON: {text[:200]}")
        return []

    if not isinstance(data, list):
        logger.warning("LLM response is not a JSON array")
        return []

    valid_categories = {
        "work_style", "learning_style", "collaboration",
        "strengths", "challenges", "triggers",
    }

    results = []
    for item in data:
        if not isinstance(item, dict):
            continue
        category = item.get("trait_category", "")
        if category not in valid_categories:
            continue
        results.append({
            "trait_category": category,
            "trait_value": str(item.get("trait_value", ""))[:500],
            "evidence": str(item.get("evidence", ""))[:500],
            "confidence": max(0.0, min(1.0, float(item.get("confidence", 0.5)))),
        })

    return results[:5]


def extract_traits_batch(
    catalog: Catalog,
    genai_client: GenAIClient,
    model: str = "gemini-2.0-flash",
) -> int:
    """ノートからバッチでユーザー性質を抽出

    Args:
        catalog: Iceberg catalog
        genai_client: Google GenAI client
        model: 使用するモデル名

    Returns:
        処理した件数

    Raises:
        TraitExtractionError: 全体的な失敗
    """
    try:
        notes_table = catalog.load_table("bronze.notes_raw")
        tags_table = catalog.load_table("bronze.tags_raw")
        note_tags_table = catalog.load_table("bronze.note_tags_raw")
        silver_table = catalog.load_table("silver.user_trait_segments")

        notes_df = notes_table.scan().to_arrow()
        tags_df = tags_table.scan().to_arrow()
        note_tags_df = note_tags_table.scan().to_arrow()
        existing_df = silver_table.scan().to_arrow()
    except Exception as e:
        raise TraitExtractionError(f"Failed to load tables: {e}") from e

    # Build tag lookup: tag_id -> {name, category, user_id}
    tag_info: dict[str, dict] = {}
    for i in range(len(tags_df)):
        tid = tags_df.column("id")[i].as_py()
        tag_info[tid] = {
            "name": tags_df.column("name")[i].as_py(),
            "category": tags_df.column("category")[i].as_py() if "category" in tags_df.column_names else "general",
            "user_id": tags_df.column("user_id")[i].as_py(),
        }

    # Build note_id -> [tag_names] mapping
    note_tag_names: dict[str, list[str]] = {}
    trait_tagged_notes: set[str] = set()
    for i in range(len(note_tags_df)):
        nid = note_tags_df.column("note_id")[i].as_py()
        tid = note_tags_df.column("tag_id")[i].as_py()
        tag = tag_info.get(tid)
        if tag:
            note_tag_names.setdefault(nid, []).append(tag["name"])
            if tag["category"] == "trait":
                trait_tagged_notes.add(nid)

    # Existing note_id -> extracted_at
    existing_map: dict[str, datetime] = {}
    for i in range(len(existing_df)):
        nid = existing_df.column("note_id")[i].as_py()
        ext_at = existing_df.column("extracted_at")[i].as_py()
        if nid and ext_at:
            existing_map[nid] = ext_at

    # Targets: diary notes OR trait-tagged notes
    targets: list[dict[str, Any]] = []
    for i in range(len(notes_df)):
        note_id = notes_df.column("id")[i].as_py()
        note_type = notes_df.column("type")[i].as_py()
        content = notes_df.column("content")[i].as_py()

        if not content or not content.strip():
            continue

        # Include: diary notes or notes with trait tags
        is_diary = note_type == "diary"
        has_trait_tag = note_id in trait_tagged_notes
        if not is_diary and not has_trait_tag:
            continue

        updated_at = notes_df.column("updated_at")[i].as_py()

        # Diff check
        if note_id in existing_map:
            if updated_at and updated_at <= existing_map[note_id]:
                continue

        targets.append({
            "note_id": note_id,
            "user_id": notes_df.column("user_id")[i].as_py(),
            "date": notes_df.column("date")[i].as_py(),
            "title": notes_df.column("title")[i].as_py() or "",
            "content": content,
            "tags": note_tag_names.get(note_id, []),
        })

    if not targets:
        logger.info("No new notes to process for trait extraction")
        return 0

    logger.info(f"Processing {len(targets)} notes for trait extraction")

    results: list[dict[str, Any]] = []
    now = datetime.now(timezone.utc)

    for note in targets:
        try:
            tags_str = ", ".join(note["tags"]) if note["tags"] else "（タグなし）"
            prompt = EXTRACTION_PROMPT.format(
                date=note["date"] or "不明",
                title=note["title"],
                tags=tags_str,
                content=note["content"][:3000],
            )

            response = genai_client.models.generate_content(
                model=model,
                contents=prompt,
            )

            parsed = _parse_llm_response(response.text)
            if not parsed:
                continue

            for trait in parsed:
                results.append({
                    "note_id": note["note_id"],
                    "user_id": note["user_id"],
                    "date": note["date"],
                    "trait_category": trait["trait_category"],
                    "trait_value": trait["trait_value"],
                    "evidence": trait["evidence"],
                    "confidence": trait["confidence"],
                    "source_tags_json": json.dumps(note["tags"], ensure_ascii=False),
                    "extracted_at": now,
                })
        except Exception as e:
            logger.warning(f"Failed to extract traits for note {note['note_id']}: {e}")
            continue

    if not results:
        logger.info("No successful trait extractions")
        return 0

    # Merge with existing (keep non-updated)
    new_note_ids = {r["note_id"] for r in results}
    kept: list[dict[str, Any]] = []
    for i in range(len(existing_df)):
        nid = existing_df.column("note_id")[i].as_py()
        if nid not in new_note_ids:
            kept.append({
                "note_id": nid,
                "user_id": existing_df.column("user_id")[i].as_py(),
                "date": existing_df.column("date")[i].as_py(),
                "trait_category": existing_df.column("trait_category")[i].as_py(),
                "trait_value": existing_df.column("trait_value")[i].as_py(),
                "evidence": existing_df.column("evidence")[i].as_py(),
                "confidence": existing_df.column("confidence")[i].as_py(),
                "source_tags_json": existing_df.column("source_tags_json")[i].as_py(),
                "extracted_at": existing_df.column("extracted_at")[i].as_py(),
            })

    all_records = kept + results

    try:
        arrow_table = pa.table({
            "note_id": pa.array([r["note_id"] for r in all_records], type=pa.string()),
            "user_id": pa.array([r["user_id"] for r in all_records], type=pa.string()),
            "date": pa.array([r["date"] for r in all_records], type=pa.date32()),
            "trait_category": pa.array([r["trait_category"] for r in all_records], type=pa.string()),
            "trait_value": pa.array([r["trait_value"] for r in all_records], type=pa.string()),
            "evidence": pa.array([r["evidence"] for r in all_records], type=pa.string()),
            "confidence": pa.array([r["confidence"] for r in all_records], type=pa.float32()),
            "source_tags_json": pa.array([r["source_tags_json"] for r in all_records], type=pa.string()),
            "extracted_at": pa.array([r["extracted_at"] for r in all_records], type=pa.timestamp("us", tz="UTC")),
        })
        silver_table.overwrite(arrow_table)
    except Exception as e:
        raise TraitExtractionError(f"Failed to write user_trait_segments: {e}") from e

    logger.info(f"Extracted {len(results)} traits from {len(targets)} notes ({len(all_records)} total)")
    return len(results)
