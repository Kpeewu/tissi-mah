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
The `/api/v1/booking/internal/*` routes are for inter-service use (trips-service, payment-service, chat-service) and are blocked at the api-gateway.

```http
Authorization: Bearer <firebase-id-token>
```

### Caller identity

The acting user (passenger, driver, canceller, no-show reporter) is **always the user of the JWT**.
booking-service reads the Firebase UID validated by the api-gateway (`x-firebase-uid` gRPC metadata)
and resolves it to the internal user ID through user-service (`GetUserByFirebaseID`). Bookings store
internal user IDs only.

The legacy identity fields `PassengerId`, `DriverId`, `UserId` and `ReporterId` are still accepted in
requests for backward compatibility but are **ignored**: clients should stop sending them.
A Firebase UID with no user-service profile is rejected with `403`.

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

> For non-cash bookings, the seats are held as soon as the booking is created, and the client must pay **within 10 minutes** via `POST /api/v1/payment/createPayment` (FedaPay redirect flow, see `payment-service-api.md`).
>
> - Once payment is confirmed (webhook / `verifyPayment` / callback → payment-service → internal `confirmPayment`), the booking moves to `approved` if `AutoApprove = true`, otherwise to `pendingApproval`. The driver is only notified (`NEW_BOOKING_REQUEST`) at that point, never for unpaid bookings.
> - Past the window, or if the passenger cancels the payment, the booking moves to `paymentFailed` and its seats are released.

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

**Authentication:** Firebase JWT required (the JWT user must be the passenger or driver of the booking).
A call without caller identity is rejected (`401`); there is no unauthenticated variant of this route.
Inter-service callers use the internal route [`getBookingDetails` (internal)](#get-apiv1bookinginternalgetbookingdetails).

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |

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
        "History": [
            {
                "HistoryId": "h-550e8400-e29b-41d4-a716-446655440099",
                "PreviousStatus": "pendingApproval",
                "NewStatus": "approved",
                "ChangedBy": "550e8400-e29b-41d4-a716-446655440001",
                "ChangedByName": "Ama Mensah",
                "ChangedByType": "driver",
                "ChangeReason": "",
                "Metadata": "",
                "CreatedAt": "2026-04-15T08:05:00Z"
            }
        ],
        "RoutePolyline": "ki~Fjy~uOnB...",
        "PassengerMessage": ""
    },
    "ErrorMessage": ""
}
```

> Chaque entrée d'historique inclut `ChangedByName`, le nom lisible de l'auteur du
> changement (résolu via user-service). Vide pour les transitions automatiques
> (`ChangedByType` = `system`).
>
> `RoutePolyline` est le Google encoded polyline du trajet (récupéré depuis trips-service),
> destiné à afficher une preview du tracé sur la carte. Vide si le trajet n'a pas de
> tracé ou si trips-service est indisponible.

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
| `TripId` | string | Yes | Trip UUID (bookings of the calling driver only) |
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
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Reason": "Incompatible luggage"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `Reason` | string | Yes | Rejection reason |

---

### PATCH /api/v1/booking/cancelBooking

Cancels a booking. Can be initiated by the passenger or the driver of the booking (the JWT user).
The canceller determines the refund reason (`cancelledByPassenger` / `cancelledByDriver`).
Any other user gets `403 ErrorUnauthorized`.

**Authentication:** Firebase JWT required

#### Request

```http
PATCH /api/v1/booking/cancelBooking HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
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

Reports a passenger or driver no-show. The reporter is the JWT user and must be a party to the booking:

| Reporter | Allowed `NoShowType` |
|----------|----------------------|
| Driver of the booking | `passenger` |
| Passenger of the booking | `driver` |

Any other combination returns `403 ErrorUnauthorized`.

Timing and status rules:

- A no-show can only be reported between **15 minutes and 3 hours after the scheduled pickup time** of the booking's pickup waypoint (read from trips-service). Before that, the call returns `400 ErrorNoShowTooEarly`; after, `400 ErrorNoShowReportClosed` (gRPC `FailedPrecondition`). If the scheduled pickup time cannot be read, the call fails (`ErrorDataRetrievalFailed`) and nothing is recorded.
- The booking must be `approved` or `inProgress`. Once it is `inProgress` (the driver has validated the pickup and the passenger is on board), a `driver` no-show is refused (`ErrorInvalidStatusTransition`).

Contest period:

