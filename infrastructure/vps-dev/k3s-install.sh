#!/bin/bash

################################################################################
# Script d'installation K3s pour TissiMah VPS-dev
# OS: Ubuntu 24.04
# Mode: Single-node
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
NC='\033[0m' # No Color

# Version K3s (fixée pour la reproductibilité)
K3S_VERSION="v1.28.5+k3s1"

# Chemin du kubeconfig
KUBECONFIG_PATH="$HOME/.kube/config"

################################################################################
# FONCTIONS UTILITAIRES
################################################################################

# Fonction pour logger en info
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

# Fonction pour logger en erreur
log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Fonction pour logger en warning
log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Fonction pour logger en debug
log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

################################################################################
# VÉRIFICATIONS
################################################################################

# Vérifier si on est root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Ce script doit être exécuté en tant que root (sudo)"
        log_info "Utilisez: sudo bash $0"
        exit 1
    fi
    log_info "Exécution en tant que root confirmée"
}

# Vérifier les prérequis système
check_prerequisites() {
    log_info "Vérification des prérequis système..."
    
    # Vérifier l'OS
    if [ ! -f /etc/os-release ]; then
        log_error "Impossible de détecter l'OS"
        exit 1
    fi
    
    . /etc/os-release
    if [ "$ID" != "ubuntu" ]; then
        log_error "Ce script est conçu pour Ubuntu. OS détecté: $ID"
        exit 1
    fi
    
    log_info "✓ OS détecté: Ubuntu $VERSION_ID"
    
    # Vérifier l'espace disque (minimum 10GB)
    AVAILABLE_SPACE=$(df / | tail -1 | awk '{print $4}')
    MIN_SPACE=10485760  # 10GB en KB
    
    if [ "$AVAILABLE_SPACE" -lt "$MIN_SPACE" ]; then
        log_error "Espace disque insuffisant. Minimum requis: 10GB"
        log_error "Espace disponible: $(($AVAILABLE_SPACE / 1024 / 1024))GB"
        exit 1
    fi
    
    log_info "✓ Espace disque disponible: $(($AVAILABLE_SPACE / 1024 / 1024))GB"
    
    # Vérifier la RAM (minimum 2GB recommandé)
    TOTAL_RAM=$(free -m | awk '/^Mem:/{print $2}')
    
    if [ "$TOTAL_RAM" -lt 2048 ]; then
        log_warn "RAM détectée: ${TOTAL_RAM}MB. Minimum recommandé: 2048MB"
        log_warn "K3s peut fonctionner mais les performances seront limitées"
    else
        log_info "✓ RAM disponible: ${TOTAL_RAM}MB"
    fi
    
    log_info "Prérequis système validés"
}

################################################################################
# INSTALLATION DES DÉPENDANCES
################################################################################

install_dependencies() {
    log_info "Installation des dépendances système..."
    
    # Mise à jour des paquets
    log_debug "Mise à jour de la liste des paquets..."
    apt-get update -qq
    
    # Installation des paquets nécessaires
    log_debug "Installation des paquets requis..."
    apt-get install -y -qq \
        curl \
        wget \
        apt-transport-https \
        ca-certificates \
        software-properties-common \
        gnupg \
        lsb-release
    
    log_info "✓ Dépendances installées avec succès"
}

################################################################################
# CONFIGURATION DU FIREWALL
################################################################################

configure_firewall() {
    log_info "Configuration du firewall..."
    
    if command -v ufw &> /dev/null; then
        if ufw status | grep -q "Status: active"; then
            log_info "UFW détecté et actif, configuration des règles..."
            
            # Port pour l'API Kubernetes (accès via SSH tunnel uniquement)
            ufw allow 6443/tcp comment 'Kubernetes API'
            
            # Ports pour les services web
            ufw allow 80/tcp comment 'HTTP'
            ufw allow 443/tcp comment 'HTTPS'
            
            # Ports pour la communication interne K3s
            ufw allow 10250/tcp comment 'Kubelet metrics'
            ufw allow 8472/udp comment 'Flannel VXLAN'
            
            log_info "✓ Règles firewall configurées"
        else
            log_info "UFW installé mais inactif, aucune configuration nécessaire"
        fi
    else
        log_info "UFW non installé, configuration firewall ignorée"
    fi
}

################################################################################
# INSTALLATION DE K3S
################################################################################

install_k3s() {
    log_info "Installation de K3s version $K3S_VERSION..."
    
    # Vérifier si K3s est déjà installé
    if command -v k3s &> /dev/null; then
        log_warn "K3s est déjà installé"
        CURRENT_VERSION=$(k3s --version | head -1 | awk '{print $3}')
        log_info "Version actuelle: $CURRENT_VERSION"
        
        read -p "Voulez-vous réinstaller K3s ? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Installation annulée"
            return
        fi
        
        log_info "Désinstallation de K3s existant..."
        /usr/local/bin/k3s-uninstall.sh || true
    fi
    
    # Options d'installation K3s
    export INSTALL_K3S_VERSION="$K3S_VERSION"
    export INSTALL_K3S_EXEC="server \
        --disable traefik \
        --disable servicelb \
        --write-kubeconfig-mode 644"
    
    log_debug "Options d'installation:"
    log_debug "  - Version: $K3S_VERSION"
    log_debug "  - Traefik désactivé (on utilise Kong)"
    log_debug "  - ServiceLB désactivé (Kong gère le routing)"
    log_debug "  - Kubeconfig mode 644 (accessible sans sudo)"
    
    # Téléchargement et installation
    log_info "Téléchargement et installation de K3s..."
    curl -sfL https://get.k3s.io | sh -
    
    if [ $? -eq 0 ]; then
        log_info "✓ K3s installé avec succès"
    else
        log_error "Échec de l'installation de K3s"
        exit 1
    fi
    
    # Attendre que K3s soit prêt
    log_info "Attente du démarrage de K3s (peut prendre jusqu'à 30s)..."
    sleep 10
    
    # Vérifier que K3s fonctionne
    log_debug "Statut du service K3s:"
    systemctl status k3s --no-pager | head -n 5
}

