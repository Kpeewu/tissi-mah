# VPS Development Environment - TissiMah

Documentation complète pour la configuration et la gestion de l'environnement de développement VPS.

---

## 📋 Table des matières

- [Vue d'ensemble](#vue-densemble)
- [Architecture](#architecture)
- [Prérequis](#prérequis)
- [Installation](#installation)
- [Configuration](#configuration)
- [Maintenance](#maintenance)
- [CI/CD](#cicd)
- [Troubleshooting](#troubleshooting)
- [Sécurité](#sécurité)

---

## 🎯 Vue d'ensemble

Le VPS de développement héberge l'infrastructure complète de TissiMah pour l'environnement de développement :

- **Cluster Kubernetes** : K3s (lightweight Kubernetes)
- **API Gateway** : Kong Gateway (mode DB-less)
- **Bases de données** : PostgreSQL, Redis, MongoDB (Docker containers)
- **Services backend** : Microservices Go avec gRPC
- **CI/CD** : GitHub Actions pour déploiement automatique

### Spécifications VPS

| Composant | Valeur |
|-----------|--------|
| **Provider** | OVH / Hetzner |
| **OS** | Ubuntu 24.04 LTS |
| **RAM** | 12 GB |
| **Storage** | 100 GB SSD |
| **CPU** | 4 vCPUs |
| **Réseau** | 1 Gbps |

---

## 🏗️ Architecture

### Architecture globale

```
┌─────────────────────────────────────────────────────────────┐
│                    VPS Ubuntu 24.04                         │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Docker (network_mode: host)             │  │
│  │                                                      │  │
│  │  ┌─────────────────┐  ┌─────────────────┐          │  │
│  │  │ PostgreSQL auth │  │ Redis auth      │          │  │
│  │  │ Port: 5433      │  │ Port: 6380      │          │  │
│  │  └─────────────────┘  └─────────────────┘          │  │
│  │                                                      │  │
│  │  ┌─────────────────┐  ┌─────────────────┐          │  │
│  │  │ Redis Kong      │  │ MongoDB user    │          │  │
│  │  │ Port: 6379      │  │ Port: 27017     │          │  │
│  │  └─────────────────┘  └─────────────────┘          │  │
│  │                                                      │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                K3s Cluster                           │  │
│  │              Namespace: default                      │  │
│  │                                                      │  │
│  │  ┌─────────────────┐  ┌─────────────────┐          │  │
│  │  │ Kong Gateway    │  │ auth-service    │          │  │
│  │  │ Port: 80/443    │  │ gRPC: 50051     │          │  │
│  │  └─────────────────┘  └─────────────────┘          │  │
│  │                                                      │  │
│  │  ┌─────────────────┐  ┌─────────────────┐          │  │
│  │  │ user-service    │  │ trips-service   │          │  │
│  │  │ gRPC: 50052     │  │ gRPC: 50053     │          │  │
│  │  └─────────────────┘  └─────────────────┘          │  │
│  │                                                      │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Flux de requête

```
Client (Mobile/Web)
    │
    │ HTTPS
    ↓
Kong Gateway (K3s)
    │
    ├─ JWT Validation (basique)
    ├─ Rate Limiting (Redis Kong)
    ├─ CORS
    ├─ HTTP → gRPC Translation
    │
    ↓
auth-service (K3s)
    │
    ├─ Firebase JWT Validation (complète)
    ├─ Business Logic
    │
    ↓
PostgreSQL auth (Docker)
Redis auth (Docker)
```

### Ports utilisés

| Service | Port | Protocol | Accès |
|---------|------|----------|-------|
| **Kong Proxy** | 80 | HTTP | Public |
| **Kong Proxy TLS** | 443 | HTTPS | Public |
| **Kong Admin** | 8001 | HTTP | Localhost |
| **K3s API** | 6443 | HTTPS | GitHub Actions |
| **PostgreSQL auth** | 5433 | TCP | Localhost |
| **Redis auth** | 6380 | TCP | Localhost |
| **Redis Kong** | 6379 | TCP | Localhost |
| **MongoDB** | 27017 | TCP | Localhost |
| **auth-service** | 50051 | gRPC | K3s internal |

---

## 📦 Prérequis

### Système

- Ubuntu 24.04 LTS (fresh install recommandé)
- Accès root (sudo)
- Minimum 2 GB RAM libre
- Minimum 10 GB espace disque libre
- Connexion internet stable

### Outils requis

Les scripts d'installation installeront automatiquement :
- Docker & Docker Compose
- K3s (Kubernetes)
- Helm 3
- kubectl

### Accès

- Accès SSH au VPS
- Clés SSH configurées
- Nom de domaine pointé vers l'IP du VPS (optionnel pour dev)

---

## 🚀 Installation

### Guide d'installation complet

#### Étape 1 : Préparation du VPS

```bash
# Mettre à jour le système
apt update && apt upgrade -y

# Installer les outils de base
apt install -y curl wget git nano ufw

# Configurer le firewall
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 6443/tcp  # K3s API (GitHub Actions)
ufw enable
```

#### Étape 2 : Cloner le repository

```bash
# Créer le dossier projet
mkdir -p /root/tissimah
cd /root/tissimah

# Cloner le repo (ou transférer les fichiers)
git clone https://github.com/votre-username/tissimah.git .

# Ou via SCP
scp -r infrastructure/ root@vps-ip:/root/tissimah/
```

#### Étape 3 : Installer K3s

```bash
cd infrastructure/vps-dev

# Rendre le script exécutable
chmod +x k3s-install.sh

# Exécuter l'installation
./k3s-install.sh

# Vérifier l'installation
kubectl get nodes
# Expected: node status Ready
```

**Durée** : ~2-3 minutes

**Ce que fait le script** :
- ✅ Vérifie les prérequis système
- ✅ Télécharge et installe K3s v1.28.5+k3s1
- ✅ Désactive Traefik (on utilise Kong)
- ✅ Configure kubectl
- ✅ Vérifie la santé du cluster

#### Étape 4 : Installer Docker

```bash
# Rendre le script exécutable
chmod +x docker-install.sh

# Exécuter l'installation
./docker-install.sh

# Vérifier l'installation
docker --version
docker compose version
```

**Durée** : ~3-5 minutes

**Ce que fait le script** :
- ✅ Installe Docker CE
- ✅ Installe Docker Compose V2
- ✅ Configure le daemon Docker
- ✅ Ajoute l'utilisateur au groupe docker
- ✅ Lance le test hello-world

#### Étape 5 : Configurer les bases de données

```bash
cd ../databases

# Créer le fichier .env depuis le template
cp .env.example .env

# Générer des mots de passe sécurisés
openssl rand -base64 32  # Pour AUTH_POSTGRES_PASSWORD
openssl rand -base64 32  # Pour AUTH_REDIS_PASSWORD
openssl rand -base64 32  # Pour KONG_REDIS_PASSWORD

# Éditer .env et ajouter les mots de passe
nano .env
```

**Contenu minimal de .env** :
```bash
# Auth Service
AUTH_POSTGRES_DB=auth_db
AUTH_POSTGRES_USER=auth_user
AUTH_POSTGRES_PASSWORD=<mot_de_passe_1>
AUTH_POSTGRES_PORT=5432
AUTH_POSTGRES_HOST=127.0.0.1
POSTGRES_HOST_AUTH_METHOD=scram-sha-256

AUTH_REDIS_PASSWORD=<mot_de_passe_2>
AUTH_REDIS_PORT=6379

# Kong
KONG_REDIS_PASSWORD=<mot_de_passe_3>
KONG_REDIS_PORT=6380
```

#### Étape 6 : Démarrer les bases de données

```bash
# Démarrer PostgreSQL et Redis pour auth-service
docker compose -f docker-compose-auth.yml up -d

# Vérifier
docker ps
docker compose -f docker-compose-auth.yml logs

# Tester la connexion
docker exec tissimah-postgres-auth pg_isready
docker exec tissimah-redis-auth redis-cli -a ${AUTH_REDIS_PASSWORD} ping
```

**Durée** : ~1 minute

#### Étape 7 : Réorganiser les migrations

```bash
cd ../../services/auth-service/migrations

# Option manuelle :
mkdir -p up down
mv *_*.up.sql up/
mv *_*.down.sql down/
```

**Structure finale** :
```
services/auth-service/migrations/
├── up/
│   └── 000001_create_auth_table.sql
└── down/
    └── 000001_drop_auth_table.sql
```

#### Étape 8 : Installer Kong

```bash
cd ../../../infrastructure/databases

# Démarrer Redis Kong
docker compose -f docker-compose-kong-redis.yml up -d

# Vérifier
docker exec tissimah-redis-kong redis-cli -p 6379 -a ${KONG_REDIS_PASSWORD} ping

# Installer Kong avec Helm
chmod +x kong-install.sh
./kong-install.sh

# Vérifier l'installation
kubectl get pods -l app.kubernetes.io/name=kong
kubectl get svc -l app.kubernetes.io/name=kong
```

**Durée** : ~5-7 minutes

**Ce que fait le script** :
- ✅ Vérifie les prérequis
- ✅ Charge les variables .env
- ✅ Démarre Redis Kong
- ✅ Crée les Secrets K8s
- ✅ Installe Kong avec Helm
- ✅ Vérifie la santé de Kong

#### Étape 9 : Appliquer les plugins et routes Kong

```bash
cd ../helm-charts/kong

# Appliquer les plugins
helm template . --values values-vps-dev.yaml \
  --show-only plugins/cors.yaml | kubectl apply -f -

helm template . --values values-vps-dev.yaml \
  --show-only plugins/jwt-firebase.yaml | kubectl apply -f -

helm template . --values values-vps-dev.yaml \
  --show-only plugins/rate-limiting.yaml | kubectl apply -f -

helm template . --values values-vps-dev.yaml \
  --show-only plugins/grpc-gateway.yaml | kubectl apply -f -

# Vérifier les plugins
kubectl get kongplugins

# Les routes seront appliquées après le déploiement d'auth-service
```

#### Étape 10 : Configurer GitHub Actions

```bash
cd ../../vps-dev

# Générer le secret VPS_KUBECONFIG
chmod +x generate-vps-kubeconfig-secret.sh
./generate-vps-kubeconfig-secret.sh

# Copier la valeur base64 affichée
# Aller sur GitHub → Settings → Secrets → Actions
# Créer un secret nommé VPS_KUBECONFIG avec la valeur
```

#### Étape 11 : Déployer auth-service

```bash
# Via GitHub Actions (automatique)
git add .
git commit -m "feat: setup VPS infrastructure"
git push origin develop

# OU manuellement avec Helm
helm upgrade --install auth-service \
  services/auth-service/deployments/helm \
  -f services/auth-service/deployments/helm/values/vps-dev.yaml \
  --namespace default \
  --set image.tag=develop-latest
```

#### Étape 12 : Appliquer les routes Kong

```bash
cd infrastructure/helm-charts/kong

# Appliquer les routes auth-service
helm template . --values values-vps-dev.yaml \
  --show-only routes/auth-routes.yaml | kubectl apply -f -

# Vérifier
kubectl get ingress
```

#### Étape 13 : Vérification finale

```bash
# Vérifier tous les pods
kubectl get pods --all-namespaces

# Vérifier les services
kubectl get svc

# Tester Kong
curl http://localhost/api/v1/auth/health

# Tester une route protégée (devrait retourner 401)
curl http://localhost/api/v1/auth/login
```

---

## ⚙️ Configuration

### Variables d'environnement (.env)

Le fichier `.env` dans `infrastructure/databases/` contient toutes les configurations sensibles.

**Structure** :
```bash
# =============================================================================
# AUTH SERVICE - PostgreSQL
# =============================================================================
AUTH_POSTGRES_DB=auth_db
AUTH_POSTGRES_USER=auth_user
AUTH_POSTGRES_PASSWORD=changeme
AUTH_POSTGRES_PORT=5432
AUTH_POSTGRES_HOST=127.0.0.1
POSTGRES_HOST_AUTH_METHOD=scram-sha-256

# =============================================================================
# AUTH SERVICE - Redis
# =============================================================================
AUTH_REDIS_PASSWORD=changeme
AUTH_REDIS_PORT=6379

# =============================================================================
# KONG API GATEWAY - Redis
# =============================================================================
KONG_REDIS_PASSWORD=changeme
KONG_REDIS_PORT=6380

# Ajouter les autres services au fur et à mesure...
```

**⚠️ IMPORTANT** :
- Ne jamais commiter `.env` dans Git
- Générer des mots de passe forts (32+ caractères)
- Sauvegarder `.env` dans un gestionnaire de secrets

### Kong Configuration

Kong est configuré via Helm values dans `infrastructure/helm-charts/kong/values-vps-dev.yaml`.

**Principales configurations** :
```yaml
environment: vps-dev
kong:
  host: dev.tissi-mah.com
  replicaCount: 1
  
redis:
  host: 127.0.0.1
  port: 6380
  
rateLimiting:
  global:
    minute: 120
    hour: 5000
```

### Kubernetes Namespace

**Tout est dans le namespace `default`** :
- Kong Gateway
- Tous les microservices
- Secrets et ConfigMaps

**Pourquoi "default" ?**
- Simplifie la communication entre services
- Pas besoin de FQDN (auth-service vs auth-service.default.svc.cluster.local)
- Cohérence avec les configurations

---

## 🔧 Maintenance

### Opérations quotidiennes

#### Vérifier la santé du système

```bash
# Status K3s
kubectl get nodes
kubectl get pods --all-namespaces

# Status Docker
docker ps
docker stats --no-stream

# Espace disque
df -h

# Mémoire
free -h
```

#### Voir les logs

```bash
# Logs Kong
kubectl logs -l app.kubernetes.io/name=kong --tail=100

# Logs auth-service
kubectl logs -l app.kubernetes.io/name=auth-service --tail=100

# Logs bases de données
docker compose -f infrastructure/databases/docker-compose-auth.yml logs
```

#### Redémarrer un service

```bash
# Redémarrer Kong
kubectl rollout restart deployment kong

# Redémarrer auth-service
kubectl rollout restart deployment auth-service

# Redémarrer Redis Kong
docker compose -f infrastructure/databases/docker-compose-kong-redis.yml restart
```

### Sauvegardes

#### Bases de données

```bash
cd infrastructure/databases

# Backup PostgreSQL auth
docker exec tissimah-postgres-auth pg_dump -U auth_user auth_db > backup-auth-$(date +%Y%m%d).sql

# Backup toutes les bases
./backup-databases.sh

# Les backups sont dans: infrastructure/databases/backups/
```

#### Configuration K3s

```bash
# Backup kubeconfig
cp /etc/rancher/k3s/k3s.yaml ~/k3s-backup-$(date +%Y%m%d).yaml

# Backup manifests
kubectl get all --all-namespaces -o yaml > ~/k8s-all-backup-$(date +%Y%m%d).yaml
```

### Mises à jour

#### Mettre à jour Kong

```bash
cd infrastructure/helm-charts/kong

# Voir la version actuelle
helm list

# Mettre à jour le repo Helm
helm repo update

# Upgrade Kong
helm upgrade kong kong/kong \
  --namespace default \
  --values values-vps-dev.yaml \
  --version 2.39.0
```

#### Mettre à jour un service

```bash
# Via GitHub Actions (automatique)
git push origin develop

# OU manuellement
helm upgrade auth-service \
  services/auth-service/deployments/helm \
  -f services/auth-service/deployments/helm/values/vps-dev.yaml \
  --namespace default \
  --set image.tag=develop-new-sha
```

---

## 🔄 CI/CD

### GitHub Actions Workflow

Le workflow `deploy-to-vps.yml` déploie automatiquement sur push vers `develop`.

**Déclenchement automatique** :
- Push sur branche `develop`
- Modifications dans `services/**` ou `pkg/**`

**Déclenchement manuel** :
```
GitHub → Actions → Deploy to VPS → Run workflow
```

### Pipeline

```
1. Detect Changes
   └─ Identifie les services modifiés

2. Build and Push
   └─ Build Docker images
   └─ Push vers GHCR

3. Deploy
   └─ Deploy avec Helm dans K3s

4. Health Check
   └─ Vérifie la santé des pods

5. Summary
   └─ Affiche le résultat
```

### Secrets GitHub requis

| Secret | Description | Comment l'obtenir |
|--------|-------------|-------------------|
| `GITHUB_TOKEN` | Token GitHub (automatique) | Fourni automatiquement |
| `VPS_KUBECONFIG` | Kubeconfig encodé base64 | `./generate-vps-kubeconfig-secret.sh` |

---

## 🐛 Troubleshooting

### Problèmes courants

#### Kong ne démarre pas

```bash
# Vérifier les logs
kubectl logs -l app.kubernetes.io/name=kong

# Vérifier que Redis Kong fonctionne
docker exec tissimah-redis-kong redis-cli -p 6380 -a ${KONG_REDIS_PASSWORD} ping

# Vérifier les secrets
kubectl get secret kong-redis-secret
kubectl describe secret kong-redis-secret
```

#### Routes Kong retournent 503

```bash
# Vérifier que auth-service est déployé
kubectl get pods -l app.kubernetes.io/name=auth-service

# Vérifier que le service existe
kubectl get svc auth-service

# Tester depuis Kong
kubectl exec -it deployment/kong -- curl http://auth-service:50051
```

#### Auth-service ne peut pas se connecter à PostgreSQL

```bash
# Vérifier que PostgreSQL fonctionne
docker ps | grep tissimah-postgres-auth

# Tester la connexion
docker exec tissimah-postgres-auth pg_isready

# Vérifier les logs PostgreSQL
docker logs tissimah-postgres-auth

# Vérifier que le port est accessible depuis K3s
kubectl run -it --rm debug --image=postgres:16-alpine --restart=Never -- \
  psql -h 127.0.0.1 -p 5432 -U auth_user -d auth_db
```

#### Rate-limiting ne fonctionne pas

```bash
# Vérifier Redis Kong
docker ps | grep tissimah-redis-kong

# Tester depuis Kong
kubectl exec -it deployment/kong -- redis-cli -h 127.0.0.1 -p 6380 -a ${KONG_REDIS_PASSWORD} ping

# Voir les clés de rate-limiting
docker exec tissimah-redis-kong redis-cli -p 6380 -a ${KONG_REDIS_PASSWORD} KEYS "rate-limit:*"
```

#### GitHub Actions ne peut pas se connecter

```bash
# Vérifier que le port 6443 est ouvert
ufw status | grep 6443

# Tester depuis l'extérieur
curl -k https://VPS_IP:6443/version

# Vérifier les certificats K3s
ls -la /var/lib/rancher/k3s/server/tls/
```

### Commandes de diagnostic

```bash
# État général du système
kubectl get all --all-namespaces
docker ps -a
docker stats --no-stream

# Espace disque
df -h
du -sh /var/lib/docker
du -sh /var/lib/rancher

# Logs système
journalctl -u k3s -n 100
journalctl -u docker -n 100

# Réseau
netstat -tulpn | grep -E '(6443|6379|6380|5432|50051)'
```

---

## 🔒 Sécurité

### Bonnes pratiques

#### Mots de passe

- ✅ Utiliser `openssl rand -base64 32` pour générer
- ✅ Minimum 16 caractères
- ✅ Rotation régulière (tous les 3 mois)
- ✅ Ne jamais commiter dans Git

#### Firewall

```bash
# Configuration minimale
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 6443/tcp  # K3s API

# Bloquer tout le reste
ufw default deny incoming
ufw default allow outgoing
```

#### Accès SSH

```bash
# Désactiver mot de passe, utiliser clés SSH
nano /etc/ssh/sshd_config
# PasswordAuthentication no
# PubkeyAuthentication yes

systemctl restart sshd
```

#### Secrets Kubernetes

```bash
# Vérifier les secrets
kubectl get secrets

# Ne jamais logger les secrets
kubectl get secret kong-redis-secret -o yaml  # ⚠️ Attention
```

### Mises à jour de sécurité

```bash
# Mettre à jour le système régulièrement
apt update && apt upgrade -y

# Redémarrer si nécessaire
reboot
```

---

## 📚 Ressources

### Documentation

- [K3s Documentation](https://docs.k3s.io/)
- [Kong Documentation](https://docs.konghq.com/)
- [Docker Documentation](https://docs.docker.com/)
- [Helm Documentation](https://helm.sh/docs/)

### Scripts disponibles

| Script | Description | Usage |
|--------|-------------|-------|
| `k3s-install.sh` | Installe K3s | `./k3s-install.sh` |
| `docker-install.sh` | Installe Docker | `./docker-install.sh` |
| `kong-install.sh` | Installe Kong | `./kong-install.sh` |
| `generate-vps-kubeconfig-secret.sh` | Génère VPS_KUBECONFIG | `./generate-vps-kubeconfig-secret.sh` |
| `backup-configs.sh` | Sauvegarde les configs | `./backup-configs.sh` |

### Commandes utiles

```bash
# K3s
kubectl get pods --all-namespaces
kubectl logs <pod-name>
kubectl describe pod <pod-name>
kubectl exec -it <pod-name> -- sh

# Docker
docker ps
docker logs <container-name>
docker exec -it <container-name> sh
docker compose -f <file> up -d

# Helm
helm list
helm status <release>
helm upgrade <release> <chart>
```

---

## 🎯 Checklist de déploiement

### Installation initiale

- [ ] VPS provisionné (Ubuntu 24.04)
- [ ] Firewall configuré
- [ ] K3s installé
- [ ] Docker installé
- [ ] .env configuré avec mots de passe
- [ ] Bases de données démarrées
- [ ] Migrations exécutées
- [ ] Kong installé
- [ ] Plugins Kong appliqués
- [ ] Secret GitHub VPS_KUBECONFIG créé
- [ ] Premier déploiement réussi
- [ ] Routes Kong appliquées
- [ ] Tests de santé passés

### Avant chaque déploiement

- [ ] Tests locaux passés
- [ ] Branch develop à jour
- [ ] Pas de secrets dans le code
- [ ] Documentation mise à jour
- [ ] CI/CD vérifié

### Après chaque déploiement

- [ ] Pods en état Running
- [ ] Health checks passés
- [ ] Logs sans erreurs critiques
- [ ] Routes Kong fonctionnelles
- [ ] Rate-limiting actif

---

## 📞 Support

En cas de problème :

1. Consulter ce README
2. Vérifier les logs (Kong, services, Docker)
3. Consulter le Troubleshooting
4. Contacter l'équipe DevOps

---

**Documentation maintenue par** : Équipe TissiMah DevOps  
**Dernière mise à jour** : Janvier 2026  
**Version** : 1.0.0