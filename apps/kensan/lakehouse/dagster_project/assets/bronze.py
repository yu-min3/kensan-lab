"""
Bronze Assets: PostgreSQL → Iceberg Bronze層
既存の ingest_postgres.py をラップ
"""

from dagster import AssetExecutionContext, MaterializeResult, MetadataValue, asset

from dagster_project.resources import IcebergCatalogResource, PostgresDsnResource
from pipelines.bronze.ingest_postgres import TABLES, ingest_table, load_state, save_state


def _make_bronze_asset(table_name: str, config: dict):
    """Bronze アセットを動的生成するファクトリ"""
    # "bronze.time_entries_raw" → "bronze_time_entries_raw"
    asset_name = table_name.replace(".", "_")

    @asset(
        name=asset_name,
        group_name="bronze",
        compute_kind="postgres_ingestion",
    )
    def _asset(
        context: AssetExecutionContext,
        iceberg_catalog: IcebergCatalogResource,
        pg_dsn: PostgresDsnResource,
    ):
        catalog = iceberg_catalog.get_catalog()
        dsn = pg_dsn.get_dsn()
        state = load_state()

        result = ingest_table(catalog, dsn, table_name, config, state)
        save_state(result)

        since_column = config.get("since_column", "updated_at")
        last_ts = result.get(table_name, "N/A")

        return MaterializeResult(
            metadata={
                "table": MetadataValue.text(table_name),
                "mode": MetadataValue.text(
                    "append" if since_column == "created_at" else "overwrite"
                ),
                "max_timestamp": MetadataValue.text(str(last_ts)),
            }
        )

    return _asset


# 9 Bronze Assets を生成
bronze_assets = [_make_bronze_asset(name, cfg) for name, cfg in TABLES.items()]
