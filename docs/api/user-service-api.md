# User Service API

This document describes the HTTP/REST API exposed by the user-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080/api/v1` |
| VPS-Dev | `https://dev.tissi-mah.com/api/v1` |
| Staging | `https://staging.tissi-mah.com/api/v1` |
| Production | `https://api.tissi-mah.com/api/v1` |

## Authentication

Protected endpoints require a valid Firebase JWT token in the `Authorization` header.

```
Authorization: Bearer <firebase_id_token>
```

The token is obtained from Firebase Authentication on the mobile client after the user signs in.

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes (protected) | Firebase JWT token: `Bearer <token>` |
| `Content-Type` | Yes (POST/PATCH) | `application/json` |
| `Accept` | No | `application/json` |
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
| `errorMessage` | string | Error identifier |
| `code` | integer | gRPC status code |
| `details` | object \| null | Additional error details (optional) |

---

## Endpoints

### POST /user/me

Retrieves the complete profile of the authenticated user, including auth data and files.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/user/me HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Content-Type: application/json
```

**Body:** None required (Firebase UID extracted from JWT)

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "user": {
        "authID": "firebase-uid-abc123",
        "profileID": "u-550e8400-e29b-41d4-a716-446655440000",
        "name": "Doe",
        "firstName": "Samuel",
        "gender": "male",
        "dateOfBirth": "1995-03-15",
        "bio": "Passager regulier",
        "email": "samuel@example.com",
        "phoneNumber": "+22891234567",
        "profileImageURL": "https://tissi-mah-files.s3.amazonaws.com/profiles/photo.jpg",
        "hasProfileImage": true,
        "isDriver": true,
        "isPassenger": true,
        "isDriverProfileVerified": false,
        "isPassengerProfileVerified": true,
        "isActive": true,
        "isSuspended": false,
        "suspensionEndDate": "",
        "tripPreferences": [
            {"preference": "music", "isAllowed": true},
            {"preference": "smoking", "isAllowed": false}
        ],
        "idCardExpirationDate": "2028-06-15",
        "driveLicenceExpirationDate": "2030-12-01",
        "userFiles": [
            {
                "fileID": "f-123",
                "fileURL": "https://tissi-mah-files.s3.amazonaws.com/idCardFront/photo.jpg",
                "fileType": "idCardFront"
            }
        ]
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed |
| `user` | object | Full user profile |
| `user.authID` | string | Auth service ID |
| `user.profileID` | string | User profile ID |
| `user.name` | string | Last name |
| `user.firstName` | string | First name |
| `user.gender` | string | Gender |
| `user.dateOfBirth` | string | Birth date (YYYY-MM-DD) |
| `user.bio` | string | User biography |
| `user.email` | string | Email address |
| `user.phoneNumber` | string | Phone number (E.164) |
| `user.profileImageURL` | string | Profile picture URL |
| `user.hasProfileImage` | boolean | Whether user has a profile image |
| `user.isDriver` | boolean | Driver account activated |
| `user.isPassenger` | boolean | Passenger account activated |
| `user.isDriverProfileVerified` | boolean | Driver verification status |
| `user.isPassengerProfileVerified` | boolean | Passenger verification status |
| `user.isActive` | boolean | Account active status |
| `user.isSuspended` | boolean | Account suspension status |
| `user.suspensionEndDate` | string | Suspension end date (if suspended) |
| `user.tripPreferences` | array | Trip preferences list |
| `user.idCardExpirationDate` | string | ID card expiration date |
| `user.driveLicenceExpirationDate` | string | Driver licence expiration date |
| `user.userFiles` | array | User uploaded files |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/user/me \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json"
```

---

### PATCH /userProfile/createDriverAccount

Activates or deactivates the driver account on the user's profile.

**Authentication:** Required (Firebase JWT)

#### Request

```http
PATCH /api/v1/userProfile/createDriverAccount HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Content-Type: application/json

{
    "profileID": "u-550e8400-e29b-41d4-a716-446655440000",
    "createDriverAccount": true
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `profileID` | string | Yes | User profile ID |
| `createDriverAccount` | boolean | Yes | `true` to activate, `false` to deactivate |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "success": true
}
```

