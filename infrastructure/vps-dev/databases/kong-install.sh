#!/bin/bash

################################################################################
# Script d'installation Kong API Gateway pour TissiMah VPS-dev
# OS: Ubuntu 24.04
# Mode: DB-less (sans PostgreSQL)
# Installation: Helm dans K3s
# Auteur: Toure-Ydaou
################################################################################

set -e  # Arrête le script si une commande échoue
set -u  # Arrête si on utilise une variable non définie

################################################################################
# VARIABLES DE CONFIGURATION
################################################################################

# Couleurs pour les logs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Chemins
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DATABASES_DIR="$SCRIPT_DIR"
KONG_HELM_DIR="$PROJECT_ROOT/infrastructure/helm-charts/kong"

# Configuration Kong
KONG_NAMESPACE="default"
KONG_RELEASE_NAME="kong"
KONG_CHART_VERSION="2.38.0"

################################################################################
# FONCTIONS UTILITAIRES
################################################################################

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

################################################################################
# VÉRIFICATIONS PRÉALABLES
################################################################################

check_prerequisites() {
    log_step "Vérification des prérequis..."
    echo ""
    
    local MISSING_TOOLS=0
    
    # Vérifier Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker n'est pas installé"
        log_info "Installez Docker avec: ./docker-install.sh"
        MISSING_TOOLS=1
    else
        DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
        log_info "✓ Docker installé: $DOCKER_VERSION"
    fi
    
    # Vérifier Docker Compose
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose n'est pas installé"
        MISSING_TOOLS=1
    else
        log_info "✓ Docker Compose installé"
    fi
    
    # Vérifier K3s/kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl n'est pas installé (K3s requis)"
        log_info "Installez K3s avec: ./k3s-install.sh"
        MISSING_TOOLS=1
    else
        log_info "✓ kubectl installé"
    fi
    
    # Vérifier que K3s fonctionne
    if ! kubectl get nodes &> /dev/null; then
        log_error "K3s ne semble pas fonctionner"
        log_info "Vérifiez avec: kubectl get nodes"
        MISSING_TOOLS=1
    else
        log_info "✓ K3s fonctionnel"
    fi
    
    # Vérifier Helm
    if ! command -v helm &> /dev/null; then
        log_error "Helm n'est pas installé"
        log_info "Installation de Helm..."
        curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    else
        HELM_VERSION=$(helm version --short | awk '{print $1}')
        log_info "✓ Helm installé: $HELM_VERSION"
    fi
    
    echo ""
    
    if [ $MISSING_TOOLS -eq 1 ]; then
        log_error "Des outils requis sont manquants. Installez-les avant de continuer."
        exit 1
    fi
    
    log_info "Tous les prérequis sont satisfaits !"
}

################################################################################
# CHARGER LES VARIABLES D'ENVIRONNEMENT
################################################################################

load_environment() {
    log_step "Chargement des variables d'environnement..."
    echo ""
    
    ENV_FILE="$DATABASES_DIR/.env"
    
    if [ ! -f "$ENV_FILE" ]; then
        log_error "Fichier .env introuvable: $ENV_FILE"
        log_info "Créez le fichier .env depuis .env.example:"
        log_info "  cd $DATABASES_DIR"
        log_info "  cp .env.example .env"
        log_info "  nano .env  # Configurez les mots de passe"
        exit 1
    fi
    
    # Charger les variables
    set -a
    source "$ENV_FILE"
    set +a
    
    # Vérifier les variables requises
    if [ -z "${KONG_REDIS_PASSWORD:-}" ]; then
        log_error "Variable KONG_REDIS_PASSWORD manquante dans .env"
        exit 1
    fi
    
    if [ -z "${KONG_REDIS_PORT:-}" ]; then
        log_error "Variable KONG_REDIS_PORT manquante dans .env"
        exit 1
    fi
    
    log_info "✓ Variables d'environnement chargées"
    log_debug "KONG_REDIS_PORT: $KONG_REDIS_PORT"
    echo ""
}

################################################################################
# DÉMARRER REDIS KONG
################################################################################

