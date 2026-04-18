# User Service API

Documentation HTTP/REST des endpoints exposés par `user-service` à travers l'api-gateway (grpc-gateway). Consommé par les clients mobiles (iOS/Android).

## Base URL

| Environnement | Base URL |
|---------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentification

Les endpoints protégés nécessitent un **Firebase JWT** valide dans l'en-tête `Authorization` :

```http
Authorization: Bearer <firebase_id_token>
```

L'api-gateway valide le token Firebase, puis injecte le `x-firebase-uid` en metadata gRPC. L'intercepteur du user-service lit ce UID et l'injecte dans le contexte via `middleware.FirebaseIDKey`. Le `UserID` interne n'est **jamais** lu depuis le JWT — le service résout lui-même l'association FirebaseUID → UserID via MongoDB.

## En-têtes communs

| En-tête | Requis | Description |
|---------|--------|-------------|
| `Authorization` | Oui (endpoints protégés) | `Bearer <firebase_id_token>` |
| `Content-Type` | Oui (PATCH) | `application/json` |
| `Accept` | Non | `application/json` |

## Format des erreurs

Toutes les erreurs renvoient un code HTTP approprié et un corps JSON au format suivant :

```json
{
    "ErrorMessage": "ErrorUserNotFound"
}
```

> **Note :** les noms de champs JSON sont en PascalCase (proto `UseProtoNames: true`). Les identifiants d'erreur sont stables et peuvent servir de clés de traduction côté frontend.

### Mapping gRPC → HTTP

| Code gRPC | HTTP | Signification |
|-----------|------|---------------|
| `OK` (0) | 200 | Succès |
| `INVALID_ARGUMENT` (3) | 400 | Paramètre invalide (UserID vide, image vide) |
| `NOT_FOUND` (5) | 404 | Ressource inexistante |
| `ALREADY_EXISTS` (6) | 409 | Profil déjà créé |
| `UNAUTHENTICATED` (16) | 401 | Token manquant/invalide ou `x-firebase-uid` absent |
| `UNAVAILABLE` (14) | 503 | Service dépendant indisponible (auth-service ou file-service) |
| `INTERNAL` (13) | 500 | Erreur interne (MongoDB, logique) |

## Rate Limiting

Tous les endpoints user-service utilisent le tier **`global`** (aucun tier spécifique configuré) :

| Tier | Minute | Heure |
|------|--------|-------|
| `global` | 60 | 1000 |

En cas de dépassement : HTTP `429 Too Many Requests` avec headers `X-RateLimit-Limit-*` et `X-RateLimit-Remaining-*`.

## Timeouts

Le user-service n'applique pas de `context.WithTimeout` dans ses handlers. Les timeouts dépendent du client et de la configuration de l'api-gateway.

---

## Endpoints

### GET /api/v1/user/me

Récupère le profil complet de l'utilisateur authentifié. Le `UserID` n'est jamais exposé dans l'URL — il est résolu en interne à partir du Firebase UID extrait du JWT.

Flux interne :
1. Firebase UID (JWT) → lookup MongoDB par `firebase_id`
2. Enrichissement email/téléphone via **auth-service** (`GetAuthInfo`)
3. Enrichissement dates d'expiration des documents via **file-service** (`GetDocumentExpiry`)

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `global` (60/min, 1000/hr)

#### Requête

```http
GET /api/v1/user/me HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
```

**Body :** Aucun. **Query params :** Aucun.

