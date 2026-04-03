#!/bin/bash
# services/api-gateway/scripts/generate_proto.sh
# Generate protobuf files for api-gateway (includes grpc-gateway stubs)

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

echo -e "${YELLOW}🔧 Generating protobuf files for api-gateway...${NC}"
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
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# Add Go bin to PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Set paths
PROTO_DIR="proto"
GOOGLE_API_DIR="${PROTO_DIR}/google/api"

# Output directories
PROTO_OUT_AUTH="${PROTO_DIR}/gen/authpb"
PROTO_OUT_USER="${PROTO_DIR}/gen/userpb"
PROTO_OUT_RATING="${PROTO_DIR}/gen/ratingpb"
PROTO_OUT_FILE="${PROTO_DIR}/gen/filepb"
PROTO_OUT_TRIP="${PROTO_DIR}/gen/trippb"
PROTO_OUT_KYC="${PROTO_DIR}/gen/kycpb"
PROTO_OUT_BOOKING="${PROTO_DIR}/gen/bookingpb"
PROTO_OUT_PAYMENT="${PROTO_DIR}/gen/paymentpb"
PROTO_OUT_NOTIFICATION="${PROTO_DIR}/gen/notificationpb"
mkdir -p ${PROTO_OUT_AUTH} ${PROTO_OUT_USER} ${PROTO_OUT_RATING} ${PROTO_OUT_FILE} ${PROTO_OUT_TRIP} ${PROTO_OUT_KYC} ${PROTO_OUT_BOOKING} ${PROTO_OUT_PAYMENT} ${PROTO_OUT_NOTIFICATION}

# Download google/api proto files if they don't exist
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

# Generate auth.proto (stubs + grpc-gateway reverse proxy)
echo ""
echo "Generating Go code from auth.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_AUTH} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_AUTH} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_AUTH} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  auth.proto

# Generate user.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from user.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_USER} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_USER} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_USER} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  user.proto

# Generate rating.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from rating.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_RATING} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_RATING} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_RATING} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  rating.proto

# Generate file.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from file.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_FILE} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_FILE} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_FILE} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  file.proto

# Generate trip.proto (stubs + grpc-gateway reverse proxy)
# allow_delete_body=true : requis pour CancelWaypoint (DELETE avec body)
echo "Generating Go code from trip.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_TRIP} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_TRIP} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_TRIP} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  --grpc-gateway_opt=allow_delete_body=true \
  trip.proto

# Generate kyc.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from kyc.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_KYC} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_KYC} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_KYC} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  kyc.proto

# Generate booking.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from booking.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_BOOKING} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_BOOKING} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_BOOKING} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  booking.proto

# Generate payment.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from payment.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_PAYMENT} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_PAYMENT} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_PAYMENT} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  payment.proto

# Generate notification.proto (stubs + grpc-gateway reverse proxy)
echo "Generating Go code from notification.proto..."
protoc \
  --proto_path=${PROTO_DIR} \
  --go_out=${PROTO_OUT_NOTIFICATION} \
  --go_opt=paths=source_relative \
  --go-grpc_out=${PROTO_OUT_NOTIFICATION} \
  --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=${PROTO_OUT_NOTIFICATION} \
  --grpc-gateway_opt=paths=source_relative \
  --grpc-gateway_opt=generate_unbound_methods=false \
  notification.proto

# Check result
if [ $? -eq 0 ]; then
  echo ""
  echo -e "${GREEN}✅ Proto files generated successfully!${NC}"
  echo ""
  echo "Generated files:"
  ls -lh ${PROTO_OUT_AUTH}/*.go 2>/dev/null || echo "No .go files in gen/authpb/"
  echo ""
  ls -lh ${PROTO_OUT_USER}/*.go 2>/dev/null || echo "No .go files in gen/userpb/"
  echo ""
  ls -lh ${PROTO_OUT_RATING}/*.go 2>/dev/null || echo "No .go files in gen/ratingpb/"
else
  echo ""
  echo -e "${RED}❌ Proto generation failed!${NC}"
  exit 1
fi
