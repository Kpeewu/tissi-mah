# TissiMah — Guide Claude Code

## Présentation du projet

**TissiMah** est une plateforme de covoiturage africaine construite en microservices (Go).
Architecture monorepo avec Go workspace, gRPC inter-services, Kong API Gateway, Kubernetes (K3s dev / EKS prod).

**Module root** : `github.com/Kpeewu/tissi-mah`
**Go version** : 1.25.6 (go.work)
**Branche principale** : `main`

---

## Structure du monorepo

```
tissiMah/
├── pkg/                    # Packages partagés (config, db, grpc, logger, errors)
├── pkg-test/               # Utilitaires de test (testcontainers)
├── services/
│   └── auth-service/       # Seul service actif (feat/auth-service-implementation)
│       # user-service, payment-service → TODO
├── infrastructure/
│   ├── helm-charts/kong/   # Kong API Gateway (plugins + routes)
│   └── terraform/          # IaC AWS (VPC, EKS, RDS, ElastiCache, DocumentDB)
├── scripts/                # DB, deployment, proto generation, utilities
├── docs/                   # Architecture, API, dev guides, ADRs
├── go.work                 # Go workspace
├── docker-compose.yml      # PostgreSQL auth + Redis (dev local)
└── Makefile                # Commandes principales
```

---

## Commandes clés (Makefile)

```bash
make deps-up          # Démarre PostgreSQL + Redis (Docker)
make run-auth         # Lance auth-service
make proto            # Génère tous les protos
make proto-sync       # Sync protos → Helm charts
make test-all         # Tous les tests
make lint             # golangci-lint
make migrate          # Migrations DB (UP)
make workspace-sync   # go work sync
```

---

## Packages partagés (`pkg/`)

| Package | Rôle |
|---------|------|
| `pkg/config` | Viper-based env var loading (`MustGetString`, `GetStringOrDefault`) |
| `pkg/database/postgres` | pgx v5 connection pool (`NewPostgresPoolFromURL`) |
| `pkg/database/redis` | Redis client (`NewRedisClientFromURL`) |
| `pkg/grpcutil/server` | Factory serveur gRPC avec health check |
| `pkg/logger` | Zap structured logger (dev/prod mode) |
| `pkg/errors` | Types d'erreurs partagés |

---

## Auth-service — Architecture en couches

```
grpc/handler.go (transport)
    ↓
middleware/interceptor.go (Firebase JWT validation)
    ↓
service/auth_service_impl.go (business logic)
    ↓
repository/implementations/ (PostgreSQL via pgx)
    ↓
domain/auth.go + domain/userPreview.go
```

**Clé de contexte Firebase UID** : `middleware.FirebaseIDKey` (type opaque `contextKey`)

### Flux d'authentification

```
Client → Bearer JWT → Kong (validation basique: format + exp)
    → auth-service middleware (Firebase Admin SDK: signature, issuer, audience)
    → context.WithValue(ctx, middleware.FirebaseIDKey, uid)
    → service layer
```

### Routes protégées (JWT requis)
- `/auth.AuthService/Login`
- `/auth.AuthService/CreateAccount`
- `/auth.AuthService/DeleteAccount`

### Routes publiques (pas de JWT)
- `/auth.AuthService/Health`
- `/auth.AuthService/CheckEmail`
- `/auth.AuthService/CheckPhoneNumber`

---

## Variables d'environnement — auth-service

| Variable | Obligatoire | Description |
|----------|-------------|-------------|
| `DATABASE_URL` | oui | PostgreSQL connection URL |
| `REDIS_URL` | oui | Redis connection URL |
| `FIREBASE_PROJECT_ID` | oui | Firebase project ID (ex: `tissi-mah-dev`) |
| `ENVIRONMENT` | oui | `local` / `vps-dev` / `staging` / `prod` |
| `GRPC_PORT` | non (50051) | Port gRPC |
| `GRPC_ADDRESS` | non (0.0.0.0) | Adresse bind |
| `USER_SERVICE_HOST` | non (0.0.0.0) | Host user-service |
| `USER_SERVICE_PORT` | non (50052) | Port user-service |
| `LOG_LEVEL` | oui | `debug` / `info` / `warn` / `error` |

---

## Conventions de code

- **Langue** : Go, code en anglais, commentaires en français
- **Erreurs** : Sentinel errors dans `pkg/errors/errors.go` et `services/*/pkg/errors/`
- **Context keys** : type opaque `contextKey` dans le package qui les **set** (middleware)
- **Repository pattern** : interfaces dans `repository/interfaces/`, impl dans `repository/implementations/`
- **Interfaces** : définies dans `service/interfaces/` et `repository/interfaces/`
- **UUIDs** : `github.com/google/uuid` pour tous les IDs internes
- **Soft delete** : `deleted_at` nullable, anonymisation GDPR via `AnonymizeAndDelete()`
- **Proto** : HTTP annotations Google API pour transcoding Kong gRPC-Gateway

---

## Dépendances principales (auth-service)

| Dépendance | Version | Usage |
|-----------|---------|-------|
| `google.golang.org/grpc` | v1.78.0 | Framework gRPC |
| `google.golang.org/protobuf` | v1.36.11 | Protobuf |
| `github.com/jackc/pgx/v5` | v5.8.0 | PostgreSQL driver |
| `firebase.google.com/go/v4` | v4.19.0 | Firebase Admin SDK (JWT validation) |
| `github.com/google/uuid` | v1.6.0 | UUID generation |
| `github.com/spf13/viper` | v1.21.0 | Configuration |

---

## État du projet

| Composant | Statut |
|-----------|--------|
| `pkg/` partagé | Complet |
| `auth-service` domain + repository | Complet |
| `auth-service` service layer | Complet |
| `auth-service` middleware + Firebase JWT | Complet |
| `auth-service` gRPC handler + server | Complet |
| `auth-service` cmd/server/main.go | Complet |
| Tests unitaires + intégration | Structure créée, à compléter |
| `user-service` | TODO |
| `payment-service` | TODO |

---

## Fichiers importants

| Fichier | Rôle |
|---------|------|
| [go.work](go.work) | Workspace Go — modules actifs |
| [Makefile](Makefile) | Toutes les commandes développeur |
| [docker-compose.yml](docker-compose.yml) | Bases de données dev local |
| [services/auth-service/cmd/server/main.go](services/auth-service/cmd/server/main.go) | Point d'entrée — wiring complet |
| [services/auth-service/internal/grpc/handler.go](services/auth-service/internal/grpc/handler.go) | Handlers gRPC + mapping erreurs |
| [services/auth-service/internal/grpc/server.go](services/auth-service/internal/grpc/server.go) | Setup serveur gRPC (grpcutil) |
| [services/auth-service/proto/auth.proto](services/auth-service/proto/auth.proto) | Contrat API gRPC |
| [services/auth-service/internal/service/interfaces/auth_service.go](services/auth-service/internal/service/interfaces/auth_service.go) | Interface métier principale |
| [services/auth-service/internal/middleware/interceptor.go](services/auth-service/internal/middleware/interceptor.go) | Intercepteur gRPC + FirebaseIDKey |
| [services/auth-service/pkg/firebase/jwt_validator.go](services/auth-service/pkg/firebase/jwt_validator.go) | Validation Firebase JWT |
| [infrastructure/helm-charts/kong/templates/plugins/jwt-firebase.yaml](infrastructure/helm-charts/kong/templates/plugins/jwt-firebase.yaml) | Stratégie validation JWT Kong |
| [docs/architecture/overview.md](docs/architecture/overview.md) | Vue d'ensemble architecture |
