.PHONY: help run build test proto

SERVICE_NAME := vehicle-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
MONGODB_URL ?= mongodb://dev:dev123@localhost:27017/vehicle_db?authSource=admin
REDIS_URL ?= redis://localhost:6380
LOG_LEVEL ?= debug
GRPC_PORT ?= 50052

help:
	@echo "vehicle Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"

run:
	@echo "🚀 Démarrage de vehicle-service..."
	@export MONGODB_URL=$(MONGODB_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=local && \
	 export FIREBASE_PROJECT_ID=tissi-mah-dev && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation de vehicle-service..."
	@go build -o bin/vehicle-service $(MAIN_PATH)
	@echo "✅ Binaire créé : bin/vehicle-service"

test:
	@echo "🧪 Tests vehicle-service..."
	@go test ./tests/unit/... -v -race -coverprofile=coverage.out
	@go test ./tests/integration/... -v

proto:
	@echo "🔧 Génération proto vehicle-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"
