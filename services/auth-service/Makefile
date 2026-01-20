.PHONY: help run build test proto migrate

SERVICE_NAME := auth-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
DATABASE_URL ?= postgresql://dev:dev123@localhost:5432/auth_db?sslmode=disable
REDIS_URL ?= redis://localhost:6379
LOG_LEVEL ?= debug
GRPC_PORT ?= 50051

help:
	@echo "Auth Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"
	@echo "  make migrate  - Exécuter les migrations"

run:
	@echo "🚀 Démarrage de auth-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation de auth-service..."
	@go build -o bin/auth-service $(MAIN_PATH)
	@echo "✅ Binaire créé : bin/auth-service"

test:
	@echo "🧪 Tests auth-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 go test ./tests/unit/... -v -race -coverprofile=coverage.out
	@go test ./tests/integration/... -v

proto:
	@echo "🔧 Génération proto auth-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"

migrate:
	@echo "📦 Exécution des migrations..."
	@./scripts/run_migration.sh