.PHONY: help run build test proto migrate

SERVICE_NAME := file-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
DATABASE_URL ?= postgresql://dev:dev123@localhost:5434/file_db?sslmode=disable
REDIS_URL ?= redis://localhost:6383/0
LOG_LEVEL ?= debug
GRPC_PORT ?= 50053
ENVIRONMENT ?= local
S3_BUCKET ?= tissi-mah-files
S3_REGION ?= us-east-1
S3_ACCESS_KEY ?= minioadmin
S3_SECRET_KEY ?= minioadmin
S3_ENDPOINT ?= http://localhost:9000
S3_FORCE_PATH_STYLE ?= true

help:
	@echo "File Service - Commandes disponibles"
	@echo ""
	@echo "  make run      - Lancer le service"
	@echo "  make build    - Compiler le binaire"
	@echo "  make test     - Lancer les tests"
	@echo "  make proto    - Générer les fichiers proto"

run:
	@echo "Démarrage de file-service..."
	@export DATABASE_URL=$(DATABASE_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export S3_BUCKET=$(S3_BUCKET) && \
	 export S3_REGION=$(S3_REGION) && \
	 export S3_ACCESS_KEY=$(S3_ACCESS_KEY) && \
	 export S3_SECRET_KEY=$(S3_SECRET_KEY) && \
	 export S3_ENDPOINT=$(S3_ENDPOINT) && \
	 export S3_FORCE_PATH_STYLE=$(S3_FORCE_PATH_STYLE) && \
	 go run $(MAIN_PATH)

build:
	@echo "Compilation de file-service..."
	@go build -o bin/file-service $(MAIN_PATH)
	@echo "Binaire créé : bin/file-service"

test:
	@echo "Tests file-service..."
	@go test ./tests/unit/... -v -race -coverprofile=coverage.out

proto:
	@echo "Génération proto file-service..."
	@./scripts/generate_proto.sh
	@echo "Fichiers proto générés"
