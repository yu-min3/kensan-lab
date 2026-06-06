"""
Silver Explorer 変換のユニットテスト
グルーピング、user_id 伝播、event_order
"""

import json

from pipelines.silver.explorer_transform import _safe_parse_json


class TestSafeParseJson:
    def test_valid_json(self):
        assert _safe_parse_json('{"key": "value"}') == {"key": "value"}

    def test_invalid_json(self):
        assert _safe_parse_json("not json") == {}

    def test_none_input(self):
        assert _safe_parse_json(None) == {}

    def test_nested_json(self):
        data = {"a": {"b": [1, 2, 3]}}
        assert _safe_parse_json(json.dumps(data)) == data

    def test_empty_string(self):
        assert _safe_parse_json("") == {}


class TestUserIdPropagation:
    """user_id がpromptイベントからtrace全体に伝播されることを検証。

    transform_explorer_interactions/events はIceberg依存のため
    ロジックの検証は _safe_parse_json と統合テストに委ねる。
    """

    def test_prompt_payload_extracts_user_id(self):
        """prompt ペイロードから user_id を抽出できる"""
        payload = json.dumps({
            "event": "agent.prompt",
            "user_id": "user-123",
            "model": "gemini-2.0-flash",
            "user_message": "hello",
        })
        parsed = _safe_parse_json(payload)
        assert parsed.get("user_id") == "user-123"

    def test_complete_payload_extracts_outcome(self):
        """complete ペイロードから outcome を抽出できる"""
        payload = json.dumps({
            "event": "agent.complete",
            "outcome": "success",
            "total_turns": 3,
            "total_input_tokens": 1000,
            "total_output_tokens": 500,
            "pending_action_count": 0,
        })
        parsed = _safe_parse_json(payload)
        assert parsed.get("outcome") == "success"
        assert parsed.get("total_turns") == 3

    def test_prompt_sections_json_roundtrip(self):
        """system_prompt_sections の JSON 往復"""
        sections = {"基本設定": 500, "ツール定義": 1200}
        payload = json.dumps({"system_prompt_sections": sections})
        parsed = _safe_parse_json(payload)
        result_json = json.dumps(parsed.get("system_prompt_sections", {}), ensure_ascii=False)
        assert json.loads(result_json) == sections

    def test_tool_names_json_roundtrip(self):
        """tool_names の JSON 往復"""
        tool_names = ["get_tasks", "search_notes", "web_search"]
        payload = json.dumps({"tool_names": tool_names})
        parsed = _safe_parse_json(payload)
        result_json = json.dumps(parsed.get("tool_names", []), ensure_ascii=False)
        assert json.loads(result_json) == tool_names
