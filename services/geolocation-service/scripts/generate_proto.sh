#!/bin/bash
# services/geolocation-service/scripts/generate_proto.sh
# Generate protobuf files for geolocation-service

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$SERVICE_DIR"

echo -e "${YELLOW}🔧 Generating protobuf files for geolocation-service...${NC}"
echo "Working directory: $(pwd)"
echo ""

if ! command -v protoc &> /dev/null; then
  echo -e "${RED}❌ protoc not found!${NC}"
  echo "Install with:"
  echo "  macOS: brew install protobuf"
  echo "  Linux: sudo apt-get install protobuf-compiler"
  exit 1
fi

PROTOC_VERSION=$(protoc --version | awk '{print $2}')
echo "protoc version: ${PROTOC_VERSION}"

echo ""
echo "Installing/updating Go protoc plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

export PATH="$PATH:$(go env GOPATH)/bin"

PROTO_DIR="proto"
PROTO_OUT="${PROTO_DIR}/gen"
GOOGLE_API_DIR="${PROTO_DIR}/google/api"

mkdir -p ${PROTO_OUT}

if [ ! -f "${GOOGLE_API_DIR}/annotations.proto" ]; then
  echo ""
  echo -e "${BLUE}Downloading google/api proto dependencies...${NC}"
  mkdir -p ${GOOGLE_API_DIR}

  curl -sSL -o "${GOOGLE_API_DIR}/annotations.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto"

  curl -sSL -o "${GOOGLE_API_DIR}/http.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto"

  echo -e "${GREEN}✓${NC} Downloaded google/api proto files"
fi

if [ ! -f "${PROTO_DIR}/geolocation.proto" ]; then
  echo -e "${RED}❌ No proto files found in ${PROTO_DIR}/${NC}"
  exit 1
fi

echo ""
echo "Generating Go code from geolocation.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  geolocation.proto

if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ Proto files generated successfully!${NC}"
  echo ""
  echo "Generated files:"
  ls -lh ${PROTO_OUT}/*.go 2>/dev/null || echo "No .go files in gen/"
else
  echo ""
  echo -e "${RED}❌ Proto generation failed!${NC}"
  exit 1
fi
