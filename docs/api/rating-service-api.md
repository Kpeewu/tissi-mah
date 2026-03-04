# Rating Service API

This document describes the HTTP/REST API exposed by the rating-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

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
| `Content-Type` | Yes (POST/PUT) | `application/json` |
| `Accept` | No | `application/json` |
| `X-Request-ID` | No | Client-generated UUID for request tracing |

## Error Response Format

All errors follow this format:

```json
{
    "ErrorMessage": "ErrorRatingNotFound"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier (matches backend sentinel error name) |

---

## Endpoints

### POST /ratings

Creates a new rating for a user. The rater is automatically identified from the Firebase JWT.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/ratings HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Content-Type: application/json

{
    "user_rated_id": "u-550e8400-e29b-41d4-a716-446655440000",
    "number_of_stars": 4,
    "comment": "Excellent trajet, conducteur ponctuel et agréable"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_rated_id` | string | Yes | ID of the user being rated |
| `number_of_stars` | integer | Yes | Rating from 1 to 5 |
| `comment` | string | No | Optional text comment |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "error_message": "",
    "rating": {
        "rating_id": "r-550e8400-e29b-41d4-a716-446655440000",
        "rater_id": "firebase-uid-abc123",
        "user_rated_id": "u-550e8400-e29b-41d4-a716-446655440000",
        "number_of_stars": 4,
        "comment": "Excellent trajet, conducteur ponctuel et agréable",
        "created_at": 1709136000,
        "updated_at": 1709136000
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `error_message` | string | Error message if failed, empty if success |
| `rating` | object | Created rating details |
| `rating.rating_id` | string | Unique rating ID |
| `rating.rater_id` | string | Firebase UID of the rater |
| `rating.user_rated_id` | string | ID of the user being rated |
| `rating.number_of_stars` | integer | Rating (1-5) |
| `rating.comment` | string | Optional comment |
| `rating.created_at` | integer | Unix timestamp |
| `rating.updated_at` | integer | Unix timestamp |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidStars` | 400 | Stars must be between 1 and 5 |
| `ErrorSelfRating` | 400 | Cannot rate yourself |
| `ErrorRatingAlreadyExists` | 409 | Already rated this user |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/ratings \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "user_rated_id": "u-550e8400",
    "number_of_stars": 4,
    "comment": "Excellent trajet"
  }'
```

---

### GET /ratings/{rating_id}

Retrieves a specific rating by its ID.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/ratings/r-550e8400-e29b-41d4-a716-446655440000 HTTP/1.1
Host: api.tissi-mah.com
```

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "error_message": "",
    "rating": {
        "rating_id": "r-550e8400-e29b-41d4-a716-446655440000",
        "rater_id": "firebase-uid-abc123",
        "user_rated_id": "u-550e8400-e29b-41d4-a716-446655440000",
        "number_of_stars": 4,
        "comment": "Excellent trajet, conducteur ponctuel et agréable",
        "created_at": 1709136000,
        "updated_at": 1709136000
    }
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorRatingNotFound` | 404 | Rating does not exist |

#### Example (cURL)

```bash
curl https://api.tissi-mah.com/api/v1/ratings/r-550e8400
```

---

### GET /ratings/user/{user_rated_id}

Retrieves all ratings received by a specific user.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/ratings/user/u-550e8400-e29b-41d4-a716-446655440000 HTTP/1.1
Host: api.tissi-mah.com
```

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "error_message": "",
    "ratings": [
        {
            "rating_id": "r-550e8400",
            "rater_id": "firebase-uid-abc123",
            "user_rated_id": "u-550e8400",
            "number_of_stars": 5,
            "comment": "Parfait",
            "created_at": 1709136000,
            "updated_at": 1709136000
        },
        {
            "rating_id": "r-660f9511",
            "rater_id": "firebase-uid-def456",
            "user_rated_id": "u-550e8400",
            "number_of_stars": 4,
            "comment": "",
            "created_at": 1709222400,
            "updated_at": 1709222400
        }
    ]
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `error_message` | string | Error message if failed, empty if success |
| `ratings` | array | List of ratings received by the user |

#### Example (cURL)

```bash
curl https://api.tissi-mah.com/api/v1/ratings/user/u-550e8400
```

---

### GET /ratings/user/{user_rated_id}/average

Retrieves the average rating and total number of ratings for a user.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/ratings/user/u-550e8400-e29b-41d4-a716-446655440000/average HTTP/1.1
Host: api.tissi-mah.com
```

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "error_message": "",
    "average": 4.5,
    "total_ratings": 12
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `error_message` | string | Error message if failed, empty if success |
| `average` | number | Average rating (0.0 if no ratings) |
| `total_ratings` | integer | Total number of ratings received |

#### Example (cURL)

```bash
curl https://api.tissi-mah.com/api/v1/ratings/user/u-550e8400/average
```

---

### PUT /ratings/{rating_id}

Updates an existing rating. Only the original rater can modify their rating.

**Authentication:** Required (Firebase JWT)

#### Request

```http
PUT /api/v1/ratings/r-550e8400-e29b-41d4-a716-446655440000 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
Content-Type: application/json

{
    "rating_id": "r-550e8400-e29b-41d4-a716-446655440000",
    "number_of_stars": 5,
    "comment": "Finalement c'était le meilleur trajet !"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `rating_id` | string | Yes | Rating ID (must match URL) |
| `number_of_stars` | integer | Yes | New rating from 1 to 5 |
| `comment` | string | No | Updated comment |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "error_message": "",
    "rating": {
        "rating_id": "r-550e8400-e29b-41d4-a716-446655440000",
        "rater_id": "firebase-uid-abc123",
        "user_rated_id": "u-550e8400-e29b-41d4-a716-446655440000",
        "number_of_stars": 5,
        "comment": "Finalement c'était le meilleur trajet !",
        "created_at": 1709136000,
        "updated_at": 1709222400
    }
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorRatingNotFound` | 404 | Rating does not exist |
| `ErrorInvalidStars` | 400 | Stars must be between 1 and 5 |
| `ErrorUnauthorizedAction` | 403 | Only the rater can modify this rating |

#### Example (cURL)

```bash
curl -X PUT https://api.tissi-mah.com/api/v1/ratings/r-550e8400 \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{
    "rating_id": "r-550e8400",
    "number_of_stars": 5,
    "comment": "Finalement c était le meilleur trajet !"
  }'
```

---

### DELETE /ratings/{rating_id}

Deletes a rating. Only the original rater can delete their rating.

**Authentication:** Required (Firebase JWT)

#### Request

```http
DELETE /api/v1/ratings/r-550e8400-e29b-41d4-a716-446655440000 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
```

**Body:** None required

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "error_message": "",
    "success": true
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `error_message` | string | Error message if failed, empty if success |
| `success` | boolean | `true` if rating was deleted |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorRatingNotFound` | 404 | Rating does not exist |
| `ErrorUnauthorizedAction` | 403 | Only the rater can delete this rating |
| `ErrorCantDeleteRating` | 412 | Cannot delete this rating |

#### Example (cURL)

```bash
curl -X DELETE https://api.tissi-mah.com/api/v1/ratings/r-550e8400 \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..."
```

---

### GET /ratings/health

Health check endpoint for the rating-service.

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/ratings/health HTTP/1.1
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
curl https://api.tissi-mah.com/api/v1/ratings/health
```

---

## Business Rules

### Rating Constraints

| Rule | Description |
|------|-------------|
| **Stars range** | Must be between 1 and 5 (inclusive) |
| **No self-rating** | A user cannot rate themselves |
| **One rating per pair** | A rater can only rate a given user once |
| **Owner-only modification** | Only the rater can update or delete their rating |

### Database Constraints

| Constraint | Type | Description |
|------------|------|-------------|
| `uq_rater_user_rated` | UNIQUE | One rating per (rater_id, user_rated_id) pair |
| `ck_number_of_stars` | CHECK | Stars between 1 and 5 |
| `ck_no_self_rating` | CHECK | rater_id != user_rated_id |

---

## Error Reference

| Error | HTTP | gRPC Code | Description | User Action |
|-------|------|-----------|-------------|-------------|
| `ErrorRatingNotFound` | 404 | NOT_FOUND (5) | Rating does not exist | Check rating ID |
| `ErrorRatingAlreadyExists` | 409 | ALREADY_EXISTS (6) | Already rated this user | Update existing rating |
| `ErrorInvalidStars` | 400 | INVALID_ARGUMENT (3) | Stars not in 1-5 range | Fix stars value |
| `ErrorSelfRating` | 400 | INVALID_ARGUMENT (3) | Cannot rate yourself | Rate another user |
| `ErrorUnauthorizedAction` | 403 | PERMISSION_DENIED (7) | Not the rater | Only rater can modify |
| `ErrorCantDeleteRating` | 412 | FAILED_PRECONDITION (9) | Cannot delete | Contact support |
| `ErrorDataRetrievalFailed` | 500 | INTERNAL (13) | Database query failed | Retry later |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Server error | Retry later |
