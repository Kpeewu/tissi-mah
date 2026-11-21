#!/bin/bash
# scripts/utilities/generate-secrets.sh
# Generate Kubernetes secrets for deployment

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
echo -e "${BLUE}║       Kubernetes Secrets Generator - tissiMah          ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Parse arguments
SERVICE=""
ENVIRONMENT=""
OUTPUT_DIR=""
DRY_RUN=false

usage() {
  echo "Usage: $0 -s <service> -e <environment> [-o <output-dir>] [--dry-run]"
  echo ""
  echo "Options:"
  echo "  -s, --service      Service name (e.g., auth-service)"
  echo "  -e, --environment  Environment (local, vps-dev, staging, prod)"
  echo "  -o, --output       Output directory (default: ./secrets)"
  echo "  --dry-run          Print secrets without writing files"
  echo "  -h, --help         Show this help message"
  echo ""
  echo "Examples:"
  echo "  $0 -s auth-service -e vps-dev"
  echo "  $0 -s auth-service -e staging --dry-run"
}

while [[ $# -gt 0 ]]; do
  case $1 in
    -s|--service)
      SERVICE="$2"
      shift 2
      ;;
    -e|--environment)
      ENVIRONMENT="$2"
      shift 2
      ;;
    -o|--output)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      usage
      exit 1
      ;;
  esac
done

# Validate arguments
if [ -z "$SERVICE" ] || [ -z "$ENVIRONMENT" ]; then
  echo -e "${RED}ERROR: Service and environment are required${NC}"
  usage
  exit 1
fi

# Set default output directory
if [ -z "$OUTPUT_DIR" ]; then
  OUTPUT_DIR="$ROOT_DIR/secrets/$ENVIRONMENT"
fi

# Set namespace based on environment
case "$ENVIRONMENT" in
  local)
    NAMESPACE="default"
    ;;
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
    echo -e "${RED}Unknown environment: $ENVIRONMENT${NC}"
    exit 1
    ;;
esac

echo -e "${YELLOW}Service:${NC} $SERVICE"
echo -e "${YELLOW}Environment:${NC} $ENVIRONMENT"
echo -e "${YELLOW}Namespace:${NC} $NAMESPACE"
echo ""

# Prompt for secrets
echo -e "${BLUE}Enter secret values (press Enter for empty/default):${NC}"
echo ""

read -p "DATABASE_URL: " DATABASE_URL
read -p "REDIS_URL: " REDIS_URL
read -s -p "JWT_SECRET: " JWT_SECRET
echo ""
read -p "FIREBASE_CREDENTIALS (base64 or path): " FIREBASE_CREDS

# Handle Firebase credentials
if [ -f "$FIREBASE_CREDS" ]; then
  FIREBASE_CREDENTIALS=$(base64 < "$FIREBASE_CREDS" | tr -d '\n')
else
  FIREBASE_CREDENTIALS="$FIREBASE_CREDS"
fi

# Generate secret manifest
SECRET_MANIFEST=$(cat << EOF
apiVersion: v1
kind: Secret
metadata:
  name: $SERVICE
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: $SERVICE
    app.kubernetes.io/environment: $ENVIRONMENT
type: Opaque
data:
  DATABASE_URL: $(echo -n "$DATABASE_URL" | base64 | tr -d '\n')
  REDIS_URL: $(echo -n "$REDIS_URL" | base64 | tr -d '\n')
  JWT_SECRET: $(echo -n "$JWT_SECRET" | base64 | tr -d '\n')
  FIREBASE_CREDENTIALS: $(echo -n "$FIREBASE_CREDENTIALS" | base64 | tr -d '\n')
EOF
)

if [ "$DRY_RUN" = true ]; then
  echo ""
  echo -e "${YELLOW}Generated secret manifest (dry-run):${NC}"
  echo "---"
  echo "$SECRET_MANIFEST"
  echo "---"
else
  mkdir -p "$OUTPUT_DIR"
  OUTPUT_FILE="$OUTPUT_DIR/$SERVICE-secrets.yaml"
  
  echo "$SECRET_MANIFEST" > "$OUTPUT_FILE"
  
  echo ""
  echo -e "${GREEN}✓ Secret manifest written to: $OUTPUT_FILE${NC}"
  echo ""
  echo -e "${YELLOW}To apply:${NC}"
  echo "  kubectl apply -f $OUTPUT_FILE"
  echo ""
  echo -e "${RED}WARNING: This file contains sensitive data. Do not commit to git!${NC}"
fi
