# Configuration HTTPS avec Certificat Manuel - TissiMah

Guide complet pour utiliser ton certificat SSL existant avec Kong (sans cert-manager).

---

## 📋 Vue d'ensemble

**Ton situation** :
- ✅ Certificat SSL obtenu avec Python/certbot
- ✅ Fichiers de certificat disponibles
- ✅ Renouvellement manuel tous les 90 jours

**Avantages** :
- ✅ Pas besoin d'installer cert-manager
- ✅ Configuration simple
- ✅ Contrôle total

**Inconvénient** :
- ❌ Renouvellement manuel nécessaire tous les ~90 jours

---

## 🚀 Installation (une seule fois)

### Étape 1 : Localiser tes fichiers de certificat

```bash
# Se connecter au VPS
ssh root@ton-vps

# Si certbot
ls -la /etc/letsencrypt/live/api.tissimah.kpeewu.dev/

# Ou recherche globale
find /etc -name "fullchain.pem" 2>/dev/null
find /etc -name "privkey.pem" 2>/dev/null
```

**Tu dois avoir 2 fichiers** :
- **fullchain.pem** (ou cert.pem + chain.pem)
- **privkey.pem** (clé privée)

**Exemple de chemins** :
```
/etc/letsencrypt/live/api.tissimah.kpeewu.dev/fullchain.pem
/etc/letsencrypt/live/api.tissimah.kpeewu.dev/privkey.pem
```

---

### Étape 2 : Créer le Secret K8s

```bash
# Remplacer les chemins par les tiens
kubectl create secret tls tissi-mah-dev-tls \
  --cert=/etc/letsencrypt/live/api.tissimah.kpeewu.dev/fullchain.pem \
  --key=/etc/letsencrypt/live/api.tissimah.kpeewu.dev/privkey.pem \
  --namespace default

# Vérifier
kubectl get secret tissi-mah-dev-tls -n default

# Voir les détails
kubectl describe secret tissi-mah-dev-tls -n default
```

**Résultat attendu** :
```
NAME                 TYPE                DATA   AGE
tissi-mah-dev-tls    kubernetes.io/tls   2      5s
```

---

### Étape 3 : Vérifier values-vps-dev.yaml

Ton fichier doit avoir :

```yaml
# Line ~116
tls:
  enabled: true
  secretName: tissi-mah-dev-tls  # ✅ Nom du Secret créé

# Line ~96 - CORRIGER SI NÉCESSAIRE
redis:
  port: 6380  # ✅ Redis Kong, pas 6379
```

**Si tu as `port: 6379`, corrige-le en `6380` !**

---

### Étape 4 : Appliquer les routes Kong

```bash
cd /root/tissimah/infrastructure/helm-charts/kong

# Appliquer les routes
helm template . --values values-vps-dev.yaml \
  --show-only routes/auth-routes.yaml | kubectl apply -f -

# Vérifier les Ingress
kubectl get ingress

# Devrait afficher:
# NAME                TYPE   ...
# auth-login          ...
# auth-service-tls    ...    (celui qui active HTTPS)
```

---

### Étape 5 : Tester HTTPS

#### Test 1 : HTTP → HTTPS redirect

```bash
curl -I http://api.tissimah.kpeewu.dev/api/v1/auth/health

# Devrait retourner:
# HTTP/1.1 301 Moved Permanently
# Location: https://api.tissimah.kpeewu.dev/api/v1/auth/health
```

#### Test 2 : HTTPS fonctionne

```bash
curl https://api.tissimah.kpeewu.dev/api/v1/auth/health

# Devrait retourner:
# {"status": "healthy", "service": "auth-service"}
```

#### Test 3 : Vérifier le certificat

```bash
openssl s_client -connect api.tissimah.kpeewu.dev:443 \
  -servername api.tissimah.kpeewu.dev < /dev/null 2>/dev/null | \
  openssl x509 -noout -text | grep -A2 "Validity"

# Devrait afficher:
# Validity
#   Not Before: Jan 24 12:00:00 2026 GMT
#   Not After : Apr 24 12:00:00 2026 GMT
```

#### Test 4 : Navigateur

Ouvre un navigateur et va sur :
```
https://api.tissimah.kpeewu.dev/api/v1/auth/health
```

Tu devrais voir :
- 🔒 Cadenas vert (certificat valide)
- Réponse JSON : `{"status": "healthy"}`

---

## 🔄 Renouvellement (tous les 90 jours)

