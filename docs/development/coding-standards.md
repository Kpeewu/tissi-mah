# Coding Standards

This document defines the coding conventions for the Tissi-Mah project. All developers must follow these standards to ensure consistency across the codebase.

## Table of Contents

1. [Tools and Automation](#tools-and-automation)
2. [Go Conventions](#go-conventions)
3. [Protocol Buffers (gRPC)](#protocol-buffers-grpc)
4. [SQL Migrations](#sql-migrations)
5. [Project Structure](#project-structure)

---

## Tools and Automation

We use automated tools to enforce formatting and detect issues. Run these before every commit.

### Required Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| `gofmt` | Code formatting | Included with Go |
| `golangci-lint` | Linting (100+ linters) | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| `staticcheck` | Deep static analysis | Included in golangci-lint |

### Usage

```bash
# Format all Go files
gofmt -w .

# Or use the Makefile
make fmt

# Run linter
golangci-lint run

# Or use the Makefile
make lint

# Run all checks before committing
make check
```

### golangci-lint Configuration

Create a `.golangci.yml` file at the root of each service:

```yaml
linters:
  enable:
    - errcheck      # Check for unchecked errors
    - gosimple      # Simplify code
    - govet         # Report suspicious constructs
    - ineffassign   # Detect unused assignments
    - staticcheck   # Deep static analysis
    - typecheck     # Type checking
    - unused        # Find unused code
    - gofmt         # Check formatting
    - goimports     # Check imports
    - misspell      # Find misspelled words
    - gosec         # Security checks

linters-settings:
  errcheck:
    check-type-assertions: true
  govet:
    check-shadowing: true

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

---

## Go Conventions

### Visibility

Go determines visibility by the first letter of the identifier:

```go
// Exported (public) - accessible from other packages
func CreateBooking() {}
var MaxPassengers = 4
type User struct {}

// Non-exported (private) - only accessible within the package
func validateInput() {}
var defaultTimeout = 30
type bookingValidator struct {}
```

### Naming Conventions

#### Variables and Constants

```go
// Exported (PascalCase)
const MaxRetryAttempts = 3
const DefaultCurrency = "XOF"
var BookingTimeout = 30 * time.Second

// Non-exported (camelCase)
const maxSegments = 10
const defaultPageSize = 20
var currentRetryCount = 0
```

#### Functions and Methods

```go
// Exported (PascalCase)
func CalculateRefund(booking *Booking) (int64, error) {}
func (s *BookingService) CreateBooking(ctx context.Context, req *Request) {}

// Non-exported (camelCase)
func validateSegments(segments []Segment) error {}
func (s *BookingService) notifyPassenger(userID string) {}
```

#### Structs

```go
// Exported struct with exported fields
type Booking struct {
    ID          string
    PassengerID string
    TotalAmount int64
    CreatedAt   time.Time
}

// Non-exported struct
type bookingValidator struct {
    minAmount int64
    maxSeats  int
}
```

#### Acronyms

Acronyms must remain fully capitalized:

```go
// ✅ Correct
var userID string
var kycStatus string
func GetHTTPClient() *http.Client {}
func ValidateKYCDocuments() error {}
func ParseJSONResponse() {}
type HTTPHandler struct {}

// ❌ Incorrect
var odId string
var kycStatus string
func GetHttpClient() *http.Client {}
func ValidateKycDocuments() error {}
```

**Common acronyms in Tissi-Mah:**
- `ID` - Identifier
- `KYC` - Know Your Customer
- `CFA` - CFA Franc (currency)
- `HTTP` - Hypertext Transfer Protocol
- `JSON` - JavaScript Object Notation
- `UUID` - Universally Unique Identifier
- `gRPC` - gRPC Remote Procedure Call
- `API` - Application Programming Interface
- `URL` - Uniform Resource Locator
- `SQL` - Structured Query Language

#### Files

Always use `snake_case` for file names:

```
✅ Correct
user_service.go
booking_repository.go
jwt_validator.go
auth_handler_test.go
grpc_server.go

❌ Incorrect
userService.go
BookingRepository.go
JWTValidator.go
```

### Error Handling

#### Error Variables

Custom errors follow this convention:
- Prefix with `Err`
- **Message equals the variable name** (for easy log searching and client-side mapping)

```go
// ✅ Correct - Message equals variable name
var (
    ErrUserNotFound      = errors.New("ErrUserNotFound")
    ErrInvalidToken      = errors.New("ErrInvalidToken")
    ErrBookingCancelled  = errors.New("ErrBookingCancelled")
    ErrInsufficientSeats = errors.New("ErrInsufficientSeats")
    ErrKYCNotVerified    = errors.New("ErrKYCNotVerified")
    ErrTripFull          = errors.New("ErrTripFull")
    ErrTokenExpired      = errors.New("ErrTokenExpired")
)

// ❌ Incorrect - Message differs from variable name
var (
    ErrUserNotFound = errors.New("user not found")
    ErrInvalidToken = errors.New("the token is invalid")
)
```

**Why this convention?**
- **Easy debugging**: See `ErrUserNotFound` in logs → search `ErrUserNotFound` in code → find it immediately
- **Client mapping**: Mobile app receives `{"message": "ErrUserNotFound"}` and maps to localized user-friendly message

#### Error Handling Patterns

Always check and handle errors explicitly:

```go
// ✅ Correct - Error is checked
user, err := s.repo.FindByID(ctx, userID)
if err != nil {
    if errors.Is(err, ErrUserNotFound) {
        return nil, status.Error(codes.NotFound, err.Error())
    }
    return nil, status.Error(codes.Internal, err.Error())
}

// ❌ Incorrect - Error is ignored
user, _ := s.repo.FindByID(ctx, userID)
```

#### Wrapping Errors

Use `fmt.Errorf` with `%w` to wrap errors and preserve the error chain:

```go
// ✅ Correct - Error is wrapped with context
user, err := s.repo.FindByID(ctx, userID)
if err != nil {
    return nil, fmt.Errorf("failed to find user %s: %w", userID, err)
}

// Checking wrapped errors
if errors.Is(err, ErrUserNotFound) {
    // Handle not found case
}
```

#### gRPC Error Mapping

Map business errors to appropriate gRPC codes. Kong (API Gateway) automatically converts these to HTTP status codes.

**gRPC to HTTP mapping (handled by Kong):**

| gRPC Code | HTTP Status | Use Case |
|-----------|-------------|----------|
| `OK` | 200 | Success |
| `INVALID_ARGUMENT` | 400 | Validation errors |
| `UNAUTHENTICATED` | 401 | Authentication errors |
| `PERMISSION_DENIED` | 403 | Authorization errors |
| `NOT_FOUND` | 404 | Resource not found |
| `FAILED_PRECONDITION` | 412 | Invalid state |
| `INTERNAL` | 500 | Server errors |

**Implement a mapper in each service:**

```go
// internal/errors/errors.go
package errors

import (
    "errors"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// Business errors
var (
    ErrUserNotFound     = errors.New("ErrUserNotFound")
    ErrInvalidToken     = errors.New("ErrInvalidToken")
    ErrTokenExpired     = errors.New("ErrTokenExpired")
    ErrBookingNotFound  = errors.New("ErrBookingNotFound")
    ErrBookingCancelled = errors.New("ErrBookingCancelled")
    ErrTripFull         = errors.New("ErrTripFull")
    ErrKYCNotVerified   = errors.New("ErrKYCNotVerified")
    ErrPermissionDenied = errors.New("ErrPermissionDenied")
    ErrInvalidInput     = errors.New("ErrInvalidInput")
)

// GRPCCode returns the appropriate gRPC code for a business error
func GRPCCode(err error) codes.Code {
    switch {
    // Not Found errors
    case errors.Is(err, ErrUserNotFound),
         errors.Is(err, ErrBookingNotFound):
        return codes.NotFound

    // Authentication errors
    case errors.Is(err, ErrInvalidToken),
         errors.Is(err, ErrTokenExpired):
        return codes.Unauthenticated

    // Permission errors
    case errors.Is(err, ErrPermissionDenied):
        return codes.PermissionDenied

    // Precondition errors (invalid state)
    case errors.Is(err, ErrBookingCancelled),
         errors.Is(err, ErrTripFull),
         errors.Is(err, ErrKYCNotVerified):
        return codes.FailedPrecondition

    // Validation errors
    case errors.Is(err, ErrInvalidInput):
        return codes.InvalidArgument

    // Default to internal error
    default:
        return codes.Internal
    }
}

// ToGRPCError converts a business error to a gRPC error
func ToGRPCError(err error) error {
    if err == nil {
        return nil
    }
    code := GRPCCode(err)
    return status.Error(code, err.Error())
}
```

**Usage in gRPC handlers:**

```go
// internal/grpc/handler.go
func (h *UserHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
    user, err := h.service.GetByID(ctx, req.UserId)
    if err != nil {
        return nil, errors.ToGRPCError(err)
    }
    return &pb.UserResponse{User: toProto(user)}, nil
}
```

**What the mobile client receives (via Kong):**

```json
HTTP/1.1 404 Not Found
Content-Type: application/json

{
    "code": 5,
    "message": "ErrUserNotFound"
}
```


### Interfaces

#### Single-Method Interfaces

Use the `-er` suffix based on the method name:

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Validator interface {
    Validate() error
}

type BookingCreator interface {
    CreateBooking(ctx context.Context, req *CreateBookingRequest) (*Booking, error)
}

type PaymentProcessor interface {
    ProcessPayment(ctx context.Context, payment *Payment) error
}
```

#### Multi-Method Interfaces

Use descriptive names:

```go
type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

type BookingService interface {
    Create(ctx context.Context, req *CreateRequest) (*Booking, error)
    Cancel(ctx context.Context, bookingID string, reason string) error
    GetByID(ctx context.Context, id string) (*Booking, error)
    ListByUser(ctx context.Context, userID string) ([]*Booking, error)
}
```

#### Interface Segregation

Prefer small, focused interfaces:

```go
// ✅ Correct - Small, focused interfaces
type UserReader interface {
    FindByID(ctx context.Context, id string) (*User, error)
}

type UserWriter interface {
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
}

// Combine when needed
type UserRepository interface {
    UserReader
    UserWriter
}

// ❌ Avoid - Large, monolithic interfaces
type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    FindByPhone(ctx context.Context, phone string) (*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter *Filter) ([]*User, error)
    Count(ctx context.Context, filter *Filter) (int, error)
    // ... 10 more methods
}
```

### Comments and Documentation

#### Package Comments

Every package must have a package comment:

```go
// Package booking provides functionality for managing trip bookings
// including creation, cancellation, and segment management.
package booking
```

#### Exported Functions

All exported functions must have a comment starting with the function name:

```go
// CreateBooking creates a new booking for the specified trip and passenger.
// It validates seat availability and initiates the payment process.
// Returns ErrInsufficientSeats if no seats are available.
func CreateBooking(ctx context.Context, req *CreateBookingRequest) (*Booking, error) {
    // ...
}

// CalculateRefund calculates the refund amount based on cancellation policy.
// The refund percentage depends on how far in advance the cancellation is made.
func CalculateRefund(booking *Booking, cancelledAt time.Time) int64 {
    // ...
}
```

#### Inline Comments

Use inline comments sparingly to explain **why**, not **what**:

```go
// ✅ Correct - Explains WHY
// We add a 24-hour buffer because drivers need time to prepare
departureWindow := trip.DepartureTime.Add(-24 * time.Hour)

// ❌ Incorrect - Explains WHAT (obvious from code)
// Subtract 24 hours from departure time
departureWindow := trip.DepartureTime.Add(-24 * time.Hour)
```

### Testing

#### Test File Naming

Test files must end with `_test.go`:

```
user_service.go       → user_service_test.go
booking_repository.go → booking_repository_test.go
jwt_validator.go      → jwt_validator_test.go
```

#### Test Function Naming

Use descriptive names that explain the scenario:

```go
// ✅ Correct - Descriptive test names
func TestCreateBooking_WithValidRequest_ReturnsBooking(t *testing.T) {}
func TestCreateBooking_WithInsufficientSeats_ReturnsError(t *testing.T) {}
func TestCalculateRefund_CancelledMoreThan24HoursBefore_Returns100Percent(t *testing.T) {}

// ❌ Incorrect - Vague names
func TestCreateBooking(t *testing.T) {}
func TestCreateBooking2(t *testing.T) {}
func TestError(t *testing.T) {}
```

#### Table-Driven Tests

Prefer table-driven tests for multiple scenarios:

```go
func TestCalculateRefund(t *testing.T) {
    tests := []struct {
        name           string
        hoursBeforeTrip int
        totalAmount    int64
        expectedRefund int64
    }{
        {
            name:           "cancelled more than 24 hours before",
            hoursBeforeTrip: 48,
            totalAmount:    10000,
            expectedRefund: 10000, // 100%
        },
        {
            name:           "cancelled less than 24 hours before",
            hoursBeforeTrip: 12,
            totalAmount:    10000,
            expectedRefund: 5000, // 50%
        },
        {
            name:           "cancelled less than 2 hours before",
            hoursBeforeTrip: 1,
            totalAmount:    10000,
            expectedRefund: 0, // 0%
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CalculateRefund(tt.totalAmount, tt.hoursBeforeTrip)
            if result != tt.expectedRefund {
                t.Errorf("expected %d, got %d", tt.expectedRefund, result)
            }
        })
    }
}
```

---

## Protocol Buffers (gRPC)

### File Naming

Use `snake_case` for `.proto` files:

```
auth.proto
booking_service.proto
user_messages.proto
```

### Package Naming

Use lowercase package names:

```protobuf
syntax = "proto3";

package tissimah.booking.v1;

option go_package = "github.com/tissi-mah/booking-service/proto/gen;bookingpb";
```

### Message Naming

Use `PascalCase` for message names:

```protobuf
message CreateBookingRequest {
    // ...
}

message BookingResponse {
    // ...
}

message UserProfile {
    // ...
}
```

### Field Naming

Use `snake_case` for field names (Google's official convention):

```protobuf
message Booking {
    string booking_id = 1;
    string passenger_id = 2;
    string driver_id = 3;
    string trip_id = 4;
    int64 total_amount = 5;
    int32 seats_booked = 6;
    BookingStatus status = 7;
    google.protobuf.Timestamp created_at = 8;
    google.protobuf.Timestamp updated_at = 9;
}

message User {
    string user_id = 1;
    string first_name = 2;
    string last_name = 3;
    string phone_number = 4;
    string email_address = 5;
    bool is_driver = 6;
    bool is_passenger = 7;
    KYCStatus kyc_status = 8;
}
```

### Enum Naming

Use `PascalCase` for enum names and `SCREAMING_SNAKE_CASE` for values:

```protobuf
enum BookingStatus {
    BOOKING_STATUS_UNSPECIFIED = 0;
    BOOKING_STATUS_PENDING = 1;
    BOOKING_STATUS_APPROVED = 2;
    BOOKING_STATUS_REJECTED = 3;
    BOOKING_STATUS_CANCELLED = 4;
    BOOKING_STATUS_COMPLETED = 5;
}

enum PaymentMethod {
    PAYMENT_METHOD_UNSPECIFIED = 0;
    PAYMENT_METHOD_MOBILE_MONEY = 1;
    PAYMENT_METHOD_CARD = 2;
    PAYMENT_METHOD_PAYPAL = 3;
}
```

### Service Naming

Use `PascalCase` with `Service` suffix:

```protobuf
service BookingService {
    rpc CreateBooking(CreateBookingRequest) returns (CreateBookingResponse);
    rpc CancelBooking(CancelBookingRequest) returns (CancelBookingResponse);
    rpc GetBooking(GetBookingRequest) returns (GetBookingResponse);
    rpc ListBookings(ListBookingsRequest) returns (ListBookingsResponse);
}

service AuthService {
    rpc VerifyToken(VerifyTokenRequest) returns (VerifyTokenResponse);
    rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
}
```

### RPC Naming

Use `PascalCase` with verb-noun format:

```protobuf
// ✅ Correct - Verb + Noun
rpc CreateBooking(CreateBookingRequest) returns (CreateBookingResponse);
rpc GetUser(GetUserRequest) returns (GetUserResponse);
rpc ListTrips(ListTripsRequest) returns (ListTripsResponse);
rpc CancelBooking(CancelBookingRequest) returns (CancelBookingResponse);
rpc ValidateKYC(ValidateKYCRequest) returns (ValidateKYCResponse);

// ❌ Incorrect
rpc Booking(BookingRequest) returns (BookingResponse);
rpc User(UserRequest) returns (UserResponse);
```

---

## SQL Migrations

### File Naming

Use sequential numbering with descriptive names:

```
migrations/
├── 000001_create_users_table.up.sql
├── 000001_create_users_table.down.sql
├── 000002_create_trips_table.up.sql
├── 000002_create_trips_table.down.sql
├── 000003_add_kyc_status_to_users.up.sql
├── 000003_add_kyc_status_to_users.down.sql
```

### Table Naming

Use `snake_case` and plural nouns:

```sql
-- ✅ Correct
CREATE TABLE users (...);
CREATE TABLE trips (...);
CREATE TABLE bookings (...);
CREATE TABLE booking_segments (...);
CREATE TABLE payment_transactions (...);
CREATE TABLE user_documents (...);

-- ❌ Incorrect
CREATE TABLE User (...);
CREATE TABLE trip (...);
CREATE TABLE BookingSegment (...);
```

### Primary Key Prefixes

All primary keys use a **prefixed UUID** format to identify the entity type at a glance:

```
<prefix>-<UUID>
```

**Prefix rules:**
- Use the initials of the table name
- In case of conflict, the table created **first** keeps the short prefix, later tables add more letters

**Complete prefix list:**

| Order | Table | Prefix | Example |
|-------|-------|--------|---------|
| 1 | auth | `a` | `a-550e8400-e29b-41d4-a716-446655440000` |
| 2 | users | `u` | `u-7c9e6679-7425-40de-944b-e07fc1f90ae7` |
| 3 | vehicles | `v` | `v-f47ac10b-58cc-4372-a567-0e02b2c3d479` |
| 4 | trips | `t` | `t-9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d` |
| 5 | trips_waypoints | `tw` | `tw-1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed` |
| 6 | recurring_patterns | `rp` | `rp-6ec0bd7f-11c0-43da-975e-2a8ad9ebae0b` |
| 7 | waypoint_recurring_patterns | `wrp` | `wrp-3f333df6-90a4-4fda-8dd3-9485d27cee36` |
| 8 | bookings | `b` | `b-c56a4180-65aa-42ec-a945-5fd21dec0538` |
| 9 | booking_segments | `bs` | `bs-2e0e4c5a-7b8a-4e5b-9c1d-3f2a1b4c5d6e` |
| 10 | booking_status_history | `bsh` | `bsh-a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d` |
| 11 | payments | `p` | `p-d4e5f6a7-b8c9-0d1e-2f3a-4b5c6d7e8f9a` |
| 12 | refunds | `r` | `r-8f9a0b1c-2d3e-4f5a-6b7c-8d9e0f1a2b3c` |
| 13 | payouts | `po` | `po-4b5c6d7e-8f9a-0b1c-2d3e-4f5a6b7c8d9e` |
| 14 | ratings | `ra` | `ra-0f1a2b3c-4d5e-6f7a-8b9c-0d1e2f3a4b5c` |
| 15 | user_documents | `ud` | `ud-6b7c8d9e-0f1a-2b3c-4d5e-6f7a8b9c0d1e` |
| 16 | document_reviews | `dr` | `dr-2b3c4d5e-6f7a-8b9c-0d1e-2f3a4b5c6d7e` |
| 17 | vehicle_documents | `vd` | `vd-8b9c0d1e-2f3a-4b5c-6d7e-8f9a0b1c2d3e` |

**Implementation in Go:**

All prefixed IDs are generated **application-side** in Go, not in the database. This approach allows you to know the ID before inserting, simplifies transactions across multiple tables, and enables communication with other microservices before the database operation completes.

```go
package id

import (
    "fmt"

    "github.com/google/uuid"
)

// Prefixes for all entities
const (
    PrefixAuth                     = "a"
    PrefixUser                     = "u"
    PrefixVehicle                  = "v"
    PrefixTrip                     = "t"
    PrefixTripWaypoint             = "tw"
    PrefixRecurringPattern         = "rp"
    PrefixWaypointRecurringPattern = "wrp"
    PrefixBooking                  = "b"
    PrefixBookingSegment           = "bs"
    PrefixBookingStatusHistory     = "bsh"
    PrefixPayment                  = "p"
    PrefixRefund                   = "r"
    PrefixPayout                   = "po"
    PrefixRating                   = "ra"
    PrefixUserDocument             = "ud"
    PrefixDocumentReview           = "dr"
    PrefixVehicleDocument          = "vd"
)

// Generate creates a new prefixed UUID
func Generate(prefix string) string {
    return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}

// Example usage:
// userID := id.Generate(id.PrefixUser)        // u-550e8400-e29b-41d4-a716-446655440000
// bookingID := id.Generate(id.PrefixBooking)  // b-7c9e6679-7425-40de-944b-e07fc1f90ae7
```

**Usage example in a service:**

```go
func (s *BookingService) CreateBooking(ctx context.Context, req *CreateBookingRequest) (*Booking, error) {
    // Generate IDs before any database operation
    bookingID := id.Generate(id.PrefixBooking)
    paymentID := id.Generate(id.PrefixPayment)

    // Now you can use these IDs across multiple tables in a single transaction
    tx, _ := s.db.BeginTx(ctx, nil)
    
    tx.Exec("INSERT INTO bookings (booking_id, ...) VALUES ($1, ...)", bookingID)
    tx.Exec("INSERT INTO payments (payment_id, booking_id, ...) VALUES ($1, $2, ...)", paymentID, bookingID)
    
    tx.Commit()

    // Send notification immediately - no need to fetch the ID
    s.notifier.Send(bookingID, "Your booking is confirmed")

    return &Booking{ID: bookingID, ...}, nil
}
```

**Database schema (no auto-generation):**

```sql
CREATE TABLE users (
    user_id TEXT PRIMARY KEY,  -- No DEFAULT, ID comes from Go
    first_name VARCHAR(100),
    -- other columns...
);

CREATE TABLE bookings (
    booking_id TEXT PRIMARY KEY,  -- No DEFAULT, ID comes from Go
    passenger_id TEXT NOT NULL REFERENCES users(user_id),
    -- other columns...
);
```

**Benefits of application-side ID generation:**
- Know the ID before inserting (no extra query to retrieve it)
- Simplify transactions across multiple tables
- Enable communication with other microservices before the INSERT completes
- Easier testing with predictable IDs
- Support idempotent operations (retry with the same ID)

### Column Naming

Use `snake_case`:

```sql
CREATE TABLE bookings (
    booking_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_reference VARCHAR(20) NOT NULL UNIQUE,
    passenger_id UUID NOT NULL,
    driver_id UUID NOT NULL,
    trip_id UUID NOT NULL,
    total_amount INTEGER NOT NULL,
    seats_booked SMALLINT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payment_method VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    cancelled_at TIMESTAMP WITH TIME ZONE,
    cancellation_reason TEXT,
    
    CONSTRAINT fk_passenger FOREIGN KEY (passenger_id) REFERENCES users(user_id),
    CONSTRAINT fk_driver FOREIGN KEY (driver_id) REFERENCES users(user_id),
    CONSTRAINT fk_trip FOREIGN KEY (trip_id) REFERENCES trips(trip_id)
);
```

### Index Naming

Use the format `idx_<table>_<column(s)>`:

```sql
-- Single column index
CREATE INDEX idx_bookings_passenger_id ON bookings(passenger_id);
CREATE INDEX idx_bookings_trip_id ON bookings(trip_id);

-- Composite index
CREATE INDEX idx_bookings_status_created_at ON bookings(status, created_at);

-- Unique index
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;
```

### Constraint Naming

Use descriptive prefixes:

```sql
-- Primary key: pk_<table>
CONSTRAINT pk_bookings PRIMARY KEY (booking_id)

-- Foreign key: fk_<table>_<referenced_table>
CONSTRAINT fk_bookings_users FOREIGN KEY (passenger_id) REFERENCES users(user_id)

-- Unique: uq_<table>_<column(s)>
CONSTRAINT uq_users_email UNIQUE (email)

-- Check: ck_<table>_<description>
CONSTRAINT ck_bookings_positive_amount CHECK (total_amount > 0)
CONSTRAINT ck_bookings_valid_status CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled', 'completed'))
```

### Migration Best Practices

#### Always Include Down Migrations

```sql
-- 000005_add_phone_verified_to_users.up.sql
ALTER TABLE users ADD COLUMN phone_verified BOOLEAN DEFAULT FALSE;

-- 000005_add_phone_verified_to_users.down.sql
ALTER TABLE users DROP COLUMN phone_verified;
```

#### Make Migrations Idempotent When Possible

```sql
-- ✅ Correct - Won't fail if run twice
CREATE TABLE IF NOT EXISTS users (...);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- For columns, check existence first (in application code or use IF NOT EXISTS where supported)
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_verified BOOLEAN DEFAULT FALSE;
```

#### Avoid Data Loss in Down Migrations

```sql
-- ⚠️ Warning - This loses data
ALTER TABLE users DROP COLUMN important_data;

-- Consider keeping a backup or making the migration irreversible
-- Add a comment explaining why
-- DOWN: This migration is irreversible. The 'important_data' column cannot be restored.
```

---

## Project Structure

### Service Directory Structure

Each service follows this structure:

```
services/
└── booking-service/
    ├── cmd/
    │   └── server/
    │       └── main.go              # Entry point
    ├── internal/
    │   ├── config/
    │   │   └── config.go            # Configuration loading
    │   ├── domain/
    │   │   └── booking.go           # Domain models
    │   ├── repository/
    │   │   └── booking_repository.go # Data access
    │   ├── service/
    │   │   └── booking_service.go   # Business logic
    │   ├── grpc/
    │   │   ├── handler.go           # gRPC handlers
    │   │   └── server.go            # gRPC server setup
    │   ├── client/
    │   │   └── payment_client.go    # External service clients
    │   └── middleware/
    │       └── interceptor.go       # gRPC interceptors
    ├── pkg/
    │   └── errors/
    │       └── errors.go            # Shared error definitions
    ├── proto/
    │   ├── booking.proto            # Proto definitions
    │   └── gen/                     # Generated code
    ├── migrations/
    │   └── *.sql                    # Database migrations
    ├── tests/
    │   ├── unit/
    │   └── integration/
    ├── deployments/
    │   ├── Dockerfile
    │   └── helm/
    ├── Makefile
    └── README.md
```

### Import Organization

Organize imports in three groups, separated by blank lines:

```go
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // Third-party packages
    "github.com/google/uuid"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"

    // Internal packages
    "github.com/tissi-mah/booking-service/internal/domain"
    "github.com/tissi-mah/booking-service/internal/repository"
)
```

Use `goimports` (included in `golangci-lint`) to automatically organize imports.

---

## Quick Reference

### Go Naming Cheatsheet

| Element | Exported | Non-exported | Example |
|---------|----------|--------------|---------|
| Variable | `PascalCase` | `camelCase` | `MaxRetries` / `retryCount` |
| Function | `PascalCase` | `camelCase` | `CreateBooking` / `validate` |
| Constant | `PascalCase` | `camelCase` | `DefaultTimeout` / `maxSize` |
| Struct | `PascalCase` | `camelCase` | `UserProfile` / `config` |
| Interface | `PascalCase` + `-er` | `camelCase` + `-er` | `Reader` / `validator` |
| Error | `Err` + `PascalCase` | `err` + `camelCase` | `ErrNotFound` / `errInternal` |
| File | `snake_case` | `snake_case` | `user_service.go` |
| Acronym | `ALLCAPS` | `ALLCAPS` | `UserID`, `HTTPClient`, `KYCStatus` |

### Proto Naming Cheatsheet

| Element | Convention | Example |
|---------|------------|---------|
| File | `snake_case.proto` | `booking_service.proto` |
| Package | `lowercase` | `tissimah.booking.v1` |
| Message | `PascalCase` | `CreateBookingRequest` |
| Field | `snake_case` | `booking_id`, `created_at` |
| Enum | `PascalCase` | `BookingStatus` |
| Enum Value | `SCREAMING_SNAKE_CASE` | `BOOKING_STATUS_PENDING` |
| Service | `PascalCase` + `Service` | `BookingService` |
| RPC | `PascalCase` (Verb + Noun) | `CreateBooking`, `GetUser` |

### SQL Naming Cheatsheet

| Element | Convention | Example |
|---------|------------|---------|
| Table | `snake_case` (plural) | `bookings`, `user_documents` |
| Column | `snake_case` | `created_at`, `passenger_id` |
| Primary Key ID | `<prefix>-<UUID>` | `u-550e8400...`, `b-7c9e6679...` |
| Index | `idx_<table>_<column>` | `idx_bookings_status` |
| Primary Key | `pk_<table>` | `pk_bookings` |
| Foreign Key | `fk_<table>_<ref>` | `fk_bookings_users` |
| Unique | `uq_<table>_<column>` | `uq_users_email` |
| Check | `ck_<table>_<desc>` | `ck_bookings_positive_amount` |

### Primary Key Prefixes Cheatsheet

| Table | Prefix | Table | Prefix |
|-------|--------|-------|--------|
| auth | `a` | payments | `p` |
| users | `u` | refunds | `r` |
| vehicles | `v` | payouts | `po` |
| trips | `t` | ratings | `ra` |
| trips_waypoints | `tw` | user_documents | `ud` |
| recurring_patterns | `rp` | document_reviews | `dr` |
| waypoint_recurring_patterns | `wrp` | vehicle_documents | `vd` |
| bookings | `b` | | |
| booking_segments | `bs` | | |
| booking_status_history | `bsh` | | |