"""Lakehouse integration for kensan-ai.

Fire & forget writes to Iceberg Bronze tables via Polaris catalog.
Read from Gold/Silver layers for prompt enrichment and Explorer data.
"""

from kensan_ai.lakehouse.reader import LakehouseReader, get_reader
from kensan_ai.lakehouse.writer import LakehouseWriter, get_writer

__all__ = ["LakehouseReader", "LakehouseWriter", "get_reader", "get_writer"]
