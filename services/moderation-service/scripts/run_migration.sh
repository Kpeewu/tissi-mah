#!/bin/bash
# services/moderation-service/scripts/run_migration.sh
# Run database migrations for moderation-service

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

MIGRATIONS_DIR="migrations"
DATABASE_URL=${DATABASE_URL:-"postgresql://dev:dev123@localhost:5443/moderation_db?sslmode=disable"}
DIRECTION=${1:-"up"}

echo -e "${YELLOW}📦 Running database migrations for moderation-service...${NC}"
echo ""

if [ ! -d "${MIGRATIONS_DIR}" ]; then
  echo -e "${RED}❌ Migrations directory not found: ${MIGRATIONS_DIR}${NC}"
  exit 1
fi

MIGRATION_COUNT=$(ls -1 ${MIGRATIONS_DIR}/*.sql 2>/dev/null | wc -l)
if [ ${MIGRATION_COUNT} -eq 0 ]; then
  echo -e "${YELLOW}⚠️  No migration files found in ${MIGRATIONS_DIR}${NC}"
  exit 0
fi

echo "Database: ${DATABASE_URL}"
echo "Migrations: ${MIGRATIONS_DIR} (${MIGRATION_COUNT} files)"
echo "Direction: ${DIRECTION}"
echo ""

if command -v migrate &> /dev/null; then
  echo -e "${BLUE}Using golang-migrate...${NC}"
  case ${DIRECTION} in
    up)   migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" up ;;
    down) migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" down ;;
    *)    migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" goto ${DIRECTION} ;;
  esac
else
  echo -e "${BLUE}Using psql (golang-migrate not found)...${NC}"
  echo -e "${YELLOW}⚠️  Install golang-migrate: brew install golang-migrate${NC}"
  echo ""

  DB_USER=$(echo ${DATABASE_URL} | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
  DB_PASS=$(echo ${DATABASE_URL} | sed -n 's/.*:\/\/[^:]*:\([^@]*\)@.*/\1/p')
  DB_HOST=$(echo ${DATABASE_URL} | sed -n 's/.*@\([^:]*\):.*/\1/p')
  DB_PORT=$(echo ${DATABASE_URL} | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
  DB_NAME=$(echo ${DATABASE_URL} | sed -n 's/.*\/\([^?]*\).*/\1/p')

  export PGPASSWORD=${DB_PASS}

  if [ "${DIRECTION}" == "up" ]; then
    for migration in ${MIGRATIONS_DIR}/*.up.sql; do
      [ -f "${migration}" ] && echo "  → $(basename ${migration})" && \
        psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f ${migration}
    done
  elif [ "${DIRECTION}" == "down" ]; then
    for migration in $(ls -r ${MIGRATIONS_DIR}/*.down.sql 2>/dev/null); do
      [ -f "${migration}" ] && echo "  → $(basename ${migration})" && \
        psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f ${migration}
      break
    done
  else
    echo -e "${RED}❌ Direction invalide: ${DIRECTION}${NC}" && exit 1
  fi

  unset PGPASSWORD
fi

echo ""
echo -e "${GREEN}✅ Migrations terminées avec succès !${NC}"

if command -v migrate &> /dev/null; then
  echo ""
  echo -e "${BLUE}Version actuelle :${NC}"
  migrate -path ${MIGRATIONS_DIR} -database "${DATABASE_URL}" version
fi
