# Booking Service API

This document describes the HTTP/REST API exposed by the booking-service through the api-gateway (grpc-gateway). The booking service manages trip reservations between passengers and drivers.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

All `/api/v1/booking/*` endpoints (except `/api/v1/booking/health` and `/api/v1/booking/internal/*`) require a valid **Firebase JWT**.
The `/api/v1/booking/internal/*` routes are for inter-service use (trips-service) and are not exposed to mobile clients.

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

### POST /api/v1/booking/createBooking

Creates a new trip reservation for a passenger.

**Authentication:** Firebase JWT required

#### Request

```http
POST /api/v1/booking/createBooking HTTP/1.1
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

### GET /api/v1/booking/getBookingDetails

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

### GET /api/v1/booking/getPassengerBookings

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

### GET /api/v1/booking/getDriverTripBookings

Returns the paginated list of bookings for a specific trip, enriched with passenger info.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DriverId` | string | Yes | Driver's internal user ID |
| `TripId` | string | Yes | Trip UUID |
| `Index` | integer | No | Page index (0-based, 10 items/page) |

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
            "DepartureTime": "08:00",
            "PassengerName": "Koffi Asante",
            "PassengerRating": 4.8,
            "PassengerTripCount": 12,
            "IsPassengerVerified": true,
            "PassengerMessage": "Je serai à l'heure",
            "PaymentMethod": "mobileMoney",
            "CreatedAt": "2026-04-14T15:30:00Z",
            "ExtraMinutesDetour": 0
        }
    ],
    "ErrorMessage": ""
}
```

#### Response Fields (`DriverBookingPreview`)

| Field | Type | Description |
|-------|------|-------------|
| `BookingId` | string | Booking UUID |
| `BookingReference` | string | Human-readable reference (e.g. `TM-2026-0001`) |
| `TripId` | string | Trip UUID |
| `Status` | string | Booking status |
| `SeatsBooked` | integer | Number of seats |
| `TotalAmount` | integer | Total amount (XOF) |
| `PickupLocationName` | string | Pickup location name |
| `DropoffLocationName` | string | Dropoff location name |
| `DepartureDate` | string | `YYYY-MM-DD` |
| `DepartureTime` | string | `HH:MM` |
| `PassengerName` | string | Passenger's full name (from user-service) |
| `PassengerRating` | float | Passenger rating average (0 if no ratings) |
| `PassengerTripCount` | integer | Number of trips completed by passenger |
| `IsPassengerVerified` | boolean | Whether the passenger is KYC-verified |
| `PassengerMessage` | string | Optional message from passenger |
| `PaymentMethod` | string | `mobileMoney` \| `card` \| `paypal` \| `cash` |
| `CreatedAt` | string | ISO 8601 — booking creation datetime |
| `ExtraMinutesDetour` | integer | Requested detour in minutes (0 = none) |

---

### PATCH /api/v1/booking/approveBooking

Driver approves a pending booking.

**Authentication:** Firebase JWT required (driver only)

#### Request

```http
PATCH /api/v1/booking/approveBooking HTTP/1.1
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

### PATCH /api/v1/booking/rejectBooking

Driver rejects a pending booking.

**Authentication:** Firebase JWT required (driver only)

#### Request

```http
PATCH /api/v1/booking/rejectBooking HTTP/1.1
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

### PATCH /api/v1/booking/cancelBooking

Cancels a booking. Can be initiated by the passenger or the driver.

**Authentication:** Firebase JWT required

#### Request

```http
PATCH /api/v1/booking/cancelBooking HTTP/1.1
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

### POST /api/v1/booking/reportNoShow

Reports a passenger or driver no-show.

**Authentication:** Firebase JWT required

#### Request

```http
POST /api/v1/booking/reportNoShow HTTP/1.1
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

### POST /api/v1/booking/confirmPayment

Confirms payment for a booking. **Called internally by payment-service** after a successful payment (FedaPay webhook → payment-service → this endpoint). Mobile clients do not call this directly.

**Authentication:** Firebase JWT required

#### Request

```http
POST /api/v1/booking/confirmPayment HTTP/1.1
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

### PATCH /api/v1/booking/internal/startBookingsForWaypoint

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

### PATCH /api/v1/booking/internal/completeBookingsForWaypoint

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

### GET /api/v1/booking/health

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

### GET /api/v1/booking/getDriverPendingBookings

Returns all bookings awaiting driver approval, across all trips of the driver.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DriverId` | string | Yes | Driver's internal user ID |
| `Index` | integer | No | Page index (0-based, 10 items/page) |

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
            "Status": "pendingApproval",
            "SeatsBooked": 1,
            "TotalAmount": 5500,
            "PickupLocationName": "Liberté 6",
            "DropoffLocationName": "Saint-Louis Centre",
            "DepartureDate": "2026-04-15",
            "DepartureTime": "08:00",
            "PassengerName": "Ama Owusu",
            "PassengerRating": 4.5,
            "PassengerTripCount": 7,
            "IsPassengerVerified": true,
            "PassengerMessage": "",
            "PaymentMethod": "cash",
            "CreatedAt": "2026-04-14T20:00:00Z",
            "ExtraMinutesDetour": 0
        }
    ],
    "ErrorMessage": ""
}
```

> Response uses the same `DriverBookingPreview` structure as `getDriverTripBookings`. All items have `Status = "pendingApproval"`.

#### Errors

