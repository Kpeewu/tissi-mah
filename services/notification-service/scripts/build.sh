#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SERVICE_NAME="notification-service"
OUTPUT_DIR="bin"
BINARY_NAME="notification-service"
MAIN_PATH="cmd/server/main.go"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo -e "${YELLOW}🔨 Building ${SERVICE_NAME}...${NC}"

mkdir -p ${OUTPUT_DIR}

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s \
    -X main.version=${VERSION} \
    -X main.buildTime=${BUILD_TIME} \
    -X main.gitCommit=${GIT_COMMIT}" \
  -o ${OUTPUT_DIR}/${BINARY_NAME} \
  ${MAIN_PATH}

echo -e "${GREEN}✅ Build successful! → ${OUTPUT_DIR}/${BINARY_NAME}${NC}"
