# Vehicle Service API

This document describes the HTTP/REST API exposed by the vehicle-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://dev.tissi-mah.com` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

Vehicle-service endpoints are currently **public** (no Firebase JWT required).
`UserId` is provided in the request body for create, update, and delete operations.

> **Note:** When Firebase JWT is fully integrated, protected endpoints will require a valid token and `UserId` will be extracted from the JWT instead of the body.

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes (POST/PATCH) | `application/json` |
| `Accept` | No | `application/json` |
| `X-Request-ID` | No | Client-generated UUID for request tracing |

## Error Response Format

All errors follow this format:

```json
{
    "ErrorMessage": "ErrorVehicleNotFound"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier (matches backend sentinel error name) |

---

## Endpoints

### POST /api/v1/vehicle/add

Creates a new vehicle for a user.

**Authentication:** Not required (public)

#### Request

```http
POST /api/v1/vehicle/add HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "UserId": "firebase-uid-abc123",
    "Brand": "Toyota",
    "NumberOfSeats": 5,
    "BrandModel": "Corolla",
    "Color": "Blanc",
    "LicencePlate": "TG-1234-AB"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserId` | string | Yes | Firebase UID of the vehicle owner |
| `Brand` | string | Yes | Vehicle brand (e.g. Toyota, Honda) |
| `NumberOfSeats` | integer | Yes | Number of available passenger seats |
| `BrandModel` | string | Yes | Vehicle model (e.g. Corolla, Civic) |
| `Color` | string | Yes | Vehicle color |
| `LicencePlate` | string | Yes | Vehicle licence plate (must be unique) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `VehicleId` | string | Unique ID of the created vehicle |
| `ErrorMessage` | string | Error message if failed, empty if success |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing or invalid field |
| `ErrorLicencePlateConflict` | 409 | Licence plate already registered |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/vehicle/add \
  -H "Content-Type: application/json" \
  -d '{
    "UserId": "firebase-uid-abc123",
    "Brand": "Toyota",
    "NumberOfSeats": 5,
    "BrandModel": "Corolla",
    "Color": "Blanc",
    "LicencePlate": "TG-1234-AB"
  }'
```

---

### PATCH /api/v1/vehicle/update

Updates the mutable fields of a vehicle (color and/or licence plate). Only the owner can update their vehicle.

**Authentication:** Not required (public, UserId in body)

#### Request

```http
PATCH /api/v1/vehicle/update HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "UserId": "firebase-uid-abc123",
    "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
    "Color": "Gris",
    "LicencePlate": "TG-5678-CD"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserId` | string | Yes | Firebase UID of the vehicle owner |
| `VehicleId` | string | Yes | ID of the vehicle to update |
| `Color` | string | No | New vehicle color |
| `LicencePlate` | string | No | New licence plate (must be unique) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Success": true,
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Success` | boolean | `true` if update was applied |
| `ErrorMessage` | string | Error message if failed, empty if success |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing or invalid field |
| `ErrorVehicleNotFound` | 404 | Vehicle does not exist |
| `ErrorUnauthorized` | 403 | UserId does not own this vehicle |
| `ErrorLicencePlateConflict` | 409 | New licence plate already registered |

#### Example (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/vehicle/update \
  -H "Content-Type: application/json" \
  -d '{
    "UserId": "firebase-uid-abc123",
    "VehicleId": "v-550e8400",
    "Color": "Gris"
  }'
```

---

### POST /api/v1/vehicle/delete

Deletes a vehicle. Only the owner can delete their vehicle.

**Authentication:** Not required (public, UserId in body)

#### Request

```http
POST /api/v1/vehicle/delete HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "UserId": "firebase-uid-abc123",
    "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserId` | string | Yes | Firebase UID of the vehicle owner |
| `VehicleId` | string | Yes | ID of the vehicle to delete |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Success": true,
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Success` | boolean | `true` if vehicle was deleted |
| `ErrorMessage` | string | Error message if failed, empty if success |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorVehicleNotFound` | 404 | Vehicle does not exist |
| `ErrorUnauthorized` | 403 | UserId does not own this vehicle |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/vehicle/delete \
  -H "Content-Type: application/json" \
  -d '{
    "UserId": "firebase-uid-abc123",
    "VehicleId": "v-550e8400"
  }'
```

---

### POST /api/v1/vehicle/details

Retrieves full details of a vehicle, including documents fetched from file-service (assurance and registration card URLs).

**Authentication:** Not required (public)

#### Request

```http
POST /api/v1/vehicle/details HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "UserId": "firebase-uid-abc123",
    "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserId` | string | Yes | Firebase UID of the requester |