### Méthode automatique avec script

```bash
# Sur le VPS
cd /root/tissimah/infrastructure/vps-dev

# Rendre le script exécutable
chmod +x renew-certificate.sh

# Exécuter (tous les 90 jours)
./renew-certificate.sh
```

**Le script fait** :
1. ✅ Renouvelle le certificat avec certbot
2. ✅ Supprime l'ancien Secret K8s
3. ✅ Crée un nouveau Secret avec le nouveau certificat
4. ✅ Redémarre Kong
5. ✅ Teste que HTTPS fonctionne

---

### Méthode manuelle

Si tu préfères faire à la main :

#### Étape 1 : Renouveler le certificat

```bash
# Si certbot
certbot renew --cert-name api.tissimah.kpeewu.dev

# Ou autre outil Python que tu utilises
python ton-outil-ssl.py renew
```

#### Étape 2 : Mettre à jour le Secret K8s

```bash
# Supprimer l'ancien Secret
kubectl delete secret tissi-mah-dev-tls -n default

# Créer le nouveau Secret
kubectl create secret tls tissi-mah-dev-tls \
  --cert=/etc/letsencrypt/live/api.tissimah.kpeewu.dev/fullchain.pem \
  --key=/etc/letsencrypt/live/api.tissimah.kpeewu.dev/privkey.pem \
  --namespace default
```

#### Étape 3 : Redémarrer Kong

```bash
# Kong appliquera automatiquement le nouveau certificat
kubectl rollout restart deployment kong -n default

# Attendre que Kong soit prêt
kubectl rollout status deployment kong -n default
```

#### Étape 4 : Tester

```bash
curl https://api.tissimah.kpeewu.dev/api/v1/auth/health
```

---

## 📅 Rappel de renouvellement

### Configurer un rappel

**Option A : Calendrier**
- Créer un événement récurrent tous les 80 jours
- Titre : "Renouveler certificat SSL TissiMah"
- Description : `ssh root@vps && cd /root/tissimah/infrastructure/vps-dev && ./renew-certificate.sh`

**Option B : Cron (automatique)**

```bash
# Ajouter dans crontab
crontab -e

# Renouveler tous les 80 jours à 3h du matin
0 3 */80 * * /root/tissimah/infrastructure/vps-dev/renew-certificate.sh >> /var/log/ssl-renewal.log 2>&1
```

**Option C : Email reminder**

Utiliser un service comme :
- Google Calendar avec notifications email
- Calendly
- IFTTT

---

## 🧪 Tests complets

### Vérifier la date d'expiration

```bash
# Depuis le VPS
kubectl get secret tissi-mah-dev-tls -n default -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -dates

# Devrait afficher:
# notBefore=Jan 24 12:00:00 2026 GMT
# notAfter=Apr 24 12:00:00 2026 GMT  (90 jours plus tard)
```

### Test du workflow complet

```bash
# 1. Health check (route publique)
curl https://api.tissimah.kpeewu.dev/api/v1/auth/health
# {"status": "healthy"}

# 2. Login sans JWT (doit échouer 401)
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phoneNumber": "+22500000000"}'
# {"message": "Unauthorized"}

# 3. Vérifier la redirection HTTP → HTTPS
curl -I http://api.tissimah.kpeewu.dev/api/v1/auth/health
# HTTP/1.1 301 Moved Permanently

# 4. Vérifier le certificat SSL
curl -vI https://api.tissimah.kpeewu.dev 2>&1 | grep -E "(subject|issuer|expire)"
```

---

## 🐛 Troubleshooting

### Certificat non trouvé

**Erreur** :
```bash
kubectl get secret tissi-mah-dev-tls -n default
Error from server (NotFound): secrets "tissi-mah-dev-tls" not found
```

**Solution** :
```bash
# Recréer le Secret
kubectl create secret tls tissi-mah-dev-tls \
  --cert=/chemin/vers/fullchain.pem \
  --key=/chemin/vers/privkey.pem \
  --namespace default
```

---

### HTTPS ne fonctionne pas

**Erreur** :
```bash
curl https://api.tissimah.kpeewu.dev
curl: (35) error:14094410:SSL routines:ssl3_read_bytes:sslv3 alert handshake failure
```

**Causes possibles** :
1. ❌ Secret K8s pas créé
2. ❌ Ingress TLS mal configuré
3. ❌ Kong ne voit pas le Secret

