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

# Database host (localhost en dev, configurable via env pour le VPS)
DB_HOST="${DB_HOST:-localhost}"

# Service -> database URL (construites à partir des variables .env)
declare -A SERVICE_DB_URLS
SERVICE_DB_URLS["auth-service"]="postgresql://${POSTGRES_AUTH_USER:-dev}:${POSTGRES_AUTH_PASSWORD:-dev123}@${DB_HOST}:5433/${POSTGRES_AUTH_DB:-auth_db}?sslmode=disable"
SERVICE_DB_URLS["file-service"]="postgresql://${POSTGRES_FILE_USER:-dev}:${POSTGRES_FILE_PASSWORD:-dev123}@${DB_HOST}:5434/${POSTGRES_FILE_DB:-file_db}?sslmode=disable"
SERVICE_DB_URLS["rating-service"]="postgresql://${POSTGRES_RATING_USER:-dev}:${POSTGRES_RATING_PASSWORD:-dev123}@${DB_HOST}:5435/${POSTGRES_RATING_DB:-rating_db}?sslmode=disable"
SERVICE_DB_URLS["vehicle-service"]="postgresql://${POSTGRES_VEHICLE_USER:-dev}:${POSTGRES_VEHICLE_PASSWORD:-dev123}@${DB_HOST}:5436/${POSTGRES_VEHICLE_DB:-vehicle_db}?sslmode=disable"
SERVICE_DB_URLS["trips-service"]="postgresql://${POSTGRES_TRIPS_USER:-dev}:${POSTGRES_TRIPS_PASSWORD:-dev123}@${DB_HOST}:5437/${POSTGRES_TRIPS_DB:-trips_db}?sslmode=disable"
SERVICE_DB_URLS["booking-service"]="postgresql://${POSTGRES_BOOKING_USER:-dev}:${POSTGRES_BOOKING_PASSWORD:-dev123}@${DB_HOST}:5438/${POSTGRES_BOOKING_DB:-booking_db}?sslmode=disable"
SERVICE_DB_URLS["payment-service"]="postgresql://${POSTGRES_PAYMENT_USER:-dev}:${POSTGRES_PAYMENT_PASSWORD:-dev123}@${DB_HOST}:5439/${POSTGRES_PAYMENT_DB:-payment_db}?sslmode=disable"
SERVICE_DB_URLS["notification-service"]="postgresql://${POSTGRES_NOTIFICATION_USER:-dev}:${POSTGRES_NOTIFICATION_PASSWORD:-dev123}@${DB_HOST}:5440/${POSTGRES_NOTIFICATION_DB:-notification_db}?sslmode=disable"
SERVICE_DB_URLS["support-service"]="postgresql://${POSTGRES_SUPPORT_USER:-dev}:${POSTGRES_SUPPORT_PASSWORD:-dev123}@${DB_HOST}:5441/${POSTGRES_SUPPORT_DB:-support_db}?sslmode=disable"

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
