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

### POST /api/v1/file/uploadIdDocument

Uploads one or more identity documents for a user profile. Files are sent as base64-encoded bytes in the JSON body. The service uploads each file to S3/MinIO and stores the URL in the database.

**Authentication:** Required (Firebase JWT)

> **Security note:** The user identity is resolved server-side from the Firebase JWT (`x-firebase-uid`). Any `UserID` field in the request body is ignored. The caller cannot impersonate another user.

#### Request

```http
POST /api/v1/file/uploadIdDocument HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "DocumentType": "IDCard",
    "IDCardRecto": "<base64-encoded bytes>",
    "IDCardVerso": "<base64-encoded bytes>"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DocumentType` | string | Yes | `IDCard`, `Passport`, or `DriverLicence` |
| `IDCardRecto` | bytes (base64) | If `IDCard` | Front of ID card |
| `IDCardVerso` | bytes (base64) | If `IDCard` | Back of ID card |
| `DriverLicence` | bytes (base64) | If `DriverLicence` | Driver's licence scan (single image) |
| `Passport` | bytes (base64) | If `Passport` | Passport scan |

> **Note:** grpc-gateway automatically handles base64 encoding/decoding for `bytes` fields. Send files as base64 strings in the JSON body.

> **Driver licence sharing:** the driver's licence is a single user-level document shared between identity verification and vehicle verification. Once submitted here (or via `uploadVehicleDocuments`), it covers all the user's vehicles — approval or rejection applies to both contexts.

#### Required fields per DocumentType

| DocumentType | Required fields |
|--------------|----------------|
| `IDCard` | `IDCardRecto` + `IDCardVerso` |
| `Passport` | `Passport` |
| `DriverLicence` | `DriverLicence` |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Success": true,
    "ErrorMessage": "",
    "Documents": [
        {
            "DocumentID": "d-550e8400-e29b-41d4-a716-446655440001",
            "DocumentURL": "https://storage.example.com/IDCard/uuid/d-550e8400.jpg",
            "DocumentType": "IDCard",
            "DocumentName": "dupont_jean_20260425_143052_id_card_recto"
        },
        {
            "DocumentID": "d-550e8400-e29b-41d4-a716-446655440002",
            "DocumentURL": "https://storage.example.com/IDCard/uuid/d-550e8400.jpg",
            "DocumentType": "IDCard",
            "DocumentName": "dupont_jean_20260425_143052_id_card_verso"
        }
    ]
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Success` | boolean | `true` if all files uploaded successfully |
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `Documents` | array | Uploaded documents — 1 item for `Passport` / `DriverLicence`, 2 for `IDCard` (recto + verso) |
| `Documents[].DocumentID` | string | Document UUID — use this ID when calling `/kyc/inquiries/add` |
| `Documents[].DocumentURL` | string | S3/MinIO URL of the uploaded file |
| `Documents[].DocumentType` | string | Document type |
| `Documents[].DocumentName` | string | Generated name: `{lastname}_{firstname}_{YYYYMMDD}_{HHMMSS}_{type}` (e.g. `dupont_jean_20260425_143052_id_card_recto`) |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorInvalidDocumentType` | 400 | Unknown `DocumentType` value |
| `ErrorMissingDocumentFiles` | 400 | Required files missing for the given `DocumentType` |
| `ErrorUserServiceUnavailable` | 503 | user-service unreachable (Firebase UID resolution failed) |
| `ErrorUploadFailed` | 500 | S3/MinIO upload failed |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
# IDCard upload
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/file/uploadIdDocument \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "DocumentType": "IDCard",
    "IDCardRecto": "<base64>",
    "IDCardVerso": "<base64>"
  }'

# Passport upload
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/file/uploadIdDocument \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "DocumentType": "Passport",
    "Passport": "<base64>"
  }'
