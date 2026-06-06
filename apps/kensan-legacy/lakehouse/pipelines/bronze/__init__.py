"""Bronze Layer: Raw data ingestion from PostgreSQL"""

from pipelines.bronze.ingest_postgres import main as ingest_main

__all__ = ["ingest_main"]
