#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$SERVICE_DIR"

echo -e "${YELLOW}🔧 Generating protobuf files for notification-service...${NC}"

if ! command -v protoc &> /dev/null; then
  echo -e "${RED}❌ protoc not found!${NC}"
  exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

export PATH="$PATH:$(go env GOPATH)/bin"

PROTO_DIR="proto"
PROTO_OUT="${PROTO_DIR}/gen"
GOOGLE_API_DIR="${PROTO_DIR}/google/api"

mkdir -p ${PROTO_OUT}

if [ ! -f "${GOOGLE_API_DIR}/annotations.proto" ]; then
  echo -e "${BLUE}Downloading google/api proto dependencies...${NC}"
  mkdir -p ${GOOGLE_API_DIR}
  curl -sSL -o "${GOOGLE_API_DIR}/annotations.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto"
  curl -sSL -o "${GOOGLE_API_DIR}/http.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto"
fi

echo "Generating Go code from notification.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  notification.proto

echo -e "${GREEN}✅ Proto files generated successfully!${NC}"
ls -lh ${PROTO_OUT}/*.go 2>/dev/null