```

---

### POST /api/v1/file/uploadVehicleDocuments

Uploads the vehicle documents (insurance, registration card) and, if the user does not already have one, the driver's licence. Files are sent as base64-encoded bytes in the JSON body. The service uploads each file to S3/MinIO and stores the URL in the database.

> **Driver licence sharing:** the driver's licence is stored as a **user document** (type `driverLicence`), not a vehicle document. It is shared between identity verification and vehicle verification and covers all the user's vehicles. If the user already submitted a licence (via `uploadIdDocument` or a previous vehicle upload), `DriverLicenceImage` is ignored; otherwise it is required.

**Authentication:** Required (Firebase JWT)

> **Security note:** The user identity is resolved server-side from the Firebase JWT (`x-firebase-uid`). Any `UserID` field in the request body is ignored. The caller cannot impersonate another user.

#### Request

```http
POST /api/v1/file/uploadVehicleDocuments HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "VehicleID": "v-550e8400-e29b-41d4-a716-446655440000",
    "DriverLicenceImage": "<base64-encoded bytes>",
    "Assurance": "<base64-encoded bytes>",
    "VehicleRegistration": "<base64-encoded bytes>"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `VehicleID` | string | Yes | Vehicle ID |
| `DriverLicenceImage` | bytes (base64) | If no current licence | Driver's licence scan — ignored if the user already has a current `driverLicence` document, required otherwise |
| `Assurance` | bytes (base64) | Yes | Insurance document |
| `VehicleRegistration` | bytes (base64) | Yes | Vehicle registration card (carte grise) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Success": true,
    "ErrorMessage": "",
    "Documents": [
        {
            "DocumentID": "d-550e8400-e29b-41d4-a716-000000000001",
            "DocumentURL": "https://storage.example.com/driverLicence/uuid/d-000001.jpg",
            "DocumentType": "driverLicence",
            "DocumentName": "dupont_jean_20260425_143052_driver_licence"
        },
        {
            "DocumentID": "d-550e8400-e29b-41d4-a716-000000000002",
            "DocumentURL": "https://storage.example.com/insurance/uuid/d-000002.jpg",
            "DocumentType": "insurance",
            "DocumentName": "dupont_jean_20260425_143052_assurance"
        },
        {
            "DocumentID": "d-550e8400-e29b-41d4-a716-000000000003",
            "DocumentURL": "https://storage.example.com/registrationCard/uuid/d-000003.jpg",
            "DocumentType": "registrationCard",
            "DocumentName": "dupont_jean_20260425_143052_vehicle_registration"
        }
    ]
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Success` | boolean | `true` if all files uploaded successfully |
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `Documents` | array | 2 or 3 items in order: `driverLicence` (only if just uploaded), `insurance`, `registrationCard` |
| `Documents[].DocumentID` | string | Document UUID — use this ID when calling `/kyc/inquiries/add` |
| `Documents[].DocumentURL` | string | S3/MinIO URL of the uploaded file |
| `Documents[].DocumentType` | string | `driverLicence`, `insurance`, or `registrationCard` |
| `Documents[].DocumentName` | string | Generated name: `{lastname}_{firstname}_{YYYYMMDD}_{HHMMSS}_{type}` |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorInvalidDocumentType` | 400 | A required file is missing or `VehicleID` is empty |
| `ErrorDriverLicenceRequired` | 400 | No current driver licence on file and `DriverLicenceImage` not provided |
| `ErrorUserServiceUnavailable` | 503 | user-service unreachable (Firebase UID or profile resolution failed) |
| `ErrorUploadFailed` | 500 | S3/MinIO upload failed |
| `ErrorInternalServer` | 500 | Internal error |

#### Example (cURL)

```bash
curl -X POST https://api.tissimah.kpeewu.dev/api/v1/file/uploadVehicleDocuments \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "VehicleID": "v-550e8400-e29b-41d4-a716-446655440000",
    "DriverLicenceImage": "<base64>",
    "Assurance": "<base64>",
    "VehicleRegistration": "<base64>"
  }'
```

---

### PATCH /api/v1/file/changeDocument

Replaces the file of an existing document with a new one. The old file is deleted from S3/MinIO and a new document record is created.

**Authentication:** Required (Firebase JWT)

#### Request

```http
PATCH /api/v1/file/changeDocument HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "FileID": "d-550e8400-e29b-41d4-a716-446655440001",
    "NewDocument": "<base64-encoded bytes>"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserID` | string | Yes | User UUID (internal ID) — ownership check |
