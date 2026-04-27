#!/bin/bash
# =============================================================================
# scripts/geo/init-geo-backends.sh
#
# Initialise Nominatim (geocoding) et TileServer GL (tuiles vectorielles)
# dans le cluster K3s, en cohérence avec les manifests de
# infrastructure/manifests/nominatim/ et infrastructure/manifests/tileserver/.
#
# Prérequis :
#   - kubectl configuré sur le bon contexte
#   - Le PVC osrm-data existe ET contient west-africa.osm.pbf
#     (produit par le Job osm-data-prep ou téléchargé manuellement)
#   - Docker pull access pour mediagis/nominatim:4.4 et maptiler/tileserver-gl
#
# Usage :
#   ./scripts/geo/init-geo-backends.sh [options]
#
# Options :
#   --namespace NS          Namespace K8s cible (défaut: default)
#   --nominatim-password PW Mot de passe Postgres Nominatim (requis)
#   --osm-pvc NOM           Nom du PVC contenant west-africa.osm.pbf (défaut: osrm-data)
#   --skip-tiles            Sauter la génération des tuiles
#   --skip-nominatim        Sauter le déploiement de Nominatim
#   --dry-run               Afficher les manifests sans les appliquer
#
# Exemples :
#   # Déploiement complet en namespace default
#   ./scripts/geo/init-geo-backends.sh --nominatim-password "S3cr3t"
#
#   # Namespace dev avec mot de passe
#   ./scripts/geo/init-geo-backends.sh --namespace dev --nominatim-password "S3cr3t"
#
#   # Tuiles seulement (si Nominatim est déjà déployé)
#   ./scripts/geo/init-geo-backends.sh --skip-nominatim --nominatim-password ""
# =============================================================================

set -euo pipefail

# === Couleurs ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()   { printf "${BLUE}[geo-init]${NC} %s\n" "$*"; }
ok()    { printf "${GREEN}[geo-init ✓]${NC} %s\n" "$*"; }
warn()  { printf "${YELLOW}[geo-init ⚠]${NC} %s\n" "$*"; }
err()   { printf "${RED}[geo-init ✗]${NC} %s\n" "$*" >&2; }
die()   { err "$*"; exit 1; }

# === Valeurs par défaut ===
NAMESPACE="default"
NOMINATIM_PASSWORD=""
OSM_PVC="osrm-data"
SKIP_TILES=false
SKIP_NOMINATIM=false
SKIP_CHECK=false
DRY_RUN=false

# === Parse des arguments ===
while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)            NAMESPACE="$2";           shift 2 ;;
    --nominatim-password)   NOMINATIM_PASSWORD="$2";  shift 2 ;;
    --osm-pvc)              OSM_PVC="$2";             shift 2 ;;
    --skip-tiles)           SKIP_TILES=true;          shift   ;;
    --skip-nominatim)       SKIP_NOMINATIM=true;      shift   ;;
    --skip-check)           SKIP_CHECK=true;          shift   ;;
    --dry-run)              DRY_RUN=true;             shift   ;;
    *) die "Option inconnue: $1. Utilise --help pour l'aide." ;;
  esac
done

# === Validation ===
if [[ "$SKIP_NOMINATIM" == false && -z "$NOMINATIM_PASSWORD" ]]; then
  die "--nominatim-password est requis (sauf si --skip-nominatim)"
fi

KUBECTL="kubectl"
if [[ "$DRY_RUN" == true ]]; then
  KUBECTL="kubectl --dry-run=client -o yaml"
  warn "Mode dry-run activé — aucune ressource ne sera créée"
fi

# === Vérification kubectl ===
if ! command -v kubectl &>/dev/null; then
  die "kubectl introuvable. Installe-le et configure le contexte."
fi
log "Contexte K8s : $(kubectl config current-context)"
log "Namespace    : $NAMESPACE"

# =============================================================================
# 1. Vérification que le PVC source OSM existe et contient le .osm.pbf
# =============================================================================
log "Étape 1/6 — Vérification du PVC OSM source ($OSM_PVC)"

