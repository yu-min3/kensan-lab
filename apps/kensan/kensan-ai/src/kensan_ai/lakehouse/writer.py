"""Lakehouse Writer - Fire & forget append to Bronze Iceberg tables.

All errors are logged and swallowed so that tool responses are never blocked.
When lakehouse_enabled=False, all operations are no-ops.
"""

import asyncio
import json
import logging
from datetime import datetime, timezone
from typing import Any

import pyarrow as pa

from kensan_ai.config import Settings, get_settings

logger = logging.getLogger(__name__)


class LakehouseWriter:
    """Fire & forget で Bronze テーブルに append する共通ライター."""

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

    async def append_tool_result(
        self,
        tool_name: str,
        input_data: str,
        result_json: str,
        result_count: int,
        metadata: dict[str, Any] | None = None,
    ):
        """bronze.external_tool_results_raw に非同期 append.

        Args:
            tool_name: ツール名 (e.g. "web_search", "web_fetch")
            input_data: 入力データ (検索クエリ or URL)
            result_json: 結果のJSON文字列
            result_count: 結果件数
            metadata: 追加メタデータ (search_depth等)
        """
        if not self._enabled:
            return

        # Run blocking I/O in executor to avoid blocking the event loop
        loop = asyncio.get_event_loop()
        await loop.run_in_executor(
            None,
            self._append_tool_result_sync,
            tool_name,
            input_data,
            result_json,
            result_count,
            metadata,
        )

    def _append_tool_result_sync(
        self,
        tool_name: str,
        input_data: str,
        result_json: str,
        result_count: int,
        metadata: dict[str, Any] | None,
    ):
        """Synchronous append implementation."""
        try:
            catalog = self._get_catalog()
            table = catalog.load_table("bronze.external_tool_results_raw")
            now = datetime.now(timezone.utc)

            arrow_table = pa.table(
                {
                    "tool_name": [tool_name],
                    "input_data": [input_data],
                    "result_json": [result_json],
                    "result_count": [result_count],
                    "metadata_json": [json.dumps(metadata or {}, ensure_ascii=False)],
                    "executed_at": [now],
                    "_ingested_at": [now],
                },
                schema=pa.schema([
                    ("tool_name", pa.string()),
                    ("input_data", pa.string()),
                    ("result_json", pa.string()),
                    ("result_count", pa.int32()),
                    ("metadata_json", pa.string()),
                    ("executed_at", pa.timestamp("us", tz="UTC")),
                    ("_ingested_at", pa.timestamp("us", tz="UTC")),
                ]),
            )

            table.append(arrow_table)
            logger.debug(f"Lakehouse: appended {tool_name} result")
        except Exception as e:
            logger.warning(f"Lakehouse write failed (ignored): {e}")


# Singleton
_writer: LakehouseWriter | None = None


def get_writer() -> LakehouseWriter:
    """Get or create the singleton LakehouseWriter."""
    global _writer
    if _writer is None:
        _writer = LakehouseWriter(get_settings())
    return _writer
