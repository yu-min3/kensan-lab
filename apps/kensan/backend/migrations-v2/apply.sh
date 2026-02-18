#!/bin/bash
# ============================================================================
# migrations-v2: Apply consolidated schema + persona seeds
# ============================================================================
# Usage:
#   ./backend/migrations-v2/apply.sh                    # Schema + master only
#   ./backend/migrations-v2/apply.sh tanaka_shota       # + Tanaka persona
#   ./backend/migrations-v2/apply.sh all                # + All 4 personas
#   ./backend/migrations-v2/apply.sh tanaka_shota suzuki_misaki  # Multiple
#
# Safe to run multiple times (cleanup runs first per persona).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONTAINER="kensan-postgres"
DB_USER="kensan"
DB_NAME="kensan"

PERSONAS=(tanaka_shota suzuki_misaki yamada_takuya takahashi_aya)

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo ""
echo "=== Kensan migrations-v2 ==="
echo ""

# Check if container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    echo -e "${RED}ERROR${NC}: Container '${CONTAINER}' is not running."
    echo "Run 'make up' or 'make db' first."
    exit 1
fi

run_sql() {
    local file="$1"
    local filename
    filename=$(basename "$file")
    echo -e "  ${GREEN}✓${NC} ${filename}"
    docker exec -i "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -q < "$file" 2>&1 | grep -v "^$" || true
}

# ------------------------------------------------------------------
# 1. Apply schema
# ------------------------------------------------------------------
echo "📦 Applying schema..."
run_sql "${SCRIPT_DIR}/001_init.sql"

# ------------------------------------------------------------------
# 2. Apply master data
# ------------------------------------------------------------------
echo "📦 Applying master data..."
run_sql "${SCRIPT_DIR}/002_master.sql"

# ------------------------------------------------------------------
# 3. Apply persona seeds (if requested)
# ------------------------------------------------------------------
requested=("$@")

# Expand "all" to all personas
if [[ "${#requested[@]}" -gt 0 && "${requested[0]}" == "all" ]]; then
    requested=("${PERSONAS[@]}")
fi

if [[ "${#requested[@]}" -eq 0 ]]; then
    echo ""
    echo "No persona specified. Schema and master data applied."
    echo ""
    echo "Available personas:"
    for p in "${PERSONAS[@]}"; do
        echo "  - ${p}"
    done
    echo ""
    echo "Usage: $0 <persona> [persona2 ...]"
    echo "       $0 all"
    exit 0
fi

for persona in "${requested[@]}"; do
    persona_dir="${SCRIPT_DIR}/seeds/${persona}"

    if [[ ! -d "$persona_dir" ]]; then
        echo -e "${RED}ERROR${NC}: Unknown persona '${persona}'"
        echo "Available: ${PERSONAS[*]}"
        exit 1
    fi

    echo ""
    echo "👤 Applying persona: ${persona}"

    for f in "${persona_dir}"/0*.sql; do
        [[ -f "$f" ]] || continue
        run_sql "$f"
    done
done

echo ""
echo "=== Done ==="
echo ""

# Show login info for applied personas
for persona in "${requested[@]}"; do
    case "$persona" in
        tanaka_shota)   echo "  Login: demo@kensan.dev / demo1234  (田中翔太)" ;;
        suzuki_misaki)  echo "  Login: misaki@kensan.dev / demo1234  (鈴木美咲)" ;;
        yamada_takuya)  echo "  Login: takuya@kensan.dev / demo1234  (山田拓也)" ;;
        takahashi_aya)  echo "  Login: aya@kensan.dev / demo1234  (高橋彩)" ;;
    esac
done
echo ""
