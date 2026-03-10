# Auth Service API

This document describes the HTTP/REST API exposed by the auth-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

Protected endpoints require a valid Firebase JWT token in the `Authorization` header.

```
Authorization: Bearer <firebase_id_token>
```

The token is obtained from Firebase Authentication on the mobile client after the user signs in with email, phone, or social providers.

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes (protected) | Firebase JWT token: `Bearer <token>` |
| `Content-Type` | Yes (POST) | `application/json` |

## Error Response Format

All errors return the appropriate HTTP status code with this JSON body:

```json
{
    "ErrorMessage": "ErrorPhoneNumberNotAvailable"
}
```

> **Note:** JSON field names use PascalCase throughout the API (matching proto field names with `UseProtoNames: true`).

| HTTP Code | Meaning |
|-----------|---------|
| 200 | Success |
| 400 | Invalid request parameters (`INVALID_ARGUMENT`) |
| 401 | Missing or invalid token (`UNAUTHENTICATED`) |
| 404 | Resource not found (`NOT_FOUND`) |
| 409 | Resource already exists (`ALREADY_EXISTS`) |
| 412 | Pre-condition not met (`FAILED_PRECONDITION`) |
| 500 | Internal server error (`INTERNAL`) |

---

## Endpoints

### POST /api/v1/auth/login

Checks if the authenticated Firebase user has an existing account. If yes, returns the user profile.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/auth/login HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json
```

**Body:** None required (Firebase UID extracted from JWT)

#### Response (Account Exists)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Exists": true,
    "User": {
        "AuthID": "8b1518f9-0949-4872-92a4-5dbdfb7863d9",
        "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "Doe",
        "FirstName": "Samuel",
        "Email": "samuel@example.com",
        "PhoneNumber": "+22890123456",
        "ProfileImageURL": ""
    }
}
```

#### Response (Account Does Not Exist)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Exists": false,
    "User": null
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `Exists` | boolean | `true` if account exists, `false` otherwise |
| `User` | object \| null | User profile if exists, `null` otherwise |
| `User.AuthID` | string | Auth service account ID |
| `User.UserID` | string | User profile ID |
| `User.Name` | string | Last name |
| `User.FirstName` | string | First name |
| `User.Email` | string | Email address |
| `User.PhoneNumber` | string | Phone number (E.164 format) |
| `User.ProfileImageURL` | string | URL to profile picture |

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/auth/login \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json"
```

---

### POST /api/v1/auth/createAccount

Creates a new account for the authenticated Firebase user.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/auth/createAccount HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "Name": "samuel",
    "FirstName": "Doe",
    "Email": "samuel@example.com",
    "PhoneNumber": "+22890123456",
    "ProfileImageURL": "https://example.com/photo.jpg"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | string | Yes | Last name |
| `FirstName` | string | Yes | First name |
| `Email` | string | No | Email address (required if no PhoneNumber) |
| `PhoneNumber` | string | No | Phone number E.164 (required if no Email) |
| `ProfileImageURL` | string | No | URL to profile picture |

> At least one of `Email` or `PhoneNumber` must be provided.

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "User": {
        "AuthID": "8b1518f9-0949-4872-92a4-5dbdfb7863d9",
        "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "samuel",
        "FirstName": "Doe",
        "Email": "samuel@example.com",
        "PhoneNumber": "+22890123456",
        "ProfileImageURL": ""
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `User` | object | Created user profile |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorInternalServer` | 500 | Internal error (e.g. DB) |
| `ErrorEmailNotAvailable` | 409 | Email already in use |
| `ErrorPhoneNumberNotAvailable` | 409 | Phone already in use |

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/auth/createAccount \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "Name": "samuel",
    "FirstName": "Doe",
    "Email": "samuel@example.com",
    "PhoneNumber": "+22890123456"
  }'
```

---

### POST /api/v1/auth/checkPhoneNumber

Checks if a phone number is available for registration.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/auth/checkPhoneNumber HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "PhoneNumber": "+22890123456"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PhoneNumber` | string | Yes | Phone number to check (E.164 format) |

#### Response (Available)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "IsAvailable": true
}
```

#### Response (Not Available)

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{
    "ErrorMessage": "ErrorPhoneNumberNotAvailable"
}
```

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/auth/checkPhoneNumber \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{"PhoneNumber": "+22890123456"}'
```

---

### POST /api/v1/auth/checkEmail

Checks if an email address is available for registration.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/auth/checkEmail HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "Email": "samuel@example.com"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Email` | string | Yes | Email to check |

#### Response (Available)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "IsAvailable": true
}
```

#### Response (Not Available)

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{
    "ErrorMessage": "ErrorEmailNotAvailable"
}
```

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/auth/checkEmail \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{"Email": "samuel@example.com"}'
```

---

### DELETE /api/v1/auth/deleteAccount

Permanently deletes the user's account and anonymizes their data (GDPR).

**Authentication:** Required (Firebase JWT)

**⚠️ Irreversible.** All personal data will be deleted or anonymized.

#### Request

```http
DELETE /api/v1/auth/deleteAccount HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
```

**Body:** None required

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Success": true
}
```

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorCantDeleteAccount` | 412 | Active bookings or trips pending |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
curl -X DELETE https://api.tissimah.kpeewu.dev/api/v1/auth/deleteAccount \
  -H "Authorization: Bearer <firebase_token>"
```

---

## Inter-Service RPCs (gRPC only)

Not exposed via HTTP. Called directly by other services.

### GetAuthInfo

Retrieves authentication information for a user by their auth ID.

```protobuf
rpc GetAuthInfo(GetAuthInfoRequest) returns (GetAuthInfoResponse);
```

| Field | Type | Description |
|-------|------|-------------|
| `AuthID` | string | Auth service account ID |

Response fields: `AuthID`, `Email`, `PhoneNumber`, `IsActive`, `IsSuspended`, `SuspensionEndDate`.
