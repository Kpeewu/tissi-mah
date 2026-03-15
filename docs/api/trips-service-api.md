# Trips Service API

This document describes the HTTP/REST API exposed by the trips-service through the api-gateway (grpc-gateway). This API is consumed by mobile clients (iOS/Android).

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://dev.tissi-mah.com` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

All `/trip/driver/*` endpoints require a valid **Firebase JWT** in the `Authorization` header.
The `DriverId` is provided in the request body and corresponds to the user's **internal ID** in the `users` table (UUID), not the Firebase UID.

The `/trip/health` endpoint is **public** (no token required).

```http
Authorization: Bearer <firebase-id-token>
```

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes (POST/PATCH) | `application/json` |
| `Authorization` | Yes (driver routes) | `Bearer <firebase-id-token>` |
| `Accept` | No | `application/json` |

## Error Response Format

All errors follow this format:

```json
{
    "ErrorMessage": "ErrorTripNotFound"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Error identifier (matches backend sentinel error name) |

---

## Endpoints

### POST /trip/driver/createTrip

Creates a new trip with its waypoints.

**Authentication:** Firebase JWT required

#### Request

```http
POST /trip/driver/createTrip HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
    "DepartureDatetime": "2026-04-15T08:00:00Z",
    "EstimatedArrivalDatetime": "2026-04-15T12:00:00Z",
    "EstimatedDurationMinutes": 240,
    "EstimatedDistanceMeters": 350000,
    "TotalSeats": 4,
    "PricePerSeat": 5000,
    "AllowLuggages": true,
    "AllowPets": false,
    "AllowSmoking": false,
    "AllowFood": true,
    "AutoApprove": false,
    "Description": "Trajet Dakar → Saint-Louis, départ depuis Liberté 6",
    "PaymentMethodsAccepted": ["mobileMoney", "cash"],
    "TripWaypoints": [
        {
            "SequencerOrder": 1,
            "WaypointType": "departure",
            "LocationName": "Liberté 6",
            "LocationLng": -17.4441,
            "LocationLat": 14.7167,
            "City": "Dakar",
            "Country": "Sénégal",
            "ScheduledDatetime": "",
            "PriceFromPrevious": 0
        },
        {
            "SequencerOrder": 2,
            "WaypointType": "stop",
            "LocationName": "Thiès Centre",
            "LocationLng": -16.9356,
            "LocationLat": 14.7886,
            "City": "Thiès",
            "Country": "Sénégal",
            "ScheduledDatetime": "2026-04-15T09:30:00Z",
            "PriceFromPrevious": 2000
        },
        {
            "SequencerOrder": 3,
            "WaypointType": "arrival",
            "LocationName": "Saint-Louis Centre",
            "LocationLng": -16.5085,
            "LocationLat": 16.0179,
            "City": "Saint-Louis",
            "Country": "Sénégal",
            "ScheduledDatetime": "2026-04-15T12:00:00Z",
            "PriceFromPrevious": 3000
        }
    ]
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `VehicleId` | string | Yes | UUID of the vehicle to use |
| `DepartureDatetime` | string | Yes | Departure datetime (RFC3339) |
| `EstimatedArrivalDatetime` | string | Yes | Estimated arrival datetime (RFC3339), must be after departure |
| `EstimatedDurationMinutes` | integer | Yes | Estimated trip duration in minutes |
| `EstimatedDistanceMeters` | integer | Yes | Estimated trip distance in meters |
| `TotalSeats` | integer | Yes | Number of available seats |
| `PricePerSeat` | integer | Yes | Price per seat in local currency (XOF) |
| `AllowLuggages` | boolean | Yes | Whether luggage is allowed |
| `AllowPets` | boolean | Yes | Whether pets are allowed |
| `AllowSmoking` | boolean | Yes | Whether smoking is allowed |
| `AllowFood` | boolean | Yes | Whether food is allowed |
| `AutoApprove` | boolean | Yes | Automatically approve passenger requests |
| `Description` | string | No | Optional trip description |
| `PaymentMethodsAccepted` | array | Yes | Accepted payment methods: `mobileMoney`, `card`, `paypal`, `cash` |
| `TripWaypoints` | array | Yes | List of waypoints (min 2: departure + arrival) |
| `TripWaypoints[].SequencerOrder` | integer | Yes | Order of the waypoint (1-based) |
| `TripWaypoints[].WaypointType` | string | Yes | `departure`, `stop`, or `arrival` |
| `TripWaypoints[].LocationName` | string | Yes | Human-readable location name |
| `TripWaypoints[].LocationLng` | float | Yes | Longitude (WGS84) |
| `TripWaypoints[].LocationLat` | float | Yes | Latitude (WGS84) |
| `TripWaypoints[].City` | string | Yes | City name |
| `TripWaypoints[].Country` | string | Yes | Country name |
| `TripWaypoints[].ScheduledDatetime` | string | No | Scheduled datetime for this waypoint (RFC3339, empty for departure) |
| `TripWaypoints[].PriceFromPrevious` | integer | No | Price from previous waypoint (0 for departure) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `TripId` | string | UUID of the created trip |
| `ErrorMessage` | string | Error identifier if failed, empty if success |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing or invalid required fields |
| `ErrorInvalidDatetime` | 400 | Invalid datetime format or arrival before departure |
| `ErrorInvalidWaypoints` | 400 | Invalid waypoints (missing departure/arrival, wrong order) |
| `ErrorDriverNotFound` | 404 | Driver does not exist |
| `ErrorDriverNotVerified` | 403 | Driver not verified/certified |
| `ErrorVehicleNotFound` | 404 | Vehicle does not exist or does not belong to driver |
| `ErrorVehicleInsufficientSeats` | 422 | Vehicle does not have enough seats |
| `ErrorInternalServer` | 500 | Internal server error |

