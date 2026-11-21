#!/bin/bash
# scripts/deployment/rollback-service.sh
# Rollback a service to previous version

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Arguments
SERVICE=${1}
ENVIRONMENT=${2:-"vps-dev"}
REVISION=${3:-"previous"}

if [ -z "$SERVICE" ]; then
  echo -e "${RED}❌ Usage: ./rollback-service.sh <service-name> [environment] [revision]${NC}"
  echo ""
  echo "Examples:"
  echo "  ./rollback-service.sh auth-service vps-dev"
  echo "  ./rollback-service.sh auth-service staging previous"
  echo "  ./rollback-service.sh auth-service prod 2"
  exit 1
fi

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Service Rollback - Tissi-Mah                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

echo "Service: $SERVICE"
echo "Environment: $ENVIRONMENT"
echo "Revision: $REVISION"
echo ""

# Determine namespace
case $ENVIRONMENT in
  vps-dev)
    NAMESPACE="dev"
    ;;
  staging)
    NAMESPACE="staging"
    ;;
  prod)
    NAMESPACE="production"
    ;;
  *)
    NAMESPACE="default"
    ;;
esac

# Confirm rollback
echo -e "${YELLOW}⚠️  This will rollback $SERVICE in $ENVIRONMENT${NC}"
echo ""

if [ "$ENVIRONMENT" == "prod" ]; then
  read -p "Are you sure? Type 'rollback' to confirm: " -r
  if [ "$REPLY" != "rollback" ]; then
    echo "Cancelled"
    exit 0
  fi
else
  read -p "Continue? (y/N): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Cancelled"
    exit 0
  fi
fi

echo ""

# Show current revision
echo -e "${YELLOW}Current deployment:${NC}"
kubectl rollout status deployment/$SERVICE -n $NAMESPACE
echo ""

# Show rollout history
echo -e "${YELLOW}Rollout history:${NC}"
kubectl rollout history deployment/$SERVICE -n $NAMESPACE
echo ""

# Perform rollback
echo -e "${YELLOW}Rolling back...${NC}"

if [ "$REVISION" == "previous" ]; then
  kubectl rollout undo deployment/$SERVICE -n $NAMESPACE
else
  kubectl rollout undo deployment/$SERVICE -n $NAMESPACE --to-revision=$REVISION
fi

# Wait for rollback
echo ""
echo -e "${YELLOW}Waiting for rollback to complete...${NC}"
kubectl rollout status deployment/$SERVICE -n $NAMESPACE --timeout=5m

# Check health
echo ""
echo -e "${YELLOW}Checking service health...${NC}"
sleep 10
./scripts/deployment/health-check.sh $SERVICE $ENVIRONMENT

echo ""
echo -e "${GREEN}✅ Rollback complete!${NC}"