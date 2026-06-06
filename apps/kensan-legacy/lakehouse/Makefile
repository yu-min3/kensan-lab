.PHONY: up down dremio-up dremio-down trino-up trino-down all-up all-down \
       openmetadata-up openmetadata-down \
       bootstrap init ingest transform aggregate pipeline query summary logs health install test \
       dagster-logs dagster-dev clean

COMMON       := -f docker-compose.common.yml
DREMIO       := $(COMMON) -f docker-compose.dremio.yml
TRINO        := $(COMMON) -f docker-compose.trino.yml
ALL          := $(COMMON) -f docker-compose.dremio.yml -f docker-compose.trino.yml
OPENMETADATA := -f docker-compose.openmetadata.yml

# =============================================================================
# Lakehouse Stack (requires: make up in root for MinIO)
# =============================================================================

## Base services are now in root docker-compose.yml
up:
	@echo "Polaris + Dagster are now managed by root docker-compose.yml."
	@echo "Use 'make up' in project root to start all services."

down:
	@echo "Polaris + Dagster are now managed by root docker-compose.yml."
	@echo "Use 'make down' in project root to stop all services."

## Pattern A: Base + Dremio
dremio-up:
	docker compose $(DREMIO) up -d
	@echo ""
	@echo "Polaris API:      http://localhost:8181"
	@echo "Dagster UI:       http://localhost:3070"
	@echo "Dremio UI:        http://localhost:9047"

dremio-down:
	docker compose $(DREMIO) down

## Pattern B: Base + Trino + Superset
trino-up:
	docker compose $(TRINO) up -d
	@echo ""
	@echo "Polaris API:      http://localhost:8181"
	@echo "Dagster UI:       http://localhost:3070"
	@echo "Trino:            http://localhost:8085"
	@echo "Superset:         http://localhost:8088 (admin / admin)"

trino-down:
	docker compose $(TRINO) down

## Pattern C: Base + Dremio + Trino + Superset (all query engines)
all-up:
	docker compose $(ALL) up -d
	@echo ""
	@echo "Polaris API:      http://localhost:8181"
	@echo "Dagster UI:       http://localhost:3070"
	@echo "Dremio UI:        http://localhost:9047"
	@echo "Trino:            http://localhost:8085"
	@echo "Superset:         http://localhost:8088 (admin / admin)"

all-down:
	docker compose $(ALL) down

## OpenMetadata (standalone - separate from query engine patterns)
openmetadata-up:
	docker compose $(OPENMETADATA) up -d
	@echo ""
	@echo "OpenMetadata UI:  http://localhost:8585 (admin@open-metadata.org / admin)"
	@echo "Airflow UI:       http://localhost:8080 (admin / admin)"

openmetadata-down:
	docker compose $(OPENMETADATA) down

# =============================================================================
# Health Check
# =============================================================================

health:
	@echo "=== MinIO (main stack) ==="
	@curl -sf http://localhost:9000/minio/health/live && echo " OK" || echo " FAIL"
	@echo "=== Polaris ==="
	@curl -sf http://localhost:8182/q/health > /dev/null && echo " OK" || echo " FAIL"

# =============================================================================
# Python Dependencies
# =============================================================================

install:
	uv sync

# =============================================================================
# Pipeline
# =============================================================================

bootstrap:
	uv run python -m catalog.bootstrap_polaris

init: bootstrap
	uv run python -m catalog.init_catalog

ingest:
	uv run python pipelines/bronze/ingest_postgres.py

transform:
	uv run python pipelines/silver/transform.py

aggregate:
	uv run python pipelines/gold/aggregate.py

pipeline: ingest transform aggregate
	@echo "Pipeline complete."

# =============================================================================
# Query
# =============================================================================

query:
	uv run python queries/query.py

summary:
	uv run python queries/query.py summary

# =============================================================================
# Test
# =============================================================================

test:
	uv run pytest tests/ -v

# =============================================================================
# Dagster
# =============================================================================

dagster-logs:
	docker logs -f kensan-dagster-webserver kensan-dagster-daemon kensan-dagster-user-code

dagster-dev:
	uv run dagster dev -p 3070 -m dagster_project

# =============================================================================
# Logs
# =============================================================================

logs:
	docker logs -f kensan-polaris kensan-dagster-webserver kensan-dagster-daemon kensan-dagster-user-code

# =============================================================================
# Clean
# =============================================================================

clean:
	@echo "Lakehouse volumes are managed by root docker-compose.yml."
	@echo "Use 'make clean' in project root to remove all volumes."

# =============================================================================
# Help
# =============================================================================

help:
	@echo "Lakehouse - Development Commands"
	@echo ""
	@echo "Core services (Polaris + Dagster) are managed by root docker-compose.yml."
	@echo "Use 'make up' / 'make down' in project root to start/stop them."
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Query Engines (overlay):"
	@echo "  dremio-up    Dremio"
	@echo "  trino-up     Trino + Superset"
	@echo "  all-up       Dremio + Trino + Superset"
	@echo ""
	@echo "  dremio-down  Stop Dremio"
	@echo "  trino-down   Stop Trino + Superset"
	@echo "  all-down     Stop all query engines"
	@echo ""
	@echo "  openmetadata-up    OpenMetadata + Airflow"
	@echo "  openmetadata-down  Stop OpenMetadata"
	@echo ""
	@echo "Pipeline:"
	@echo "  init         Initialize Iceberg catalog tables"
	@echo "  ingest       Bronze: ingest from PostgreSQL"
	@echo "  transform    Silver: clean and enrich"
	@echo "  aggregate    Gold: weekly aggregations"
	@echo "  pipeline     Run full pipeline (ingest -> transform -> aggregate)"
	@echo ""
	@echo "Query:"
	@echo "  query        Interactive DuckDB REPL"
	@echo "  summary      Run summary query"
	@echo ""
	@echo "Utilities:"
	@echo "  health       Check MinIO and Polaris health"
	@echo "  logs         View lakehouse logs"
	@echo "  dagster-logs View Dagster logs"
	@echo "  dagster-dev  Run Dagster dev server locally"
	@echo "  install      Install Python dependencies"
	@echo "  test         Run tests"
