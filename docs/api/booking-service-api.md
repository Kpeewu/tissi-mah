# Booking Service API

This document describes the HTTP/REST API exposed by the booking-service through the api-gateway (grpc-gateway). The booking service manages trip reservations between passengers and drivers.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://dev.tissi-mah.com` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

All `/booking/*` endpoints (except `/booking/health` and `/booking/internal/*`) require a valid **Firebase JWT**.
The `/booking/internal/*` routes are for inter-service use (trips-service) and are not exposed to mobile clients.

```http
Authorization: Bearer <firebase-id-token>
```

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes (POST/PATCH) | `application/json` |
| `Authorization` | Yes (protected routes) | `Bearer <firebase-id-token>` |

## Error Response Format

```json
{
    "ErrorMessage": "ErrorBookingNotFound"
}
```

---

## Endpoints

### POST /booking/createBooking

Creates a new trip reservation for a passenger.

**Authentication:** Firebase JWT required

#### Request

```http
POST /booking/createBooking HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "PassengerId": "550e8400-e29b-41d4-a716-446655440010",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "PickupWaypointId": "w-dep-001",
    "DropoffWaypointId": "w-arr-002",
    "SeatsBooked": 2,
    "PaymentMethod": "mobileMoney",
    "Segments": [
        {
            "PickupWaypointId": "w-dep-001",
            "DropoffWaypointId": "w-arr-002",
            "PickupLocationName": "Gare routière de Dakar",
            "PickupCity": "Dakar",
            "PickupLat": 14.6928,
            "PickupLng": -17.4467,
            "PickupScheduledAt": "2026-04-15T08:00:00Z",
            "DropoffLocationName": "Gare de Saint-Louis",
            "DropoffCity": "Saint-Louis",
            "DropoffLat": 16.0197,
            "DropoffLng": -16.4896,
            "DropoffScheduledAt": "2026-04-15T12:00:00Z",
            "SegmentDistanceMeters": 280000,
            "SegmentDurationMinutes": 240,
            "SegmentPrice": 10000
        }
    ]
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PassengerId` | string | Yes | Internal user ID of the passenger (UUID) |
| `TripId` | string | Yes | UUID of the trip to book |
| `PickupWaypointId` | string | Yes | Waypoint where the passenger boards |
| `DropoffWaypointId` | string | Yes | Waypoint where the passenger exits |
| `SeatsBooked` | integer | Yes | Number of seats to reserve (≥ 1) |
| `PaymentMethod` | string | Yes | `mobileMoney` \| `card` \| `paypal` \| `cash` |
| `Segments` | array | Yes | Journey segments (one per leg of the trip) |

> **Note on `SegmentPrice`:** The price sent in each segment is informational only. The actual price is recalculated server-side from the trip's waypoint prices.

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "BookingReference": "TM-2026-0001",
    "Status": "paymentPending",
    "TotalAmount": 20000,
    "ErrorMessage": ""
}
```

> `Status` will be `paymentPending` for non-cash payments, or `pendingApproval` / `approved` for cash payments.

#### Booking Status After Creation

| Payment Method | AutoApprove | Initial Status |
|---------------|-------------|----------------|
| `cash` | `true` | `approved` |
| `cash` | `false` | `pendingApproval` |
| non-cash (`mobileMoney`, `card`, `paypal`) | — | `paymentPending` |

> For non-cash bookings, the client must initiate payment via `POST /payment/createPayment`. Once payment is confirmed (via FedaPay webhook → payment-service → `confirmPayment`), the booking transitions to `pendingApproval` (or `approved` if `AutoApprove = true`).

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorPassengerNotFound` | 404 | Passenger does not exist |
| `ErrorInsufficientSeats` | 422 | Not enough available seats |
| `ErrorTripNotBookable` | 422 | Trip is not in `scheduled` status |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /booking/getBookingDetails

Returns the complete details of a booking including segments and status history.

