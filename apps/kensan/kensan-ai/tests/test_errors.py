"""Tests for error handling."""

import pytest

from kensan_ai.errors import (
    ToolError,
    ValidationError,
    NotFoundError,
    AuthenticationError,
    AuthorizationError,
    DatabaseError,
    ExternalServiceError,
)


class TestToolError:
    """Tests for ToolError base class."""

    def test_tool_error_str(self):
        """Test string representation."""
        err = ToolError(code="TEST_ERROR", message="Test message")
        assert str(err) == "TEST_ERROR: Test message"

    def test_tool_error_to_dict_without_details(self):
        """Test conversion to dict without details."""
        err = ToolError(code="TEST_ERROR", message="Test message")
        result = err.to_dict()
        assert result == {"code": "TEST_ERROR", "message": "Test message"}

    def test_tool_error_to_dict_with_details(self):
        """Test conversion to dict with details."""
        err = ToolError(code="TEST_ERROR", message="Test message", details={"key": "value"})
        result = err.to_dict()
        assert result == {
            "code": "TEST_ERROR",
            "message": "Test message",
            "details": {"key": "value"},
        }


class TestValidationError:
    """Tests for ValidationError."""

    def test_validation_error(self):
        """Test validation error creation."""
        err = ValidationError(field="user_id", message="must be a valid UUID")
        assert err.code == "VALIDATION_ERROR"
        assert "user_id" in err.message
        assert "must be a valid UUID" in err.message
        assert err.details == {"field": "user_id"}

    def test_validation_error_to_dict(self):
        """Test validation error dict conversion."""
        err = ValidationError(field="email", message="invalid format")
        result = err.to_dict()
        assert result["code"] == "VALIDATION_ERROR"
        assert "email" in result["message"]
        assert result["details"]["field"] == "email"


class TestNotFoundError:
    """Tests for NotFoundError."""

    def test_not_found_without_id(self):
        """Test not found error without resource ID."""
        err = NotFoundError(resource="Task")
        assert err.code == "NOT_FOUND"
        assert err.message == "Task not found"
        assert err.details["resource"] == "Task"
        assert err.details["id"] is None

    def test_not_found_with_id(self):
        """Test not found error with resource ID."""
        err = NotFoundError(resource="Task", resource_id="abc-123")
        assert err.code == "NOT_FOUND"
        assert err.message == "Task 'abc-123' not found"
        assert err.details["resource"] == "Task"
        assert err.details["id"] == "abc-123"


class TestAuthenticationError:
    """Tests for AuthenticationError."""

    def test_default_message(self):
        """Test default authentication error message."""
        err = AuthenticationError()
        assert err.code == "AUTHENTICATION_ERROR"
        assert err.message == "Authentication required"

    def test_custom_message(self):
        """Test custom authentication error message."""
        err = AuthenticationError(message="Token expired")
        assert err.code == "AUTHENTICATION_ERROR"
        assert err.message == "Token expired"


class TestAuthorizationError:
    """Tests for AuthorizationError."""

    def test_default_message(self):
        """Test default authorization error message."""
        err = AuthorizationError()
        assert err.code == "AUTHORIZATION_ERROR"
        assert err.message == "Permission denied"

    def test_custom_message(self):
        """Test custom authorization error message."""
        err = AuthorizationError(message="Admin access required")
        assert err.code == "AUTHORIZATION_ERROR"
        assert err.message == "Admin access required"


class TestDatabaseError:
    """Tests for DatabaseError."""

    def test_default_message(self):
        """Test default database error message."""
        err = DatabaseError()
        assert err.code == "DATABASE_ERROR"
        assert err.message == "Database operation failed"

    def test_custom_message_with_details(self):
        """Test custom database error with details."""
        err = DatabaseError(message="Connection failed", details={"host": "localhost"})
        assert err.code == "DATABASE_ERROR"
        assert err.message == "Connection failed"
        assert err.details == {"host": "localhost"}


class TestExternalServiceError:
    """Tests for ExternalServiceError."""

    def test_external_service_error(self):
        """Test external service error creation."""
        err = ExternalServiceError(service="Anthropic", message="API rate limited")
        assert err.code == "EXTERNAL_SERVICE_ERROR"
        assert err.message == "API rate limited"
        assert err.details["service"] == "Anthropic"
