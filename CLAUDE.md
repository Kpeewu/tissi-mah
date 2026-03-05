# TissiMah — Guide Claude Code

## Présentation du projet

**TissiMah** est une plateforme de covoiturage africaine construite en microservices (Go).
Architecture monorepo avec Go workspace, gRPC inter-services, api-gateway custom (grpc-gateway), Kubernetes (K3s dev / EKS prod).

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
│   ├── api-gateway/        # API Gateway HTTP→gRPC (grpc-gateway, Firebase JWT, rate limiting, CORS)
│   ├── auth-service/       # Service d'authentification (PostgreSQL)
│   ├── user-service/       # Service utilisateur (MongoDB)
│   └── rating-service/     # Service de notation (PostgreSQL)
│       # payment-service → TODO
├── infrastructure/
│   └── terraform/          # IaC AWS (VPC, EKS, RDS, ElastiCache, DocumentDB)
├── scripts/                # DB, deployment, proto generation, utilities
├── docs/                   # Architecture, API, dev guides, ADRs
├── go.work                 # Go workspace
├── docker-compose.yml      # PostgreSQL, MongoDB, Redis x3 (dev local)
└── Makefile                # Commandes principales
```

---

## Architecture

```
Client (HTTPS) → [K8s Ingress/LoadBalancer]
    → api-gateway (HTTP :8080)
        → CORS middleware
        → Rate limiting middleware (Redis)
        → JWT Firebase middleware (validation complète)
        → grpc-gateway mux (HTTP→gRPC, injecte x-firebase-uid en metadata)
            → auth-service:50051 (gRPC)
            → user-service:50052 (gRPC)
            → rating-service:50054 (gRPC)
```

### Flux d'authentification

```
Client → Bearer JWT → api-gateway (Firebase Admin SDK: signature, issuer, audience)
    → Set header x-firebase-uid → grpc-gateway → metadata gRPC
    → service middleware: lire x-firebase-uid depuis metadata
    → context.WithValue(ctx, middleware.FirebaseIDKey, uid)
    → service layer
```

**Clé de contexte Firebase UID** : `middleware.FirebaseIDKey` (type opaque `contextKey`)

---

## Commandes clés (Makefile)

```bash
make deps-up          # Démarre PostgreSQL, MongoDB, Redis x3 (Docker)
make run-auth         # Lance auth-service
make run-user         # Lance user-service
make run-gateway      # Lance api-gateway
make proto            # Génère tous les protos
make proto-gateway    # Génère proto api-gateway (avec grpc-gateway)
make proto-sync       # Sync protos → api-gateway
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
| `pkg/database/mongodb` | MongoDB client (`NewMongoClientFromURL`) |
| `pkg/database/redis` | Redis client (`NewRedisClientFromURL`) |
| `pkg/grpcutil/server` | Factory serveur gRPC avec health check |
| `pkg/logger` | Zap structured logger (dev/prod mode) |
| `pkg/errors` | Types d'erreurs partagés |

---

## Services

### api-gateway — HTTP→gRPC reverse proxy

| Responsabilité | Détails |
|----------------|---------|
| Routing HTTP→gRPC | grpc-gateway/v2 (annotations `google.api.http` des protos) |
| JWT Firebase | Validation complète (Firebase Admin SDK), injection `x-firebase-uid` |
| Rate limiting | Redis distribué, 4 tiers (global/auth/create/sensitive) |
| CORS | Middleware HTTP, configurable par environnement |

**Port** : 8080 (HTTP)

### auth-service — Authentification

```
grpc/handler.go → middleware/interceptor.go (lire x-firebase-uid) → service → repository (PostgreSQL)
```

**Port** : 50051 (gRPC)

### user-service — Profils utilisateur

```
grpc/handler.go → middleware/interceptor.go (lire x-firebase-uid) → service → repository (MongoDB)
```

**Port** : 50052 (gRPC)

### rating-service — Notation des utilisateurs

```
grpc/handler.go → middleware/interceptor.go (lire x-firebase-uid) → service → repository (PostgreSQL)
```

**Port** : 50054 (gRPC)

**Endpoints HTTP** (via api-gateway) :
- `POST /api/v1/ratings` — Créer une note (public, rater_id dans le body)
- `GET /api/v1/ratings/{rating_id}` — Récupérer une note (public)
- `GET /api/v1/ratings/user/{user_rated_id}` — Notes d'un utilisateur (public)
- `GET /api/v1/ratings/user/{user_rated_id}/average` — Moyenne (public)
- `PATCH /api/v1/ratings/{rating_id}` — Modifier une note (public, rater_id dans le body)
- `POST /api/v1/ratings/{rating_id}/delete` — Supprimer une note (public, rater_id dans le body)

---

## Variables d'environnement

### api-gateway

| Variable | Obligatoire | Description |
|----------|-------------|-------------|
| `ENVIRONMENT` | oui | `local` / `vps-dev` / `staging` / `prod` |
| `REDIS_URL` | oui | Redis rate limiting (ex: `redis://localhost:6381/0`) |
| `FIREBASE_PROJECT_ID` | oui | Firebase project ID |
| `LOG_LEVEL` | oui | `debug` / `info` / `warn` / `error` |
| `HTTP_PORT` | non (8080) | Port HTTP |
| `AUTH_SERVICE_HOST` | non (0.0.0.0) | Host auth-service |
| `AUTH_SERVICE_PORT` | non (50051) | Port auth-service |
| `USER_SERVICE_HOST` | non (0.0.0.0) | Host user-service |
| `USER_SERVICE_PORT` | non (50052) | Port user-service |
| `RATING_SERVICE_HOST` | non (0.0.0.0) | Host rating-service |
| `RATING_SERVICE_PORT` | non (50054) | Port rating-service |
| `CORS_ALLOWED_ORIGINS` | non (*) | Origines CORS séparées par virgules |

