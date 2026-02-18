"""
共通設定: Polaris Catalog接続、S3設定、ロギング
"""

import logging
import os
import sys
from pathlib import Path

from dotenv import load_dotenv
from pyiceberg.catalog import Catalog, load_catalog

# .envファイルを読み込み
load_dotenv(Path(__file__).parent.parent / ".env")


def setup_logging(name: str, level: int = logging.INFO) -> logging.Logger:
    """構造化ロガーをセットアップ"""
    logger = logging.getLogger(name)
    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        formatter = logging.Formatter(
            "%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )
        handler.setFormatter(formatter)
        logger.addHandler(handler)
    logger.setLevel(level)
    return logger


def get_catalog() -> Catalog:
    """Polaris Iceberg REST Catalog への接続を返す"""
    return load_catalog(
        "polaris",
        **{
            "type": "rest",
            "uri": os.environ.get("POLARIS_URI", "http://localhost:8181/api/catalog"),
            "credential": os.environ.get("POLARIS_CREDENTIAL", "root:s3cr3t"),
            "scope": "PRINCIPAL_ROLE:ALL",
            "warehouse": os.environ.get("POLARIS_WAREHOUSE", "kensan-lakehouse"),
            "s3.endpoint": os.environ.get("S3_ENDPOINT", "http://localhost:9000"),
            "s3.access-key-id": os.environ.get("S3_ACCESS_KEY", "kensan"),
            "s3.secret-access-key": os.environ.get("S3_SECRET_KEY", "kensan-minio"),
            "s3.path-style-access": "true",
            "s3.region": "us-east-1",
        },
    )


def get_pg_dsn() -> str:
    """PostgreSQL接続文字列を返す"""
    host = os.environ.get("PG_HOST", "localhost")
    port = os.environ.get("PG_PORT", "5432")
    user = os.environ.get("PG_USER", "kensan")
    password = os.environ.get("PG_PASSWORD", "kensan")
    database = os.environ.get("PG_DATABASE", "kensan")
    return f"postgresql://{user}:{password}@{host}:{port}/{database}"
