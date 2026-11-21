#!/bin/bash
# services/vehicle-service/scripts/run_migration.sh
# Run database migrations for vehicle-service

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default values
MIGRATIONS_DIR="migrations"
DATABASE_URL=${DATABASE_URL:-"postgresql://dev:dev123@localhost:5432/vehicle_db?sslmode=disable"}
DIRECTION=${1:-"up"}  # up, down, or version number

echo -e "${YELLOW}📦 Running database migrations for vehicle-service...${NC}"
echo ""

# Check if migrations directory exists
if [ ! -d "${MIGRATIONS_DIR}" ]; then
  echo -e "${RED}❌ Migrations directory not found: ${MIGRATIONS_DIR}${NC}"
  exit 1
fi

echo "Database: ${DATABASE_URL}"
echo "Migrations directory: ${MIGRATIONS_DIR}"
echo "Direction: ${DIRECTION}"
echo ""

# Check if using golang-migrate or manual
if command -v migrate &> /dev/null; then
  # Using golang-migrate (recommended)
  echo -e "${BLUE}Using golang-migrate...${NC}"

  case ${DIRECTION} in
    up)
      migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" up
      ;;
    down)
      migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" down
      ;;
    *)
      migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" goto ${DIRECTION}
      ;;
  esac

else
  # Manual migration with psql
  echo -e "${BLUE}Using psql (manual mode)...${NC}"
  echo -e "${YELLOW}⚠️  Install golang-migrate for better migration management:${NC}"
  echo "  macOS: brew install golang-migrate"
  echo "  Linux: curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz"
  echo ""

  # Extract connection params from DATABASE_URL
  DB_USER=$(echo ${DATABASE_URL} | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
  DB_PASS=$(echo ${DATABASE_URL} | sed -n 's/.*:\/\/[^:]*:\([^@]*\)@.*/\1/p')
  DB_HOST=$(echo ${DATABASE_URL} | sed -n 's/.*@\([^:]*\):.*/\1/p')
  DB_PORT=$(echo ${DATABASE_URL} | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
  DB_NAME=$(echo ${DATABASE_URL} | sed -n 's/.*\/\([^?]*\).*/\1/p')

  export PGPASSWORD=${DB_PASS}

  if [ "${DIRECTION}" == "up" ]; then
    echo "Running migrations UP..."
    for migration in ${MIGRATIONS_DIR}/up/*.sql; do
      if [ -f "${migration}" ]; then
        echo "  → Applying: $(basename ${migration})"
        psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f ${migration}
      fi
    done
  elif [ "${DIRECTION}" == "down" ]; then
    echo "Running migrations DOWN..."
    for migration in $(ls -r ${MIGRATIONS_DIR}/down/*.sql); do
      if [ -f "${migration}" ]; then
        echo "  → Reverting: $(basename ${migration})"
        psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f ${migration}
      fi
    done
  else
    echo -e "${RED}❌ Invalid direction: ${DIRECTION}${NC}"
    echo "Usage: ./run_migration.sh [up|down]"
    exit 1
  fi

  unset PGPASSWORD
fi

if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ Migrations completed successfully!${NC}"
else
  echo ""
  echo -e "${RED}❌ Migration failed!${NC}"
  exit 1
fi

# Show current schema version (if using golang-migrate)
if command -v migrate &> /dev/null; then
  echo ""
  echo -e "${BLUE}Current schema version:${NC}"
  migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" version
fi
