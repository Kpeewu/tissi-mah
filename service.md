# TissiMah — Architecture et structure d'un service

Ce document décrit les conventions, l'architecture et le fonctionnement applicables à **tous les services** du monorepo.
Le service de référence est `auth-service` (seul service implémenté à ce jour).

---

## 1. Structure type d'un service

```
services/<service-name>/
│
├── cmd/server/
│   └── main.go                          # Point d'entrée — wiring de toutes les dépendances
│
├── internal/                            # Code privé (non importable de l'extérieur)
│   ├── client/
│   │   └── <dep>_service_client.go      # Clients vers d'autres services (gRPC stubs)
│   ├── config/
│   │   └── config.go                    # Chargement config spécifique au service
│   ├── domain/
│   │   ├── <entity>.go                  # Entités métier + méthodes business
│   │   └── <dto>.go                     # DTOs (ex: UserPreview)
│   ├── grpc/
│   │   ├── handler.go                   # Handlers gRPC (implémente l'interface proto)
│   │   └── server.go                    # Setup serveur gRPC (wiring)
│   ├── middleware/
│   │   └── interceptor.go               # Intercepteurs gRPC (auth, logging...)
│   ├── repository/
│   │   ├── interfaces/
│   │   │   ├── <entity>_repository_read.go
│   │   │   └── <entity>_repository_write.go
│   │   └── implementations/
│   │       ├── <entity>_repository_read_impl.go
│   │       └── <entity>_repository_write_impl.go
│   └── service/
│       ├── interfaces/
│       │   └── <entity>_service.go      # Interface métier (contrat)
│       └── <entity>_service_impl.go     # Implémentation business logic
│
├── pkg/                                 # Code public du service (réutilisable)
│   ├── errors/
│   │   └── errors.go                    # Sentinel errors du service
│   └── <feature>/
│       └── <feature>.go                 # Ex: firebase/jwt_validator.go
│
├── proto/
│   ├── <service>.proto                  # Définition du contrat gRPC
│   ├── google/api/                      # Annotations HTTP Google API
│   └── gen/                             # Code généré (*.pb.go, *_grpc.pb.go)
│
├── migrations/
│   ├── up/
│   │   └── 000001_create_<table>.sql
│   └── down/
│       └── 000001_drop_<table>.sql
│
├── tests/
│   ├── unit/
│   │   ├── repository/
│   │   ├── service/
│   │   └── <pkg>/
│   └── integration/
│       ├── main_test.go
│       ├── helpers_test.go
│       └── *.go
│
├── fixtures/
│   └── <entity>_fixtures.go             # Factories de données de test
│
├── scripts/
│   ├── proto/generate_proto.sh
│   └── run_migration.sh
│
├── deployments/
│   ├── Dockerfile
│   ├── .dockerignore
│   └── helm/
│       ├── Chart.yaml
│       ├── templates/
│       └── values/
│           ├── prod.yaml
│           ├── staging.yaml
│           ├── vps-dev.yaml
│           └── local.yaml
│
├── go.mod / go.sum
└── Makefile
```

---

## 2. Architecture en couches

```
┌──────────────────────────────────────────────────────┐
│  TRANSPORT — grpc/handler.go                          │
│  Reçoit les requêtes gRPC, mappe vers le service      │
└────────────────────┬─────────────────────────────────┘
                     │
┌────────────────────▼─────────────────────────────────┐
│  MIDDLEWARE — middleware/interceptor.go               │
│  Cross-cutting: auth JWT, logging, recovery           │
└────────────────────┬─────────────────────────────────┘
                     │
┌────────────────────▼─────────────────────────────────┐
│  SERVICE — service/<entity>_service_impl.go           │
│  Business logic pure, orchestre repos + clients       │
└────────────────────┬─────────────────────────────────┘
                     │
┌────────────────────▼─────────────────────────────────┐
│  REPOSITORY — repository/implementations/             │
│  Accès données : PostgreSQL (pgx), Redis, etc.        │
└────────────────────┬─────────────────────────────────┘
                     │
┌────────────────────▼─────────────────────────────────┐
│  DOMAIN — domain/<entity>.go                          │
│  Entités, valeurs, méthodes purement métier (no I/O)  │
└──────────────────────────────────────────────────────┘
```

**Règle d'import** : chaque couche n'importe que la couche immédiatement en dessous.
Jamais de dépendance circulaire.

---

## 3. Domain Layer

