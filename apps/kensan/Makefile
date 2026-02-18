.PHONY: up down build logs ps clean frontend backend db storage help dev dev-backend e2e-install e2e e2e-ui e2e-headed demo-seed demo-clean backup restore db-backup db-restore storage-backup storage-restore \
       lakehouse lakehouse-dremio lakehouse-trino lakehouse-all lakehouse-down openmetadata openmetadata-down \
       prod-up prod-down prod-logs prod-lakehouse deploy

# Default target
.DEFAULT_GOAL := help

# =============================================================================
# Full Stack Commands
# =============================================================================

## Start all services (frontend + backend + database + storage + lakehouse)
up:
	docker compose up -d --remove-orphans
	@echo ""
	@echo "🚀 Kensan is starting..."
	@echo ""
	@echo "Frontend:  http://localhost:5173"
	@echo "Services:"
	@echo "  - user-service:      http://localhost:8081/health"
	@echo "  - task-service:      http://localhost:8082/health"
	@echo "  - timeblock-service: http://localhost:8084/health"
	@echo "  - analytics-service: http://localhost:8088/health"
	@echo "  - ai-service:        http://localhost:8089/health"
	@echo "  - memo-service:      http://localhost:8090/health"
	@echo "  - note-service:      http://localhost:8091/health"
	@echo ""
	@echo "Database:  postgres://kensan:kensan@localhost:5432/kensan"
	@echo "Storage:   http://localhost:9000 (MinIO API)"
	@echo "           http://localhost:9001 (MinIO Console - kensan/kensan123)"
	@echo ""
	@echo "Lakehouse:"
	@echo "  - Polaris API:       http://localhost:8181"
	@echo "  - Polaris Health:    http://localhost:8182/q/health"
	@echo "  - Dagster UI:        http://localhost:3070"
	@echo ""
	@echo "Use 'make logs' to view logs"
	@echo "Use 'make down' to stop all services"

## Stop all services
down:
	docker compose down

## Build all images
build:
	docker compose build

## Rebuild and start all services
rebuild: build up

## View logs (all services)
logs:
	docker compose logs -f

## View logs for specific service (usage: make log SERVICE=frontend)
log:
	docker compose logs -f $(SERVICE)

## Show running containers
ps:
	docker compose ps

## Remove all containers, networks, and volumes
clean:
	docker compose down -v --remove-orphans
	docker system prune -f

# =============================================================================
# Selective Start Commands
# =============================================================================

## Start only frontend
frontend:
	docker compose up -d frontend
	@echo "Frontend: http://localhost:5173"

## Start only database
db:
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@echo "Database: postgres://kensan:kensan@localhost:5432/kensan"

## Start only storage (MinIO)
storage:
	docker compose up -d minio minio-init
	@echo "Waiting for MinIO to be ready..."
	@sleep 3
	@echo "Storage API:     http://localhost:9000"
	@echo "Storage Console: http://localhost:9001 (kensan/kensan123)"

## Start only backend services (requires db and storage)
backend: db storage
	docker compose up -d user-service task-service timeblock-service analytics-service ai-service memo-service note-service
	@echo "All backend services started"

# =============================================================================
# Development Commands
# =============================================================================

## Start in development mode with MSW mocking (frontend-only, no backend needed)
dev:
	@echo ""
	@echo "🔧 Development Mode (MSW Mocking)"
	@echo ""
	@echo "Starting frontend with MSW enabled..."
	@echo "All API requests will be mocked. No backend required."
	@echo ""
	cd frontend && npm run dev:mock

## Start backend services only (for local frontend development)
dev-backend: db backend
	@echo ""
	@echo "Backend services started. Now run 'npm run dev' for frontend."

# =============================================================================
# Production (GCE)
# =============================================================================
# ローカル (make up) は docker-compose.yml のみ → 各ポートが直接公開される
# 本番 (make prod-up) は docker-compose.prod.yml を overlay → nginx :443 経由、内部ポート非公開

## Start production stack (nginx + HTTPS, internal ports hidden)
prod-up:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
	@echo ""
	@echo "🚀 Kensan (production) is starting..."
	@echo "   App: https://kensan.yu-min3.com"
	@echo "   MinIO S3: port 9000"
	@echo ""
	@echo "All other ports are internal only (nginx proxy)."

## Stop production stack
prod-down:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml down

## View production logs
prod-logs:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f

## Initialize lakehouse catalog (production)
prod-lakehouse:
	POLARIS_URI=http://localhost:8181/api/catalog S3_ENDPOINT=http://kensan-minio:9000 $(MAKE) -C lakehouse init

## Deploy to GCE
## Required env vars:
##   JWT_SECRET     - JWT signing secret for auth services
##   GOOGLE_API_KEY - Google AI API key for ai-service (Gemini)
## Optional env vars:
##   GOOGLE_MODEL   - AI model name (default: gemini-2.0-flash)
## Example: JWT_SECRET=xxx GOOGLE_API_KEY=xxx make deploy
deploy:
	@bash scripts/gce-deploy.sh

