.PHONY: help run build test proto migrate

SERVICE_NAME := payment-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Charger le .env s'il existe
-include .env
export

# Valeurs par défaut (écrasées par le .env)
DATABASE_URL ?= postgresql://dev:dev123@localhost:5439/payment_db?sslmode=disable
REDIS_URL ?= redis://localhost:6388
LOG_LEVEL ?= debug
GRPC_PORT ?= 50059
ENVIRONMENT ?= local
BOOKING_SERVICE_HOST ?= 0.0.0.0
BOOKING_SERVICE_PORT ?= 50058
USER_SERVICE_HOST ?= 0.0.0.0
USER_SERVICE_PORT ?= 50052
FEDAPAY_SANDBOX_API_URL ?= https://sandbox-api.fedapay.com
FEDAPAY_SANDBOX_API_KEY ?= sk_sandbox_xxxxx
FEDAPAY_SANDBOX_WEBHOOK_SECRET ?= whsec_sandbox_xxxxx
FEDAPAY_LIVE_API_URL ?= https://api.fedapay.com
FEDAPAY_LIVE_API_KEY ?= sk_live_xxxxx
FEDAPAY_LIVE_WEBHOOK_SECRET ?= whsec_live_xxxxx

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
	@go run $(MAIN_PATH)

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