#### Réponse (succès)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "User": {
        "AuthID": "8b1518f9-0949-4872-92a4-5dbdfb7863d9",
        "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "Doe",
        "FirstName": "Samuel",
        "Gender": "male",
        "DateOfBirth": "1995-03-15",
        "Bio": "Passager régulier Lomé-Kara",
        "Email": "samuel@example.com",
        "PhoneNumber": "+22890123456",
        "ProfileImageURL": "https://tissi-mah-files.s3.amazonaws.com/profiles/photo.jpg",
        "HasProfileImage": true,
        "IsDriver": true,
        "IsPassenger": true,
        "IsDriverProfileVerified": false,
        "IsPassengerProfileVerified": true,
        "IsActive": true,
        "IsSuspended": false,
        "SuspensionEndDate": "",
        "TripPreferences": [
            {"Preference": "music", "IsAllowed": true},
            {"Preference": "smoking", "IsAllowed": false},
            {"Preference": "pets", "IsAllowed": true}
        ],
        "IDCardExpirationDate": "2028-06-15",
        "DriveLicenceExpirationDate": "2030-12-01",
        "WithdrawNumber": "+22890123456"
    }
}
```

#### Champs de la réponse

| Champ | Type | Description |
|-------|------|-------------|
| `ErrorMessage` | string | Identifiant d'erreur, `""` si succès |
| `User.AuthID` | string (UUID) | ID auth-service |
| `User.UserID` | string (UUID) | ID profil MongoDB |
| `User.Name` | string | Nom de famille |
| `User.FirstName` | string | Prénom |
| `User.Gender` | string | Genre |
| `User.DateOfBirth` | string | Date de naissance (YYYY-MM-DD) |
| `User.Bio` | string | Biographie |
| `User.Email` | string | Email (depuis auth-service) |
| `User.PhoneNumber` | string | Téléphone E.164 (depuis auth-service) |
| `User.ProfileImageURL` | string | URL photo de profil |
| `User.HasProfileImage` | bool | Présence d'une photo de profil |
| `User.IsDriver` | bool | Compte conducteur activé |
| `User.IsPassenger` | bool | Compte passager activé |
| `User.IsDriverProfileVerified` | bool | Profil conducteur vérifié (KYC) |
| `User.IsPassengerProfileVerified` | bool | Profil passager vérifié (KYC) |
| `User.IsActive` | bool | Compte actif |
| `User.IsSuspended` | bool | Compte suspendu |
| `User.SuspensionEndDate` | string | Fin de suspension (RFC 3339) ou `""` |
| `User.TripPreferences` | array | Liste des préférences de trajet |
| `User.TripPreferences[].Preference` | string | Nom de la préférence |
| `User.TripPreferences[].IsAllowed` | bool | Autorisée ou non |
| `User.IDCardExpirationDate` | string | Date d'expiration de la carte d'identité ou `""` |
| `User.DriveLicenceExpirationDate` | string | Date d'expiration du permis de conduire ou `""` |
| `User.WithdrawNumber` | string | Numéro de retrait (Mobile Money) |

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorUserNotFound` | 404 | Aucun profil associé au Firebase UID |
| `ErrorAuthServiceUnavailable` | 503 | auth-service indisponible pour l'enrichissement |
| `ErrorInternalServer` | 500 | Erreur MongoDB ou Firebase UID absent du contexte |

#### Exemple (cURL)

```bash
curl -X GET https://api.tissi-mah.com/api/v1/user/me \
  -H "Authorization: Bearer <firebase_token>"
```

---

### PATCH /api/v1/userProfile/createDriverAccount

Active ou désactive le statut conducteur sur le profil de l'utilisateur.

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `global` (60/min, 1000/hr)

#### Requête

```http
PATCH /api/v1/userProfile/createDriverAccount HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "CreateDriverAccount": true
}
```

#### Champs de la requête

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `UserID` | string (UUID) | Oui | ID profil MongoDB |
| `CreateDriverAccount` | bool | Oui | `true` → active `IsDriver`, `false` → désactive `IsDriver` |

#### Réponse (succès)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Success": true
}
```

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorInvalidUserID` | 400 | `UserID` vide |
| `ErrorUserNotFound` | 404 | Profil introuvable |
| `ErrorInternalServer` | 500 | Erreur MongoDB |

