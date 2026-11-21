#!/bin/bash
# scripts/development/clean-docker.sh
# Clean Docker resources for Tissi-Mah

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║            Docker Cleanup - Tissi-Mah                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Function to confirm action
confirm() {
  read -p "$1 (y/N): " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    return 0
  else
    return 1
  fi
}

# Show current Docker usage
echo -e "${YELLOW}Current Docker usage:${NC}"
echo ""
docker system df
echo ""

# Option 1: Stop containers
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Option 1: Stop all Tissi-Mah containers${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if confirm "Stop all containers?"; then
  echo "Stopping containers..."
  docker-compose down
  echo -e "${GREEN}✓${NC} Containers stopped"
fi

echo ""

# Option 2: Remove volumes
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Option 2: Remove volumes (⚠️  deletes data)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if confirm "Remove all volumes?"; then
  echo "Removing volumes..."
  docker-compose down -v
  echo -e "${GREEN}✓${NC} Volumes removed"
fi

echo ""

# Option 3: Remove images
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Option 3: Remove Tissi-Mah images${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if confirm "Remove Tissi-Mah images?"; then
  echo "Removing images..."
  
  # Remove service images
  docker images | grep "tissi-mah" | awk '{print $3}' | xargs -r docker rmi -f
  
  # Remove from GitHub Container Registry
  docker images | grep "ghcr.io/kpeewu/tissi-mah" | awk '{print $3}' | xargs -r docker rmi -f
  
  echo -e "${GREEN}✓${NC} Images removed"
fi

echo ""

# Option 4: Clean everything
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${RED}Option 4: Nuclear clean (⚠️  removes everything)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "This will remove:"
echo "  - All stopped containers"
echo "  - All unused networks"
echo "  - All dangling images"
echo "  - All build cache"
echo ""

if confirm "Perform nuclear clean?"; then
  echo "Performing nuclear clean..."
  docker system prune -a -f --volumes
  echo -e "${GREEN}✓${NC} Nuclear clean complete"
fi

echo ""

# Show final usage
echo -e "${YELLOW}Docker usage after cleanup:${NC}"
echo ""
docker system df
echo ""

echo -e "${GREEN}✅ Cleanup complete!${NC}"