# Auth Service

This document describes the architecture of the auth-service.

## Table of Contents

1. [Overview](#overview)
2. [Directory Structure](#directory-structure)
3. [Layer Responsibilities](#layer-responsibilities)
4. [Request Flow](#request-flow)
5. [Service Dependencies](#service-dependencies)

---

## Overview

### Purpose

The auth-service is responsible for managing user authentication state and account lifecycle. It does **not** validate Firebase tokens directly — that responsibility belongs to Kong API Gateway.

### Responsibilities

| Responsibility | Description |
|----------------|-------------|
| Account existence check | Verify if a Firebase ID has a corresponding account |
| Account creation | Create auth record and trigger user profile creation |
| Availability checks | Verify if phone/email are available for registration |
| Account deletion | Anonymize user data in compliance with GDPR |

### What auth-service does NOT do

| Not Responsible | Handled By |
|-----------------|------------|
| Firebase token validation | Kong (jwt-firebase plugin) |
| User profile management | user-service |
| Session/token storage | Firebase (client-side) |
| Logout | Mobile app (delete token locally) |

---

## Directory Structure

The auth-service follows the standard Go project layout with a layered architecture.

```
services/auth-service/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
├── internal/
│   ├── client/
│   │   └── user_service_client.go  # gRPC client for user-service
│   ├── config/
│   │   └── config.go               # Configuration loading
│   ├── domain/
│   │   └── auth.go                 # Domain models and business entities
│   ├── grpc/
│   │   ├── handler.go              # gRPC request handlers
│   │   └── server.go               # gRPC server setup
│   ├── middleware/
│   │   └── interceptor.go          # gRPC interceptors (auth, logging)
│   ├── repository/
│   │   └── auth_repository.go      # Database access layer
│   └── service/
│       └── auth_service.go         # Business logic layer
├── pkg/
│   ├── errors/
│   │   └── errors.go               # Custom error definitions
│   └── firebase/
│       └── jwt_validator.go        # Firebase JWT utilities
├── proto/
│   ├── auth.proto                  # Protocol Buffers definitions
│   └── gen/
│       ├── auth.pb.go              # Generated protobuf code
│       └── auth_grpc.pb.go         # Generated gRPC code
├── migrations/
│   ├── 000001_create_auth_table.up.sql
│   └── 000001_create_auth_table.down.sql
├── tests/
│   ├── integration/
│   │   ├── database_test.go        # Database integration tests
│   │   └── grpc_test.go            # gRPC endpoint tests
│   └── unit/
│       ├── repository/
│       │   └── auth_repository_test.go
│       └── service/
│           └── auth_service_test.go
├── fixtures/
│   ├── auth_fixtures.go            # Test fixtures and factories
│   └── test_data.json              # Test data
├── deployments/
│   ├── Dockerfile                  # Container image definition
│   └── helm/
│       ├── Chart.yaml              # Helm chart metadata
│       ├── templates/
│       │   ├── _helpers.tpl        # Template helpers
│       │   ├── configmap.yaml      # ConfigMap template
│       │   ├── deployment.yaml     # Deployment template
│       │   ├── hpa.yaml            # Horizontal Pod Autoscaler
│       │   ├── pdb.yaml            # Pod Disruption Budget
│       │   ├── secrets.yaml        # Secrets template
│       │   ├── service.yaml        # Service template
│       │   └── serviceaccount.yaml # ServiceAccount template
│       └── values/
│           ├── local.yaml          # Local dev values
│           ├── vps-dev.yaml        # VPS dev values
│           ├── staging.yaml        # Staging values
│           └── prod.yaml           # Production values
├── scripts/
│   ├── build.sh                    # Build script
│   ├── generate_proto.sh           # Proto generation script
│   ├── run_migration.sh            # Database migration script
│   └── run_tests.sh                # Test runner script
├── Makefile                        # Common commands
└── README.md                       # Service documentation
```

---

## Layer Responsibilities

### Overview

| Layer | Directory | Responsibility |
|-------|-----------|----------------|
| **Entry Point** | `cmd/server/` | Application bootstrap, dependency injection |
| **Handlers** | `internal/grpc/` | Request parsing, response formatting, input validation |
| **Middleware** | `internal/middleware/` | Cross-cutting concerns (auth, logging, tracing) |
| **Service** | `internal/service/` | Business logic, orchestration |
| **Repository** | `internal/repository/` | Database operations, queries |
| **Domain** | `internal/domain/` | Business entities, domain models |
| **Client** | `internal/client/` | gRPC clients for other services |
| **Config** | `internal/config/` | Configuration loading and validation |
| **Errors** | `pkg/errors/` | Custom error definitions |

### Layer Details

#### cmd/server/

Application entry point. Responsible for:
- Loading configuration
- Initializing database and Redis connections
- Creating service dependencies (dependency injection)
- Starting the gRPC server

#### internal/grpc/

Handles incoming gRPC requests:
- `server.go` — gRPC server setup and registration
- `handler.go` — Maps gRPC methods to service layer calls

#### internal/middleware/

gRPC interceptors for cross-cutting concerns:
- Extract Firebase ID from metadata
- Add request ID for tracing
- Logging
- Panic recovery

#### internal/service/

Business logic layer:
- Orchestrates calls to repositories and external clients
- Implements business rules
- Transaction management

#### internal/repository/

Data access layer:
- SQL queries
- Database operations
- No business logic

#### internal/domain/

Domain models:
- Structs representing business entities
- No dependencies on infrastructure

#### internal/client/

gRPC clients to communicate with other services:
- user-service client
- file-service client
- etc.

#### pkg/

Shared packages that could potentially be used by other services:
- Custom error definitions
- Utilities

---

## Request Flow

### Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Request Flow                                    │
└─────────────────────────────────────────────────────────────────────────────┘

  Mobile App
      │
      │ HTTPS + Firebase JWT
      ▼
┌──────────┐
│   Kong   │  ─── Validates JWT, extracts firebase_id
└────┬─────┘
     │ gRPC + metadata (x-firebase-id)
     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                            auth-service                                   │
│                                                                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌───────────┐ │
│  │ Interceptor │───►│   Handler   │───►│   Service   │───►│Repository │ │
│  │(middleware) │    │   (grpc)    │    │  (service)  │    │           │ │
│  └─────────────┘    └─────────────┘    └──────┬──────┘    └─────┬─────┘ │
│                                               │                  │       │
│                                               │                  ▼       │
│                                               │           ┌───────────┐  │
│                                               │           │PostgreSQL │  │
│                                               │           └───────────┘  │
│                                               │                          │
│                                               │           ┌───────────┐  │
│                                               │           │   Redis   │  │
│                                               │           │  (cache)  │  │
│                                               │           └───────────┘  │
│                                               │                          │
│                                               ▼                          │
│                                        ┌─────────────┐                   │
│                                        │   Client    │                   │
│                                        │(other svcs) │                   │
│                                        └──────┬──────┘                   │
│                                               │                          │
└───────────────────────────────────────────────┼──────────────────────────┘
                                                │ gRPC
                                                ▼
                                         ┌─────────────┐
                                         │user-service │
                                         │file-service │
                                         │    etc.     │
                                         └─────────────┘
```

### Step-by-Step

| Step | Layer | Action |
|------|-------|--------|
| 1 | Kong | Validates Firebase JWT, extracts `firebase_id`, passes in metadata |
| 2 | Interceptor | Reads `x-firebase-id` from metadata, adds to context |
| 3 | Handler | Parses request, validates input, calls service |
| 4 | Service | Executes business logic, calls repository and/or clients |
| 5 | Repository | Executes database queries |
| 6 | Client | Calls other services via gRPC (if needed) |
| 7 | Response | Flows back through layers to client |

---

## Service Dependencies

### Dependency Diagram

```
                              ┌─────────────┐
                              │ auth-service│
                              └──────┬──────┘
                                     │
        ┌────────────┬───────────┬───┴───┬───────────┬────────────┐
        │            │           │       │           │            │
        ▼            ▼           ▼       ▼           ▼            ▼
┌─────────────┐ ┌─────────┐ ┌───────┐ ┌───────┐ ┌────────┐ ┌──────────┐
│user-service │ │  file-  │ │vehicle│ │booking│ │  trip- │ │ payment- │
│             │ │ service │ │service│ │service│ │ service│ │  service │
└─────────────┘ └─────────┘ └───────┘ └───────┘ └────────┘ └──────────┘
                                                                │
                                                                ▼
                                                          ┌──────────┐
                                                          │  rating- │
                                                          │  service │
                                                          └──────────┘
```

### Dependency Details

| Dependency | Methods Used | Purpose |
|------------|--------------|---------|
| **user-service** | `CreateProfile()`, `GetProfile()`, `AnonymizeProfile()` | Profile management |
| **file-service** | `DeleteAllUserFiles()` | GDPR deletion |
| **vehicle-service** | `DeleteAllUserVehicles()` | GDPR deletion |
| **booking-service** | `HasActiveBookings()`, `AnonymizeUserBookings()` | GDPR deletion |
| **trip-service** | `HasUpcomingTrips()`, `AnonymizeUserTrips()` | GDPR deletion |
| **payment-service** | `HasPendingPayouts()`, `AnonymizeUserPayments()` | GDPR deletion |
| **rating-service** | `DeleteRatingsByAuthor()` | GDPR deletion |

### Infrastructure Dependencies

| Dependency | Purpose |
|------------|---------|
| **PostgreSQL** | Primary data storage for `auth` table |
| **Redis** | Cache for account existence checks |
| **Kong** | JWT validation, request routing |