if ! kubectl get pvc "$OSM_PVC" -n "$NAMESPACE" &>/dev/null; then
  die "PVC '$OSM_PVC' introuvable dans le namespace '$NAMESPACE'.\n" \
      "Lance d'abord le Job osm-data-prep:\n" \
      "  kubectl apply -f infrastructure/manifests/osm-data-prep/job.yaml -n $NAMESPACE\n" \
      "  kubectl wait --for=condition=complete job/osm-data-prep -n $NAMESPACE --timeout=4h"
fi
ok "PVC $OSM_PVC présent"

# Vérifier que le .osm.pbf est bien dans le PVC.
# Stratégie : si un pod utilise déjà le PVC (RWO), on tente un exec dessus ;
# sinon on lance un pod temporaire sans -it pour éviter les problèmes de TTY.
log "Vérification de west-africa.osm.pbf dans le PVC..."
PBF_CHECK="MISSING"
if [[ "$SKIP_CHECK" == true ]]; then
  PBF_CHECK="OK (skipped)"
  warn "Vérification OSM ignorée (--skip-check)"
else
  # Chercher un pod qui monte déjà ce PVC (évite le problème RWO)
  EXISTING_POD=$(kubectl get pods -n "$NAMESPACE" -o json 2>/dev/null | \
    python3 -c "
import json,sys
pods=json.load(sys.stdin)
for p in pods.get('items',[]):
  phase=p.get('status',{}).get('phase','')
  if phase not in ('Running','Pending'): continue
  for v in p.get('spec',{}).get('volumes',[]):
    pvc=v.get('persistentVolumeClaim',{}).get('claimName','')
    if pvc=='$OSM_PVC':
      print(p['metadata']['name'])
      break
" 2>/dev/null | head -1)

  if [[ -n "$EXISTING_POD" ]]; then
    log "PVC monté par $EXISTING_POD — vérification via exec"
    RESULT=$(kubectl exec -n "$NAMESPACE" "$EXISTING_POD" -- \
      ls /data/west-africa.osm.pbf 2>/dev/null && echo "OK" || echo "MISSING")
    [[ "$RESULT" == *"OK"* || "$RESULT" == *"west-africa"* ]] && PBF_CHECK="OK"
  else
    # Aucun pod n'utilise le PVC — lancer un pod de vérification (sans -it)
    CHECK_POD="geo-check-$$"
    kubectl run "$CHECK_POD" \
      --image=busybox \
      --restart=Never \
      -n "$NAMESPACE" \
      --overrides='{
        "spec": {
          "containers": [{"name":"c","image":"busybox",
            "command":["sh","-c","ls /osm/west-africa.osm.pbf 2>/dev/null && echo OK || echo MISSING"],
            "volumeMounts":[{"name":"osm","mountPath":"/osm","readOnly":true}]}],
          "volumes":[{"name":"osm","persistentVolumeClaim":{"claimName":"'"$OSM_PVC"'","readOnly":true}}]
        }
      }' &>/dev/null || true
    sleep 10
    RESULT=$(kubectl logs -n "$NAMESPACE" "$CHECK_POD" 2>/dev/null || echo "MISSING")
    kubectl delete pod "$CHECK_POD" -n "$NAMESPACE" --ignore-not-found &>/dev/null || true
    [[ "$RESULT" == *"OK"* ]] && PBF_CHECK="OK"
  fi
fi

