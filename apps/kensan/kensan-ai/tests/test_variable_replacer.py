"""Tests for VariableReplacer."""

import pytest
from unittest.mock import patch, MagicMock
from uuid import UUID

from kensan_ai.context.variable_replacer import VariableReplacer


class TestSupportedVariables:
    """Test SUPPORTED_VARIABLES set."""

    def test_all_16_variables_present(self):
        expected = {
            "current_datetime",
            "user_memory",
            "today_schedule",
            "tomorrow_schedule",
            "today_entries",
            "pending_tasks",
            "recent_context",
            "weekly_summary",
            "goal_progress",
            "user_patterns",
            "emotion_summary",
            "interest_profile",
            "user_traits",
            "communication_style",
            "yesterday_entries",
            "recent_learning_notes",
        }
        assert VariableReplacer.SUPPORTED_VARIABLES == expected

    def test_variable_count(self):
        assert len(VariableReplacer.SUPPORTED_VARIABLES) == 16


class TestVariablePattern:
    """Test VARIABLE_PATTERN regex."""

    def test_matches_supported_variable(self):
        matches = VariableReplacer.VARIABLE_PATTERN.findall(
            "Hello {user_memory}, today is {current_datetime}"
        )
        assert matches == ["user_memory", "current_datetime"]

    def test_matches_unsupported_variable(self):
        matches = VariableReplacer.VARIABLE_PATTERN.findall(
            "Hello {unknown_var}"
        )
        assert matches == ["unknown_var"]

    def test_no_variables(self):
        matches = VariableReplacer.VARIABLE_PATTERN.findall(
            "No variables here"
        )
        assert matches == []

    def test_does_not_match_nested_braces(self):
        matches = VariableReplacer.VARIABLE_PATTERN.findall(
            "Code: {foo} and {{bar}}"
        )
        assert "foo" in matches
        assert "bar" in matches

    def test_does_not_match_empty_braces(self):
        matches = VariableReplacer.VARIABLE_PATTERN.findall("{}")
        assert matches == []


class TestReplace:
    """Test replace() method."""

    @pytest.mark.asyncio
    async def test_no_variables_returns_unchanged(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        prompt = "This is a prompt with no variables."
        result = await VariableReplacer.replace(prompt, user_id)
        assert result == prompt

    @pytest.mark.asyncio
    async def test_unsupported_variable_not_replaced(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        prompt = "Hello {unsupported_var}, welcome."
        result = await VariableReplacer.replace(prompt, user_id)
        assert result == prompt

    @pytest.mark.asyncio
    async def test_supported_variable_is_replaced(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        prompt = "Time: {current_datetime}"

        result = await VariableReplacer.replace(prompt, user_id)
        # current_datetime should be replaced with actual date
        assert "{current_datetime}" not in result
        assert "JST" in result

    @pytest.mark.asyncio
    async def test_multiple_variables_replaced(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        prompt = "Time: {current_datetime}, again: {current_datetime}"

        result = await VariableReplacer.replace(prompt, user_id)
        assert "{current_datetime}" not in result


class TestGetEmotionSummary:
    """Test _get_emotion_summary."""

    def test_returns_empty_when_reader_fails(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        with patch(
            "kensan_ai.context.variable_replacer.VariableReplacer._get_emotion_summary",
            wraps=VariableReplacer._get_emotion_summary,
        ):
            with patch(
                "kensan_ai.lakehouse.reader.get_reader",
                side_effect=Exception("connection failed"),
            ):
                result = VariableReplacer._get_emotion_summary(user_id)
                assert result == "（感情データなし）"

    def test_returns_empty_when_no_data(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        mock_reader = MagicMock()
        mock_reader.get_emotion_weekly.return_value = []

        with patch(
            "kensan_ai.lakehouse.reader.get_reader",
            return_value=mock_reader,
        ):
            result = VariableReplacer._get_emotion_summary(user_id)
            assert result == "（感情データなし）"

    def test_formats_data_correctly(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        mock_reader = MagicMock()
        mock_reader.get_emotion_weekly.return_value = [
            {
                "week_start": "2026-01-20",
                "avg_valence": 0.5,
                "avg_energy": 0.7,
                "avg_stress": 0.3,
                "dominant_emotion": "happy",
                "emotion_distribution_json": '{"happy": 5}',
                "diary_count": 5,
                "task_correlation_json": None,
                "valence_trend": "improving",
            }
        ]

        with patch(
            "kensan_ai.lakehouse.reader.get_reader",
            return_value=mock_reader,
        ):
            result = VariableReplacer._get_emotion_summary(user_id)
            assert "ポジティブ" in result
            assert "高い" in result  # energy > 0.6
            assert "happy" in result
            assert "改善傾向" in result


class TestGetInterestProfile:
    """Test _get_interest_profile."""

    def test_returns_empty_when_no_data(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        mock_reader = MagicMock()
        mock_reader.get_interest_profile.return_value = None

        with patch(
            "kensan_ai.lakehouse.reader.get_reader",
            return_value=mock_reader,
        ):
            result = VariableReplacer._get_interest_profile(user_id)
            assert result == "（関心データなし）"


class TestGetUserTraits:
    """Test _get_user_traits."""

    def test_returns_empty_when_no_data(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        mock_reader = MagicMock()
        mock_reader.get_trait_profile.return_value = None

        with patch(
            "kensan_ai.lakehouse.reader.get_reader",
            return_value=mock_reader,
        ):
            result = VariableReplacer._get_user_traits(user_id)
            assert result == "（特性データなし）"

    def test_formats_trait_data(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        mock_reader = MagicMock()
        mock_reader.get_trait_profile.return_value = {
            "work_style": "deep_focus",
            "learning_style": "hands-on",
            "collaboration": "async_first",
            "strengths_json": '["problem_solving", "system_design"]',
            "challenges_json": '["time_management"]',
            "triggers_json": '["new_technology"]',
            "trait_count": 10,
            "avg_confidence": 0.8,
        }

        with patch(
            "kensan_ai.lakehouse.reader.get_reader",
            return_value=mock_reader,
        ):
            result = VariableReplacer._get_user_traits(user_id)
            assert "deep_focus" in result
            assert "hands-on" in result
            assert "problem_solving" in result


class TestGetCommunicationStyle:
    """Test _get_communication_style."""

    def test_returns_empty_when_no_data(self):
        user_id = UUID("12345678-1234-1234-1234-123456789012")
        mock_reader = MagicMock()
        mock_reader.get_interest_profile.return_value = None
        mock_reader.get_trait_profile.return_value = None

        with patch(
            "kensan_ai.lakehouse.reader.get_reader",
            return_value=mock_reader,
        ):
            result = VariableReplacer._get_communication_style(user_id)
            assert result == ""
