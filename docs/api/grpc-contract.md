# gRPC Contracts

This document describes the gRPC contracts used for inter-service communication in Tissi-Mah.

## Table of Contents

1. [Overview](#overview)
2. [Auth Service](#auth-service)
3. [User Service](#user-service)
4. [Rating Service](#rating-service)
5. [File Service](#file-service)
6. [Vehicle Service](#vehicle-service)
7. [Trips Service](#trips-service)
8. [KYC Service](#kyc-service)
9. [Booking Service](#booking-service)

---

## Overview

### Communication Pattern

All inter-service communication uses **gRPC** over HTTP/2 with Protocol Buffers for serialization.

```
┌─────────────┐       gRPC        ┌─────────────┐
│  Service A  │ ────────────────► │  Service B  │
│             │ ◄──────────────── │             │
└─────────────┘    (protobuf)     └─────────────┘
```

### Service Discovery

Services discover each other via Kubernetes DNS:

```
<service-name>.<namespace>.svc.cluster.local:<port>
```

### Service Ports

| Service | Port | Transport | Exposure |
|---------|------|-----------|----------|
| api-gateway | 8080 | HTTP | External (LoadBalancer) |
| auth-service | 50051 | gRPC | Internal + api-gateway |
| user-service | 50052 | gRPC | Internal + api-gateway |
| file-service | 50053 | gRPC | Internal only |
| rating-service | 50054 | gRPC | Internal + api-gateway |
| vehicle-service | 50055 | gRPC | Internal + api-gateway |
| trips-service | 50056 | gRPC | Internal + api-gateway |
| kyc-service | 50057 | gRPC | Internal + api-gateway |
| booking-service | 50058 | gRPC | Internal + api-gateway |

### Authentication Flow

```
Client (Bearer JWT) -> api-gateway (validates Firebase JWT)
    -> Sets header: x-firebase-uid
    -> grpc-gateway converts to gRPC metadata
    -> service middleware reads x-firebase-uid from metadata
    -> ctx.Value(middleware.FirebaseIDKey)
```

### Error Handling

gRPC errors use standard status codes with sentinel error messages:

```go
// Server side
return nil, status.Error(codes.NotFound, "ErrUserNotFound")

// Client side
if st, ok := status.FromError(err); ok {
    code := st.Code()        // codes.NotFound
    msg := st.Message()      // "ErrUserNotFound"
}
```

### Proto File Organization

```
services/
├── auth-service/proto/       auth.proto      → port 50051
├── user-service/proto/       user.proto      → port 50052
├── file-service/proto/       file.proto      → port 50053
├── rating-service/proto/     rating.proto    → port 50054
├── vehicle-service/proto/    vehicle.proto   → port 50055
├── trips-service/proto/      trip.proto      → port 50056
├── kyc-service/proto/        kyc.proto       → port 50057
├── booking-service/proto/    booking.proto   → port 50058
└── api-gateway/proto/
    ├── *.proto   (synced from each service via make proto-sync)
    └── gen/      (grpc-gateway stubs — HTTP transcoding)
```

---

## Auth Service

**Proto:** `services/auth-service/proto/auth.proto`
**Package:** `auth`
**Go package:** `github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen;auth`
**Address:** `auth-service:50051`

### RPCs

| RPC | Type | HTTP Route | Auth | Description |
|-----|------|------------|------|-------------|
| `Login` | Unary | `POST /api/v1/auth/login` | JWT | Check if account exists |
| `CreateAccount` | Unary | `POST /api/v1/auth/createAccount` | JWT | Create new account |
| `CheckPhoneNumber` | Unary | `POST /api/v1/auth/checkPhoneNumber` | Public | Check phone availability |
| `CheckEmail` | Unary | `POST /api/v1/auth/checkEmail` | Public | Check email availability |
| `DeleteAccount` | Unary | `DELETE /api/v1/auth/deleteAccount` | JWT | Delete account (GDPR) |
| `GetAuthInfo` | Unary | N/A | Inter-service | Get auth info by AuthID |
| `Health` | Unary | `GET /api/v1/auth/health` | Public | Health check |

### Messages

```protobuf
// --- Client-facing ---

message LoginRequest {}
message LoginResponse {
    string ErrorMessage = 1;
    bool Exists = 2;
    UserPreview User = 3;
}

message CreateAccountRequest {
    string Name = 1;
    string FirstName = 2;
    string Email = 3;              // optional
    string PhoneNumber = 4;        // optional
    string ProfileImageURL = 5;    // optional
}
message CreateAccountResponse {
    string ErrorMessage = 1;
    UserPreview User = 2;
}

message CheckPhoneNumberRequest { string PhoneNumber = 1; }
message CheckEmailRequest { string Email = 1; }
message CheckPhoneOrEmailResponse {
    string ErrorMessage = 1;
    bool IsAvailable = 2;
}

message DeleteAccountRequest {}
message AuthServerResponse {
    string ErrorMessage = 1;
    bool Success = 2;
}

// --- Inter-service ---

message GetAuthInfoRequest { string AuthID = 1; }
message GetAuthInfoResponse {
    string AuthID = 1;
    string Email = 2;
    string PhoneNumber = 3;
    bool IsActive = 4;
    bool IsSuspended = 5;
    string SuspensionEndDate = 6;
}

// --- Shared ---

message UserPreview {
    string AuthID = 1;
    string UserID = 2;
    string Name = 3;
    string FirstName = 4;
    string Email = 5;
    string PhoneNumber = 6;
    string ProfileImageURL = 7;
}

message HealthRequest {}
message HealthResponse {
    string Status = 1;
    string Version = 2;
    int64 Timestamp = 3;
}
```

### Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrMissingFirebaseID` | UNAUTHENTICATED | Firebase ID not in metadata |
| `ErrAccountNotFound` | NOT_FOUND | Account does not exist |
| `ErrAccountAlreadyExists` | FAILED_PRECONDITION | Account already exists |
| `ErrAccountSuspended` | UNAUTHENTICATED | Account is suspended |
| `ErrAccountDeactivated` | UNAUTHENTICATED | Account was deleted |
| `ErrPhoneNumberTaken` | FAILED_PRECONDITION | Phone number in use |
| `ErrEmailTaken` | FAILED_PRECONDITION | Email in use |
| `ErrInvalidPhoneNumber` | INVALID_ARGUMENT | Invalid phone format |
| `ErrInvalidEmail` | INVALID_ARGUMENT | Invalid email format |

### Metadata

| Key | Source | Description |
|-----|--------|-------------|
| `x-firebase-uid` | api-gateway | Firebase UID extracted from JWT |

---

## User Service

**Proto:** `services/user-service/proto/user.proto`
**Package:** `user`
**Go package:** `github.com/Kpeewu/tissi-mah/services/user-service/proto/gen;user`
**Address:** `user-service:50052`

### RPCs

| RPC | Type | HTTP Route | Auth | Description |
|-----|------|------------|------|-------------|
| `CreateUser` | Unary | N/A | Inter-service | Create user profile |
| `GetUserByAuthID` | Unary | N/A | Inter-service | Get profile by auth ID |
| `GetMyProfile` | Unary | `POST /api/v1/user/me` | JWT | Get own full profile |
| `CreateDriverAccount` | Unary | `PATCH /api/v1/userProfile/createDriverAccount` | JWT | Activate driver |
| `AddTripPreferences` | Unary | `POST /api/v1/userProfile/addTripPreferences` | JWT | Set preferences |
| `UpdateProfile` | Unary | `PATCH /api/v1/userProfile/updateProfile` | JWT | Update profile |
| `Health` | Unary | `GET /api/v1/user/health` | Public | Health check |

### Messages

```protobuf
// --- Inter-service ---

message CreateUserRequest {
    string AuthID = 1;
    string Name = 2;
    string FirstName = 3;
    string ProfilePhotoURL = 4;    // optional
    string FirebaseID = 5;
}

message GetUserByAuthIDRequest { string AuthID = 1; }

message UserProfileResponse {
    string UserID = 1;
    string AuthID = 2;
    string Name = 3;
    string FirstName = 4;
    string Gender = 5;
    string DateOfBirth = 6;
    string Bio = 7;
    bool HasProfileImage = 8;
    string ProfileImageURL = 9;
    bool IsDriver = 10;
    bool IsPassenger = 11;
    bool IsDriverProfileVerified = 12;
    bool IsPassengerProfileVerified = 13;
    repeated TripPreference TripPreferences = 14;
    string IDCardExpirationDate = 15;
    string DriveLicenceExpirationDate = 16;
}

// --- Client-facing ---

message GetMyProfileRequest {}
message GetMyProfileResponse {
    string ErrorMessage = 1;
    FullUserProfile User = 2;
}

message CreateDriverAccountRequest {
    string ProfileID = 1;
    bool CreateDriverAccount = 2;
}

message AddTripPreferencesRequest {
    string ProfileID = 1;
    repeated TripPreference Preferences = 2;
}

message UpdateProfileRequest {
    string ProfileID = 1;
    optional string FirstName = 2;
    optional string LastName = 3;
    optional string BirthDate = 4;
    optional string Email = 5;
    optional string PhoneNumber = 6;
    reserved 7;
    reserved "ProfilePictureURL";
}
message UpdateProfileResponse {
    string ErrorMessage = 1;
    FullUserProfile User = 2;
}

// --- Shared ---

message FullUserProfile {
    string AuthID = 1;
    string ProfileID = 2;
    string Name = 3;
    string FirstName = 4;
    string Gender = 5;
    string DateOfBirth = 6;
    string Bio = 7;
    string Email = 8;
    string PhoneNumber = 9;
    string ProfileImageURL = 10;
    bool HasProfileImage = 11;
    bool IsDriver = 12;
    bool IsPassenger = 13;
    bool IsDriverProfileVerified = 14;
    bool IsPassengerProfileVerified = 15;
    bool IsActive = 16;
    bool IsSuspended = 17;
    string SuspensionEndDate = 18;
    repeated TripPreference TripPreferences = 19;
    string IDCardExpirationDate = 20;
    string DriveLicenceExpirationDate = 21;
    repeated UserFile UserFiles = 22;
}

message TripPreference {
    string Preference = 1;
    bool IsAllowed = 2;
}

message UserFile {
    string FileID = 1;
    string FileURL = 2;
    string FileType = 3;
}

message OperationResponse {
    string ErrorMessage = 1;
    bool Success = 2;
}
```

### Metadata

| Key | Source | Description |
|-----|--------|-------------|
| `x-firebase-uid` | api-gateway | Firebase UID extracted from JWT |

---

## Rating Service

**Proto:** `services/rating-service/proto/rating.proto`
**Package:** `rating`
**Go package:** `github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen;rating`
**Address:** `rating-service:50054`

### RPCs

| RPC | Type | HTTP Route | Auth | Cache | Description |
|-----|------|------------|------|-------|-------------|
| `RateUser` | Unary | `POST /api/v1/ratings/rateUser` | Public | Invalidates | Submit a rating (RaterId in body) |
| `GetUserRatings` | Unary | `GET /api/v1/ratings/user/getUserRatings` | Public | 5 min TTL | Get ratings for a user |
| `GetUserRatingsAverage` | Unary | `GET /api/v1/ratings/user/getUserRatingsAverage` | Public | 5 min TTL | Get user's average rating |
| `UpdateRating` | Unary | `PATCH /api/v1/ratings/updateRating` | Public | Invalidates | Update a rating (RaterId in body) |
| `Health` | Unary | `GET /api/v1/ratings/health` | Public | - | Health check |

### Messages (PascalCase)

```protobuf
// --- Client-facing ---

message RateUserRequest {
    string RaterId = 1;
    string UserRatedId = 2;
    int32 NumberOfStars = 3;
    string Comment = 4;          // optional
}
message RateUserResponse {
    string ErrorMessage = 1;
    RatingDetail Rating = 2;
}

message GetUserRatingsRequest { string UserRatedId = 1; }
message GetUserRatingsResponse {
    string ErrorMessage = 1;
    repeated RatingDetail Ratings = 2;
}

message GetUserRatingsAverageRequest { string UserRatedId = 1; }
message GetUserRatingsAverageResponse {
    string ErrorMessage = 1;
    double Average = 2;          // Arrondi à 1 décimale
    int32 TotalRatings = 3;
}

message UpdateRatingRequest {
    string RatingId = 1;
    string RaterId = 2;
    string UserRatedId = 3;
    int32 NumberOfStars = 4;
    string Comment = 5;          // optional
}
message UpdateRatingResponse {
    string ErrorMessage = 1;
    RatingDetail Rating = 2;
}

// --- Shared ---

message RatingDetail {
    string RatingId = 1;
    string RaterId = 2;
    string UserRatedId = 3;
    int32 NumberOfStars = 4;
    string Comment = 5;
    string CreatedAt = 6;       // ISO 8601 (TIMESTAMPTZ)
    string UpdatedAt = 7;       // ISO 8601 (TIMESTAMPTZ)
}

message HealthRequest {}
message HealthResponse {
    string Status = 1;
    string Version = 2;
    int64 Timestamp = 3;
}
```

### Caching (Redis)

| Route | Strategy | TTL | Key Pattern |
|-------|----------|-----|-------------|
| `GetUserRatings` | Cache-aside | 5 min | `rating:user:{userRatedId}` |
| `GetUserRatingsAverage` | Cache-aside | 5 min | `rating:average:{userRatedId}` |
| `RateUser` | Invalidate | - | Deletes both keys for the rated user |
| `UpdateRating` | Invalidate | - | Deletes both keys for the rated user |

Redis est non-bloquant : si Redis est indisponible, le service fonctionne sans cache.

### Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrorMissingRaterID` | INVALID_ARGUMENT | RaterId is required |
| `ErrorMissingUserRatedID` | INVALID_ARGUMENT | UserRatedId is required |
| `ErrorRatingNotFound` | NOT_FOUND | Rating does not exist |
| `ErrorUserNotFound` | NOT_FOUND | User does not exist in user-service |
| `ErrorRatingAlreadyExists` | ALREADY_EXISTS | Rating already exists for this rater/user pair |
| `ErrorInvalidStars` | INVALID_ARGUMENT | Stars must be between 1 and 5 |
| `ErrorSelfRating` | INVALID_ARGUMENT | Cannot rate yourself |
| `ErrorUnauthorizedAction` | PERMISSION_DENIED | Only the rater can modify |
| `ErrorInternalServer` | INTERNAL | Server error |

### Notes

- Les utilisateurs ne peuvent **pas supprimer** leurs avis
- Le `RaterId` et `UserRatedId` sont des UserIDs validés contre le user-service (doivent exister)
- Tous les endpoints sont publics (pas de JWT Firebase requis pour l'instant)

---

## File Service

**Proto:** `services/file-service/proto/file.proto`
**Package:** `file`
**Go package:** `github.com/Kpeewu/tissi-mah/services/file-service/proto/gen;file`
**Address:** `file-service:50053`

> **Note:** This service is inter-service only. No HTTP routes are exposed via api-gateway.

### RPCs

| RPC | Type | Description |
|-----|------|-------------|
| `UploadUserDocument` | Client streaming | Upload user document (metadata + chunks) |
| `UploadVehicleDocument` | Client streaming | Upload vehicle document (metadata + chunks) |
| `GetUserDocuments` | Unary | Get all documents for a user |
| `GetUserDocument` | Unary | Get document by ID |
| `GetCurrentUserDocument` | Unary | Get current document by user + type |
| `GetVehicleDocuments` | Unary | Get all documents for a vehicle |
| `GetVehicleDocument` | Unary | Get vehicle document by ID |
| `DeleteUserDocument` | Unary | Delete user document |
| `DeleteVehicleDocument` | Unary | Delete vehicle document |
| `CreateDocumentReview` | Unary | Create review for a document |
| `GetDocumentReviews` | Unary | Get reviews for a document |
| `Health` | Unary | Health check |

### Streaming Upload Protocol

The upload RPCs use **client-side streaming**:

1. **First message:** Metadata only (no chunk data)
2. **Subsequent messages:** Binary chunks (max 64KB each)
3. **Server responds** with the created document after all chunks are received

```protobuf
message UploadUserDocumentRequest {
    oneof data {
        UserDocumentMetadata metadata = 1;
        bytes chunk = 2;
    }
}

message UserDocumentMetadata {
    string user_id = 1;
    string document_name = 2;
    string document_type = 3;       // idCardFront, idCardBack, passport, etc.
    string mime_type = 4;           // image/jpeg, image/png, application/pdf
    int64 file_size_bytes = 5;      // max 10MB
    string document_number = 6;    // optional
    string issuing_country = 7;    // optional
}

message UploadVehicleDocumentRequest {
    oneof data {
        VehicleDocumentMetadata metadata = 1;
        bytes chunk = 2;
    }
}

message VehicleDocumentMetadata {
    string vehicle_id = 1;
    string document_name = 2;
    string document_type = 3;        // insurance, registrationCard
    string mime_type = 4;
    int64 file_size_bytes = 5;
    string document_number = 6;     // optional
    string issuing_authority = 7;   // optional
}
```

### Document Types

| Category | Types |
|----------|-------|
| User documents | `idCardFront`, `idCardBack`, `passport`, `driverLicenceFront`, `driverLicenceBack`, `profilePicture` |
| Vehicle documents | `insurance`, `registrationCard` |

### Document Statuses

| Status | Description |
|--------|-------------|
| `pending` | Uploaded, awaiting review |
| `underReview` | Currently being reviewed |
| `approved` | Verified and accepted |
| `rejected` | Rejected |
| `expired` | Document has expired |

### Messages

```protobuf
// --- Read ---

message GetUserDocumentsRequest { string user_id = 1; }
message GetDocumentByIDRequest { string document_id = 1; }
message GetCurrentUserDocumentRequest {
    string user_id = 1;
    string document_type = 2;
}
message GetVehicleDocumentsRequest { string vehicle_id = 1; }
message DeleteDocumentRequest { string document_id = 1; }

// --- Review ---

message CreateDocumentReviewRequest {
    string user_document_id = 1;      // one required
    string vehicle_document_id = 2;   // one required
    string decision = 3;              // approved, rejected, resubmission
    string reason_rejection = 4;      // optional
    string rejection_details = 5;     // optional
    string reviewed_by = 6;
    string reviewed_by_type = 7;      // manual, automatic
    string notes = 8;                 // optional
    bytes extracted_data = 9;         // optional (JSON)
}
message GetDocumentReviewsRequest {
    string user_document_id = 1;      // optional
    string vehicle_document_id = 2;   // optional
}

// --- Responses ---

message UserDocumentResponse {
    string document_id = 1;
    string user_id = 2;
    string document_name = 3;
    string document_type = 4;
    string document_url = 5;
    int64 file_size_bytes = 6;
    string mime_type = 7;
    string document_number = 8;
    string issuing_country = 9;
    string status = 10;
    bool is_current = 11;
    string uploaded_at = 12;
    string updated_at = 13;
}

message VehicleDocumentResponse {
    string document_id = 1;
    string vehicle_id = 2;
    string document_name = 3;
    string document_type = 4;
    string document_url = 5;
    int64 file_size_bytes = 6;
    string mime_type = 7;
    string document_number = 8;
    string issuing_authority = 9;
    string status = 10;
    bool is_current = 11;
    string uploaded_at = 12;
    string updated_at = 13;
}

message GetUserDocumentsResponse { repeated UserDocumentResponse documents = 1; }
message GetVehicleDocumentsResponse { repeated VehicleDocumentResponse documents = 1; }
message OperationResponse { bool success = 1; }

message DocumentReviewResponse {
    string review_id = 1;
    string user_document_id = 2;
    string vehicle_document_id = 3;
    string decision = 4;
    string reason_rejection = 5;
    string rejection_details = 6;
    string reviewed_by = 7;
    string reviewed_by_type = 8;
    string reviewed_at = 9;
    string notes = 10;
}
message GetDocumentReviewsResponse { repeated DocumentReviewResponse reviews = 1; }
```

### S3 Key Format

```
{document_type}/{user_id|vehicle_id}/{document_id}.{extension}
```

Example: `idCardFront/u-550e8400/d-123.jpg`

### Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrorDocumentNotFound` | NOT_FOUND | Document does not exist |
| `ErrorInvalidDocumentType` | INVALID_ARGUMENT | Invalid document type |
| `ErrorInvalidMimeType` | INVALID_ARGUMENT | Unsupported MIME type |
| `ErrorUploadFailed` | INTERNAL | S3 upload failed |
| `ErrorFileTooLarge` | INVALID_ARGUMENT | File exceeds 10MB |
| `ErrorReviewNotFound` | NOT_FOUND | Review does not exist |
| `ErrorInvalidReviewDecision` | INVALID_ARGUMENT | Invalid review decision |

---

## Proto Generation

### Generate Go Code

```bash
# All services
make proto

# Individual service
make proto-auth
make proto-user
make proto-rating
make proto-file

# Sync protos to api-gateway (for grpc-gateway)
make proto-sync
make proto-gateway
```

### Required Tools

```bash
# protoc compiler
brew install protobuf          # macOS
apt-get install protobuf-compiler  # Ubuntu

# Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# grpc-gateway (for api-gateway only)
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

---

## Inter-Service Communication Map

```
auth-service     ──gRPC──> user-service      (CreateUser, GetUserByAuthID)
user-service     ──gRPC──> auth-service      (GetAuthInfo)
user-service     ──gRPC──> file-service      (Upload/Get/Delete documents)
rating-service   ──gRPC──> user-service      (GetUserByUserID — validate user exists)
vehicle-service  ──gRPC──> file-service      (Upload/Get vehicle documents)
kyc-service      ──gRPC──> file-service      (GetDocument — get document for Persona inquiry)
trips-service    ──gRPC──> user-service      (GetDriverName — enrichment)
trips-service    ──gRPC──> vehicle-service   (GetVehicleInfo — enrichment + seat check)
trips-service    ──gRPC──> booking-service   (StartBookingsForWaypoint, CompleteBookingsForWaypoint)
booking-service  ──gRPC──> trips-service     (GetTripByID — validate trip, UpdateAvailableSeats)
booking-service  ──gRPC──> user-service      (GetUserByUserID — validate passenger)

api-gateway      ──gRPC──> auth-service      (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> user-service      (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> rating-service    (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> file-service      (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> vehicle-service   (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> trips-service     (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> kyc-service       (HTTP transcoding via grpc-gateway)
api-gateway      ──gRPC──> booking-service   (HTTP transcoding via grpc-gateway)
```

---

## Vehicle Service

**Proto:** `services/vehicle-service/proto/vehicle.proto`
**Package:** `vehicle`
**Address:** `vehicle-service:50055`

### RPCs

| RPC | Type | HTTP Route | Auth | Description |
|-----|------|------------|------|-------------|
| `AddVehicle` | Unary | `POST /vehicle/add` | JWT | Register a new vehicle |
| `UpdateVehicle` | Unary | `PATCH /vehicle/update` | JWT | Update vehicle info |
| `DeleteVehicle` | Unary | `DELETE /vehicle/delete` | JWT | Soft-delete a vehicle |
| `GetVehicleDetails` | Unary | `GET /vehicle/details` | JWT | Get vehicle by ID |
| `GetUserVehicles` | Unary | `GET /vehicle/getUserVehicles` | JWT | List vehicles for a user |
| `GetVehicleInfo` | Unary | N/A | Inter-service | Minimal info for trips enrichment |
| `Health` | Unary | `GET /vehicle/health` | Public | Health check |

---

## Trips Service

**Proto:** `services/trips-service/proto/trip.proto`
**Package:** `trip`
**Address:** `trips-service:50056`

### RPCs

| RPC | Type | HTTP Route | Auth | Description |
|-----|------|------------|------|-------------|
| `CreateTrip` | Unary | `POST /trip/driver/createTrip` | JWT | Create a new trip |
| `CreateRecurringTrip` | Unary | `POST /trip/driver/createRecurringTrip` | JWT | Schedule a recurring trip |
| `GetTripsPreviews` | Unary | `GET /trip/driver/getTripsPreviews` | JWT | List active trips (paginated) |
| `GetCompletedTripsPreviews` | Unary | `GET /trip/driver/getCompletedTripsPreviews` | JWT | List completed trips (paginated) |
| `ChangeTripDateAndTime` | Unary | `PATCH /trip/driver/changeTripDateAndTime` | JWT | Update departure datetime |
| `ChangeTripVehicle` | Unary | `PATCH /trip/driver/changeTripVehicle` | JWT | Change vehicle |
| `ChangeTripAllowances` | Unary | `PATCH /trip/driver/changeTripAllowances` | JWT | Update luggage/pet/food/smoking flags |
| `ChangeAutoApprove` | Unary | `PATCH /trip/driver/activeAutoApprouve` | JWT | Toggle auto-approve |
| `StartTrip` | Unary | `PATCH /trip/driver/startTrip` | JWT | Start a scheduled trip |
| `EndTrip` | Unary | `PATCH /trip/driver/endTrip` | JWT | End an in-progress trip |
| `ConfirmWaypointArrival` | Unary | `PATCH /trip/driver/confirmWaypointArrival` | JWT | Record arrival at a stop |
| `ConfirmWaypointDeparture` | Unary | `PATCH /trip/driver/confirmWaypointDeparture` | JWT | Record departure from a stop |
| `GetTripByID` | Unary | `GET /trip/getTripByID` | Public | Get trip details + waypoints |
| `UpdateAvailableSeats` | Unary | `PATCH /trip/internal/updateAvailableSeats` | Internal | Update available seat count |
| `Health` | Unary | `GET /trip/health` | Public | Health check |

---

## KYC Service

**Proto:** `services/kyc-service/proto/kyc.proto`
**Package:** `kyc`
**Address:** `kyc-service:50057`

### RPCs

| RPC | Type | HTTP Route | Auth | Description |
|-----|------|------------|------|-------------|
| `CreateInquiry` | Unary | `POST /api/v1/kyc/inquiries/add` | JWT | Start a Persona identity verification |
| `GetInquiry` | Unary | `GET /api/v1/kyc/inquiries/getInquiry` | JWT | Get inquiry detail |
| `GetKYCStatus` | Unary | `GET /api/v1/kyc/me/getStatus` | JWT | Get user's KYC status |
| `ResumeInquiry` | Unary | `POST /api/v1/kyc/inquiries/resume` | JWT | Resume an interrupted verification |
| `ProcessWebhook` | Unary | `POST /api/v1/kyc/webhooks/persona` | Public | Receive Persona webhook |
| `GetAdminReviews` | Unary | `GET /api/v1/kyc/admin/reviews/getReviews` | JWT (admin) | List reviews with filters |
| `GetAdminReview` | Unary | `GET /api/v1/kyc/admin/reviews/getReview` | JWT (admin) | Get full review detail |
| `OverrideReview` | Unary | `POST /api/v1/kyc/admin/reviews/override` | JWT (admin) | Manual review override |
| `Health` | Unary | `GET /api/v1/kyc/health` | Public | Health check |

---

## Booking Service

**Proto:** `services/booking-service/proto/booking.proto`
**Package:** `booking`
**Address:** `booking-service:50058`

### RPCs

| RPC | Type | HTTP Route | Auth | Description |
|-----|------|------------|------|-------------|
| `CreateBooking` | Unary | `POST /booking/createBooking` | JWT | Create a new reservation |
| `GetBookingDetails` | Unary | `GET /booking/getBookingDetails` | JWT | Get full booking details |
| `GetPassengerBookings` | Unary | `GET /booking/getPassengerBookings` | JWT | List passenger bookings (paginated) |
| `GetDriverTripBookings` | Unary | `GET /booking/getDriverTripBookings` | JWT | List bookings for a driver's trip |
| `ApproveBooking` | Unary | `PATCH /booking/approveBooking` | JWT | Driver approves a pending booking |
| `RejectBooking` | Unary | `PATCH /booking/rejectBooking` | JWT | Driver rejects a pending booking |
| `CancelBooking` | Unary | `PATCH /booking/cancelBooking` | JWT | Cancel a booking (passenger or driver) |
| `ReportNoShow` | Unary | `POST /booking/reportNoShow` | JWT | Report passenger or driver no-show |
| `ConfirmPayment` | Unary | `POST /booking/confirmPayment` | JWT | Confirm payment (payment callback) |
| `StartBookingsForWaypoint` | Unary | `PATCH /booking/internal/startBookingsForWaypoint` | Internal | Start approved bookings for a waypoint |
| `CompleteBookingsForWaypoint` | Unary | `PATCH /booking/internal/completeBookingsForWaypoint` | Internal | Complete active bookings for a waypoint |
| `Health` | Unary | `GET /booking/health` | Public | Health check |
