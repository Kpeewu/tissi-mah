#!/bin/bash
# scripts/utilities/sync-protos-to-helm.sh
# Synchronize proto files from services to their Helm charts
#
# This script copies proto files from their source location (services/<service>/proto/)
# to the Helm chart directory (services/<service>/deployments/helm/proto/)
# This is required because Helm's .Files.Get only works within the chart directory
#
# Usage:
#   ./sync-protos-to-helm.sh              # Sync all services
#   ./sync-protos-to-helm.sh auth-service # Sync specific service

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
echo -e "${BLUE}║        Sync Proto Files to Helm Charts                 ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# Functions
# =============================================================================

usage() {
    echo "Usage: $0 [service]"
    echo ""
    echo "Arguments:"
    echo "  service    Specific service to sync (optional)"
    echo "             If not provided, syncs all services"
    echo ""
    echo "Examples:"
    echo "  $0                  # Sync all services"
    echo "  $0 auth-service     # Sync auth-service only"
}

sync_service_proto() {
    local service=$1
    local service_dir="$ROOT_DIR/services/$service"
    local proto_src="$service_dir/proto"
    local helm_proto_dest="$service_dir/deployments/helm/proto"

    echo -e "${YELLOW}Syncing $service...${NC}"

    # Check if service exists
    if [ ! -d "$service_dir" ]; then
        echo -e "  ${RED}✗ Service directory not found: $service_dir${NC}"
        return 1
    fi

    # Check if proto source exists
    if [ ! -d "$proto_src" ]; then
        echo -e "  ${YELLOW}⚠ No proto directory found: $proto_src${NC}"
        return 0
    fi

    # Check if there are proto files
    proto_count=$(ls -1 "$proto_src"/*.proto 2>/dev/null | wc -l | tr -d ' ')
    if [ "$proto_count" -eq 0 ]; then
        echo -e "  ${YELLOW}⚠ No .proto files found in $proto_src${NC}"
        return 0
    fi

    # Check if Helm chart exists
    if [ ! -d "$service_dir/deployments/helm" ]; then
        echo -e "  ${YELLOW}⚠ No Helm chart found: $service_dir/deployments/helm${NC}"
        return 0
    fi

    # Create destination directory
    mkdir -p "$helm_proto_dest"

    # Copy proto files
    echo "  → Found $proto_count proto file(s)"
    for proto_file in "$proto_src"/*.proto; do
        if [ -f "$proto_file" ]; then
            filename=$(basename "$proto_file")
            cp "$proto_file" "$helm_proto_dest/$filename"
            echo -e "  ${GREEN}✓${NC} Copied $filename"
        fi
    done

    # Also copy any google/api dependencies if they exist locally
    if [ -d "$proto_src/google" ]; then
        cp -r "$proto_src/google" "$helm_proto_dest/"
        echo -e "  ${GREEN}✓${NC} Copied google/api dependencies"
    fi

    echo -e "  ${GREEN}✓ $service proto files synced${NC}"
    return 0
}

# =============================================================================
# Main
# =============================================================================

# Parse arguments
if [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
    usage
    exit 0
fi

SERVICE=${1:-""}

# Track results
synced=0
failed=0

if [ -n "$SERVICE" ]; then
    # Sync specific service
    if sync_service_proto "$SERVICE"; then
        synced=$((synced + 1))
    else
        failed=$((failed + 1))
    fi
else
    # Sync all services
    for service_dir in "$ROOT_DIR/services/"*/; do
        if [ -d "$service_dir" ]; then
            service=$(basename "$service_dir")
            if sync_service_proto "$service"; then
                synced=$((synced + 1))
            else
                failed=$((failed + 1))
            fi
            echo ""
        fi
    done
fi

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
if [ $failed -eq 0 ]; then
    echo -e "${GREEN}✓ Proto sync completed: $synced service(s) synced${NC}"
else
    echo -e "${YELLOW}⚠ Proto sync completed with issues${NC}"
    echo "  Synced: $synced"
    echo "  Failed: $failed"
fi
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Commit the synced proto files"
echo "  2. Deploy with Helm: helm upgrade --install <service> ./deployments/helm"
