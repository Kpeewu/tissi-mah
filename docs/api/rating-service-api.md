# Rating Service API

This document describes the HTTP/REST API exposed by the rating-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080/api/v1` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev/api/v1` |
| Staging | `https://staging.tissi-mah.com/api/v1` |
| Production | `https://api.tissi-mah.com/api/v1` |

## Authentication

Rating endpoints (except `health`) require a **Firebase JWT** (`Authorization: Bearer <token>`).
The rater is always the **caller**: the api-gateway validates the JWT and injects the Firebase UID,
which rating-service resolves to the internal UserID (user-service `GetUserByFirebaseID`).
`RaterId` is no longer accepted in the request body. `UserRatedId` is validated against user-service.

Internal services (booking-service, trips-service) read averages through the gRPC-only route
`GetUserRatingsAverageInternal` (`/api/v1/ratings/internal/...`, blocked at the gateway edge).

## Caching

Read endpoints (`GetUserRatings`, `GetUserRatingsAverage`) are cached in Redis with a 5-minute TTL.
Cache is automatically invalidated when a new rating is created or an existing rating is updated.
If Redis is unavailable, the service operates without cache (graceful degradation).

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes (except health) | `Bearer <firebase-id-token>` |
| `Content-Type` | Yes (POST/PATCH) | `application/json` |
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

### POST /ratings/rateUser

Creates a new rating for a user.

**Authentication:** Firebase JWT required (rater = caller)

#### Request

```http
POST /api/v1/ratings/rateUser HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "UserRatedId": "u-550e8400-e29b-41d4-a716-446655440000",
    "NumberOfStars": 4,
    "Comment": "Excellent trajet, conducteur ponctuel et agréable"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserRatedId` | string | Yes | UserID of the user being rated (validated against user-service) |
| `NumberOfStars` | integer | Yes | Rating from 1 to 5 |
| `Comment` | string | No | Optional text comment |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Rating": {
        "RatingId": "r-550e8400-e29b-41d4-a716-446655440000",
        "RaterId": "u-abc123-user-id",
        "UserRatedId": "u-550e8400-e29b-41d4-a716-446655440000",
        "NumberOfStars": 4,
        "Comment": "Excellent trajet, conducteur ponctuel et agréable",
        "CreatedAt": "2024-02-28T12:00:00Z",
        "UpdatedAt": "2024-02-28T12:00:00Z"
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error message if failed, empty if success |
| `Rating` | object | Created rating details |
| `Rating.RatingId` | string | Unique rating ID |
| `Rating.RaterId` | string | UserID of the rater |
| `Rating.UserRatedId` | string | UserID of the user being rated |
| `Rating.NumberOfStars` | integer | Rating (1-5) |
| `Rating.Comment` | string | Optional comment |
| `Rating.CreatedAt` | string | ISO 8601 timestamp (TIMESTAMPTZ) |
| `Rating.UpdatedAt` | string | ISO 8601 timestamp (TIMESTAMPTZ) |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorMissingUserRatedID` | 400 | UserRatedId is required |
| `ErrorInvalidStars` | 400 | Stars must be between 1 and 5 |
| `ErrorSelfRating` | 400 | Cannot rate yourself |
| `ErrorUserNotFound` | 404 | UserRatedId does not exist in user-service |
| `ErrorRatingAlreadyExists` | 409 | Already rated this user |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/ratings/rateUser \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <firebase-id-token>" \
  -d '{
    "UserRatedId": "u-550e8400",
    "NumberOfStars": 4,
    "Comment": "Excellent trajet"
  }'
```

---

### GET /ratings/user/getUserRatings

Retrieves all ratings received by a specific user. **Cached** (5 min TTL).

**Authentication:** Firebase JWT required

#### Request

```http
GET /api/v1/ratings/user/getUserRatings?UserRatedId=u-550e8400 HTTP/1.1
Host: api.tissi-mah.com
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserRatedId` | string | Yes | UserID of the user whose ratings to retrieve |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Ratings": [
        {
            "RatingId": "r-550e8400",
            "RaterId": "u-abc123-user-id",
            "UserRatedId": "u-550e8400",
            "NumberOfStars": 5,
            "Comment": "Parfait",
            "CreatedAt": "2024-02-28T12:00:00Z",
            "UpdatedAt": "2024-02-28T12:00:00Z",
            "RaterFirstName": "Amadou",
            "RaterLastName": "Diallo",
            "RaterProfileImageURL": "https://s3.../selfie.jpg?X-Amz-Signature=..."
        },
        {
            "RatingId": "r-660f9511",
            "RaterId": "u-def456-user-id",
            "UserRatedId": "u-550e8400",
            "NumberOfStars": 4,
            "Comment": "",
            "CreatedAt": "2024-03-01T12:00:00Z",
            "UpdatedAt": "2024-03-01T12:00:00Z",
            "RaterFirstName": "",
            "RaterLastName": "",
            "RaterProfileImageURL": ""
        }
    ]
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error message if failed, empty if success |
| `Ratings` | array | List of ratings received by the user |
| `Ratings[].RaterFirstName` | string | Prénom du noteur, résolu via user-service (batch). Vide si le profil est introuvable ou si user-service est indisponible (dégradation gracieuse) |
| `Ratings[].RaterLastName` | string | Nom du noteur (même règle) |
| `Ratings[].RaterProfileImageURL` | string | URL présignée de la photo de profil du noteur (même règle) — le client affiche « Anonyme » quand les trois champs sont vides |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorMissingUserRatedID` | 400 | UserRatedId is required |

#### Example (cURL)

```bash
curl "https://api.tissi-mah.com/api/v1/ratings/user/getUserRatings?UserRatedId=u-550e8400" \
  -H "Authorization: Bearer <firebase-id-token>"