- The party reported absent is notified (`NO_SHOW_AT_DEPARTURE` for a passenger, `DRIVER_NO_SHOW_REPORTED` for a driver) and can contest within **48 hours** with [`contestNoShow`](#post-apiv1bookingcontestnoshow).
- No money moves during these 48 hours. The refund (`noShowDriver`: 100 % to the passenger) or the driver payment (`noShowPassenger`) is sent to payment-service only once the period ends without a contest, or once support has [resolved](#post-apiv1bookingadminresolvenoshowcontest) the contest.

**Authentication:** Firebase JWT required

#### Request

```http
POST /api/v1/booking/reportNoShow HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "NoShowType": "passenger",
    "Description": "Passenger did not show at pickup location"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `NoShowType` | string | Yes | `driver` or `passenger` |
| `Description` | string | No | Additional details |

---

### POST /api/v1/booking/contestNoShow

Contests a reported no-show. The caller is the JWT user and must be the party reported absent: the driver for a `driver` no-show, the passenger for a `passenger` no-show (otherwise `403 ErrorUnauthorized`). The reporter is notified (`NO_SHOW_CONTESTED`) and the refund stays blocked until support decides.

**Authentication:** Firebase JWT required — rate limit tier `sensitive`

#### Request

```json
{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Reason": "I was waiting at the pickup point from 7:55"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `Reason` | string | Yes | Why the no-show is wrong, 1000 characters max |

#### Errors

| Error | HTTP | When |
|-------|------|------|
| `ErrorInvalidInput` | 400 | Empty or too long `Reason` |
| `ErrorUnauthorized` | 403 | Caller is not the party reported absent |
| `ErrorInvalidStatusTransition` | 400 | Booking is not `noShow` |
| `ErrorNoShowAlreadyContested` | 400 | The no-show has already been contested |
| `ErrorNoShowContestClosed` | 400 | More than 48 h since the report, or refund already sent |

---

### POST /api/v1/booking/internal/confirmPayment

Confirms payment for a booking. **Internal route** (gRPC `ConfirmPayment`), called by payment-service once FedaPay has approved the payment. It is blocked at the api-gateway like every `/internal/` route; mobile clients never call it.

**Authentication:** Not required (internal)

#### Request

```json
{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "TransactionId": "123456"
}
```

#### Behaviour

| Current booking status | Result |
|---|---|
| `paymentPending` | → `approved` (with `approved_at`) if the trip has auto-approve enabled, otherwise → `pendingApproval`. The seats are already held. |
| `paymentFailed` (late FedaPay approval) | The seats are **re-reserved** (Redis + trip legs), then same transition as above. If the trip is full or the passenger already holds another active booking on the trip → `ErrorNoSeatsAvailable` (FAILED_PRECONDITION) and payment-service refunds the passenger in full. |
| already confirmed | `ErrorPaymentAlreadyConfirmed` (FAILED_PRECONDITION, idempotent) |

On success, notifications are sent: `NEW_BOOKING_REQUEST` to the driver, plus `BOOKING_CONFIRMED` to the passenger when auto-approved.

#### Response (Success)

```json
{
    "Success": true,
    "Status": "approved",
    "ErrorMessage": ""
}
```

> **Sweeper:** booking-service releases `paymentPending` bookings older than `PAYMENT_PENDING_TIMEOUT_MINUTES` (default 12) when payment-service has no successful payment for them.
>
> **Cancelling a `paymentPending` booking:** `cancelBooking` first deletes the FedaPay transaction through payment-service (`CancelPaymentForBooking`). If FedaPay had already approved it, the booking is cancelled with a refund.
>
> **Refund requests:** when a booking paid online is cancelled, rejected or marked as no-show, the refund reason (`refund_reason`) is stored in the same transaction as the status change. The request is sent to payment-service right away, except for a no-show: it waits for the end of the 48 h contest period, or for the support decision if the no-show is contested. If it fails, a worker replays it every `REFUND_REQUEST_WORKER_INTERVAL_SECONDS` (default 60) until payment-service acknowledges it (`refund_requested_at`).
> - Transient failures (payment-service or trips-service unavailable, timeout) are retried with no limit.
> - Rejections (`InvalidArgument`, `NotFound`, `FailedPrecondition`) are counted (`refund_request_attempts`). After 5, the request is no longer replayed and an error is logged for manual handling.

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

**Authentication:** Firebase JWT required (driver of the trip only — otherwise `403 ErrorUnauthorized`)

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

### GET /api/v1/booking/internal/getBookingDetails

Returns the raw details of a booking (same `Booking` payload as `getBookingDetails`, without the
history actor names and the route polyline). **Internal route** (gRPC `GetBookingDetailsInternal`),
called by payment-service and chat-service, which perform their own access checks. Blocked at the
api-gateway like every `/internal/` route.

**Authentication:** Not required (internal)

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |

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
| `BookingReference` | Recherche exacte par référence |
| `DateFrom` / `DateTo` | Bornes RFC3339 sur `created_at` |
| `NoShowContestPending` | `true` : uniquement les no-shows contestés en attente de décision (file d'attente du support) |
| `Index` | Index de page (0-based) |
| `PageSize` | Taille de page (défaut 20, max 100) |

**Response** : `{ Bookings: AdminBookingPreview[], Total: int }` où `AdminBookingPreview` contient
les infos résumées + `PassengerName` / `DriverName` (enrichis via user-service, best-effort).

### GET /api/v1/booking/admin/getBookingDetail

Détail complet d'une réservation (segments + historique de statut) **sans** contrôle
d'appartenance, enrichi de `PassengerName` / `DriverName`. Chaque entrée de
`Booking.History` inclut `ChangedByName`, le nom lisible de l'auteur du changement
(vide pour les transitions automatiques, `ChangedByType` = `system`). `Booking.RoutePolyline`
contient le Google encoded polyline du trajet (preview carte ; vide si indisponible).

`Booking` inclut la contestation éventuelle (`NoShowContestedAt`, `NoShowContestedBy`,
`NoShowContestReason`) et la décision (`NoShowResolvedAt`, `NoShowResolution`,
`NoShowResolutionNote`) ; ces champs sont aussi visibles des deux parties via `getBookingDetails`.
`NoShowResolvedBy` (UID de l'agent) n'est renvoyé qu'ici.

**Query param** : `BookingId`.
**Response** : `{ Booking: BookingDetail, PassengerName, DriverName, NoShowResolvedBy }`.

### POST /api/v1/booking/admin/resolveNoShowContest

Tranche une contestation de no-show. L'agent choisit librement le motif de remboursement final ;
il remplace `refund_reason` (réservation payée en ligne uniquement) et la demande part aussitôt
vers payment-service. Les deux parties reçoivent `NO_SHOW_CONTEST_RESOLVED`. Tier de rate
limiting `support_review`.

```json
{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "RefundReason": "noShowPassenger",
    "Note": "Conducteur présent au point de rendez-vous (échanges dans le chat)"
}
```

| Champ | Description |
|-------|-------------|
| `RefundReason` | `noShowDriver` (passager remboursé à 100 %), `noShowPassenger` (conducteur payé), `tripCancelled` (passager remboursé, sans faute) |
| `Note` | Justification de la décision, obligatoire, 1000 caractères max |

Erreurs : `ErrorInvalidInput` (motif hors liste, note vide), `ErrorNoShowNotContested` (pas de
contestation), `ErrorNoShowAlreadyResolved` (déjà tranchée) — ces deux dernières en
`FailedPrecondition` (HTTP 400).

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

The service fee is a fixed amount per booking, configurable by the platform administrator.
Default: **0** (`SERVICE_FEE_AMOUNT=0`). The operator must set this variable before go-live.

```
Subtotal    = SeatsBooked × PricePerSeat
ServiceFee  = SERVICE_FEE_AMOUNT
TotalAmount = Subtotal + ServiceFee
```

## Error Reference

| Error | HTTP Code | gRPC Code | Description |
|-------|-----------|-----------|-------------|
| `ErrorInvalidInput` | 400 | INVALID_ARGUMENT (3) | Missing or invalid fields |
| `ErrorBookingNotFound` | 404 | NOT_FOUND (5) | Booking does not exist |
| `ErrorTripNotFound` | 404 | NOT_FOUND (5) | Trip does not exist |
| `ErrorPassengerNotFound` | 404 | NOT_FOUND (5) | Passenger does not exist |
| `ErrorUnauthorized` | 403 | PERMISSION_DENIED (7) | JWT user is not allowed to act on this booking/trip |
| `ErrorInsufficientSeats` | 422 | FAILED_PRECONDITION (9) | Not enough available seats |
| `ErrorTripNotBookable` | 422 | FAILED_PRECONDITION (9) | Trip is not bookable |
| `ErrorBookingNotPending` | 422 | FAILED_PRECONDITION (9) | Booking is not in expected status |
| `ErrorBookingNotCancellable` | 422 | FAILED_PRECONDITION (9) | Booking cannot be cancelled |
| `ErrorNoShowTooEarly` | 400 | FAILED_PRECONDITION (9) | Less than 15 min after the scheduled pickup |
| `ErrorNoShowReportClosed` | 400 | FAILED_PRECONDITION (9) | More than 3 h after the scheduled pickup |
| `ErrorNoShowAlreadyContested` | 400 | FAILED_PRECONDITION (9) | No-show already contested |
| `ErrorNoShowContestClosed` | 400 | FAILED_PRECONDITION (9) | Contest period (48 h) over, or refund already sent |
| `ErrorNoShowNotContested` | 400 | FAILED_PRECONDITION (9) | Support decision on a no-show that is not contested |
| `ErrorNoShowAlreadyResolved` | 400 | FAILED_PRECONDITION (9) | Contest already resolved |
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
| `SERVICE_FEE_AMOUNT` | No | `0` | Fixed service fee per booking (currency unit); configure before go-live |
| `RECONCILIATION_INTERVAL_SECONDS` | No | `300` | Booking reconciliation interval (seconds) |
