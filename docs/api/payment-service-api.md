# Payment Service API

This document describes the HTTP/REST API exposed by the payment-service through the api-gateway (grpc-gateway). The payment service manages payments, refunds, and driver payouts via FedaPay.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://dev.tissi-mah.com` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

| Route | Auth |
|-------|------|
| `/api/v1/payment/createPayment` | Firebase JWT required (booking passenger) |
| `/api/v1/payment/verifyPayment` | Firebase JWT required (booking passenger) |
| `/api/v1/payment/cancelPayment` | Firebase JWT required (booking passenger) |
| `/api/v1/payment/callback` | Public (FedaPay return URL; status re-checked with FedaPay) |
| `/api/v1/payment/getPaymentStatus` | Firebase JWT required |
| `/api/v1/payment/getPaymentByBooking` | Firebase JWT required |
| `/api/v1/payment/getRefundStatus` | Firebase JWT required |
| `/api/v1/payment/getPayoutStatus` | Firebase JWT required |
| `/api/v1/payment/getDriverPayouts` | Firebase JWT required |
| `/api/v1/payment/webhooks/fedapay` | Public (HMAC-SHA256 signature verified internally) |
| `/api/v1/payment/internal/*` | Public (inter-service only, not for mobile clients) |
| `/api/v1/payment/health` | Public |

```http
Authorization: Bearer <firebase-id-token>
```

## Error Response Format

```json
{
    "ErrorMessage": "ErrorPaymentNotFound"
}
```

---

## Endpoints

### Mobile integration — FedaPay payment with redirection

Payments use the FedaPay **redirect** flow. The passenger picks their operator and phone number on the FedaPay payment page. The app never sends the mobile money operator.

```
App                      api-gateway / payment-service              FedaPay
 │ POST createPayment ──────────►│ POST /v1/transactions (callback_url) ─────►│
 │                               │ POST /v1/transactions/{id}/token ─────────►│
 │◄── PaymentUrl, ExpiresAt ─────│                                            │
 │ open PaymentUrl (in-app browser) ─────────────────────────────────────────►│ passenger pays
 │                               │◄──── GET /api/v1/payment/callback?id=&status=
 │                               │ GET /v1/transactions/{id} (re-check) ─────►│
 │◄── 302 tissimah://payment/result?paymentId=…&status=… ──────────────────────│
 │ POST verifyPayment ──────────►│ GET /v1/transactions/{id} ────────────────►│
 │◄── Status / BookingStatus ────│                                            │
```

1. **`POST /api/v1/payment/createPayment`** returns `PaymentUrl` and `ExpiresAt`.
2. Open `PaymentUrl` in an in-app browser that can catch the custom-scheme redirect: `ASWebAuthenticationSession` on iOS, Custom Tabs on Android (or `flutter_web_auth_2`), with callback scheme `tissimah`.
3. After payment, FedaPay redirects the browser to `GET /api/v1/payment/callback`.
   - The api-gateway checks the transaction with FedaPay.
   - It then answers `302` to `tissimah://payment/result?paymentId=…&status=…&providerStatus=…&bookingStatus=…`.
4. On the deep link, **or** whenever the browser is closed without it, call **`POST /api/v1/payment/verifyPayment`**. This is the source of truth; never trust the deep-link query alone.
5. Act on `Status`:

| `Status` | `ProviderStatus` | Meaning for the app |
|---|---|---|
| `held` | `approved` | Paid. Show the booking: `BookingStatus` is `approved` (auto-approval) or `pendingApproval` (waiting for the driver). |
| `pending` | `pending` | Not paid yet. Offer **Resume payment** (`createPayment` again returns the same `PaymentUrl` with `Reused=true`) or **Cancel** (`cancelPayment`). |
| `pending` | `declined` / `canceled` | The attempt failed (for example, insufficient balance), but **a retry is possible** with the same link until `ExpiresAt`. |
| `failed` | `deleted` / `expired` | Payment window over or cancelled. **The seats were released** (`SeatsReleased=true`, `BookingStatus=paymentFailed`), so the passenger must book again. |
| `refunded` | `approved` | Paid too late and the trip is now full: a full refund was issued to the refund number. |

