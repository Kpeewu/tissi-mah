#!/bin/bash
# scripts/geo/prepare-osm-data.sh
#
# Pipeline de préparation des données OSM pour le geolocation-service :
#   1. Télécharge les extracts Geofabrik (Togo, Ghana, Bénin, Burkina Faso).
#   2. Merge les 4 extracts en un seul fichier west-africa.osm.pbf.
#   3. Pre-traitement OSRM (mode MLD) → produit west-africa.osrm + fichiers
#      compagnons (.osrm.cell_metrics, .osrm.cells, .osrm.geometry, etc.).
#
# Sortie attendue dans $DATA_DIR (défaut: /data) :
#   - west-africa.osm.pbf (~1 Go)
#   - west-africa.osrm + ~10 fichiers compagnons (~5-7 Go au total)
#
# Pré-requis (sur la machine d'exécution) :
#   - curl
#   - osmium (paquet osmium-tool)
#   - osrm-extract, osrm-partition, osrm-customize (image osrm/osrm-backend)
#
# Durée typique : 2-4h pour les 4 pays sur un VPS modeste.
#
# Utilisation :
#   ./prepare-osm-data.sh              # sortie dans /data
#   DATA_DIR=/tmp/osrm ./prepare-osm-data.sh
#   STEPS=download,merge ./prepare-osm-data.sh    # skip OSRM steps
#
# La variable STEPS permet d'exécuter sélectivement (défaut: tous les steps).

set -euo pipefail

DATA_DIR="${DATA_DIR:-/data}"
WORK_DIR="${WORK_DIR:-${DATA_DIR}/work}"
STEPS="${STEPS:-download,merge,extract,partition,customize}"

# Profile OSRM (chemin par défaut dans l'image osrm/osrm-backend).
OSRM_PROFILE="${OSRM_PROFILE:-/opt/car.lua}"

COUNTRIES=(togo ghana benin burkina-faso)
GEOFABRIK_BASE="https://download.geofabrik.de/africa"

log() { printf '\033[1;34m[osm-prep]\033[0m %s\n' "$*"; }
err() { printf '\033[1;31m[osm-prep ERR]\033[0m %s\n' "$*" >&2; }

has_step() { [[ ",${STEPS}," == *",${1},"* ]]; }

mkdir -p "$DATA_DIR" "$WORK_DIR"
cd "$WORK_DIR"

# =============================================================================
# Step 1 — Download extracts Geofabrik
# =============================================================================
if has_step download; then
  log "Step 1/5 — download extracts Geofabrik (4 pays)"
  for country in "${COUNTRIES[@]}"; do
    file="${country}-latest.osm.pbf"
    if [[ -f "$file" ]]; then
      log "  ${country}: déjà téléchargé (réutilise ${file})"
      continue
    fi
    log "  ${country}: téléchargement..."
    curl -fSL --retry 3 --retry-delay 30 \
      -o "$file" \
      "${GEOFABRIK_BASE}/${file}"
  done
fi

# =============================================================================
# Step 2 — Merge 4 extracts → west-africa.osm.pbf
# =============================================================================
if has_step merge; then
  log "Step 2/5 — merge des 4 extracts"
  inputs=()
  for country in "${COUNTRIES[@]}"; do
    inputs+=("${country}-latest.osm.pbf")
  done
  if ! command -v osmium &>/dev/null; then
    err "osmium-tool requis (apt-get install osmium-tool ou paquet équivalent)"
    exit 1
  fi
  osmium merge --overwrite -o west-africa.osm.pbf "${inputs[@]}"
  log "  west-africa.osm.pbf produit ($(du -h west-africa.osm.pbf | cut -f1))"
fi

# =============================================================================
# Step 3 — OSRM extract (parse OSM → réseau routier)
# =============================================================================
if has_step extract; then
  log "Step 3/5 — osrm-extract (profile: car.lua)"
  if ! command -v osrm-extract &>/dev/null; then
    err "osrm-extract requis (image osrm/osrm-backend ou paquet osrm-tools)"
    exit 1
  fi
  osrm-extract -p "$OSRM_PROFILE" west-africa.osm.pbf
fi

# =============================================================================
# Step 4 — OSRM partition (MLD : découpe en cellules)
# =============================================================================
if has_step partition; then
  log "Step 4/5 — osrm-partition (Multi-Level Dijkstra)"
  osrm-partition west-africa.osrm
fi

# =============================================================================
# Step 5 — OSRM customize (calcul des coûts)
# =============================================================================
if has_step customize; then
  log "Step 5/5 — osrm-customize"
  osrm-customize west-africa.osrm
fi

# =============================================================================
# Move final artifacts to DATA_DIR
# =============================================================================
log "Déplacement des artifacts vers ${DATA_DIR}"
# La sortie d'osrm-* produit west-africa.osrm + fichiers .osrm.* dans le cwd.
mv -f west-africa.osm.pbf west-africa.osrm* "$DATA_DIR"/ 2>/dev/null || true

log "✅ Préparation OSM terminée. Fichiers dans ${DATA_DIR}:"
ls -lh "$DATA_DIR"/west-africa.osrm* | head -20
