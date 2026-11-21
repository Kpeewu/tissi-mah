#!/bin/bash
# scripts/development/setup-local-env.sh
# Setup complete local development environment

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        Tissi-Mah Local Environment Setup              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Function to check command
check_command() {
  if command -v $1 &> /dev/null; then
    echo -e "  ${GREEN}✓${NC} $1 installed"
    return 0
  else
    echo -e "  ${RED}✗${NC} $1 not found"
    return 1
  fi
}

# Function to check go version
check_go_version() {
  if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.21"
    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
      echo -e "  ${GREEN}✓${NC} Go ${GO_VERSION} (>= ${REQUIRED_VERSION})"
      return 0
    else
      echo -e "  ${RED}✗${NC} Go ${GO_VERSION} (require >= ${REQUIRED_VERSION})"
      return 1
    fi
  else
    echo -e "  ${RED}✗${NC} Go not installed"
    return 1
  fi
}

# Step 1: Check prerequisites
echo -e "${YELLOW}Step 1: Checking prerequisites...${NC}"
echo ""

MISSING_TOOLS=()

check_go_version || MISSING_TOOLS+=("go>=1.21")
check_command "docker" || MISSING_TOOLS+=("docker")
check_command "docker-compose" || MISSING_TOOLS+=("docker-compose")
check_command "protoc" || MISSING_TOOLS+=("protoc")
check_command "golangci-lint" || MISSING_TOOLS+=("golangci-lint")
check_command "git" || MISSING_TOOLS+=("git")

echo ""

if [ ${#MISSING_TOOLS[@]} -ne 0 ]; then
  echo -e "${RED}❌ Missing required tools:${NC}"
  for tool in "${MISSING_TOOLS[@]}"; do
    echo "  - $tool"
  done
  echo ""
  echo -e "${YELLOW}Installation instructions:${NC}"
  echo ""
  echo "Go (1.21+):"
  echo "  macOS: brew install go"
  echo "  Linux: https://golang.org/doc/install"
  echo ""
  echo "Docker:"
  echo "  macOS: brew install --cask docker"
  echo "  Linux: https://docs.docker.com/engine/install/"
  echo ""
  echo "Protoc:"
  echo "  macOS: brew install protobuf"
  echo "  Linux: sudo apt-get install protobuf-compiler"
  echo ""
  echo "golangci-lint:"
  echo "  brew install golangci-lint"
  echo "  or: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin"
  echo ""
  exit 1
fi

echo -e "${GREEN}✅ All prerequisites installed${NC}"
echo ""

# Step 2: Setup Go workspace
echo -e "${YELLOW}Step 2: Setting up Go workspace...${NC}"
echo ""

if [ ! -f "go.work" ]; then
  echo "Creating go.work..."
  go work init
  go work use ./services/auth-service
  go work use ./services/user-service
  go work use ./services/client-service
  echo -e "${GREEN}✓${NC} go.work created"
else
  echo -e "${GREEN}✓${NC} go.work already exists"
fi

echo ""

# Step 3: Install Go dependencies
echo -e "${YELLOW}Step 3: Installing Go dependencies...${NC}"
echo ""

SERVICES=("auth-service" "user-service" "client-service")

for service in "${SERVICES[@]}"; do
  if [ -d "services/$service" ]; then
    echo "Installing dependencies for $service..."
    cd "services/$service"
    if [ -f "go.mod" ]; then
      go mod download
      echo -e "${GREEN}✓${NC} $service dependencies installed"
    else
      echo -e "${YELLOW}⚠${NC}  $service go.mod not found, skipping"
    fi
    cd ../..
  fi
done

echo ""

# Step 4: Install protoc plugins
echo -e "${YELLOW}Step 4: Installing protoc plugins...${NC}"
echo ""

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

echo -e "${GREEN}✓${NC} protoc-gen-go installed"
echo -e "${GREEN}✓${NC} protoc-gen-go-grpc installed"
echo ""

# Step 5: Setup environment files
echo -e "${YELLOW}Step 5: Setting up environment files...${NC}"
echo ""

if [ ! -f ".env" ]; then
  if [ -f ".env.example" ]; then
    cp .env.example .env
    echo -e "${GREEN}✓${NC} .env created from .env.example"
    echo -e "${YELLOW}⚠${NC}  Please edit .env and configure your values"
  else
    echo -e "${RED}✗${NC} .env.example not found"
  fi
else
  echo -e "${GREEN}✓${NC} .env already exists"
fi

echo ""

# Step 6: Start databases
echo -e "${YELLOW}Step 6: Starting databases...${NC}"
echo ""

docker-compose up -d

echo ""
echo "Waiting for databases to be ready..."
sleep 10

# Check database health
if docker-compose ps | grep -q "healthy"; then
  echo -e "${GREEN}✓${NC} Databases are healthy"
else
  echo -e "${YELLOW}⚠${NC}  Some databases may still be starting..."
fi

echo ""

# Step 7: Generate proto files
echo -e "${YELLOW}Step 7: Generating protobuf files...${NC}"
echo ""

./scripts/development/generate-all-protos.sh

echo ""

# Step 8: Run migrations (if needed)
echo -e "${YELLOW}Step 8: Database migrations...${NC}"
echo ""

read -p "Run database migrations? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  for service in "${SERVICES[@]}"; do
    if [ -d "services/$service/migrations" ]; then
      echo "Running migrations for $service..."
      cd "services/$service"
      if [ -f "scripts/run_migration.sh" ]; then
        ./scripts/run_migration.sh up
      fi
      cd ../..
    fi
  done
else
  echo "Skipping migrations"
fi

echo ""

# Summary
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                 Setup Complete! 🎉                     ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}Your development environment is ready!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo ""
echo "1. Edit .env file with your configuration"
echo "2. Start a service:"
echo "   ${BLUE}make run-auth${NC}     # Start auth-service"
echo "   ${BLUE}make run-user${NC}     # Start user-service"
echo "   ${BLUE}make run-client${NC}   # Start client-service"
echo ""
echo "3. Run tests:"
echo "   ${BLUE}make test-all${NC}     # Run all tests"
echo ""
echo "4. View database logs:"
echo "   ${BLUE}make deps-logs${NC}    # View database logs"
echo ""
echo -e "${YELLOW}Documentation:${NC}"
echo "  - docs/development/getting-started.md"
echo "  - docs/development/environment-setup.md"
echo ""