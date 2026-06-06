"""
Bronze Ingestion: Loki → Iceberg Bronze層
Loki query_range API から AI agent イベントを取得し append する。

- OTel エンベロープをパース
- agent.* イベントのみフィルタ
- .ingestion_state.json の bronze.ai_explorer_events_raw キーで状態管理
"""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

import pyarrow as pa
import requests

from catalog.config import setup_logging

logger = setup_logging("bronze.ingest_loki")

STATE_FILE = Path(__file__).parent.parent.parent / ".ingestion_state.json"

AGENT_EVENT_TYPES = {
    "agent.prompt",
    "agent.turn",
    "agent.tool_call",
    "agent.complete",
    "agent.system_prompt",
}

TABLE_NAME = "bronze.ai_explorer_events_raw"

ARROW_SCHEMA = pa.schema([
    ("trace_id", pa.string()),
    ("event_type", pa.string()),
    ("user_id", pa.string()),
    ("conversation_id", pa.string()),
    ("timestamp", pa.timestamp("us", tz="UTC")),
    ("payload", pa.string()),
    ("_ingested_at", pa.timestamp("us", tz="UTC")),
])


def load_state() -> dict[str, str]:
    """ステートファイルから前回取り込み時刻を読み込み"""
    if STATE_FILE.exists():
        try:
            return json.loads(STATE_FILE.read_text())
        except json.JSONDecodeError:
            return {}
    return {}


def save_state(state: dict[str, str]) -> None:
    """ステートファイルに取り込み時刻を保存"""
    STATE_FILE.write_text(json.dumps(state, indent=2, default=str))


def query_loki(
    base_url: str,
    start_ns: str,
    end_ns: str,
    limit: int = 5000,
) -> list[dict[str, Any]]:
    """Loki query_range API でログ取得.

    Returns:
        パース済みイベントのリスト
    """
    params = {
        "query": '{job="kensan-ai"}',
        "start": start_ns,
        "end": end_ns,
        "limit": str(limit),
        "direction": "forward",
    }

    resp = requests.get(f"{base_url}/loki/api/v1/query_range", params=params)
    resp.raise_for_status()

    data = resp.json()
    streams = data.get("data", {}).get("result", [])

    events: list[dict[str, Any]] = []
    for stream in streams:
        for nano_ts, line in stream.get("values", []):
            parsed = parse_otel_log(nano_ts, line)
            if parsed and parsed["event_type"] in AGENT_EVENT_TYPES:
                events.append(parsed)

    return events


def parse_otel_log(nano_ts: str, line: str) -> dict[str, Any] | None:
    """OTel エンベロープ形式のログ行をパース."""
    try:
        envelope = json.loads(line)
        body_str = envelope.get("body", "{}")
        body = json.loads(body_str)

        event_type = body.get("event", "")
        trace_id = envelope.get("traceid", "")
        timestamp_us = int(nano_ts) // 1000  # nano → micro

        # user_id は prompt/complete にのみ存在
        user_id = body.get("user_id", "")
        conversation_id = body.get("conversation_id", "")

        return {
            "trace_id": trace_id,
            "event_type": event_type,
            "user_id": user_id or "",
            "conversation_id": conversation_id or "",
            "timestamp_ns": nano_ts,
            "timestamp": datetime.fromtimestamp(
                int(nano_ts) / 1_000_000_000, tz=timezone.utc
            ),
            "payload": body_str,
        }
    except (json.JSONDecodeError, ValueError, KeyError):
        return None


def ingest_loki_events(
    catalog: Any,
    loki_base_url: str,
    state: dict[str, str],
) -> dict[str, str]:
    """Loki からイベントを取得して Bronze テーブルに append.

    Returns:
        更新後の state dict
    """
    # 前回の最終 timestamp (ナノ秒)
    # 初回 (state なし) は直近24時間から取得 (Loki は start=0 を拒否する)
    default_start_ns = str(
        int((datetime.now(timezone.utc) - timedelta(hours=24)).timestamp() * 1_000_000_000)
    )
    last_ns = state.get(TABLE_NAME) or default_start_ns
    now_ns = str(int(datetime.now(timezone.utc).timestamp() * 1_000_000_000))

    logger.info(f"Querying Loki from {last_ns} to {now_ns}")
    events = query_loki(loki_base_url, last_ns, now_ns)

    if not events:
        logger.info("No new Loki events")
        return state

    now = datetime.now(timezone.utc)
    table = catalog.load_table(TABLE_NAME)

    arrow_table = pa.table(
        {
            "trace_id": [e["trace_id"] for e in events],
            "event_type": [e["event_type"] for e in events],
            "user_id": [e["user_id"] for e in events],
            "conversation_id": [e["conversation_id"] for e in events],
            "timestamp": [e["timestamp"] for e in events],
            "payload": [e["payload"] for e in events],
            "_ingested_at": [now] * len(events),
        },
        schema=ARROW_SCHEMA,
    )

    table.append(arrow_table)

    # 最後のイベントの timestamp_ns を記録
    max_ns = max(e["timestamp_ns"] for e in events)
    state[TABLE_NAME] = max_ns
    logger.info(f"Ingested {len(events)} Loki events into {TABLE_NAME}")

    return state