#### Exemple (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/userProfile/createDriverAccount \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{"UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545", "CreateDriverAccount": true}'
```

---

### PATCH /api/v1/userProfile/addTripPreferences

Définit les préférences de trajet de l'utilisateur. **Remplace intégralement** les préférences existantes (non-additif).

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `global` (60/min, 1000/hr)

#### Requête

```http
PATCH /api/v1/userProfile/addTripPreferences HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "Preferences": [
        {"Preference": "music", "IsAllowed": true},
        {"Preference": "smoking", "IsAllowed": false},
        {"Preference": "pets", "IsAllowed": true},
        {"Preference": "conversation", "IsAllowed": true}
    ]
}
```

#### Champs de la requête

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `UserID` | string (UUID) | Oui | ID profil MongoDB |
| `Preferences` | array | Oui | Liste complète des préférences (remplace l'existant) |
| `Preferences[].Preference` | string | Oui | Nom de la préférence (ex: `"music"`, `"smoking"`, `"pets"`) |
| `Preferences[].IsAllowed` | bool | Oui | `true` = autorisée, `false` = non autorisée |

#### Réponse (succès)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "Success": true
}
```

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorInvalidUserID` | 400 | `UserID` vide |
| `ErrorUserNotFound` | 404 | Profil introuvable |
| `ErrorInternalServer` | 500 | Erreur MongoDB |

#### Exemple (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/userProfile/addTripPreferences \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "Preferences": [
        {"Preference": "music", "IsAllowed": true},
        {"Preference": "smoking", "IsAllowed": false},
        {"Preference": "pets", "IsAllowed": true}
    ]
  }'
```

---

### PATCH /api/v1/userProfile/updateProfile

Met à jour les informations du profil. Tous les champs sauf `UserID` sont optionnels (seuls les champs présents sont modifiés).

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `global` (60/min, 1000/hr)

> ⚠️ **Email et PhoneNumber ne sont pas modifiables ici.** Bien que présents dans le proto, ils sont ignorés dans l'implémentation — email et téléphone sont gérés exclusivement par auth-service.

#### Requête

```http
PATCH /api/v1/userProfile/updateProfile HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "FirstName": "Samuel",
    "LastName": "Doe",
    "BirthDate": "1995-03-15",
    "WithdrawNumber": "+22890123456"
}
```

#### Champs de la requête

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `UserID` | string (UUID) | Oui | ID profil MongoDB |
| `FirstName` | string | Non | Nouveau prénom |
| `LastName` | string | Non | Nouveau nom de famille |
| `BirthDate` | string | Non | Date de naissance (YYYY-MM-DD) |
| `WithdrawNumber` | string | Non | Numéro Mobile Money pour les retraits |
| ~~`Email`~~ | string | — | Ignoré (géré par auth-service) |
| ~~`PhoneNumber`~~ | string | — | Ignoré (géré par auth-service) |

#### Réponse (succès)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "User": {
        "AuthID": "8b1518f9-0949-4872-92a4-5dbdfb7863d9",
        "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "Doe",
        "FirstName": "Samuel",
        "Gender": "male",
        "DateOfBirth": "1995-03-15",
        "Bio": "Passager régulier",
        "Email": "samuel@example.com",
        "PhoneNumber": "+22890123456",
        "ProfileImageURL": "...",
        "HasProfileImage": true,
        "IsDriver": false,
        "IsPassenger": true,
        "IsDriverProfileVerified": false,
        "IsPassengerProfileVerified": true,
        "IsActive": true,
        "IsSuspended": false,
        "SuspensionEndDate": "",
        "TripPreferences": [],
        "IDCardExpirationDate": "",
        "DriveLicenceExpirationDate": "",
        "WithdrawNumber": "+22890123456"
    }
}
```

Le profil retourné est enrichi avec les données auth-service (email, téléphone) et les dates d'expiration des documents (file-service).

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorInvalidUserID` | 400 | `UserID` vide |
| `ErrorUserNotFound` | 404 | Profil introuvable |
| `ErrorAuthServiceUnavailable` | 503 | auth-service indisponible pour l'enrichissement post-mise à jour |
| `ErrorInternalServer` | 500 | Erreur MongoDB |

#### Exemple (cURL)

```bash
curl -X PATCH https://api.tissi-mah.com/api/v1/userProfile/updateProfile \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "FirstName": "Samuel",
    "BirthDate": "1995-03-15"
  }'
```

