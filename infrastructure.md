# TissiMah — Infrastructure & Stratégie de Déploiement

## Vue d'ensemble

Le projet utilise une infrastructure **multi-environnements progressive** :

```
Local Dev (Docker Compose)
    ↓
VPS-Dev (K3s + Kong — serveur dédié OVH/Hetzner)
    ↓
Staging (AWS EKS — scaled-down)
    ↓
Production (AWS EKS — multi-AZ, HA)
```

Toute l'infrastructure est en **Infrastructure-as-Code** :
- **Kubernetes deployments** : Helm Charts
- **Cloud provisioning** : Terraform
- **API Gateway** : Kong (Helm chart dédié)

---

## 1. Développement Local

### Stack Docker Compose

```yaml
# docker-compose.yml
services:
  auth-postgres:
    image: postgres:15-alpine
    port: 5432
    database: auth_db
    user/password: dev/dev123

  redis:
    image: redis:7-alpine
    port: 6379

networks:
  tissi-mah-network: bridge
```

**Commandes** :
```bash
make deps-up     # Démarre les containers
make deps-down   # Arrête les containers
make deps-clean  # Supprime les volumes (destructif)
make deps-logs   # Logs en direct
```

### Migrations locales
```bash
make migrate         # UP (applique les migrations)
make migrate-down    # Rollback
make migrate-status  # État actuel
```

Script : `services/auth-service/scripts/run_migration.sh`
Fichiers SQL : `services/auth-service/migrations/up/` et `migrations/down/`

---

## 2. VPS Dev (K3s)

### Spécifications serveur
| Paramètre | Valeur |
|-----------|--------|
| OS | Ubuntu 24.04 LTS |
| RAM | 12 GB |
| CPU | 4 vCPUs |
| Stockage | 100 GB SSD |
| Réseau | 1 Gbps |
| Provider | OVH ou Hetzner |

### Architecture VPS

```
Ubuntu VPS
├── Docker (network_mode: host)
│   ├── PostgreSQL auth      :5433
│   ├── Redis auth           :6380
│   ├── Redis Kong           :6379
│   └── MongoDB user         :27017
│
└── K3s Cluster
    ├── Kong Gateway         :80 / :443
    ├── auth-service         :50051 (gRPC)
    ├── user-service         :50052 (gRPC) — TODO
    └── trips-service        :50053 (gRPC) — TODO
```

### Scripts de setup

```bash
infrastructure/vps-dev/
├── docker-install.sh              # Docker + Docker Compose
├── k3s-install.sh                 # Kubernetes léger
├── databases/kong-install.sh      # Kong en mode DB-less
├── renew-certificate.sh           # Renouvellement SSL/TLS
├── backup-configs.sh              # Sauvegarde configurations
└── generate-vps-kubeconfig-secret.sh
```

### Valeurs Helm (vps-dev)
```
services/auth-service/deployments/helm/values/vps-dev.yaml
infrastructure/helm-charts/kong/values-vps-dev.yaml
```

---

## 3. Kong API Gateway

Kong est le **point d'entrée unique** de toutes les requêtes. Il est déployé via Helm chart dédié.

### Rôle de Kong

| Responsabilité | Plugin Kong |
|---------------|-------------|
| Validation JWT Firebase (basique) | `jwt-firebase.yaml` → `jwt-validation-basic` |
| Transcoding HTTP ↔ gRPC | `grpc-gateway.yaml` → `grpc-gateway-auth` |
| Rate limiting | `rate-limiting.yaml` |
| CORS | `cors.yaml` |
| Ajout headers | `request-transformer-jwt` |
| Load balancing | Upstream round-robin |
| TLS termination | Ingress TLS |

### Stratégie de validation JWT (Approche 3)

```
Client
  │ Authorization: Bearer <firebase_jwt>
  ▼
Kong (validation BASIQUE)
  ├── ✅ Format JWT valide
  ├── ✅ Claim exp non expiré
  ├── ❌ Pas de vérification signature
  └── Ajoute header: X-Kong-JWT-Validated: true
  │
  ▼
auth-service middleware (validation COMPLÈTE)
  ├── Signature RS256 (clés publiques Firebase)
  ├── Issuer: https://securetoken.google.com/<project-id>
  ├── Audience: <project-id>
  └── Extrait claim "sub" = Firebase UID → context
```