#### Example (cURL)

```bash
curl -X POST https://api.tissi-mah.com/trip/driver/createTrip \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "VehicleId": "v-550e8400",
    "DepartureDatetime": "2026-04-15T08:00:00Z",
    "EstimatedArrivalDatetime": "2026-04-15T12:00:00Z",
    "EstimatedDurationMinutes": 240,
    "EstimatedDistanceMeters": 350000,
    "TotalSeats": 4,
    "PricePerSeat": 5000,
    "AllowLuggages": true,
    "AllowPets": false,
    "AllowSmoking": false,
    "AllowFood": true,
    "AutoApprove": false,
    "PaymentMethodsAccepted": ["mobileMoney", "cash"],
    "TripWaypoints": [
      {"SequencerOrder": 1, "WaypointType": "departure", "LocationName": "Liberté 6", "LocationLng": -17.4441, "LocationLat": 14.7167, "City": "Dakar", "Country": "Sénégal"},
      {"SequencerOrder": 2, "WaypointType": "arrival", "LocationName": "Saint-Louis Centre", "LocationLng": -16.5085, "LocationLat": 16.0179, "City": "Saint-Louis", "Country": "Sénégal", "PriceFromPrevious": 5000}
    ]
  }'
```

---

### POST /trip/driver/createRecurringTrip

Creates a recurring trip pattern (daily, weekly, or custom days).

**Authentication:** Firebase JWT required

#### Request

