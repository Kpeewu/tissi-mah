#!/bin/bash
# scripts/development/run-all-tests.sh
# Run tests for all services

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║            Running All Tests - Tissi-Mah               ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Configuration
TEST_TYPE=${1:-"all"}  # all, unit, integration
PARALLEL=${PARALLEL:-false}
COVERAGE=${COVERAGE:-true}

SERVICES=("auth-service")  # TODO: Ajouter user-service et client-service quand implémentés
FAILED_SERVICES=()
PASSED_SERVICES=()

# Function to run tests for a service
run_service_tests() {
  local service=$1
  
  if [ ! -d "services/$service" ]; then
    echo -e "${YELLOW}⚠️  Service not found: $service${NC}"
    return 0
  fi
  
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${BLUE}Testing: $service${NC}"
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  
  cd "services/$service"
  
  if [ -f "scripts/run_tests.sh" ]; then
    export COVERAGE=$COVERAGE
    if ./scripts/run_tests.sh $TEST_TYPE; then
      PASSED_SERVICES+=("$service")
      cd ../..
      return 0
    else
      FAILED_SERVICES+=("$service")
      cd ../..
      return 1
    fi
  else
    echo -e "${YELLOW}⚠️  No test script found for $service${NC}"
    cd ../..
    return 0
  fi
}

# Check if databases are running
echo -e "${YELLOW}Checking databases...${NC}"
if ! docker-compose ps | grep -q "Up"; then
  echo -e "${RED}❌ Databases not running!${NC}"
  echo "Start with: make deps-up"
  exit 1
fi
echo -e "${GREEN}✓${NC} Databases are running"
echo ""

# Run tests
START_TIME=$(date +%s)

if [ "$PARALLEL" = true ]; then
  echo -e "${YELLOW}Running tests in parallel...${NC}"
  echo ""
  
  for service in "${SERVICES[@]}"; do
    run_service_tests "$service" &
  done
  
  wait
else
  echo -e "${YELLOW}Running tests sequentially...${NC}"
  echo ""
  
  for service in "${SERVICES[@]}"; do
    run_service_tests "$service" || true
    echo ""
  done
fi

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Summary
echo ""
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                   Test Summary                         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Duration: ${DURATION}s"
echo ""

if [ ${#PASSED_SERVICES[@]} -gt 0 ]; then
  echo -e "${GREEN}✅ Passed (${#PASSED_SERVICES[@]}):${NC}"
  for service in "${PASSED_SERVICES[@]}"; do
    echo "  - $service"
  done
  echo ""
fi

if [ ${#FAILED_SERVICES[@]} -gt 0 ]; then
  echo -e "${RED}❌ Failed (${#FAILED_SERVICES[@]}):${NC}"
  for service in "${FAILED_SERVICES[@]}"; do
    echo "  - $service"
  done
  echo ""
  exit 1
fi

# Generate combined coverage report
if [ "$COVERAGE" = true ]; then
  echo -e "${YELLOW}Generating combined coverage report...${NC}"
  echo ""
  
  # Combine coverage files
  COVERAGE_FILES=$(find services/*/coverage.out 2>/dev/null || true)
  
  if [ -n "$COVERAGE_FILES" ]; then
    echo "mode: atomic" > coverage-all.out
    for file in $COVERAGE_FILES; do
      tail -n +2 $file >> coverage-all.out
    done
    
    TOTAL_COVERAGE=$(go tool cover -func=coverage-all.out | tail -n 1 | awk '{print $3}')
    echo -e "${GREEN}Total Coverage: $TOTAL_COVERAGE${NC}"
    
    # Generate HTML report
    go tool cover -html=coverage-all.out -o coverage-all.html
    echo ""
    echo "Combined coverage report: coverage-all.html"
  fi
fi

echo ""
echo -e "${GREEN}✅ All tests completed successfully!${NC}"