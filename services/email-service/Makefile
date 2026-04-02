.PHONY: help run build test proto

SERVICE_NAME := email-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
LOG_LEVEL ?= debug
GRPC_PORT ?= 50061

help:
	@echo "Email Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"

run:
	@echo "🚀 Démarrage de email-service..."
	@export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation de email-service..."
	@go build -o bin/email-service $(MAIN_PATH)
	@echo "✅ Binaire créé : bin/email-service"

test:
	@echo "🧪 Tests email-service..."
	@go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@echo "🔧 Génération proto email-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"