```http
POST /trip/driver/createRecurringTrip HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
    "DepartureTime": "2026-04-15T07:30:00Z",
    "RecurrenceType": "weekly",
    "DaysOfWeek": { "Days": [1, 3, 5] },
    "StartDate": "2026-04-15",
    "EndDate": "2026-06-30",
    "TotalSeats": 3,
    "PricePerSeat": 4000,
    "AllowLuggages": true,
    "AllowPets": false,
    "AllowFood": true,
    "AllowSmoking": false,
    "AutoApprove": true,
    "Description": "Trajet hebdomadaire Dakar → Thiès",
    "GenerationHorizonDays": 30,
    "TripWaypoints": [...]
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `VehicleId` | string | Yes | UUID of the vehicle |
| `DepartureTime` | string | Yes | Departure time (RFC3339, only time portion is used) |
| `RecurrenceType` | string | Yes | `daily`, `weekly`, or `custom` |
| `DaysOfWeek` | object | No | Days for `weekly`/`custom` recurrence (1=Monday … 7=Sunday) |
| `DaysOfWeek.Days` | array\<integer\> | No | List of day numbers (1–7) |
| `StartDate` | string | Yes | Start date (YYYY-MM-DD) |
| `EndDate` | string | Yes | End date (YYYY-MM-DD) |
| `TotalSeats` | integer | Yes | Number of seats per trip instance |
| `PricePerSeat` | integer | Yes | Price per seat (XOF) |
| `AllowLuggages` | boolean | Yes | Whether luggage is allowed |
| `AllowPets` | boolean | Yes | Whether pets are allowed |
| `AllowFood` | boolean | Yes | Whether food is allowed |
| `AllowSmoking` | boolean | Yes | Whether smoking is allowed |
| `AutoApprove` | boolean | Yes | Automatically approve requests |
| `Description` | string | No | Optional description |
| `GenerationHorizonDays` | integer | Yes | How many days ahead to pre-generate trip instances |
| `TripWaypoints` | array | Yes | Same structure as `CreateTrip` |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing or invalid required fields |
| `ErrorInvalidDatetime` | 400 | Invalid date/time format |
| `ErrorInvalidWaypoints` | 400 | Invalid waypoints |
| `ErrorDriverNotFound` | 404 | Driver does not exist |
| `ErrorDriverNotVerified` | 403 | Driver not verified/certified |
| `ErrorVehicleNotFound` | 404 | Vehicle does not exist |
| `ErrorVehicleInsufficientSeats` | 422 | Vehicle does not have enough seats |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /trip/driver/getTripsPreviews

Returns a paginated list of the driver's trips with status different from `completed`.

**Authentication:** Firebase JWT required

#### Request

```http
GET /trip/driver/getTripsPreviews?DriverId=550e8400-e29b-41d4-a716-446655440001&Index=0 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `Index` | integer | Yes | Page index (0-based, 10 items per page) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "TripsPreviews": [
        {
            "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
            "DriverId": "550e8400-e29b-41d4-a716-446655440001",
            "DriverName": "Mamadou Diallo",
            "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
            "VehicleBrand": "Toyota",
            "VehiclePlate": "DK-1234-AB",
            "DepartureDate": "2026-04-15",
            "DepartureTime": "08:00",
            "TotalSeats": 4,
            "AvailableSeats": 3,
            "DepartureLocationName": "Liberté 6",
            "ArrivalLocationName": "Saint-Louis Centre"
        }
    ],
    "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `TripsPreviews` | array | List of trip previews |
| `TripsPreviews[].TripId` | string | Trip UUID |
| `TripsPreviews[].DriverId` | string | Driver internal user ID (UUID) |
| `TripsPreviews[].DriverName` | string | Driver full name (from user-service) |
| `TripsPreviews[].VehicleId` | string | Vehicle UUID |
| `TripsPreviews[].VehicleBrand` | string | Vehicle brand (from vehicle-service) |
| `TripsPreviews[].VehiclePlate` | string | Vehicle plate number |
| `TripsPreviews[].DepartureDate` | string | Departure date (YYYY-MM-DD) |
| `TripsPreviews[].DepartureTime` | string | Departure time (HH:MM) |
| `TripsPreviews[].TotalSeats` | integer | Total seats |
| `TripsPreviews[].AvailableSeats` | integer | Remaining available seats |
| `TripsPreviews[].DepartureLocationName` | string | Name of the departure location |
| `TripsPreviews[].ArrivalLocationName` | string | Name of the arrival location |
| `ErrorMessage` | string | Error identifier if failed, empty if success |

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorDriverNotFound` | 404 | Driver does not exist |
| `ErrorDataRetrievalFailed` | 500 | Database query failed |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /trip/driver/getCompletedTripsPreviews

Returns a paginated list of the driver's completed trips.

**Authentication:** Firebase JWT required

#### Request

```http
GET /trip/driver/getCompletedTripsPreviews?DriverId=550e8400-e29b-41d4-a716-446655440001&Index=0 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `Index` | integer | Yes | Page index (0-based, 10 items per page) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "TripsPreviews": [
        {
            "TripId": "t-660f9511-e29b-41d4-a716-446655440000",
            "DriverId": "550e8400-e29b-41d4-a716-446655440001",
            "DriverName": "Mamadou Diallo",
            "VehicleId": "v-550e8400-e29b-41d4-a716-446655440000",
            "VehicleBrand": "Toyota",
            "VehiclePlateNumber": "DK-1234-AB",
            "DepartureDate": "2026-03-10",
            "DepartureTime": "08:00",
            "TotalSeats": 4,
            "AvailableSeats": 1,
            "DepartureLocationName": "Liberté 6",
            "ArrivalLocationName": "Saint-Louis Centre"
        }
    ],
    "ErrorMessage": ""
}
```

> **Note:** The plate number field is named `VehiclePlateNumber` (vs `VehiclePlate` in `GetTripsPreviews`).

#### Errors

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorDriverNotFound` | 404 | Driver does not exist |
| `ErrorDataRetrievalFailed` | 500 | Database query failed |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/changeTripDateAndTime

