# File Service API

This document describes the gRPC API exposed by the file-service. This service is **inter-service only** and is not exposed via the api-gateway HTTP endpoints.

## Overview

The file-service is responsible for:
- Receiving files from other services via gRPC client-side streaming
- Uploading files to S3/MinIO object storage
- Storing document metadata in PostgreSQL
- Managing document reviews and verification workflows

## Service Discovery

| Environment | Address |
|-------------|---------|
| Local | `localhost:50053` |
| Kubernetes | `file-service.default.svc.cluster.local:50053` |

## Communication Flow

```
client -> api-gateway -> user-service -> file-service -> S3/MinIO
                                      -> file-service -> PostgreSQL (metadata)
```

---

## RPCs

### UploadUserDocument (Client Streaming)

Uploads a user document using client-side streaming. The first message contains metadata, subsequent messages contain binary chunks (64KB max per chunk).

```protobuf
rpc UploadUserDocument(stream UploadUserDocumentRequest) returns (UserDocumentResponse);
```

#### Streaming Protocol

1. **First message:** Send metadata only

```json
{
    "metadata": {
        "user_id": "u-550e8400-e29b-41d4-a716-446655440000",
        "document_name": "id-card-front.jpg",
        "document_type": "idCardFront",
        "mime_type": "image/jpeg",
        "file_size_bytes": 245760,
        "document_number": "AB123456",
        "issuing_country": "TG"
    }
}
```

2. **Subsequent messages:** Send binary chunks

```json
{
    "chunk": "<binary data, max 64KB>"
}
```

#### Metadata Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | string | Yes | User profile ID |
| `document_name` | string | Yes | File name |
| `document_type` | string | Yes | Document type (see below) |
| `mime_type` | string | Yes | MIME type (image/jpeg, image/png, application/pdf) |
| `file_size_bytes` | int64 | Yes | Total file size in bytes (max 10MB) |
| `document_number` | string | No | Document number |
| `issuing_country` | string | No | Country code (ISO 3166-1 alpha-2) |

#### Valid User Document Types

| Type | Description |
|------|-------------|
| `idCardFront` | Front of ID card |
| `idCardBack` | Back of ID card |
| `passport` | Passport |
| `driverLicenceFront` | Front of driver's licence |
| `driverLicenceBack` | Back of driver's licence |
| `profilePicture` | Profile picture |

#### Response

```json
{
    "document_id": "d-550e8400-e29b-41d4-a716-446655440000",
    "user_id": "u-550e8400",
    "document_name": "id-card-front.jpg",
    "document_type": "idCardFront",
    "document_url": "http://minio:9000/tissi-mah-files/idCardFront/u-550e8400/d-550e8400.jpg",
    "file_size_bytes": 245760,
    "mime_type": "image/jpeg",
    "document_number": "AB123456",
    "issuing_country": "TG",
    "status": "pending",
    "is_current": true,
    "uploaded_at": "2026-02-28T10:00:00Z",
    "updated_at": "2026-02-28T10:00:00Z"
}
```

---

### UploadVehicleDocument (Client Streaming)

Uploads a vehicle document. Same streaming protocol as UploadUserDocument.

```protobuf
rpc UploadVehicleDocument(stream UploadVehicleDocumentRequest) returns (VehicleDocumentResponse);
```

#### Metadata Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vehicle_id` | string | Yes | Vehicle ID |
| `document_name` | string | Yes | File name |
| `document_type` | string | Yes | `insurance` or `registrationCard` |
| `mime_type` | string | Yes | MIME type |
| `file_size_bytes` | int64 | Yes | Total file size (max 10MB) |
| `document_number` | string | No | Document number |
| `issuing_authority` | string | No | Issuing authority name |

#### Valid Vehicle Document Types

| Type | Description |
|------|-------------|
| `insurance` | Vehicle insurance document |
| `registrationCard` | Vehicle registration card |

---

### GetUserDocuments

Retrieves all documents for a specific user.

```protobuf
rpc GetUserDocuments(GetUserDocumentsRequest) returns (GetUserDocumentsResponse);
```

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | string | Yes | User profile ID |

#### Response

```json
{
    "documents": [
        {
            "document_id": "d-123",
            "user_id": "u-550e8400",
            "document_name": "id-card-front.jpg",
            "document_type": "idCardFront",
            "document_url": "http://...",
            "file_size_bytes": 245760,
            "mime_type": "image/jpeg",
            "status": "approved",
            "is_current": true,
            "uploaded_at": "2026-02-28T10:00:00Z",
            "updated_at": "2026-02-28T10:00:00Z"
        }
    ]
}
```

---

### GetUserDocument

Retrieves a specific user document by ID.

