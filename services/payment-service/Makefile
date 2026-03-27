.PHONY: help run build test proto migrate

SERVICE_NAME := payment-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
DATABASE_URL ?= postgresql://dev:dev123@localhost:5439/payment_db?sslmode=disable
REDIS_URL ?= redis://localhost:6388
LOG_LEVEL ?= debug
GRPC_PORT ?= 50059
ENVIRONMENT ?= local
BOOKING_SERVICE_HOST ?= 0.0.0.0
BOOKING_SERVICE_PORT ?= 50058
USER_SERVICE_HOST ?= 0.0.0.0
USER_SERVICE_PORT ?= 50052
FEDAPAY_API_URL ?= https://sandbox-api.fedapay.com
FEDAPAY_API_KEY ?= sk_sandbox_xxxxx
FEDAPAY_WEBHOOK_SECRET ?= whsec_xxxxx

help:
	@echo "Payment Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"
	@echo "  make migrate  - Exécuter les migrations"

run:
	@echo "🚀 Démarrage de payment-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export BOOKING_SERVICE_HOST=$(BOOKING_SERVICE_HOST) && \
	 export BOOKING_SERVICE_PORT=$(BOOKING_SERVICE_PORT) && \
	 export USER_SERVICE_HOST=$(USER_SERVICE_HOST) && \
	 export USER_SERVICE_PORT=$(USER_SERVICE_PORT) && \
	 export FEDAPAY_API_URL=$(FEDAPAY_API_URL) && \
	 export FEDAPAY_API_KEY=$(FEDAPAY_API_KEY) && \
	 export FEDAPAY_WEBHOOK_SECRET=$(FEDAPAY_WEBHOOK_SECRET) && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation de payment-service..."
	@go build -o bin/payment-service $(MAIN_PATH)
	@echo "✅ Binaire créé : bin/payment-service"

test:
	@echo "🧪 Tests payment-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@echo "🔧 Génération proto payment-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"

migrate:
	@echo "📦 Exécution des migrations..."
	@./scripts/run_migration.sh