**Solutions** :
```bash
# 1. Vérifier le Secret
kubectl get secret tissi-mah-dev-tls -n default

# 2. Vérifier les Ingress
kubectl get ingress auth-service-tls -o yaml | grep -A5 "tls:"

# Doit avoir:
# tls:
#   - hosts:
#       - api.tissimah.kpeewu.dev
#     secretName: tissi-mah-dev-tls

# 3. Redémarrer Kong
kubectl rollout restart deployment kong -n default
```

---

### Certificat expiré

**Erreur** :
```
curl: (60) SSL certificate problem: certificate has expired
```

**Solution** :
```bash
# Vérifier la date d'expiration
kubectl get secret tissi-mah-dev-tls -n default -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -enddate

# Si expiré, renouveler immédiatement
./renew-certificate.sh
```

---

### Fichier de certificat introuvable

**Erreur** :
```bash
kubectl create secret tls ...
Error: stat /etc/letsencrypt/live/.../fullchain.pem: no such file or directory
```

**Solution** :
```bash
# Rechercher les certificats
find /etc -name "*fullchain*" -o -name "*cert*" 2>/dev/null
find ~ -name "*.pem" 2>/dev/null

# Ou lister les certificats certbot
certbot certificates

# Utiliser le bon chemin
kubectl create secret tls tissi-mah-dev-tls \
  --cert=/bon/chemin/vers/cert.pem \
  --key=/bon/chemin/vers/key.pem \
  --namespace default
```

---

## 📊 Comparaison : Manuel vs Automatique

| Aspect | Certificat Manuel | cert-manager |
|--------|-------------------|--------------|
| **Installation** | Simple ✅ | Complexe ⚠️ |
| **Configuration** | 5 minutes ✅ | 15 minutes ⚠️ |
| **Renouvellement** | Manuel ❌ | Automatique ✅ |
| **Maintenance** | Tous les 90j ⚠️ | Aucune ✅ |
| **Dépendances** | Aucune ✅ | cert-manager ⚠️ |
| **Contrôle** | Total ✅ | Délégué ⚠️ |

**Pour VPS-dev** : Certificat manuel est parfait ✅

**Pour production** : Envisager cert-manager pour éviter d'oublier

---

## ✅ Checklist

### Installation initiale
- [ ] Fichiers de certificat localisés
- [ ] Secret K8s créé
- [ ] values-vps-dev.yaml corrigé (port Redis 6380)
- [ ] Routes Kong appliquées
- [ ] HTTPS fonctionne
- [ ] HTTP redirige vers HTTPS
- [ ] Certificat valide dans navigateur

### Avant chaque renouvellement (tous les 90j)
- [ ] Sauvegarder le certificat actuel
- [ ] Renouveler avec certbot/outil Python
- [ ] Vérifier le nouveau certificat
- [ ] Mettre à jour le Secret K8s
- [ ] Redémarrer Kong
- [ ] Tester HTTPS
- [ ] Configurer rappel pour prochain renouvellement

---

## 📚 Commandes utiles

```bash
# Voir la date d'expiration du certificat
kubectl get secret tissi-mah-dev-tls -n default -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -enddate

# Voir tous les détails du certificat
kubectl get secret tissi-mah-dev-tls -n default -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -text

# Tester HTTPS
curl -vI https://api.tissimah.kpeewu.dev 2>&1 | grep -E "(SSL|certificate)"

# Voir les logs Kong
kubectl logs -l app.kubernetes.io/name=kong --tail=50

# Redémarrer Kong
kubectl rollout restart deployment kong -n default
```

---

## 🎯 Résumé

**Configuration HTTPS manuelle** :
1. ✅ Créer Secret K8s avec certificat existant
2. ✅ Configurer Kong pour utiliser ce Secret
3. ✅ Appliquer les routes avec TLS activé
4. ✅ Tester HTTPS

**Renouvellement (tous les 90 jours)** :
1. ✅ Renouveler le certificat
2. ✅ Mettre à jour le Secret K8s
3. ✅ Redémarrer Kong
4. ✅ Tester

**Résultat final** :
```
http://api.tissimah.kpeewu.dev → 301 → https://api.tissimah.kpeewu.dev
HTTPS avec certificat SSL valide ✅
```

---

**Configuration réalisée le** : _______________

**Certificat expire le** : _______________

**Prochain renouvellement** : _______________ (80 jours après)

**Rappel configuré** : ☐ Oui  ☐ Non