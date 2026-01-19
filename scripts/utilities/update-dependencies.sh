#!/bin/bash
# scripts/update-deps.sh
# Script pour mettre à jour les dépendances d'un service

set -e

SERVICE=$1
MODE=${2:-"minor"}  # minor (default), patch, major

if [ -z "$SERVICE" ]; then
    echo "❌ Usage: ./scripts/utilities/update-deps.sh <service-name> [mode]"
    echo ""
    echo "Arguments:"
    echo "  service-name: auth-service, user-service, client-service, or 'all'"
    echo "  mode: patch, minor (default), major"
    echo ""
    echo "Examples:"
    echo "  ./scripts/utilities/update-deps.sh auth-service          # Update to latest minor versions"
    echo "  ./scripts/utilities/update-deps.sh auth-service patch    # Update only patch versions"
    echo "  ./scripts/utilities/update-deps.sh auth-service major    # Update to latest major versions"
    echo "  ./scripts/utilities/update-deps.sh all                   # Update all services"
    exit 1
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

update_service() {
    local service=$1
    local service_path="services/$service"
    
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ Service $service not found at $service_path${NC}"
        return 1
    fi
    
    echo -e "${YELLOW}📦 Updating dependencies for $service...${NC}"
    cd "$service_path"
    
    # Backup current go.mod
    cp go.mod go.mod.backup
    
    case "$MODE" in
        patch)
            echo "  ➜ Updating to latest patch versions..."
            go get -u=patch ./...
            ;;
        
        minor)
            echo "  ➜ Updating to latest minor versions..."
            go get -u ./...
            ;;
        
        major)
            echo "  ➜ Updating to latest major versions (including breaking changes)..."
            echo -e "${RED}  ⚠️  Warning: This may introduce breaking changes!${NC}"
            read -p "  Are you sure? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                echo "  Cancelled."
                mv go.mod.backup go.mod
                cd ../..
                return 0
            fi
            go get -u ./...
            ;;
        
        *)
            echo -e "${RED}❌ Invalid mode: $MODE${NC}"
            echo "  Valid modes: patch, minor, major"
            mv go.mod.backup go.mod
            cd ../..
            return 1
            ;;
    esac
    
    # Tidy up
    echo "  ➜ Tidying go.mod..."
    go mod tidy
    
    # Verify
    echo "  ➜ Verifying dependencies..."
    if go mod verify; then
        echo -e "${GREEN}✅ Dependencies updated successfully for $service${NC}"
        rm go.mod.backup
        
        # Show diff
        echo ""
        echo "  📊 Changes:"
        git diff go.mod | grep "^\+" | grep -v "^+++" | head -20
        echo ""
    else
        echo -e "${RED}❌ Verification failed! Restoring backup...${NC}"
        mv go.mod.backup go.mod
        cd ../..
        return 1
    fi
    
    cd ../..
    return 0
}

# Main logic
if [ "$SERVICE" == "all" ]; then
    echo -e "${YELLOW}🔄 Updating all services...${NC}"
    echo ""
    
    SERVICES=("auth-service" "user-service" "client-service")
    FAILED_SERVICES=()
    
    for svc in "${SERVICES[@]}"; do
        if [ -d "services/$svc" ]; then
            if ! update_service "$svc"; then
                FAILED_SERVICES+=("$svc")
            fi
            echo ""
        fi
    done
    
    # Summary
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if [ ${#FAILED_SERVICES[@]} -eq 0 ]; then
        echo -e "${GREEN}✅ All services updated successfully!${NC}"
    else
        echo -e "${RED}❌ Some services failed to update:${NC}"
        for failed in "${FAILED_SERVICES[@]}"; do
            echo "  - $failed"
        done
        exit 1
    fi
else
    update_service "$SERVICE"
fi

echo ""
echo -e "${YELLOW}📝 Next steps:${NC}"
echo "  1. Review the changes: git diff services/$SERVICE/go.mod"
echo "  2. Run tests: make test"
echo "  3. Update DEPENDENCIES.md if needed"
echo "  4. Commit: git commit -am 'chore: update $SERVICE dependencies'"