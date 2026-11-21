.PHONY: help run build test proto migrate

SERVICE_NAME := moderation-service
MAIN_PATH    := ./cmd/server/main.go

DATABASE_URL ?= postgresql://dev:dev123@localhost:5443/moderation_db?sslmode=disable
REDIS_URL    ?= redis://localhost:6381/0
LOG_LEVEL    ?= debug
GRPC_PORT    ?= 50066
ENVIRONMENT  ?= local

help:
	@echo "Moderation Service - Commandes disponibles"
	@echo "  make run     - Lancer le service"
	@echo "  make build   - Compiler le binaire"
	@echo "  make test    - Lancer les tests"
	@echo "  make proto   - Générer les fichiers proto"
	@echo "  make migrate - Exécuter les migrations"

run:
	@echo "Démarrage de moderation-service..."
	@export DATABASE_URL=$(DATABASE_URL) REDIS_URL=$(REDIS_URL) \
	        LOG_LEVEL=$(LOG_LEVEL) GRPC_PORT=$(GRPC_PORT) ENVIRONMENT=$(ENVIRONMENT) && \
	 go run $(MAIN_PATH)

build:
	@go build -o bin/moderation-service $(MAIN_PATH)

test:
	@go test ./... -v -race

proto:
	@echo "Génération proto moderation-service..."
	@./scripts/generate_proto.sh

migrate:
	@./scripts/run_migration.sh
