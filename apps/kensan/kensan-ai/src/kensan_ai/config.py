"""Application configuration using pydantic-settings."""

from functools import lru_cache
from typing import Any

from pydantic import computed_field, field_validator, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )

    # Database (supports both single URL and individual components)
    database_url: str | None = None
    db_host: str = "localhost"
    db_port: int = 5432
    db_user: str = "kensan"
    db_password: str = "kensan"
    db_name: str = "kensan"

    @computed_field
    @property
    def effective_database_url(self) -> str:
        """Get the database URL, constructing from components if not provided."""
        if self.database_url:
            return self.database_url
        return f"postgresql://{self.db_user}:{self.db_password}@{self.db_host}:{self.db_port}/{self.db_name}"

    # AI Provider ("anthropic", "google", or "google-adk")
    ai_provider: str = "anthropic"

    # Anthropic API
    anthropic_api_key: str = ""
    anthropic_model: str = "claude-sonnet-4-20250514"
    anthropic_fast_model: str = "claude-haiku-4-5-20251001"

    # Google GenAI API
    google_api_key: str = ""
    google_model: str = "gemini-2.0-flash"

    # OpenAI API (for embeddings)
    openai_api_key: str = ""
    embedding_model: str = "text-embedding-3-small"

    # Embedding provider ("openai" or "gemini")
    embedding_provider: str = "openai"
    gemini_embedding_model: str = "gemini-embedding-001"

    # MinIO Storage (read-only access for note content)
    minio_endpoint: str = "localhost:9000"
    minio_access_key: str = "kensan"
    minio_secret_key: str = "kensan-minio"
    minio_bucket: str = "kensan-notes"
    minio_use_ssl: bool = False

    # JWT (for verifying tokens from user-service)
    jwt_secret: str = "dev-secret-key-change-in-production"

    # Server
    server_port: int = 8089
    server_env: str = "development"
    host: str = "0.0.0.0"
    debug: bool = False

    # Agent settings
    default_max_turns: int = 10
    default_temperature: float = 0.7

    # OpenTelemetry
    otel_enabled: bool = False
    otel_collector_url: str = "localhost:4318"

    # External Tools
    tavily_api_key: str = ""

    # Lakehouse (Iceberg direct write via Polaris)
    polaris_uri: str = "http://localhost:8181/api/catalog"
    polaris_credential: str = "root:s3cr3t"
    polaris_warehouse: str = "kensan-lakehouse"
    lakehouse_s3_endpoint: str = "http://localhost:9000"
    lakehouse_s3_access_key: str = "kensan"
    lakehouse_s3_secret_key: str = "kensan-minio"
    lakehouse_s3_bucket: str = "kensan-lakehouse"
    lakehouse_enabled: bool = False

    @field_validator("debug", mode="before")
    @classmethod
    def parse_debug_bool(cls, v: Any) -> bool:
        """Parse debug flag from string or boolean."""
        if isinstance(v, bool):
            return v
        if isinstance(v, str):
            return v.lower() in ("true", "1", "yes")
        return bool(v)

    @model_validator(mode="after")
    def validate_production_settings(self) -> "Settings":
        """Validate critical settings in production environment."""
        if self.server_env == "production":
            if self.ai_provider == "anthropic" and not self.anthropic_api_key:
                raise ValueError("ANTHROPIC_API_KEY is required in production with ai_provider=anthropic")
            if self.ai_provider in ("google", "google-adk") and not self.google_api_key:
                raise ValueError("GOOGLE_API_KEY is required in production with ai_provider=google")
            if self.jwt_secret == "dev-secret-key-change-in-production":
                raise ValueError("JWT_SECRET must be changed in production")
        return self


@lru_cache
def get_settings() -> Settings:
    """Get cached settings instance."""
    return Settings()