| `VehicleId` | string | Yes | ID of the vehicle |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Vehicle": {
        "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
        "UserId": "firebase-uid-abc123",
        "Brand": "Toyota",
        "NumberOfSeats": 5,
        "BrandModel": "Corolla",
        "Color": "Blanc",
        "LicencePlate": "TG-1234-AB",
        "IsVerified": false,
        "Documents": {
            "AssuranceUrl": "https://storage.tissi-mah.com/files/assurance-abc123.pdf",
            "VehicleRegistrationUrl": "https://storage.tissi-mah.com/files/carte-grise-abc123.pdf"
        }
    },
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Vehicle` | object | Full vehicle details |
| `Vehicle.VehicleId` | string | Unique vehicle ID |
| `Vehicle.UserId` | string | Firebase UID of the owner |
| `Vehicle.Brand` | string | Vehicle brand |
| `Vehicle.NumberOfSeats` | integer | Number of passenger seats |
| `Vehicle.BrandModel` | string | Vehicle model |
| `Vehicle.Color` | string | Vehicle color |
| `Vehicle.LicencePlate` | string | Licence plate |
| `Vehicle.IsVerified` | boolean | Whether the vehicle has been verified by an admin |
| `Vehicle.Documents.AssuranceUrl` | string | URL of the assurance document (empty if not uploaded) |
| `Vehicle.Documents.VehicleRegistrationUrl` | string | URL of the registration card (empty if not uploaded) |
| `ErrorMessage` | string | Error message if failed, empty if success |

> **Note:** `Documents` are fetched from file-service. If file-service is unavailable, document URLs will be empty strings — the vehicle data is still returned.

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorVehicleNotFound` | 404 | Vehicle does not exist |
| `ErrorUnauthorized` | 403 | UserId does not own this vehicle |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/vehicle/details \
  -H "Content-Type: application/json" \
  -d '{
    "UserId": "firebase-uid-abc123",
    "VehicleId": "v-550e8400"
  }'
```

---

### POST /api/v1/vehicle/getUserVehicles

Retrieves the list of all vehicles belonging to a user.

**Authentication:** Not required (public)

#### Request

```http
POST /api/v1/vehicle/getUserVehicles HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "UserId": "firebase-uid-abc123"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserId` | string | Yes | Firebase UID of the user |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Vehicles": [
        {
            "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
            "Brand": "Toyota",
            "BrandModel": "Corolla",
            "LicencePlate": "TG-1234-AB",
            "IsVerified": false
        },
        {
            "VehicleId": "v-660f9511-e29b-41d4-a716-446655440001",
            "Brand": "Honda",
            "BrandModel": "Civic",
            "LicencePlate": "TG-5678-CD",
            "IsVerified": true
        }
    ],
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Vehicles` | array | List of vehicle previews (empty array if none) |
| `Vehicles[].VehicleId` | string | Unique vehicle ID |
| `Vehicles[].Brand` | string | Vehicle brand |
| `Vehicles[].BrandModel` | string | Vehicle model |
| `Vehicles[].LicencePlate` | string | Licence plate |
| `Vehicles[].IsVerified` | boolean | Whether the vehicle has been verified |
| `ErrorMessage` | string | Error message if failed, empty if success |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing UserId |
| `ErrorDataRetrievalFailed` | 500 | Database query failed |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/vehicle/getUserVehicles \
  -H "Content-Type: application/json" \
  -d '{"UserId": "firebase-uid-abc123"}'
```

---

### GET /api/v1/vehicle/health

Health check endpoint for the vehicle-service.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/vehicle/health HTTP/1.1
Host: api.tissi-mah.com
```

#### Response

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Status": "SERVING",
    "Version": "v1.0.0",
    "Timestamp": 1709136000
}
```

#### Example (cURL)

```bash
curl https://api.tissi-mah.com/api/v1/vehicle/health
```

---

## Business Rules

| Rule | Description |
|------|-------------|
| **Unique licence plate** | Each licence plate can only be registered once across the platform |
| **Owner-only modification** | Only the vehicle owner (matching `UserId`) can update or delete their vehicle |
| **Verification by admin** | `IsVerified` is set to `false` on creation and can only be changed by an admin |
| **Fault-tolerant documents** | Document URLs are populated from file-service; if unavailable, empty strings are returned without error |

---

## Error Reference

| Error | HTTP | gRPC Code | Description | User Action |
|-------|------|-----------|-------------|-------------|
| `ErrorVehicleNotFound` | 404 | NOT_FOUND (5) | Vehicle does not exist | Check vehicle ID |
| `ErrorUnauthorized` | 403 | PERMISSION_DENIED (7) | UserId does not own this vehicle | Use the correct UserId |
| `ErrorLicencePlateConflict` | 409 | ALREADY_EXISTS (6) | Licence plate already registered | Use a different licence plate |
| `ErrorInvalidInput` | 400 | INVALID_ARGUMENT (3) | Missing or invalid request field | Fix the request body |
| `ErrorDataRetrievalFailed` | 500 | INTERNAL (13) | Database query failed | Retry later |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Unexpected server error | Retry later |