### Responsabilités
- Définit les entités métier (`struct`)
- Contient la logique business **sans I/O** (pas de DB, pas de réseau)
- Méthodes de vérification d'état, transformation, anonymisation

### Exemple — `domain/auth.go`

```go
type Auth struct {
    AuthID            string
    FirebaseID        string
    Email             *string        // Nullable
    PhoneNumber       *string        // Nullable
    IsActive          bool
    IsSuspended       bool
    SuspensionEndDate *time.Time
    CreatedAt         time.Time
    UpdatedAt         time.Time
    DeletedAt         *time.Time     // Soft delete
}

// Méthodes métier pures
func (a *Auth) CanLogin() bool          { ... }
func (a *Auth) IsSuspendedNow() bool    { ... }
func (a *Auth) AnonymizeAndDelete()     { ... }  // GDPR
func (a *Auth) Suspend(endDate time.Time) { ... }
```

### Conventions
- **Soft delete** : `DeletedAt *time.Time` nullable
- **Nullables** : pointeurs (`*string`) pour champs optionnels
- **Timestamps** : `time.Now().UTC()` systématiquement
- **GDPR** : `AnonymizeAndDelete()` efface les PII avant soft-delete

---

## 4. Repository Layer

### Pattern Read / Write séparés

Chaque entité a **deux interfaces** :

```go
// repository/interfaces/auth_repository_read.go
type AuthRepositoryRead interface {
    GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error)
    GetByAuthID(ctx context.Context, authID string) (*domain.Auth, error)
    GetByEmail(ctx context.Context, email string) (*domain.Auth, error)
    GetByPhoneNumber(ctx context.Context, phoneNumber string) (*domain.Auth, error)
    EmailExists(ctx context.Context, email string) (bool, error)
    PhoneNumberExists(ctx context.Context, phoneNumber string) (bool, error)
}

// repository/interfaces/auth_repository_write.go
type AuthRepositoryWrite interface {
    Create(ctx context.Context, auth *domain.Auth) (string, error)
    Update(ctx context.Context, auth *domain.Auth) (*domain.Auth, error)
    Delete(ctx context.Context, auth *domain.Auth) error
}
```

### Gestion des erreurs repository

```go
// Mapper les erreurs pgx vers les sentinel errors du service
if errors.Is(err, pgx.ErrNoRows) {
    return nil, authErrors.ErrorUserNotFound
}
return nil, authErrors.ErrorInternalServer
```

### Conventions SQL
- `deleted_at IS NULL` dans **toutes** les requêtes de lecture
- `RETURNING` pour récupérer l'état après insert/update
- Indexes sur les colonnes fréquemment recherchées
- Trigger `update_updated_at_column()` pour `updated_at`

---

## 5. Service Layer

### Responsabilités
- Logique métier pure (validation, orchestration)
- Coordonne repository + clients externes
- Ne connaît pas gRPC, pas SQL

### Conventions

```go
// 1. Toujours valider les inputs
if firebaseID == "" {
    return nil, authErrors.ErrorUserNotFound
}

// 2. Récupérer le Firebase UID depuis le contexte (injecté par middleware)
firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
if !ok || firebaseID == "" {
    return nil, authErrors.ErrorInternalServer
}

// 3. Propager les erreurs telles quelles (pas de wrapping)
auth, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
if err != nil {
    return nil, err
}

// 4. Utiliser les méthodes métier du domain
if !auth.CanLogin() {
    return nil, authErrors.ErrorInternalServer
}
```

### Interface service (contrat)

```go
// service/interfaces/auth_service.go
type AuthService interface {
    GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error)
    RegisterUser(ctx context.Context, name, firstName, email, phoneNumber, profilePhotoURL string) (*domain.UserPreview, error)
    LoginUser(ctx context.Context) (*domain.UserPreview, error)
    CheckEmail(ctx context.Context, email string) (bool, error)
    CheckPhoneNumber(ctx context.Context, phoneNumber string) (bool, error)
    DeleteUserAccount(ctx context.Context, firebaseID string) error
}
```

---

## 6. Middleware (gRPC Interceptors)

### Rôle de l'intercepteur

L'intercepteur gRPC est la **première couche applicative** après Kong.
Il valide le JWT Firebase et injecte le Firebase UID dans le contexte.

