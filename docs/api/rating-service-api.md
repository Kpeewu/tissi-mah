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

All rating-service endpoints are currently **public** (no Firebase JWT required).
The `rater_id` is provided in the request body for create and update operations.
Both `rater_id` and `user_rated_id` are validated against the user-service to ensure they exist.

> **Note:** When Firebase JWT is fully integrated, protected endpoints will require a valid token and `rater_id` will be extracted from the JWT instead of the body.

## Caching

Read endpoints (`GetUserRatings`, `GetUserRatingsAverage`) are cached in Redis with a 5-minute TTL.
Cache is automatically invalidated when a new rating is created or an existing rating is updated.
If Redis is unavailable, the service operates without cache (graceful degradation).

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

**Authentication:** Not required (public)

#### Request

```http
POST /api/v1/ratings/rateUser HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "RaterId": "firebase-uid-abc123",
    "UserRatedId": "u-550e8400-e29b-41d4-a716-446655440000",
    "NumberOfStars": 4,
    "Comment": "Excellent trajet, conducteur ponctuel et agréable"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `RaterId` | string | Yes | AuthID of the rater (validated against user-service) |
| `UserRatedId` | string | Yes | AuthID of the user being rated (validated against user-service) |
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
        "RaterId": "firebase-uid-abc123",
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
| `Rating.RaterId` | string | AuthID of the rater |
| `Rating.UserRatedId` | string | AuthID of the user being rated |
| `Rating.NumberOfStars` | integer | Rating (1-5) |
| `Rating.Comment` | string | Optional comment |
| `Rating.CreatedAt` | string | ISO 8601 timestamp (TIMESTAMPTZ) |
| `Rating.UpdatedAt` | string | ISO 8601 timestamp (TIMESTAMPTZ) |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorMissingRaterID` | 400 | RaterId is required |
| `ErrorMissingUserRatedID` | 400 | UserRatedId is required |
| `ErrorInvalidStars` | 400 | Stars must be between 1 and 5 |
| `ErrorSelfRating` | 400 | Cannot rate yourself |
| `ErrorUserNotFound` | 404 | RaterId or UserRatedId does not exist in user-service |
| `ErrorRatingAlreadyExists` | 409 | Already rated this user |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/ratings/rateUser \
  -H "Content-Type: application/json" \
  -d '{
    "RaterId": "firebase-uid-abc123",
    "UserRatedId": "u-550e8400",
    "NumberOfStars": 4,
    "Comment": "Excellent trajet"
  }'
```

---

### GET /ratings/user/getUserRatings

Retrieves all ratings received by a specific user. **Cached** (5 min TTL).

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/ratings/user/getUserRatings?UserRatedId=u-550e8400 HTTP/1.1
Host: api.tissi-mah.com
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserRatedId` | string | Yes | AuthID of the user whose ratings to retrieve |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Ratings": [
        {
            "RatingId": "r-550e8400",
            "RaterId": "firebase-uid-abc123",
            "UserRatedId": "u-550e8400",
            "NumberOfStars": 5,
            "Comment": "Parfait",
            "CreatedAt": "2024-02-28T12:00:00Z",
            "UpdatedAt": "2024-02-28T12:00:00Z"
        },
        {
            "RatingId": "r-660f9511",
            "RaterId": "firebase-uid-def456",
            "UserRatedId": "u-550e8400",
            "NumberOfStars": 4,
            "Comment": "",
            "CreatedAt": "2024-03-01T12:00:00Z",
            "UpdatedAt": "2024-03-01T12:00:00Z"
        }
    ]
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error message if failed, empty if success |
| `Ratings` | array | List of ratings received by the user |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorMissingUserRatedID` | 400 | UserRatedId is required |

#### Example (cURL)

```bash
curl "https://api.tissi-mah.com/api/v1/ratings/user/getUserRatings?UserRatedId=u-550e8400"
```

---

### GET /ratings/user/getUserRatingsAverage

Retrieves the average rating (1 decimal place) and total number of ratings for a user. **Cached** (5 min TTL).

**Authentication:** Not required (public)

#### Request

```http
GET /api/v1/ratings/user/getUserRatingsAverage?UserRatedId=u-550e8400 HTTP/1.1
Host: api.tissi-mah.com
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserRatedId` | string | Yes | AuthID of the user whose average to retrieve |

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
curl "https://api.tissi-mah.com/api/v1/ratings/user/getUserRatingsAverage?UserRatedId=u-550e8400"
```

---

### PATCH /ratings/updateRating

Updates an existing rating. Only the original rater can modify their rating.

**Authentication:** Not required (public, RaterId in body)

#### Request

```http
PATCH /api/v1/ratings/updateRating HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "RatingId": "r-550e8400-e29b-41d4-a716-446655440000",
    "RaterId": "firebase-uid-abc123",
    "UserRatedId": "u-550e8400-e29b-41d4-a716-446655440000",
    "NumberOfStars": 5,
    "Comment": "Finalement c'était le meilleur trajet !"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `RatingId` | string | Yes | Rating ID to update |
| `RaterId` | string | Yes | AuthID of the rater (must match original rater) |
| `UserRatedId` | string | Yes | AuthID of the user being rated |
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
        "RaterId": "firebase-uid-abc123",
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
| `ErrorMissingRaterID` | 400 | RaterId is required |
| `ErrorRatingNotFound` | 404 | Rating does not exist |
| `ErrorInvalidStars` | 400 | Stars must be between 1 and 5 |
| `ErrorUnauthorizedAction` | 403 | Only the rater can modify this rating |

#### Example (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/ratings/updateRating \
  -H "Content-Type: application/json" \
  -d '{
    "RatingId": "r-550e8400",
    "RaterId": "firebase-uid-abc123",
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
| **User validation** | Both RaterId and UserRatedId must exist in user-service |
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
| `ErrorMissingRaterID` | 400 | INVALID_ARGUMENT (3) | RaterId is required | Provide RaterId |
| `ErrorMissingUserRatedID` | 400 | INVALID_ARGUMENT (3) | UserRatedId is required | Provide UserRatedId |
| `ErrorRatingNotFound` | 404 | NOT_FOUND (5) | Rating does not exist | Check rating ID |
| `ErrorUserNotFound` | 404 | NOT_FOUND (5) | User does not exist in user-service | Check RaterId or UserRatedId |
| `ErrorRatingAlreadyExists` | 409 | ALREADY_EXISTS (6) | Already rated this user | Update existing rating |
| `ErrorInvalidStars` | 400 | INVALID_ARGUMENT (3) | Stars not in 1-5 range | Fix stars value |
| `ErrorSelfRating` | 400 | INVALID_ARGUMENT (3) | Cannot rate yourself | Rate another user |
| `ErrorUnauthorizedAction` | 403 | PERMISSION_DENIED (7) | Not the rater | Only rater can modify |
| `ErrorDataRetrievalFailed` | 500 | INTERNAL (13) | Database query failed | Retry later |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Server error | Retry later |
