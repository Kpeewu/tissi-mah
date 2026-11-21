#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

DIRECTION=${1:-up}
DATABASE_URL=${DATABASE_URL:-""}

if [ -z "$DATABASE_URL" ]; then
  echo -e "${RED}❌ DATABASE_URL is not set${NC}"
  exit 1
fi

MIGRATION_DIR="${SERVICE_DIR}/migrations/${DIRECTION}"

if [ ! -d "$MIGRATION_DIR" ]; then
  echo -e "${RED}❌ Migration directory not found: ${MIGRATION_DIR}${NC}"
  exit 1
fi

echo -e "${YELLOW}Running ${DIRECTION} migrations...${NC}"

for file in "${MIGRATION_DIR}"/*.sql; do
  if [ -f "$file" ]; then
    echo "  Applying: $(basename $file)"
    psql "$DATABASE_URL" -f "$file"
  fi
done

echo -e "${GREEN}✅ Migrations applied!${NC}"
