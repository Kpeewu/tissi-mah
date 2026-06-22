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
| `/api/v1/payment/createPayment` | Firebase JWT required |
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

### POST /api/v1/payment/createPayment

Initiates a FedaPay payment for a booking. Creates the transaction and sends a mobile money request to the passenger's phone.

**Authentication:** Firebase JWT required

> Currently only `mobileMoney` is supported. `card` and `paypal` are reserved for future use.

#### Request

```http
POST /api/v1/payment/createPayment HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "BookingId": "bk-550e8400-e29b-41d4-a716-446655440020",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "PaymentMethod": "mobileMoney",
    "Amount": 11000,
    "PassengerPhoneNumber": "+22890000000",
    "MobileMoneyMode": "moov_tg"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BookingId` | string | Yes | Booking UUID |
| `TripId` | string | Yes | Trip UUID |
| `PaymentMethod` | string | Yes | `mobileMoney` (only supported value currently) |
| `Amount` | integer | Yes | Total amount to charge in XOF (must equal booking `TotalAmount`) |
| `PassengerPhoneNumber` | string | Yes | Passenger's phone number in international format |
| `MobileMoneyMode` | string | Yes | `moov_tg` (Moov Togo) or `togocel` (Togocel) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "PaymentId": "pay-550e8400-e29b-41d4-a716-446655440030",
    "PaymentReference": "TM-PAY-2026-0001",
    "Status": "pending",
    "ErrorMessage": ""
}
```

#### Payment Flow After Creation

1. Payment is created with status `pending`
2. FedaPay sends a mobile money request to the passenger's phone
3. Passenger approves on their phone
4. FedaPay calls `POST /api/v1/payment/webhooks/fedapay` with the result
5. On `transaction.approved` → payment status becomes `held`, booking transitions to `pendingApproval`
6. On `transaction.declined` or `transaction.canceled` → payment status becomes `failed`

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing or invalid fields |
| `ErrorInvalidPaymentMethod` | 400 | Payment method not `mobileMoney` |
| `ErrorPaymentAlreadyExists` | 422 | A payment already exists for this booking |
| `ErrorInternalServer` | 500 | Internal server error |

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
    "ErrorMessage": ""
}
```

#### Payment Statuses

| Status | Description |
|--------|-------------|
| `pending` | Payment initiated, awaiting passenger confirmation |
| `held` | Payment confirmed by FedaPay, funds held pending trip completion |
| `released` | Funds released to driver after contestation delay |
| `paidOut` | Driver payout processed |
| `failed` | Payment declined or cancelled |
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
    "ErrorMessage": ""
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

> The api-gateway uses a raw HTTP handler for this route to preserve the exact body bytes needed for signature verification. The signature is computed as `HMAC-SHA256(secret, rawBody)` encoded as hex.

#### Headers

| Header | Description |
|--------|-------------|
| `X-FEDAPAY-SIGNATURE` | HMAC-SHA256 hex signature of the raw request body |
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
| `transaction.approved` | Payment → `held`, booking → `pendingApproval` (or `approved`) |
| `transaction.declined` | Payment → `failed` |
| `transaction.canceled` | Payment → `failed` |

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
pending → held → released → paidOut
            ↓
         refunded
            ↓
          failed
```

| Status | Description |
|--------|-------------|
| `pending` | FedaPay transaction initiated, awaiting mobile money confirmation |
| `held` | Payment confirmed, funds held during contestation period (default 2h after trip completion) |
| `released` | Contestation period expired, funds released for payout |
| `paidOut` | Driver payout sent via FedaPay |
| `refunded` | Full or partial refund processed |
| `failed` | Payment declined or cancelled by FedaPay |

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
| `ErrorPaymentAlreadyExists` | 422 | FAILED_PRECONDITION (9) | Payment already created for this booking |
| `ErrorDuplicateWebhookEvent` | 422 | FAILED_PRECONDITION (9) | Webhook event already processed |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Internal server error |
