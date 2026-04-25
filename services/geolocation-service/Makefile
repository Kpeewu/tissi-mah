.PHONY: help run build test proto

SERVICE_NAME := geolocation-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

# Variables d'environnement (override avec .env)
OSRM_URL ?= http://localhost:5000
NOMINATIM_URL ?= http://localhost:8080
REDIS_URL ?= redis://localhost:6385
LOG_LEVEL ?= debug
GRPC_PORT ?= 50064
ENVIRONMENT ?= local
GEOCODE_DEFAULT_COUNTRIES ?= tg,gh,bj,bf

help:
	@echo "Geolocation Service - Commandes disponibles"
	@echo ""
	@echo "  make run    - Lancer le service"
	@echo "  make build  - Compiler le binaire"
	@echo "  make test   - Lancer les tests"
	@echo "  make proto  - Générer les fichiers proto"

run:
	@echo "🚀 Démarrage de geolocation-service..."
	@export OSRM_URL=$(OSRM_URL) && \
	 export NOMINATIM_URL=$(NOMINATIM_URL) && \
	 export REDIS_URL=$(REDIS_URL) && \
	 export LOG_LEVEL=$(LOG_LEVEL) && \
	 export GRPC_PORT=$(GRPC_PORT) && \
	 export ENVIRONMENT=$(ENVIRONMENT) && \
	 export GEOCODE_DEFAULT_COUNTRIES=$(GEOCODE_DEFAULT_COUNTRIES) && \
	 go run $(MAIN_PATH)

build:
	@echo "🔨 Compilation..."
	@CGO_ENABLED=0 go build -o bin/$(SERVICE_NAME) $(MAIN_PATH)

test:
	@echo "🧪 Tests unitaires..."
	@go test ./tests/unit/... -race -count=1

proto:
	@echo "🔧 Génération proto geolocation-service..."
	@./scripts/generate_proto.sh
	@echo "✅ Fichiers proto générés"
