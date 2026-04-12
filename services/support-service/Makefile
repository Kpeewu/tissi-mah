.PHONY: help run build test proto

SERVICE_NAME := support-service
MAIN_PATH := ./cmd/server/main.go

DATABASE_URL ?= postgresql://dev:dev123@localhost:5441/support_db?sslmode=disable
REDIS_URL ?= redis://localhost:6390/0
ENVIRONMENT ?= local
LOG_LEVEL ?= debug
GRPC_PORT ?= 50063
EMAIL_SERVICE_HOST ?= 0.0.0.0
EMAIL_SERVICE_PORT ?= 50061
JWT_SECRET ?= change-me-dev-secret-min-32-bytes-long!!

help:
	@echo "Support Service - Commandes disponibles"
	@echo "  make run    - Lancer le service"
	@echo "  make build  - Compiler le binaire"
	@echo "  make test   - Lancer les tests"
	@echo "  make proto  - Générer les fichiers proto"

run:
	@echo "🚀 Démarrage de support-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export EMAIL_SERVICE_HOST=$(EMAIL_SERVICE_HOST) && \
	 export EMAIL_SERVICE_PORT=$(EMAIL_SERVICE_PORT) && \
	 export JWT_SECRET=$(JWT_SECRET) && \
	 go run $(MAIN_PATH)

build:
	@go build -o bin/support-service $(MAIN_PATH)

test:
	@go test ./... -v -race

proto:
	@./scripts/generate_proto.sh
