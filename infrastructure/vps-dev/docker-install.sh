#!/bin/bash

################################################################################
# Script d'installation Docker CE pour TissiMah VPS-dev
# OS: Ubuntu 24.04
# Mode: Root (daemon Docker en root)
# Version: Stable channel
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

# Versions
DOCKER_COMPOSE_VERSION="v2.24.5"

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

################################################################################
# VÉRIFICATIONS
################################################################################

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Ce script doit être exécuté en tant que root (sudo)"
        log_info "Utilisez: sudo bash $0"
        exit 1
    fi
    log_info "Exécution en tant que root confirmée"
}

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
    
    # Vérifier l'architecture (Docker nécessite x86_64 ou arm64)
    ARCH=$(uname -m)
    if [ "$ARCH" != "x86_64" ] && [ "$ARCH" != "aarch64" ]; then
        log_error "Architecture non supportée: $ARCH"
        log_error "Docker nécessite x86_64 ou arm64"
        exit 1
    fi
    
    log_info "✓ Architecture supportée: $ARCH"
}

################################################################################
# DÉSINSTALLATION DES ANCIENNES VERSIONS
################################################################################

remove_old_docker() {
    log_info "Vérification des anciennes versions de Docker..."
    
    # Liste des paquets Docker à désinstaller
    OLD_PACKAGES=(
        "docker"
        "docker-engine"
        "docker.io"
        "containerd"
        "runc"
        "docker-ce"
        "docker-ce-cli"
        "containerd.io"
        "docker-buildx-plugin"
        "docker-compose-plugin"
    )
    
    FOUND_OLD=false
    for package in "${OLD_PACKAGES[@]}"; do
        if dpkg -l | grep -q "^ii.*$package"; then
            FOUND_OLD=true
            log_warn "Package trouvé: $package"
        fi
    done
    
    if [ "$FOUND_OLD" = true ]; then
        log_warn "Anciennes versions de Docker détectées"
        read -p "Voulez-vous les désinstaller ? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Désinstallation des anciennes versions..."
            apt-get remove -y "${OLD_PACKAGES[@]}" 2>/dev/null || true
            apt-get autoremove -y
            log_info "✓ Anciennes versions désinstallées"
        else
            log_warn "Désinstallation annulée. Attention aux conflits possibles !"
        fi
    else
        log_info "✓ Aucune ancienne version détectée"
    fi
}

################################################################################
# INSTALLATION DES DÉPENDANCES
################################################################################

install_dependencies() {
    log_info "Installation des dépendances système..."
    
    # Mise à jour des paquets
    log_debug "Mise à jour de la liste des paquets..."
    apt-get update -qq
    
    # Installation des dépendances
    log_debug "Installation des paquets requis..."
    apt-get install -y -qq \
        ca-certificates \
        curl \
        gnupg \
        lsb-release \
        apt-transport-https \
        software-properties-common
    
    log_info "✓ Dépendances installées avec succès"
}

################################################################################
# AJOUT DU REPOSITORY DOCKER OFFICIEL
################################################################################

add_docker_repository() {
    log_info "Ajout du repository officiel Docker..."
    
    # Créer le répertoire pour les clés GPG
    install -m 0755 -d /etc/apt/keyrings
    
    # Télécharger et ajouter la clé GPG officielle de Docker
    log_debug "Téléchargement de la clé GPG Docker..."
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    
    # Ajouter le repository Docker aux sources APT
    log_debug "Ajout du repository Docker à APT sources..."
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
        $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
        tee /etc/apt/sources.list.d/docker.list > /dev/null
    
    # Mettre à jour la liste des paquets
    log_debug "Mise à jour des sources APT..."
    apt-get update -qq
    
    log_info "✓ Repository Docker ajouté avec succès"
}

################################################################################
# INSTALLATION DE DOCKER CE
################################################################################

install_docker() {
    log_info "Installation de Docker CE (Community Edition)..."
    
    log_debug "Installation des composants Docker:"
    log_debug "  - docker-ce: Docker Engine"
    log_debug "  - docker-ce-cli: Interface en ligne de commande"
    log_debug "  - containerd.io: Runtime pour conteneurs"
    log_debug "  - docker-buildx-plugin: Build multi-architecture"
    log_debug "  - docker-compose-plugin: Docker Compose V2"
    
    # Installation des paquets Docker
    apt-get install -y -qq \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin
    
    log_info "✓ Docker CE installé avec succès"
    
    # Vérifier la version installée
    DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
    log_info "Version installée: Docker $DOCKER_VERSION"
}

################################################################################
# CONFIGURATION POST-INSTALLATION
################################################################################