---

### PATCH /api/v1/userProfile/changeProfilePicture

Change la photo de profil de l'utilisateur. L'image est uploadée via `file-service`, puis l'URL est enregistrée dans MongoDB.

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `global` (60/min, 1000/hr)

> ⚠️ **Le champ `NewProfilePicture` doit être envoyé en base64.** Le grpc-gateway transcrit les champs `bytes` proto en chaînes base64 standard dans le JSON. La taille maximum de l'image dépend de la configuration de l'api-gateway et de file-service.

Flux interne :
1. Validation des champs (`UserID` et `NewProfilePicture` non vides)
2. Lookup du profil MongoDB
3. Upload via **file-service** → obtention de l'URL
4. Mise à jour de `ProfileImageURL` et `HasProfileImage` dans MongoDB
5. Enrichissement via **auth-service** + **file-service** (dates d'expiration)

#### Requête

```http
PATCH /api/v1/userProfile/changeProfilePicture HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
    "NewProfilePicture": "<base64-encoded-image-bytes>"
}
```

#### Champs de la requête

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `UserID` | string (UUID) | Oui | ID profil MongoDB |
| `NewProfilePicture` | string (base64) | Oui | Image encodée en base64 (JPEG ou PNG recommandé) |

#### Réponse (succès)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "User": {
        "AuthID": "8b1518f9-0949-4872-92a4-5dbdfb7863d9",
        "UserID": "8b1d4173-d563-4f81-aeb1-8bf565816545",
        "Name": "Doe",
        "FirstName": "Samuel",
        "ProfileImageURL": "https://tissi-mah-files.s3.amazonaws.com/profiles/8b1d4173.jpg",
        "HasProfileImage": true,
        "..."  : "..."
    }
}
```

Le profil retourné a la même structure que la réponse de `GET /api/v1/user/me`.

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorInvalidUserID` | 400 | `UserID` vide |
| `"new_profile_picture est requis"` | 400 | `NewProfilePicture` vide ou absent |
| `ErrorUserNotFound` | 404 | Profil introuvable |
| `ErrorAuthServiceUnavailable` | 503 | auth-service indisponible pour l'enrichissement post-upload |
| `ErrorInternalServer` | 500 | Échec upload file-service ou erreur MongoDB |

#### Exemple (cURL)

```bash
# Encoder l'image en base64
IMAGE_B64=$(base64 -i photo.jpg)

curl -X PATCH https://api.tissi-mah.com/api/v1/userProfile/changeProfilePicture \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d "{\"UserID\": \"8b1d4173-d563-4f81-aeb1-8bf565816545\", \"NewProfilePicture\": \"$IMAGE_B64\"}"
```

---

### GET /api/v1/user/health

Health check du service.

**Authentification :** Aucune (public)
**Rate limit :** `global` (60/min, 1000/hr)

#### Requête

```http
GET /api/v1/user/health HTTP/1.1
Host: api.tissi-mah.com
```

