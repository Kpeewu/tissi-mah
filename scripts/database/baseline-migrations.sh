#!/bin/bash
# scripts/database/baseline-migrations.sh
# Script de transition : initialise schema_migrations pour les bases VPS existantes.
#
# A exécuter UNE SEULE FOIS avant le premier deploiement avec initContainers.
# Il force le numero de version dans schema_migrations sans executer de SQL,
# indiquant a golang-migrate que toutes les migrations jusqu'a cette version
# sont deja appliquees.
#
# Usage:
#   ./scripts/database/baseline-migrations.sh
#   SUPPORT_DATABASE_URL=... ./scripts/database/baseline-migrations.sh support-service

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Baseline Migrations - tissiMah (one-time)         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Load environment
if [ -f "$ROOT_DIR/.env" ]; then
    export $(cat "$ROOT_DIR/.env" | grep -v '^#' | xargs)
fi

# Check if migrate is installed
if ! command -v migrate &> /dev/null; then
    echo -e "${RED}golang-migrate not found!${NC}"
    echo "Install: brew install golang-migrate"
    exit 1
fi

# Service -> baseline version (current latest migration number)
declare -A BASELINE_VERSIONS
BASELINE_VERSIONS["auth-service"]=1
BASELINE_VERSIONS["file-service"]=2
BASELINE_VERSIONS["rating-service"]=2
BASELINE_VERSIONS["vehicle-service"]=1
BASELINE_VERSIONS["trips-service"]=7
BASELINE_VERSIONS["booking-service"]=5
BASELINE_VERSIONS["payment-service"]=2
BASELINE_VERSIONS["notification-service"]=4
BASELINE_VERSIONS["support-service"]=2

# Service -> database URL
declare -A SERVICE_DB_URLS
SERVICE_DB_URLS["auth-service"]="${AUTH_DATABASE_URL:-postgresql://dev:dev123@localhost:5433/auth_db?sslmode=disable}"
SERVICE_DB_URLS["file-service"]="${FILE_DATABASE_URL:-postgresql://dev:dev123@localhost:5434/file_db?sslmode=disable}"
SERVICE_DB_URLS["rating-service"]="${RATING_DATABASE_URL:-postgresql://dev:dev123@localhost:5435/rating_db?sslmode=disable}"
SERVICE_DB_URLS["vehicle-service"]="${VEHICLE_DATABASE_URL:-postgresql://dev:dev123@localhost:5436/vehicle_db?sslmode=disable}"
SERVICE_DB_URLS["trips-service"]="${TRIPS_DATABASE_URL:-postgresql://dev:dev123@localhost:5437/trips_db?sslmode=disable}"
SERVICE_DB_URLS["booking-service"]="${BOOKING_DATABASE_URL:-postgresql://dev:dev123@localhost:5438/booking_db?sslmode=disable}"
SERVICE_DB_URLS["payment-service"]="${PAYMENT_DATABASE_URL:-postgresql://dev:dev123@localhost:5439/payment_db?sslmode=disable}"
SERVICE_DB_URLS["notification-service"]="${NOTIFICATION_DATABASE_URL:-postgresql://dev:dev123@localhost:5440/notification_db?sslmode=disable}"
SERVICE_DB_URLS["support-service"]="${SUPPORT_DATABASE_URL:-postgresql://dev:dev123@localhost:5441/support_db?sslmode=disable}"

# Determine which services to process
if [ -n "$1" ]; then
    SERVICES=("$1")
else
    SERVICES=("auth-service" "file-service" "rating-service" "vehicle-service" "trips-service" "booking-service" "payment-service" "notification-service" "support-service")
fi

for service in "${SERVICES[@]}"; do
    version="${BASELINE_VERSIONS[$service]}"
    db_url="${SERVICE_DB_URLS[$service]}"
    migrations_dir="$ROOT_DIR/services/$service/migrations"

    if [ -z "$version" ]; then
        echo -e "${RED}Unknown service: $service${NC}"
        continue
    fi

    echo -e "${BLUE}━━━ $service ━━━${NC}"

    # Check if schema_migrations already exists
    current_version=$(migrate -path "$migrations_dir" -database "$db_url" version 2>&1) && has_version=true || has_version=false

    if [ "$has_version" = true ]; then
        echo -e "  ${GREEN}Already at version $current_version — skipping${NC}"
    else
        echo -e "  ${YELLOW}No schema_migrations found — baselining to version $version...${NC}"
        migrate -path "$migrations_dir" -database "$db_url" force "$version"
        echo -e "  ${GREEN}Baselined to version $version${NC}"
    fi
    echo ""
done

echo -e "${GREEN}Baseline complete.${NC}"
echo "Les prochains deploiements appliqueront uniquement les nouvelles migrations."
