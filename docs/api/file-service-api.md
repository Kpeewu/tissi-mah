# File Service API

This document describes the API exposed by the file-service. Some endpoints are accessible via HTTP through the api-gateway; others are inter-service gRPC only.

## Overview

The file-service is responsible for:
- Receiving files from mobile clients (base64 JSON) or other services (gRPC streaming)
- Uploading files to S3/MinIO object storage
- Storing document metadata and URLs in PostgreSQL
- Managing document reviews and verification workflows

## Base URL (HTTP endpoints)

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Service Discovery (gRPC inter-service)

| Environment | Address |
|-------------|---------|
| Local | `localhost:50053` |
| Kubernetes | `file-service.default.svc.cluster.local:50053` |

## Authentication

Protected HTTP endpoints require a valid Firebase JWT token in the `Authorization` header.

```
Authorization: Bearer <firebase_id_token>
```

## Error Response Format (HTTP)

All HTTP errors return the appropriate status code with this JSON body:

```json
{
    "ErrorMessage": "ErrorInvalidDocumentType"
}
```

| HTTP Code | Meaning |
|-----------|---------|
| 200 | Success |
| 400 | Invalid request parameters (`INVALID_ARGUMENT`) |
| 401 | Missing or invalid token (`UNAUTHENTICATED`) |
| 404 | Resource not found (`NOT_FOUND`) |
| 500 | Internal server error (`INTERNAL`) |

---

## HTTP Endpoints

### POST /file/uploadIdDocument

Uploads one or more identity documents for a user profile. Files are sent as base64-encoded bytes in the JSON body. The service uploads each file to S3/MinIO and stores the URL in the database.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /file/uploadIdDocument HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "DocumentType": "IDCard",
    "IDCardRecto": "<base64-encoded bytes>",
    "IDCardVerso": "<base64-encoded bytes>"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ProfileID` | string | Yes | User profile ID |
| `DocumentType` | string | Yes | `IDCard`, `Passport`, or `DriverLicence` |
| `IDCardRecto` | bytes (base64) | If `IDCard` | Front of ID card |
| `IDCardVerso` | bytes (base64) | If `IDCard` | Back of ID card |
| `DriverLicenceRecto` | bytes (base64) | If `DriverLicence` | Front of driver's licence |
| `DriverLicenceVerso` | bytes (base64) | If `DriverLicence` | Back of driver's licence |
| `Passport` | bytes (base64) | If `Passport` | Passport scan |

> **Note:** grpc-gateway automatically handles base64 encoding/decoding for `bytes` fields. Send files as base64 strings in the JSON body.

#### Required fields per DocumentType

| DocumentType | Required fields |
|--------------|----------------|
| `IDCard` | `IDCardRecto` + `IDCardVerso` |
| `Passport` | `Passport` |
| `DriverLicence` | `DriverLicenceRecto` + `DriverLicenceVerso` |

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
| `Success` | boolean | `true` if all files uploaded successfully |
| `ErrorMessage` | string | Error identifier if failed, `""` if success |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorInvalidDocumentType` | 400 | Unknown `DocumentType` value |
| `ErrorMissingDocumentFiles` | 400 | Required files missing for the given `DocumentType` |
| `ErrorUploadFailed` | 500 | S3/MinIO upload failed |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
# IDCard upload
curl -X POST https://api.tissimah.kpeewu.dev/file/uploadIdDocument \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "DocumentType": "IDCard",
    "IDCardRecto": "<base64>",
    "IDCardVerso": "<base64>"
  }'

# Passport upload
curl -X POST https://api.tissimah.kpeewu.dev/file/uploadIdDocument \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ProfileID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "DocumentType": "Passport",
    "Passport": "<base64>"
  }'
```

---

## Inter-Service RPCs (gRPC only)

These RPCs are not exposed via HTTP. They are called directly by other services.

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
        "user_id": "8b1d4173-d563-4f81-aeb1-8bf565816545",
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

---

### GetUserDocuments

Retrieves all documents for a specific user.

```protobuf
rpc GetUserDocuments(GetUserDocumentsRequest) returns (GetUserDocumentsResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | string | Yes | User profile ID |

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

```protobuf
rpc GetVehicleDocuments(GetVehicleDocumentsRequest) returns (GetVehicleDocumentsResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vehicle_id` | string | Yes | Vehicle ID |

---

### GetVehicleDocument

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

---

### DeleteVehicleDocument

```protobuf
rpc DeleteVehicleDocument(DeleteDocumentRequest) returns (OperationResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `document_id` | string | Yes | Document ID to delete |

---

### CreateDocumentReview

Creates a review for a document. Exactly one of `user_document_id` or `vehicle_document_id` must be provided.

```protobuf
rpc CreateDocumentReview(CreateDocumentReviewRequest) returns (DocumentReviewResponse);
```

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

| Decision | Document Status | Description |
|----------|----------------|-------------|
| `approved` | `approved` | Document is verified and accepted |
| `rejected` | `rejected` | Document is rejected permanently |
| `resubmission` | `pending` | Document needs to be re-submitted |

---

### GetDocumentReviews

```protobuf
rpc GetDocumentReviews(GetDocumentReviewsRequest) returns (GetDocumentReviewsResponse);
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_document_id` | string | One required | User document ID |
| `vehicle_document_id` | string | One required | Vehicle document ID |

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

Example: `idCardFront/8b1d4173/d-550e8400.jpg`

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
| Max chunk size (streaming) | 64 KB |

## Error Reference

| ErrorMessage | HTTP | gRPC | Description |
|--------------|------|------|-------------|
| `ErrorDocumentNotFound` | 404 | NOT_FOUND | Document does not exist |
| `ErrorInvalidDocumentType` | 400 | INVALID_ARGUMENT | Invalid document type |
| `ErrorMissingDocumentFiles` | 400 | INVALID_ARGUMENT | Required files missing for document type |
| `ErrorInvalidMimeType` | 400 | INVALID_ARGUMENT | Unsupported MIME type |
| `ErrorUploadFailed` | 500 | INTERNAL | S3 upload failed |
| `ErrorFileTooLarge` | 400 | INVALID_ARGUMENT | File exceeds 10MB limit |
| `ErrorReviewNotFound` | 404 | NOT_FOUND | Review does not exist |
| `ErrorInvalidReviewDecision` | 400 | INVALID_ARGUMENT | Invalid review decision |
| `ErrorDataRetrievalFailed` | 500 | INTERNAL | Database query failed |
| `ErrorInternalServer` | 500 | INTERNAL | Internal server error |
