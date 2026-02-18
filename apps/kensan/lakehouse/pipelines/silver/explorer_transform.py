"""
Silver Transform: Bronze AI Explorer → Silver AI Explorer
trace_id でグループ化し、インタラクション単位・イベント単位のサマリーを作成。
user_id を prompt イベントから trace 全体に伝播。
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pyarrow as pa

from catalog.config import setup_logging

if TYPE_CHECKING:
    from pyiceberg.catalog import Catalog

logger = setup_logging("silver.explorer_transform")


def transform_explorer_interactions(catalog: Catalog) -> int:
    """Bronze events → Silver interactions (trace_id でグループ化).

    Returns:
        変換したインタラクション数
    """
    bronze = catalog.load_table("bronze.ai_explorer_events_raw")
    silver = catalog.load_table("silver.ai_explorer_interactions")

    df = bronze.scan().to_arrow()
    if len(df) == 0:
        logger.info("No data in bronze.ai_explorer_events_raw")
        return 0

    # Group by trace_id
    rows = df.to_pydict()
    traces: dict[str, list[dict[str, Any]]] = {}
    for i in range(df.num_rows):
        trace_id = rows["trace_id"][i]
        event = {
            "event_type": rows["event_type"][i],
            "user_id": rows["user_id"][i],
            "conversation_id": rows["conversation_id"][i],
            "timestamp": rows["timestamp"][i],
            "payload": rows["payload"][i],
        }
        traces.setdefault(trace_id, []).append(event)

    interactions: list[dict[str, Any]] = []
    for trace_id, events in traces.items():
        # Sort by timestamp
        events.sort(key=lambda e: e["timestamp"])

        # Find prompt and complete events
        prompt_payload: dict[str, Any] = {}
        complete_payload: dict[str, Any] = {}
        user_id = ""

        for ev in events:
            payload = _safe_parse_json(ev["payload"])
            if ev["event_type"] == "agent.prompt":
                prompt_payload = payload
                user_id = ev["user_id"] or payload.get("user_id", "")
            elif ev["event_type"] == "agent.complete":
                complete_payload = payload

        # Propagate user_id from prompt if missing
        if not user_id:
            for ev in events:
                if ev["user_id"]:
                    user_id = ev["user_id"]
                    break

        interactions.append({
            "trace_id": trace_id,
            "user_id": user_id,
            "timestamp": events[0]["timestamp"],
            "outcome": complete_payload.get("outcome", "in_progress"),
            "model": prompt_payload.get("model", ""),
            "total_turns": int(complete_payload.get("total_turns", 0)),
            "total_input_tokens": int(complete_payload.get("total_input_tokens", 0)),
            "total_output_tokens": int(complete_payload.get("total_output_tokens", 0)),
            "pending_action_count": int(complete_payload.get("pending_action_count", 0)),
            "user_message": prompt_payload.get("user_message", ""),
            "context_id": prompt_payload.get("context_id", ""),
            "context_name": prompt_payload.get("context_name", ""),
            "context_version": prompt_payload.get("context_version", ""),
            "experiment_id": prompt_payload.get("experiment_id", ""),
            "system_prompt_length": int(prompt_payload.get("system_prompt_length", 0)),
            "system_prompt_sections_json": json.dumps(
                prompt_payload.get("system_prompt_sections", {}), ensure_ascii=False
            ),
            "tool_count": int(prompt_payload.get("tool_count", 0)),
            "tool_names_json": json.dumps(
                prompt_payload.get("tool_names", []), ensure_ascii=False
            ),
            "tool_definitions_length": int(prompt_payload.get("tool_definitions_length", 0)),
            "event_count": len(events),
        })

    arrow_schema = pa.schema([
        ("trace_id", pa.string()),
        ("user_id", pa.string()),
        ("timestamp", pa.timestamp("us", tz="UTC")),
        ("outcome", pa.string()),
        ("model", pa.string()),
        ("total_turns", pa.int32()),
        ("total_input_tokens", pa.int32()),
        ("total_output_tokens", pa.int32()),
        ("pending_action_count", pa.int32()),
        ("user_message", pa.string()),
        ("context_id", pa.string()),
        ("context_name", pa.string()),
        ("context_version", pa.string()),
        ("experiment_id", pa.string()),
        ("system_prompt_length", pa.int32()),
        ("system_prompt_sections_json", pa.string()),
        ("tool_count", pa.int32()),
        ("tool_names_json", pa.string()),
        ("tool_definitions_length", pa.int32()),
        ("event_count", pa.int32()),
    ])

    columns = {field.name: [r[field.name] for r in interactions] for field in arrow_schema}
    arrow_table = pa.table(columns, schema=arrow_schema)
    silver.overwrite(arrow_table)

    logger.info(f"Transformed {len(interactions)} explorer interactions")
    return len(interactions)


def transform_explorer_events(catalog: Catalog) -> int:
    """Bronze events → Silver events (user_id 伝播 + event_order 付与).

    Returns:
        変換したイベント数
    """
    bronze = catalog.load_table("bronze.ai_explorer_events_raw")
    silver = catalog.load_table("silver.ai_explorer_events")

    df = bronze.scan().to_arrow()
    if len(df) == 0:
        logger.info("No data in bronze.ai_explorer_events_raw")
        return 0

    rows = df.to_pydict()

    # First pass: collect user_id per trace from prompt events
    trace_user_map: dict[str, str] = {}
    for i in range(df.num_rows):
        event_type = rows["event_type"][i]
        user_id = rows["user_id"][i]
        trace_id = rows["trace_id"][i]
        if event_type == "agent.prompt" and user_id:
            trace_user_map[trace_id] = user_id

    # Second pass: build events with propagated user_id + event_order
    trace_order: dict[str, int] = {}
    events_out: list[dict[str, Any]] = []

    # Sort by trace_id then timestamp for ordering
    indices = sorted(
        range(df.num_rows),
        key=lambda i: (rows["trace_id"][i], rows["timestamp"][i]),
    )

    for i in indices:
        trace_id = rows["trace_id"][i]
        order = trace_order.get(trace_id, 0)
        trace_order[trace_id] = order + 1

        user_id = rows["user_id"][i] or trace_user_map.get(trace_id, "")

        events_out.append({
            "trace_id": trace_id,
            "event_type": rows["event_type"][i],
            "user_id": user_id,
            "conversation_id": rows["conversation_id"][i] or "",
            "timestamp": rows["timestamp"][i],
            "payload": rows["payload"][i],
            "event_order": order,
        })

    arrow_schema = pa.schema([
        ("trace_id", pa.string()),
        ("event_type", pa.string()),
        ("user_id", pa.string()),
        ("conversation_id", pa.string()),
        ("timestamp", pa.timestamp("us", tz="UTC")),
        ("payload", pa.string()),
        ("event_order", pa.int32()),
    ])

    columns = {field.name: [e[field.name] for e in events_out] for field in arrow_schema}
    arrow_table = pa.table(columns, schema=arrow_schema)
    silver.overwrite(arrow_table)

    logger.info(f"Transformed {len(events_out)} explorer events")
    return len(events_out)


def _safe_parse_json(s: str) -> dict[str, Any]:
    """JSON をパース。失敗時は空 dict を返す。"""
    try:
        return json.loads(s)
    except (json.JSONDecodeError, TypeError):
        return {}
