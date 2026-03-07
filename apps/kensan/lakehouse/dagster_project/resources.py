"""
Dagster Resources: Iceberg Catalog, PostgreSQL DSN, Loki, kensan-ai をラップ
"""

import os

from dagster import ConfigurableResource

from catalog.config import get_catalog, get_catalog_for_env, get_lakehouse_envs, get_pg_dsn


class IcebergCatalogResource(ConfigurableResource):
    """Polaris Iceberg REST Catalog リソース"""

    def get_catalog(self):
        return get_catalog()

    def get_catalogs(self) -> dict:
        """全環境のカタログを返す: {"dev": Catalog, "prod": Catalog}"""
        return {env: get_catalog_for_env(env) for env in get_lakehouse_envs()}


class PostgresDsnResource(ConfigurableResource):
    """PostgreSQL 接続文字列リソース"""

    def get_dsn(self) -> str:
        return get_pg_dsn()


class LokiResource(ConfigurableResource):
    """Loki API リソース (env: LOKI_URL)"""

    base_url: str = os.environ.get("LOKI_URL", "http://localhost:3100")

    def get_base_url(self) -> str:
        return self.base_url


class KensanAiResource(ConfigurableResource):
    """kensan-ai API リソース (env: KENSAN_AI_URL)"""

    base_url: str = os.environ.get("KENSAN_AI_URL", "http://localhost:8089")

    def get_base_url(self) -> str:
        return self.base_url
