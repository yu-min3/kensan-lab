"""Read-only S3 client for fetching note content from MinIO."""

import logging
from typing import Any

from kensan_ai.config import get_settings

logger = logging.getLogger(__name__)


class MinIOReadClient:
    """Read-only S3 client for fetching note content from MinIO."""

    def __init__(self) -> None:
        settings = get_settings()
        self.endpoint = settings.minio_endpoint
        self.access_key = settings.minio_access_key
        self.secret_key = settings.minio_secret_key
        self.bucket = settings.minio_bucket
        self.use_ssl = settings.minio_use_ssl
        self._client: Any = None

    def _get_client(self) -> Any:
        """Get or create the boto3 S3 client."""
        if self._client is None:
            import boto3

            protocol = "https" if self.use_ssl else "http"
            self._client = boto3.client(
                "s3",
                endpoint_url=f"{protocol}://{self.endpoint}",
                aws_access_key_id=self.access_key,
                aws_secret_access_key=self.secret_key,
            )
        return self._client

    def download_text(self, storage_key: str) -> str:
        """Download a text object from MinIO and return as string.

        Args:
            storage_key: The S3 key of the object (e.g. 'notes/{note_id}/{content_id}.md')

        Returns:
            The object content decoded as UTF-8 text.

        Raises:
            RuntimeError: If the download fails.
        """
        try:
            client = self._get_client()
            response = client.get_object(Bucket=self.bucket, Key=storage_key)
            return response["Body"].read().decode("utf-8")
        except Exception as e:
            logger.error("Failed to download %s from MinIO: %s", storage_key, e)
            raise RuntimeError(f"MinIO download failed for {storage_key}: {e}") from e


_client: MinIOReadClient | None = None


def get_minio_client() -> MinIOReadClient:
    """Get or create the global MinIOReadClient instance (singleton)."""
    global _client
    if _client is None:
        _client = MinIOReadClient()
    return _client