```go
// middleware/interceptor.go

// La clé de contexte — définie ici car c'est ici qu'elle est SET
type contextKey string
const FirebaseIDKey contextKey = "firebaseID"

// Routes nécessitant un JWT valide
var protectedMethods = map[string]bool{
    "/auth.AuthService/Login":         true,
    "/auth.AuthService/CreateAccount": true,
    "/auth.AuthService/DeleteAccount": true,
}

func AuthInterceptor(validator *firebase.JWTValidator) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        // 1. Skip si route publique
        if !protectedMethods[info.FullMethod] {
            return handler(ctx, req)
        }
        // 2. Extraire Bearer token du metadata gRPC
        // 3. Appeler Firebase Admin SDK → validation complète
        // 4. context.WithValue(ctx, FirebaseIDKey, firebaseID)
        // 5. Appeler handler avec ctx enrichi
    }
}
```

### Clé de contexte

- Définie dans le package `middleware` (là où elle est **settée**)
- Lue dans le package `service` via `middleware.FirebaseIDKey`
- Type opaque `contextKey` — empêche les collisions entre packages

### Headers gRPC

Kong transmet les headers HTTP comme metadata gRPC (lowercase) :
- `Authorization: Bearer <token>` → metadata key `authorization`
- `X-Kong-JWT-Validated: true` → metadata key `x-kong-jwt-validated`

---

## 7. Firebase JWT Validator

```
pkg/firebase/jwt_validator.go
```

Utilise **Firebase Admin SDK** (`firebase.google.com/go/v4`) :

```go
type JWTValidator struct {
    client *firebaseAuth.Client
}

// Initialise avec Application Default Credentials (ADC)
// En GKE : Workload Identity automatique
// En local : GOOGLE_APPLICATION_CREDENTIALS vers service account JSON
func NewJWTValidator(ctx context.Context, projectID string) (*JWTValidator, error)

// Valide : signature RS256, issuer, audience, expiration
// Retourne le Firebase UID (claim "sub")
func (v *JWTValidator) VerifyToken(ctx context.Context, idToken string) (string, error)
```

---

## 8. Configuration d'un service

```go
// internal/config/config.go
type Config struct {
    Server      ServerConfig       // GRPC_ADDRESS, GRPC_PORT
    Environment EnvironmentConfig  // ENVIRONMENT
    Database    DatabaseConfig     // DATABASE_URL
    Redis       RedisConfig        // REDIS_URL
    UserService UserServiceConfig  // USER_SERVICE_HOST, USER_SERVICE_PORT
    Firebase    FirebaseConfig     // FIREBASE_PROJECT_ID
    LogLevel    string             // LOG_LEVEL
}
```

**Convention** : toutes les variables env sont chargées via `pkg/config` (Viper).
- `MustGetString(values, "KEY")` → panique si absent
- `GetStringOrDefault(values, "KEY", "default")` → fallback silencieux

---

## 9. Errors

### Structure

```
services/<service>/pkg/errors/errors.go  # Sentinel errors du service
pkg/errors/errors.go                     # Errors partagées (si besoin)
```

### Exemple auth-service

```go
var (
    ErrorInternalServer          = errors.New("ErrorInternalServer")
    ErrorUserNotFound            = errors.New("ErrorUserNotFound")
    ErrorEmailNotAvailable       = errors.New("ErrorEmailNotAvailable")
    ErrorPhoneNumberNotAvailable = errors.New("ErrorPhoneNumberNotAvailable")
    ErrorDataRetrievalFailed     = errors.New("ErrorDataRetrievalFailed")
    ErrorCantDeleteAccount       = errors.New("ErrorCantDeleteAccount")
)
```

### Mapping gRPC Status Codes (dans le handler)

| Sentinel Error | gRPC Code | HTTP |
|---------------|-----------|------|
| `ErrorUserNotFound` | `codes.NotFound` (5) | 404 |
| `ErrorInternalServer` | `codes.Internal` (13) | 500 |
| `ErrorEmailNotAvailable` | `codes.FailedPrecondition` (9) | 412 |
| `ErrorPhoneNumberNotAvailable` | `codes.FailedPrecondition` (9) | 412 |
| `ErrorDataRetrievalFailed` | `codes.Internal` (13) | 500 |
| `ErrorCantDeleteAccount` | `codes.Internal` (13) | 500 |

---

## 10. Proto — Contrat gRPC

### Structure d'un proto

