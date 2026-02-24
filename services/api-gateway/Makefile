.PHONY: run build proto clean

# Couleurs
GREEN='\033[0;32m'
NC='\033[0m'

run:
	@echo "🚀 Démarrage api-gateway..."
	@go run ./cmd/server/

build:
	@echo "🔧 Build api-gateway..."
	@go build -o bin/api-gateway ./cmd/server/
	@echo "✅ Build terminé: bin/api-gateway"

proto:
	@echo "🔧 Génération proto api-gateway..."
	@bash scripts/generate_proto.sh
	@echo "✅ Proto générés"

clean:
	@rm -rf bin/