configure_docker() {
    log_info "Configuration de Docker..."
    
    # Activer et démarrer le service Docker
    log_debug "Activation du service Docker au démarrage..."
    systemctl enable docker
    systemctl start docker
    
    if systemctl is-active --quiet docker; then
        log_info "✓ Service Docker actif et configuré pour démarrer au boot"
    else
        log_error "Échec du démarrage du service Docker"
        systemctl status docker --no-pager
        exit 1
    fi
    
    # Ajouter l'utilisateur actuel au groupe docker (si pas root)
    ACTUAL_USER=${SUDO_USER:-$USER}
    if [ "$ACTUAL_USER" != "root" ]; then
        log_info "Ajout de l'utilisateur $ACTUAL_USER au groupe docker..."
        usermod -aG docker $ACTUAL_USER
        log_info "✓ Utilisateur ajouté au groupe docker"
        log_warn "Vous devrez vous déconnecter/reconnecter pour que les permissions prennent effet"
    fi
    
    # Configuration Docker daemon (optionnel mais recommandé)
    log_debug "Configuration du daemon Docker..."
    
    # Créer le fichier de configuration si inexistant
    if [ ! -f /etc/docker/daemon.json ]; then
        cat > /etc/docker/daemon.json <<EOF
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "storage-driver": "overlay2"
}
EOF
        log_info "✓ Configuration daemon.json créée"
        
        # Redémarrer Docker pour appliquer la config
        systemctl restart docker
        log_debug "Docker redémarré avec la nouvelle configuration"
    else
        log_debug "daemon.json existe déjà, configuration ignorée"
    fi
}

################################################################################
# VÉRIFICATION DE L'INSTALLATION
################################################################################

verify_installation() {
    log_info "Vérification de l'installation..."
    
    # Vérifier que Docker fonctionne
    log_debug "Test: docker version"
    docker version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        log_info "✓ Docker Engine fonctionnel"
    else
        log_error "Docker Engine ne répond pas"
        exit 1
    fi
    
    # Vérifier Docker Compose
    log_debug "Test: docker compose version"
    docker compose version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        COMPOSE_VERSION=$(docker compose version --short)
        log_info "✓ Docker Compose fonctionnel (version $COMPOSE_VERSION)"
    else
        log_error "Docker Compose ne fonctionne pas"
        exit 1
    fi
    
    # Test avec hello-world
    log_info "Test avec le conteneur hello-world..."
    docker run --rm hello-world > /tmp/docker-test.log 2>&1
    if [ $? -eq 0 ]; then
        log_info "✓ Conteneur de test exécuté avec succès"
        log_debug "Contenu du test:"
        grep "Hello from Docker!" /tmp/docker-test.log
        rm -f /tmp/docker-test.log
    else
        log_error "Échec du test hello-world"
        cat /tmp/docker-test.log
        exit 1
    fi
    
    # Afficher les informations Docker
    echo ""
    log_info "Informations système Docker:"
    docker info | grep -E "Server Version|Storage Driver|Logging Driver|Cgroup Driver|Operating System|Architecture"
    
    # Nettoyer l'image hello-world
    log_debug "Nettoyage de l'image de test..."
    docker rmi hello-world > /dev/null 2>&1 || true
}

################################################################################
# FONCTION PRINCIPALE
################################################################################

main() {
    echo ""
    log_info "=========================================="
    log_info "  Installation Docker CE - TissiMah VPS"
    log_info "=========================================="
    echo ""
    
    check_root
    check_prerequisites
    remove_old_docker
    install_dependencies
    add_docker_repository
    install_docker
    configure_docker
    verify_installation
    
    echo ""
    log_info "=========================================="
    log_info "  Installation terminée avec succès ! 🐳"
    log_info "=========================================="
    echo ""
    log_info "Commandes utiles:"
    log_info "  docker --version              # Voir la version"
    log_info "  docker ps                     # Lister les conteneurs actifs"
    log_info "  docker images                 # Lister les images"
    log_info "  docker compose up -d          # Démarrer avec docker-compose"
    log_info "  systemctl status docker       # Statut du service"
    echo ""
    
    ACTUAL_USER=${SUDO_USER:-$USER}
    if [ "$ACTUAL_USER" != "root" ]; then
        log_warn "IMPORTANT: Déconnectez-vous et reconnectez-vous pour que les permissions Docker prennent effet !"
        log_info "Ou exécutez: newgrp docker"
    fi
    
    echo ""
    log_info "Prochaine étape: Créer le docker-compose.yml pour PostgreSQL et Redis"
    echo ""
}

################################################################################
# EXÉCUTION
################################################################################

main "$@"