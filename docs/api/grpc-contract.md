# gRPC Contracts

This document describes the gRPC contracts used for inter-service communication in Tissi-Mah. Each service exposes a gRPC API that other services can consume.

## Table of Contents

1. [Overview](#overview)
2. [Common Types](#common-types)
3. [Auth Service](#auth-service)
4. [User Service](#user-service)
5. [Vehicle Service](#vehicle-service)
6. [Trips Service](#trips-service)
7. [Booking Service](#booking-service)
8. [Payment Service](#payment-service)
9. [Rating Service](#rating-service)
10. [File Service](#file-service)
11. [Notification Service](#notification-service)
12. [Stats Service](#stats-service)
13. [Geolocation Service](#geolocation-service)

---

## Overview

### Communication Pattern

All inter-service communication uses **gRPC** over HTTP/2 with Protocol Buffers for serialization.

```
┌─────────────┐       gRPC        ┌─────────────┐
│  Service A  │ ────────────────► │  Service B  │
│             │ ◄──────────────── │             │
└─────────────┘    (protobuf)     └─────────────┘
```

### Service Discovery

Services discover each other via Kubernetes DNS:

```
<service-name>.tissi-mah.svc.cluster.local:<port>
```

### Service Ports

| Service | Port |
|---------|------|
| auth-service | 50051 |
| user-service | 50052 |
| vehicle-service | 50053 |
| trips-service | 50054 |
| booking-service | 50055 |
| payment-service | 50056 |
| rating-service | 50057 |
| file-service | 50058 |
| notification-service | 50059 |
| stats-service | 50060 |
| geolocation-service | 50061 |

### Proto File Organization

```
services/
├── auth-service/
│   └── proto/
│       ├── auth.proto
│       └── gen/
│           ├── auth.pb.go
│           └── auth_grpc.pb.go
├── user-service/
│   └── proto/
│       ├── user.proto
│       └── gen/
└── ... (other services)
```

---

## Common Types

Common types shared across multiple services.


### Error Handling

gRPC errors are returned using standard gRPC status codes with the error message in the status description:

```go
// Server side
return nil, status.Error(codes.NotFound, "ErrUserNotFound")

// Client side
if st, ok := status.FromError(err); ok {
    switch st.Code() {
    case codes.NotFound:
        // Handle not found
    }
    errorMessage := st.Message() // "ErrUserNotFound"
}
```

---

## Auth Service

**Address:** `auth-service.tissi-mah.svc.cluster.local:50051`

### Proto Definition

```protobuf
syntax = "proto3";

package tissimah.auth.v1;

option go_package = "github.com/tissi-mah/auth-service/proto/gen;authpb";

import "google/protobuf/timestamp.proto";

service AuthService {
    // Check if account exists and return user profile
    rpc Login(LoginRequest) returns (LoginResponse);
    
    // Create new account
    rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse);
    
    // Check if phone number is available
    rpc CheckPhoneNumber(CheckPhoneNumberRequest) returns (CheckPhoneNumberResponse);
    
    // Check if email is available
    rpc CheckEmail(CheckEmailRequest) returns (CheckEmailResponse);
    
    // Delete account (GDPR)
    rpc DeleteAccount(DeleteAccountRequest) returns (DeleteAccountResponse);
    
    // Internal: Check if account exists by Firebase ID
    rpc AccountExists(AccountExistsRequest) returns (AccountExistsResponse);
    
    // Internal: Get auth record by Firebase ID
    rpc GetAuthByFirebaseID(GetAuthByFirebaseIDRequest) returns (GetAuthByFirebaseIDResponse);
    
    // Internal: Deactivate account
    rpc DeactivateAccount(DeactivateAccountRequest) returns (DeactivateAccountResponse);
}
```

### Messages

#### Login

```protobuf
message LoginRequest {
    // Empty - Firebase ID is extracted from gRPC metadata
}

message LoginResponse {
    bool exists = 1;
    UserProfile user = 2;  // null if exists = false
}

message UserProfile {
    string profile_id = 1;
    string name = 2;
    string first_name = 3;
    string email = 4;
    string phone_number = 5;
    string profile_image_url = 6;
}
```

#### CreateAccount

```protobuf
message CreateAccountRequest {
    string name = 1;
    string first_name = 2;
    string email = 3;           // optional
    string phone_number = 4;    // optional
    string profile_image_url = 5; // optional
}

message CreateAccountResponse {
    UserProfile user = 1;
}
```

#### CheckPhoneNumber

```protobuf
message CheckPhoneNumberRequest {
    string phone_number = 1;
}

message CheckPhoneNumberResponse {
    bool is_available = 1;
}
```

#### CheckEmail

```protobuf
message CheckEmailRequest {
    string email = 1;
}

message CheckEmailResponse {
    bool is_available = 1;
}
```

#### DeleteAccount

```protobuf
message DeleteAccountRequest {
    // Empty - Firebase ID is extracted from gRPC metadata
}

message DeleteAccountResponse {
    bool success = 1;
}
```

#### Internal: AccountExists

Used by other services to check if an account exists.

```protobuf
message AccountExistsRequest {
    string firebase_id = 1;
}

message AccountExistsResponse {
    bool exists = 1;
    string auth_id = 2;     // Only set if exists = true
    bool is_active = 3;
}
```

#### Internal: GetAuthByFirebaseID

Used by other services to get full auth record.

```protobuf
message GetAuthByFirebaseIDRequest {
    string firebase_id = 1;
}

message GetAuthByFirebaseIDResponse {
    Auth auth = 1;
}

message Auth {
    string auth_id = 1;
    string firebase_id = 2;
    string email = 3;
    string phone = 4;
    string provider = 5;
    bool is_active = 6;
    google.protobuf.Timestamp suspension_end_date = 7;
    google.protobuf.Timestamp created_at = 8;
    google.protobuf.Timestamp updated_at = 9;
}
```

#### Internal: DeactivateAccount

Used during GDPR deletion process.

```protobuf
message DeactivateAccountRequest {
    string auth_id = 1;
}

message DeactivateAccountResponse {
    bool success = 1;
}
```

### Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrMissingFirebaseID` | `UNAUTHENTICATED` | Firebase ID not in metadata |
| `ErrAccountNotFound` | `NOT_FOUND` | Account does not exist |
| `ErrAccountAlreadyExists` | `FAILED_PRECONDITION` | Account already exists |
| `ErrAccountSuspended` | `UNAUTHENTICATED` | Account is suspended |
| `ErrAccountDeactivated` | `UNAUTHENTICATED` | Account is deactivated |
| `ErrPhoneNumberTaken` | `FAILED_PRECONDITION` | Phone number in use |
| `ErrEmailTaken` | `FAILED_PRECONDITION` | Email in use |
| `ErrInvalidPhoneNumber` | `INVALID_ARGUMENT` | Invalid phone format |
| `ErrInvalidEmail` | `INVALID_ARGUMENT` | Invalid email format |
| `ErrActiveBookingsExist` | `FAILED_PRECONDITION` | Cannot delete with active bookings |
| `ErrActiveTripsExist` | `FAILED_PRECONDITION` | Cannot delete with upcoming trips |
| `ErrPendingPayoutsExist` | `FAILED_PRECONDITION` | Cannot delete with pending payouts |

### Client Usage Example (Go)

```go
package client

import (
    "context"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    authpb "github.com/tissi-mah/auth-service/proto/gen"
)

type AuthClient struct {
    client authpb.AuthServiceClient
    conn   *grpc.ClientConn
}

func NewAuthClient(address string) (*AuthClient, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    conn, err := grpc.DialContext(ctx, address,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
    )
    if err != nil {
        return nil, err
    }

    return &AuthClient{
        client: authpb.NewAuthServiceClient(conn),
        conn:   conn,
    }, nil
}

func (c *AuthClient) AccountExists(ctx context.Context, firebaseID string) (bool, string, error) {
    resp, err := c.client.AccountExists(ctx, &authpb.AccountExistsRequest{
        FirebaseId: firebaseID,
    })
    if err != nil {
        return false, "", err
    }
    return resp.Exists, resp.AuthId, nil
}

func (c *AuthClient) DeactivateAccount(ctx context.Context, authID string) error {
    _, err := c.client.DeactivateAccount(ctx, &authpb.DeactivateAccountRequest{
        AuthId: authID,
    })
    return err
}

func (c *AuthClient) Close() error {
    return c.conn.Close()
}
```

### Metadata

Auth service reads the following metadata from incoming requests:

| Key | Source | Description |
|-----|--------|-------------|
| `x-firebase-id` | Kong | Firebase UID extracted from JWT |
| `x-request-id` | Kong/Client | Request tracing ID |

---

## User Service

**Address:** `user-service.tissi-mah.svc.cluster.local:50052`

> Documentation to be added.

---

## Vehicle Service

**Address:** `vehicle-service.tissi-mah.svc.cluster.local:50053`

> Documentation to be added.

---

## Trips Service

**Address:** `trips-service.tissi-mah.svc.cluster.local:50054`

> Documentation to be added.

---

## Booking Service

**Address:** `booking-service.tissi-mah.svc.cluster.local:50055`

> Documentation to be added.

---

## Payment Service

**Address:** `payment-service.tissi-mah.svc.cluster.local:50056`

> Documentation to be added.

---

## Rating Service

**Address:** `rating-service.tissi-mah.svc.cluster.local:50057`

> Documentation to be added.

---

## File Service

**Address:** `file-service.tissi-mah.svc.cluster.local:50058`

> Documentation to be added.

---

## Notification Service

**Address:** `notification-service.tissi-mah.svc.cluster.local:50059`

> Documentation to be added.

---

## Stats Service

**Address:** `stats-service.tissi-mah.svc.cluster.local:50060`

> Documentation to be added.

---

## Geolocation Service

**Address:** `geolocation-service.tissi-mah.svc.cluster.local:50061`

> Documentation to be added.

---

## Proto Generation

### Generate Go Code

Each service has a script to generate Go code from proto files:

```bash
# From service directory
./scripts/generate_proto.sh

# Or using Make
make proto
```

### Script Example

```bash
#!/bin/bash
# scripts/generate_proto.sh

PROTO_DIR="./proto"
GEN_DIR="./proto/gen"

mkdir -p $GEN_DIR

protoc \
    --proto_path=$PROTO_DIR \
    --go_out=$GEN_DIR \
    --go_opt=paths=source_relative \
    --go-grpc_out=$GEN_DIR \
    --go-grpc_opt=paths=source_relative \
    $PROTO_DIR/*.proto

echo "Proto files generated in $GEN_DIR"
```

### Required Tools

```bash
# Install protoc compiler
# macOS
brew install protobuf

# Ubuntu
apt-get install -y protobuf-compiler

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```