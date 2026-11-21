#!/bin/bash
# services/email-service/scripts/run_tests.sh
# Run tests for email-service

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           Email Service - Test Runner                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

cd "$SERVICE_DIR"

echo -e "${YELLOW}Running unit tests...${NC}"
go test -v -race -coverprofile=coverage.out ./tests/unit/...

echo ""
echo -e "${GREEN}✅ All tests completed successfully!${NC}"
