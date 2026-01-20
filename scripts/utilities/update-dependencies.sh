#!/bin/bash
# scripts/utilities/update-dependencies.sh
# Update Go dependencies for the monorepo workspace
#
# This script supports updating:
#   - pkg/         : Shared dependencies (updates all services)
#   - services/*   : Service-specific dependencies only

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
echo -e "${BLUE}║       Update Dependencies - tissiMah Workspace         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# Functions
# =============================================================================

usage() {
    echo "Usage: $0 <target> [mode]"
    echo ""
    echo "Targets:"
    echo "  all            Update all modules"
    echo "  shared         Update shared pkg dependencies (affects all services)"
    echo "  auth-service   Update auth-service specific dependencies"
    echo "  <service>      Update specific service dependencies"
    echo ""
    echo "Modes:"
    echo "  patch          Update patch versions only (default)"
    echo "  minor          Update minor versions"
    echo "  major          Update major versions (may break compatibility)"
    echo ""
    echo "Examples:"
    echo "  $0 shared patch    # Update shared deps to latest patch"
    echo "  $0 auth-service    # Update auth-service deps (patch)"
    echo "  $0 all minor       # Update all modules to latest minor"
}

backup_module() {
    local mod_path=$1
    cp "$mod_path/go.mod" "$mod_path/go.mod.backup"
    if [ -f "$mod_path/go.sum" ]; then
        cp "$mod_path/go.sum" "$mod_path/go.sum.backup"
    fi
}

restore_module() {
    local mod_path=$1
    if [ -f "$mod_path/go.mod.backup" ]; then
        mv "$mod_path/go.mod.backup" "$mod_path/go.mod"
    fi
    if [ -f "$mod_path/go.sum.backup" ]; then
        mv "$mod_path/go.sum.backup" "$mod_path/go.sum"
    fi
}

cleanup_backup() {
    local mod_path=$1
    rm -f "$mod_path/go.mod.backup" "$mod_path/go.sum.backup"
}

update_module() {
    local mod_path=$1
    local mode=$2
    local mod_name=$(basename "$mod_path")

    echo -e "${YELLOW}Updating $mod_name...${NC}"

    cd "$mod_path"
    backup_module "$mod_path"

    case "$mode" in
        patch)
            echo "  → Updating to latest patch versions..."
            go get -u=patch ./...
            ;;
        minor)
            echo "  → Updating to latest minor versions..."
            go get -u ./...
            ;;
        major)
            echo -e "  → ${RED}Updating to latest major versions (may break compatibility)${NC}"
            go get -u ./...
            ;;
    esac

    echo "  → Running go mod tidy..."
    go mod tidy

    echo "  → Verifying dependencies..."
    if go mod verify; then
        echo -e "${GREEN}  ✓ $mod_name updated successfully${NC}"
        cleanup_backup "$mod_path"

        # Show changes
        if git diff --quiet go.mod 2>/dev/null; then
            echo "  (no changes)"
        else
            echo ""
            echo "  Changes in go.mod:"
            git diff go.mod 2>/dev/null | grep "^[+-]" | grep -v "^[+-][+-][+-]" | head -15 || true
        fi
        return 0
    else
        echo -e "${RED}  ✗ Verification failed, restoring backup${NC}"
        restore_module "$mod_path"
        return 1
    fi
}

update_shared() {
    local mode=$1

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Updating shared dependencies (pkg)${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}⚠️  Warning: Updating shared dependencies affects ALL services!${NC}"
    echo ""

    if ! update_module "$ROOT_DIR/pkg" "$mode"; then
        return 1
    fi

    echo ""
    echo -e "${YELLOW}Syncing workspace after shared update...${NC}"
    cd "$ROOT_DIR"
    go work sync

    # Tidy all services that depend on pkg
    echo ""
    echo -e "${YELLOW}Tidying dependent services...${NC}"
    for service_dir in "$ROOT_DIR/services/"*/; do
        if [ -d "$service_dir" ] && [ -f "$service_dir/go.mod" ]; then
            service=$(basename "$service_dir")
            echo "  → Tidying $service..."
            cd "$service_dir"
            go mod tidy
        fi
    done

    echo ""
    echo -e "${GREEN}✓ Shared dependencies updated${NC}"
}

update_service() {
    local service=$1
    local mode=$2
    local service_path="$ROOT_DIR/services/$service"

    if [ ! -d "$service_path" ]; then
        echo -e "${RED}ERROR: Service $service not found${NC}"
        return 1
    fi

    if [ ! -f "$service_path/go.mod" ]; then
        echo -e "${RED}ERROR: No go.mod found for $service${NC}"
        return 1
    fi

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Updating $service dependencies${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    if ! update_module "$service_path" "$mode"; then
        return 1
    fi

    echo ""
    echo -e "${GREEN}✓ $service dependencies updated${NC}"
}

update_all() {
    local mode=$1
    local failed=()

    # Update shared first
    if ! update_shared "$mode"; then
        failed+=("pkg")
    fi

    echo ""

    # Update each service
    for service_dir in "$ROOT_DIR/services/"*/; do
        if [ -d "$service_dir" ] && [ -f "$service_dir/go.mod" ]; then
            service=$(basename "$service_dir")
            echo ""
            if ! update_service "$service" "$mode"; then
                failed+=("$service")
            fi
        fi
    done

    # Final sync
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Final workspace sync${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    cd "$ROOT_DIR"
    go work sync
    echo -e "${GREEN}✓ Workspace synced${NC}"

    # Summary
    echo ""
    if [ ${#failed[@]} -eq 0 ]; then
        echo -e "${GREEN}✓ All modules updated successfully${NC}"
    else
        echo -e "${RED}✗ Some modules failed to update:${NC}"
        for f in "${failed[@]}"; do
            echo "  - $f"
        done
        return 1
    fi
}

# =============================================================================
# Main
# =============================================================================

TARGET=${1:-""}
MODE=${2:-"patch"}

if [ -z "$TARGET" ] || [ "$TARGET" == "-h" ] || [ "$TARGET" == "--help" ]; then
    usage
    exit 0
fi

# Validate mode
case "$MODE" in
    patch|minor|major)
        ;;
    *)
        echo -e "${RED}Invalid mode: $MODE${NC}"
        echo "Valid modes: patch, minor, major"
        exit 1
        ;;
esac

# Confirm major updates
if [ "$MODE" == "major" ]; then
    echo -e "${RED}⚠️  WARNING: Major version updates may introduce breaking changes!${NC}"
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cancelled."
        exit 0
    fi
fi

echo "Mode: $MODE"
echo ""

case "$TARGET" in
    all)
        update_all "$MODE"
        ;;
    shared|pkg)
        update_shared "$MODE"
        ;;
    *)
        if [ -d "$ROOT_DIR/services/$TARGET" ]; then
            update_service "$TARGET" "$MODE"
        else
            echo -e "${RED}Unknown target: $TARGET${NC}"
            usage
            exit 1
        fi
        ;;
esac

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           Update completed successfully!               ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Review changes: git diff"
echo "  2. Run tests: make test-all"
echo "  3. Commit: git commit -am 'chore(deps): update dependencies'"
