#!/bin/bash

################################################################################
# Script pour générer VPS_KUBECONFIG pour GitHub Actions
# TissiMah - Infrastructure
#
# Ce script génère le secret VPS_KUBECONFIG encodé en base64
# à ajouter dans GitHub → Settings → Secrets → Actions
################################################################################

set -e

# Couleurs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

################################################################################
# ÉTAPE 1 : Vérifications
################################################################################

log_info "Génération du secret VPS_KUBECONFIG pour GitHub Actions"
echo ""

# Vérifier que K3s est installé
if ! command -v kubectl &> /dev/null; then
    log_error "kubectl n'est pas installé. K3s doit être installé d'abord."
    exit 1
fi

# Vérifier que K3s fonctionne
if ! kubectl get nodes &> /dev/null; then
    log_error "Impossible de se connecter à K3s. Vérifiez que K3s fonctionne."
    exit 1
fi

log_info "✓ K3s est installé et fonctionne"
echo ""

################################################################################
# ÉTAPE 2 : Récupérer l'IP publique du VPS
################################################################################

log_info "Détection de l'IP publique du VPS..."

# Méthode 1 : curl
PUBLIC_IP=$(curl -s ifconfig.me || true)

# Méthode 2 : ip command (backup)
if [ -z "$PUBLIC_IP" ]; then
    PUBLIC_IP=$(ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -n1)
fi

if [ -z "$PUBLIC_IP" ]; then
    log_error "Impossible de détecter l'IP publique automatiquement"
    read -p "Entrez l'IP publique de votre VPS: " PUBLIC_IP
fi

log_info "IP publique détectée: $PUBLIC_IP"
echo ""

# Confirmation
read -p "Est-ce la bonne IP publique ? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    read -p "Entrez l'IP publique correcte: " PUBLIC_IP
fi

################################################################################
# ÉTAPE 3 : Générer le kubeconfig modifié
################################################################################

log_info "Génération du kubeconfig pour GitHub Actions..."

TEMP_KUBECONFIG=$(mktemp)

# Récupérer le kubeconfig brut
kubectl config view --raw > "$TEMP_KUBECONFIG"

# Remplacer 127.0.0.1 par l'IP publique
sed -i "s|server: https://127.0.0.1:6443|server: https://$PUBLIC_IP:6443|g" "$TEMP_KUBECONFIG"

log_info "✓ Kubeconfig généré avec l'IP publique"
echo ""

################################################################################
# ÉTAPE 4 : Vérifier le contenu
################################################################################

log_info "Vérification du kubeconfig généré..."
echo ""

log_info "Aperçu du kubeconfig:"
echo "===================="
head -n 10 "$TEMP_KUBECONFIG"
echo "..."
echo "===================="
echo ""

# Vérifier que l'IP est bien présente
if ! grep -q "server: https://$PUBLIC_IP:6443" "$TEMP_KUBECONFIG"; then
    log_error "L'IP publique n'a pas été correctement insérée dans le kubeconfig"
    exit 1
fi

log_info "✓ IP publique correctement configurée: https://$PUBLIC_IP:6443"
echo ""

################################################################################
# ÉTAPE 5 : Encoder en base64
################################################################################

log_info "Encodage du kubeconfig en base64..."

ENCODED_KUBECONFIG=$(cat "$TEMP_KUBECONFIG" | base64 -w 0)

log_info "✓ Kubeconfig encodé en base64"
echo ""

################################################################################
# ÉTAPE 6 : Afficher les instructions
################################################################################

log_info "=========================================="
log_info "  SECRET VPS_KUBECONFIG GÉNÉRÉ"
log_info "=========================================="
echo ""

log_info "ÉTAPES POUR AJOUTER LE SECRET DANS GITHUB:"
echo ""

log_info "1. Aller sur GitHub:"
log_info "   https://github.com/VOTRE_USERNAME/VOTRE_REPO/settings/secrets/actions"
echo ""

log_info "2. Cliquer sur 'New repository secret'"
echo ""

log_info "3. Remplir:"
log_info "   Name: VPS_KUBECONFIG"
log_info "   Value: [coller la valeur ci-dessous]"
echo ""

log_info "4. Cliquer sur 'Add secret'"
echo ""

log_info "=========================================="
log_info "  VALEUR À COPIER (SECRET):"
log_info "=========================================="
echo ""
echo "$ENCODED_KUBECONFIG"
echo ""
log_info "=========================================="
echo ""

log_warn "⚠️  IMPORTANT: Cette valeur contient des credentials sensibles !"
log_warn "⚠️  Ne la partagez jamais publiquement"
log_warn "⚠️  Ne la commitez jamais dans Git"
echo ""

################################################################################
# ÉTAPE 7 : Sauvegarder dans un fichier (optionnel)
################################################################################

read -p "Voulez-vous sauvegarder cette valeur dans un fichier ? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    OUTPUT_FILE="vps-kubeconfig-secret.txt"
    echo "$ENCODED_KUBECONFIG" > "$OUTPUT_FILE"
    log_info "✓ Secret sauvegardé dans: $OUTPUT_FILE"
    log_warn "⚠️  Supprimez ce fichier après l'avoir ajouté dans GitHub !"
fi

# Nettoyer
rm -f "$TEMP_KUBECONFIG"

echo ""
log_info "=========================================="
log_info "  VÉRIFICATIONS IMPORTANTES"
log_info "=========================================="
echo ""

log_info "Avant de tester le workflow, vérifiez:"
echo ""

log_info "1. Firewall: Le port 6443 doit être ouvert"
log_info "   Commande: ufw allow 6443/tcp"
echo ""

log_info "2. Certificats K3s: Doivent inclure l'IP publique"
log_info "   Si erreur x509, regénérer avec:"
log_info "   - Arrêter K3s: systemctl stop k3s"
log_info "   - Ajouter dans /etc/rancher/k3s/config.yaml:"
log_info "     tls-san:"
log_info "       - $PUBLIC_IP"
log_info "   - Supprimer les certs: rm -rf /var/lib/rancher/k3s/server/tls"
log_info "   - Redémarrer: systemctl start k3s"
echo ""

log_info "3. Tester la connexion depuis un autre ordinateur:"
log_info "   kubectl --kubeconfig=/tmp/kubeconfig-test.yaml get nodes"
echo ""

log_info "=========================================="
log_info "  PROCHAINES ÉTAPES"
log_info "=========================================="
echo ""

log_info "1. Copier la valeur base64 ci-dessus"
log_info "2. Créer le secret VPS_KUBECONFIG dans GitHub"
log_info "3. Tester avec un workflow simple (test-kubeconfig.yml)"
log_info "4. Si le test fonctionne, déployer avec deploy-to-vps.yml"
echo ""

log_info "Terminé ! 🎉"