if [[ "$PBF_CHECK" != *"OK"* ]]; then
  err "west-africa.osm.pbf introuvable dans le PVC $OSM_PVC."
  err "Pour le générer, lance le Job osm-data-prep ou télécharge manuellement :"
  cat <<'HINT'

  # Option A : Job K8s (télécharge + OSRM prep, ~2-4h)
  kubectl apply -f infrastructure/manifests/osm-data-prep/job.yaml -n NAMESPACE
  kubectl wait --for=condition=complete job/osm-data-prep -n NAMESPACE --timeout=4h

  # Option B : Copie directe si tu as déjà le fichier sur le host
  # 1. Créer un pod de transit
  kubectl run osm-copy --image=busybox --restart=Never -n NAMESPACE \
    --overrides='{"spec":{"containers":[{"name":"c","image":"busybox","command":["sleep","3600"],
    "volumeMounts":[{"name":"pvc","mountPath":"/data"}]}],
    "volumes":[{"name":"pvc","persistentVolumeClaim":{"claimName":"osrm-data"}}]}}'
  # 2. Copier le fichier
  kubectl cp /chemin/local/west-africa.osm.pbf osm-copy:/data/west-africa.osm.pbf -n NAMESPACE
  # 3. Supprimer le pod transit
  kubectl delete pod osm-copy -n NAMESPACE

HINT
  exit 1
fi
ok "west-africa.osm.pbf présent dans le PVC"

# =============================================================================
# 2. Génération des tuiles (tile-prep Job → PVC tileserver-data)
# =============================================================================
if [[ "$SKIP_TILES" == false ]]; then
  log "Étape 2/6 — Génération des tuiles vectorielles (planetiler)"

  # PVC tileserver-data
  if ! kubectl get pvc tileserver-data -n "$NAMESPACE" &>/dev/null; then
    log "Création du PVC tileserver-data (5 Gi)..."
    $KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tileserver-data
  namespace: $NAMESPACE
  labels:
    app: tileserver-gl
    tier: tiles
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 5Gi
EOF
    ok "PVC tileserver-data créé"
  else
    ok "PVC tileserver-data déjà présent"
  fi

  # Vérifier si le .pmtiles existe déjà
  PMTILES_CHECK=$(kubectl run geo-init-tiles-check-$$ \
    --image=busybox \
    --restart=Never \
    --rm \
    -it \
    -n "$NAMESPACE" \
    --overrides='{
      "spec": {
        "containers": [{
          "name": "check",
          "image": "busybox",
          "command": ["sh", "-c", "ls -lh /tiles/west-africa.pmtiles 2>/dev/null && echo OK || echo MISSING"],
          "volumeMounts": [{"name": "tiles", "mountPath": "/tiles", "readOnly": true}]
        }],
        "volumes": [{"name": "tiles", "persistentVolumeClaim": {"claimName": "tileserver-data", "readOnly": true}}]
      }
    }' 2>/dev/null | tr -d '\r' | tail -1 || echo "MISSING")

  if [[ "$PMTILES_CHECK" == *"OK"* ]]; then
    warn "west-africa.pmtiles existe déjà dans tileserver-data — skip génération."
    warn "Pour forcer la régénération, supprime le fichier ou le PVC et relance."
  else
    log "Lancement du Job tile-prep (planetiler, ~30-60 min)..."

    # Supprimer un ancien Job éventuel
    kubectl delete job tile-prep -n "$NAMESPACE" --ignore-not-found 2>/dev/null | true

    $KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: tile-prep
  namespace: $NAMESPACE
  labels:
    app: tile-prep
    tier: tiles-prep
spec:
  ttlSecondsAfterFinished: 86400
  backoffLimit: 1
  template:
    metadata:
      labels:
        app: tile-prep
    spec:
      restartPolicy: Never
      containers:
        - name: planetiler
          image: ghcr.io/onthegomap/planetiler:latest
          imagePullPolicy: IfNotPresent
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -euo pipefail
              echo "Generating west-africa.pmtiles..."
              java -Xmx4g -jar /app/planetiler.jar \
                --osm-path=/osm/west-africa.osm.pbf \
                --output=/tiles/west-africa.pmtiles \
                --force \
                --languages=en,fr \
                --maxzoom=14
              echo "Done:"
              ls -lh /tiles/west-africa.pmtiles
          resources:
            requests:
              cpu: 1000m
              memory: 4Gi
            limits:
              cpu: 4000m
              memory: 6Gi
          volumeMounts:
            - name: osm-data
              mountPath: /osm
              readOnly: true
            - name: tile-data
              mountPath: /tiles
      volumes:
        - name: osm-data
          persistentVolumeClaim:
            claimName: $OSM_PVC
            readOnly: true
        - name: tile-data
          persistentVolumeClaim:
            claimName: tileserver-data