### Routes configurées

| Route | HTTP | Méthodes | JWT requis |
|-------|------|---------|------------|
| `/api/v1/auth/login` | POST | POST, OPTIONS | ✅ |
| `/api/v1/auth/createAccount` | POST | POST, OPTIONS | ✅ |
| `/api/v1/auth/deleteAccount` | DELETE | DELETE, OPTIONS | ✅ |
| `/api/v1/auth/checkEmail` | POST | POST, OPTIONS | ❌ |
| `/api/v1/auth/checkPhoneNumber` | POST | POST, OPTIONS | ❌ |
| `/api/v1/auth/health` | GET | GET, OPTIONS | ❌ |

### Rate Limiting

| Endpoint | /sec | /min | /hour | /day |
|----------|------|------|-------|------|
| Global | — | 60 | 1000 | — |
| Login | 1 | 10 | 100 | — |
| CreateAccount | — | 3 | 10 | 20 |
| Sensitive ops | — | 1 | 5 | 10 |

Stockage : Redis cluster

### Configuration prod Kong

```
Replicas:       3 (HA)
Image:          kong:3.5
Proxy:          LoadBalancer (NLB) + Cross-AZ
Ports:          8000 (HTTP), 8443 (TLS)
Admin API:      Désactivé en prod
Autoscaling:    Min 3, Max 10, CPU 60%, Memory 70%
Pod Affinity:   Anti-affinity requise, spread par zone
Security:       RunAsNonRoot (uid 1000), ReadOnlyRootFS
Monitoring:     ServiceMonitor Prometheus (interval: 30s)
```

### Helm chart Kong

```
infrastructure/helm-charts/kong/
├── Chart.yaml                  # v0.1.0
├── values-prod.yaml
├── values-staging.yaml
├── values-vps-dev.yaml
└── templates/
    ├── plugins/
    │   ├── cors.yaml
    │   ├── grpc-gateway.yaml
    │   ├── jwt-firebase.yaml
    │   └── rate-limiting.yaml
    └── routes/
        └── auth-routes.yaml
```

---

## 4. Helm Charts — Services

### auth-service Helm Chart

```
services/auth-service/deployments/helm/
├── Chart.yaml
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secrets.yaml
│   ├── serviceaccount.yaml
│   ├── hpa.yaml             # Horizontal Pod Autoscaler
│   ├── pdb.yaml             # Pod Disruption Budget
│   ├── proto-configmap.yaml
│   └── _helpers.tpl
└── values/
    ├── prod.yaml
    ├── staging.yaml
    ├── vps-dev.yaml
    └── local.yaml
```

### Ressources par environnement

| Paramètre | Local | VPS-Dev | Staging | Prod |
|-----------|-------|---------|---------|------|
| Replicas | 1 | 1 | 2 | 3 |
| CPU Request | — | 500m | 500m | 500m |
| CPU Limit | — | 1000m | 1000m | 1000m |
| RAM Request | — | 512Mi | 512Mi | 512Mi |
| RAM Limit | — | 1Gi | 1Gi | 1Gi |
| Autoscaling | Non | Non | Oui | Oui (3-10) |

### Image Docker

- **Registry** : `ghcr.io/kpeewu/tissi-mah/auth-service`
- **Tag** : SemVer (ex: `v1.2.3`)
- **Dockerfile** : multi-stage (120 lignes)
  - Stage 1 : `golang:1.25-alpine` (build)
  - Stage 2 : `alpine:latest` (runtime minimal)
- **Security** : User non-root (uid 1000), ReadOnlyRootFS

### Probes santé

```yaml
livenessProbe:
  grpc: port 50051
  initialDelaySeconds: 20
  periodSeconds: 10

readinessProbe:
  grpc: port 50051
  initialDelaySeconds: 15
  periodSeconds: 5
```

Protocole : **gRPC Health Check v1**

---

## 5. Terraform (AWS)

### Structure

