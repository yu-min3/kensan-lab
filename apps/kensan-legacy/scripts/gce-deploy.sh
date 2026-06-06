#!/bin/bash
set -euo pipefail

# =============================================================================
# Kensan GCE Deploy Script
# Deploys to existing GCE instance via SSH
#
# Required env vars:
#   JWT_SECRET     - JWT signing secret for auth services
#   GOOGLE_API_KEY - Google AI API key for ai-service (Gemini)
#
# Optional env vars:
#   GOOGLE_MODEL   - AI model name (default: gemini-2.0-flash)
#
# Usage:
#   JWT_SECRET=xxx GOOGLE_API_KEY=xxx make deploy
# =============================================================================

INSTANCE_NAME="kensan-app"
ZONE="asia-northeast1-a"
REPO_URL="https://github.com/yu-min3/kensan-mockup.git"
BRANCH="hackathon/gch4"

# === Colors ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# === Pre-flight checks ===
command -v gcloud >/dev/null 2>&1 || error "gcloud CLI not found"

# === Step 1: Firewall rules ===
info "Step 1: Configuring firewall rules (port 80, 9000)"
gcloud compute firewall-rules create allow-kensan-prod \
  --allow tcp:80,tcp:9000 \
  --target-tags=http-server \
  --description="Kensan production: nginx(80) + MinIO S3(9000)" \
  2>/dev/null || warn "Firewall rule already exists"

# === Step 2: Get external IP ===
info "Step 2: Getting external IP"
GCE_IP=$(gcloud compute instances describe "$INSTANCE_NAME" \
  --zone="$ZONE" \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)')
[ -z "$GCE_IP" ] && error "Could not get external IP for $INSTANCE_NAME"
info "External IP: $GCE_IP"

# === Step 3: Check required env vars ===
if [ -z "${JWT_SECRET:-}" ] || [ -z "${GOOGLE_API_KEY:-}" ]; then
  warn "JWT_SECRET and GOOGLE_API_KEY must be set"
  warn "Example: JWT_SECRET=xxx GOOGLE_API_KEY=xxx bash scripts/gce-deploy.sh"
  read -rp "Continue anyway? (y/N): " confirm
  [ "$confirm" != "y" ] && exit 1
fi

# === Step 4: Remote setup via SSH ===
info "Step 3: Deploying to GCE instance"
gcloud compute ssh "$INSTANCE_NAME" --zone="$ZONE" --command="
set -euo pipefail

echo '=== Installing Docker (if needed) ==='
if ! command -v docker &>/dev/null; then
  sudo apt-get update
  sudo apt-get install -y ca-certificates curl
  sudo install -m 0755 -d /etc/apt/keyrings
  sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  sudo chmod a+r /etc/apt/keyrings/docker.asc
  echo \"deb [arch=\$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \$(. /etc/os-release && echo \$VERSION_CODENAME) stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt-get update
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
  sudo usermod -aG docker \$USER
  echo 'Docker installed. You may need to re-run this script for group changes.'
fi

echo '=== Cloning/updating repository ==='
if [ -d ~/kensan-mockup ]; then
  cd ~/kensan-mockup
  git fetch origin
  git checkout $BRANCH
  git reset --hard origin/$BRANCH
else
  git clone -b $BRANCH $REPO_URL ~/kensan-mockup
  cd ~/kensan-mockup
fi

echo '=== Creating .env ==='
cat > .env << ENVEOF
GCE_IP=$GCE_IP
JWT_SECRET=${JWT_SECRET:-dev-secret-key-change-in-production}
GOOGLE_API_KEY=${GOOGLE_API_KEY:-}
GOOGLE_MODEL=${GOOGLE_MODEL:-gemini-2.0-flash}
ENVEOF

echo '=== Creating lakehouse/.env (Docker-internal hostnames) ==='
cp .env lakehouse/.env
cat >> lakehouse/.env << ENVEOF
POLARIS_URI=http://kensan-polaris:8181/api/catalog
S3_ENDPOINT=http://kensan-minio:9000
LOKI_URL=http://kensan-loki:3100
KENSAN_AI_URL=http://kensan-ai-service:8089
ENVEOF

echo '=== Starting main application ==='
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build

echo '=== Waiting for services to be healthy ==='
sleep 10

echo '=== Applying database migrations ==='
for f in backend/migrations-v2/[0-9]*.sql; do
  echo \"  Applying: \$(basename \$f)\"
  sudo docker exec -i kensan-postgres psql -U kensan -d kensan < \"\$f\" 2>&1 || true
done

echo '=== Reloading nginx (refresh upstream DNS) ==='
sudo docker exec kensan-nginx nginx -s reload

echo '=== Waiting for Polaris to be healthy ==='
for i in {1..20}; do
  if sudo docker exec kensan-polaris curl -sf http://localhost:8182/q/health > /dev/null 2>&1; then
    echo 'Polaris is healthy.'
    break
  fi
  echo \"Waiting for Polaris... (\$i/20)\"
  sleep 3
done

echo '=== Bootstrapping & initializing Iceberg catalog ==='
sudo docker exec -e POLARIS_MANAGEMENT_URL=http://kensan-polaris:8181/api/management/v1 \
  -e POLARIS_CATALOG_URL=http://kensan-polaris:8181/api/catalog/v1 \
  -e S3_ENDPOINT=http://kensan-minio:9000 \
  kensan-dagster-user-code uv run python -m catalog.bootstrap_polaris
sudo docker exec -e POLARIS_URI=http://kensan-polaris:8181/api/catalog \
  -e S3_ENDPOINT=http://kensan-minio:9000 \
  kensan-dagster-user-code uv run python -m catalog.init_catalog

echo '=== Demo seed is applied on first demo-login (no manual trigger needed) ==='

echo ''
echo '=== Deployment complete ==='
sudo docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
"

# === Step 5: Summary ===
echo ""
info "=== Deployment Complete ==="
info "Application: http://$GCE_IP"
info "MinIO S3:    http://$GCE_IP:9000"
info ""
info "Test login: test@kensan.dev / password123"
info ""
info "To SSH: gcloud compute ssh $INSTANCE_NAME --zone=$ZONE"