start_redis_kong() {
    log_step "Démarrage de Redis Kong..."
    echo ""
    
    REDIS_COMPOSE="$DATABASES_DIR/docker-compose-kong-redis.yml"
    
    if [ ! -f "$REDIS_COMPOSE" ]; then
        log_error "Fichier docker-compose-kong-redis.yml introuvable: $REDIS_COMPOSE"
        exit 1
    fi
    
    log_info "Vérification de l'état de Redis Kong..."
    
    if docker ps | grep -q tissimah-redis-kong; then
        log_warn "Redis Kong est déjà démarré"
        read -p "Voulez-vous le redémarrer ? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Redémarrage de Redis Kong..."
            docker compose -f "$REDIS_COMPOSE" restart
        fi
    else
        log_info "Démarrage de Redis Kong..."
        docker compose -f "$REDIS_COMPOSE" up -d
    fi
    
    # Attendre que Redis soit prêt
    log_info "Attente de la disponibilité de Redis Kong..."
    RETRY=0
    MAX_RETRY=30
    
    while [ $RETRY -lt $MAX_RETRY ]; do
        if docker exec tissimah-redis-kong redis-cli -p "$KONG_REDIS_PORT" -a "$KONG_REDIS_PASSWORD" ping &> /dev/null; then
            log_info "✓ Redis Kong est prêt !"
            break
        fi
        log_debug "Redis Kong pas encore prêt, nouvelle tentative dans 2s..."
        sleep 2
        RETRY=$((RETRY + 1))
    done
    
    if [ $RETRY -eq $MAX_RETRY ]; then
        log_error "Redis Kong n'a pas démarré après 60s"
        docker compose -f "$REDIS_COMPOSE" logs redis-kong
        exit 1
    fi
    
    echo ""
}

################################################################################
# GÉNÉRER ET APPLIQUER LES SECRETS K8S
################################################################################

apply_secrets() {
    log_step "Création des Secrets Kubernetes..."
    echo ""
    
    TEMPLATE_FILE="$DATABASES_DIR/kong-secrets.yaml.template"
    SECRETS_FILE="$DATABASES_DIR/kong-secrets.yaml"
    
    if [ ! -f "$TEMPLATE_FILE" ]; then
        log_error "Template introuvable: $TEMPLATE_FILE"
        exit 1
    fi
    
    log_info "Génération de kong-secrets.yaml depuis le template..."
    
    # Copier le template et remplacer les placeholders
    cp "$TEMPLATE_FILE" "$SECRETS_FILE"
    
    # Remplacer les variables
    sed -i "s|__KONG_REDIS_PASSWORD__|${KONG_REDIS_PASSWORD}|g" "$SECRETS_FILE"
    sed -i "s|__KONG_REDIS_PORT__|${KONG_REDIS_PORT}|g" "$SECRETS_FILE"
    
    log_info "✓ Fichier kong-secrets.yaml généré"
    
    # Appliquer les secrets
    log_info "Application des secrets dans Kubernetes..."
    kubectl apply -f "$SECRETS_FILE"
    
    # Vérifier que les secrets sont créés
    if kubectl get secret kong-redis-secret -n "$KONG_NAMESPACE" &> /dev/null; then
        log_info "✓ Secret kong-redis-secret créé"
    else
        log_error "Échec de la création du secret kong-redis-secret"
        exit 1
    fi
    
    # Supprimer le fichier temporaire (contient des mots de passe)
    log_warn "Suppression de kong-secrets.yaml (contient des secrets)"
    rm -f "$SECRETS_FILE"
    
    echo ""
}

################################################################################
# INSTALLER KONG AVEC HELM
################################################################################

