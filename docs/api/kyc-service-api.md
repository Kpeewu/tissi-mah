# KYC Service API

This document describes the HTTP/REST API exposed by the kyc-service through the api-gateway (grpc-gateway). The KYC service integrates with [Persona](https://withpersona.com) for identity verification.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://dev.tissi-mah.com` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

All `/api/v1/kyc/inquiries/*` and `/api/v1/kyc/me/*` endpoints require a valid **Firebase JWT**.
The `/api/v1/kyc/admin/*` endpoints require a JWT with admin privileges.
The `/api/v1/kyc/webhooks/persona` and `/api/v1/kyc/health` endpoints are **public**.

```http
Authorization: Bearer <firebase-id-token>
```

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes (POST) | `application/json` |
| `Authorization` | Yes (protected routes) | `Bearer <firebase-id-token>` |

## Error Response Format

```json
{
    "ErrorMessage": "ErrorInquiryNotFound"
}
```

---

## Endpoints

### POST /api/v1/kyc/inquiries/add

Starts a new Persona identity verification inquiry for a document already uploaded via the file-service.

**Authentication:** Firebase JWT required

> **Flow:** Upload the document first with `/api/v1/file/uploadIdDocument` or `/api/v1/file/uploadVehicleDocuments`, then pass the returned `DocumentID` to this endpoint. The service verifies that the document exists and belongs to the authenticated user before submitting it to Persona.

#### Request

```http
POST /api/v1/kyc/inquiries/add HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DocumentType": "IDCard",
    "VehicleId": "",
    "DocumentId": "d-550e8400-e29b-41d4-a716-446655440001"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DocumentType` | string | Yes | Document type: `IDCard`, `Passport`, `DriverLicence` for user docs; `insurance`, `registrationCard` for vehicle docs |
| `VehicleId` | string | Conditional | Required when submitting a vehicle document |
| `DocumentId` | string | Yes | ID of the uploaded document — returned by `/api/v1/file/uploadIdDocument` or `/api/v1/file/uploadVehicleDocuments` |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440000",
    "PersonaInquiryId": "inq_abc123",
    "PersonaTemplateId": "itmpl_default",
    "SessionToken": "eyJhbGci...",
    "SessionExpiresAt": "2026-04-15T09:00:00Z",
    "Status": "pending",
    "AttemptNumber": 1,
    "CreatedAt": "2026-04-15T08:00:00Z",
    "ErrorMessage": ""
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorMissingDocumentID` | 400 | `DocumentId` is absent or empty |
| `ErrorDocumentMismatch` | 400 | Document type does not match `DocumentType`, or the document does not belong to the authenticated user |
| `ErrorInquiryAlreadyActive` | 409 | An inquiry is already in progress |
| `ErrorFileServiceUnavailable` | 503 | file-service is unreachable |
| `ErrorPersonaUnavailable` | 503 | Persona API is unreachable |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/kyc/inquiries/getInquiry

Returns the details of a specific Persona inquiry.

**Authentication:** Firebase JWT required

#### Request

```http
GET /api/v1/kyc/inquiries/getInquiry?PersonaInquiryId=inq_abc123 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `PersonaInquiryId` | string | Yes | Persona inquiry ID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Inquiry": {
        "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440000",
        "PersonaInquiryId": "inq_abc123",
        "PersonaTemplateId": "itmpl_default",
        "Status": "pending",
        "Decision": "",
        "AttemptNumber": 1,
        "CreatedAt": "2026-04-15T08:00:00Z",
        "UpdatedAt": "2026-04-15T08:00:00Z"
    },
    "ErrorMessage": ""
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInquiryNotFound` | 404 | Inquiry does not exist |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/kyc/me/getStatus

Returns the KYC status of the currently authenticated user.

**Authentication:** Firebase JWT required

#### Request

```http
GET /api/v1/kyc/me/getStatus HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
```

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "IdentityVerified": true,
    "DriverVerified": false,
    "PendingReviews": [
        {
            "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440001",
            "PersonaInquiryId": "inq_xyz789",
            "Status": "pending",
            "AttemptNumber": 1,
            "SessionExpiresAt": "2026-04-15T09:00:00Z"
        }
    ],
    "LatestRejection": null,
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `IdentityVerified` | boolean | True if identity document is approved |
| `DriverVerified` | boolean | True if driver license is approved |
| `PendingReviews` | array | Active inquiries awaiting completion |
| `LatestRejection` | object\|null | Most recent rejection detail |

---

### POST /api/v1/kyc/inquiries/resume

Resumes an interrupted Persona verification session.

**Authentication:** Firebase JWT required

#### Request

```http
POST /api/v1/kyc/inquiries/resume HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "PersonaInquiryId": "inq_abc123"
}
```

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440000",
    "PersonaInquiryId": "inq_abc123",
    "SessionToken": "eyJhbGci...",
    "SessionExpiresAt": "2026-04-15T09:00:00Z",
    "Status": "pending",
    "AttemptNumber": 1,
    "ErrorMessage": ""
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInquiryNotFound` | 404 | Inquiry does not exist |
| `ErrorInquiryNotResumable` | 410 | Inquiry cannot be resumed (wrong status) |
| `ErrorPersonaUnavailable` | 503 | Persona API is unreachable |
| `ErrorInternalServer` | 500 | Internal server error |