### auth-service

| Variable | Obligatoire | Description |
|----------|-------------|-------------|
| `DATABASE_URL` | oui | PostgreSQL connection URL |
| `REDIS_URL` | oui | Redis connection URL |
| `ENVIRONMENT` | oui | `local` / `vps-dev` / `staging` / `prod` |
| `LOG_LEVEL` | oui | `debug` / `info` / `warn` / `error` |
| `GRPC_PORT` | non (50051) | Port gRPC |
| `USER_SERVICE_HOST` | non (0.0.0.0) | Host user-service |
| `USER_SERVICE_PORT` | non (50052) | Port user-service |

### user-service

| Variable | Obligatoire | Description |
|----------|-------------|-------------|
| `MONGODB_URL` | oui | MongoDB connection URL |
| `REDIS_URL` | oui | Redis connection URL |
| `ENVIRONMENT` | oui | `local` / `vps-dev` / `staging` / `prod` |
| `LOG_LEVEL` | oui | `debug` / `info` / `warn` / `error` |
| `GRPC_PORT` | non (50052) | Port gRPC |
| `AUTH_SERVICE_HOST` | non (0.0.0.0) | Host auth-service |
| `AUTH_SERVICE_PORT` | non (50051) | Port auth-service |

### rating-service

| Variable | Obligatoire | Description |
|----------|-------------|-------------|
| `DATABASE_URL` | oui | PostgreSQL connection URL |
| `REDIS_URL` | oui | Redis connection URL |
| `ENVIRONMENT` | oui | `local` / `vps-dev` / `staging` / `prod` |
| `LOG_LEVEL` | oui | `debug` / `info` / `warn` / `error` |
| `GRPC_PORT` | non (50054) | Port gRPC |
| `USER_SERVICE_HOST` | non (0.0.0.0) | Host user-service |
| `USER_SERVICE_PORT` | non (50052) | Port user-service |

---

## Conventions de code

- **Langue** : Go, code en anglais, commentaires en français
- **Erreurs** : Sentinel errors dans `pkg/errors/errors.go` et `services/*/pkg/errors/`
- **Context keys** : type opaque `contextKey` dans le package qui les **set** (middleware)
- **Repository pattern** : interfaces dans `repository/interfaces/`, impl dans `repository/implementations/`
- **Interfaces** : définies dans `service/interfaces/` et `repository/interfaces/`
- **UUIDs** : `github.com/google/uuid` pour tous les IDs internes
- **Soft delete** : `deleted_at` nullable, anonymisation GDPR via `AnonymizeAndDelete()`
- **Proto** : HTTP annotations Google API pour transcoding grpc-gateway
- **Firebase UID** : transmis via metadata gRPC `x-firebase-uid` (injecté par api-gateway)

---

## État du projet

| Composant | Statut |
|-----------|--------|
| `pkg/` partagé | Complet |
| `api-gateway` | Complet |
| `auth-service` | Complet |
| `user-service` | Complet |
| `rating-service` | Complet |
| Tests unitaires + intégration | Structure créée, à compléter |
| `payment-service` | TODO |

---

## Fichiers importants

| Fichier | Rôle |
|---------|------|
| [go.work](go.work) | Workspace Go — modules actifs |
| [Makefile](Makefile) | Toutes les commandes développeur |
| [docker-compose.yml](docker-compose.yml) | Bases de données + Redis dev local |
| [services/api-gateway/cmd/server/main.go](services/api-gateway/cmd/server/main.go) | Point d'entrée api-gateway |
| [services/api-gateway/internal/gateway/mux.go](services/api-gateway/internal/gateway/mux.go) | grpc-gateway mux + metadata annotator |
| [services/api-gateway/internal/gateway/routes.go](services/api-gateway/internal/gateway/routes.go) | Routes protégées + tiers rate limiting |
| [services/api-gateway/internal/middleware/](services/api-gateway/internal/middleware/) | CORS, JWT Firebase, rate limiting |
| [services/api-gateway/pkg/firebase/jwt_validator.go](services/api-gateway/pkg/firebase/jwt_validator.go) | Validation Firebase JWT |
| [services/auth-service/cmd/server/main.go](services/auth-service/cmd/server/main.go) | Point d'entrée auth-service |
| [services/auth-service/internal/middleware/interceptor.go](services/auth-service/internal/middleware/interceptor.go) | Intercepteur gRPC (lire x-firebase-uid) |
| [services/auth-service/proto/auth.proto](services/auth-service/proto/auth.proto) | Contrat API gRPC auth |
| [services/user-service/cmd/server/main.go](services/user-service/cmd/server/main.go) | Point d'entrée user-service |
| [services/user-service/proto/user.proto](services/user-service/proto/user.proto) | Contrat API gRPC user |
| [services/rating-service/cmd/server/main.go](services/rating-service/cmd/server/main.go) | Point d'entrée rating-service |
| [services/rating-service/proto/rating.proto](services/rating-service/proto/rating.proto) | Contrat API gRPC rating |
| [docs/architecture/overview.md](docs/architecture/overview.md) | Vue d'ensemble architecture |
