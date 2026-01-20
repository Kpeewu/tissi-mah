#!/bin/bash
# scripts/deployment/health-check.sh
# Health check for deployed services

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SERVICE=${1:-"all"}
ENVIRONMENT=${2:-"vps-dev"}
TIMEOUT=${TIMEOUT:-30}

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Health Check - Tissi-Mah                      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

echo "Service: $SERVICE"
echo "Environment: $ENVIRONMENT"
echo "Timeout: ${TIMEOUT}s"
echo ""

# Function to check service health
check_service_health() {
  local service_name=$1
  local service_port=$2
  local namespace=${3:-"default"}
  
  echo -e "${YELLOW}Checking $service_name...${NC}"
  
  # Get pod status
  POD_STATUS=$(kubectl get pods -n $namespace -l app=$service_name -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")
  
  if [ "$POD_STATUS" != "Running" ]; then
    echo -e "${RED}✗${NC} Pod status: $POD_STATUS"
    return 1
  fi
  
  echo -e "${GREEN}✓${NC} Pod status: Running"
  
  # Check if pod is ready
  POD_READY=$(kubectl get pods -n $namespace -l app=$service_name -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
  
  if [ "$POD_READY" != "True" ]; then
    echo -e "${RED}✗${NC} Pod ready: False"
    return 1
  fi
  
  echo -e "${GREEN}✓${NC} Pod ready: True"
  
  # Check service endpoint
  SERVICE_IP=$(kubectl get svc -n $namespace $service_name -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
  
  if [ -z "$SERVICE_IP" ]; then
    echo -e "${RED}✗${NC} Service IP not found"
    return 1
  fi
  
  echo -e "${GREEN}✓${NC} Service IP: $SERVICE_IP"
  
  # gRPC health check (if grpcurl is available)
  if command -v grpcurl &> /dev/null; then
    if timeout $TIMEOUT grpcurl -plaintext $SERVICE_IP:$service_port list &>/dev/null; then
      echo -e "${GREEN}✓${NC} gRPC endpoint responsive"
    else
      echo -e "${RED}✗${NC} gRPC endpoint not responsive"
      return 1
    fi
  fi
  
  # Check logs for errors
  ERROR_COUNT=$(kubectl logs -n $namespace -l app=$service_name --tail=100 2>/dev/null | grep -i "error\|fatal\|panic" | wc -l || echo "0")
  
  if [ "$ERROR_COUNT" -gt 0 ]; then
    echo -e "${YELLOW}⚠${NC}  Found $ERROR_COUNT errors in recent logs"
  else
    echo -e "${GREEN}✓${NC} No errors in recent logs"
  fi
  
  echo ""
  return 0
}

# Services configuration
declare -A SERVICES_MAP
SERVICES_MAP[auth-service]=50051
SERVICES_MAP[user-service]=50052
SERVICES_MAP[client-service]=50053

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

# Check services
FAILED_SERVICES=()

if [ "$SERVICE" == "all" ]; then
  for svc in "${!SERVICES_MAP[@]}"; do
    if ! check_service_health "$svc" "${SERVICES_MAP[$svc]}" "$NAMESPACE"; then
      FAILED_SERVICES+=("$svc")
    fi
  done
else
  if [ -n "${SERVICES_MAP[$SERVICE]}" ]; then
    if ! check_service_health "$SERVICE" "${SERVICES_MAP[$SERVICE]}" "$NAMESPACE"; then
      FAILED_SERVICES+=("$SERVICE")
    fi
  else
    echo -e "${RED}❌ Unknown service: $SERVICE${NC}"
    exit 1
  fi
fi

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

if [ ${#FAILED_SERVICES[@]} -eq 0 ]; then
  echo -e "${GREEN}✅ All services healthy!${NC}"
  exit 0
else
  echo -e "${RED}❌ Health check failed for:${NC}"
  for failed in "${FAILED_SERVICES[@]}"; do
    echo "  - $failed"
  done
  exit 1
fi