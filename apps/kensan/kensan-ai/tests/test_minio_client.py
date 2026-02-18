"""Tests for kensan_ai.storage.minio_client — MinIOクライアントのユニットテスト。"""

from io import BytesIO
from unittest.mock import MagicMock, patch

import pytest


class TestMinIOReadClient:
    """MinIOReadClient のテスト。"""

    def _make_client(self):
        """テスト用にget_settingsをモックしてMinIOReadClientを生成する。"""
        with patch("kensan_ai.storage.minio_client.get_settings") as mock_settings:
            mock_settings.return_value = MagicMock(
                minio_endpoint="localhost:9000",
                minio_access_key="testkey",
                minio_secret_key="testsecret",
                minio_bucket="test-bucket",
                minio_use_ssl=False,
            )
            from kensan_ai.storage.minio_client import MinIOReadClient
            return MinIOReadClient()

    def test_download_text_success(self):
        """download_text成功 → UTF-8テキスト返却"""
        client = self._make_client()
        mock_s3 = MagicMock()
        body = BytesIO("テストコンテンツ".encode("utf-8"))
        mock_s3.get_object.return_value = {"Body": body}
        client._client = mock_s3

        result = client.download_text("notes/123/content.md")
        assert result == "テストコンテンツ"
        mock_s3.get_object.assert_called_once_with(
            Bucket="test-bucket", Key="notes/123/content.md"
        )

    def test_download_text_failure_raises_runtime_error(self):
        """download_text失敗 → RuntimeError発生"""
        client = self._make_client()
        mock_s3 = MagicMock()
        mock_s3.get_object.side_effect = Exception("Connection refused")
        client._client = mock_s3

        with pytest.raises(RuntimeError, match="MinIO download failed"):
            client.download_text("notes/bad/key.md")

    def test_get_client_lazy_initialization(self):
        """_get_clientは遅延初期化（2回呼んでも同じクライアント）"""
        client = self._make_client()
        assert client._client is None

        mock_boto3 = MagicMock()
        mock_boto3.client.return_value = MagicMock()
        with patch.dict("sys.modules", {"boto3": mock_boto3}):
            first = client._get_client()
            second = client._get_client()
            assert first is second
            mock_boto3.client.assert_called_once()

    def test_ssl_uses_https(self):
        """use_ssl=True → https:// エンドポイント"""
        with patch("kensan_ai.storage.minio_client.get_settings") as mock_settings:
            mock_settings.return_value = MagicMock(
                minio_endpoint="minio.example.com",
                minio_access_key="key",
                minio_secret_key="secret",
                minio_bucket="bucket",
                minio_use_ssl=True,
            )
            from kensan_ai.storage.minio_client import MinIOReadClient
            client = MinIOReadClient()

        mock_boto3 = MagicMock()
        mock_boto3.client.return_value = MagicMock()
        with patch.dict("sys.modules", {"boto3": mock_boto3}):
            client._get_client()
            mock_boto3.client.assert_called_once_with(
                "s3",
                endpoint_url="https://minio.example.com",
                aws_access_key_id="key",
                aws_secret_access_key="secret",
            )


class TestGetMinioClient:
    """get_minio_client シングルトンのテスト。"""

    def test_singleton(self):
        """2回呼んで同じインスタンスが返る"""
        import kensan_ai.storage.minio_client as mod

        # Reset singleton
        mod._client = None

        with patch.object(mod, "get_settings") as mock_settings:
            mock_settings.return_value = MagicMock(
                minio_endpoint="localhost:9000",
                minio_access_key="key",
                minio_secret_key="secret",
                minio_bucket="bucket",
                minio_use_ssl=False,
            )
            first = mod.get_minio_client()
            second = mod.get_minio_client()
            assert first is second

        # Cleanup
        mod._client = None
