"""
Emotion Extractor のユニットテスト
LLMはモック、JSONパース・バリデーションのテスト
"""

from pipelines.silver.emotion_extractor import _parse_llm_response


class TestParseLlmResponse:
    """LLM応答のパース・バリデーションテスト"""

    def test_valid_json(self):
        """正常なJSON応答をパースできる"""
        response = '''{
            "time_hint": "morning",
            "valence": 0.7,
            "energy": 0.8,
            "stress": 0.2,
            "dominant_emotion": "joy",
            "keywords": ["楽しい", "達成感"],
            "related_tasks": [{"task_id": "abc-123", "task_name": "プログラミング"}],
            "confidence": 0.9
        }'''
        result = _parse_llm_response(response)
        assert result is not None
        assert result["time_hint"] == "morning"
        assert result["valence"] == 0.7
        assert result["energy"] == 0.8
        assert result["stress"] == 0.2
        assert result["dominant_emotion"] == "joy"
        assert result["keywords"] == ["楽しい", "達成感"]
        assert len(result["related_tasks"]) == 1
        assert result["confidence"] == 0.9

    def test_markdown_code_block(self):
        """マークダウンコードブロックで囲まれたJSON"""
        response = '''```json
{
    "valence": 0.5,
    "energy": 0.6,
    "stress": 0.3,
    "dominant_emotion": "focus",
    "confidence": 0.8
}
```'''
        result = _parse_llm_response(response)
        assert result is not None
        assert result["dominant_emotion"] == "focus"
        assert result["valence"] == 0.5

    def test_clamp_values(self):
        """範囲外の値がクランプされる"""
        response = '''{
            "valence": 1.5,
            "energy": -0.5,
            "stress": 2.0,
            "dominant_emotion": "joy",
            "confidence": 1.5
        }'''
        result = _parse_llm_response(response)
        assert result is not None
        assert result["valence"] == 1.0
        assert result["energy"] == 0.0
        assert result["stress"] == 1.0
        assert result["confidence"] == 1.0

    def test_default_values(self):
        """オプションフィールドにデフォルト値が設定される"""
        response = '''{
            "valence": 0.0,
            "energy": 0.5,
            "stress": 0.5,
            "dominant_emotion": "neutral",
            "confidence": 0.5
        }'''
        result = _parse_llm_response(response)
        assert result is not None
        assert result["time_hint"] == "unknown"
        assert result["keywords"] == []
        assert result["related_tasks"] == []

    def test_invalid_time_hint(self):
        """無効なtime_hintがunknownに修正される"""
        response = '''{
            "time_hint": "midnight",
            "valence": 0.0,
            "energy": 0.5,
            "stress": 0.5,
            "dominant_emotion": "neutral",
            "confidence": 0.5
        }'''
        result = _parse_llm_response(response)
        assert result is not None
        assert result["time_hint"] == "unknown"

    def test_missing_required_fields(self):
        """必須フィールドが不足している場合はNone"""
        response = '{"valence": 0.5}'
        result = _parse_llm_response(response)
        assert result is None

    def test_invalid_json(self):
        """JSONでない応答はNone"""
        result = _parse_llm_response("This is not JSON")
        assert result is None

    def test_empty_string(self):
        """空文字列はNone"""
        result = _parse_llm_response("")
        assert result is None
