# Dépendances Tissi-Mah

## Versions de référence

Toutes les services doivent utiliser ces versions pour garantir la compatibilité.

### Core Dependencies

| Package | Version | Usage |
|---------|---------|-------|
| `google.golang.org/grpc` | v1.60.1 | Communication gRPC |
| `google.golang.org/protobuf` | v1.32.0 | Protocol Buffers |
| `github.com/spf13/viper` | v1.18.2 | Configuration |
| `github.com/sirupsen/logrus` | v1.9.3 | Logging |
| `github.com/google/uuid` | v1.5.0 | UUID generation |

### Database Drivers

| Package | Version | Service(s) |
|---------|---------|------------|
| `github.com/lib/pq` | v1.10.9 | auth-service, client-service |
| `go.mongodb.org/mongo-driver` | v1.13.1 | user-service |

### Auth & Security

| Package | Version | Service(s) |
|---------|---------|------------|
| `firebase.google.com/go/v4` | v4.13.0 | auth-service |
| `google.golang.org/api` | v0.157.0 | auth-service (Firebase) |

### Utilities

| Package | Version | Usage |
|---------|---------|-------|
| `golang.org/x/net` | v0.20.0 | Context & HTTP utilities |

## Installation

### Auth Service
```bash
cd services/auth-service
go get google.golang.org/grpc@v1.60.1
go get google.golang.org/protobuf@v1.32.0
go get github.com/lib/pq@v1.10.9
go get github.com/spf13/viper@v1.18.2
go get github.com/sirupsen/logrus@v1.9.3
go get firebase.google.com/go/v4@v4.13.0
go get google.golang.org/api@v0.157.0
go get golang.org/x/net@v0.20.0
go get github.com/google/uuid@v1.5.0
```





## Mise à jour

Pour mettre à jour une dépendance :
1. Modifier la version dans ce fichier
2. Lancer `make update-deps SERVICE=<service-name>`
3. Tester
4. Commit