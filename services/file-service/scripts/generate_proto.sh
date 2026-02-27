#!/bin/bash
# services/file-service/scripts/generate_proto.sh
# Generate protobuf files for file-service

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get script directory and change to service root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$SERVICE_DIR"

echo -e "${YELLOW}Generating protobuf files for file-service...${NC}"
echo "Working directory: $(pwd)"
echo ""

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
  echo -e "${RED}protoc not found!${NC}"
  echo "Install with:"
  echo "  macOS: brew install protobuf"
  echo "  Linux: sudo apt-get install protobuf-compiler"
  exit 1
fi

# Install/update Go protoc plugins
echo "Installing/updating Go protoc plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add Go bin to PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Set paths
PROTO_DIR="proto"
PROTO_OUT="${PROTO_DIR}/gen"

# Create output directory
mkdir -p ${PROTO_OUT}

# Check if proto files exist
if [ ! -f "${PROTO_DIR}/file.proto" ]; then
  echo -e "${RED}No proto files found in ${PROTO_DIR}/${NC}"
  exit 1
fi

# Generate file.proto (pas de google/api car inter-service uniquement)
echo "Generating Go code from file.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  file.proto

if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}Proto files generated successfully!${NC}"
  echo ""
  echo "Generated files:"
  ls -lh ${PROTO_OUT}/*.go 2>/dev/null || echo "No .go files in gen/"
else
  echo ""
  echo -e "${RED}Proto generation failed!${NC}"
  exit 1
fi