# =============================================================================
# Lakehouse (delegates to lakehouse/Makefile, requires app stack running)
# =============================================================================

## Initialize Iceberg catalog (services started by 'make up')
lakehouse:
	@echo "Waiting for Polaris to be healthy..."
	@for i in $$(seq 1 20); do \
		curl -sf http://localhost:8182/q/health > /dev/null 2>&1 && break; \
		sleep 3; \
	done
	@curl -sf http://localhost:8182/q/health > /dev/null 2>&1 || { echo "ERROR: Polaris did not become healthy within 60s"; exit 1; }
	@echo "Polaris is healthy."
	@echo ""
	@echo "Initializing Iceberg catalog..."
	POLARIS_URI=http://localhost:8181/api/catalog S3_ENDPOINT=http://kensan-minio:9000 $(MAKE) -C lakehouse init
	@echo ""
	@echo "Lakehouse catalog initialized!"
	@echo "  Polaris API:      http://localhost:8181"
	@echo "  Dagster UI:       http://localhost:3070"
	@echo ""
	@echo "AI Explorer data will appear after Dagster pipelines run."

## Start lakehouse + Dremio
lakehouse-dremio:
	$(MAKE) -C lakehouse dremio-up

## Start lakehouse + Trino + Superset
lakehouse-trino:
	$(MAKE) -C lakehouse trino-up

## Start lakehouse + all query engines
lakehouse-all:
	$(MAKE) -C lakehouse all-up

## Stop all lakehouse services (included in 'make down')
lakehouse-down:
	@echo "Lakehouse services are managed by root docker-compose.yml."
	@echo "Use 'make down' to stop all services."

## Start OpenMetadata
openmetadata:
	$(MAKE) -C lakehouse openmetadata-up

## Stop OpenMetadata
openmetadata-down:
	$(MAKE) -C lakehouse openmetadata-down

# =============================================================================
# Health Check
# =============================================================================

## Check health of all services
health:
	@echo "Checking service health..."
	@echo ""
	@curl -s http://localhost:8081/health 2>/dev/null | jq . || echo "user-service: DOWN"
	@curl -s http://localhost:8082/health 2>/dev/null | jq . || echo "task-service: DOWN"
	@curl -s http://localhost:8084/health 2>/dev/null | jq . || echo "timeblock-service: DOWN"
	@curl -s http://localhost:8088/health 2>/dev/null | jq . || echo "analytics-service: DOWN"
	@curl -s http://localhost:8089/health 2>/dev/null | jq . || echo "ai-service: DOWN"
	@curl -s http://localhost:8090/health 2>/dev/null | jq . || echo "memo-service: DOWN"
	@curl -s http://localhost:8091/health 2>/dev/null | jq . || echo "note-service: DOWN"
	@echo ""
	@echo "Lakehouse:"
	@curl -sf http://localhost:8182/q/health > /dev/null 2>&1 && echo "  polaris: UP" || echo "  polaris: DOWN"
	@curl -sf http://localhost:3070 > /dev/null 2>&1 && echo "  dagster-webserver: UP" || echo "  dagster-webserver: DOWN"

# =============================================================================
# Help
# =============================================================================

# =============================================================================
# E2E Testing (Playwright)
# =============================================================================

## Install Playwright browsers (chromium)
e2e-install:
	cd frontend && npx playwright install chromium

## Run E2E tests
e2e:
	cd frontend && npx playwright test --config=../e2e/playwright.config.ts

## Run E2E tests in UI mode
e2e-ui:
	cd frontend && npx playwright test --config=../e2e/playwright.config.ts --ui

## Run E2E tests headed (visible browser)
e2e-headed:
	cd frontend && npx playwright test --config=../e2e/playwright.config.ts --headed

# =============================================================================
# Demo Data
# =============================================================================

## Apply demo seed data (Tanaka Shota persona)
demo-seed:
	@bash backend/migrations-v2/apply.sh tanaka_shota

## Remove demo seed data only
demo-clean:
	@echo "Removing demo user data..."
	@docker exec -i kensan-postgres psql -U kensan -d kensan < backend/migrations-v2/seeds/tanaka_shota/000_cleanup.sql
	@echo "Demo data removed."

# =============================================================================
# Database Backup / Restore
# =============================================================================

BACKUP_DIR := backups
BACKUP_FILE := $(BACKUP_DIR)/kensan_$(shell date +%Y%m%d_%H%M%S).sql

## Backup database + MinIO storage
backup: db-backup storage-backup

## Restore database + MinIO storage
restore: db-restore storage-restore

## Backup database to backups/ directory
db-backup:
	@mkdir -p $(BACKUP_DIR)
	@echo "Backing up database..."
	@docker exec kensan-postgres pg_dump -U kensan -d kensan --clean --if-exists > $(BACKUP_FILE)
	@echo "Saved to $(BACKUP_FILE) ($$(du -h $(BACKUP_FILE) | cut -f1))"

