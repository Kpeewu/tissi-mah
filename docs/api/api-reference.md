# TissiMah — Référence API

Guide complet pour intégrer l'API TissiMah depuis une application web ou mobile.

---

## Table des matières

1. [Environnements et URLs de base](#1-environnements-et-urls-de-base)
2. [Authentification](#2-authentification)
3. [Format des requêtes et réponses](#3-format-des-requêtes-et-réponses)
4. [Gestion des erreurs](#4-gestion-des-erreurs)
5. [Rate limiting](#5-rate-limiting)
6. [CORS](#6-cors)
7. [Endpoints par service](#7-endpoints-par-service)
   - [Auth](#auth-service)
   - [Utilisateur](#user-service)
   - [Véhicule](#vehicle-service)
   - [Fichiers / Documents](#file-service)
   - [KYC](#kyc-service)
   - [Trajets](#trips-service)
   - [Réservations](#booking-service)
   - [Paiement](#payment-service)
   - [Notation](#rating-service)
   - [Géolocalisation](#geolocation-service)
   - [Chat](#chat-service)
   - [Notifications](#notification-service)
   - [Support (back-office)](#support-service)
8. [Exemples d'intégration](#8-exemples-dintégration)

---

## 1. Environnements et URLs de base

| Environnement | Base URL |
|---|---|
| **Production** | `https://api.tissimah.kpeewu.dev` |
| **Local** | `http://localhost:8080` |

Toutes les requêtes passent par l'**api-gateway** — point d'entrée unique sur le port 8080. Il n'y a pas d'accès direct aux microservices.

**Limite de body :** 10 Mo par requête.

---

## 2. Authentification

L'API utilise deux schémas d'authentification distincts selon le type d'utilisateur.

### 2.1 Utilisateurs (Firebase JWT)

Pour les passagers et conducteurs. Le token est un **Firebase ID Token** obtenu via le SDK Firebase côté client.

```
Authorization: Bearer <firebase_id_token>
```

**Flux :**

```
1. Authentification Firebase (email/password, Google, téléphone…)
2. Récupérer l'ID Token → firebase.auth().currentUser.getIdToken()
3. Envoyer le token dans le header Authorization de chaque requête
4. L'api-gateway valide la signature, l'émetteur et l'audience
5. Le UID Firebase est automatiquement propagé aux microservices
```

**Durée de vie :** 1 heure. Rafraîchir avec `getIdToken(true)` (force refresh).

**Web — exemple Firebase :**
```javascript
const user = firebase.auth().currentUser;
const token = await user.getIdToken();

const response = await fetch('https://api.tissimah.kpeewu.dev/api/v1/user/me', {
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
});
```

**Mobile Flutter — exemple :**
```dart
final user = FirebaseAuth.instance.currentUser!;
final token = await user.getIdToken();

final response = await http.get(
  Uri.parse('https://api.tissimah.kpeewu.dev/api/v1/user/me'),
  headers: {
    'Authorization': 'Bearer $token',
    'Content-Type': 'application/json',
  },
);
```

### 2.2 Agents support (Support JWT)

Pour le back-office uniquement. Authentification par email + mot de passe + OTP.

```
Authorization: Bearer <support_access_token>
```

**Flux :**
```
1. POST /api/v1/support/login → { Email, Password } → reçoit OtpSessionId
2. POST /api/v1/support/verifyOtp → { OtpSessionId, Code } → reçoit AccessToken + RefreshToken
3. Envoyer AccessToken dans Authorization pour toutes les routes /api/v1/support/*
4. POST /api/v1/support/refreshToken pour renouveler avant expiration
```

### 2.3 Routes publiques

Ces endpoints n'exigent **aucun token** :

| Route | Description |
|---|---|
| `GET /api/v1/auth/checkEmail` | Vérifier si un email est disponible |
| `GET /api/v1/auth/checkPhoneNumber` | Vérifier si un numéro est disponible |
| `GET /api/v1/trip/passenger/getScheduledTripsPreviews` | Recherche de trajets |
| `GET /api/v1/trip/passenger/getTripDetails` | Détail d'un trajet (vue passager) |
| `POST /api/v1/geolocation/route` | Calcul d'itinéraire OSRM |
| `GET /api/v1/geolocation/geocode` | Géocodage (texte → coords) |
| `GET /api/v1/geolocation/reverse` | Géocodage inverse (coords → adresse) |
| `GET /*/health` | Health check de chaque service |

---

## 3. Format des requêtes et réponses

### Corps des requêtes

- **Content-Type :** `application/json`
- **Noms de champs :** PascalCase (ex: `BookingId`, `DepartureDatetime`)
- **Dates :** RFC3339 (ex: `"2024-12-01T08:00:00Z"`)
- **Coordonnées :** float64, WGS84 (ex: `"Lat": 6.1256, "Lng": 1.2317`)

### Corps des réponses

- Les champs vides sont toujours présents (`"ErrorMessage": ""` si succès)
- Les champs inconnus en entrée sont ignorés silencieusement

**Exemple de réponse succès :**
```json
{
  "BookingId": "b2c3d4e5-...",
  "BookingReference": "TMH-2024-001234",
  "Status": "pending",
  "TotalAmount": 2500,
  "ErrorMessage": ""
}
```

---

## 4. Gestion des erreurs

En cas d'erreur, l'API retourne :

```json
{
  "ErrorMessage": "<code_erreur>"
}
```

### Codes HTTP

| HTTP | gRPC | Signification |
|---|---|---|
| 200 | OK | Succès |
| 400 | INVALID_ARGUMENT | Paramètres invalides |
| 401 | UNAUTHENTICATED | Token absent ou expiré |
| 403 | PERMISSION_DENIED | Action non autorisée |
| 404 | NOT_FOUND | Ressource introuvable |
| 409 | ALREADY_EXISTS | Conflit (doublon) |
| 412 | FAILED_PRECONDITION | Précondition non remplie |
| 413 | — | Body trop grand (> 10 Mo) |
| 429 | — | Rate limit dépassé |
| 500 | INTERNAL | Erreur serveur |
| 503 | UNAVAILABLE | Service indisponible |

### Gestion côté client recommandée

```javascript
async function apiCall(url, options = {}) {
  const user = firebase.auth().currentUser;
  const token = await user.getIdToken();

  const res = await fetch(url, {
    ...options,
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (res.status === 401) {
    // Token expiré — forcer le refresh et réessayer une fois
    const freshToken = await user.getIdToken(true);
    // … retry
  }

  if (res.status === 429) {
    // Rate limit : attendre avant de réessayer
    const retryAfter = res.headers.get('Retry-After') ?? 60;
    await sleep(retryAfter * 1000);
    // … retry
  }

  const data = await res.json();

  if (!res.ok || data.ErrorMessage) {
    throw new ApiError(data.ErrorMessage, res.status);
  }

  return data;
}
```

---

## 5. Rate limiting

Sliding window Redis par utilisateur authentifié (UID Firebase) ou par IP pour les routes publiques.

| Tier | Routes concernées | /min | /heure |
|---|---|---|---|
| **global** | Toutes les autres routes | 60 | 1 000 |
| **auth** | `checkEmail`, `checkPhoneNumber` | 10 | 100 |
| **create** | `createAccount`, `POST …/messages` (chat) | 3 | 10 |
| **sensitive** | `deleteAccount`, `POST …/flag` (chat) | 1 | 5 |

**Headers de réponse :**
```
X-RateLimit-Limit-Minute: 60
X-RateLimit-Remaining-Minute: 58
X-RateLimit-Limit-Hour: 1000
X-RateLimit-Remaining-Hour: 997
```

Quand la limite est atteinte : **HTTP 429**. Implémenter un exponential backoff.

---

## 6. CORS

| Environnement | Origines autorisées |
|---|---|
| Production | `https://tissi-mah.com`, `https://www.tissi-mah.com` |
| VPS-Dev | `*` |

**Méthodes autorisées :** GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD

**Headers exposés :** `X-Request-ID`, `X-RateLimit-Limit-Minute`, `X-RateLimit-Remaining-Minute`, `X-RateLimit-Limit-Hour`, `X-RateLimit-Remaining-Hour`

Les requêtes preflight OPTIONS sont mises en cache 3 600 secondes.

---

## 7. Endpoints par service

> **Légende :**
> - 🔒 Firebase JWT requis
> - 🛡️ Support JWT requis
> - 🌐 Public (aucun token)

---

### Auth Service

Base : `/api/v1/auth`

#### `POST /api/v1/auth/createAccount` 🔒

Crée le compte utilisateur après la première connexion Firebase.

**Body :**
```json
{
  "Name": "Touré",
  "FirstName": "Ydaou",
  "PhoneNumber": "+22890000000",
  "Email": "ydaou@example.com",
  "ProfileImageURL": "",
  "BirthDate": "1995-01-15"
}
```

**Réponse :**
```json
{
  "ErrorMessage": "",
  "User": {
    "AuthID": "firebase-uid-xxx",
    "UserID": "uuid-xxx",
    "Name": "Touré",
    "FirstName": "Ydaou",
    "Email": "ydaou@example.com",
    "PhoneNumber": "+22890000000",
    "ProfileImageURL": ""
  }
}
```

#### `GET /api/v1/auth/checkEmail?Email=foo@bar.com` 🌐

```json
{ "ErrorMessage": "", "IsAvailable": true }
```

#### `GET /api/v1/auth/checkPhoneNumber?PhoneNumber=%2B22890000000` 🌐

```json
{ "ErrorMessage": "", "IsAvailable": true }
```

#### `DELETE /api/v1/auth/deleteAccount` 🔒

```json
{ "ErrorMessage": "", "Success": true }
```

---

### User Service

Base : `/api/v1/user` et `/api/v1/userProfile`

#### `GET /api/v1/user/me` 🔒

Profil complet de l'utilisateur connecté.

```json
{
  "ErrorMessage": "",
  "User": {
    "AuthID": "firebase-uid",
    "UserID": "uuid",
    "Name": "Touré",
    "FirstName": "Ydaou",
    "Email": "ydaou@example.com",
    "PhoneNumber": "+22890000000",
    "ProfileImageURL": "https://...",
    "BirthDate": "1995-01-15",
    "IsDriver": false,
    "WithdrawNumber": "",
    "Bio": ""
  }
}
```

#### `PATCH /api/v1/userProfile/updateProfile` 🔒

```json
{
  "UserID": "uuid",
  "FirstName": "Ydaou",
  "LastName": "Touré",
  "PhoneNumber": "+22890000000",
  "Bio": "Conducteur ponctuel"
}
```

#### `PATCH /api/v1/userProfile/createDriverAccount` 🔒

Active le compte conducteur.

```json
{
  "UserID": "uuid",
  "CreateDriverAccount": true
}
```

#### `PATCH /api/v1/userProfile/addTripPreferences` 🔒

```json
{
  "UserID": "uuid",
  "Preferences": [
    { "Preference": "music", "IsAllowed": true },
    { "Preference": "smoking", "IsAllowed": false }
  ]
}
```

#### `PATCH /api/v1/userProfile/changeProfilePicture` 🔒

```json
{
  "UserID": "uuid",
  "NewProfilePicture": "<base64-encoded-bytes>"
}
```

---

### Vehicle Service

Base : `/vehicle`

#### `POST /api/v1/vehicle/add` 🔒

```json
{
  "UserId": "uuid",
  "Brand": "Toyota",
  "BrandModel": "Corolla",
  "Color": "Blanc",
  "NumberOfSeats": 4,
  "LicencePlate": "TG-1234-AB"
}
```

**Réponse :** `{ "VehicleId": "uuid", "ErrorMessage": "" }`

#### `POST /api/v1/vehicle/getUserVehicles` 🔒

```json
{ "UserId": "uuid" }
```

**Réponse :**
```json
{
  "Vehicles": [
    {
      "VehicleId": "uuid",
      "Brand": "Toyota",
      "BrandModel": "Corolla",
      "LicencePlate": "TG-1234-AB",
      "IsVerified": false
    }
  ],
  "ErrorMessage": ""
}
```

#### `POST /api/v1/vehicle/details` 🔒

```json
{ "UserId": "uuid", "VehicleId": "uuid" }
```

#### `PATCH /api/v1/vehicle/update` 🔒

```json
{ "UserId": "uuid", "VehicleId": "uuid", "Color": "Noir", "LicencePlate": "TG-5678-CD" }
```

#### `POST /api/v1/vehicle/delete` 🔒

```json
{ "UserId": "uuid", "VehicleId": "uuid" }
```

---

### File Service

Base : `/file`

> Les fichiers sont envoyés en **bytes encodés en base64** dans le corps JSON.
> Limite globale : 10 Mo par requête.

#### `POST /api/v1/file/uploadIdDocument` 🔒

```json
{
  "UserID": "uuid",
  "DocumentType": "IDCard",
  "IDCardRecto": "<base64>",
  "IDCardVerso": "<base64>"
}
```

`DocumentType` : `IDCard` | `Passport` | `DriverLicence`

Champs fichiers selon le type : `IDCardRecto`+`IDCardVerso` (IDCard), `Passport` (Passport), `DriverLicenceRecto`+`DriverLicenceVerso` (DriverLicence — recto-verso, document partagé identité/véhicule).

**Réponse :**
```json
{
  "Success": true,
  "ErrorMessage": "",
  "Documents": [
    { "DocumentID": "uuid", "DocumentURL": "https://...", "DocumentType": "IDCard", "DocumentName": "recto" }
  ]
}
```

#### `POST /api/v1/file/uploadVehicleDocuments` 🔒

```json
{
  "UserID": "uuid",
  "VehicleID": "uuid",
  "DriverLicenceRecto": "<base64>",
  "DriverLicenceVerso": "<base64>",
  "Assurance": "<base64>",
  "VehicleRegistration": "<base64>"
}
```

`DriverLicenceRecto` / `DriverLicenceVerso` : ignorés si l'utilisateur a déjà un permis courant (soumis via `uploadIdDocument` ou un précédent flux véhicule), obligatoires sinon (les deux faces ensemble). Le permis est stocké comme documents utilisateur (`driverLicenceFront` / `driverLicenceBack`, type logique `driverLicence`) et couvre tous les véhicules.

#### `PATCH /api/v1/file/changeDocument` 🔒

```json
{ "UserID": "uuid", "FileID": "uuid", "NewDocument": "<base64>" }
```

#### `GET /api/v1/file/getDocument?FileID=uuid` 🔒

```json
{ "ErrorMessage": "", "File": { "FileID": "uuid", "FileURL": "https://...", "FileType": "IDCard" } }
```

#### `POST /api/v1/file/deleteFile` 🔒

```json
{ "UserID": "uuid", "FileID": "uuid" }
```

---

### KYC Service

Base : `/api/v1/kyc`

#### `POST /api/v1/kyc/inquiries/add` 🔒

Démarre une vérification d'identité via Persona.

```json
{
  "DocumentType": "IDCard",
  "DocumentId": "uuid-du-document-uploadé",
  "DocumentIdBack": "uuid-verso-optionnel",
  "VehicleId": "uuid-si-vérification-véhicule"
}
```

**Réponse :**
```json
{
  "ErrorMessage": "",
  "ReviewId": "uuid",
  "PersonaInquiryId": "inq_xxx",
  "PersonaTemplateId": "tmpl_xxx",
  "SessionToken": "xxx",
  "SessionExpiresAt": "2024-12-01T09:00:00Z",
  "Status": "pending",
  "AttemptNumber": 1
}
```

Utiliser `SessionToken` pour ouvrir le SDK Persona côté client.

#### `GET /api/v1/kyc/me/getStatus` 🔒

```json
{
  "ErrorMessage": "",
  "IdentityVerified": false,
  "DriverVerified": false,
  "PendingReviews": [],
  "LatestRejection": null
}
```

#### `POST /api/v1/kyc/inquiries/resume` 🔒

Reprendre une vérification interrompue.

```json
{ "PersonaInquiryId": "inq_xxx" }
```

---

### Trips Service

Base : `/trip`

#### `GET /api/v1/trip/passenger/getScheduledTripsPreviews` 🌐

Recherche de trajets disponibles.

**Paramètres query :**

| Paramètre | Type | Description |
|---|---|---|
| `PassengerPositionLng` | float | Longitude du passager |
| `PassengerPositionLat` | float | Latitude du passager |
| `DistanceRange` | int | Rayon de recherche (mètres) |
| `DepartureLocationName` | string | Nom du lieu de départ |
| `ArrivalLocationName` | string | Nom de la destination |
| `TripStartDate` | string | Date (`YYYY-MM-DD`) |
| `TripStartHour` | int | Heure de départ (0–23) |
| `TripArrivalHour` | int | Heure d'arrivée max |
| `Index` | int | Pagination (commence à 0) |

**Réponse :**
```json
{
  "TripsPreviews": [
    {
      "TripId": "uuid",
      "DriverId": "uuid",
      "DriverName": "Ydaou Touré",
      "DriverProfileImageURL": "https://...",
      "DriverRatingAverage": 4.7,
      "Status": "scheduled",
      "AvailableSeats": 3,
      "PricePerSeat": 2500,
      "DepartureDatetime": "2024-12-01T08:00:00Z",
      "VehicleBrand": "Toyota",
      "AllowLuggages": true
    }
  ],
  "NextIndex": 10,
  "TotalCount": 42,
  "ErrorMessage": ""
}
```

#### `GET /api/v1/trip/passenger/getTripDetails?TripId=uuid` 🌐

Détail complet d'un trajet pour un passager.

#### `POST /api/v1/trip/driver/createTrip` 🔒

Crée un nouveau trajet.

```json
{
  "DriverId": "uuid",
  "VehicleId": "uuid",
  "DepartureDatetime": "2024-12-01T08:00:00Z",
  "EstimatedArrivalDatetime": "2024-12-01T12:00:00Z",
  "EstimatedDurationMinutes": 240,
  "EstimatedDistanceMeters": 350000,
  "TotalSeats": 4,
  "PricePerSeat": 5000,
  "AllowLuggages": true,
  "AllowPets": false,
  "AllowSmoking": false,
  "AllowFood": true,
  "AutoApprove": false,
  "Description": "Départ ponctuel, musique douce",
  "PaymentMethodsAccepted": ["mobileMoney", "cash"],
  "RoutePolyline": "encoded_polyline_string",
  "TripWaypoints": [
    {
      "SequencerOrder": 0,
      "WaypointType": "departure",
      "LocationName": "Lomé, Gare routière",
      "LocationLng": 1.2317,
      "LocationLat": 6.1375,
      "City": "Lomé",
      "Country": "TG",
      "ScheduledDatetime": "2024-12-01T08:00:00Z",
      "PriceFromPrevious": 0
    },
    {
      "SequencerOrder": 1,
      "WaypointType": "arrival",
      "LocationName": "Accra, Accra Mall",
      "LocationLng": -0.1768,
      "LocationLat": 5.6037,
      "City": "Accra",
      "Country": "GH",
      "ScheduledDatetime": "2024-12-01T12:00:00Z",
      "PriceFromPrevious": 5000
    }
  ]
}
```

**Réponse :** `{ "TripId": "uuid", "ErrorMessage": "" }`

> **Workflow recommandé :** Appeler `/api/v1/geolocation/route` avec les waypoints pour obtenir `EstimatedDistanceMeters`, `EstimatedDurationMinutes` et `RoutePolyline` avant de créer le trajet.

#### Autres endpoints conducteur

| Endpoint | Method | Description |
|---|---|---|
| `GET /api/v1/trip/driver/getTripsPreviews?DriverId=uuid&Index=0` | GET 🔒 | Mes trajets à venir |
| `GET /api/v1/trip/driver/getCompletedTripsPreviews?DriverId=uuid&Index=0` | GET 🔒 | Mes trajets passés |
| `GET /api/v1/trip/driver/getTripDetails?TripId=uuid` | GET 🔒 | Détail complet (avec réservations) |
| `PATCH /api/v1/trip/driver/startTrip` | PATCH 🔒 | `{ "DriverId": "uuid", "TripId": "uuid" }` |
| `PATCH /api/v1/trip/driver/endTrip` | PATCH 🔒 | `{ "DriverId": "uuid", "TripId": "uuid" }` |
| `PATCH /api/v1/trip/driver/confirmWaypointArrival` | PATCH 🔒 | `{ "DriverId": "uuid", "WaypointId": "uuid" }` |
| `PATCH /api/v1/trip/driver/confirmWaypointDeparture` | PATCH 🔒 | `{ "DriverId": "uuid", "WaypointId": "uuid" }` |
| `DELETE /api/v1/trip/driver/cancelTrip` | DELETE 🔒 | `{ "DriverId": "uuid", "TripId": "uuid", "CancellationReason": "..." }` |
| `PATCH /api/v1/trip/driver/changeTripDateAndTime` | PATCH 🔒 | `{ "DriverId": "uuid", "TripId": "uuid", "DepartureDatetime": "..." }` |

---

### Booking Service

Base : `/booking`

#### `POST /api/v1/booking/createBooking` 🔒

```json
{
  "PassengerId": "uuid",
  "TripId": "uuid",
  "PickupWaypointId": "uuid",
  "DropoffWaypointId": "uuid",
  "SeatsBooked": 2,
  "PaymentMethod": "mobileMoney",
  "Segments": [
    {
      "PickupWaypointId": "uuid",
      "DropoffWaypointId": "uuid",
      "PickupLocationName": "Lomé, Gare",
      "PickupCity": "Lomé",
      "PickupLat": 6.1375,
      "PickupLng": 1.2317,
      "PickupScheduledAt": "2024-12-01T08:00:00Z",
      "DropoffLocationName": "Accra, Mall",
      "DropoffCity": "Accra",
      "DropoffLat": 5.6037,
      "DropoffLng": -0.1768,
      "DropoffScheduledAt": "2024-12-01T12:00:00Z",
      "SegmentDistanceMeters": 350000,
      "SegmentDurationMinutes": 240,
      "SegmentPrice": 10000
    }
  ]
}
```

**PaymentMethod :** `mobileMoney` | `card` | `paypal` | `cash`

**Réponse :**
```json
{
  "BookingId": "uuid",
  "BookingReference": "TMH-2024-001234",
  "Status": "pending",
  "TotalAmount": 10000,
  "ErrorMessage": ""
}
```

#### `GET /api/v1/booking/getBookingDetails?BookingId=uuid&UserId=uuid` 🔒

#### `GET /api/v1/booking/getPassengerBookings?PassengerId=uuid&Index=0` 🔒

#### `GET /api/v1/booking/getDriverTripBookings?DriverId=uuid&TripId=uuid&Index=0` 🔒

#### Actions sur une réservation

| Endpoint | Auth | Body |
|---|---|---|
| `PATCH /api/v1/booking/approveBooking` | 🔒 | `{ "DriverId": "uuid", "BookingId": "uuid" }` |
| `PATCH /api/v1/booking/rejectBooking` | 🔒 | `{ "DriverId": "uuid", "BookingId": "uuid", "Reason": "..." }` |
| `PATCH /api/v1/booking/cancelBooking` | 🔒 | `{ "UserId": "uuid", "BookingId": "uuid", "Reason": "..." }` |
| `POST /api/v1/booking/reportNoShow` | 🔒 | `{ "BookingId": "uuid", "ReporterId": "uuid", "NoShowType": "driver\|passenger", "Description": "..." }` |
| `POST /api/v1/booking/confirmPayment` | 🔒 | `{ "BookingId": "uuid", "TransactionId": "xxx" }` |

---

### Payment Service

Base : `/payment`

#### `POST /api/v1/payment/createPayment` 🔒

```json
{
  "BookingId": "uuid",
  "TripId": "uuid",
  "PaymentMethod": "mobileMoney",
  "Amount": 10000,
  "PassengerPhoneNumber": "+22890000000",
  "MobileMoneyMode": "moov_tg"
}
```

**MobileMoneyMode :** `moov_tg` | `togocel`

**Réponse :**
```json
{
  "PaymentId": "uuid",
  "PaymentReference": "PAY-xxx",
  "Status": "pending",
  "ErrorMessage": ""
}
```

#### `GET /api/v1/payment/getPaymentStatus?PaymentId=uuid` 🔒

#### `GET /api/v1/payment/getPaymentByBooking?BookingId=uuid` 🔒

#### `GET /api/v1/payment/getRefundStatus?RefundId=uuid` 🔒

#### `GET /api/v1/payment/getDriverPayouts?DriverId=uuid&PageIndex=0` 🔒

**Réponse :**
```json
{
  "Payouts": [
    {
      "PayoutId": "uuid",
      "PayoutReference": "PAY-OUT-xxx",
      "TripId": "uuid",
      "NetAmount": 45000,
      "Status": "completed",
      "CompletedAt": "2024-12-01T23:30:00Z",
      "CreatedAt": "2024-12-01T22:00:00Z"
    }
  ],
  "ErrorMessage": ""
}
```

---

### Rating Service

Base : `/api/v1/ratings`

#### `POST /api/v1/ratings/rateUser` 🔒

```json
{
  "RaterId": "uuid-du-noteur",
  "UserRatedId": "uuid-du-noté",
  "NumberOfStars": 5,
  "Comment": "Très ponctuel et agréable"
}
```

#### `GET /api/v1/ratings/user/getUserRatings?UserRatedId=uuid` 🔒

#### `GET /api/v1/ratings/user/getUserRatingsAverage?UserRatedId=uuid` 🔒

```json
{ "ErrorMessage": "", "Average": 4.7, "TotalRatings": 42 }
```

#### `PATCH /api/v1/ratings/updateRating` 🔒

```json
{
  "RatingId": "uuid",
  "RaterId": "uuid",
  "UserRatedId": "uuid",
  "NumberOfStars": 4,
  "Comment": "Bon conducteur"
}
```

---

### Geolocation Service

Base : `/api/v1/geolocation`

> Tous les endpoints de géolocalisation sont **publics** (aucun token requis).
> Backend : OSRM (routing) + Nominatim (geocoding), données OSM pour TG, GH, BJ, BF.

#### `POST /api/v1/geolocation/route` 🌐

Calcule la distance, durée et polyline entre waypoints.

```json
{
  "Waypoints": [
    { "Lat": 6.1375, "Lng": 1.2317 },
    { "Lat": 7.5519, "Lng": 2.2251 },
    { "Lat": 5.6037, "Lng": -0.1768 }
  ],
  "Profile": "driving",
  "DepartureTime": "2024-12-01T08:00:00Z"
}
```

**Réponse :**
```json
{
  "DistanceMeters": 580000,
  "DurationSeconds": 25200,
  "PolylineEncoded": "encoded_polyline...",
  "Legs": [
    {
      "DistanceMeters": 200000,
      "DurationSeconds": 9000,
      "MinutesFromDeparture": 0,
      "EstimatedArrivalTime": "2024-12-01T10:30:00Z"
    },
    {
      "DistanceMeters": 380000,
      "DurationSeconds": 16200,
      "MinutesFromDeparture": 150,
      "EstimatedArrivalTime": "2024-12-01T15:00:00Z"
    }
  ],
  "ErrorMessage": ""
}
```

#### `GET /api/v1/geolocation/geocode?Query=Lomé+gare&Limit=5` 🌐

```json
{
  "Results": [
    {
      "DisplayName": "Gare routière de Lomé, Lomé, Togo",
      "Lat": 6.1375,
      "Lng": 1.2317,
      "Country": "TG",
      "City": "Lomé",
      "Type": "bus_station"
    }
  ],
  "ErrorMessage": ""
}
```

#### `GET /api/v1/geolocation/reverse?Lat=6.1375&Lng=1.2317` 🌐

```json
{
  "Result": {
    "DisplayName": "Gare routière de Lomé, Lomé, Togo",
    "Lat": 6.1375,
    "Lng": 1.2317,
    "Country": "TG",
    "City": "Lomé",
    "Type": "bus_station"
  },
  "ErrorMessage": ""
}
```

#### Tuiles cartographiques

Les tuiles vectorielles **ne passent pas par l'api-gateway** — accès direct :

```
https://tiles.tissimah.kpeewu.dev/data/west-africa/{z}/{x}/{y}.pbf
```

Compatible MapLibre GL JS, Mapbox GL JS. Données : Togo, Ghana, Bénin, Burkina Faso.

---

### Chat Service

Base : `/api/v1/chat`

> Le chat est limité aux binômes passager-conducteur d'une réservation acceptée.
> Les messages sont chiffrés AES-256-GCM côté serveur.

#### `POST /api/v1/chat/threads` 🔒

Ouvre (ou récupère) le thread de discussion associé à une réservation.

```json
{ "BookingId": "uuid" }
```

**Réponse :**
```json
{
  "ErrorMessage": "",
  "Thread": {
    "ThreadId": "uuid",
    "BookingId": "uuid",
    "TripId": "uuid",
    "DriverId": "uuid",
    "PassengerId": "uuid",
    "Status": "open",
    "CreatedAt": "2024-12-01T08:00:00Z",
    "ClosedAt": null,
    "UnreadCount": 0,
    "LastMessage": null
  }
}
```

#### `GET /api/v1/chat/threads?PageSize=20&PageToken=` 🔒

Liste tous les threads de l'utilisateur connecté.

#### `POST /api/v1/chat/threads/{thread_id}/messages` 🔒

Envoie un message.

```json
{ "Content": "Bonjour, je serai à la gare à 7h45." }
```

**Réponse :**
```json
{
  "ErrorMessage": "",
  "HasRedaction": false,
  "Message": {
    "MessageId": "uuid",
    "ThreadId": "uuid",
    "SenderId": "uuid",
    "SenderRole": "passenger",
    "Content": "Bonjour, je serai à la gare à 7h45.",
    "HasRedaction": false,
    "Flagged": false,
    "CreatedAt": "2024-12-01T07:30:00Z",
    "ReadAt": null
  }
}
```

`HasRedaction: true` signifie que des informations personnelles (numéro de téléphone, email…) ont été filtrées automatiquement.

#### `GET /api/v1/chat/threads/{thread_id}/messages?PageSize=50&AfterMessageId=` 🔒

Récupère les messages (du plus récent au plus ancien).

```json
{
  "ErrorMessage": "",
  "Messages": [...],
  "HasMore": true,
  "UnreadCount": 3
}
```

#### `PATCH /api/v1/chat/threads/{thread_id}/read` 🔒

Marque les messages comme lus.

```json
{ "LastReadMessageId": "uuid-du-dernier-message-vu" }
```

#### `POST /api/v1/chat/messages/{message_id}/flag` 🔒

Signale un message au support.

```json
{ "Reason": "Contenu inapproprié" }
```

---

### Notification Service

Base : `/api/v1/notifications`

#### `GET /api/v1/notifications/inbox?Page=1&PageSize=20` 🔒

```json
{
  "Entries": [
    {
      "InboxId": "uuid",
      "EventType": "booking.approved",
      "Title": "Réservation acceptée",
      "Body": "Votre réservation pour le trajet Lomé → Accra a été acceptée.",
      "ActionType": "booking",
      "ActionId": "uuid-booking",
      "IsRead": false,
      "CreatedAt": "2024-12-01T08:05:00Z"
    }
  ],
  "TotalCount": 12
}
```

#### `PUT /api/v1/notifications/inbox/{inbox_id}/read` 🔒

Marque une notification comme lue.

#### `PUT /api/v1/notifications/inbox/readAll` 🔒

Marque toutes les notifications comme lues. Réponse : `{ "UpdatedCount": 5 }`

#### `GET /api/v1/notifications/inbox/unreadCount` 🔒

```json
{ "Count": 5 }
```

#### `POST /api/v1/notifications/deviceToken` 🔒

Enregistre un token FCM pour les notifications push.

```json
{
  "FcmToken": "fcm-token-xxx",
  "Platform": "android",
  "DeviceName": "Samsung Galaxy S24"
}
```

#### `DELETE /api/v1/notifications/deviceToken?FcmToken=xxx` 🔒

Désinscrire un appareil (déconnexion).

#### `GET /api/v1/notifications/preferences` 🔒

```json
{ "PushEnabled": true, "EmailEnabled": false }
```

#### `PUT /api/v1/notifications/preferences` 🔒

```json
{ "PushEnabled": true, "EmailEnabled": true }
```

---

### Support Service

Base : `/api/v1/support`

> Réservé au back-office. Ne pas exposer ces routes dans les applications grand public.

#### `POST /api/v1/support/login` 🌐

```json
{ "Email": "agent@tissimah.local", "Password": "mot-de-passe" }
```

**Réponse :** `{ "OtpSessionId": "uuid", "ExpiresInSeconds": 300 }`

#### `POST /api/v1/support/verifyOtp` 🌐

```json
{ "OtpSessionId": "uuid", "Code": "123456" }
```

**Réponse :**
```json
{
  "AccessToken": "jwt...",
  "AccessExpiresAt": "2024-12-01T20:00:00Z",
  "RefreshToken": "uuid",
  "RefreshExpiresAt": "2025-01-01T08:00:00Z",
  "Role": "agent",
  "MustChangePassword": false
}
```

#### `POST /api/v1/support/refreshToken` 🌐

```json
{ "RefreshToken": "uuid" }
```

#### `POST /api/v1/support/logout` 🛡️

```json
{ "RefreshToken": "uuid" }
```

#### `GET /api/v1/support/me` 🛡️

#### `POST /api/v1/support/me/password` 🛡️

```json
{ "CurrentPassword": "ancien", "NewPassword": "nouveau" }
```

---

## 8. Exemples d'intégration

### Recherche et réservation d'un trajet (web)

```javascript
const BASE_URL = 'https://api.tissimah.kpeewu.dev';

// 1. Rechercher des trajets (public — pas de token)
async function searchTrips({ from, to, date }) {
  const params = new URLSearchParams({
    DepartureLocationName: from.name,
    ArrivalLocationName: to.name,
    PassengerPositionLat: from.lat,
    PassengerPositionLng: from.lng,
    DistanceRange: 5000,
    TripStartDate: date,     // YYYY-MM-DD
    TripStartHour: 0,
    TripArrivalHour: 23,
    Index: 0,
  });

  const res = await fetch(`${BASE_URL}/api/v1/trip/passenger/getScheduledTripsPreviews?${params}`);
  return res.json();
}

// 2. Réserver (token Firebase requis)
async function bookTrip(user, tripId, pickupId, dropoffId) {
  const token = await user.getIdToken();

  const res = await fetch(`${BASE_URL}/api/v1/booking/createBooking`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      PassengerId: user.uid,
      TripId: tripId,
      PickupWaypointId: pickupId,
      DropoffWaypointId: dropoffId,
      SeatsBooked: 1,
      PaymentMethod: 'mobileMoney',
      Segments: [/* ... */],
    }),
  });

  return res.json();
}
```

### Création d'un trajet (Flutter / mobile)

```dart
Future<String> createTrip(String driverId, String vehicleId) async {
  final user = FirebaseAuth.instance.currentUser!;
  final token = await user.getIdToken();

  // 1. Obtenir l'itinéraire OSRM
  final routeRes = await http.post(
    Uri.parse('$baseUrl/api/v1/geolocation/route'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode({
      'Waypoints': [
        {'Lat': 6.1375, 'Lng': 1.2317},
        {'Lat': 5.6037, 'Lng': -0.1768},
      ],
      'Profile': 'driving',
    }),
  );
  final route = jsonDecode(routeRes.body);

  // 2. Créer le trajet avec les données de route
  final tripRes = await http.post(
    Uri.parse('$baseUrl/api/v1/trip/driver/createTrip'),
    headers: {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    },
    body: jsonEncode({
      'DriverId': driverId,
      'VehicleId': vehicleId,
      'DepartureDatetime': '2024-12-01T08:00:00Z',
      'EstimatedArrivalDatetime': '2024-12-01T12:00:00Z',
      'EstimatedDurationMinutes': route['DurationSeconds'] ~/ 60,
      'EstimatedDistanceMeters': route['DistanceMeters'],
      'RoutePolyline': route['PolylineEncoded'],
      'TotalSeats': 3,
      'PricePerSeat': 5000,
      'AllowLuggages': true,
      'AllowPets': false,
      'AllowSmoking': false,
      'AllowFood': true,
      'AutoApprove': false,
      'PaymentMethodsAccepted': ['mobileMoney'],
      'TripWaypoints': [/* ... */],
    }),
  );

  final trip = jsonDecode(tripRes.body);
  return trip['TripId'];
}
```

### Envoi de token FCM au démarrage (mobile)

```dart
Future<void> registerFcmToken() async {
  final fcmToken = await FirebaseMessaging.instance.getToken();
  final user = FirebaseAuth.instance.currentUser!;
  final idToken = await user.getIdToken();

  await http.post(
    Uri.parse('$baseUrl/api/v1/notifications/deviceToken'),
    headers: {
      'Authorization': 'Bearer $idToken',
      'Content-Type': 'application/json',
    },
    body: jsonEncode({
      'FcmToken': fcmToken,
      'Platform': Platform.isAndroid ? 'android' : 'ios',
      'DeviceName': await _getDeviceName(),
    }),
  );
}
```

### Intercepteur Axios avec refresh automatique (web)

```javascript
import axios from 'axios';
import { getAuth } from 'firebase/auth';

const api = axios.create({ baseURL: 'https://api.tissimah.kpeewu.dev' });

api.interceptors.request.use(async (config) => {
  const user = getAuth().currentUser;
  if (user) {
    const token = await user.getIdToken();
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (res) => res,
  async (error) => {
    if (error.response?.status === 401) {
      const user = getAuth().currentUser;
      const freshToken = await user?.getIdToken(true);
      if (freshToken) {
        error.config.headers.Authorization = `Bearer ${freshToken}`;
        return api.request(error.config);
      }
    }
    return Promise.reject(error);
  }
);

export default api;
```

---

## Health checks

Chaque service expose un endpoint de santé :

| Service | Endpoint |
|---|---|
| Auth | `GET /api/v1/auth/health` |
| User | `GET /api/v1/user/health` |
| Vehicle | `GET /api/v1/vehicle/health` |
| Trips | `GET /api/v1/trip/health` |
| Booking | `GET /api/v1/booking/health` |
| Payment | `GET /api/v1/payment/health` |
| Rating | `GET /api/v1/ratings/health` |
| KYC | `GET /api/v1/kyc/health` |
| Geolocation | `GET /api/v1/geolocation/health` |
| Chat | `GET /api/v1/chat/health` |
| Notifications | `GET /api/v1/notifications/health` |
| Support | `GET /api/v1/support/health` |
