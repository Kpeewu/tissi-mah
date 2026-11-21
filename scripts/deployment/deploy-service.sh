#!/bin/bash
# scripts/deployment/deploy-service.sh
# Deploy a service to an environment

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
VERSION=${3:-"latest"}

if [ -z "$SERVICE" ]; then
  echo -e "${RED}❌ Usage: ./deploy-service.sh <service-name> [environment] [version]${NC}"
  echo ""
  echo "Examples:"
  echo "  ./deploy-service.sh auth-service vps-dev latest"
  echo "  ./deploy-service.sh auth-service staging v1.2.3"
  echo "  ./deploy-service.sh auth-service prod v1.2.3"
  exit 1
fi

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Service Deployment - Tissi-Mah                ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

echo "Service: $SERVICE"
echo "Environment: $ENVIRONMENT"
echo "Version: $VERSION"
echo ""

# Determine namespace
case $ENVIRONMENT in
  vps-dev)
    NAMESPACE="dev"
    VALUES_FILE="values/vps-dev.yaml"
    ;;
  staging)
    NAMESPACE="staging"
    VALUES_FILE="values/staging.yaml"
    ;;
  prod)
    NAMESPACE="production"
    VALUES_FILE="values/prod.yaml"
    ;;
  *)
    echo -e "${RED}❌ Invalid environment: $ENVIRONMENT${NC}"
    exit 1
    ;;
esac

# Check if service exists
SERVICE_PATH="services/$SERVICE"
if [ ! -d "$SERVICE_PATH" ]; then
  echo -e "${RED}❌ Service not found: $SERVICE${NC}"
  exit 1
fi

HELM_CHART="$SERVICE_PATH/deployments/helm"
if [ ! -d "$HELM_CHART" ]; then
  echo -e "${RED}❌ Helm chart not found: $HELM_CHART${NC}"
  exit 1
fi

# Confirm deployment
if [ "$ENVIRONMENT" == "prod" ]; then
  echo -e "${RED}⚠️  PRODUCTION DEPLOYMENT${NC}"
  echo ""
  read -p "Are you sure? Type '$SERVICE' to confirm: " -r
  if [ "$REPLY" != "$SERVICE" ]; then
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

# Create namespace if it doesn't exist
if ! kubectl get namespace $NAMESPACE &>/dev/null; then
  echo -e "${YELLOW}Creating namespace: $NAMESPACE${NC}"
  kubectl create namespace $NAMESPACE
fi

# Deploy with Helm
echo -e "${YELLOW}Deploying $SERVICE...${NC}"

helm upgrade --install $SERVICE $HELM_CHART \
  --namespace $NAMESPACE \
  --values $HELM_CHART/$VALUES_FILE \
  --set image.tag=$VERSION \
  --wait \
  --timeout 5m

# Wait for rollout
echo ""
echo -e "${YELLOW}Waiting for deployment...${NC}"
kubectl rollout status deployment/$SERVICE -n $NAMESPACE --timeout=5m

# Check health
echo ""
echo -e "${YELLOW}Checking service health...${NC}"
sleep 10
./scripts/deployment/health-check.sh $SERVICE $ENVIRONMENT

# Show deployment info
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ Deployment complete!${NC}"
echo ""
echo "Deployment info:"
kubectl get deployment $SERVICE -n $NAMESPACE
echo ""
echo "Pods:"
kubectl get pods -n $NAMESPACE -l app=$SERVICE
echo ""
echo "Service:"
kubectl get svc $SERVICE -n $NAMESPACE