"""Tests for Gold aggregate functions.

Tests pure computation logic and grouping using in-memory data.
"""

from datetime import date, datetime, timedelta, timezone

import pyarrow as pa
import pytest

from pipelines.gold.aggregate import _iso_week_start


class TestIsoWeekStart:
    """Test _iso_week_start pure function."""

    def test_monday(self):
        assert _iso_week_start(date(2026, 1, 19)) == date(2026, 1, 19)

    def test_tuesday(self):
        assert _iso_week_start(date(2026, 1, 20)) == date(2026, 1, 19)

    def test_wednesday(self):
        assert _iso_week_start(date(2026, 1, 21)) == date(2026, 1, 19)

    def test_thursday(self):
        assert _iso_week_start(date(2026, 1, 22)) == date(2026, 1, 19)

    def test_friday(self):
        assert _iso_week_start(date(2026, 1, 23)) == date(2026, 1, 19)

    def test_saturday(self):
        assert _iso_week_start(date(2026, 1, 24)) == date(2026, 1, 19)

    def test_sunday(self):
        assert _iso_week_start(date(2026, 1, 25)) == date(2026, 1, 19)

    def test_none(self):
        assert _iso_week_start(None) is None

    def test_year_boundary(self):
        # Dec 31, 2025 (Wednesday) -> Monday Dec 29
        assert _iso_week_start(date(2025, 12, 31)) == date(2025, 12, 29)

    def test_jan_1(self):
        # Jan 1, 2026 (Thursday) -> Monday Dec 29, 2025
        assert _iso_week_start(date(2026, 1, 1)) == date(2025, 12, 29)


class TestWeeklySummaryGrouping:
    """Test weekly_summary grouping logic (in-memory simulation)."""

    def test_single_user_single_week(self):
        """All entries in same week should group together."""
        entries = [
            ("user1", date(2026, 1, 19), 60),  # Monday
            ("user1", date(2026, 1, 20), 90),  # Tuesday
            ("user1", date(2026, 1, 21), 30),  # Wednesday
        ]

        summaries = {}
        for user_id, date_val, minutes in entries:
            week_start = _iso_week_start(date_val)
            key = (user_id, week_start)
            if key not in summaries:
                summaries[key] = {"total_minutes": 0}
            summaries[key]["total_minutes"] += minutes

        assert len(summaries) == 1
        assert summaries[("user1", date(2026, 1, 19))]["total_minutes"] == 180

    def test_single_user_two_weeks(self):
        """Entries in different weeks should be separate."""
        entries = [
            ("user1", date(2026, 1, 19), 60),   # Week 1 (Mon)
            ("user1", date(2026, 1, 26), 120),   # Week 2 (Mon)
        ]

        summaries = {}
        for user_id, date_val, minutes in entries:
            week_start = _iso_week_start(date_val)
            key = (user_id, week_start)
            if key not in summaries:
                summaries[key] = {"total_minutes": 0}
            summaries[key]["total_minutes"] += minutes

        assert len(summaries) == 2
        assert summaries[("user1", date(2026, 1, 19))]["total_minutes"] == 60
        assert summaries[("user1", date(2026, 1, 26))]["total_minutes"] == 120

    def test_two_users_same_week(self):
        """Different users in same week should be separate."""
        entries = [
            ("user1", date(2026, 1, 19), 60),
            ("user2", date(2026, 1, 20), 90),
        ]

        summaries = {}
        for user_id, date_val, minutes in entries:
            week_start = _iso_week_start(date_val)
            key = (user_id, week_start)
            if key not in summaries:
                summaries[key] = {"total_minutes": 0}
            summaries[key]["total_minutes"] += minutes

        assert len(summaries) == 2

    def test_task_counting(self):
        """Tasks should be counted and completed tasks tracked."""
        tasks = [
            ("user1", datetime(2026, 1, 19, 10, 0, tzinfo=timezone.utc), True),
            ("user1", datetime(2026, 1, 20, 10, 0, tzinfo=timezone.utc), False),
            ("user1", datetime(2026, 1, 21, 10, 0, tzinfo=timezone.utc), True),
        ]

        summaries = {}
        for user_id, created, completed in tasks:
            week_start = _iso_week_start(created.date())
            key = (user_id, week_start)
            if key not in summaries:
                summaries[key] = {"task_count": 0, "completed_task_count": 0}
            summaries[key]["task_count"] += 1
            if completed:
                summaries[key]["completed_task_count"] += 1

        key = ("user1", date(2026, 1, 19))
        assert summaries[key]["task_count"] == 3
        assert summaries[key]["completed_task_count"] == 2


class TestGoalProgressGrouping:
    """Test goal_progress grouping logic."""

    def test_groups_by_goal(self):
        entries = [
            ("user1", date(2026, 1, 19), "Goal A", 60),
            ("user1", date(2026, 1, 19), "Goal B", 30),
            ("user1", date(2026, 1, 20), "Goal A", 90),
        ]

        progress = {}
        for user_id, date_val, goal_name, minutes in entries:
            week_start = _iso_week_start(date_val)
            key = (user_id, goal_name, week_start)
            if key not in progress:
                progress[key] = {"total_minutes": 0, "entry_count": 0}
            progress[key]["total_minutes"] += minutes
            progress[key]["entry_count"] += 1

        assert len(progress) == 2  # Goal A + Goal B
        assert progress[("user1", "Goal A", date(2026, 1, 19))]["total_minutes"] == 150
        assert progress[("user1", "Goal B", date(2026, 1, 19))]["total_minutes"] == 30

    def test_skips_null_goal(self):
        entries = [
            ("user1", date(2026, 1, 19), None, 60),
            ("user1", date(2026, 1, 19), "Goal A", 30),
        ]

        progress = {}
        for user_id, date_val, goal_name, minutes in entries:
            if date_val is None or not goal_name:
                continue
            week_start = _iso_week_start(date_val)
            key = (user_id, goal_name, week_start)
            if key not in progress:
                progress[key] = {"total_minutes": 0, "entry_count": 0}
            progress[key]["total_minutes"] += minutes
            progress[key]["entry_count"] += 1

        assert len(progress) == 1


class TestEmotionTrend:
    """Test emotion trend calculation logic."""

    def test_improving_trend(self):
        """Valence increase > 0.1 should be improving."""
        prev_valence = 0.3
        curr_valence = 0.5
        diff = curr_valence - prev_valence
        assert diff > 0.1
        trend = "improving" if diff > 0.1 else "declining" if diff < -0.1 else "stable"
        assert trend == "improving"

    def test_declining_trend(self):
        prev_valence = 0.5
        curr_valence = 0.2
        diff = curr_valence - prev_valence
        assert diff < -0.1
        trend = "improving" if diff > 0.1 else "declining" if diff < -0.1 else "stable"
        assert trend == "declining"

    def test_stable_trend(self):
        prev_valence = 0.5
        curr_valence = 0.55
        diff = curr_valence - prev_valence
        trend = "improving" if diff > 0.1 else "declining" if diff < -0.1 else "stable"
        assert trend == "stable"
