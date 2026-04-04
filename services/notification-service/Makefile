.PHONY: help run build test proto migrate migrate-down

SERVICE_NAME := notification-service
MAIN_PATH := ./cmd/server/main.go

LOG_LEVEL ?= debug
GRPC_PORT ?= 50060

help:
	@echo "Notification Service - Commandes disponibles"
	@echo ""
	@echo "  make run          - Lancer le service"
	@echo "  make build        - Compiler le binaire"
	@echo "  make test         - Lancer les tests"
	@echo "  make proto        - Générer les fichiers proto"
	@echo "  make migrate      - Appliquer les migrations UP"
	@echo "  make migrate-down - Appliquer les migrations DOWN"

run:
	@echo "🚀 Démarrage de notification-service..."
	@go run $(MAIN_PATH)

build:
	@./scripts/build.sh

test:
	@go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@./scripts/generate_proto.sh

migrate:
	@./scripts/run_migration.sh up

migrate-down:
	@./scripts/run_migration.sh down
