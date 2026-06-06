"""Tests for configuration validation."""

import os
import pytest
from unittest.mock import patch

from pydantic import ValidationError

from kensan_ai.config import Settings


class TestDebugParsing:
    """Tests for debug flag parsing."""

    def test_debug_true_string(self):
        """Test parsing 'true' string."""
        settings = Settings(debug="true")  # type: ignore
        assert settings.debug is True

    def test_debug_false_string(self):
        """Test parsing 'false' string."""
        settings = Settings(debug="false")  # type: ignore
        assert settings.debug is False

    def test_debug_1_string(self):
        """Test parsing '1' string."""
        settings = Settings(debug="1")  # type: ignore
        assert settings.debug is True

    def test_debug_0_string(self):
        """Test parsing '0' string."""
        settings = Settings(debug="0")  # type: ignore
        assert settings.debug is False

    def test_debug_yes_string(self):
        """Test parsing 'yes' string."""
        settings = Settings(debug="yes")  # type: ignore
        assert settings.debug is True

    def test_debug_bool_true(self):
        """Test boolean True."""
        settings = Settings(debug=True)
        assert settings.debug is True

    def test_debug_bool_false(self):
        """Test boolean False."""
        settings = Settings(debug=False)
        assert settings.debug is False


class TestProductionValidation:
    """Tests for production environment validation."""

    def test_development_no_api_key_ok(self):
        """Test that missing API key is OK in development."""
        settings = Settings(
            server_env="development",
            anthropic_api_key="",
        )
        assert settings.anthropic_api_key == ""

    def test_development_default_jwt_ok(self):
        """Test that default JWT secret is OK in development."""
        settings = Settings(
            server_env="development",
            jwt_secret="dev-secret-key-change-in-production",
        )
        assert settings.jwt_secret == "dev-secret-key-change-in-production"

    def test_production_missing_api_key_fails(self):
        """Test that missing API key fails in production."""
        with pytest.raises(ValidationError) as exc_info:
            Settings(
                server_env="production",
                anthropic_api_key="",
            )
        assert "ANTHROPIC_API_KEY is required in production" in str(exc_info.value)

    def test_production_default_jwt_fails(self):
        """Test that default JWT secret fails in production."""
        with pytest.raises(ValidationError) as exc_info:
            Settings(
                server_env="production",
                anthropic_api_key="sk-valid-key",
                jwt_secret="dev-secret-key-change-in-production",
            )
        assert "JWT_SECRET must be changed in production" in str(exc_info.value)

    def test_production_valid_settings_ok(self):
        """Test that valid production settings work."""
        settings = Settings(
            server_env="production",
            anthropic_api_key="sk-valid-key",
            jwt_secret="secure-production-secret-key-123",
        )
        assert settings.server_env == "production"
        assert settings.anthropic_api_key == "sk-valid-key"
        assert settings.jwt_secret == "secure-production-secret-key-123"


class TestDatabaseUrl:
    """Tests for database URL construction."""

    def test_explicit_database_url(self):
        """Test that explicit database URL is used."""
        settings = Settings(database_url="postgresql://user:pass@host:5432/db")
        assert settings.effective_database_url == "postgresql://user:pass@host:5432/db"

    def test_constructed_database_url(self):
        """Test that database URL is constructed from components."""
        settings = Settings(
            db_host="myhost",
            db_port=5433,
            db_user="myuser",
            db_password="mypass",
            db_name="mydb",
        )
        assert settings.effective_database_url == "postgresql://myuser:mypass@myhost:5433/mydb"

    def test_default_database_url(self):
        """Test default database URL."""
        settings = Settings()
        assert settings.effective_database_url == "postgresql://kensan:kensan@localhost:5432/kensan"
