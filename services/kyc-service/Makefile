.PHONY: help run build test proto

SERVICE_NAME := kyc-service
MAIN_PATH := ./cmd/server/main.go

# Variables d'environnement (override avec .env)
REDIS_URL ?= redis://localhost:6384/0
LOG_LEVEL ?= debug
GRPC_PORT ?= 50055
ENVIRONMENT ?= local
FILE_SERVICE_HOST ?= 0.0.0.0
FILE_SERVICE_PORT ?= 50053
PERSONA_API_KEY ?= persona_sandbox_xxx
PERSONA_WEBHOOK_SECRET ?= whsec_xxx

help:
	@echo "KYC Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"

run:
	@echo "Démarrage de kyc-service..."
	@export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export FILE_SERVICE_HOST=$(FILE_SERVICE_HOST) && \
	 export FILE_SERVICE_PORT=$(FILE_SERVICE_PORT) && \
	 export PERSONA_API_KEY=$(PERSONA_API_KEY) && \
	 export PERSONA_WEBHOOK_SECRET=$(PERSONA_WEBHOOK_SECRET) && \
	 go run $(MAIN_PATH)

build:
	@echo "Compilation de kyc-service..."
	@go build -o bin/kyc-service $(MAIN_PATH)
	@echo "Binaire créé : bin/kyc-service"

test:
	@echo "Tests kyc-service..."
	@go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@echo "Génération proto kyc-service..."
	@./scripts/generate_proto.sh
	@echo "Fichiers proto générés"