#### Réponse

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Status": "healthy",
    "Version": "1.0.0",
    "Timestamp": 1713388800
}
```

---

## Catalogue des codes d'erreur

Sentinel errors retournés par l'API. Ces identifiants sont **stables** et peuvent servir de clés de traduction côté frontend.

### Validation (400 INVALID_ARGUMENT)

| Code | Origine | Description |
|------|---------|-------------|
| `ErrorInvalidUserID` | `pkg/errors.ErrorInvalidUserID` | `UserID` vide ou invalide |
| `"new_profile_picture est requis"` | handler direct | `NewProfilePicture` vide (ChangeProfilePicture) |

### Ressources (404 / 409)

| Code | HTTP | Origine | Description |
|------|------|---------|-------------|
| `ErrorUserNotFound` | 404 | `pkg/errors.ErrorUserNotFound` | Profil MongoDB introuvable |
| `ErrorProfileAlreadyExists` | 409 | `pkg/errors.ErrorProfileAlreadyExists` | Profil déjà créé pour cet AuthID (inter-service CreateUser) |

### Services dépendants (503)

| Code | HTTP | Origine | Description |
|------|------|---------|-------------|
| `ErrorAuthServiceUnavailable` | 503 | `pkg/errors.ErrorAuthServiceUnavailable` | auth-service inaccessible (enrichissement email/phone impossible) |

### Internes (500)

| Code | HTTP | Origine | Description |
|------|------|---------|-------------|
| `ErrorDataRetrievalFailed` | 500 | `pkg/errors.ErrorDataRetrievalFailed` | Erreur lecture MongoDB |
| `ErrorInternalServer` | 500 | `pkg/errors.ErrorInternalServer` | Erreur MongoDB, file-service indisponible, Firebase UID absent du contexte |

---

## RPCs inter-services (gRPC uniquement)

Non exposés en HTTP. Appelés directement par les autres services du monorepo via gRPC (port 50052).

### CreateUser

Appelé par **auth-service** après la création d'un compte pour créer le profil MongoDB.

```protobuf
rpc CreateUser(CreateUserRequest) returns (UserProfileResponse);
```

| Champ requête | Type | Description |
|---------------|------|-------------|
| `AuthID` | string (UUID) | ID auth-service |
| `Name` | string | Nom de famille |
| `FirstName` | string | Prénom |
| `ProfilePhotoURL` | string | URL photo (optionnel) |
| `FirebaseID` | string | Firebase UID |

**Erreurs :** `ErrorProfileAlreadyExists` (ALREADY_EXISTS), `ErrorInternalServer` (INTERNAL).

### GetUserByAuthID

Appelé par **auth-service** (login).

```protobuf
rpc GetUserByAuthID(GetUserByAuthIDRequest) returns (UserProfileResponse);
```

| Champ | Type | Description |
|-------|------|-------------|
| `AuthID` | string (UUID) | ID auth-service |

### GetUserByFirebaseID

Appelé par **trips-service**.

```protobuf
rpc GetUserByFirebaseID(GetUserByFirebaseIDRequest) returns (UserProfileResponse);
```

| Champ | Type | Description |
|-------|------|-------------|
| `FirebaseID` | string | Firebase UID |

### GetUserByUserID

Appelé par **trips-service** et **notification-service**. Enrichit la réponse avec email/téléphone depuis auth-service.

```protobuf
rpc GetUserByUserID(GetUserByUserIDRequest) returns (UserProfileResponse);
```

| Champ | Type | Description |
|-------|------|-------------|
| `UserID` | string (UUID) | ID profil MongoDB |

### SoftDeleteUser

Appelé par **auth-service** lors de la suppression de compte. Anonymise et soft-delete le profil MongoDB (GDPR).

```protobuf
rpc SoftDeleteUser(SoftDeleteUserRequest) returns (OperationResponse);
```

| Champ | Type | Description |
|-------|------|-------------|
| `AuthID` | string (UUID) | ID auth-service |

---

## Résumé des endpoints

| Méthode | Path | Auth | Rate limit | Type |
|---------|------|------|------------|------|
| `GET` | `/api/v1/user/me` | JWT | `global` | Lecture (enrichi auth + files) |
| `PATCH` | `/api/v1/userProfile/createDriverAccount` | JWT | `global` | Mise à jour flag |
| `PATCH` | `/api/v1/userProfile/addTripPreferences` | JWT | `global` | Remplacement préférences |
| `PATCH` | `/api/v1/userProfile/updateProfile` | JWT | `global` | Mise à jour partielle |
| `PATCH` | `/api/v1/userProfile/changeProfilePicture` | JWT | `global` | Upload + mise à jour |
| `GET` | `/api/v1/user/health` | — | `global` | Health |
| gRPC | `CreateUser` | — | — | Inter-service |
| gRPC | `GetUserByAuthID` | — | — | Inter-service |
| gRPC | `GetUserByFirebaseID` | — | — | Inter-service |
| gRPC | `GetUserByUserID` | — | — | Inter-service |
| gRPC | `SoftDeleteUser` | — | — | Inter-service |
