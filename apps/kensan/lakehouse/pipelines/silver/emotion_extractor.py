"""
Silver Emotion Extractor: bronze.notes_raw (diary) → silver.emotion_segments
LLMで日記テキストから感情を抽出し、構造化データとしてIcebergに保存。
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

logger = setup_logging("silver.emotion_extractor")


class EmotionExtractionError(Exception):
    """感情抽出処理の全体エラー"""

    pass


EXTRACTION_PROMPT = """\
以下の日記テキストから感情状態を分析してJSON形式で出力してください。

## 日記情報
- 日付: {date}
- タイトル: {title}

## 本文
{content}

## ユーザーのタスク一覧（言及があればマッチング）
{task_list}

## 出力形式（JSON）
```json
{{
  "time_hint": "morning|afternoon|evening|unknown",
  "valence": 0.0,
  "energy": 0.0,
  "stress": 0.0,
  "dominant_emotion": "感情名",
  "keywords": ["キーワード1", "キーワード2"],
  "related_tasks": [
    {{"task_id": "UUID or null", "task_name": "タスク名"}}
  ],
  "confidence": 0.0
}}
```

## ルール
- valence: -1.0（非常にネガティブ）〜 1.0（非常にポジティブ）
- energy: 0.0（低エネルギー）〜 1.0（高エネルギー）
- stress: 0.0（リラックス）〜 1.0（高ストレス）
- dominant_emotion: joy, satisfaction, focus, calm, frustration, anxiety, fatigue, boredom, neutral等
- keywords: 感情に関連するキーワード（最大5個）
- related_tasks: 日記中で言及されているタスク。タスク一覧にマッチするものはtask_idを含める
- confidence: 分析の確信度 0.0〜1.0（テキストが短い/曖昧なら低く）
- time_hint: テキスト内容から推測される時間帯。朝の話題ならmorning、不明ならunknown
- JSON以外のテキストは出力しない
"""


def _parse_llm_response(response_text: str) -> dict[str, Any] | None:
    """LLM応答からJSONをパースしバリデーション"""
    text = response_text.strip()
    # マークダウンコードブロックの除去
    if text.startswith("```"):
        lines = text.split("\n")
        lines = [l for l in lines if not l.strip().startswith("```")]
        text = "\n".join(lines)

    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        logger.warning(f"Failed to parse LLM response as JSON: {text[:200]}")
        return None

    # バリデーション
    required = {"valence", "energy", "stress", "dominant_emotion", "confidence"}
    if not required.issubset(data.keys()):
        logger.warning(f"Missing required fields: {required - data.keys()}")
        return None

    # 値の範囲クランプ
    data["valence"] = max(-1.0, min(1.0, float(data["valence"])))
    data["energy"] = max(0.0, min(1.0, float(data["energy"])))
    data["stress"] = max(0.0, min(1.0, float(data["stress"])))
    data["confidence"] = max(0.0, min(1.0, float(data["confidence"])))

    # デフォルト値
    data.setdefault("time_hint", "unknown")
    data.setdefault("keywords", [])
    data.setdefault("related_tasks", [])

    valid_hints = {"morning", "afternoon", "evening", "unknown"}
    if data["time_hint"] not in valid_hints:
        data["time_hint"] = "unknown"

    return data


def _build_task_list(tasks_df: pa.Table, user_id: str) -> str:
    """タスクDataFrameからユーザーのタスク一覧文字列を構築"""
    items = []
    for i in range(len(tasks_df)):
        if tasks_df.column("user_id")[i].as_py() != user_id:
            continue
        task_id = tasks_df.column("id")[i].as_py()
        name = tasks_df.column("name")[i].as_py()
        if name:
            items.append(f"- {name} (id: {task_id})")
    return "\n".join(items) if items else "（タスク情報なし）"


def extract_emotions_batch(
    catalog: Catalog,
    genai_client: GenAIClient,
    model: str = "gemini-2.0-flash",
) -> int:
    """日記ノートからバッチで感情を抽出

    Args:
        catalog: Iceberg catalog
        genai_client: Google GenAI client
        model: 使用するモデル名

    Returns:
        処理した件数

    Raises:
        EmotionExtractionError: 全体的な失敗
    """
    try:
        notes_table = catalog.load_table("bronze.notes_raw")
        tasks_table = catalog.load_table("bronze.tasks_raw")
        silver_table = catalog.load_table("silver.emotion_segments")

        notes_df = notes_table.scan().to_arrow()
        tasks_df = tasks_table.scan().to_arrow()
        existing_df = silver_table.scan().to_arrow()
    except Exception as e:
        raise EmotionExtractionError(f"Failed to load tables: {e}") from e

    # 既存のnote_id → extracted_atマッピング
    existing_map: dict[str, datetime] = {}
    for i in range(len(existing_df)):
        nid = existing_df.column("note_id")[i].as_py()
        ext_at = existing_df.column("extracted_at")[i].as_py()
        if nid and ext_at:
            existing_map[nid] = ext_at

    # 処理対象: type='diary', content非空, 新規 or 更新済み
    targets: list[dict[str, Any]] = []
    for i in range(len(notes_df)):
        note_type = notes_df.column("type")[i].as_py()
        if note_type != "diary":
            continue

        content = notes_df.column("content")[i].as_py()
        if not content or not content.strip():
            continue

        note_id = notes_df.column("id")[i].as_py()
        updated_at = notes_df.column("updated_at")[i].as_py()

        # 差分チェック: 新規 or updated_at > extracted_at
        if note_id in existing_map:
            if updated_at and updated_at <= existing_map[note_id]:
                continue

        targets.append({
            "note_id": note_id,
            "user_id": notes_df.column("user_id")[i].as_py(),
            "date": notes_df.column("date")[i].as_py(),
            "title": notes_df.column("title")[i].as_py() or "",
            "content": content,
            "goal_name": notes_df.column("goal_name")[i].as_py(),
            "task_id": notes_df.column("task_id")[i].as_py(),
        })

    if not targets:
        logger.info("No new diary notes to process")
        return 0

    logger.info(f"Processing {len(targets)} diary notes for emotion extraction")

    # LLM抽出
    results: list[dict[str, Any]] = []
    now = datetime.now(timezone.utc)

    for note in targets:
        try:
            task_list = _build_task_list(tasks_df, note["user_id"])
            prompt = EXTRACTION_PROMPT.format(
                date=note["date"] or "不明",
                title=note["title"],
                content=note["content"][:3000],  # 長すぎるテキストを切り詰め
                task_list=task_list,
            )

            response = genai_client.models.generate_content(
                model=model,
                contents=prompt,
            )

            parsed = _parse_llm_response(response.text)
            if parsed is None:
                logger.warning(f"Skipping note {note['note_id']}: failed to parse LLM response")
                continue

            results.append({
                "note_id": note["note_id"],
                "user_id": note["user_id"],
                "date": note["date"],
                "time_hint": parsed["time_hint"],
                "valence": parsed["valence"],
                "energy": parsed["energy"],
                "stress": parsed["stress"],
                "dominant_emotion": parsed["dominant_emotion"],
                "keywords_json": json.dumps(parsed["keywords"], ensure_ascii=False),
                "related_tasks_json": json.dumps(parsed["related_tasks"], ensure_ascii=False),
                "confidence": parsed["confidence"],
                "extracted_at": now,
            })
        except Exception as e:
            logger.warning(f"Failed to extract emotion for note {note['note_id']}: {e}")
            continue

    if not results:
        logger.info("No successful extractions")
        return 0

    # 既存レコード(更新対象外)と新規結果をマージしてoverwrite
    new_note_ids = {r["note_id"] for r in results}
    kept: list[dict[str, Any]] = []
    for i in range(len(existing_df)):
        nid = existing_df.column("note_id")[i].as_py()
        if nid not in new_note_ids:
            kept.append({
                "note_id": nid,
                "user_id": existing_df.column("user_id")[i].as_py(),
                "date": existing_df.column("date")[i].as_py(),
                "time_hint": existing_df.column("time_hint")[i].as_py(),
                "valence": existing_df.column("valence")[i].as_py(),
                "energy": existing_df.column("energy")[i].as_py(),
                "stress": existing_df.column("stress")[i].as_py(),
                "dominant_emotion": existing_df.column("dominant_emotion")[i].as_py(),
                "keywords_json": existing_df.column("keywords_json")[i].as_py(),
                "related_tasks_json": existing_df.column("related_tasks_json")[i].as_py(),
                "confidence": existing_df.column("confidence")[i].as_py(),
                "extracted_at": existing_df.column("extracted_at")[i].as_py(),
            })

    all_records = kept + results

    try:
        arrow_table = pa.table(
            {
                "note_id": pa.array([r["note_id"] for r in all_records], type=pa.string()),
                "user_id": pa.array([r["user_id"] for r in all_records], type=pa.string()),
                "date": pa.array([r["date"] for r in all_records], type=pa.date32()),
                "time_hint": pa.array([r["time_hint"] for r in all_records], type=pa.string()),
                "valence": pa.array([r["valence"] for r in all_records], type=pa.float32()),
                "energy": pa.array([r["energy"] for r in all_records], type=pa.float32()),
                "stress": pa.array([r["stress"] for r in all_records], type=pa.float32()),
                "dominant_emotion": pa.array([r["dominant_emotion"] for r in all_records], type=pa.string()),
                "keywords_json": pa.array([r["keywords_json"] for r in all_records], type=pa.string()),
                "related_tasks_json": pa.array([r["related_tasks_json"] for r in all_records], type=pa.string()),
                "confidence": pa.array([r["confidence"] for r in all_records], type=pa.float32()),
                "extracted_at": pa.array([r["extracted_at"] for r in all_records], type=pa.timestamp("us", tz="UTC")),
            }
        )
        silver_table.overwrite(arrow_table)
    except Exception as e:
        raise EmotionExtractionError(f"Failed to write emotion_segments: {e}") from e

    logger.info(f"Extracted emotions for {len(results)} notes ({len(all_records)} total records)")
    return len(results)
