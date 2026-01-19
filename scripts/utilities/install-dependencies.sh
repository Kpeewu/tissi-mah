#!/bin/bash
# scripts/install-deps.sh
# Script pour installer les dépendances avec les versions définies

set -e

SERVICE=$1

if [ -z "$SERVICE" ]; then
    echo "Usage: ./scripts/utilities/install-deps.sh <service-name>"
    echo "Example: ./scripts/utilities/install-deps.sh auth-service"
    exit 1
fi

SERVICE_PATH="services/$SERVICE"

if [ ! -d "$SERVICE_PATH" ]; then
    echo "❌ Service $SERVICE not found at $SERVICE_PATH"
    exit 1
fi

echo "📦 Installing dependencies for $SERVICE..."
cd "$SERVICE_PATH"

# Versions (référence unique)
GRPC_VERSION="v1.60.1"
PROTOBUF_VERSION="v1.32.0"
VIPER_VERSION="v1.18.2"
LOGRUS_VERSION="v1.9.3"
UUID_VERSION="v1.5.0"
POSTGRES_VERSION="v1.10.9"
MONGODB_VERSION="v1.13.1"
FIREBASE_VERSION="v4.13.0"
GOOGLE_API_VERSION="v0.157.0"
GOLANG_NET_VERSION="v0.20.0"

# Common dependencies
echo "  ➜ Installing common dependencies..."
go get google.golang.org/grpc@$GRPC_VERSION
go get google.golang.org/protobuf@$PROTOBUF_VERSION
go get github.com/spf13/viper@$VIPER_VERSION
go get github.com/sirupsen/logrus@$LOGRUS_VERSION
go get github.com/google/uuid@$UUID_VERSION
go get golang.org/x/net@$GOLANG_NET_VERSION

# Service-specific dependencies
case "$SERVICE" in
    auth-service)
        echo "  ➜ Installing auth-service specific dependencies..."
        go get github.com/lib/pq@$POSTGRES_VERSION
        go get firebase.google.com/go/v4@$FIREBASE_VERSION
        go get google.golang.org/api@$GOOGLE_API_VERSION
        ;;
    
    user-service)
        echo "  ➜ Installing user-service specific dependencies..."
        go get go.mongodb.org/mongo-driver@$MONGODB_VERSION
        ;;
    
    client-service)
        echo "  ➜ Installing client-service specific dependencies..."
        go get github.com/lib/pq@$POSTGRES_VERSION
        ;;
    
    *)
        echo "⚠️  Unknown service: $SERVICE"
        echo "  Only common dependencies installed"
        ;;
esac

echo "✅ Dependencies installed for $SERVICE"
cd ../..