```

---

### GET /ratings/user/getUserRatingsAverage

Retrieves the average rating (1 decimal place) and total number of ratings for a user. **Cached** (5 min TTL).

**Authentication:** Firebase JWT required

#### Request

```http
GET /api/v1/ratings/user/getUserRatingsAverage?UserRatedId=u-550e8400 HTTP/1.1
Host: api.tissi-mah.com
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserRatedId` | string | Yes | UserID of the user whose average to retrieve |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Average": 4.5,
    "TotalRatings": 12
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error message if failed, empty if success |
| `Average` | number | Average rating rounded to 1 decimal place (0.0 if no ratings) |
| `TotalRatings` | integer | Total number of ratings received |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorMissingUserRatedID` | 400 | UserRatedId is required |

#### Example (cURL)

```bash
curl "https://api.tissi-mah.com/api/v1/ratings/user/getUserRatingsAverage?UserRatedId=u-550e8400" \
  -H "Authorization: Bearer <firebase-id-token>"
```

---

### PATCH /ratings/updateRating

Updates an existing rating. Only the original rater (the caller) can modify their rating.

**Authentication:** Firebase JWT required (rater = caller)

#### Request

```http
PATCH /api/v1/ratings/updateRating HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "RatingId": "r-550e8400-e29b-41d4-a716-446655440000",
    "UserRatedId": "u-550e8400-e29b-41d4-a716-446655440000",
    "NumberOfStars": 5,
    "Comment": "Finalement c'était le meilleur trajet !"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `RatingId` | string | Yes | Rating ID to update |
| `UserRatedId` | string | Yes | UserID of the user being rated |
| `NumberOfStars` | integer | Yes | New rating from 1 to 5 |
| `Comment` | string | No | Updated comment |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Rating": {
        "RatingId": "r-550e8400-e29b-41d4-a716-446655440000",
        "RaterId": "u-abc123-user-id",
        "UserRatedId": "u-550e8400-e29b-41d4-a716-446655440000",
        "NumberOfStars": 5,
        "Comment": "Finalement c'était le meilleur trajet !",
        "CreatedAt": "2024-02-28T12:00:00Z",
        "UpdatedAt": "2024-03-01T12:00:00Z"
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
curl -X PATCH https://api.tissi-mah.com/api/v1/ratings/updateRating \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <firebase-id-token>" \
  -d '{
    "RatingId": "r-550e8400",
    "UserRatedId": "u-550e8400",
    "NumberOfStars": 5,
    "Comment": "Finalement c était le meilleur trajet !"
  }'
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
    "Status": "SERVING",
    "Version": "1.0.0",
    "Timestamp": 1709136000
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
| **Owner-only modification** | Only the rater can update their rating |
| **No deletion** | Users cannot delete ratings |
| **Caller identity** | The rater is the authenticated caller (Firebase UID resolved to the internal UserID); UserRatedId must exist in user-service |
| **Average precision** | Average rounded to 1 decimal place |

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
| `ErrorMissingUserRatedID` | 400 | INVALID_ARGUMENT (3) | UserRatedId is required | Provide UserRatedId |
| `ErrorRatingNotFound` | 404 | NOT_FOUND (5) | Rating does not exist | Check rating ID |
| `ErrorUserNotFound` | 404 | NOT_FOUND (5) | User does not exist in user-service | Check UserRatedId |
| `ErrorRatingAlreadyExists` | 409 | ALREADY_EXISTS (6) | Already rated this user | Update existing rating |
| `ErrorInvalidStars` | 400 | INVALID_ARGUMENT (3) | Stars not in 1-5 range | Fix stars value |
| `ErrorSelfRating` | 400 | INVALID_ARGUMENT (3) | Cannot rate yourself | Rate another user |
| `ErrorUnauthorizedAction` | 403 | PERMISSION_DENIED (7) | Not the rater | Only rater can modify |
| `ErrorDataRetrievalFailed` | 500 | INTERNAL (13) | Database query failed | Retry later |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Server error | Retry later |
