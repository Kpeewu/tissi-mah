#!/bin/bash
# services/push-service/scripts/run_tests.sh
# Run push-service tests

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}🧪 Running push-service tests...${NC}"
echo ""

# Run unit tests
go test ./tests/unit/... -v -race -coverprofile=coverage.out

if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ All tests passed!${NC}"
else
  echo ""
  echo -e "${RED}❌ Some tests failed!${NC}"
  exit 1
fi
