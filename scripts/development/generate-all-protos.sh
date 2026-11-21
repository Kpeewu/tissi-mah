#!/bin/bash
# scripts/development/generate-all-protos.sh
# Génère les fichiers proto pour tous les services, synchronise vers api-gateway et régénère les bindings gateway

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Se placer à la racine du monorepo (au cas où le script est appelé depuis un autre répertoire)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT_DIR"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         Génération des protos - Tissi-Mah              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Services à traiter (dans l'ordre)
SERVICES=(
  "auth-service"
  "user-service"
  "file-service"
  "kyc-service"
  "booking-service"
  "trips-service"
  "rating-service"
  "vehicle-service"
)

FAILED=()
SKIPPED=()

# Génération des protos par service
echo -e "${YELLOW}Étape 1/3 — Génération des protos par service${NC}"
echo ""

for service in "${SERVICES[@]}"; do
  SERVICE_DIR="services/$service"

  if [ ! -d "$SERVICE_DIR" ]; then
    echo -e "${YELLOW}⚠  $service — répertoire introuvable, ignoré${NC}"
    SKIPPED+=("$service")
    continue
  fi

  if [ ! -f "$SERVICE_DIR/Makefile" ]; then
    echo -e "${YELLOW}⚠  $service — pas de Makefile, ignoré${NC}"
    SKIPPED+=("$service")
    continue
  fi

  # Vérifie que le service a bien un target proto dans son Makefile
  if ! grep -q "^proto:" "$SERVICE_DIR/Makefile" 2>/dev/null; then
    echo -e "${YELLOW}⚠  $service — pas de target 'proto' dans le Makefile, ignoré${NC}"
    SKIPPED+=("$service")
    continue
  fi

  echo -e "${BLUE}→ Génération : $service${NC}"
  if (cd "$SERVICE_DIR" && make proto --no-print-directory 2>&1); then
    echo -e "${GREEN}  ✓ $service${NC}"
  else
    echo -e "${RED}  ✗ $service — échec de la génération${NC}"
    FAILED+=("$service")
  fi
  echo ""
done

# Synchronisation vers api-gateway
echo -e "${YELLOW}Étape 2/3 — Synchronisation des protos vers api-gateway${NC}"
if make proto-sync --no-print-directory; then
  echo -e "${GREEN}✓ Synchronisation terminée${NC}"
else
  echo -e "${RED}✗ Échec de la synchronisation${NC}"
  FAILED+=("proto-sync")
fi
echo ""

# Régénération des bindings grpc-gateway
echo -e "${YELLOW}Étape 3/3 — Génération proto api-gateway (grpc-gateway)${NC}"
if make proto-gateway --no-print-directory; then
  echo -e "${GREEN}✓ Bindings gateway générés${NC}"
else
  echo -e "${RED}✗ Échec de la génération gateway${NC}"
  FAILED+=("proto-gateway")
fi
echo ""

# Résumé
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                      Résumé                           ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ ${#SKIPPED[@]} -gt 0 ]; then
  echo -e "${YELLOW}Ignorés (${#SKIPPED[@]}) :${NC}"
  for s in "${SKIPPED[@]}"; do
    echo "  - $s"
  done
  echo ""
fi

if [ ${#FAILED[@]} -gt 0 ]; then
  echo -e "${RED}Échecs (${#FAILED[@]}) :${NC}"
  for s in "${FAILED[@]}"; do
    echo "  - $s"
  done
  echo ""
  exit 1
fi

echo -e "${GREEN}✅ Tous les protos générés avec succès${NC}"
