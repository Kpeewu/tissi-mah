#!/bin/bash
# scripts/utilities/sync-dependencies.sh
# Synchronize Go dependencies across the monorepo workspace

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
echo -e "${BLUE}║      Go Dependencies Sync - tissiMah Workspace         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

cd "$ROOT_DIR"

# Check if go.work exists
if [ ! -f "go.work" ]; then
    echo -e "${RED}ERROR: go.work not found at root${NC}"
    echo "Initialize the workspace first with: go work init"
    exit 1
fi

# Parse arguments
TIDY_ONLY=false
VERIFY=false
DOWNLOAD=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --tidy)
            TIDY_ONLY=true
            shift
            ;;
        --verify)
            VERIFY=true
            shift
            ;;
        --download)
            DOWNLOAD=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--tidy] [--verify] [--download]"
            echo ""
            echo "Options:"
            echo "  --tidy      Only run go mod tidy on all modules"
            echo "  --verify    Verify dependencies after sync"
            echo "  --download  Download all dependencies"
            echo "  -h          Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Get all modules from go.work
echo -e "${YELLOW}Reading modules from go.work...${NC}"
MODULES=$(grep -E '^\s*\./' go.work | sed 's/^[[:space:]]*//' | tr -d '\r')

echo "Found modules:"
for mod in $MODULES; do
    echo "  - $mod"
done
echo ""

# Step 1: Tidy shared pkg first (other modules depend on it)
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Step 1: Tidy shared pkg module${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

if [ -d "$ROOT_DIR/pkg" ]; then
    cd "$ROOT_DIR/pkg"
    echo -e "${YELLOW}Running go mod tidy in pkg...${NC}"
    go mod tidy
    echo -e "${GREEN}✓ pkg tidied${NC}"
else
    echo -e "${YELLOW}⚠️  pkg directory not found, skipping${NC}"
fi

echo ""

# Step 2: Tidy all service modules
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Step 2: Tidy service modules${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

for mod in $MODULES; do
    # Skip pkg (already done)
    if [ "$mod" == "./pkg" ]; then
        continue
    fi
    
    mod_path="$ROOT_DIR/${mod#./}"
    
    if [ -d "$mod_path" ] && [ -f "$mod_path/go.mod" ]; then
        cd "$mod_path"
        echo -e "${YELLOW}Running go mod tidy in ${mod}...${NC}"
        go mod tidy
        echo -e "${GREEN}✓ ${mod} tidied${NC}"
    else
        echo -e "${YELLOW}⚠️  ${mod} not found or has no go.mod, skipping${NC}"
    fi
done

echo ""

if [ "$TIDY_ONLY" = true ]; then
    echo -e "${GREEN}✅ Tidy complete!${NC}"
    exit 0
fi

# Step 3: Sync workspace
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Step 3: Sync workspace${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

cd "$ROOT_DIR"
echo -e "${YELLOW}Running go work sync...${NC}"
go work sync
echo -e "${GREEN}✓ Workspace synced${NC}"

echo ""

# Step 4: Download dependencies (optional)
if [ "$DOWNLOAD" = true ]; then
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Step 4: Download dependencies${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    for mod in $MODULES; do
        mod_path="$ROOT_DIR/${mod#./}"
        
        if [ -d "$mod_path" ] && [ -f "$mod_path/go.mod" ]; then
            cd "$mod_path"
            echo -e "${YELLOW}Downloading dependencies for ${mod}...${NC}"
            go mod download
            echo -e "${GREEN}✓ ${mod} downloaded${NC}"
        fi
    done
    
    echo ""
fi

# Step 5: Verify (optional)
if [ "$VERIFY" = true ]; then
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Step 5: Verify dependencies${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    for mod in $MODULES; do
        mod_path="$ROOT_DIR/${mod#./}"
        
        if [ -d "$mod_path" ] && [ -f "$mod_path/go.mod" ]; then
            cd "$mod_path"
            echo -e "${YELLOW}Verifying ${mod}...${NC}"
            if go mod verify; then
                echo -e "${GREEN}✓ ${mod} verified${NC}"
            else
                echo -e "${RED}✗ ${mod} verification failed${NC}"
                exit 1
            fi
        fi
    done
    
    echo ""
fi

echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         Dependencies synchronized successfully!        ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