## Restore database from backup (usage: make db-restore FILE=backups/kensan_xxx.sql)
db-restore:
ifndef FILE
	@echo "Usage: make db-restore FILE=backups/kensan_20260203_120000.sql"
	@echo ""
	@echo "Available backups:"
	@ls -1t $(BACKUP_DIR)/*.sql 2>/dev/null || echo "  (none)"
	@exit 1
endif
	@echo "Restoring from $(FILE)..."
	@docker exec -i kensan-postgres psql -U kensan -d kensan < $(FILE)
	@echo "Restore complete."

STORAGE_BACKUP_DIR := $(BACKUP_DIR)/minio

## Backup MinIO storage to backups/minio/
storage-backup:
	@mkdir -p $(STORAGE_BACKUP_DIR)
	@echo "Backing up MinIO buckets..."
	@docker exec kensan-minio mc alias set local http://localhost:9000 kensan kensan-minio 2>/dev/null
	@docker exec kensan-minio mkdir -p /tmp/minio-backup/kensan-notes
	@docker exec kensan-minio mc mirror --overwrite local/kensan-notes /tmp/minio-backup/kensan-notes
	@docker cp kensan-minio:/tmp/minio-backup/kensan-notes $(STORAGE_BACKUP_DIR)/
	@docker exec kensan-minio rm -rf /tmp/minio-backup
	@echo "Saved to $(STORAGE_BACKUP_DIR)/kensan-notes ($$(du -sh $(STORAGE_BACKUP_DIR)/kensan-notes | cut -f1))"

## Restore MinIO storage from backup
storage-restore:
	@if [ ! -d "$(STORAGE_BACKUP_DIR)/kensan-notes" ]; then echo "No backup found at $(STORAGE_BACKUP_DIR)/kensan-notes"; exit 1; fi
	@echo "Restoring MinIO buckets..."
	@docker cp $(STORAGE_BACKUP_DIR)/kensan-notes kensan-minio:/tmp/minio-restore/kensan-notes
	@docker exec kensan-minio mc alias set local http://localhost:9000 kensan kensan-minio 2>/dev/null
	@docker exec kensan-minio mc mirror --overwrite /tmp/minio-restore/kensan-notes local/kensan-notes
	@docker exec kensan-minio rm -rf /tmp/minio-restore
	@echo "Restore complete."

# =============================================================================
# Help
# =============================================================================

## Show this help
help:
	@echo "Kensan - Development Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Full Stack:"
	@echo "  up        Start all services (frontend + backend + db + storage + lakehouse)"
	@echo "  down      Stop all services"
	@echo "  build     Build all Docker images"
	@echo "  rebuild   Rebuild and start all services"
	@echo "  logs      View logs for all services"
	@echo "  ps        Show running containers"
	@echo "  clean     Remove all containers and volumes"
	@echo ""
	@echo "Selective Start:"
	@echo "  frontend  Start only frontend"
	@echo "  backend   Start database + storage + all backend services"
	@echo "  db        Start only database"
	@echo "  storage   Start only storage (MinIO)"
	@echo ""
	@echo "Development:"
	@echo "  dev           Start frontend with MSW mocking (npm, no backend)"
	@echo "  dev-backend   Start backend services for local frontend"
	@echo ""
	@echo "E2E Testing:"
	@echo "  e2e-install  Install Playwright browsers (chromium)"
	@echo "  e2e          Run E2E tests"
	@echo "  e2e-ui       Run E2E tests in UI mode"
	@echo "  e2e-headed   Run E2E tests headed (visible browser)"
	@echo ""
	@echo "Demo Data:"
	@echo "  demo-seed   Apply demo seed data (Tanaka Shota persona)"
	@echo "  demo-clean  Remove demo seed data only"
	@echo ""
	@echo "Backup:"
	@echo "  backup                 Backup database + MinIO storage"
	@echo "  restore FILE=x.sql    Restore database + MinIO storage"
	@echo "  db-backup              Backup database only"
	@echo "  db-restore FILE=x.sql  Restore database only"
	@echo "  storage-backup         Backup MinIO buckets to backups/minio/"
	@echo "  storage-restore        Restore MinIO buckets from backup"
	@echo ""
	@echo "Lakehouse:"
	@echo "  lakehouse         Initialize Iceberg catalog (services started by 'make up')"
	@echo "  lakehouse-dremio  Dremio query engine"
	@echo "  lakehouse-trino   Trino + Superset query engine"
	@echo "  lakehouse-all     All query engines"
	@echo "  openmetadata      OpenMetadata + Airflow"
	@echo "  openmetadata-down Stop OpenMetadata"
	@echo ""
	@echo "Production (GCE):"
	@echo "  prod-up        Start with nginx reverse proxy (HTTPS, ports hidden)"
	@echo "  prod-down      Stop production stack"
	@echo "  prod-logs      View production logs"
	@echo "  prod-lakehouse Start lakehouse (prod, ports hidden)"
	@echo "  deploy         Deploy to GCE via SSH (requires JWT_SECRET, GOOGLE_API_KEY)"
	@echo ""
	@echo "Utilities:"
	@echo "  health    Check health of all services"
	@echo "  log SERVICE=x  View logs for specific service"
