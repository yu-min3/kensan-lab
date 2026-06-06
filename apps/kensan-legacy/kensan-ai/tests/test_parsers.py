"""Tests for shared parser utilities."""

import pytest
from datetime import date, time
from uuid import UUID

from kensan_ai.lib.parsers import (
    parse_uuid,
    parse_date,
    parse_time,
    require_uuid,
    require_date,
)


class TestParseUuid:
    """Tests for parse_uuid function."""

    def test_valid_uuid(self):
        """Test parsing a valid UUID string."""
        result = parse_uuid("550e8400-e29b-41d4-a716-446655440000")
        assert result == UUID("550e8400-e29b-41d4-a716-446655440000")

    def test_valid_uuid_uppercase(self):
        """Test parsing uppercase UUID string."""
        result = parse_uuid("550E8400-E29B-41D4-A716-446655440000")
        assert result == UUID("550e8400-e29b-41d4-a716-446655440000")

    def test_invalid_uuid(self):
        """Test parsing invalid UUID string."""
        result = parse_uuid("not-a-uuid")
        assert result is None

    def test_empty_string(self):
        """Test parsing empty string."""
        result = parse_uuid("")
        assert result is None

    def test_none(self):
        """Test parsing None."""
        result = parse_uuid(None)
        assert result is None


class TestParseDate:
    """Tests for parse_date function."""

    def test_valid_date(self):
        """Test parsing a valid date string."""
        result = parse_date("2026-01-27")
        assert result == date(2026, 1, 27)

    def test_invalid_date_format(self):
        """Test parsing invalid date format."""
        result = parse_date("01-27-2026")
        assert result is None

    def test_invalid_date_value(self):
        """Test parsing invalid date value."""
        result = parse_date("2026-13-01")  # Invalid month
        assert result is None

    def test_empty_string(self):
        """Test parsing empty string."""
        result = parse_date("")
        assert result is None

    def test_none(self):
        """Test parsing None."""
        result = parse_date(None)
        assert result is None


class TestParseTime:
    """Tests for parse_time function."""

    def test_valid_time_hhmm(self):
        """Test parsing HH:MM format."""
        result = parse_time("14:30")
        assert result == time(14, 30)

    def test_valid_time_hhmmss(self):
        """Test parsing HH:MM:SS format."""
        result = parse_time("14:30:45")
        assert result == time(14, 30, 45)

    def test_invalid_time(self):
        """Test parsing invalid time string."""
        result = parse_time("not-a-time")
        assert result is None

    def test_invalid_time_value(self):
        """Test parsing invalid time value."""
        result = parse_time("25:00")  # Invalid hour
        assert result is None

    def test_empty_string(self):
        """Test parsing empty string."""
        result = parse_time("")
        assert result is None

    def test_none(self):
        """Test parsing None."""
        result = parse_time(None)
        assert result is None


class TestRequireUuid:
    """Tests for require_uuid function."""

    def test_valid_uuid(self):
        """Test requiring a valid UUID."""
        result = require_uuid("550e8400-e29b-41d4-a716-446655440000")
        assert result == UUID("550e8400-e29b-41d4-a716-446655440000")

    def test_invalid_uuid_raises(self):
        """Test that invalid UUID raises ValueError."""
        with pytest.raises(ValueError) as exc_info:
            require_uuid("not-a-uuid", field_name="user_id")
        assert "user_id" in str(exc_info.value)

    def test_none_raises(self):
        """Test that None raises ValueError."""
        with pytest.raises(ValueError) as exc_info:
            require_uuid(None, field_name="task_id")
        assert "task_id" in str(exc_info.value)

    def test_default_field_name(self):
        """Test default field name in error message."""
        with pytest.raises(ValueError) as exc_info:
            require_uuid(None)
        assert "id" in str(exc_info.value)


class TestRequireDate:
    """Tests for require_date function."""

    def test_valid_date(self):
        """Test requiring a valid date."""
        result = require_date("2026-01-27")
        assert result == date(2026, 1, 27)

    def test_invalid_date_raises(self):
        """Test that invalid date raises ValueError."""
        with pytest.raises(ValueError) as exc_info:
            require_date("not-a-date", field_name="due_date")
        assert "due_date" in str(exc_info.value)

    def test_none_raises(self):
        """Test that None raises ValueError."""
        with pytest.raises(ValueError) as exc_info:
            require_date(None, field_name="start_date")
        assert "start_date" in str(exc_info.value)

    def test_default_field_name(self):
        """Test default field name in error message."""
        with pytest.raises(ValueError) as exc_info:
            require_date(None)
        assert "date" in str(exc_info.value)