install_kong() {
    log_step "Installation de Kong avec Helm..."
    echo ""
    
    # Ajouter le repo Helm Kong
    log_info "Ajout du repository Helm Kong..."
    helm repo add kong https://charts.konghq.com
    helm repo update
    
    log_info "✓ Repository Kong ajouté"
    
    # Vérifier si Kong est déjà installé
    if helm list -n "$KONG_NAMESPACE" | grep -q "$KONG_RELEASE_NAME"; then
        log_warn "Kong est déjà installé"
        read -p "Voulez-vous le mettre à jour ? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Mise à jour de Kong..."
            helm upgrade "$KONG_RELEASE_NAME" kong/kong \
                --namespace "$KONG_NAMESPACE" \
                --version "$KONG_CHART_VERSION" \
                --values "$KONG_HELM_DIR/values-vps-dev.yaml" \
                --wait \
                --timeout 5m
            log_info "✓ Kong mis à jour"
        fi
    else
        log_info "Installation de Kong version $KONG_CHART_VERSION..."
        
        # Vérifier que le fichier values existe
        if [ ! -f "$KONG_HELM_DIR/values-vps-dev.yaml" ]; then
            log_error "Fichier values-vps-dev.yaml introuvable: $KONG_HELM_DIR/values-vps-dev.yaml"
            exit 1
        fi
        
        helm install "$KONG_RELEASE_NAME" kong/kong \
            --namespace "$KONG_NAMESPACE" \
            --version "$KONG_CHART_VERSION" \
            --values "$KONG_HELM_DIR/values-vps-dev.yaml" \
            --wait \
            --timeout 5m
        
        log_info "✓ Kong installé avec succès"
    fi
    
    echo ""
}

################################################################################
# ATTENDRE QUE KONG SOIT PRÊT
################################################################################

wait_kong_ready() {
    log_step "Attente de la disponibilité de Kong..."
    echo ""
    
    log_info "Vérification des pods Kong..."
    
    RETRY=0
    MAX_RETRY=60
    
    while [ $RETRY -lt $MAX_RETRY ]; do
        # Vérifier que tous les pods Kong sont prêts
        READY_PODS=$(kubectl get pods -n "$KONG_NAMESPACE" -l app.kubernetes.io/name=kong -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -c "True" || echo "0")
        TOTAL_PODS=$(kubectl get pods -n "$KONG_NAMESPACE" -l app.kubernetes.io/name=kong --no-headers 2>/dev/null | wc -l)
        
        if [ "$READY_PODS" -gt 0 ] && [ "$READY_PODS" -eq "$TOTAL_PODS" ]; then
            log_info "✓ Tous les pods Kong sont prêts ($READY_PODS/$TOTAL_PODS)"
            break
        fi
        
        log_debug "Pods Kong prêts: $READY_PODS/$TOTAL_PODS, nouvelle tentative dans 5s..."
        sleep 5
        RETRY=$((RETRY + 1))
    done
    
    if [ $RETRY -eq $MAX_RETRY ]; then
        log_error "Kong n'est pas prêt après 5 minutes"
        kubectl get pods -n "$KONG_NAMESPACE" -l app.kubernetes.io/name=kong
        kubectl logs -n "$KONG_NAMESPACE" -l app.kubernetes.io/name=kong --tail=50
        exit 1
    fi
    
    echo ""
}

################################################################################
# APPLIQUER LES PLUGINS KONG
################################################################################