```
infrastructure/terraform/
├── modules/           # Modules réutilisables
│   ├── vpc/           # VPC, Subnets, Security Groups, NAT Gateway
│   ├── eks/           # EKS Cluster, Node Groups, IAM roles, Addons
│   ├── rds/           # RDS PostgreSQL managé + backups
│   ├── documentdb/    # MongoDB-compatible (profils utilisateurs)
│   ├── elasticache/   # Redis managé (cache + rate limiting)
│   └── monitoring/    # CloudWatch dashboards + alertes
└── environments/
    ├── prod/          # 8 fichiers .tf
    └── staging/       # Configuration similaire, scaled-down
```

### Environnement prod — fichiers Terraform

| Fichier | Contenu |
|---------|---------|
| `main.tf` | Providers, versions, modules |
| `networking.tf` | VPC, subnets, security groups |
| `eks.tf` | EKS cluster, node groups, IRSA |
| `databases.tf` | RDS PostgreSQL + DocumentDB |
| `cache.tf` | ElastiCache Redis (cluster mode) |
| `monitoring.tf` | CloudWatch, métriques, alertes |
| `variables.tf` | Variables d'entrée |
| `backend.tf` | Remote state (S3 + DynamoDB) |

### Services AWS provisionés

| Service AWS | Usage |
|-------------|-------|
| EKS | Kubernetes managé (multi-AZ) |
| RDS PostgreSQL | Auth data (persistant) |
| DocumentDB | Profils utilisateurs (MongoDB-compatible) |
| ElastiCache Redis | Caching + Kong rate limiting |
| VPC + Security Groups | Isolation réseau |
| CloudWatch | Logs centralisés + alertes |
| S3 + DynamoDB | Terraform remote state |

### Commandes Terraform

```bash
cd infrastructure/terraform/environments/prod
terraform init
terraform plan
terraform apply

# Module spécifique
terraform apply -target=module.eks
```

---

## 6. Scripts utilitaires

```
scripts/
├── database/
│   ├── migrate-all-services.sh   # Orchestration migrations
│   ├── backup-databases.sh
│   ├── restore-databases.sh
│   └── init-databases.sql
│
├── development/
│   ├── generate-all-protos.sh
│   ├── run-all-tests.sh
│   ├── clean-docker.sh
│   └── setup-local-env.sh
│
├── deployment/
│   ├── deploy-service.sh         # Deploy via Helm
│   ├── health-check.sh
│   └── rollback-service.sh
│
└── utilities/
    ├── generate-secrets.sh
    ├── install-dependencies.sh
    ├── update-dependencies.sh
    ├── sync-protos-to-helm.sh    # Copie protos générés → Helm configmaps
    └── lint-all.sh
```

### Déployer un service

```bash
# Deploy
./scripts/deployment/deploy-service.sh auth-service

# Health check
./scripts/deployment/health-check.sh auth-service

# Rollback
./scripts/deployment/rollback-service.sh auth-service
```

---

## 7. Observabilité

### Métriques Kong (Prometheus)

```
kong_http_requests_total       # Requêtes par code/route
kong_latency_*                 # Histogrammes de latence
kong_upstream_target_health    # Santé des endpoints
```

### Alertes Kong (prod)

```yaml
KongHighErrorRate:
  condition: rate(HTTP 5xx) > 5% pendant 5 min
  severity: critical

KongHighLatency:
  condition: P99 latency > 1000ms pendant 5 min
  severity: warning

KongPodNotReady:
  condition: pod not ready > 2 min
  severity: critical
```

### Logs

- Zap structured logging (JSON en prod, console coloré en dev)
- CloudWatch Logs en prod/staging
- Log level configurable : `LOG_LEVEL=debug|info|warn|error`

---

## 8. Sécurité

| Mesure | Description |
|--------|-------------|
| TLS | Terminaison Kong, certificats manuels (dev) |
| JWT Firebase | Validation complète côté service (signature RS256) |
| Non-root containers | uid 1000, ReadOnlyRootFS |
| Network isolation | VPC, Security Groups (AWS) |
| Secrets Kubernetes | Credentials DB/Redis/Firebase en Secret K8s |
| Rate limiting | Kong + Redis, limites strictes par endpoint |
| Admin API Kong | Désactivée en production |
| Pod Security | Anti-affinity + Pod Disruption Budget (prod) |