---

### POST /api/v1/kyc/webhooks/persona

Receives and processes a Persona webhook event. Called by Persona's servers after an inquiry is completed.

**Authentication:** Not required (public — Persona webhook signature verified internally)

#### Request

```http
POST /api/v1/kyc/webhooks/persona HTTP/1.1
Host: api.tissi-mah.com
Content-Type: application/json

{
    "Signature": "t=...,v1=...",
    "PersonaInquiryId": "inq_abc123",
    "WebhookEventType": "inquiry.completed",
    "OccurredAt": "2026-04-15T08:30:00Z",
    "PersonaRawPayload": "<base64-encoded JSON>"
}
```

#### Response (Success)

```json
{ "Success": true, "ErrorMessage": "" }
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidSignature` | 401 | Invalid Persona webhook signature |
| `ErrorInvalidInput` | 400 | Malformed payload |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/kyc/admin/reviews/getReviews

Lists all reviews with optional filters. Admin only.

**Authentication:** Firebase JWT required (admin)

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserId` | string | No | Filter by user ID |
| `Status` | string | No | Filter by status (`pending`, `completed`, …) |
| `Decision` | string | No | Filter by decision (`approved`, `rejected`, …) |
| `Index` | integer | No | Page index (0-based, 20 items/page) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Reviews": [
        {
            "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440000",
            "PersonaInquiryId": "inq_abc123",
            "Status": "completed",
            "Decision": "approved",
            "AttemptNumber": 1,
            "CreatedAt": "2026-04-15T08:00:00Z"
        }
    ],
    "ErrorMessage": ""
}
```

---

### GET /api/v1/kyc/admin/reviews/getReview

Returns the full detail of a single review. Admin only.

**Authentication:** Firebase JWT required (admin)

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `ReviewId` | string | Yes | Review UUID |

---

### POST /api/v1/kyc/admin/reviews/override

Manually overrides a review decision. Admin only.

**Authentication:** Firebase JWT required (admin)

#### Request

```http
POST /api/v1/kyc/admin/reviews/override HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440000",
    "Decision": "rejected",
    "ReasonRejection": "Document expired",
    "RejectionDetails": "Expiration date: 2025-01-01",
    "Notes": "Manual review by support agent"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ReviewId` | string | Yes | Review to override |
| `Decision` | string | Yes | `approved` or `rejected` |
| `ReasonRejection` | string | Conditional | Required if `Decision = rejected` |
| `RejectionDetails` | string | No | Additional rejection details |
| `Notes` | string | No | Internal notes |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorReviewNotFound` | 404 | Review does not exist |
| `ErrorReviewNotOverridable` | 410 | Review cannot be overridden in its current state |
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/kyc/health

Health check endpoint.

**Authentication:** Not required (public)

#### Response

```json
{
    "Status": "SERVING",
    "Version": "1.0.0",
    "Timestamp": 1742040000
}
```

---

## KYC Statuses

| Status | Description |
|--------|-------------|
| `pending` | Inquiry created, waiting for user to complete the flow |
| `completed` | User completed the flow, waiting for Persona decision |
| `approved` | Document approved by Persona |
| `rejected` | Document rejected |
| `expired` | Session expired before completion |

## Error Reference

| Error | HTTP Code | gRPC Code | Description |
|-------|-----------|-----------|-------------|
| `ErrorInvalidInput` | 400 | INVALID_ARGUMENT (3) | Missing or invalid fields |
| `ErrorMissingDocumentID` | 400 | INVALID_ARGUMENT (3) | `DocumentId` absent or empty in CreateInquiry |
| `ErrorDocumentMismatch` | 400 | INVALID_ARGUMENT (3) | Document type mismatch or document does not belong to the user |
| `ErrorInquiryNotFound` | 404 | NOT_FOUND (5) | Inquiry does not exist |
| `ErrorReviewNotFound` | 404 | NOT_FOUND (5) | Review does not exist |
| `ErrorInquiryAlreadyActive` | 409 | ALREADY_EXISTS (6) | Inquiry already in progress |
| `ErrorPermissionDenied` | 403 | PERMISSION_DENIED (7) | Not authorized |
| `ErrorInquiryNotResumable` | 410 | FAILED_PRECONDITION (9) | Inquiry cannot be resumed |
| `ErrorReviewNotOverridable` | 410 | FAILED_PRECONDITION (9) | Review cannot be overridden |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Internal server error |
| `ErrorFileServiceUnavailable` | 503 | UNAVAILABLE (14) | file-service unreachable |
| `ErrorPersonaUnavailable` | 503 | UNAVAILABLE (14) | Persona API unreachable |
| `ErrorInvalidSignature` | 401 | UNAUTHENTICATED (16) | Invalid webhook signature |
