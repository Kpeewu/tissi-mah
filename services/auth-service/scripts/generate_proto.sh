#!/bin/bash
# services/auth-service/scripts/generate_proto.sh
# Generate protobuf files for auth-service

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}🔧 Generating protobuf files for auth-service...${NC}"
echo ""

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
  echo -e "${RED}❌ protoc not found!${NC}"
  echo "Install with:"
  echo "  macOS: brew install protobuf"
  echo "  Linux: sudo apt-get install protobuf-compiler"
  exit 1
fi

# Check protoc version
PROTOC_VERSION=$(protoc --version | awk '{print $2}')
echo "protoc version: ${PROTOC_VERSION}"

# Install/update Go protoc plugins
echo ""
echo "Installing/updating Go protoc plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Set paths
PROTO_DIR="proto"
PROTO_OUT="${PROTO_DIR}/gen"

# Create output directory
mkdir -p ${PROTO_OUT}

# Generate
echo ""
echo "Generating Go code from proto files..."
protoc \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  ${PROTO_DIR}/*.proto

# Check result
if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ Proto files generated successfully!${NC}"
  echo ""
  echo "Generated files:"
  ls -lh ${PROTO_OUT}
else
  echo ""
  echo -e "${RED}❌ Proto generation failed!${NC}"
  exit 1
fi