#!/bin/bash
# scripts/utilities/install-dependencies.sh
# Install Go dependencies for the monorepo workspace
#
# This script supports the workspace structure:
#   - pkg/         : Shared dependencies (common to all services)
#   - services/*   : Service-specific dependencies

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║      Install Dependencies - tissiMah Workspace         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# Version definitions (single source of truth)
# =============================================================================

# Shared dependencies (in pkg/go.mod)
GRPC_VERSION="v1.60.1"
PROTOBUF_VERSION="v1.32.0"
PGX_VERSION="v5.5.1"
REDIS_VERSION="v9.3.1"
ZAP_VERSION="v1.26.0"
VIPER_VERSION="v1.18.2"
VALIDATOR_VERSION="v10.16.0"
UUID_VERSION="v1.5.0"
OTEL_VERSION="v1.21.0"
PROMETHEUS_VERSION="v1.18.0"

# Auth-service specific
FIREBASE_VERSION="v4.13.0"
JWT_VERSION="v5.2.0"
CRYPTO_VERSION="v0.17.0"

# User-service specific (future)
MONGODB_VERSION="v1.13.1"

# Payment-service specific (future)
STRIPE_VERSION="v76.10.0"

# =============================================================================
# Functions
# =============================================================================

usage() {
    echo "Usage: $0 [target]"
    echo ""
    echo "Targets:"
    echo "  all            Install dependencies for all modules (default)"
    echo "  shared         Install shared dependencies (pkg module only)"
    echo "  auth-service   Install auth-service dependencies"
    echo "  <service>      Install specific service dependencies"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0              # Install all dependencies"
    echo "  $0 shared       # Install only shared pkg dependencies"
    echo "  $0 auth-service # Install auth-service dependencies"
}

install_shared() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Installing shared dependencies (pkg)${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    cd "$ROOT_DIR/pkg"

    echo -e "${YELLOW}Installing database dependencies...${NC}"
    go get github.com/jackc/pgx/v5@$PGX_VERSION
    go get github.com/redis/go-redis/v9@$REDIS_VERSION

    echo -e "${YELLOW}Installing gRPC dependencies...${NC}"
    go get google.golang.org/grpc@$GRPC_VERSION
    go get google.golang.org/protobuf@$PROTOBUF_VERSION

    echo -e "${YELLOW}Installing logging & config...${NC}"
    go get go.uber.org/zap@$ZAP_VERSION
    go get github.com/spf13/viper@$VIPER_VERSION

    echo -e "${YELLOW}Installing utilities...${NC}"
    go get github.com/go-playground/validator/v10@$VALIDATOR_VERSION
    go get github.com/google/uuid@$UUID_VERSION

    echo -e "${YELLOW}Installing observability...${NC}"
    go get go.opentelemetry.io/otel@$OTEL_VERSION
    go get go.opentelemetry.io/otel/trace@$OTEL_VERSION
    go get github.com/prometheus/client_golang@$PROMETHEUS_VERSION

    echo -e "${YELLOW}Running go mod tidy...${NC}"
    go mod tidy

    echo -e "${GREEN}✓ Shared dependencies installed${NC}"
    echo ""
}

install_auth_service() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Installing auth-service dependencies${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    cd "$ROOT_DIR/services/auth-service"

    echo -e "${YELLOW}Installing auth-specific dependencies...${NC}"
    go get firebase.google.com/go/v4@$FIREBASE_VERSION
    go get github.com/golang-jwt/jwt/v5@$JWT_VERSION
    go get golang.org/x/crypto@$CRYPTO_VERSION

    echo -e "${YELLOW}Running go mod tidy...${NC}"
    go mod tidy

    echo -e "${GREEN}✓ auth-service dependencies installed${NC}"
    echo ""
}

install_service() {
    local service=$1
    local service_path="$ROOT_DIR/services/$service"

    if [ ! -d "$service_path" ]; then
        echo -e "${RED}ERROR: Service $service not found at $service_path${NC}"
        return 1
    fi

    if [ ! -f "$service_path/go.mod" ]; then
        echo -e "${RED}ERROR: No go.mod found in $service_path${NC}"
        return 1
    fi

    case "$service" in
        auth-service)
            install_auth_service
            ;;
        user-service)
            echo -e "${BLUE}Installing user-service dependencies...${NC}"
            cd "$service_path"
            go get go.mongodb.org/mongo-driver@$MONGODB_VERSION
            go mod tidy
            echo -e "${GREEN}✓ user-service dependencies installed${NC}"
            ;;
        payment-service)
            echo -e "${BLUE}Installing payment-service dependencies...${NC}"
            cd "$service_path"
            go get github.com/stripe/stripe-go/v76@$STRIPE_VERSION
            go mod tidy
            echo -e "${GREEN}✓ payment-service dependencies installed${NC}"
            ;;
        *)
            echo -e "${YELLOW}No specific dependencies defined for $service${NC}"
            cd "$service_path"
            go mod tidy
            echo -e "${GREEN}✓ $service go.mod tidied${NC}"
            ;;
    esac
}

install_all() {
    # First install shared dependencies
    install_shared

    # Then install service-specific dependencies
    for service_dir in "$ROOT_DIR/services/"*/; do
        if [ -d "$service_dir" ] && [ -f "$service_dir/go.mod" ]; then
            service=$(basename "$service_dir")
            install_service "$service"
        fi
    done

    # Sync workspace
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Syncing workspace${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    cd "$ROOT_DIR"
    go work sync
    echo -e "${GREEN}✓ Workspace synced${NC}"
}

# =============================================================================
# Main
# =============================================================================

TARGET=${1:-"all"}

case "$TARGET" in
    -h|--help)
        usage
        exit 0
        ;;
    all)
        install_all
        ;;
    shared|pkg)
        install_shared
        ;;
    *)
        if [ -d "$ROOT_DIR/services/$TARGET" ]; then
            install_service "$TARGET"
        else
            echo -e "${RED}Unknown target: $TARGET${NC}"
            usage
            exit 1
        fi
        ;;
esac

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         Dependencies installed successfully!           ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
