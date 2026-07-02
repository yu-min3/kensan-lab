"""
Polaris Catalog Bootstrap: OAuth2トークン取得 → Catalog作成 → Principal設定

Polaris 起動後に1回実行。冪等: 既存 catalog/principal はスキップ。
"""

import json
import os
import sys

import requests

from catalog.config import setup_logging

logger = setup_logging("polaris.bootstrap")

POLARIS_MANAGEMENT_URL = os.environ.get(
    "POLARIS_MANAGEMENT_URL", "http://localhost:8181/api/management/v1"
)
POLARIS_CATALOG_URL = os.environ.get(
    "POLARIS_CATALOG_URL", "http://localhost:8181/api/catalog/v1"
)
ROOT_CLIENT_ID = os.environ.get("POLARIS_ROOT_CLIENT_ID", "root")
ROOT_CLIENT_SECRET = os.environ.get("POLARIS_ROOT_CLIENT_SECRET", "s3cr3t")

# MinIO / S3 settings
S3_ENDPOINT = os.environ.get("S3_ENDPOINT", "http://localhost:9000")
S3_BUCKET = os.environ.get("S3_BUCKET", "kensan-lakehouse")

CATALOG_NAME = os.environ.get("CATALOG_NAME", "kensan-lakehouse")


def get_token() -> str:
    """OAuth2 client_credentials でアクセストークンを取得"""
    resp = requests.post(
        f"{POLARIS_CATALOG_URL}/oauth/tokens",
        data={
            "grant_type": "client_credentials",
            "client_id": ROOT_CLIENT_ID,
            "client_secret": ROOT_CLIENT_SECRET,
            "scope": "PRINCIPAL_ROLE:ALL",
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"},
    )
    resp.raise_for_status()
    token = resp.json()["access_token"]
    logger.info("OAuth2 token acquired")
    return token


def create_catalog(token: str) -> None:
    """Catalog を作成 (MinIO をストレージに使用)"""
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    # Check if catalog already exists
    resp = requests.get(f"{POLARIS_MANAGEMENT_URL}/catalogs/{CATALOG_NAME}", headers=headers)
    if resp.status_code == 200:
        logger.info(f"Catalog '{CATALOG_NAME}' already exists, skipping")
        return

    payload = {
        "catalog": {
            "name": CATALOG_NAME,
            "type": "INTERNAL",
            "properties": {
                "default-base-location": f"s3://{S3_BUCKET}/",
            },
            "storageConfigInfo": {
                "storageType": "S3",
                "endpoint": S3_ENDPOINT,
                "endpointInternal": os.environ.get("S3_ENDPOINT_INTERNAL", S3_ENDPOINT.replace("localhost", "kensan-minio")),
                "pathStyleAccess": True,
            },
        }
    }

    resp = requests.post(
        f"{POLARIS_MANAGEMENT_URL}/catalogs",
        headers=headers,
        data=json.dumps(payload),
    )
    if resp.status_code == 409:
        logger.info(f"Catalog '{CATALOG_NAME}' already exists")
    else:
        resp.raise_for_status()
        logger.info(f"Catalog '{CATALOG_NAME}' created")


def grant_catalog_admin(token: str) -> None:
    """Root principal に catalog admin 権限を付与"""
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    # Assign catalog role to the root principal role
    resp = requests.put(
        f"{POLARIS_MANAGEMENT_URL}/catalogs/{CATALOG_NAME}/catalog-roles/catalog_admin/grants",
        headers=headers,
        data=json.dumps({
            "type": "catalog",
            "privilege": "CATALOG_MANAGE_CONTENT",
        }),
    )
    if resp.status_code in (200, 201, 204):
        logger.info("Catalog admin grants applied")
    else:
        logger.warning(f"Grant response: {resp.status_code} {resp.text}")


def main() -> None:
    logger.info("Bootstrapping Polaris catalog...")
    try:
        token = get_token()
        create_catalog(token)
        grant_catalog_admin(token)
        logger.info("Bootstrap complete.")
    except requests.RequestException as e:
        logger.error(f"Bootstrap failed: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