apply_plugins() {
    log_step "Application des plugins Kong (CRDs)..."
    echo ""
    
    PLUGINS_DIR="$KONG_HELM_DIR/plugins"
    
    if [ ! -d "$PLUGINS_DIR" ]; then
        log_error "Dossier plugins introuvable: $PLUGINS_DIR"
        exit 1
    fi
    
    log_info "Application des plugins depuis $PLUGINS_DIR..."
    
    # Appliquer chaque fichier de plugin
    for plugin_file in "$PLUGINS_DIR"/*.yaml; do
        if [ -f "$plugin_file" ]; then
            plugin_name=$(basename "$plugin_file")
            log_info "  - Applying $plugin_name..."
            
            # Utiliser helm template pour rendre les templates
            helm template kong-plugins "$KONG_HELM_DIR" \
                --show-only "plugins/$plugin_name" \
                --values "$KONG_HELM_DIR/values-vps-dev.yaml" \
                | kubectl apply -f -
        fi
    done
    
    log_info "✓ Plugins appliqués"
    echo ""
}

################################################################################
# APPLIQUER LES ROUTES KONG
################################################################################

apply_routes() {
    log_step "Application des routes Kong..."
    echo ""
    
    ROUTES_DIR="$KONG_HELM_DIR/routes"
    
    if [ ! -d "$ROUTES_DIR" ]; then
        log_error "Dossier routes introuvable: $ROUTES_DIR"
        exit 1
    fi
    
    log_info "Application des routes depuis $ROUTES_DIR..."
    
    # Appliquer chaque fichier de routes
    for route_file in "$ROUTES_DIR"/*.yaml; do
        if [ -f "$route_file" ]; then
            route_name=$(basename "$route_file")
            log_info "  - Applying $route_name..."
            
            # Utiliser helm template pour rendre les templates
            helm template kong-routes "$KONG_HELM_DIR" \
                --show-only "routes/$route_name" \
                --values "$KONG_HELM_DIR/values-vps-dev.yaml" \
                | kubectl apply -f -
        fi
    done
    
    log_info "✓ Routes appliquées"
    echo ""
}

################################################################################
# VÉRIFICATION DE L'INSTALLATION
################################################################################

verify_installation() {
    log_step "Vérification de l'installation Kong..."
    echo ""
    
    # Vérifier les pods Kong
    log_info "Pods Kong:"
    kubectl get pods -n "$KONG_NAMESPACE" -l app.kubernetes.io/name=kong
    echo ""
    
    # Vérifier les services Kong
    log_info "Services Kong:"
    kubectl get svc -n "$KONG_NAMESPACE" -l app.kubernetes.io/name=kong
    echo ""
    
    # Vérifier les plugins Kong
    log_info "Plugins Kong (KongPlugin CRDs):"
    kubectl get kongplugins -n "$KONG_NAMESPACE" 2>/dev/null || log_warn "Aucun plugin trouvé"
    echo ""
    
    # Vérifier les routes Kong (Ingress)
    log_info "Routes Kong (Ingress):"
    kubectl get ingress -n "$KONG_NAMESPACE" 2>/dev/null || log_warn "Aucune route trouvée"
    echo ""
    
    # Tester Kong Admin API (si activé)
    log_info "Test Kong Admin API (port-forward)..."
    log_warn "Pour tester Kong Admin API, exécutez:"
    log_info "  kubectl port-forward -n $KONG_NAMESPACE svc/kong-admin 8001:8001"
    log_info "  curl http://localhost:8001/"
    echo ""
    
    # Tester Kong Proxy
    KONG_PROXY_PORT=$(kubectl get svc -n "$KONG_NAMESPACE" kong-proxy -o jsonpath='{.spec.ports[?(@.name=="proxy")].nodePort}' 2>/dev/null || echo "")
    
    if [ -n "$KONG_PROXY_PORT" ]; then
        log_info "Kong Proxy accessible sur le port NodePort: $KONG_PROXY_PORT"
        log_info "Test: curl http://localhost:$KONG_PROXY_PORT/api/v1/auth/health"
    else
        log_warn "Kong Proxy port non trouvé (service type LoadBalancer ?)"
    fi
    
    echo ""
}

################################################################################
# FONCTION PRINCIPALE
################################################################################

main() {
    echo ""
    log_info "==========================================="
    log_info "  Installation Kong API Gateway"
    log_info "  TissiMah VPS-dev"
    log_info "==========================================="
    echo ""
    
    check_prerequisites
    load_environment
    start_redis_kong
    apply_secrets
    install_kong
    wait_kong_ready
    # Note: Les plugins et routes seront appliqués manuellement après
    # car ils nécessitent que auth-service soit déployé
    
    echo ""
    log_info "==========================================="
    log_info "  Installation Kong terminée ! 🎉"
    log_info "==========================================="
    echo ""
    
    log_info "Prochaines étapes:"
    log_info "1. Déployer auth-service dans K3s"
    log_info "2. Appliquer les plugins Kong:"
    log_info "   cd $KONG_HELM_DIR"
    log_info "   helm template . --values values-vps-dev.yaml --show-only plugins/cors.yaml | kubectl apply -f -"
    log_info "3. Appliquer les routes Kong:"
    log_info "   helm template . --values values-vps-dev.yaml --show-only routes/auth-routes.yaml | kubectl apply -f -"
    log_info "4. Tester Kong:"
    log_info "   curl http://localhost/api/v1/auth/health"
    echo ""
    
    verify_installation
}

################################################################################
# EXÉCUTION
################################################################################

main "$@"