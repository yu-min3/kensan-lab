"""Unit tests for _FilteringSpanProcessor."""

from __future__ import annotations

from unittest.mock import MagicMock

from kensan_ai.telemetry import _FilteringSpanProcessor


def _make_span(name: str) -> MagicMock:
    span = MagicMock()
    span.name = name
    return span


class TestFilteringSpanProcessor:
    """_FilteringSpanProcessor のテスト。"""

    def setup_method(self):
        self.delegate = MagicMock()
        self.processor = _FilteringSpanProcessor(self.delegate)

    # --- on_end フィルタリング ---

    def test_drops_post_sse_http_send(self):
        span = _make_span("POST /api/v1/agent/stream http send")
        self.processor.on_end(span)
        self.delegate.on_end.assert_not_called()

    def test_drops_get_sse_http_send(self):
        span = _make_span("GET /api/v1/agent/stream http send")
        self.processor.on_end(span)
        self.delegate.on_end.assert_not_called()

    # --- on_end 通過 ---

    def test_passes_normal_http_span(self):
        span = _make_span("POST /api/v1/agent/stream")
        self.processor.on_end(span)
        self.delegate.on_end.assert_called_once_with(span)

    def test_passes_other_endpoint_http_send(self):
        span = _make_span("GET /api/v1/health http send")
        self.processor.on_end(span)
        self.delegate.on_end.assert_called_once_with(span)

    def test_passes_db_span(self):
        span = _make_span("pg.query")
        self.processor.on_end(span)
        self.delegate.on_end.assert_called_once_with(span)

    def test_passes_http_send_without_stream_path(self):
        span = _make_span("POST /api/v1/chat http send")
        self.processor.on_end(span)
        self.delegate.on_end.assert_called_once_with(span)

    # --- デリゲーション ---

    def test_on_start_delegates(self):
        span = _make_span("test")
        ctx = MagicMock()
        self.processor.on_start(span, ctx)
        self.delegate.on_start.assert_called_once_with(span, ctx)

    def test_shutdown_delegates(self):
        self.processor.shutdown()
        self.delegate.shutdown.assert_called_once()

    def test_force_flush_delegates(self):
        self.processor.force_flush(5000)
        self.delegate.force_flush.assert_called_once_with(5000)
