#!/bin/bash
# scripts/utilities/lint-all.sh
# Run golangci-lint on all services

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
echo -e "${BLUE}║            Lint All Services - tissiMah                ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Configuration
FIX_MODE=false
VERBOSE=false
TIMEOUT="5m"

# Currently implemented services
SERVICES=("auth-service")  # TODO: Add user-service and client-service when implemented

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --fix)
      FIX_MODE=true
      shift
      ;;
    -v|--verbose)
      VERBOSE=true
      shift
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [--fix] [-v|--verbose] [--timeout <duration>]"
      echo ""
      echo "Options:"
      echo "  --fix       Auto-fix issues where possible"
      echo "  -v          Verbose output"
      echo "  --timeout   Lint timeout per service (default: 5m)"
      echo "  -h          Show this help message"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      exit 1
      ;;
  esac
done

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
  echo -e "${RED}ERROR: golangci-lint is not installed${NC}"
  echo ""
  echo "Install with:"
  echo "  brew install golangci-lint"
  echo "  # or"
  echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  exit 1
fi

echo -e "${YELLOW}golangci-lint version:${NC} $(golangci-lint --version)"
echo ""

# Build lint flags
LINT_FLAGS="--timeout=$TIMEOUT"
if [ "$FIX_MODE" = true ]; then
  LINT_FLAGS="$LINT_FLAGS --fix"
  echo -e "${YELLOW}Running in fix mode${NC}"
fi
if [ "$VERBOSE" = true ]; then
  LINT_FLAGS="$LINT_FLAGS -v"
fi

FAILED_SERVICES=()
PASSED_SERVICES=()

# Lint each service
for service in "${SERVICES[@]}"; do
  SERVICE_DIR="$ROOT_DIR/services/$service"
  
  if [ ! -d "$SERVICE_DIR" ]; then
    echo -e "${YELLOW}⚠️  Service not found: $service${NC}"
    continue
  fi
  
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${BLUE}Linting: $service${NC}"
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  
  cd "$SERVICE_DIR"
  
  if golangci-lint run $LINT_FLAGS ./...; then
    echo -e "${GREEN}✓ $service: No issues found${NC}"
    PASSED_SERVICES+=("$service")
  else
    echo -e "${RED}✗ $service: Issues found${NC}"
    FAILED_SERVICES+=("$service")
  fi
  
  echo ""
done

# Summary
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Lint Summary                        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
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
  
  if [ "$FIX_MODE" = false ]; then
    echo -e "${YELLOW}Tip: Run with --fix to auto-fix some issues${NC}"
  fi
  
  exit 1
fi

echo -e "${GREEN}✅ All services passed linting!${NC}"