Changes the departure datetime of a scheduled trip.

**Authentication:** Firebase JWT required
**Constraint:** Trip must have status `scheduled`.

#### Request

```http
PATCH /trip/driver/changeTripDateAndTime HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "DepartureDatetime": "2026-04-16T09:00:00Z"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `TripId` | string | Yes | UUID of the trip to update |
| `DepartureDatetime` | string | Yes | New departure datetime (RFC3339) |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorInvalidDatetime` | 400 | Invalid datetime format |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this trip |
| `ErrorTripNotScheduled` | 422 | Trip is not in `scheduled` status |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/changeTripVehicle

Changes the vehicle associated with a scheduled trip.

**Authentication:** Firebase JWT required
**Constraint:** Trip must have status `scheduled`.

#### Request

```http
PATCH /trip/driver/changeTripVehicle HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "VehicleId": "v-661f9511-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `TripId` | string | Yes | UUID of the trip to update |
| `VehicleId` | string | Yes | UUID of the new vehicle |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this trip |
| `ErrorTripNotScheduled` | 422 | Trip is not in `scheduled` status |
| `ErrorVehicleNotFound` | 404 | Vehicle does not exist or does not belong to driver |
| `ErrorVehicleInsufficientSeats` | 422 | New vehicle does not have enough seats |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/changeTripAllowances

Changes the allowances (pets, food, smoking, luggage) of a scheduled trip.

**Authentication:** Firebase JWT required
**Constraint:** Trip must have status `scheduled`. Cannot be changed within 24h of departure.

#### Request

```http
PATCH /trip/driver/changeTripAllowances HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "AllowPets": true,
    "AllowFood": true,
    "AllowSmoking": false,
    "AllowLuggage": true
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `TripId` | string | Yes | UUID of the trip to update |
| `AllowPets` | boolean | Yes | Whether pets are allowed |
| `AllowFood` | boolean | Yes | Whether food is allowed |
| `AllowSmoking` | boolean | Yes | Whether smoking is allowed |
| `AllowLuggage` | boolean | Yes | Whether luggage is allowed |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this trip |
| `ErrorTripNotScheduled` | 422 | Trip is not in `scheduled` status |
| `ErrorTripDepartureTooSoon` | 422 | Cannot change allowances within 24h of departure |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/activeAutoApprouve

Enables or disables automatic passenger approval for a trip.

**Authentication:** Firebase JWT required
**Constraint:** Trip must have status `scheduled` or `inProgress`.

#### Request

```http
PATCH /trip/driver/activeAutoApprouve HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000",
    "AutoApprove": true
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `TripId` | string | Yes | UUID of the trip |
| `AutoApprove` | boolean | Yes | `true` to enable, `false` to disable |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this trip |
| `ErrorTripNotScheduled` | 422 | Trip status is not `scheduled` or `inProgress` |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/startTrip

Starts a scheduled trip, changing its status to `inProgress`.

**Authentication:** Firebase JWT required
**Constraint:** Trip must have status `scheduled`. The driver can only have one `inProgress` trip at a time.

#### Request

```http
PATCH /trip/driver/startTrip HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `TripId` | string | Yes | UUID of the trip to start |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this trip |
| `ErrorTripNotScheduled` | 422 | Trip is not in `scheduled` status |
| `ErrorDriverAlreadyHasActiveTrip` | 422 | Driver already has an `inProgress` trip |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/endTrip

