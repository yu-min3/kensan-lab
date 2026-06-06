"""Lakehouse Reader - Read Gold/Silver layer tables via PyIceberg.

Reads pre-aggregated data from Iceberg tables for prompt injection and Explorer.
All errors return empty results so that prompt resolution is never blocked.
When lakehouse_enabled=False, all operations return empty results.
"""

import json
import logging
from datetime import datetime
from typing import Any

from kensan_ai.config import Settings, get_settings

logger = logging.getLogger(__name__)


class LakehouseReader:
    """Gold層テーブルを読み取る共通リーダー."""

    def __init__(self, settings: Settings):
        self._catalog = None  # lazy init
        self._enabled = settings.lakehouse_enabled
        self._settings = settings

    def _get_catalog(self):
        """Polaris catalog への接続 (lazy, cached)."""
        if self._catalog is None:
            from pyiceberg.catalog import load_catalog

            self._catalog = load_catalog(
                "polaris",
                **{
                    "type": "rest",
                    "uri": self._settings.polaris_uri,
                    "credential": self._settings.polaris_credential,
                    "scope": "PRINCIPAL_ROLE:ALL",
                    "warehouse": self._settings.polaris_warehouse,
                    "s3.endpoint": self._settings.lakehouse_s3_endpoint,
                    "s3.access-key-id": self._settings.lakehouse_s3_access_key,
                    "s3.secret-access-key": self._settings.lakehouse_s3_secret_key,
                    "s3.path-style-access": "true",
                    "s3.region": "us-east-1",
                },
            )
        return self._catalog

    def get_emotion_weekly(self, user_id: str, weeks: int = 4) -> list[dict[str, Any]]:
        """Gold層の感情週次集計を取得.

        Args:
            user_id: ユーザーID
            weeks: 取得する週数（最新N週）

        Returns:
            週次感情データのリスト。エラー時は空リスト。
        """
        if not self._enabled:
            return []

        try:
            from pyiceberg.expressions import EqualTo

            catalog = self._get_catalog()
            table = catalog.load_table("gold.emotion_weekly")
            df = table.scan(row_filter=EqualTo("user_id", user_id)).to_arrow()

            rows = df.to_pydict()
            results: list[dict[str, Any]] = []
            for i in range(df.num_rows):
                results.append({
                    "week_start": rows["week_start"][i],
                    "avg_valence": rows["avg_valence"][i],
                    "avg_energy": rows["avg_energy"][i],
                    "avg_stress": rows["avg_stress"][i],
                    "dominant_emotion": rows["dominant_emotion"][i],
                    "emotion_distribution_json": rows["emotion_distribution_json"][i],
                    "diary_count": rows["diary_count"][i],
                    "task_correlation_json": rows["task_correlation_json"][i],
                    "valence_trend": rows["valence_trend"][i],
                })

            # 最新N週に絞る
            results.sort(key=lambda x: x["week_start"] or "", reverse=True)
            return results[:weeks]

        except Exception as e:
            logger.warning(f"Lakehouse read failed (returning empty): {e}")
            return []

    def get_interest_profile(self, user_id: str) -> dict[str, Any] | None:
        """Gold層のユーザー関心プロファイルを取得.

        Args:
            user_id: ユーザーID

        Returns:
            最新の関心プロファイル。エラー時はNone。
        """
        if not self._enabled:
            return None

        try:
            from pyiceberg.expressions import EqualTo

            catalog = self._get_catalog()
            table = catalog.load_table("gold.user_interest_profile")
            df = table.scan(row_filter=EqualTo("user_id", user_id)).to_arrow()

            if df.num_rows == 0:
                return None

            rows = df.to_pydict()
            results: list[dict[str, Any]] = []
            for i in range(df.num_rows):
                results.append({
                    "week_start": rows["week_start"][i],
                    "top_tags_json": rows["top_tags_json"][i],
                    "emerging_tags_json": rows["emerging_tags_json"][i],
                    "fading_tags_json": rows["fading_tags_json"][i],
                    "tag_clusters_json": rows["tag_clusters_json"][i],
                    "total_tagged_notes": rows["total_tagged_notes"][i],
                })

            # 最新1件
            results.sort(key=lambda x: x["week_start"] or "", reverse=True)
            return results[0]

        except Exception as e:
            logger.warning(f"Lakehouse read failed for interest_profile: {e}")
            return None


    def get_trait_profile(self, user_id: str) -> dict[str, Any] | None:
        """Gold層のユーザー性格プロファイルを取得.

        Args:
            user_id: ユーザーID

        Returns:
            性格プロファイル。エラー時はNone。
        """
        if not self._enabled:
            return None

        try:
            from pyiceberg.expressions import EqualTo

            catalog = self._get_catalog()
            table = catalog.load_table("gold.user_trait_profile")
            df = table.scan(row_filter=EqualTo("user_id", user_id)).to_arrow()

            if df.num_rows == 0:
                return None

            rows = df.to_pydict()
            return {
                "work_style": rows["work_style"][0],
                "learning_style": rows["learning_style"][0],
                "collaboration": rows["collaboration"][0],
                "strengths_json": rows["strengths_json"][0],
                "challenges_json": rows["challenges_json"][0],
                "triggers_json": rows["triggers_json"][0],
                "trait_count": rows["trait_count"][0],
                "avg_confidence": rows["avg_confidence"][0],
            }

        except Exception as e:
            logger.warning(f"Lakehouse read failed for trait_profile: {e}")
            return None

    def get_explorer_interactions(
        self,
        user_id: str,
        start: datetime,
        end: datetime,
    ) -> list[dict[str, Any]]:
        """Silver層の AI Explorer インタラクションを取得.

        Args:
            user_id: ユーザーID
            start: 開始時刻 (UTC)
            end: 終了時刻 (UTC)

        Returns:
            Interaction 互換データのリスト。エラー時は空リスト。
        """
        if not self._enabled:
            return []

        try:
            from pyiceberg.expressions import And, EqualTo, GreaterThanOrEqual, LessThan

            catalog = self._get_catalog()

            # 1. Silver interactions を取得
            interactions_table = catalog.load_table("silver.ai_explorer_interactions")
            row_filter = And(
                EqualTo("user_id", user_id),
                GreaterThanOrEqual("timestamp", start),
                LessThan("timestamp", end),
            )
            interactions_df = interactions_table.scan(row_filter=row_filter).to_arrow()

            if interactions_df.num_rows == 0:
                return []

            interactions_rows = interactions_df.to_pydict()
            trace_ids = set(interactions_rows["trace_id"])

            # 2. Silver events を取得 (該当 trace_id のみ)
            events_table = catalog.load_table("silver.ai_explorer_events")
            events_df = events_table.scan(
                row_filter=EqualTo("user_id", user_id),
            ).to_arrow()

            events_rows = events_df.to_pydict()
            events_by_trace: dict[str, list[dict[str, Any]]] = {}
            for i in range(events_df.num_rows):
                trace_id = events_rows["trace_id"][i]
                if trace_id not in trace_ids:
                    continue
                event = {
                    "event": events_rows["event_type"][i],
                    "traceId": trace_id,
                    "conversation_id": events_rows["conversation_id"][i],
                    "timestamp": events_rows["timestamp"][i].isoformat()
                    if events_rows["timestamp"][i] else None,
                    "payload": events_rows["payload"][i],
                    "event_order": events_rows["event_order"][i],
                }
                events_by_trace.setdefault(trace_id, []).append(event)

            # Sort events by event_order
            for trace_events in events_by_trace.values():
                trace_events.sort(key=lambda e: e.get("event_order", 0))

            # 3. Interaction レスポンス構築
            results: list[dict[str, Any]] = []
            for i in range(interactions_df.num_rows):
                trace_id = interactions_rows["trace_id"][i]
                ts = interactions_rows["timestamp"][i]

                # Parse JSON fields
                system_prompt_sections = _safe_json_parse(
                    interactions_rows["system_prompt_sections_json"][i]
                )
                tool_names = _safe_json_parse(
                    interactions_rows["tool_names_json"][i]
                )

                # Build events with parsed payload
                trace_events = events_by_trace.get(trace_id, [])
                parsed_events = []
                for ev in trace_events:
                    payload = _safe_json_parse(ev.get("payload", "{}"))
                    parsed_event = {
                        "event": ev["event"],
                        "traceId": ev["traceId"],
                        "conversation_id": ev.get("conversation_id"),
                        "timestamp": ev["timestamp"],
                        **payload,
                    }
                    parsed_events.append(parsed_event)

                results.append({
                    "traceId": trace_id,
                    "timestamp": ts.isoformat() if ts else None,
                    "outcome": interactions_rows["outcome"][i] or "in_progress",
                    "model": interactions_rows["model"][i] or "",
                    "totalTurns": interactions_rows["total_turns"][i] or 0,
                    "totalInputTokens": interactions_rows["total_input_tokens"][i] or 0,
                    "totalOutputTokens": interactions_rows["total_output_tokens"][i] or 0,
                    "pendingActionCount": interactions_rows["pending_action_count"][i] or 0,
                    "userMessage": interactions_rows["user_message"][i] or "",
                    "contextId": interactions_rows["context_id"][i] or "",
                    "contextName": interactions_rows["context_name"][i] or "",
                    "contextVersion": interactions_rows["context_version"][i] or "",
                    "experimentId": interactions_rows["experiment_id"][i] or "",
                    "systemPromptLength": interactions_rows["system_prompt_length"][i] or 0,
                    "systemPromptSections": system_prompt_sections if isinstance(system_prompt_sections, dict) else {},
                    "toolCount": interactions_rows["tool_count"][i] or 0,
                    "toolNames": tool_names if isinstance(tool_names, list) else [],
                    "toolDefinitionsLength": interactions_rows["tool_definitions_length"][i] or 0,
                    "events": parsed_events,
                })

            # Most recent first
            results.sort(key=lambda x: x["timestamp"] or "", reverse=True)
            return results

        except Exception as e:
            logger.warning(f"Lakehouse read failed for explorer_interactions: {e}")
            return []


def _safe_json_parse(s: str | None) -> Any:
    """JSON をパース。失敗時はそのまま返す。"""
    if not s:
        return {}
    try:
        return json.loads(s)
    except (json.JSONDecodeError, TypeError):
        return {}


# Singleton
_reader: LakehouseReader | None = None


def get_reader() -> LakehouseReader:
    """Get or create the singleton LakehouseReader."""
    global _reader
    if _reader is None:
        _reader = LakehouseReader(get_settings())
    return _reader
