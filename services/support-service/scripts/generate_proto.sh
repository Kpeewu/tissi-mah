#!/bin/bash
# services/support-service/scripts/generate_proto.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$SERVICE_DIR"

if ! command -v protoc &> /dev/null; then
  echo "protoc not found"
  exit 1
fi

export PATH="$PATH:$(go env GOPATH)/bin"

PROTO_DIR="proto"
PROTO_OUT="${PROTO_DIR}/gen"
GOOGLE_API_DIR="${PROTO_DIR}/google/api"

mkdir -p ${PROTO_OUT}

if [ ! -f "${GOOGLE_API_DIR}/annotations.proto" ]; then
  mkdir -p ${GOOGLE_API_DIR}
  curl -sSL -o "${GOOGLE_API_DIR}/annotations.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto"
  curl -sSL -o "${GOOGLE_API_DIR}/http.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto"
fi

protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  support.proto

echo "Proto generation OK"
