#!/bin/bash
# scripts/database/backup-databases.sh
# Backup all Tissi-Mah databases

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
BACKUP_DIR=${BACKUP_DIR:-"./backups"}
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
KEEP_DAYS=${KEEP_DAYS:-7}

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           Database Backup - Tissi-Mah                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Load environment
if [ -f ".env" ]; then
  export $(cat .env | grep -v '^#' | xargs)
fi

echo "Backup directory: $BACKUP_DIR"
echo "Timestamp: $TIMESTAMP"
echo ""

# Backup Auth PostgreSQL
echo -e "${YELLOW}Backing up auth-postgres...${NC}"

POSTGRES_AUTH_USER=${POSTGRES_AUTH_USER:-dev}
POSTGRES_AUTH_PASSWORD=${POSTGRES_AUTH_PASSWORD:-dev123}
POSTGRES_AUTH_DB=${POSTGRES_AUTH_DB:-auth_db}

export PGPASSWORD=$POSTGRES_AUTH_PASSWORD

pg_dump -h localhost -p 5432 -U $POSTGRES_AUTH_USER $POSTGRES_AUTH_DB \
  > "$BACKUP_DIR/auth_db_$TIMESTAMP.sql"

if [ $? -eq 0 ]; then
  gzip "$BACKUP_DIR/auth_db_$TIMESTAMP.sql"
  echo -e "${GREEN}✓${NC} auth-postgres backed up"
else
  echo -e "${RED}✗${NC} auth-postgres backup failed"
fi

unset PGPASSWORD
echo ""

# Backup Client PostgreSQL
echo -e "${YELLOW}Backing up client-postgres...${NC}"

POSTGRES_CLIENT_USER=${POSTGRES_CLIENT_USER:-dev}
POSTGRES_CLIENT_PASSWORD=${POSTGRES_CLIENT_PASSWORD:-dev123}
POSTGRES_CLIENT_DB=${POSTGRES_CLIENT_DB:-client_db}

export PGPASSWORD=$POSTGRES_CLIENT_PASSWORD

pg_dump -h localhost -p 5433 -U $POSTGRES_CLIENT_USER $POSTGRES_CLIENT_DB \
  > "$BACKUP_DIR/client_db_$TIMESTAMP.sql"

if [ $? -eq 0 ]; then
  gzip "$BACKUP_DIR/client_db_$TIMESTAMP.sql"
  echo -e "${GREEN}✓${NC} client-postgres backed up"
else
  echo -e "${RED}✗${NC} client-postgres backup failed"
fi

unset PGPASSWORD
echo ""

# Backup User MongoDB
echo -e "${YELLOW}Backing up user-mongodb...${NC}"

MONGODB_USER=${MONGODB_USER:-dev}
MONGODB_PASSWORD=${MONGODB_PASSWORD:-dev123}
MONGODB_DB=${MONGODB_DB:-user_db}

mongodump \
  --host localhost:27017 \
  --username $MONGODB_USER \
  --password $MONGODB_PASSWORD \
  --authenticationDatabase admin \
  --db $MONGODB_DB \
  --out "$BACKUP_DIR/mongodb_$TIMESTAMP"

if [ $? -eq 0 ]; then
  tar -czf "$BACKUP_DIR/mongodb_$TIMESTAMP.tar.gz" -C "$BACKUP_DIR" "mongodb_$TIMESTAMP"
  rm -rf "$BACKUP_DIR/mongodb_$TIMESTAMP"
  echo -e "${GREEN}✓${NC} user-mongodb backed up"
else
  echo -e "${RED}✗${NC} user-mongodb backup failed"
fi

echo ""

# Clean old backups
echo -e "${YELLOW}Cleaning old backups (older than $KEEP_DAYS days)...${NC}"

find "$BACKUP_DIR" -name "*.sql.gz" -mtime +$KEEP_DAYS -delete
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +$KEEP_DAYS -delete

echo -e "${GREEN}✓${NC} Old backups cleaned"
echo ""

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ Backup complete!${NC}"
echo ""
echo "Backup files:"
ls -lh "$BACKUP_DIR"/*_$TIMESTAMP* 2>/dev/null || echo "No backups created"
echo ""
echo "Total backup size:"
du -sh "$BACKUP_DIR"