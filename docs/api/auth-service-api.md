# Auth Service API

This document describes the HTTP/REST API exposed by the auth-service through Kong API Gateway. This API is consumed by mobile clients (iOS/Android).

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8000/api/v1` |
| VPS-Dev | `https://dev.tissi-mah.com/api/v1` |
| Staging | `https://staging.tissi-mah.com/api/v1` |
| Production | `https://api.tissi-mah.com/api/v1` |

## Authentication

All endpoints require a valid Firebase JWT token in the `Authorization` header.

```
Authorization: Bearer <firebase_id_token>
```

The token is obtained from Firebase Authentication on the mobile client after the user signs in with email, phone, or social providers.

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | Firebase JWT token: `Bearer <token>` |
| `Content-Type` | Yes (POST/PUT) | `application/json` |
| `Accept` | No | `application/json` |
| `Accept-Language` | No | Preferred language: `fr`, `en` |
| `X-Request-ID` | No | Client-generated UUID for request tracing |

## Error Response Format

All errors follow this format:

```json
{
    "errorMessage": "ErrUserNotFound",
    "code": 5,
    "details": null
}
```

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string | Error identifier (matches backend error variable name) |
| `code` | integer | gRPC status code |
| `details` | object \| null | Additional error details (optional) |

---

## Endpoints

### POST /auth/login

Checks if the authenticated user has an existing account. If yes, returns the user profile.

#### Request

```http
POST /api/v1/auth/login HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json
Accept: application/json
X-Request-ID: 550e8400-e29b-41d4-a716-446655440000
```

**Body:** None required

#### Response (Account Exists)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "exists": true,
    "user": {
        "profileId": "u-550e8400-e29b-41d4-a716-446655440000",
        "name": "Doe",
        "firstName": "Samuel",
        "email": "samuel@example.com",
        "phoneNumber": "+22891",
        "profileImageURL": "https://tissi-mah-files.s3.amazonaws.com/profiles/u-550e8400/photo.jpg"
    }
}
```

#### Response (Account Does Not Exist)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "exists": false,
    "user": null
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed, `null` if success |
| `exists` | boolean | `true` if account exists, `false` otherwise |
| `user` | object \| null | User profile if exists, `null` otherwise |
| `user.profileId` | string | Unique user ID (prefixed with `u-`) |
| `user.name` | string | Last name |
| `user.firstName` | string | First name |
| `user.email` | string | Email address |
| `user.phoneNumber` | string | Phone number (E.164 format) |
| `user.profileImageURL` | string | URL to profile picture |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrAccountSuspended` | 401 | Account is temporarily suspended |
| `ErrAccountDeactivated` | 401 | Account has been deleted |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/auth/login \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json"
```

---

### POST /auth/createAccount

Creates a new account for the authenticated Firebase user.

#### Request

```http
POST /api/v1/auth/createAccount HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json
Accept: application/json
X-Request-ID: 550e8400-e29b-41d4-a716-446655440001

