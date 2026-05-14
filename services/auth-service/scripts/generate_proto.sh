#!/bin/bash
# services/auth-service/scripts/generate_proto.sh
# Generate protobuf files for auth-service

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Get script directory and change to service root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
cd "$SERVICE_DIR"

echo -e "${YELLOW}🔧 Generating protobuf files for auth-service...${NC}"
echo "Working directory: $(pwd)"
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

# Add Go bin to PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Set paths
PROTO_DIR="proto"
PROTO_OUT="${PROTO_DIR}/gen"
GOOGLE_API_DIR="${PROTO_DIR}/google/api"

# Create output directory
mkdir -p ${PROTO_OUT}

# Download google/api proto files if they don't exist
if [ ! -f "${GOOGLE_API_DIR}/annotations.proto" ]; then
  echo ""
  echo -e "${BLUE}Downloading google/api proto dependencies...${NC}"
  mkdir -p ${GOOGLE_API_DIR}

  # Download annotations.proto
  curl -sSL -o "${GOOGLE_API_DIR}/annotations.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto"

  # Download http.proto
  curl -sSL -o "${GOOGLE_API_DIR}/http.proto" \
    "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto"

  echo -e "${GREEN}✓${NC} Downloaded google/api proto files"
fi

# Check if proto files exist
if [ ! -f "${PROTO_DIR}/auth.proto" ]; then
  echo -e "${RED}❌ No proto files found in ${PROTO_DIR}/${NC}"
  exit 1
fi

# Output directories
PROTO_OUT_USER="${PROTO_OUT}/userpb"
PROTO_OUT_TRIPS="${PROTO_OUT}/tripspb"
PROTO_OUT_BOOKING="${PROTO_OUT}/bookingpb"
PROTO_OUT_PAYMENT="${PROTO_OUT}/paymentpb"
PROTO_OUT_CHAT="${PROTO_OUT}/chatpb"
PROTO_OUT_FILE="${PROTO_OUT}/filepb"
mkdir -p ${PROTO_OUT_USER} ${PROTO_OUT_TRIPS} ${PROTO_OUT_BOOKING} ${PROTO_OUT_PAYMENT} ${PROTO_OUT_CHAT} ${PROTO_OUT_FILE}

# Generate auth.proto
echo ""
echo "Generating Go code from auth.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT} \
  --go-grpc_opt=paths=source_relative \
  auth.proto

# Generate user.proto (client gRPC vers user-service)
echo "Generating Go code from user.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_USER} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_USER} \
  --go-grpc_opt=paths=source_relative \
  user.proto

# Generate trips.proto (client gRPC vers trips-service)
echo "Generating Go code from trips.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_TRIPS} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_TRIPS} \
  --go-grpc_opt=paths=source_relative \
  trips.proto

# Generate booking.proto (client gRPC vers booking-service)
echo "Generating Go code from booking.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_BOOKING} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_BOOKING} \
  --go-grpc_opt=paths=source_relative \
  booking.proto

# Generate payment.proto (client gRPC vers payment-service)
echo "Generating Go code from payment.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_PAYMENT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_PAYMENT} \
  --go-grpc_opt=paths=source_relative \
  payment.proto

# Generate chat.proto (client gRPC vers chat-service)
echo "Generating Go code from chat.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_CHAT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_CHAT} \
  --go-grpc_opt=paths=source_relative \
  chat.proto

# Generate file.proto (client gRPC vers file-service)
echo "Generating Go code from file.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_FILE} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_FILE} \
  --go-grpc_opt=paths=source_relative \
  file.proto

# Check result
if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ Proto files generated successfully!${NC}"
  echo ""
  echo "Generated files:"
  ls -lh ${PROTO_OUT}/*.go 2>/dev/null || echo "No .go files in gen/"
  ls -lh ${PROTO_OUT_USER}/*.go 2>/dev/null || echo "No .go files in gen/userpb/"
else
  echo ""
  echo -e "${RED}❌ Proto generation failed!${NC}"
  exit 1
fi