Ends an in-progress trip, changing its status to `completed`.

**Authentication:** Firebase JWT required
**Constraint:** Trip must have status `inProgress`.

#### Request

```http
PATCH /trip/driver/endTrip HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "TripId": "t-550e8400-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `TripId` | string | Yes | UUID of the trip to end |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorTripNotFound` | 404 | Trip does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this trip |
| `ErrorTripNotInProgress` | 422 | Trip is not in `inProgress` status |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/confirmWaypointArrival

Records the driver's arrival at a `stop`-type waypoint.

**Authentication:** Firebase JWT required
**Constraint:** Waypoint must be of type `stop`. Previous waypoint must have been confirmed. No other stop can be currently active.

#### Request

```http
PATCH /trip/driver/confirmWaypointArrival HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "WaypointId": "w-550e8400-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `WaypointId` | string | Yes | UUID of the waypoint |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorWaypointNotFound` | 404 | Waypoint does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this waypoint's trip |
| `ErrorWaypointNotAStop` | 422 | Waypoint is not of type `stop` |
| `ErrorWaypointAlreadyArrived` | 422 | Arrival already confirmed for this waypoint |
| `ErrorPreviousWaypointNotConfirmed` | 422 | Previous waypoint has not been confirmed yet |
| `ErrorAnotherStopAlreadyActive` | 422 | Another stop is already active (arrived but not departed) |
| `ErrorInternalServer` | 500 | Internal server error |

---

### PATCH /trip/driver/confirmWaypointDeparture

Records the driver's departure from a `stop`-type waypoint.

**Authentication:** Firebase JWT required
**Constraint:** Arrival at this waypoint must have been confirmed first.

#### Request

```http
PATCH /trip/driver/confirmWaypointDeparture HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase-id-token>
Content-Type: application/json

{
    "DriverId": "550e8400-e29b-41d4-a716-446655440001",
    "WaypointId": "w-550e8400-e29b-41d4-a716-446655440000"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DriverId` | string | Yes | Internal user ID of the driver (UUID from the `users` table) |
| `WaypointId` | string | Yes | UUID of the waypoint |

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

| Error | HTTP Code | Description |
|-------|-----------|-------------|
| `ErrorInvalidInput` | 400 | Missing required fields |
| `ErrorWaypointNotFound` | 404 | Waypoint does not exist |
| `ErrorUnauthorized` | 403 | Driver does not own this waypoint's trip |
| `ErrorWaypointNotAStop` | 422 | Waypoint is not of type `stop` |
| `ErrorWaypointNotArrived` | 422 | Arrival has not been confirmed for this waypoint |
| `ErrorWaypointAlreadyDeparted` | 422 | Departure already confirmed for this waypoint |
| `ErrorInternalServer` | 500 | Internal server error |

---

### GET /trip/health

Health check endpoint.

**Authentication:** Not required (public)

#### Request

```http
GET /trip/health HTTP/1.1
Host: api.tissi-mah.com
```

#### Response

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Status": "SERVING",
    "Version": "1.0.0",
    "Timestamp": 1742040000
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `Status` | string | Always `SERVING` when the service is up |
| `Version` | string | Service version |
| `Timestamp` | integer | Unix timestamp of the response |

#### Example (cURL)

```bash
curl https://api.tissi-mah.com/trip/health
```

---

## Business Rules

### Trip Lifecycle

```
scheduled → inProgress → completed
```

| Status | Description |
|--------|-------------|
| `scheduled` | Trip created, not yet started |
| `inProgress` | Trip started by the driver |
| `completed` | Trip ended by the driver |

### Trip Constraints