| Error | HTTP | Description |
|-------|------|-------------|
| `ErrorDriverNotFound` | 404 | Driver does not exist |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/booking/getActivePassengerSummaries

Returns real-time summaries of passengers currently on board a trip. Used by the driver's live tracking screen.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `TripId` | string | Yes | Trip UUID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Summaries": [
        {
            "PassengerId": "550e8400-e29b-41d4-a716-446655440010",
            "PassengerName": "Koffi Asante",
            "SeatsBooked": 2,
            "PaymentMethod": "mobileMoney",
            "PaymentStatus": "paid",
            "Rating": 4.8,
            "IsVerified": true,
            "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020"
        }
    ],
    "ErrorMessage": ""
}
```

#### Response Fields (`PassengerSummary`)

| Field | Type | Description |
|-------|------|-------------|
| `PassengerId` | string | Passenger's internal user ID |
| `PassengerName` | string | Full name |
| `SeatsBooked` | integer | Number of seats |
| `PaymentMethod` | string | `mobileMoney` \| `card` \| `paypal` \| `cash` |
| `PaymentStatus` | string | `paid` \| `pending` |
| `Rating` | float | Passenger rating average |
| `IsVerified` | boolean | KYC verification status |
| `BookingId` | string | Booking UUID |

---

### PATCH /api/v1/booking/internal/cancelBookingsForWaypoint

Cancels all bookings associated with a waypoint that was removed. **Internal route** called by trips-service on `CancelWaypoint`.

**Authentication:** Not required (internal)

#### Request

```json
{
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "WaypointId": "w-stop-002"
}
```

#### Response

```json
{ "Success": true, "BookingsCount": 2, "ErrorMessage": "" }
```

---

### PATCH /api/v1/booking/internal/cancelBookingsForTrip

Cancels all bookings for a trip that was cancelled. **Internal route** called by trips-service on `CancelTrip`.

**Authentication:** Not required (internal)

#### Request

```json
{ "TripId": "t-550e8400-e29b-41d4-a716-446655440000" }
```

#### Response

```json
{ "Success": true, "BookingsCount": 5, "ErrorMessage": "" }
```

---

### POST /api/v1/booking/internal/failPayment

Marks a booking's payment as failed. **Internal route** called by payment-service on a failed FedaPay webhook event.

**Authentication:** Not required (internal)

#### Request

```json
{ "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020", "Reason": "transaction.declined" }
```

#### Response

```json
{ "Success": true, "ErrorMessage": "" }
```

---

### GET /api/v1/booking/internal/getActivePassengerIDsForTrip

Returns the list of passenger IDs with an `inProgress` booking for a given trip. Used by trips-service and payment-service.

**Authentication:** Not required (internal)

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `TripID` | string | Yes | Trip UUID |

#### Response

```json
{ "PassengerIDs": ["550e8400-e29b-41d4-a716-446655440010", "550e8400-e29b-41d4-a716-446655440011"] }
```

---

## Support (admin) endpoints

Réservés aux **agents support** (JWT support, header `x-support-uid` injecté par l'api-gateway —
routes dans `SupportProtectedRoutes`). Pas de scoping par acteur : le support voit toutes les
réservations.

### GET /api/v1/booking/admin/listBookings

Liste paginée et filtrée de toutes les réservations, enrichie des noms passager/conducteur.

**Query params** (tous optionnels sauf pagination) :

| Param | Description |
|-------|-------------|
| `Status` | Filtre par statut (`created`, `paymentPending`, `pendingApproval`, `approved`, `rejected`, `cancelled`, `inProgress`, `completed`, `noShow`, `expired`) |
| `PassengerId` / `DriverId` / `TripId` | Filtres d'identité |
| `BookingReference` | Recherche exacte par référence |
| `DateFrom` / `DateTo` | Bornes RFC3339 sur `created_at` |
| `Index` | Index de page (0-based) |
| `PageSize` | Taille de page (défaut 20, max 100) |

**Response** : `{ Bookings: AdminBookingPreview[], Total: int }` où `AdminBookingPreview` contient
les infos résumées + `PassengerName` / `DriverName` (enrichis via user-service, best-effort).

### GET /api/v1/booking/admin/getBookingDetail

Détail complet d'une réservation (segments + historique de statut) **sans** contrôle
d'appartenance, enrichi de `PassengerName` / `DriverName`.

**Query param** : `BookingId`.
**Response** : `{ Booking: BookingDetail, PassengerName, DriverName }`.

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

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | PostgreSQL connection URL (`booking_db`, port 5438 dev) |
| `REDIS_URL` | Yes | — | Redis connection URL (port 6387 dev) |
| `ENVIRONMENT` | Yes | — | `local` / `vps-dev` / `staging` / `prod` |
| `LOG_LEVEL` | Yes | — | `debug` / `info` / `warn` / `error` |
| `GRPC_PORT` | No | `50058` | gRPC server port |
| `TRIPS_SERVICE_HOST` | No | `0.0.0.0` | Host for trips-service |
| `TRIPS_SERVICE_PORT` | No | `50056` | Port for trips-service |
| `USER_SERVICE_HOST` | No | `0.0.0.0` | Host for user-service |
| `USER_SERVICE_PORT` | No | `50052` | Port for user-service |
| `SERVICE_FEE_PERCENT` | No | `10` | Platform fee percentage applied to subtotal |
| `RECONCILIATION_INTERVAL_SECONDS` | No | `300` | Booking reconciliation interval (seconds) |
