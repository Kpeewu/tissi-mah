#!/bin/bash

################################################################################
# Script de renouvellement du certificat SSL - TissiMah VPS
# À exécuter tous les 90 jours (ou quand le certificat expire)
#
# Usage: ./renew-certificate.sh
################################################################################

set -e

# Couleurs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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
# Configuration
################################################################################

DOMAIN="api.tissimah.kpeewu.dev"
SECRET_NAME="tissi-mah-dev-tls"
NAMESPACE="default"

# Chemins des certificats (à adapter selon ton installation)
CERT_PATH="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
KEY_PATH="/etc/letsencrypt/live/$DOMAIN/privkey.pem"

################################################################################
# Étape 1 : Vérifier la date d'expiration actuelle
################################################################################

log_info "Vérification du certificat actuel..."

# Vérifier le certificat dans K8s
CURRENT_EXPIRY=$(kubectl get secret $SECRET_NAME -n $NAMESPACE -o jsonpath='{.data.tls\.crt}' 2>/dev/null | \
  base64 -d | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)

if [ -z "$CURRENT_EXPIRY" ]; then
    log_warn "Aucun certificat trouvé dans K8s. C'est peut-être la première installation."
else
    log_info "Certificat actuel expire le: $CURRENT_EXPIRY"
fi

echo ""
read -p "Voulez-vous continuer avec le renouvellement ? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Renouvellement annulé."
    exit 0
fi

################################################################################
# Étape 2 : Renouveler le certificat
################################################################################

log_info "Renouvellement du certificat avec certbot/python..."

# Si tu utilises certbot
if command -v certbot &> /dev/null; then
    log_info "certbot détecté. Renouvellement automatique..."
    certbot renew --cert-name $DOMAIN
    
    # Vérifier que le renouvellement a réussi
    if [ $? -ne 0 ]; then
        log_error "Échec du renouvellement avec certbot"
        exit 1
    fi
else
    log_warn "certbot non trouvé. Veuillez renouveler le certificat manuellement."
    echo ""
    read -p "Appuyez sur Entrée quand le certificat est renouvelé..."
fi

################################################################################
# Étape 3 : Vérifier les nouveaux fichiers
################################################################################

log_info "Vérification des fichiers de certificat..."

# Vérifier que les fichiers existent
if [ ! -f "$CERT_PATH" ]; then
    log_error "Certificat introuvable: $CERT_PATH"
    log_info "Veuillez spécifier le chemin correct:"
    read -p "Chemin du certificat (fullchain.pem): " CERT_PATH
fi

if [ ! -f "$KEY_PATH" ]; then
    log_error "Clé privée introuvable: $KEY_PATH"
    log_info "Veuillez spécifier le chemin correct:"
    read -p "Chemin de la clé privée (privkey.pem): " KEY_PATH
fi

# Vérifier la nouvelle date d'expiration
NEW_EXPIRY=$(openssl x509 -in "$CERT_PATH" -noout -enddate | cut -d= -f2)
log_info "Nouveau certificat expire le: $NEW_EXPIRY"

################################################################################
# Étape 4 : Supprimer l'ancien Secret K8s
################################################################################

log_info "Suppression de l'ancien Secret K8s..."

kubectl delete secret $SECRET_NAME -n $NAMESPACE 2>/dev/null || true

log_info "✓ Ancien Secret supprimé"

################################################################################
# Étape 5 : Créer le nouveau Secret K8s
################################################################################

log_info "Création du nouveau Secret K8s avec le certificat renouvelé..."

kubectl create secret tls $SECRET_NAME \
  --cert="$CERT_PATH" \
  --key="$KEY_PATH" \
  --namespace $NAMESPACE

if [ $? -ne 0 ]; then
    log_error "Échec de la création du Secret K8s"
    exit 1
fi

log_info "✓ Nouveau Secret créé"

################################################################################
# Étape 6 : Vérifier le Secret
################################################################################

log_info "Vérification du nouveau Secret..."

# Vérifier que le Secret existe
kubectl get secret $SECRET_NAME -n $NAMESPACE &> /dev/null

if [ $? -ne 0 ]; then
    log_error "Le Secret n'a pas été créé correctement"
    exit 1
fi

# Afficher les détails
kubectl describe secret $SECRET_NAME -n $NAMESPACE

################################################################################
# Étape 7 : Redémarrer Kong (optionnel)
################################################################################

log_info "Redémarrage de Kong pour appliquer le nouveau certificat..."

kubectl rollout restart deployment kong -n $NAMESPACE

# Attendre que Kong soit prêt
kubectl rollout status deployment kong -n $NAMESPACE --timeout=120s

log_info "✓ Kong redémarré"

################################################################################
# Étape 8 : Tester le nouveau certificat
################################################################################

log_info "Test du nouveau certificat HTTPS..."

# Attendre quelques secondes pour que Kong applique le certificat
sleep 5

# Test HTTPS
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" https://$DOMAIN/api/v1/auth/health)

if [ "$HTTP_CODE" == "200" ]; then
    log_info "✓ HTTPS fonctionne (code: $HTTP_CODE)"
else
    log_warn "⚠ HTTPS a retourné le code: $HTTP_CODE"
fi

# Vérifier le certificat SSL
CERT_VALIDITY=$(echo | openssl s_client -connect $DOMAIN:443 -servername $DOMAIN 2>/dev/null | \
  openssl x509 -noout -dates | grep notAfter | cut -d= -f2)

log_info "Certificat SSL actif expire le: $CERT_VALIDITY"

################################################################################
# Résumé
################################################################################

echo ""
log_info "=========================================="
log_info "  RENOUVELLEMENT TERMINÉ"
log_info "=========================================="
echo ""
log_info "Actions effectuées:"
log_info "  ✓ Certificat renouvelé"
log_info "  ✓ Secret K8s mis à jour"
log_info "  ✓ Kong redémarré"
log_info "  ✓ HTTPS testé"
echo ""
log_info "Nouveau certificat expire le: $NEW_EXPIRY"
log_info "Prochain renouvellement dans ~90 jours"
echo ""
log_info "Test manuel:"
log_info "  curl https://$DOMAIN/api/v1/auth/health"
echo ""
log_info "Vérifier dans un navigateur:"
log_info "  https://$DOMAIN/api/v1/auth/health"
echo ""

################################################################################
# Rappel pour le prochain renouvellement
################################################################################

# Calculer la date de renouvellement (70 jours = 10 080 minutes)
NEXT_RENEWAL=$(date -d "+70 days" +"%Y-%m-%d")

log_warn "=========================================="
log_warn "  RAPPEL IMPORTANT"
log_warn "=========================================="
log_warn "Renouveler le certificat AVANT le: $NEXT_RENEWAL"
log_warn "Commande: ./renew-certificate.sh"
log_warn ""
log_warn "Configurez un rappel dans votre calendrier !"
log_warn "=========================================="

exit 0