```protobuf
rpc GetUserDocument(GetDocumentByIDRequest) returns (UserDocumentResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `document_id` | string | Yes | Document ID |

---

### GetCurrentUserDocument

Retrieves the current (active) document for a user and document type.

```protobuf
rpc GetCurrentUserDocument(GetCurrentUserDocumentRequest) returns (UserDocumentResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | string | Yes | User profile ID |
| `document_type` | string | Yes | Document type (e.g., `idCardFront`) |

---

### GetVehicleDocuments

Retrieves all documents for a specific vehicle.

```protobuf
rpc GetVehicleDocuments(GetVehicleDocumentsRequest) returns (GetVehicleDocumentsResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vehicle_id` | string | Yes | Vehicle ID |

---

### GetVehicleDocument

Retrieves a specific vehicle document by ID.

```protobuf
rpc GetVehicleDocument(GetDocumentByIDRequest) returns (VehicleDocumentResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `document_id` | string | Yes | Document ID |

---

### DeleteUserDocument

Deletes a user document (file from S3 + metadata from database).

```protobuf
rpc DeleteUserDocument(DeleteDocumentRequest) returns (OperationResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `document_id` | string | Yes | Document ID to delete |

#### Response

```json
{
    "success": true
}
```

---

### DeleteVehicleDocument

Deletes a vehicle document (file from S3 + metadata from database).

```protobuf
rpc DeleteVehicleDocument(DeleteDocumentRequest) returns (OperationResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `document_id` | string | Yes | Document ID to delete |

---

### CreateDocumentReview

Creates a review for a document (user or vehicle). Exactly one of `user_document_id` or `vehicle_document_id` must be provided.

```protobuf
rpc CreateDocumentReview(CreateDocumentReviewRequest) returns (DocumentReviewResponse);
```

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_document_id` | string | One required | User document ID |
| `vehicle_document_id` | string | One required | Vehicle document ID |
| `decision` | string | Yes | `approved`, `rejected`, or `resubmission` |
| `reason_rejection` | string | No | Rejection reason code |
| `rejection_details` | string | No | Detailed rejection explanation |
| `reviewed_by` | string | Yes | Reviewer identifier |
| `reviewed_by_type` | string | Yes | `manual` or `automatic` |
| `notes` | string | No | Internal review notes |
| `extracted_data` | bytes | No | JSON data extracted from document |

#### Decision Effects

| Decision | Document Status | Description |
|----------|----------------|-------------|
| `approved` | `approved` | Document is verified and accepted |
| `rejected` | `rejected` | Document is rejected permanently |
| `resubmission` | `pending` | Document needs to be re-submitted |

#### Response

```json
{
    "review_id": "r-550e8400",
    "user_document_id": "d-123",
    "vehicle_document_id": "",
    "decision": "approved",
    "reason_rejection": "",
    "rejection_details": "",
    "reviewed_by": "admin@tissi-mah.com",
    "reviewed_by_type": "manual",
    "reviewed_at": "2026-02-28T10:30:00Z",
    "notes": "Document verified"
}
```

---

### GetDocumentReviews

Retrieves all reviews for a document. Provide either `user_document_id` or `vehicle_document_id`.

```protobuf
rpc GetDocumentReviews(GetDocumentReviewsRequest) returns (GetDocumentReviewsResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_document_id` | string | One required | User document ID |
| `vehicle_document_id` | string | One required | Vehicle document ID |

#### Response

```json
{
    "reviews": [
        {
            "review_id": "r-550e8400",
            "decision": "approved",
            "reviewed_by": "admin@tissi-mah.com",
            "reviewed_by_type": "manual",
            "reviewed_at": "2026-02-28T10:30:00Z"
        }
    ]
}
```

---

### Health

Health check endpoint.

```protobuf
rpc Health(HealthRequest) returns (HealthResponse);
```

#### Response

```json
{
    "status": "SERVING",
    "version": "v1.0.0",
    "timestamp": 1709136000
}
```

---

## Document Status Lifecycle

```
pending -> underReview -> approved
                       -> rejected
                       -> pending (resubmission)
                       -> expired
```

| Status | Description |
|--------|-------------|
| `pending` | Document uploaded, awaiting review |
| `underReview` | Document is being reviewed |
| `approved` | Document verified and accepted |
| `rejected` | Document rejected |
| `expired` | Document has expired |

## S3 Storage

Documents are stored in S3/MinIO with the following key pattern:

```
{document_type}/{user_id|vehicle_id}/{document_id}.{extension}
```

Example: `idCardFront/u-550e8400/d-123.jpg`

## Supported MIME Types

| MIME Type | Extension |
|-----------|-----------|
| `image/jpeg` | .jpg, .jpeg |
| `image/png` | .png |
| `application/pdf` | .pdf |

## Limits

| Limit | Value |
|-------|-------|
| Max file size | 10 MB |
| Max chunk size | 64 KB |

## Error Codes

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrorDocumentNotFound` | NOT_FOUND (5) | Document does not exist |
| `ErrorInvalidDocumentType` | INVALID_ARGUMENT (3) | Invalid document type |
| `ErrorInvalidMimeType` | INVALID_ARGUMENT (3) | Unsupported MIME type |
| `ErrorUploadFailed` | INTERNAL (13) | S3 upload failed |
| `ErrorFileTooLarge` | INVALID_ARGUMENT (3) | File exceeds 10MB limit |
| `ErrorReviewNotFound` | NOT_FOUND (5) | Review does not exist |
| `ErrorInvalidReviewDecision` | INVALID_ARGUMENT (3) | Invalid review decision |
| `ErrorDataRetrievalFailed` | INTERNAL (13) | Database query failed |
| `ErrorInternalServer` | INTERNAL (13) | Internal server error |
