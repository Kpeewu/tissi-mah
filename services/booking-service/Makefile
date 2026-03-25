.PHONY: help run build test proto migrate

SERVICE_NAME := booking-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
DATABASE_URL ?= postgresql://dev:dev123@localhost:5438/booking_db?sslmode=disable
REDIS_URL ?= redis://localhost:6387
LOG_LEVEL ?= debug
GRPC_PORT ?= 50058
ENVIRONMENT ?= local
USER_SERVICE_HOST ?= 0.0.0.0
USER_SERVICE_PORT ?= 50052
TRIPS_SERVICE_HOST ?= 0.0.0.0
TRIPS_SERVICE_PORT ?= 50056
SERVICE_FEE_PERCENT ?= 10
RECONCILIATION_INTERVAL_SECONDS ?= 300

help:
	@echo "Booking Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"
	@echo "  make migrate  - Exécuter les migrations"

run:
	@echo "🚀 Démarrage de booking-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export USER_SERVICE_HOST=$(USER_SERVICE_HOST) && \
	 export USER_SERVICE_PORT=$(USER_SERVICE_PORT) && \
	 export TRIPS_SERVICE_HOST=$(TRIPS_SERVICE_HOST) && \
	 export TRIPS_SERVICE_PORT=$(TRIPS_SERVICE_PORT) && \
	 export SERVICE_FEE_PERCENT=$(SERVICE_FEE_PERCENT) && \
	 export RECONCILIATION_INTERVAL_SECONDS=$(RECONCILIATION_INTERVAL_SECONDS) && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation de booking-service..."
	@go build -o bin/booking-service $(MAIN_PATH)
	@echo "✅ Binaire créé : bin/booking-service"

test:
	@echo "🧪 Tests booking-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@echo "🔧 Génération proto booking-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"

migrate:
	@echo "📦 Exécution des migrations..."
	@./scripts/run_migration.sh