**Authentication:** Firebase JWT required (passenger or driver of the trip)

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `UserId` | string | Yes | Requesting user ID (authorization check) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Booking": {
        "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
        "BookingReference": "TM-2026-0001",
        "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
        "PassengerId": "550e8400-e29b-41d4-a716-446655440010",
        "DriverId": "550e8400-e29b-41d4-a716-446655440001",
        "PickupWaypointId": "w-dep-001",
        "DropoffWaypointId": "w-arr-002",
        "SeatsBooked": 2,
        "PricePerSeat": 5000,
        "Subtotal": 10000,
        "ServiceFee": 1000,
        "TotalAmount": 11000,
        "PaymentMethod": "mobileMoney",
        "Status": "approved",
        "PaymentCompletedAt": "2026-04-15T08:02:00Z",
        "ApprovedAt": "2026-04-15T08:05:00Z",
        "RejectedAt": "",
        "CancelledAt": "",
        "CompletedAt": "",
        "CancellerId": "",
        "CancellationReason": "",
        "NoShowType": "",
        "NoShowReportedBy": "",
        "NoShowReportedAt": "",
        "NoShowDescription": "",
        "CreatedAt": "2026-04-15T08:00:00Z",
        "UpdatedAt": "2026-04-15T08:05:00Z",
        "Segments": [...],
        "History": [...]
    },
    "ErrorMessage": ""
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorBookingNotFound` | 404 | Booking does not exist |
| `ErrorUnauthorized` | 403 | User is not the passenger or driver |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /booking/getPassengerBookings

Returns the paginated list of bookings for a passenger.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `PassengerId` | string | Yes | Passenger's internal user ID |
| `Index` | integer | No | Page index (0-based, 10 items/page) |
| `StatusFilter` | string | No | Filter by booking status |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Bookings": [
        {
            "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
            "BookingReference": "TM-2026-0001",
            "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
            "Status": "approved",
            "SeatsBooked": 2,
            "TotalAmount": 11000,
            "PickupLocationName": "Gare routière de Dakar",
            "DropoffLocationName": "Gare de Saint-Louis",
            "DepartureDate": "2026-04-15",
            "DepartureTime": "08:00"
        }
    ],
    "ErrorMessage": ""
}
```

---

### GET /booking/getDriverTripBookings

Returns the paginated list of bookings for a specific trip, for the driver.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DriverId` | string | Yes | Driver's internal user ID |
| `TripId` | string | Yes | Trip UUID |
| `Index` | integer | No | Page index (0-based, 10 items/page) |

---

### PATCH /booking/approveBooking

Driver approves a pending booking.

**Authentication:** Firebase JWT required (driver only)

#### Request

```http
PATCH /booking/approveBooking HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020"
}
```

#### Response (Success)

```json
{ "Success": true, "ErrorMessage": "" }
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorBookingNotFound` | 404 | Booking does not exist |
| `ErrorUnauthorized` | 403 | User is not the driver of this trip |
| `ErrorBookingNotPending` | 422 | Booking is not in `pendingApproval` status |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /booking/rejectBooking

Driver rejects a pending booking.

**Authentication:** Firebase JWT required (driver only)

#### Request

```http
PATCH /booking/rejectBooking HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Reason": "Incompatible luggage"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Driver's internal user ID |
| `BookingId` | string | Yes | Booking UUID |
| `Reason` | string | Yes | Rejection reason |

---

### PATCH /booking/cancelBooking

Cancels a booking. Can be initiated by the passenger or the driver.

**Authentication:** Firebase JWT required

#### Request

```http
PATCH /booking/cancelBooking HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "UserId": "550e8400-e29b-41d4-a716-446655440010",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Reason": "Plans changed"
}
```

#### Cancellable Statuses

| Status | Can Cancel |
|--------|-----------|
| `created` | ✓ |
| `paymentPending` | ✓ |
| `pendingApproval` | ✓ |
| `approved` | ✓ |
| `inProgress` | ✗ |
| `completed` | ✗ |

---

### POST /booking/reportNoShow

Reports a passenger or driver no-show.

**Authentication:** Firebase JWT required

#### Request