#### Example (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/userProfile/createDriverAccount \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"profileID": "u-550e8400", "createDriverAccount": true}'
```

---

### POST /userProfile/addTripPreferences

Adds or updates trip preferences for the user.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/userProfile/addTripPreferences HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Content-Type: application/json

{
    "profileID": "u-550e8400-e29b-41d4-a716-446655440000",
    "preferences": [
        {"preference": "music", "isAllowed": true},
        {"preference": "smoking", "isAllowed": false},
        {"preference": "pets", "isAllowed": true},
        {"preference": "conversation", "isAllowed": true}
    ]
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `profileID` | string | Yes | User profile ID |
| `preferences` | array | Yes | List of trip preferences |
| `preferences[].preference` | string | Yes | Preference name |
| `preferences[].isAllowed` | boolean | Yes | Whether the preference is allowed |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "success": true
}
```

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/userProfile/addTripPreferences \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "profileID": "u-550e8400",
    "preferences": [
        {"preference": "music", "isAllowed": true},
        {"preference": "smoking", "isAllowed": false}
    ]
  }'
```

---

### PATCH /userProfile/updateProfile

Updates the user's profile information. All fields are optional.

**Authentication:** Required (Firebase JWT)

#### Request

```http
PATCH /api/v1/userProfile/updateProfile HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Content-Type: application/json

{
    "profileID": "u-550e8400-e29b-41d4-a716-446655440000",
    "firstName": "Samuel",
    "lastName": "Doe",
    "birthDate": "1995-03-15",
    "email": "new-email@example.com",
    "phoneNumber": "+22891234567",
    "profilePictureURL": "https://example.com/new-photo.jpg"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `profileID` | string | Yes | User profile ID |
| `firstName` | string | No | New first name |
| `lastName` | string | No | New last name |
| `birthDate` | string | No | Birth date (YYYY-MM-DD) |
| `email` | string | No | New email address |
| `phoneNumber` | string | No | New phone number (E.164) |
| `profilePictureURL` | string | No | New profile picture URL |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "errorMessage": null,
    "user": {
        "authID": "firebase-uid-abc123",
        "profileID": "u-550e8400-e29b-41d4-a716-446655440000",
        "name": "Doe",
        "firstName": "Samuel",
        ...
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `errorMessage` | string \| null | Error message if failed |
| `user` | object | Updated full user profile (same structure as GetMyProfile) |

#### Example (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/userProfile/updateProfile \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "profileID": "u-550e8400",
    "firstName": "Samuel",
    "lastName": "Doe"
  }'
```

---

### GET /user/health

Health check endpoint for the user-service.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/user/health HTTP/1.1
Host: api.tissi-mah.com
```

#### Response

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "status": "SERVING",
    "version": "v1.0.0",
    "timestamp": 1709136000
}
```

#### Example (cURL)

```bash
curl https://api.tissi-mah.com/api/v1/user/health
```

---

## Inter-Service RPCs (gRPC only)

These RPCs are not exposed via HTTP. They are called directly by other services.

### CreateUser

Called by auth-service after account creation to create the user profile.

```protobuf
rpc CreateUser(CreateUserRequest) returns (UserProfileResponse);
```

| Field | Type | Description |
|-------|------|-------------|
| `AuthID` | string | Auth service account ID |
| `Name` | string | Last name |
| `FirstName` | string | First name |
| `ProfilePhotoURL` | string | Profile picture URL (optional) |
| `FirebaseID` | string | Firebase UID |

### GetUserByAuthID

Retrieves user profile by auth service ID.

```protobuf
rpc GetUserByAuthID(GetUserByAuthIDRequest) returns (UserProfileResponse);
```

| Field | Type | Description |
|-------|------|-------------|
| `AuthID` | string | Auth service account ID |

---

## Error Reference

| Error | HTTP | Description | User Action |
|-------|------|-------------|-------------|
| `ErrUserNotFound` | 404 | User profile not found | Create account |
| `ErrInvalidProfileID` | 400 | Invalid profile ID format | Check ID format |
| `ErrInvalidPhoneNumber` | 400 | Invalid phone format | Use E.164 format |
| `ErrInvalidEmail` | 400 | Invalid email format | Check email |
| `ErrPhoneNumberTaken` | 412 | Phone already in use | Use different phone |
| `ErrEmailTaken` | 412 | Email already in use | Use different email |