> **Payment window: 10 minutes from booking creation** (`PAYMENT_TIMEOUT_MINUTES`). After `ExpiresAt`, the FedaPay transaction is deleted, the booking moves to `paymentFailed` and its seats go back on sale.

---

### POST /api/v1/payment/createPayment

Creates the FedaPay transaction of a booking and returns the payment page link. The call is **idempotent per booking**: while a `pending` payment exists, it returns the same link with `Reused=true`.

**Authentication:** Firebase JWT required. The caller must be the booking's passenger.

#### Request

```http
POST /api/v1/payment/createPayment HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
X-App-ID: <mobile-app-id>
Content-Type: application/json

{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "PaymentMethod": "mobileMoney",
    "PassengerPhoneNumber": "90000000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID. The booking must be in `paymentPending`. |
| `TripId` | string | No | If provided, it must match the booking's trip. |
| `PaymentMethod` | string | Yes | `mobileMoney` |
| `PassengerPhoneNumber` | string | Yes | **Refund** number (Togocel, 8 digits; `+228`/`00228` prefixes accepted). It is *not* the paying number, which the passenger chooses on the FedaPay page. |
| `Amount` | integer | Deprecated | Ignored. The amount charged is the booking's `TotalAmount`. |
| `MobileMoneyMode` | string | Deprecated | Ignored. The operator is chosen on the FedaPay page. |

#### Response (Success)

```json
{
    "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030",
    "PaymentReference": "PAY-20261011-7KQ2ZD",
    "Status": "pending",
    "ErrorMessage": "",
    "PaymentUrl": "https://process.fedapay.com/eyJ0eXAiOiJKV1QiLCJhbGciOi...",
    "ExpiresAt": "2026-10-11T10:10:00Z",
    "Reused": false,
    "Amount": 11000
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing or invalid fields (method, refund number, trip mismatch) |
| `ErrorBookingNotPayable` | 400 | Booking is cash or no longer in `paymentPending` |
| `ErrorBookingPaymentExpired` | 400 | The 10-minute payment window is over (book again) |
| `ErrorPaymentNotOwned` | 403 | The caller is not the booking's passenger |
| `ErrorBookingNotFound` | 404 | Unknown booking |
| `ErrorDuplicatePayment` | 409 | The booking is already paid (`held`) |
| `ErrorFedaPayAPIError` | 503 | FedaPay unavailable; retry |
| `ErrorInternalServer` | 500 | Internal server error |

---

### POST /api/v1/payment/verifyPayment

Re-synchronizes the payment with FedaPay (`GET /v1/transactions/{id}`) and applies the matching transition to the payment and the booking. Call it when the app gets the deep link back or when the payment browser is closed. It is safe to call repeatedly.

**Authentication:** Firebase JWT required (booking's passenger)

#### Request

```json
{ "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030" }
```

#### Response (Success) — `PaymentStatusResponse`

```json
{
    "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Status": "held",
    "ProviderStatus": "approved",
    "IsFinal": true,
    "PaymentUrl": "",
    "ExpiresAt": "2026-10-11T10:10:00Z",
    "BookingStatus": "pendingApproval",
    "SeatsReleased": false,
    "Amount": 11000,
    "ErrorMessage": ""
}
```

| Field | Description |
|-------|-------------|
| `Status` | Internal payment status: `pending`, `held`, `failed`, `refunded`, … |
| `ProviderStatus` | Last FedaPay status: `pending`, `approved`, `declined`, `canceled`, `expired`, `deleted` |
| `IsFinal` | `false` while `Status=pending` |
| `PaymentUrl` | Payment link, filled only while `pending` |
| `BookingStatus` | `paymentPending`, `pendingApproval`, `approved`, `paymentFailed`, … |
| `SeatsReleased` | `true` if the seats were released (a new booking is required) |

#### Errors

`ErrorInvalidInput` (400), `ErrorPaymentNotOwned` (403), `ErrorPaymentNotFound` (404), `ErrorFedaPayAPIError` (503).

---

### POST /api/v1/payment/cancelPayment

The passenger gives up a `pending` payment. The FedaPay transaction is **deleted** (the link stops working), the payment becomes `failed` and the booking becomes `paymentFailed` with its **seats released**.

If FedaPay has already approved the payment, nothing is cancelled: the approval is applied and `Status=held` is returned.

**Authentication:** Firebase JWT required (booking's passenger)

#### Request

```json
{ "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030" }
```

#### Response

`PaymentStatusResponse` (see `verifyPayment`). For a payment that is already `held` or `failed`, the current state is returned unchanged.

#### Errors

`ErrorPaymentNotCancellable` (400, already released, paid out or refunded), `ErrorPaymentNotOwned` (403), `ErrorPaymentNotFound` (404), `ErrorFedaPayAPIError` (503, the deletion could not be confirmed, so nothing was changed).

---

### GET /api/v1/payment/callback

The FedaPay `callback_url`. FedaPay redirects the passenger's browser here after payment with `?id=<fedapay-transaction-id>&status=<status>`.

**Authentication:** None. The browser carries no JWT or App-ID. The `status` query parameter is **ignored**: payment-service re-checks the transaction with FedaPay.

#### Response

`302 Found` to the deep link `PAYMENT_DEEPLINK_URL` (default `tissimah://payment/result`):

```
tissimah://payment/result?paymentId=pay-…&status=held&providerStatus=approved&bookingStatus=pendingApproval
```

On any error (invalid id, unknown transaction, FedaPay unavailable), the redirect is `tissimah://payment/result?status=error`. The app then calls `verifyPayment` with the `PaymentId` it got from `createPayment`.

---

### GET /api/v1/payment/getPaymentStatus

Returns the current status of a payment.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `PaymentId` | string | Yes | Payment UUID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Amount": 11000,
    "PaymentMethod": "mobileMoney",
    "Status": "held",
    "PaymentReference": "TM-PAY-2026-0001",
    "CreatedAt": "2026-04-15T08:00:00Z",
    "CompletedAt": "2026-04-15T08:02:30Z",
    "ErrorMessage": "",
    "PaymentUrl": "",
    "ProviderStatus": "approved",
    "ExpiresAt": "2026-04-15T08:10:00Z"
}
```

> `PaymentUrl` is only filled while the payment is `pending` (to resume the payment). `ProviderStatus` is the last FedaPay status observed.

#### Payment Statuses

| Status | Description |
|--------|-------------|
| `pending` | Payment page created, waiting for the passenger (a `declined`/`canceled` attempt keeps the payment `pending`) |
| `held` | Payment confirmed by FedaPay, funds held pending trip completion |
| `released` | Funds released to driver after contestation delay |
| `paidOut` | Driver payout processed |
| `failed` | Payment window expired or payment cancelled; FedaPay transaction deleted, seats released |
| `refunded` | Refund processed |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorPaymentNotFound` | 404 | Payment does not exist |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/payment/getPaymentByBooking

Returns the payment associated with a booking.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "Amount": 11000,
    "PaymentMethod": "mobileMoney",
    "Status": "held",
    "PaymentReference": "TM-PAY-2026-0001",
    "CreatedAt": "2026-04-15T08:00:00Z",
    "ErrorMessage": "",
    "CompletedAt": "2026-04-15T08:02:30Z",
    "PaymentUrl": "",
    "ProviderStatus": "approved",
    "ExpiresAt": "2026-04-15T08:10:00Z"
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorPaymentNotFound` | 404 | No payment found for this booking |
| `ErrorInternalServer` | 500 | Internal server error |

---

### POST /api/v1/payment/webhooks/fedapay

Receives and processes FedaPay webhook events. **Public route** — not called by mobile clients.

**Authentication:** None (signature verified via HMAC-SHA256 using `X-FEDAPAY-SIGNATURE` header)

> The api-gateway uses a raw HTTP handler for this route to preserve the exact body bytes needed for signature verification. The header has the form `t=<timestamp>,s=<hex>` and the signature is `HMAC-SHA256(secret, "<timestamp>.<rawBody>")` encoded as hex.

#### Headers

| Header | Description |
|--------|-------------|
| `X-FEDAPAY-SIGNATURE` | `t=<timestamp>,s=<hex HMAC-SHA256 of "<timestamp>.<raw body>">` |
| `Content-Type` | `application/json` |

#### Request Body (sent by FedaPay)

```json
{
    "name": "transaction.approved",
    "description": "...",
    "entity": {
        "id": 123456,
        "klass": "Transaction",
        "reference": "TM-PAY-2026-0001",
        "amount": 11000,
        "status": "approved",
        "mode": "live",
        "created_at": "2026-04-15T08:00:00Z",
        "updated_at": "2026-04-15T08:02:30Z"
    }
}
```

#### Handled Events

| Event | Action |
|-------|--------|
| `transaction.approved` / `transaction.transferred` | Payment → `held`, booking → `approved` (auto-approval) or `pendingApproval`. If the payment had already been failed (late approval), the seats are re-reserved; if the trip is full, the passenger is refunded in full (`seatsUnavailable`). If the booking can no longer be confirmed (`cancelled`, `rejected`, etc.), the passenger is refunded in full (`bookingUnavailable`) and `PAYMENT_COMPLETED` is not published. |
| `transaction.declined` / `transaction.canceled` | Payment **stays `pending`** (`ProviderStatus` updated): with the redirect flow these statuses are not final and the passenger can retry on the same page. |
| `transaction.expired` / `transaction.deleted` | Payment → `failed`, booking → `paymentFailed`, seats released |
| `payout.completed` | Driver payout / refund → `completed` |

The same transition logic is shared by the webhook, `verifyPayment`, the callback and the expiration worker. It is idempotent.

An event is recorded on receipt and only marked as processed (`webhook_events.processed_at`) once it has been applied successfully. If processing fails (e.g. database error), an error is returned and the FedaPay retry replays the event; only events that were already processed are deduplicated.

#### Response

```json
{ "Success": true, "ErrorMessage": "" }
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidSignature` | 422 | HMAC signature mismatch |
| `ErrorDuplicateWebhookEvent` | 422 | Event already processed (idempotent) |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/payment/getRefundStatus

Returns the details and status of a refund.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `RefundId` | string | Yes | Refund UUID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "RefundId": "ref-550e8400-e29b-41d4-a716-446655440040",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "RefundReason": "cancelledByPassenger",
    "RefundRule": "cancelled24hBefore",
    "OriginalAmount": 10000,
    "RefundPercentage": 100,
    "RefundAmount": 10000,
    "AmountToPassenger": 10000,
    "AmountToDriver": 0,
    "AmountToPlatform": 1000,
    "Status": "completed",
    "ProcessedAt": "2026-04-14T10:00:00Z",
    "CompletedAt": "2026-04-14T10:01:00Z",
    "ErrorMessage": ""
}
```

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorRefundNotFound` | 404 | Refund does not exist |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/payment/getRefundByBooking

Returns the most recent refund associated with a booking. Symmetric to
`getPaymentByBooking`: the mobile client only knows the `BookingId` — the `RefundId` is
assigned server-side when booking-service triggers the refund (cancellation, no-show) or
when a worker creates it later. Poll this endpoint after a cancellation until it stops
returning `ErrorRefundNotFound`.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "RefundId": "ref-550e8400-e29b-41d4-a716-446655440040",
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "RefundReason": "cancelledByDriver",
    "RefundRule": "driverCancellation",
    "OriginalAmount": 10000,
    "RefundPercentage": 100,
    "RefundAmount": 10000,
    "AmountToPassenger": 10000,
    "AmountToDriver": 0,
    "AmountToPlatform": 0,
    "Status": "pending",
    "ProcessedAt": "",
    "CompletedAt": "",
    "ErrorMessage": ""
}
```

Same fields as `getRefundStatus`. `Status` is one of `pending` (awaiting the refund
worker), `processing`, `completed` (FedaPay payout done) or `failed`; `ProcessedAt` and
`CompletedAt` stay empty until the corresponding step happens.

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | `BookingId` is empty |
| `ErrorRefundNotFound` | 404 | No refund exists (yet) for this booking |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/payment/getPayoutStatus

Returns the details and status of a driver payout.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `PayoutId` | string | Yes | Payout UUID |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "PayoutId": "pout-550e8400-e29b-41d4-a716-446655440050",
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "GrossAmount": 10000,
    "PlatformFee": 1000,
    "NetAmount": 9000,
    "Status": "completed",
    "ScheduledAt": "2026-04-17T10:00:00Z",
    "CompletedAt": "2026-04-17T10:05:00Z",
    "FailureReason": "",
    "ErrorMessage": ""
}
```

#### Payout Statuses

| Status | Description |
|--------|-------------|
| `pending` | Payout queued, not yet scheduled |
| `scheduled` | Payout scheduled for processing |
| `processing` | Payout being sent to FedaPay |
| `completed` | Payout successfully sent to driver |
| `failed` | Payout failed (see `FailureReason`) |
| `cancelled` | Payout cancelled |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorPayoutNotFound` | 404 | Payout does not exist |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /api/v1/payment/getDriverPayouts

Returns the paginated list of payouts for a driver.

**Authentication:** Firebase JWT required

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DriverId` | string | Yes | Driver's internal user ID |
| `PageIndex` | integer | No | Page index (0-based, 10 items/page) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Payouts": [
        {
            "PayoutId": "pout-550e8400-e29b-41d4-a716-446655440050",
            "PayoutReference": "TM-POUT-2026-0001",
            "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
            "NetAmount": 9000,
            "Status": "completed",
            "CompletedAt": "2026-04-17T10:05:00Z",
            "CreatedAt": "2026-04-15T14:00:00Z"
        }
    ],
    "ErrorMessage": ""
}
```

---

### GET /api/v1/payment/health

Health check endpoint.

**Authentication:** Not required (public)

#### Response

```json
{
    "Status": "ok",
    "Version": "1.0.0",
    "Timestamp": 1742040000
}
```

---

## Internal Endpoints (inter-service only)

These routes are called by booking-service and are not accessible to mobile clients.

### POST /api/v1/payment/internal/requestRefund

Creates a refund for a booking. Called by booking-service on cancellation, rejection, or no-show.

#### Request

```json
{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "RefundReason": "cancelledByPassenger",
    "OriginalAmount": 10000,
    "ServiceFee": 1000,
    "DepartureDatetime": "2026-04-15T08:00:00Z",
    "ApprovedAt": "2026-04-14T10:00:00Z",
    "CancelledAt": "2026-04-14T11:00:00Z"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `RefundReason` | string | Yes | See refund reasons below |
| `OriginalAmount` | integer | Yes | Booking subtotal (excluding service fee) in XOF |
| `ServiceFee` | integer | Yes | Service fee in XOF |
| `DepartureDatetime` | string | Yes | Trip departure datetime (RFC3339) |
| `ApprovedAt` | string | No | When booking was approved (RFC3339) — used for grace period calculation |
| `CancelledAt` | string | Yes | When cancellation occurred (RFC3339) |

#### Refund Reasons

| Reason | Description |
|--------|-------------|
| `cancelledByDriver` | Driver cancelled the booking |
| `cancelledByPassenger` | Passenger cancelled the booking |
| `bookingRejected` | Driver rejected the booking |
| `noShowDriver` | Driver did not show up |
| `noShowPassenger` | Passenger did not show up |
| `tripCancelled` | Trip was cancelled |
| `waypointCancelled` | The driver removed the passenger's pickup or dropoff stop |
| `seatsUnavailable` | Internal only: payment approved by FedaPay after the seats were released, and the trip is full. Full refund. |
| `bookingUnavailable` | Internal only: payment approved by FedaPay for a booking that can no longer be confirmed (`cancelled`, `rejected`, etc.). Full refund. |

#### Response

```json
{
    "RefundId": "ref-550e8400-e29b-41d4-a716-446655440040",
    "RefundReference": "TM-REF-2026-0001",
    "Status": "pending",
    "RefundAmount": 10000,
    "ErrorMessage": ""
}
```

---

### POST /api/v1/payment/internal/releasePayment

Releases a held payment to the driver after the contestation delay. Called by booking-service's background worker.

#### Request

```json
{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020"
}
```

#### Response

```json
{
    "Success": true,
    "Status": "released",
    "ErrorMessage": ""
}
```

---

## Refund Rules

The refund amount depends on who triggered the cancellation and when it occurred relative to departure.

| Scenario | Rule | Passenger refund | Driver receives | Platform receives |
|----------|------|-----------------|-----------------|-------------------|
| Driver cancellation / Booking rejected / Trip cancelled | `driverCancellation` | 100% (subtotal + service fee) | 0 | 0 |
| Driver no-show | `noShowDriver` | 100% (subtotal + service fee) | 0 | 0 |
| Passenger cancels > `CANCELLATION_FULL_REFUND_HOURS` before departure (default 24h) | `cancelled24hBefore` | 100% of subtotal | 0 | service fee |
| Passenger cancels within `CANCELLATION_GRACE_PERIOD_MINUTES` after approval (default 15min) | `cancelled30minAfterApproval` | 100% of subtotal | 0 | service fee |
| Passenger cancels after grace period and < 24h before departure | `cancelledOver30minAfterApproval` | 50% of subtotal | 50% of subtotal | service fee |
| Passenger no-show | `noShowPassenger` | 0 | subtotal | service fee |

> Configurable via env vars: `CANCELLATION_FULL_REFUND_HOURS` (default `24`), `CANCELLATION_GRACE_PERIOD_MINUTES` (default `15`).

---

## Payment Lifecycle

```
                 approved (webhook / verifyPayment / callback / worker)
pending ───────────────────────────────────────────────► held → released → paidOut
  │  ▲ declined / canceled (retry possible, stays pending)    │
  │  └──────────┘                                            └──► refunded
  │
  │ expired / cancelPayment / 10-min window over
  │ (FedaPay transaction DELETEd, seats released)
  ▼
failed ── late approval re-checked with FedaPay ──► held (seats re-reserved) or refunded (trip full)

approval for a booking that is no longer confirmable (cancelled, rejected…) ──► refunded (bookingUnavailable)
```

| Status | Description |
|--------|-------------|
| `pending` | FedaPay payment page created, waiting for the passenger (10-minute window from booking creation) |
| `held` | Payment confirmed, funds held during contestation period (default 2h after trip completion) |
| `released` | Contestation period expired, funds released for payout |
| `paidOut` | Driver payout sent via FedaPay |
| `refunded` | Full or partial refund processed |
| `failed` | Window expired or payment cancelled; FedaPay transaction deleted, booking `paymentFailed`, seats released |

### Expiration and seat release

- **payment-service expiration worker** (every `EXPIRATION_INTERVAL_SECONDS`, Redis lock): for every `pending` payment with `expires_at < now`, it checks FedaPay.
  - If the transaction was approved, it applies the approval.
  - Otherwise it deletes the transaction (`DELETE /v1/transactions/{id}`), marks the payment `failed`, calls booking-service `FailPayment` (seats released) and publishes `PAYMENT_FAILED`.
- **booking-service sweeper** (`PAYMENT_PENDING_TIMEOUT_MINUTES`, default 12): it handles `paymentPending` bookings whose payment never started (`createPayment` was never called) or whose confirmation was lost.
  - No payment, `failed` or `refunded`: `FailPayment`.
  - `held`: `ConfirmPayment`.
- **Held payment not attached to its booking** (booking-service unreachable during the approval, typically a late approval on a `paymentFailed` booking, which the booking-service sweeper does not see): the payment stays `held` with `booking_synced_at` NULL. On every tick, under the same Redis lock, the expiration worker retries the payments that have been `held` for more than 2 minutes (`RetryUnsyncedHeldPayments`, batches of 100).
  - `ConfirmPayment` accepted, or booking already confirmed: the payment is marked as attached (`booking_synced_at`).
  - Trip full (`seatsUnavailable`) or booking no longer confirmable (`bookingUnavailable`): full refund.
  - booking-service still unreachable: the attempt is counted (`booking_sync_attempts`) and retried on the next tick.
  - After **5 failed attempts**, the payment is no longer attached: the passenger is refunded in full automatically (`bookingUnavailable`, payment → `refunded`). If booking-service is still unreachable, the collected amount (`Amount`) is refunded as is. A failed refund is retried on the next tick, without calling `ConfirmPayment` again.
  - A refund already exists for the payment: marked as attached, no second refund.
- **Booking cancelled while `paymentPending`**: booking-service first calls `CancelPaymentForBooking` (internal gRPC), so the FedaPay link stops working before the seats are released.
- **Trip cancelled or stop removed by the driver** (`CancelBookingsForTrip` / `CancelBookingsForWaypoint`): bookings are cancelled first, then booking-service calls `CancelPaymentForBooking` asynchronously for each `paymentPending` booking (and `RequestRefund` with `tripCancelled` / `waypointCancelled` for paid ones).
  - If FedaPay had already approved, the approval reaches a cancelled booking and the passenger is refunded in full (`bookingUnavailable`).
  - If the call fails, the expiration worker deletes the transaction when the payment window ends.

### Passenger notifications (refunds)

Events published on the `notification-events` Redis stream (`ReferenceType` = `refund`, `ReferenceID` = refund ID):

| Event | When | Payload |
|-------|------|---------|
| `REFUND_PROCESSED` | Refund created (passenger cancellation, driver rejection, unconfirmable booking, etc.) | `amount`, `amount_to_passenger`, `reason` |
| `REFUND_COMPLETED` | FedaPay payout to the passenger sent by the refund worker | `amount`, `currency`, `payment_provider_reference`, `reason` |
| `REFUND_FAILED` | Refund failed for good (max retries exceeded, missing phone number): handled manually by support | `amount`, `currency`, `reason` |

`reason` is one of the [refund reasons](#refund-reasons) (e.g. `bookingUnavailable`), so the app can tell the passenger why they are refunded.

---

## Error Reference

| Error | HTTP Code | gRPC Code | Description |
|-------|-----------|-----------|-------------|
| `ErrorInvalidInput` | 400 | INVALID_ARGUMENT (3) | Missing or invalid fields |
| `ErrorInvalidPaymentMethod` | 400 | INVALID_ARGUMENT (3) | Unsupported payment method |
| `ErrorInvalidSignature` | 422 | FAILED_PRECONDITION (9) | FedaPay webhook HMAC mismatch |
| `ErrorPaymentNotFound` | 404 | NOT_FOUND (5) | Payment does not exist |
| `ErrorRefundNotFound` | 404 | NOT_FOUND (5) | Refund does not exist |
| `ErrorPayoutNotFound` | 404 | NOT_FOUND (5) | Payout does not exist |
| `ErrorDuplicatePayment` | 409 | ALREADY_EXISTS (6) | Booking already paid |
| `ErrorBookingNotPayable` | 400 | FAILED_PRECONDITION (9) | Booking is cash or no longer `paymentPending` |
| `ErrorBookingPaymentExpired` | 400 | FAILED_PRECONDITION (9) | 10-minute payment window over |
| `ErrorPaymentNotCancellable` | 400 | FAILED_PRECONDITION (9) | Payment already released / paid out / refunded |
| `ErrorPaymentNotOwned` | 403 | PERMISSION_DENIED (7) | Caller is not the booking's passenger |
| `ErrorFedaPayAPIError` | 503 | UNAVAILABLE (14) | FedaPay unavailable |
| `ErrorDuplicateWebhookEvent` | 422 | FAILED_PRECONDITION (9) | Webhook event already processed |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Internal server error |