```http
POST /booking/reportNoShow HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "ReporterId": "550e8400-e29b-41d4-a716-446655440001",
    "NoShowType": "passenger",
    "Description": "Passenger did not show at pickup location"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `ReporterId` | string | Yes | ID of the user reporting the no-show |
| `NoShowType` | string | Yes | `driver` or `passenger` |
| `Description` | string | No | Additional details |

---

### POST /booking/confirmPayment

Confirms payment for a booking. **Called internally by payment-service** after a successful payment (FedaPay webhook → payment-service → this endpoint). Mobile clients do not call this directly.

**Authentication:** Firebase JWT required

#### Request

```http
POST /booking/confirmPayment HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "TransactionId": "txn_abc123"
}
```

#### Response (Success)

```json
{
    "Success": true,
    "Status": "pendingApproval",
    "ErrorMessage": ""
}
```

> `Status` will be `pendingApproval` if the driver has not enabled auto-approve, or `approved` if auto-approve is enabled on the trip.

---

### PATCH /booking/internal/startBookingsForWaypoint

Starts approved bookings associated with a trip waypoint. **Internal route** called by trips-service on `StartTrip` (for the departure waypoint) and `ConfirmWaypointDeparture` (for stop waypoints).

**Authentication:** Not required (internal)

#### Request

```json
{
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "WaypointId": "w-dep-001"
}
```

#### Response

```json
{ "Success": true, "BookingsCount": 3, "ErrorMessage": "" }
```

---

### PATCH /booking/internal/completeBookingsForWaypoint

Completes active bookings associated with a trip waypoint. **Internal route** called by trips-service on `EndTrip` (for the arrival waypoint) and `ConfirmWaypointArrival` (for stop waypoints).

**Authentication:** Not required (internal)

#### Request

```json
{
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "WaypointId": "w-arr-002"
}
```

#### Response

```json
{ "Success": true, "BookingsCount": 3, "ErrorMessage": "" }
```

---

### GET /booking/health

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

## Booking Lifecycle

```
                              ┌─── (cash, autoApprove=true) ──────────────────────────┐
                              │                                                         ▼
created → paymentPending ─────┤        pendingApproval → approved → inProgress → completed
          (non-cash)          │               ↓               ↓           ↓
                              └─── (cash) ──► ↑           rejected    cancelled      noShow
                                                           cancelled    noShow
```

**Simplified:**
```
created → paymentPending → pendingApproval → approved → inProgress → completed
                ↓                  ↓             ↓
            cancelled          rejected       cancelled
                                              noShow
```

| Status | Description |
|--------|-------------|
| `created` | Booking created (cash only — intermediate state before persistence) |
| `paymentPending` | Non-cash booking awaiting payment confirmation |
| `pendingApproval` | Payment confirmed (or cash booking) — awaiting driver approval |
| `approved` | Driver approved the booking (or auto-approved) |
| `rejected` | Driver rejected the booking |
| `cancelled` | Cancelled by passenger or driver |
| `inProgress` | Trip started, passenger on board |
| `completed` | Trip ended, booking finalized |
| `noShow` | Passenger or driver no-show reported |
| `expired` | Booking expired (payment timeout) |

## Service Fee

The service fee is a configurable percentage applied on top of the subtotal.
Default: **10%** (`SERVICE_FEE_PERCENT=10`).

```
Subtotal    = SeatsBooked × PricePerSeat
ServiceFee  = Subtotal × SERVICE_FEE_PERCENT / 100
TotalAmount = Subtotal + ServiceFee
```

## Error Reference

| Error | HTTP Code | gRPC Code | Description |
|-------|-----------|-----------|-------------|
| `ErrorInvalidInput` | 400 | INVALID_ARGUMENT (3) | Missing or invalid fields |
| `ErrorBookingNotFound` | 404 | NOT_FOUND (5) | Booking does not exist |
| `ErrorTripNotFound` | 404 | NOT_FOUND (5) | Trip does not exist |
| `ErrorPassengerNotFound` | 404 | NOT_FOUND (5) | Passenger does not exist |
| `ErrorUnauthorized` | 403 | PERMISSION_DENIED (7) | Not the passenger or driver |
| `ErrorInsufficientSeats` | 422 | FAILED_PRECONDITION (9) | Not enough available seats |
| `ErrorTripNotBookable` | 422 | FAILED_PRECONDITION (9) | Trip is not bookable |
| `ErrorBookingNotPending` | 422 | FAILED_PRECONDITION (9) | Booking is not in expected status |
| `ErrorBookingNotCancellable` | 422 | FAILED_PRECONDITION (9) | Booking cannot be cancelled |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Internal server error |