| Rule | Description |
|------|-------------|
| **Arrival after departure** | `EstimatedArrivalDatetime` must be strictly after `DepartureDatetime` |
| **Waypoint ordering** | Waypoints must have sequential `SequencerOrder` values starting at 1 |
| **Minimum waypoints** | At least one `departure` and one `arrival` waypoint required |
| **Single active trip** | A driver can only have one `inProgress` trip at a time |
| **Allowances lock** | Allowances cannot be changed within 24h of departure |
| **Modify scheduled only** | Date, vehicle, and allowances can only be changed when status is `scheduled` |
| **AutoApprove scope** | AutoApprove can be changed for `scheduled` and `inProgress` trips |
| **Driver verified** | Only verified/certified drivers can create trips |

### Waypoint Rules

| Rule | Description |
|------|-------------|
| **Types** | `departure` (exactly 1), `arrival` (exactly 1), `stop` (0 or more) |
| **Arrival confirmation order** | Waypoints must be confirmed in sequence — previous waypoint must be confirmed first |
| **Active stop** | Only one `stop` can be in arrived-but-not-departed state at a time |
| **Departure requires arrival** | `ConfirmWaypointDeparture` requires prior `ConfirmWaypointArrival` |

---

## Error Reference

| Error | HTTP | gRPC Code | Description | User Action |
|-------|------|-----------|-------------|-------------|
| `ErrorInvalidInput` | 400 | INVALID_ARGUMENT (3) | Missing or invalid input fields | Fix the request body |
| `ErrorInvalidDatetime` | 400 | INVALID_ARGUMENT (3) | Invalid datetime format or logical error | Use RFC3339 format, check arrival > departure |
| `ErrorInvalidWaypoints` | 400 | INVALID_ARGUMENT (3) | Waypoints are invalid or incomplete | Ensure departure + arrival, valid sequence |
| `ErrorDriverNotFound` | 404 | NOT_FOUND (5) | Driver does not exist | Check DriverId |
| `ErrorTripNotFound` | 404 | NOT_FOUND (5) | Trip does not exist | Check TripId |
| `ErrorVehicleNotFound` | 404 | NOT_FOUND (5) | Vehicle does not exist or wrong owner | Check VehicleId |
| `ErrorWaypointNotFound` | 404 | NOT_FOUND (5) | Waypoint does not exist | Check WaypointId |
| `ErrorDriverNotVerified` | 403 | PERMISSION_DENIED (7) | Driver is not verified/certified | Complete driver verification |
| `ErrorUnauthorized` | 403 | PERMISSION_DENIED (7) | Driver does not own this resource | Use correct DriverId |
| `ErrorTripNotScheduled` | 422 | FAILED_PRECONDITION (9) | Trip status is not `scheduled` | Check trip status before calling |
| `ErrorTripNotInProgress` | 422 | FAILED_PRECONDITION (9) | Trip status is not `inProgress` | Start the trip first |
| `ErrorVehicleInsufficientSeats` | 422 | FAILED_PRECONDITION (9) | Vehicle does not have enough seats | Choose a vehicle with more seats |
| `ErrorTripDepartureTooSoon` | 422 | FAILED_PRECONDITION (9) | Cannot modify allowances within 24h of departure | Modify earlier |
| `ErrorDriverAlreadyHasActiveTrip` | 422 | FAILED_PRECONDITION (9) | Driver already has an `inProgress` trip | End the current trip first |
| `ErrorWaypointNotAStop` | 422 | FAILED_PRECONDITION (9) | Waypoint is not of type `stop` | Only confirm arrival/departure on stop waypoints |
| `ErrorWaypointAlreadyArrived` | 422 | FAILED_PRECONDITION (9) | Arrival already confirmed | No action needed |
| `ErrorWaypointNotArrived` | 422 | FAILED_PRECONDITION (9) | Arrival not yet confirmed | Confirm arrival first |
| `ErrorWaypointAlreadyDeparted` | 422 | FAILED_PRECONDITION (9) | Departure already confirmed | No action needed |
| `ErrorPreviousWaypointNotConfirmed` | 422 | FAILED_PRECONDITION (9) | Previous waypoint not yet confirmed | Confirm waypoints in sequence |
| `ErrorAnotherStopAlreadyActive` | 422 | FAILED_PRECONDITION (9) | Another stop is currently active | Confirm departure from current stop first |
| `ErrorDataRetrievalFailed` | 500 | INTERNAL (13) | Database query failed | Retry later |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Internal server error | Retry later |
