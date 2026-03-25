# User Service API

This document describes the HTTP/REST API exposed by the user-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

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

The token is obtained from Firebase Authentication on the mobile client after the user signs in.

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes (protected) | Firebase JWT token: `Bearer <token>` |
| `Content-Type` | Yes (POST/PATCH) | `application/json` |

## Error Response Format

All errors return the appropriate HTTP status code with this JSON body:

```json
{
    "ErrorMessage": "ErrUserNotFound"
}
```

> **Note:** JSON field names use PascalCase throughout the API (matching proto field names with `UseProtoNames: true`).

| HTTP Code | Meaning |
|-----------|---------|
| 200 | Success |
| 400 | Invalid request parameters (`INVALID_ARGUMENT`) |
| 401 | Missing or invalid token (`UNAUTHENTICATED`) |
| 404 | Resource not found (`NOT_FOUND`) |
| 412 | Pre-condition not met (`FAILED_PRECONDITION`) |
| 500 | Internal server error (`INTERNAL`) |

---

## Endpoints

### POST /api/v1/user/me

Retrieves the complete profile of the authenticated user, including auth data and files.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/user/me HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json
```

**Body:** None required (Firebase UID extracted from JWT)

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "User": {
        "AuthID": "firebase-uid-abc123",
        "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "Doe",
        "FirstName": "Samuel",
        "Gender": "male",
        "DateOfBirth": "1995-03-15",
        "Bio": "Passager regulier",
        "Email": "samuel@example.com",
        "PhoneNumber": "+22891234567",
        "ProfileImageURL": "https://tissi-mah-files.s3.amazonaws.com/profiles/photo.jpg",
        "HasProfileImage": true,
        "IsDriver": true,
        "IsPassenger": true,
        "IsDriverProfileVerified": false,
        "IsPassengerProfileVerified": true,
        "IsActive": true,
        "IsSuspended": false,
        "SuspensionEndDate": "",
        "TripPreferences": [
            {"Preference": "music", "IsAllowed": true},
            {"Preference": "smoking", "IsAllowed": false}
        ],
        "IDCardExpirationDate": "2028-06-15",
        "DriveLicenceExpirationDate": "2030-12-01",
        "UserFiles": [
            {
                "FileID": "d-550e8400-e29b-41d4-a716-446655440000",
                "FileURL": "https://tissi-mah-files.s3.amazonaws.com/idCardFront/photo.jpg",
                "FileType": "idCardFront"
            }
        ]
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `User` | object | Full user profile |
| `User.AuthID` | string | Auth service ID |
| `User.ProfileID` | string | User profile ID |
| `User.Name` | string | Last name |
| `User.FirstName` | string | First name |
| `User.Gender` | string | Gender |
| `User.DateOfBirth` | string | Birth date (YYYY-MM-DD) |
| `User.Bio` | string | User biography |
| `User.Email` | string | Email address |
| `User.PhoneNumber` | string | Phone number (E.164) |
| `User.ProfileImageURL` | string | Profile picture URL |
| `User.HasProfileImage` | boolean | Whether user has a profile image |
| `User.IsDriver` | boolean | Driver account activated |
| `User.IsPassenger` | boolean | Passenger account activated |
| `User.IsDriverProfileVerified` | boolean | Driver verification status |
| `User.IsPassengerProfileVerified` | boolean | Passenger verification status |
| `User.IsActive` | boolean | Account active status |
| `User.IsSuspended` | boolean | Account suspension status |
| `User.SuspensionEndDate` | string | Suspension end date (if suspended) |
| `User.TripPreferences` | array | Trip preferences list |
| `User.IDCardExpirationDate` | string | ID card expiration date |
| `User.DriveLicenceExpirationDate` | string | Driver licence expiration date |
| `User.UserFiles` | array | User uploaded files |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrUserNotFound` | 404 | User profile not found |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/user/me \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json"
```

---

### PATCH /api/v1/userProfile/createDriverAccount

Activates or deactivates the driver account on the user's profile.

**Authentication:** Required (Firebase JWT)

#### Request

```http
PATCH /api/v1/userProfile/createDriverAccount HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "CreateDriverAccount": true
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ProfileID` | string | Yes | User profile ID |
| `CreateDriverAccount` | boolean | Yes | `true` to activate, `false` to deactivate |

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
| `ErrUserNotFound` | 404 | User profile not found |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
curl -X PATCH https://api.tissimah.kpeewu.dev/api/v1/userProfile/createDriverAccount \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{"ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545", "CreateDriverAccount": true}'
```

---

### POST /api/v1/userProfile/addTripPreferences

Adds or updates trip preferences for the user.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/userProfile/addTripPreferences HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "Preferences": [
        {"Preference": "music", "IsAllowed": true},
        {"Preference": "smoking", "IsAllowed": false},
        {"Preference": "pets", "IsAllowed": true},
        {"Preference": "conversation", "IsAllowed": true}
    ]
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ProfileID` | string | Yes | User profile ID |
| `Preferences` | array | Yes | List of trip preferences |
| `Preferences[].Preference` | string | Yes | Preference name |
| `Preferences[].IsAllowed` | boolean | Yes | Whether the preference is allowed |

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
| `ErrUserNotFound` | 404 | User profile not found |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/userProfile/addTripPreferences \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "Preferences": [
        {"Preference": "music", "IsAllowed": true},
        {"Preference": "smoking", "IsAllowed": false}
    ]
  }'
```

---

### PATCH /api/v1/userProfile/updateProfile

Updates the user's profile information. All fields except `ProfileID` are optional.

**Authentication:** Required (Firebase JWT)

#### Request

```http
PATCH /api/v1/userProfile/updateProfile HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "FirstName": "Samuel",
    "LastName": "Doe",
    "BirthDate": "1995-03-15",
    "Email": "new-email@example.com",
    "PhoneNumber": "+22891234567"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ProfileID` | string | Yes | User profile ID |
| `FirstName` | string | No | New first name |
| `LastName` | string | No | New last name |
| `BirthDate` | string | No | Birth date (YYYY-MM-DD) |
| `Email` | string | No | New email address |
| `PhoneNumber` | string | No | New phone number (E.164) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "User": {
        "AuthID": "firebase-uid-abc123",
        "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "Doe",
        "FirstName": "Samuel",
        ...
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `User` | object | Updated full user profile (same structure as GetMyProfile) |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrUserNotFound` | 404 | User profile not found |
| `ErrPhoneNumberTaken` | 412 | Phone already in use |
| `ErrEmailTaken` | 412 | Email already in use |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
curl -X PATCH https://api.tissimah.kpeewu.dev/api/v1/userProfile/updateProfile \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "FirstName": "Samuel",
    "LastName": "Doe"
  }'
```

---

### GET /api/v1/user/health

Health check endpoint for the user-service.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/user/health HTTP/1.1
Host: api.tissimah.kpeewu.dev
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
curl https://api.tissimah.kpeewu.dev/api/v1/user/health
```

---

## Inter-Service RPCs (gRPC only)

Not exposed via HTTP. Called directly by other services.

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

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrUserNotFound` | 404 | User profile not found |
| `ErrInvalidProfileID` | 400 | Invalid profile ID format |
| `ErrInvalidPhoneNumber` | 400 | Invalid phone format (use E.164) |
| `ErrInvalidEmail` | 400 | Invalid email format |
| `ErrPhoneNumberTaken` | 412 | Phone already in use |
| `ErrEmailTaken` | 412 | Email already in use |
| `ErrorInternalServer` | 500 | Internal server error |