################################################################################
# CONFIGURATION DE KUBECTL
################################################################################

configure_kubectl() {
    log_info "Configuration de kubectl..."
    
    # Créer le répertoire .kube si nécessaire
    mkdir -p $HOME/.kube
    
    # Copier le kubeconfig
    if [ -f /etc/rancher/k3s/k3s.yaml ]; then
        cp /etc/rancher/k3s/k3s.yaml $KUBECONFIG_PATH
        
        # Changer le propriétaire (important si exécuté avec sudo)
        ACTUAL_USER=${SUDO_USER:-$USER}
        ACTUAL_HOME=$(eval echo ~$ACTUAL_USER)
        
        if [ "$ACTUAL_USER" != "root" ]; then
            mkdir -p $ACTUAL_HOME/.kube
            cp /etc/rancher/k3s/k3s.yaml $ACTUAL_HOME/.kube/config
            chown -R $ACTUAL_USER:$ACTUAL_USER $ACTUAL_HOME/.kube
            log_info "✓ Kubeconfig copié vers $ACTUAL_HOME/.kube/config"
        fi
        
        log_info "✓ Kubeconfig copié vers $KUBECONFIG_PATH"
    else
        log_error "Fichier kubeconfig K3s introuvable (/etc/rancher/k3s/k3s.yaml)"
        exit 1
    fi
    
    # Exporter KUBECONFIG dans le bashrc
    BASHRC_PATH="$HOME/.bashrc"
    ACTUAL_USER=${SUDO_USER:-$USER}
    if [ "$ACTUAL_USER" != "root" ]; then
        BASHRC_PATH=$(eval echo ~$ACTUAL_USER)/.bashrc
    fi
    
    if ! grep -q "KUBECONFIG" $BASHRC_PATH; then
        echo "" >> $BASHRC_PATH
        echo "# K3s kubeconfig" >> $BASHRC_PATH
        echo "export KUBECONFIG=\$HOME/.kube/config" >> $BASHRC_PATH
        log_info "✓ KUBECONFIG ajouté au .bashrc"
    else
        log_debug "KUBECONFIG déjà présent dans .bashrc"
    fi
    
    # Vérifier que kubectl fonctionne
    log_info "Vérification de kubectl..."
    sleep 5
    
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
    if kubectl version --short 2>/dev/null; then
        log_info "✓ kubectl fonctionnel"
    else
        log_warn "kubectl non accessible immédiatement, redémarrez votre session"
    fi
}

################################################################################
# VÉRIFICATION DE L'INSTALLATION
################################################################################

verify_installation() {
    log_info "Vérification de l'installation..."
    
    export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
    
    # Vérifier que le service K3s tourne
    if systemctl is-active --quiet k3s; then
        log_info "✓ Service K3s actif"
    else
        log_error "✗ Service K3s inactif"
        systemctl status k3s --no-pager
        exit 1
    fi
    
    # Attendre que le node soit prêt (max 60s)
    log_info "Attente de la disponibilité du node..."
    RETRY=0
    MAX_RETRY=12
    
    while [ $RETRY -lt $MAX_RETRY ]; do
        if kubectl get nodes 2>/dev/null | grep -q "Ready"; then
            break
        fi
        log_debug "Node pas encore prêt, nouvelle tentative dans 5s..."
        sleep 5
        RETRY=$((RETRY + 1))
    done
    
    if kubectl get nodes | grep -q "Ready"; then
        log_info "✓ Node Kubernetes prêt"
        echo ""
        kubectl get nodes
        echo ""
    else
        log_error "✗ Node Kubernetes non prêt après 60s"
        kubectl get nodes
        exit 1
    fi
    
    # Afficher les namespaces
    log_info "Namespaces disponibles:"
    kubectl get namespaces
    echo ""
    
    # Afficher les pods système
    log_info "Pods système (kube-system):"
    kubectl get pods -n kube-system
    echo ""
    
    # Informations sur le cluster
    log_info "Informations du cluster:"
    kubectl cluster-info
}

################################################################################
# FONCTION PRINCIPALE
################################################################################

main() {
    echo ""
    log_info "=========================================="
    log_info "  Installation K3s pour TissiMah VPS-dev"
    log_info "=========================================="
    echo ""
    
    check_root
    check_prerequisites
    install_dependencies
    configure_firewall
    install_k3s
    configure_kubectl
    verify_installation
    
    echo ""
    log_info "=========================================="
    log_info "  Installation terminée avec succès ! 🎉"
    log_info "=========================================="
    echo ""
    log_info "Prochaines étapes:"
    log_info "1. Rechargez votre session: source ~/.bashrc"
    log_info "2. Testez kubectl: kubectl get nodes"
    log_info "3. Installez PostgreSQL/Redis"
    log_info "4. Déployez auth-service"
    log_info "5. Installez Kong"
    echo ""
    log_info "Pour accéder au cluster depuis votre machine locale:"
    log_info "  scp root@votre-vps:/etc/rancher/k3s/k3s.yaml ~/.kube/config-vps"
    log_info "  # Puis éditez le fichier pour remplacer 127.0.0.1 par l'IP du VPS"
    echo ""
}

################################################################################
# EXÉCUTION
################################################################################

main "$@"