```protobuf
syntax = "proto3";
package <service>;
option go_package = "github.com/Kpeewu/tissi-mah/services/<service>/proto/gen;<package>";

import "google/api/annotations.proto";  // Pour HTTP mapping Kong

service <Service>Service {
  rpc <Method>(<Request>) returns (<Response>) {
    option (google.api.http) = {
      post: "/api/v1/<service>/<method>"
      body: "*"
    };
  }
  rpc Health(HealthRequest) returns (HealthResponse) {
    option (google.api.http) = { get: "/api/v1/<service>/health" };
  }
}
```

### Génération

```bash
make proto           # Tous les services
make proto-auth      # auth-service uniquement
make proto-sync      # Copie vers Helm configmaps
```

Script : `services/<service>/scripts/proto/generate_proto.sh`

---

## 11. Database Schema — Conventions

```sql
-- Colonnes systématiques
auth_id     VARCHAR(128) PRIMARY KEY,   -- UUID Go (google/uuid)
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
deleted_at  TIMESTAMPTZ,                -- NULL = actif, soft delete

-- Trigger automatique updated_at
CREATE TRIGGER update_<table>_updated_at BEFORE UPDATE ON <table>
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Index soft-delete
CREATE INDEX idx_<table>_deleted_at ON <table>(deleted_at) WHERE deleted_at IS NOT NULL;
```

**Règle** : toutes les requêtes de lecture filtrent sur `WHERE deleted_at IS NULL`

---

## 12. Tests

### Structure

```
tests/
├── unit/
│   ├── repository/     # Tests unitaires avec mock DB ou testcontainers
│   ├── service/        # Tests avec mocks des interfaces repository
│   └── <pkg>/          # Tests des packages pkg/
└── integration/
    ├── main_test.go    # TestMain — setup/teardown de l'environnement
    ├── helpers_test.go # Helpers partagés (connexion DB, fixtures)
    └── *.go            # Tests end-to-end gRPC

fixtures/
└── <entity>_fixtures.go   # Factory functions pour les données de test
```

### pkg-test (partagé)

```go
// pkg-test/postgres/test_containers.go
// Lance un conteneur PostgreSQL temporaire via testcontainers
```

### Conventions de test
- `TestMain` dans `integration/main_test.go` pour setup/teardown
- Factories de fixtures dans `fixtures/` pour données réutilisables
- Tests unitaires du service layer avec **interfaces mockées** (pas de DB)

---

## 13. Client inter-services

```go
// internal/client/user_service_client.go
type UserServiceClient struct {
    // connexion gRPC vers user-service
}

func (c *UserServiceClient) CreateUser(ctx context.Context, authID, name, firstName, email, phoneNumber, profilePhotoURL string) (*domain.UserPreview, error) {
    // TODO: appel gRPC vers user-service
}

func (c *UserServiceClient) GetUserByAuthID(ctx context.Context, authID string) (*domain.UserPreview, error) {
    // TODO: appel gRPC vers user-service
}
```

**Convention** : les clients sont des **stubs** jusqu'à ce que le service cible soit implémenté.
La connexion gRPC est initialisée dans `main.go` avec l'adresse depuis la config (`UserService.Address:UserService.Port`).

---

## 14. Dockerfile (multi-stage)

```dockerfile
# Stage 1 : Build
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY services/<service>/ ./services/<service>/
RUN go build -o /bin/server ./services/<service>/cmd/server

# Stage 2 : Runtime minimal
FROM alpine:latest
RUN adduser -D -u 1000 appuser
USER appuser
COPY --from=builder /bin/server /bin/server
ENTRYPOINT ["/bin/server"]
```

---

## 15. Ajouter un nouveau service

1. **Créer la structure** : copier le squelette depuis `auth-service`
2. **go.mod** : module `github.com/Kpeewu/tissi-mah/services/<name>`
3. **go.work** : ajouter `use ./services/<name>`
4. **Proto** : définir le contrat dans `proto/<name>.proto`
5. **Domain** : entités métier dans `internal/domain/`
6. **Repository** : interfaces + implémentations
7. **Service** : interface + implémentation
8. **Middleware** : réutiliser `AuthInterceptor` si JWT nécessaire
9. **gRPC handler** : implémenter l'interface proto générée
10. **Config** : ajouter les variables dans `internal/config/config.go`
11. **Migrations** : SQL dans `migrations/up/` et `migrations/down/`
12. **Helm chart** : `deployments/helm/` avec values par env
13. **Kong routes** : ajouter dans `infrastructure/helm-charts/kong/templates/routes/`
14. **docker-compose.yml** : ajouter la base de données dev si nouvelle
15. **Makefile** : ajouter `run-<service>`, `test-<service>`, `migrate-<service>`
