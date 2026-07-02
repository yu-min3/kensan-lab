"""Tests for db_tools pure utility functions."""

from datetime import date, time, datetime
from zoneinfo import ZoneInfo

from kensan_ai.tools.db_tools import _local_date_to_utc_range, _combine_to_utc


class TestLocalDateToUtcRange:
    """Test _local_date_to_utc_range."""

    def test_jst_date_to_utc(self):
        jst = ZoneInfo("Asia/Tokyo")
        target = date(2026, 1, 20)
        start, end = _local_date_to_utc_range(target, jst)

        # JST is UTC+9, so 2026-01-20 00:00 JST = 2026-01-19 15:00 UTC
        assert start == datetime(2026, 1, 19, 15, 0, tzinfo=ZoneInfo("UTC"))
        # End: 2026-01-21 00:00 JST = 2026-01-20 15:00 UTC
        assert end == datetime(2026, 1, 20, 15, 0, tzinfo=ZoneInfo("UTC"))

    def test_utc_date(self):
        utc = ZoneInfo("UTC")
        target = date(2026, 6, 15)
        start, end = _local_date_to_utc_range(target, utc)

        assert start == datetime(2026, 6, 15, 0, 0, tzinfo=ZoneInfo("UTC"))
        assert end == datetime(2026, 6, 16, 0, 0, tzinfo=ZoneInfo("UTC"))

    def test_us_eastern_date(self):
        et = ZoneInfo("America/New_York")
        target = date(2026, 3, 10)
        start, end = _local_date_to_utc_range(target, et)

        # March 10, 2026 is after DST starts (March 8), so EDT = UTC-4
        assert start == datetime(2026, 3, 10, 4, 0, tzinfo=ZoneInfo("UTC"))
        assert end == datetime(2026, 3, 11, 4, 0, tzinfo=ZoneInfo("UTC"))

    def test_range_is_exactly_one_day(self):
        jst = ZoneInfo("Asia/Tokyo")
        target = date(2026, 7, 1)
        start, end = _local_date_to_utc_range(target, jst)

        delta = end - start
        assert delta.days == 1
        assert delta.seconds == 0


class TestCombineToUtc:
    """Test _combine_to_utc."""

    def test_jst_morning(self):
        jst = ZoneInfo("Asia/Tokyo")
        target = date(2026, 1, 20)
        local_time = time(9, 30)
        result = _combine_to_utc(target, local_time, jst)

        # 09:30 JST = 00:30 UTC
        assert result == datetime(2026, 1, 20, 0, 30, tzinfo=ZoneInfo("UTC"))

    def test_jst_midnight(self):
        jst = ZoneInfo("Asia/Tokyo")
        target = date(2026, 1, 20)
        local_time = time(0, 0)
        result = _combine_to_utc(target, local_time, jst)

        # 00:00 JST = 15:00 UTC (previous day)
        assert result == datetime(2026, 1, 19, 15, 0, tzinfo=ZoneInfo("UTC"))

    def test_utc_noon(self):
        utc = ZoneInfo("UTC")
        target = date(2026, 6, 15)
        local_time = time(12, 0)
        result = _combine_to_utc(target, local_time, utc)

        assert result == datetime(2026, 6, 15, 12, 0, tzinfo=ZoneInfo("UTC"))

    def test_preserves_minutes(self):
        jst = ZoneInfo("Asia/Tokyo")
        target = date(2026, 2, 1)
        local_time = time(14, 45)
        result = _combine_to_utc(target, local_time, jst)

        # 14:45 JST = 05:45 UTC
        assert result == datetime(2026, 2, 1, 5, 45, tzinfo=ZoneInfo("UTC"))
