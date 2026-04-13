#!/bin/bash
# scripts/database/migrate-all-services.sh
# Run database migrations for all Tissi-Mah services
#
# Supports:
#   - auth-service (PostgreSQL)
#   - client-service (PostgreSQL) - future
#   - user-service (MongoDB) - future

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        Database Migrations - tissiMah                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# Configuration
# =============================================================================

# Service database configurations
declare -A SERVICE_DB_URLS
declare -A SERVICE_DB_TYPES
declare -A SERVICE_MIGRATIONS_DIRS

# Database host (localhost en dev, configurable via env pour le VPS)
DB_HOST="${DB_HOST:-localhost}"

# Auth Service (PostgreSQL — port 5433)
SERVICE_DB_URLS["auth-service"]="postgresql://${POSTGRES_AUTH_USER:-dev}:${POSTGRES_AUTH_PASSWORD:-dev123}@${DB_HOST}:5433/${POSTGRES_AUTH_DB:-auth_db}?sslmode=disable"
SERVICE_DB_TYPES["auth-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["auth-service"]="$ROOT_DIR/services/auth-service/migrations"

# File Service (PostgreSQL — port 5434)
SERVICE_DB_URLS["file-service"]="postgresql://${POSTGRES_FILE_USER:-dev}:${POSTGRES_FILE_PASSWORD:-dev123}@${DB_HOST}:5434/${POSTGRES_FILE_DB:-file_db}?sslmode=disable"
SERVICE_DB_TYPES["file-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["file-service"]="$ROOT_DIR/services/file-service/migrations"

# Rating Service (PostgreSQL — port 5435)
SERVICE_DB_URLS["rating-service"]="postgresql://${POSTGRES_RATING_USER:-dev}:${POSTGRES_RATING_PASSWORD:-dev123}@${DB_HOST}:5435/${POSTGRES_RATING_DB:-rating_db}?sslmode=disable"
SERVICE_DB_TYPES["rating-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["rating-service"]="$ROOT_DIR/services/rating-service/migrations"

# Vehicle Service (PostgreSQL — port 5436)
SERVICE_DB_URLS["vehicle-service"]="postgresql://${POSTGRES_VEHICLE_USER:-dev}:${POSTGRES_VEHICLE_PASSWORD:-dev123}@${DB_HOST}:5436/${POSTGRES_VEHICLE_DB:-vehicle_db}?sslmode=disable"
SERVICE_DB_TYPES["vehicle-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["vehicle-service"]="$ROOT_DIR/services/vehicle-service/migrations"

# Trips Service (PostgreSQL/PostGIS — port 5437)
SERVICE_DB_URLS["trips-service"]="postgresql://${POSTGRES_TRIPS_USER:-dev}:${POSTGRES_TRIPS_PASSWORD:-dev123}@${DB_HOST}:5437/${POSTGRES_TRIPS_DB:-trips_db}?sslmode=disable"
SERVICE_DB_TYPES["trips-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["trips-service"]="$ROOT_DIR/services/trips-service/migrations"

# Booking Service (PostgreSQL — port 5438)
SERVICE_DB_URLS["booking-service"]="postgresql://${POSTGRES_BOOKING_USER:-dev}:${POSTGRES_BOOKING_PASSWORD:-dev123}@${DB_HOST}:5438/${POSTGRES_BOOKING_DB:-booking_db}?sslmode=disable"
SERVICE_DB_TYPES["booking-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["booking-service"]="$ROOT_DIR/services/booking-service/migrations"

# Payment Service (PostgreSQL — port 5439)
SERVICE_DB_URLS["payment-service"]="postgresql://${POSTGRES_PAYMENT_USER:-dev}:${POSTGRES_PAYMENT_PASSWORD:-dev123}@${DB_HOST}:5439/${POSTGRES_PAYMENT_DB:-payment_db}?sslmode=disable"
SERVICE_DB_TYPES["payment-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["payment-service"]="$ROOT_DIR/services/payment-service/migrations"

# Notification Service (PostgreSQL — port 5440)
SERVICE_DB_URLS["notification-service"]="postgresql://${POSTGRES_NOTIFICATION_USER:-dev}:${POSTGRES_NOTIFICATION_PASSWORD:-dev123}@${DB_HOST}:5440/${POSTGRES_NOTIFICATION_DB:-notification_db}?sslmode=disable"
SERVICE_DB_TYPES["notification-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["notification-service"]="$ROOT_DIR/services/notification-service/migrations"

# Support Service (PostgreSQL — port 5441)
SERVICE_DB_URLS["support-service"]="postgresql://${POSTGRES_SUPPORT_USER:-dev}:${POSTGRES_SUPPORT_PASSWORD:-dev123}@${DB_HOST}:5441/${POSTGRES_SUPPORT_DB:-support_db}?sslmode=disable"
SERVICE_DB_TYPES["support-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["support-service"]="$ROOT_DIR/services/support-service/migrations"

# Client Service (PostgreSQL) - future
SERVICE_DB_URLS["client-service"]="postgresql://${POSTGRES_CLIENT_USER:-dev}:${POSTGRES_CLIENT_PASSWORD:-dev123}@${DB_HOST}:5442/${POSTGRES_CLIENT_DB:-client_db}?sslmode=disable"
SERVICE_DB_TYPES["client-service"]="postgres"
SERVICE_MIGRATIONS_DIRS["client-service"]="$ROOT_DIR/services/client-service/migrations"

# User Service (MongoDB) - future
SERVICE_DB_URLS["user-service"]="${USER_DATABASE_URL:-mongodb://${MONGO_USER_USER:-dev}:${MONGO_USER_PASSWORD:-dev123}@${DB_HOST}:27017/${MONGO_USER_DB:-user_db}?authSource=admin}"
SERVICE_DB_TYPES["user-service"]="mongodb"
SERVICE_MIGRATIONS_DIRS["user-service"]="$ROOT_DIR/services/user-service/migrations"

# Currently implemented services (PostgreSQL only)
IMPLEMENTED_SERVICES=("auth-service" "file-service" "rating-service" "vehicle-service" "trips-service" "booking-service" "payment-service" "notification-service" "support-service")

# =============================================================================
# Functions
# =============================================================================

usage() {
    echo "Usage: $0 [options] [service] [direction]"
    echo ""
    echo "Arguments:"
    echo "  service      Service to migrate (auth-service, client-service, user-service)"
    echo "               Leave empty to migrate all implemented services"
    echo "  direction    Migration direction: up (default), down, or version number"
    echo ""
    echo "Options:"
    echo "  -h, --help          Show this help message"
    echo "  -s, --status        Show migration status only (no changes)"
    echo "  -f, --force         Force migration (skip confirmation for down)"
    echo "  -n, --dry-run       Show what would be done without executing"
    echo "  --all               Include all services (even unimplemented)"
    echo ""
    echo "Examples:"
    echo "  $0                        # Migrate all services UP"
    echo "  $0 auth-service           # Migrate auth-service UP"
    echo "  $0 auth-service down      # Rollback auth-service"
    echo "  $0 auth-service 3         # Migrate auth-service to version 3"
    echo "  $0 -s                     # Show status of all services"
    echo "  $0 -s auth-service        # Show status of auth-service"
}

check_migrate_installed() {
    if ! command -v migrate &> /dev/null; then
        echo -e "${YELLOW}⚠️  golang-migrate not found. Install it for best experience:${NC}"
        echo ""
        echo "  macOS:  brew install golang-migrate"
        echo "  Linux:  curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz"
        echo "  Docker: docker pull migrate/migrate"
        echo ""
        return 1
    fi
    return 0
}

check_service_exists() {
    local service=$1
    local migrations_dir="${SERVICE_MIGRATIONS_DIRS[$service]}"

    if [ ! -d "$ROOT_DIR/services/$service" ]; then
        return 1
    fi
    return 0
}

check_migrations_exist() {
    local service=$1
    local migrations_dir="${SERVICE_MIGRATIONS_DIRS[$service]}"

    if [ ! -d "$migrations_dir" ]; then
        return 1
    fi

    local count=$(ls -1 "$migrations_dir"/*.sql 2>/dev/null | wc -l)
    if [ "$count" -eq 0 ]; then
        return 1
    fi
    return 0
}

get_migration_status() {
    local service=$1
    local db_url="${SERVICE_DB_URLS[$service]}"
    local db_type="${SERVICE_DB_TYPES[$service]}"
    local migrations_dir="${SERVICE_MIGRATIONS_DIRS[$service]}"

    if [ "$db_type" == "postgres" ]; then
        if check_migrate_installed; then
            migrate -path "$migrations_dir" -database "$db_url" version 2>&1 || echo "No migrations applied"
        else
            echo "Unknown (install golang-migrate)"
        fi
    elif [ "$db_type" == "mongodb" ]; then
        echo "MongoDB migrations not implemented"
    fi
}

count_migration_files() {
    local migrations_dir=$1
    ls -1 "$migrations_dir"/*.up.sql 2>/dev/null | wc -l | tr -d ' '
}

migrate_postgres() {
    local service=$1
    local direction=$2
    local db_url="${SERVICE_DB_URLS[$service]}"
    local migrations_dir="${SERVICE_MIGRATIONS_DIRS[$service]}"

    echo -e "${CYAN}  Database: ${db_url%%\?*}${NC}"
    echo -e "${CYAN}  Migrations: $migrations_dir${NC}"
    echo ""

    if check_migrate_installed; then
        case "$direction" in
            up)
                echo "  → Running migrations UP..."
                migrate -path "$migrations_dir" -database "$db_url" up
                ;;
            down)
                echo "  → Running migrations DOWN (1 step)..."
                migrate -path "$migrations_dir" -database "$db_url" down 1
                ;;
            down-all)
                echo "  → Running migrations DOWN (all)..."
                migrate -path "$migrations_dir" -database "$db_url" down -all
                ;;
            *)
                # Assume it's a version number
                echo "  → Migrating to version $direction..."
                migrate -path "$migrations_dir" -database "$db_url" goto "$direction"
                ;;
        esac
    else
        # Fallback to psql
        echo -e "${YELLOW}  Using psql fallback...${NC}"

        # Extract connection params from DATABASE_URL
        local db_user=$(echo "$db_url" | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
        local db_pass=$(echo "$db_url" | sed -n 's/.*:\/\/[^:]*:\([^@]*\)@.*/\1/p')
        local db_host=$(echo "$db_url" | sed -n 's/.*@\([^:]*\):.*/\1/p')
        local db_port=$(echo "$db_url" | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
        local db_name=$(echo "$db_url" | sed -n 's/.*\/\([^?]*\).*/\1/p')

        export PGPASSWORD="$db_pass"

        if [ "$direction" == "up" ]; then
            for migration in "$migrations_dir"/*.up.sql; do
                if [ -f "$migration" ]; then
                    echo "    → Applying: $(basename "$migration")"
                    psql -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" -f "$migration"
                fi
            done
        elif [ "$direction" == "down" ]; then
            for migration in $(ls -r "$migrations_dir"/*.down.sql 2>/dev/null); do
                if [ -f "$migration" ]; then
                    echo "    → Reverting: $(basename "$migration")"
                    psql -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" -f "$migration"
                    break  # Only one step
                fi
            done
        fi

        unset PGPASSWORD
    fi
}

migrate_mongodb() {
    local service=$1
    local direction=$2

    echo -e "${YELLOW}  ⚠️  MongoDB migrations not yet implemented${NC}"
    echo "  Consider using:"
    echo "    - migrate-mongo: npm install -g migrate-mongo"
    echo "    - mongock: https://mongock.io/"
    return 0
}

migrate_service() {
    local service=$1
    local direction=$2
    local db_type="${SERVICE_DB_TYPES[$service]}"

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Migrating: $service${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # Check if service exists
    if ! check_service_exists "$service"; then
        echo -e "${YELLOW}  ⚠️  Service directory not found: services/$service${NC}"
        echo -e "${YELLOW}     Skipping...${NC}"
        return 0
    fi

    # Check if migrations exist
    if ! check_migrations_exist "$service"; then
        echo -e "${YELLOW}  ⚠️  No migrations found for $service${NC}"
        echo -e "${YELLOW}     Create migrations in: ${SERVICE_MIGRATIONS_DIRS[$service]}${NC}"
        return 0
    fi

    local migration_count=$(count_migration_files "${SERVICE_MIGRATIONS_DIRS[$service]}")
    echo -e "${CYAN}  Found $migration_count migration file(s)${NC}"

    case "$db_type" in
        postgres)
            migrate_postgres "$service" "$direction"
            ;;
        mongodb)
            migrate_mongodb "$service" "$direction"
            ;;
        *)
            echo -e "${RED}  ✗ Unknown database type: $db_type${NC}"
            return 1
            ;;
    esac

    if [ $? -eq 0 ]; then
        echo ""
        echo -e "${GREEN}  ✓ $service migration completed${NC}"
    else
        echo ""
        echo -e "${RED}  ✗ $service migration failed${NC}"
        return 1
    fi
}

show_status() {
    local service=$1

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Status: $service${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    if ! check_service_exists "$service"; then
        echo -e "  ${YELLOW}Service not found${NC}"
        return 0
    fi

    if ! check_migrations_exist "$service"; then
        echo -e "  ${YELLOW}No migrations${NC}"
        return 0
    fi

    local migration_count=$(count_migration_files "${SERVICE_MIGRATIONS_DIRS[$service]}")
    echo -e "  Migration files: ${CYAN}$migration_count${NC}"
    echo -e "  Database type:   ${CYAN}${SERVICE_DB_TYPES[$service]}${NC}"
    echo -e "  Current version: ${CYAN}$(get_migration_status "$service")${NC}"
    echo ""
}

# =============================================================================
# Main
# =============================================================================

# Parse options
STATUS_ONLY=false
FORCE=false
DRY_RUN=false
INCLUDE_ALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -s|--status)
            STATUS_ONLY=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -n|--dry-run)
            DRY_RUN=true
            shift
            ;;
        --all)
            INCLUDE_ALL=true
            shift
            ;;
        -*)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
        *)
            break
            ;;
    esac
done

# Get service and direction arguments
SERVICE=${1:-""}
DIRECTION=${2:-"up"}

# Load environment
if [ -f "$ROOT_DIR/.env" ]; then
    export $(cat "$ROOT_DIR/.env" | grep -v '^#' | xargs)
fi

# Determine which services to process
if [ -n "$SERVICE" ]; then
    # Single service specified
    if [ -z "${SERVICE_DB_URLS[$SERVICE]}" ]; then
        echo -e "${RED}Unknown service: $SERVICE${NC}"
        echo ""
        echo "Available services:"
        for svc in "${!SERVICE_DB_URLS[@]}"; do
            echo "  - $svc"
        done
        exit 1
    fi
    SERVICES=("$SERVICE")
else
    # All services
    if [ "$INCLUDE_ALL" = true ]; then
        SERVICES=("${!SERVICE_DB_URLS[@]}")
    else
        SERVICES=("${IMPLEMENTED_SERVICES[@]}")
    fi
fi

# Status only mode
if [ "$STATUS_ONLY" = true ]; then
    echo -e "${YELLOW}Migration Status${NC}"
    echo ""
    for svc in "${SERVICES[@]}"; do
        show_status "$svc"
    done
    exit 0
fi

# Dry run mode
if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}Dry Run Mode - No changes will be made${NC}"
    echo ""
    echo "Would migrate the following services:"
    for svc in "${SERVICES[@]}"; do
        echo "  - $svc ($DIRECTION)"
    done
    exit 0
fi

# Confirm down migrations
if [[ "$DIRECTION" == "down"* ]] && [ "$FORCE" = false ]; then
    echo -e "${RED}⚠️  WARNING: You are about to run DOWN migrations!${NC}"
    echo "This will rollback database changes and may cause data loss."
    echo ""
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cancelled."
        exit 0
    fi
fi

echo "Direction: $DIRECTION"
echo "Services: ${SERVICES[*]}"
echo ""

# Track results
declare -a FAILED_SERVICES
declare -a SUCCESS_SERVICES

# Run migrations
for svc in "${SERVICES[@]}"; do
    if migrate_service "$svc" "$DIRECTION"; then
        SUCCESS_SERVICES+=("$svc")
    else
        FAILED_SERVICES+=("$svc")
    fi
    echo ""
done

# Summary
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                Migration Summary                       ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ ${#SUCCESS_SERVICES[@]} -gt 0 ]; then
    echo -e "${GREEN}✓ Successful:${NC}"
    for svc in "${SUCCESS_SERVICES[@]}"; do
        echo "    - $svc"
    done
fi

if [ ${#FAILED_SERVICES[@]} -gt 0 ]; then
    echo ""
    echo -e "${RED}✗ Failed:${NC}"
    for svc in "${FAILED_SERVICES[@]}"; do
        echo "    - $svc"
    done
    echo ""
    exit 1
fi

echo ""
echo -e "${GREEN}All migrations completed successfully!${NC}"