{
    "name": "samuel",
    "firstName": "Doe",
    "email": "samuel@example.com",
    "phoneNumber": "+22890123456",
    "profileImageURL": "https://example.com/photo.jpg"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Last name (max 100 characters) |
| `firstName` | string | Yes | First name (max 100 characters) |
| `email` | string | No | Email address |
| `phoneNumber` | string | No | Phone number (E.164 format: `+228XXXXXXXXXX`) |
| `profileImageURL` | string | No | URL to profile picture |

#### Response (Success)

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
    "errorMessage": null,
    "user": {
        "profileId": "u-550e8400-e29b-41d4-a716-446655440000",
        "name": "samuel",
        "firstName": "Doe",
        "email": "samuel@example.com",
        "phoneNumber": "+22890123456",
        "profileImageURL": "https://tissi-mah-files.s3.amazonaws.com/profiles/u-550e8400/photo.jpg"
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed, `null` if success |
| `user` | object | Created user profile |
| `user.profileId` | string | Unique user ID (prefixed with `u-`) |
| `user.name` | string | Last name |
| `user.firstName` | string | First name |
| `user.email` | string | Email address |
| `user.phoneNumber` | string | Phone number |
| `user.profileImageURL` | string | URL to profile picture |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrAccountAlreadyExists` | 412 | Firebase ID already has an account |
| `ErrPhoneNumberTaken` | 412 | Phone number is already in use |
| `ErrEmailTaken` | 412 | Email is already in use |
| `ErrInvalidPhoneNumber` | 400 | Invalid phone number format |
| `ErrInvalidEmail` | 400 | Invalid email format |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/auth/createAccount \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "name": "samuel",
    "firstName": "Doe",
    "email": "samuel@example.com",
    "phoneNumber": "+22890123456"
  }'
```

---

### POST /auth/checkPhoneNumber

Checks if a phone number is available for registration.

#### Request

```http
POST /api/v1/auth/checkPhoneNumber HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json
Accept: application/json

{
    "phoneNumber": "+22890123456"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `phoneNumber` | string | Yes | Phone number to check (E.164 format) |

#### Response (Available)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "isAvailable": true
}
```

#### Response (Not Available)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "isAvailable": false
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed, `null` if success |
| `isAvailable` | boolean | `true` if available, `false` if taken |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrInvalidPhoneNumber` | 400 | Invalid phone number format |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/auth/checkPhoneNumber \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"phoneNumber": "+22890123456"}'
```

---

### POST /auth/checkEmail

Checks if an email is available for registration.

#### Request

```http
POST /api/v1/auth/checkEmail HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json
Accept: application/json

{
    "email": "samuel@example.com"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | Email to check |

#### Response (Available)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "isAvailable": true
}
```

#### Response (Not Available)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "isAvailable": false
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed, `null` if success |
| `isAvailable` | boolean | `true` if available, `false` if taken |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrInvalidEmail` | 400 | Invalid email format |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/auth/checkEmail \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"email": "samuel@example.com"}'
```

---

### DELETE /auth/deleteAccount

Permanently deletes the user's account and anonymizes their data in compliance with GDPR.

**⚠️ Warning:** This action is irreversible. All personal data will be deleted or anonymized.

#### Pre-conditions

The account cannot be deleted if:
- There are active bookings (pending or approved)
- There are upcoming trips (as driver)
- There are pending payouts

#### Request

```http
DELETE /api/v1/auth/deleteAccount HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Accept: application/json
X-Request-ID: 550e8400-e29b-41d4-a716-446655440002
```

**Body:** None required

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "success": true
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed, `null` if success |
| `success` | boolean | `true` if account was deleted |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrActiveBookingsExist` | 412 | Cannot delete with active bookings |
| `ErrActiveTripsExist` | 412 | Cannot delete with upcoming trips (as driver) |
| `ErrPendingPayoutsExist` | 412 | Cannot delete with pending payouts |

#### What Gets Deleted/Anonymized

| Data | Action |
|------|--------|
| Personal info (name, email, phone) | Anonymized |
| Profile picture | Deleted |
| KYC documents | Deleted |
| Vehicles | Deleted |
| Ratings given | Deleted |
| Bookings | Anonymized (kept for legal reasons) |
| Payments | Anonymized (kept 10 years for legal reasons) |

#### Example (cURL)

```bash
curl -X DELETE https://api.tissi-mah.com/api/v1/auth/deleteAccount \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..."
```

---

## Error Reference

### HTTP Status Codes

| HTTP Code | gRPC Code | Meaning |
|-----------|-----------|---------|
| 200 | `OK` (0) | Success |
| 201 | `OK` (0) | Created successfully |
| 400 | `INVALID_ARGUMENT` (3) | Invalid request parameters |
| 401 | `UNAUTHENTICATED` (16) | Missing or invalid token |
| 403 | `PERMISSION_DENIED` (7) | Not authorized |
| 404 | `NOT_FOUND` (5) | Resource not found |
| 412 | `FAILED_PRECONDITION` (9) | Pre-condition not met |
| 500 | `INTERNAL` (13) | Server error |

### Auth Service Errors

| Error | HTTP | Description | User Action |
|-------|------|-------------|-------------|
| `ErrAccountNotFound` | 404 | Account does not exist | Create account |
| `ErrAccountAlreadyExists` | 412 | Account already exists | Login instead |
| `ErrAccountSuspended` | 401 | Account is suspended | Contact support |
| `ErrAccountDeactivated` | 401 | Account was deleted | Create new account |
| `ErrPhoneNumberTaken` | 412 | Phone already in use | Use different phone |
| `ErrEmailTaken` | 412 | Email already in use | Use different email |
| `ErrInvalidPhoneNumber` | 400 | Invalid phone format | Use E.164 format |
| `ErrInvalidEmail` | 400 | Invalid email format | Check email format |
| `ErrActiveBookingsExist` | 412 | Has active bookings | Complete/cancel bookings |
| `ErrActiveTripsExist` | 412 | Has upcoming trips | Cancel trips first |
| `ErrPendingPayoutsExist` | 412 | Has pending payouts | Wait for payout |

