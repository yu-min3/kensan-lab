"""Unified error handling for Kensan AI service."""

from dataclasses import dataclass
from typing import Any


@dataclass
class ToolError(Exception):
    """Error raised by tools with structured information."""

    code: str
    message: str
    details: Any = None

    def __str__(self) -> str:
        return f"{self.code}: {self.message}"

    def to_dict(self) -> dict[str, Any]:
        result = {"code": self.code, "message": self.message}
        if self.details:
            result["details"] = self.details
        return result


class ValidationError(ToolError):
    """Error raised when input validation fails."""

    def __init__(self, field: str, message: str):
        super().__init__(
            code="VALIDATION_ERROR",
            message=f"{field}: {message}",
            details={"field": field},
        )


class NotFoundError(ToolError):
    """Error raised when a requested resource is not found."""

    def __init__(self, resource: str, resource_id: str | None = None):
        msg = f"{resource} not found" if not resource_id else f"{resource} '{resource_id}' not found"
        super().__init__(
            code="NOT_FOUND",
            message=msg,
            details={"resource": resource, "id": resource_id},
        )


class AuthenticationError(ToolError):
    """Error raised when authentication is required or fails."""

    def __init__(self, message: str = "Authentication required"):
        super().__init__(code="AUTHENTICATION_ERROR", message=message)


class AuthorizationError(ToolError):
    """Error raised when the user lacks permission for an action."""

    def __init__(self, message: str = "Permission denied"):
        super().__init__(code="AUTHORIZATION_ERROR", message=message)


class DatabaseError(ToolError):
    """Error raised when a database operation fails."""

    def __init__(self, message: str = "Database operation failed", details: Any = None):
        super().__init__(code="DATABASE_ERROR", message=message, details=details)


class ExternalServiceError(ToolError):
    """Error raised when an external service call fails."""

    def __init__(self, service: str, message: str = "External service error"):
        super().__init__(
            code="EXTERNAL_SERVICE_ERROR",
            message=message,
            details={"service": service},
        )
