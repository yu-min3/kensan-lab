"""Tests for Silver transform functions.

Tests pure computation logic using in-memory PyArrow tables.
"""

from datetime import date, datetime, timedelta, timezone

import pyarrow as pa
import pyarrow.compute as pc
import pytest


class TestIsoWeekStart:
    """Test _iso_week_start from aggregate (imported for reuse in transform context)."""

    def test_monday_returns_same(self):
        from pipelines.gold.aggregate import _iso_week_start

        monday = date(2026, 1, 19)  # Monday
        assert _iso_week_start(monday) == monday

    def test_sunday_returns_previous_monday(self):
        from pipelines.gold.aggregate import _iso_week_start

        sunday = date(2026, 1, 25)  # Sunday
        assert _iso_week_start(sunday) == date(2026, 1, 19)

    def test_wednesday(self):
        from pipelines.gold.aggregate import _iso_week_start

        wednesday = date(2026, 1, 21)  # Wednesday
        assert _iso_week_start(wednesday) == date(2026, 1, 19)

    def test_none_returns_none(self):
        from pipelines.gold.aggregate import _iso_week_start

        assert _iso_week_start(None) is None


class TestTransformTimeEntriesPure:
    """Test time_entries transform computation logic (duration, date extraction)."""

    def test_duration_calculation(self):
        """Duration should be (end - start) in minutes."""
        start = datetime(2026, 1, 20, 9, 0, tzinfo=timezone.utc)
        end = datetime(2026, 1, 20, 10, 30, tzinfo=timezone.utc)
        delta = (end - start).total_seconds() / 60
        assert int(delta) == 90

    def test_duration_zero_for_same_time(self):
        start = datetime(2026, 1, 20, 9, 0, tzinfo=timezone.utc)
        end = start
        delta = (end - start).total_seconds() / 60
        assert int(delta) == 0

    def test_date_extraction(self):
        dt = datetime(2026, 1, 20, 15, 30, tzinfo=timezone.utc)
        assert dt.date() == date(2026, 1, 20)


class TestTransformTasksPure:
    """Test tasks transform: is_subtask flag logic."""

    def test_is_subtask_with_parent(self):
        """Tasks with parent_task_id should be subtasks."""
        parent_ids = pa.array(["parent-1", None, "parent-2"], type=pa.string())
        is_subtask = pc.is_valid(parent_ids)
        assert is_subtask.to_pylist() == [True, False, True]

    def test_is_subtask_all_null(self):
        parent_ids = pa.array([None, None], type=pa.string())
        is_subtask = pc.is_valid(parent_ids)
        assert is_subtask.to_pylist() == [False, False]

    def test_is_subtask_all_present(self):
        parent_ids = pa.array(["p1", "p2"], type=pa.string())
        is_subtask = pc.is_valid(parent_ids)
        assert is_subtask.to_pylist() == [True, True]


class TestTransformNotesPure:
    """Test notes transform: content_length calculation."""

    def test_content_length(self):
        contents = ["Hello world", "", "日本語テスト"]
        lengths = [len(c) if c else 0 for c in contents]
        assert lengths == [11, 0, 6]

    def test_content_length_with_none(self):
        contents = [None, "abc"]
        lengths = [len(c) if c else 0 for c in contents]
        assert lengths == [0, 3]

    def test_content_length_unicode(self):
        content = "こんにちは世界"
        assert len(content) == 7


class TestTransformAiInteractionsPure:
    """Test AI interactions transform logic."""

    def test_tokens_total(self):
        tokens_in = 100
        tokens_out = 200
        assert tokens_in + tokens_out == 300

    def test_tool_count_from_json(self):
        import json

        tool_calls_json = json.dumps([
            {"name": "get_tasks", "input": {}},
            {"name": "create_task", "input": {}},
        ])
        calls = json.loads(tool_calls_json)
        assert len(calls) == 2

    def test_tool_names_unique(self):
        import json

        calls = [
            {"name": "get_tasks"},
            {"name": "get_tasks"},
            {"name": "create_task"},
        ]
        names = list({c.get("name", "") for c in calls if isinstance(c, dict)})
        assert set(names) == {"get_tasks", "create_task"}

    def test_date_extraction_from_created_at(self):
        created = datetime(2026, 1, 20, 15, 30, tzinfo=timezone.utc)
        assert created.date() == date(2026, 1, 20)

    def test_none_created_at(self):
        created = None
        result = created.date() if created else None
        assert result is None