EOF

    if [[ "$DRY_RUN" == false ]]; then
      log "Job tile-prep lancé. Suivi des logs (Ctrl+C pour quitter sans arrêter le job) :"
      kubectl logs -f -n "$NAMESPACE" -l app=tile-prep --pod-running-timeout=5m || true
      log "Attente de la fin du Job..."
      kubectl wait --for=condition=complete job/tile-prep -n "$NAMESPACE" --timeout=2h
      ok "Génération des tuiles terminée"
    fi
  fi
fi

# =============================================================================
# 3. Déploiement TileServer GL
# =============================================================================
log "Étape 3/6 — Déploiement TileServer GL"

# ConfigMap config.json
$KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: tileserver-gl
  namespace: $NAMESPACE
  labels:
    app: tileserver-gl
    tier: tiles
data:
  config.json: |
    {
      "options": {
        "paths": {
          "root": "/data",
          "fonts": "/data/fonts",
          "sprites": "/data/sprites",
          "styles": "/data/styles",
          "mbtiles": "/data"
        },
        "serveAllStyles": true,
        "frontPage": false
      },
      "data": {
        "west-africa": {
          "pmtiles": "west-africa.pmtiles"
        }
      }
    }
EOF

# Deployment
$KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tileserver-gl
  namespace: $NAMESPACE
  labels:
    app: tileserver-gl
    tier: tiles
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: tileserver-gl
  template:
    metadata:
      labels:
        app: tileserver-gl
        tier: tiles
    spec:
      containers:
        - name: tileserver
          image: maptiler/tileserver-gl:v4.12
          imagePullPolicy: IfNotPresent
          args: ["--config", "/config/config.json", "--port", "8080"]
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 3
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
          volumeMounts:
            - name: config
              mountPath: /config
              readOnly: true
            - name: tile-data
              mountPath: /data
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: tileserver-gl
        - name: tile-data
          persistentVolumeClaim:
            claimName: tileserver-data
            readOnly: true
EOF

# Service ClusterIP (port 8082 sur le host pour éviter conflit avec api-gateway:8080)
$KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: tileserver-gl
  namespace: $NAMESPACE
  labels:
    app: tileserver-gl
    tier: tiles
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: http
      protocol: TCP
  selector:
    app: tileserver-gl
EOF

if [[ "$DRY_RUN" == false ]]; then
  log "Attente TileServer GL ready..."
  kubectl wait --for=condition=available deployment/tileserver-gl \
    -n "$NAMESPACE" --timeout=3m
  ok "TileServer GL prêt"
fi

# =============================================================================
# 4. Secret Nominatim (depuis secrets.yaml.example)
# =============================================================================
if [[ "$SKIP_NOMINATIM" == false ]]; then
  log "Étape 4/6 — Secret Nominatim"

  $KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: nominatim-backend
  namespace: $NAMESPACE
type: Opaque
stringData:
  NOMINATIM_PASSWORD: "$NOMINATIM_PASSWORD"
