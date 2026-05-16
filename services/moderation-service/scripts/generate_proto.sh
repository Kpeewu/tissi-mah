#!/bin/bash
# services/moderation-service/scripts/generate_proto.sh
set -e

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$SERVICE_DIR"

echo -e "${YELLOW}🔧 Generating protobuf files for moderation-service...${NC}"

if ! command -v protoc &> /dev/null; then
  echo -e "${RED}❌ protoc not found!${NC}"; exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$PATH:$(go env GOPATH)/bin"

PROTO_DIR=./proto
PROTO_OUT=./proto/gen
PROTO_OUT_AUTH=./proto/gen/authpb
PROTO_OUT_USER=./proto/gen/userpb

mkdir -p ${PROTO_OUT} ${PROTO_OUT_AUTH} ${PROTO_OUT_USER}

echo "Generating Go code from moderation.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  moderation.proto

echo "Generating Go code from auth.proto (stub)..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_AUTH} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_AUTH} \
  --go-grpc_opt=paths=source_relative \
  auth.proto

echo "Generating Go code from user.proto (stub)..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_USER} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_USER} \
  --go-grpc_opt=paths=source_relative \
  user.proto

echo ""
echo -e "${GREEN}✅ Proto files generated successfully!${NC}"
ls -lh ${PROTO_OUT}/*.go ${PROTO_OUT_AUTH}/*.go ${PROTO_OUT_USER}/*.go 2>/dev/null