| `FileID` | string | Yes | ID of the document to replace |
| `NewDocument` | bytes (base64) | Yes | Replacement file |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Success": true,
    "ErrorMessage": "",
    "Document": {
        "DocumentID": "d-new-uuid-...",
        "DocumentURL": "https://storage.example.com/IDCard/uuid/d-new-uuid.jpg",
        "DocumentType": "IDCard",
        "DocumentName": "id_card_recto"
    }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Success` | boolean | `true` if the replacement succeeded |
| `ErrorMessage` | string | Error identifier if failed, `""` if success |
| `Document` | object | Newly created document |
| `Document.DocumentID` | string | New document UUID |
| `Document.DocumentURL` | string | S3/MinIO URL of the new file |
| `Document.DocumentType` | string | Document type |
| `Document.DocumentName` | string | Document name |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorDocumentNotFound` | 404 | `FileID` does not exist |
| `ErrorUploadFailed` | 500 | S3/MinIO upload failed |
| `ErrorInternalServer` | 500 | Internal error |

---

### GET /api/v1/file/getDocument

Retrieves a document file by ID. Access is controlled by ownership: either the owning user or a support agent can retrieve the file.

**Authentication:** Required (Firebase JWT)

#### Request

```http
GET /api/v1/file/getDocument?FileID=d-550e8400-e29b-41d4-a716-446655440001&UserID=8b1d4173-d563-4f81-aeb1-8bf565816545 HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `FileID` | string | Yes | Document ID to retrieve |
| `UserID` | string | One required | User UUID — verifies ownership |
| `SupportID` | string | One required | Support agent ID — bypasses ownership check |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "File": {
        "FileID": "d-550e8400-e29b-41d4-a716-446655440001",
        "FileURL": "https://storage.example.com/IDCard/uuid/d-550e8400.jpg",
        "FileType": "image/jpeg"
    }
}
```

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorDocumentNotFound` | 404 | Document does not exist |
| `ErrorPermissionDenied` | 403 | `UserID` is not the owner of the document |
| `ErrorInternalServer` | 500 | Internal error |

---

### POST /api/v1/file/deleteFile

Deletes a file from S3/MinIO and removes its metadata from the database. The caller must be the owner of the file.

**Authentication:** Required (Firebase JWT)

#### Request

```http
POST /api/v1/file/deleteFile HTTP/1.1
Host: api.tissimah.kpeewu.dev
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "FileID": "d-550e8400-e29b-41d4-a716-446655440001"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `UserID` | string | Yes | User UUID — ownership check |
| `FileID` | string | Yes | Document ID to delete |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Success": true,
    "ErrorMessage": ""
}
```

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorDocumentNotFound` | 404 | Document does not exist |
| `ErrorPermissionDenied` | 403 | `UserID` is not the owner of the document |
| `ErrorInternalServer` | 500 | Internal error |

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
| `mime_type` | string | Yes | MIME type — see [Supported MIME Types](#supported-mime-types) |
| `file_size_bytes` | int64 | Yes | Total file size in bytes (max 10MB) |
| `document_number` | string | No | Document number |
| `issuing_country` | string | No | Country code (ISO 3166-1 alpha-2) |

#### Valid User Document Types

| Type | Description |
|------|-------------|
| `idCardFront` | Front of ID card |
| `idCardBack` | Back of ID card |
| `passport` | Passport |
| `driverLicence` | Driver's licence (single image, shared identity/vehicle) |
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
| `image/jpg` | .jpg (alias non-standard de `image/jpeg`) |
| `image/png` | .png |
| `image/webp` | .webp |
| `image/heic` | .heic (photos iPhone) |
| `image/heif` | .heif |
| `image/tiff` | .tiff |
| `application/pdf` | .pdf |

Pour `UploadIdDocument`, `UploadVehicleDocuments` et `ChangeDocument`, le MIME
type est détecté côté serveur à partir des bytes (signature magique JPEG / PNG /
WebP / TIFF / PDF via `http.DetectContentType`, signature ISO/IEC 14496-12 pour
HEIC / HEIF). Un fichier dont le type ne peut pas être identifié comme l'un des
formats ci-dessus est rejeté avec `ErrorInvalidMimeType`.

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
| `ErrorUserServiceUnavailable` | 503 | UNAVAILABLE | user-service unreachable (Firebase UID resolution failed) |
| `ErrorInternalServer` | 500 | INTERNAL | Internal server error |
