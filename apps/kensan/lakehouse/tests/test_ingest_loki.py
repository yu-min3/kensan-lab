"""
Loki インジェストのユニットテスト
OTel パース、agent イベントフィルタ、状態管理
"""

import json

from pipelines.bronze.ingest_loki import AGENT_EVENT_TYPES, parse_otel_log


def _make_otel_line(event: str, **extra) -> str:
    """テスト用 OTel ログ行を生成"""
    body = {"event": event, **extra}
    envelope = {
        "body": json.dumps(body),
        "traceid": "abc123",
        "spanid": "span1",
        "severity": "INFO",
    }
    return json.dumps(envelope)


class TestParseOtelLog:
    def test_valid_prompt_event(self):
        line = _make_otel_line(
            "agent.prompt",
            user_id="user-1",
            model="gemini-2.0-flash",
            user_message="hello",
            context_id="ctx-1",
        )
        result = parse_otel_log("1706000000000000000", line)
        assert result is not None
        assert result["trace_id"] == "abc123"
        assert result["event_type"] == "agent.prompt"
        assert result["user_id"] == "user-1"

    def test_valid_complete_event(self):
        line = _make_otel_line(
            "agent.complete",
            outcome="success",
            total_turns=3,
            total_input_tokens=1000,
            total_output_tokens=500,
        )
        result = parse_otel_log("1706000000000000000", line)
        assert result is not None
        assert result["event_type"] == "agent.complete"

    def test_valid_turn_event(self):
        line = _make_otel_line(
            "agent.turn",
            turn_number=1,
            input_tokens=500,
            output_tokens=200,
        )
        result = parse_otel_log("1706000000000000000", line)
        assert result is not None
        assert result["event_type"] == "agent.turn"

    def test_valid_tool_call_event(self):
        line = _make_otel_line(
            "agent.tool_call",
            tool_name="get_tasks",
            success=True,
        )
        result = parse_otel_log("1706000000000000000", line)
        assert result is not None
        assert result["event_type"] == "agent.tool_call"

    def test_invalid_json_returns_none(self):
        result = parse_otel_log("1706000000000000000", "not json")
        assert result is None

    def test_empty_body_returns_event(self):
        envelope = {"body": "{}", "traceid": "t1"}
        result = parse_otel_log("1706000000000000000", json.dumps(envelope))
        assert result is not None
        assert result["event_type"] == ""

    def test_missing_user_id_defaults_to_empty(self):
        line = _make_otel_line("agent.turn", turn_number=1)
        result = parse_otel_log("1706000000000000000", line)
        assert result is not None
        assert result["user_id"] == ""

    def test_timestamp_conversion(self):
        # 1706000000000000000 ns = 2024-01-23T10:13:20 UTC
        line = _make_otel_line("agent.prompt")
        result = parse_otel_log("1706000000000000000", line)
        assert result is not None
        assert result["timestamp"].year == 2024


class TestAgentEventTypes:
    def test_all_agent_events_included(self):
        expected = {
            "agent.prompt",
            "agent.turn",
            "agent.tool_call",
            "agent.complete",
            "agent.system_prompt",
        }
        assert AGENT_EVENT_TYPES == expected

    def test_non_agent_event_not_in_filter(self):
        assert "http.request" not in AGENT_EVENT_TYPES
        assert "db.query" not in AGENT_EVENT_TYPES
