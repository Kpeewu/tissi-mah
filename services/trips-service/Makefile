.PHONY: help run build test proto migrate

SERVICE_NAME := trips-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
DATABASE_URL ?= postgresql://dev:dev123@localhost:5436/trips_db?sslmode=disable
REDIS_URL ?= redis://localhost:6385
LOG_LEVEL ?= debug
GRPC_PORT ?= 50056
ENVIRONMENT ?= local
USER_SERVICE_HOST ?= 0.0.0.0
USER_SERVICE_PORT ?= 50052
VEHICLE_SERVICE_HOST ?= 0.0.0.0
VEHICLE_SERVICE_PORT ?= 50055

help:
	@echo "Trips Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"
	@echo "  make migrate  - Exécuter les migrations"

run:
	@echo "🚀 Démarrage de trips-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export USER_SERVICE_HOST=$(USER_SERVICE_HOST) && \
	 export USER_SERVICE_PORT=$(USER_SERVICE_PORT) && \
	 export VEHICLE_SERVICE_HOST=$(VEHICLE_SERVICE_HOST) && \
	 export VEHICLE_SERVICE_PORT=$(VEHICLE_SERVICE_PORT) && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation de trips-service..."
	@go build -o bin/trips-service $(MAIN_PATH)
	@echo "✅ Binaire créé : bin/trips-service"

test:
	@echo "🧪 Tests trips-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@echo "🔧 Génération proto trips-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"

migrate:
	@echo "📦 Exécution des migrations..."
	@./scripts/run_migration.sh
