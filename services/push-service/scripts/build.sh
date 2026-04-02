#!/bin/bash
# services/push-service/scripts/build.sh
# Build push-service binary

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Variables
SERVICE_NAME="push-service"
OUTPUT_DIR="bin"
BINARY_NAME="push-service"
MAIN_PATH="cmd/server/main.go"

# Get version from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo -e "${YELLOW}🔨 Building ${SERVICE_NAME}...${NC}"
echo ""
echo "Version: ${VERSION}"
echo "Commit: ${GIT_COMMIT}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Create output directory
mkdir -p ${OUTPUT_DIR}

# Build with ldflags
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s \
    -X main.version=${VERSION} \
    -X main.buildTime=${BUILD_TIME} \
    -X main.gitCommit=${GIT_COMMIT}" \
  -o ${OUTPUT_DIR}/${BINARY_NAME} \
  ${MAIN_PATH}

# Check build result
if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ Build successful!${NC}"
  echo ""
  echo "Binary location: ${OUTPUT_DIR}/${BINARY_NAME}"
  echo "Size: $(du -h ${OUTPUT_DIR}/${BINARY_NAME} | cut -f1)"
else
  echo ""
  echo -e "${RED}❌ Build failed!${NC}"
  exit 1
fi