EOF
  ok "Secret nominatim-backend créé/mis à jour"

  # =============================================================================
  # 5. PVC Nominatim
  # =============================================================================
  log "Étape 5/6 — PVC Nominatim (15 Gi pour la base Postgres)"

  if ! kubectl get pvc nominatim-data -n "$NAMESPACE" &>/dev/null; then
    $KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nominatim-data
  namespace: $NAMESPACE
  labels:
    app: nominatim-backend
    tier: geocoding
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 15Gi
EOF
    ok "PVC nominatim-data créé"
  else
    ok "PVC nominatim-data déjà présent"
  fi

  # =============================================================================
  # 6. Déploiement Nominatim
  # =============================================================================
  log "Étape 6/6 — Déploiement Nominatim (import initial ~1-2h)"
  warn "Le pod ne sera pas Ready pendant ~1-2h le temps de l'import OSM dans Postgres."
  warn "C'est normal — le geolocation-service est en graceful degradation (ErrorGeocodingUnavailable) pendant ce temps."

  $KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nominatim-backend
  namespace: $NAMESPACE
  labels:
    app: nominatim-backend
    tier: geocoding
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: nominatim-backend
  template:
    metadata:
      labels:
        app: nominatim-backend
        tier: geocoding
    spec:
      containers:
        - name: nominatim
          image: mediagis/nominatim:4.4
          imagePullPolicy: IfNotPresent
          env:
            - name: PBF_PATH
              value: /osm/west-africa.osm.pbf
            - name: REPLICATION_URL
              value: ""
            - name: NOMINATIM_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: nominatim-backend
                  key: NOMINATIM_PASSWORD
            - name: POSTGRES_SHARED_BUFFERS
              value: "1GB"
            - name: POSTGRES_MAINTENANCE_WORK_MEM
              value: "256MB"
            - name: POSTGRES_AUTOVACUUM_WORK_MEM
              value: "256MB"
            - name: POSTGRES_WORK_MEM
              value: "32MB"
            - name: POSTGRES_EFFECTIVE_CACHE_SIZE
              value: "2GB"
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /status.php
              port: 8080
            initialDelaySeconds: 7200
            periodSeconds: 60
            timeoutSeconds: 10
            failureThreshold: 5
          readinessProbe:
            httpGet:
              path: /status.php
              port: 8080
            initialDelaySeconds: 60
            periodSeconds: 30
            timeoutSeconds: 10
            failureThreshold: 10
          resources:
            requests:
              cpu: 1000m
              memory: 2Gi
            limits:
              cpu: 2000m
              memory: 4Gi
          volumeMounts:
            - name: nominatim-data
              mountPath: /var/lib/postgresql
              subPath: postgres
            - name: osm-data
              mountPath: /osm
              readOnly: true
      volumes:
        - name: nominatim-data
          persistentVolumeClaim:
            claimName: nominatim-data
        - name: osm-data
          persistentVolumeClaim:
            claimName: $OSM_PVC
            readOnly: true
EOF

  $KUBECTL apply -f - -n "$NAMESPACE" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: nominatim-backend
  namespace: $NAMESPACE
  labels:
    app: nominatim-backend
    tier: geocoding
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 7070
      targetPort: http
      protocol: TCP
  selector:
    app: nominatim-backend
EOF

  ok "Nominatim déployé — import OSM en cours en background"
fi

# =============================================================================
# Résumé
# =============================================================================
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ok "Initialisation terminée"
echo ""
echo "📋 Commandes de suivi :"
echo ""
if [[ "$SKIP_NOMINATIM" == false ]]; then
  echo "  # Logs d'import Nominatim (attend 'Started API' pour savoir que c'est prêt)"
  echo "  kubectl logs -f -n $NAMESPACE deploy/nominatim-backend"
  echo ""
  echo "  # Tester Nominatim quand prêt"
  echo "  kubectl port-forward -n $NAMESPACE svc/nominatim-backend 7070:7070 &"
  echo "  curl 'http://localhost:7070/search?q=Lomé&format=json&limit=3'"
  echo ""
fi
if [[ "$SKIP_TILES" == false ]]; then
  echo "  # Tester TileServer GL"
  echo "  kubectl port-forward -n $NAMESPACE svc/tileserver-gl 8082:8080 &"
  echo "  curl -I http://localhost:8082/data/west-africa/13/4012/3851.pbf"
  echo ""
fi
echo "  # Statut des pods"
echo "  kubectl get pods -n $NAMESPACE -l 'tier in (geocoding,tiles)'"
echo ""
echo "  # Quand Nominatim est Ready, le endpoint geocoding sera disponible :"
echo "  curl -H 'Authorization: Bearer \$TOKEN' \\"
echo "    'https://api.tissimah.kpeewu.dev/api/v1/geolocation/geocode?Query=Lomé&Limit=3'"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
