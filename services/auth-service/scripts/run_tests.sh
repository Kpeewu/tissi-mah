#!/bin/bash
# services/auth-service/scripts/run_tests.sh
# Run tests for auth-service

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           Auth Service - Test Runner                   ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Configuration
TEST_TYPE=${1:-"all"}  # all, unit, integration
COVERAGE=${COVERAGE:-true}
VERBOSE=${VERBOSE:-false}
RACE=${RACE:-true}

cd "$SERVICE_DIR"

# Build test flags
TEST_FLAGS=""
if [ "$VERBOSE" = true ]; then
  TEST_FLAGS="$TEST_FLAGS -v"
fi
if [ "$RACE" = true ]; then
  TEST_FLAGS="$TEST_FLAGS -race"
fi

run_unit_tests() {
  echo -e "${YELLOW}Running unit tests...${NC}"
  echo ""
  
  if [ "$COVERAGE" = true ]; then
    go test $TEST_FLAGS -coverprofile=coverage-unit.out -covermode=atomic ./tests/unit/...
    echo ""
    echo -e "${GREEN}Unit test coverage:${NC}"
    go tool cover -func=coverage-unit.out | tail -n 1
  else
    go test $TEST_FLAGS ./tests/unit/...
  fi
  
  echo ""
  echo -e "${GREEN}✓ Unit tests passed${NC}"
}

run_integration_tests() {
  echo -e "${YELLOW}Running integration tests...${NC}"
  echo ""
  
  # Check if database is available
  if [ -z "$DATABASE_URL" ]; then
    echo -e "${RED}ERROR: DATABASE_URL is not set${NC}"
    echo "Set DATABASE_URL or start dependencies with: make deps-up"
    exit 1
  fi
  
  if [ "$COVERAGE" = true ]; then
    go test $TEST_FLAGS -coverprofile=coverage-integration.out -covermode=atomic ./tests/integration/...
    echo ""
    echo -e "${GREEN}Integration test coverage:${NC}"
    go tool cover -func=coverage-integration.out | tail -n 1
  else
    go test $TEST_FLAGS ./tests/integration/...
  fi
  
  echo ""
  echo -e "${GREEN}✓ Integration tests passed${NC}"
}

combine_coverage() {
  if [ "$COVERAGE" = true ]; then
    echo -e "${YELLOW}Combining coverage reports...${NC}"
    
    # Check if both coverage files exist
    if [ -f "coverage-unit.out" ] && [ -f "coverage-integration.out" ]; then
      echo "mode: atomic" > coverage.out
      tail -n +2 coverage-unit.out >> coverage.out
      tail -n +2 coverage-integration.out >> coverage.out
    elif [ -f "coverage-unit.out" ]; then
      cp coverage-unit.out coverage.out
    elif [ -f "coverage-integration.out" ]; then
      cp coverage-integration.out coverage.out
    fi
    
    if [ -f "coverage.out" ]; then
      echo ""
      echo -e "${GREEN}Combined coverage:${NC}"
      go tool cover -func=coverage.out | tail -n 1
      
      # Generate HTML report
      go tool cover -html=coverage.out -o coverage.html
      echo ""
      echo "Coverage report: coverage.html"
    fi
  fi
}

# Main
case "$TEST_TYPE" in
  unit)
    run_unit_tests
    ;;
  integration)
    run_integration_tests
    ;;
  all)
    run_unit_tests
    echo ""
    run_integration_tests
    combine_coverage
    ;;
  *)
    echo -e "${RED}Unknown test type: $TEST_TYPE${NC}"
    echo "Usage: $0 [unit|integration|all]"
    exit 1
    ;;
esac

echo ""
echo -e "${GREEN}✅ All requested tests completed successfully!${NC}"
