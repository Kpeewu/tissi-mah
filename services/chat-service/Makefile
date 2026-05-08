.PHONY: help run build test proto migrate

SERVICE_NAME := chat-service
MAIN_PATH := ./cmd/server/main.go
PROTO_PATH := ./proto
PROTO_OUT := $(PROTO_PATH)/gen

DATABASE_URL ?= postgresql://dev:dev123@localhost:5440/chat_db?sslmode=disable
REDIS_URL ?= redis://localhost:6390/0
NOTIFICATION_REDIS_URL ?= redis://localhost:6386/0
LOG_LEVEL ?= debug
GRPC_PORT ?= 50065
ENVIRONMENT ?= local
BOOKING_SERVICE_HOST ?= 0.0.0.0
BOOKING_SERVICE_PORT ?= 50058
TRIPS_SERVICE_HOST ?= 0.0.0.0
TRIPS_SERVICE_PORT ?= 50055
SUPPORT_SERVICE_HOST ?= 0.0.0.0
SUPPORT_SERVICE_PORT ?= 50063
CHAT_ENCRYPTION_KEY ?= ""

help:
	@echo "Chat Service — Commandes disponibles"
	@echo "  make run     - Lancer le service"
	@echo "  make build   - Compiler le binaire"
	@echo "  make test    - Lancer les tests"
	@echo "  make proto   - Générer les fichiers proto"
	@echo "  make migrate - Exécuter les migrations"

run:
	@DATABASE_URL=$(DATABASE_URL) REDIS_URL=$(REDIS_URL) \
	 NOTIFICATION_REDIS_URL=$(NOTIFICATION_REDIS_URL) LOG_LEVEL=$(LOG_LEVEL) \
	 GRPC_PORT=$(GRPC_PORT) ENVIRONMENT=$(ENVIRONMENT) \
	 BOOKING_SERVICE_HOST=$(BOOKING_SERVICE_HOST) BOOKING_SERVICE_PORT=$(BOOKING_SERVICE_PORT) \
	 TRIPS_SERVICE_HOST=$(TRIPS_SERVICE_HOST) TRIPS_SERVICE_PORT=$(TRIPS_SERVICE_PORT) \
	 SUPPORT_SERVICE_HOST=$(SUPPORT_SERVICE_HOST) SUPPORT_SERVICE_PORT=$(SUPPORT_SERVICE_PORT) \
	 CHAT_ENCRYPTION_KEY=$(CHAT_ENCRYPTION_KEY) \
	 go run $(MAIN_PATH)

build:
	go build -o bin/$(SERVICE_NAME) $(MAIN_PATH)

test:
	go test ./...

proto:
	@echo "🔧 Génération proto chat-service..."
	@mkdir -p $(PROTO_OUT)
	@export PATH="$$PATH:$$(go env GOPATH)/bin" && \
	 protoc \
	   --proto_path=$(PROTO_PATH) \
	   --go_out=$(PROTO_OUT) \
	   --go_opt=paths=source_relative \
	   --go-grpc_out=$(PROTO_OUT) \
	   --go-grpc_opt=paths=source_relative \
	   chat.proto
	@echo "✅ Proto générés"

migrate:
	@migrate -path ./migrations -database "$(DATABASE_URL)" up
