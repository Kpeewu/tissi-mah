# Auth Service API

Documentation HTTP/REST des endpoints exposés par `auth-service` à travers l'api-gateway (grpc-gateway). Consommé par les clients mobiles (iOS/Android).

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

Le token est émis par Firebase Authentication sur le client mobile après connexion (email, téléphone, ou providers sociaux). L'api-gateway valide la signature, l'`issuer` et l'`audience` via le Firebase Admin SDK, puis injecte le `x-firebase-uid` en metadata gRPC pour auth-service.

## En-têtes communs

| En-tête | Requis | Description |
|---------|--------|-------------|
| `Authorization` | Oui (endpoints protégés) | `Bearer <firebase_id_token>` |
| `Content-Type` | Oui (POST) | `application/json` |
| `Accept` | Non | `application/json` |

## Format des erreurs

Toutes les erreurs renvoient un code HTTP approprié et un corps JSON au format suivant :

```json
{
    "ErrorMessage": "ErrorEmailNotAvailable"
}
```

> **Note :** les noms de champs JSON sont en PascalCase (proto `UseProtoNames: true`). Les identifiants d'erreur sont stables et peuvent servir de clés de traduction côté frontend.

### Mapping gRPC → HTTP

| Code gRPC | HTTP | Signification |
|-----------|------|---------------|
| `OK` (0) | 200 | Succès |
| `INVALID_ARGUMENT` (3) | 400 | Paramètres invalides (format email/téléphone, champ requis manquant) |
| `NOT_FOUND` (5) | 404 | Ressource inexistante |
| `ALREADY_EXISTS` (6) | 409 | Conflit (email/téléphone déjà utilisé) |
| `PERMISSION_DENIED` (7) | 403 | Accès refusé |
| `FAILED_PRECONDITION` (9) | 412 | Pré-condition non remplie |
| `DEADLINE_EXCEEDED` (4) | 504 | Timeout du handler (voir section Timeouts) |
| `CANCELED` (1) | 499 | Requête annulée par le client |
| `UNAUTHENTICATED` (16) | 401 | Token manquant/invalide ou `x-firebase-uid` absent |
| `INTERNAL` (13) | 500 | Erreur interne (DB, service indisponible) |

## Timeouts

Chaque handler gRPC applique un timeout via `context.WithTimeout` :

| Type d'opération | Timeout | Handlers concernés |
|------------------|---------|--------------------|
| Lecture simple | 5s | `CheckEmail`, `CheckPhoneNumber`, `GetAuthInfo` |
| Opération cross-service | 10s | `CreateAccount`, `DeleteAccount` |
| Health check | — (aucun) | `Health` |

Un dépassement renvoie `ErrorMessage: "request timeout"` avec code HTTP 504.

## Rate Limiting

Le rate limiting distribué (Redis) est appliqué par IP sur chaque route via l'api-gateway. Valeurs par défaut :

| Tier | Seconde | Minute | Heure | Routes concernées |
|------|---------|--------|-------|-------------------|
| `global` | — | 60 | 1000 | Toutes les routes sans tier spécifique (ex: `health`) |
| `auth` | 1 | 10 | 100 | `checkEmail`, `checkPhoneNumber` |
| `create` | — | 3 | 10 | `createAccount` |
| `sensitive` | — | 1 | 5 | `deleteAccount` |

En cas de dépassement : HTTP `429 Too Many Requests` avec headers `X-RateLimit-Limit-*` et `X-RateLimit-Remaining-*`.

---

## Endpoints

### POST /api/v1/auth/createAccount

Crée un nouveau compte pour l'utilisateur Firebase authentifié et son profil associé dans `user-service`. Opération cross-service avec compensation automatique : si la création du profil échoue, le record auth est supprimé pour éviter un état orphelin.

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `create` (3/min, 10/heure)
**Timeout :** 10s

#### Requête

```http
POST /api/v1/auth/createAccount HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
    "Name": "Doe",
    "FirstName": "Samuel",
    "Email": "samuel@example.com",
    "PhoneNumber": "+22890123456",
    "ProfileImageURL": "https://example.com/photo.jpg"
}
```

#### Champs de la requête

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `Name` | string | Oui | Nom de famille |
| `FirstName` | string | Oui | Prénom |
| `Email` | string | Conditionnel | Adresse email (requis si `PhoneNumber` absent) |
| `PhoneNumber` | string | Conditionnel | Numéro E.164 (ex: `+22890123456`) (requis si `Email` absent) |
| `ProfileImageURL` | string | Non | URL de la photo de profil |

> Au moins un des champs `Email` ou `PhoneNumber` doit être fourni (contrainte DB).

#### Validation

- **Email** : format RFC-compatible via regex `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`, longueur maximum 254 caractères, normalisation via `strings.ToLower + TrimSpace`.
- **Téléphone** : format **E.164 universel** `^\+[1-9]\d{6,14}$` (entre 7 et 15 chiffres après le code pays, pas de zéro en tête).

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
        "Email": "samuel@example.com",
        "PhoneNumber": "+22890123456",
        "ProfileImageURL": ""
    }
}
```

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorEmailInvalidFormat` | 400 | Format d'email invalide |
| `ErrorEmailTooLong` | 400 | Email > 254 caractères |
| `ErrorPhoneInvalidFormat` | 400 | Format E.164 invalide |
| `ErrorEmailNotAvailable` | 409 | Email déjà utilisé |
| `ErrorPhoneNumberNotAvailable` | 409 | Téléphone déjà utilisé |
| `request timeout` | 504 | Dépassement de 10s (user-service indisponible) |
| `ErrorInternalServer` | 500 | Erreur DB ou échec user-service (rollback auto) |

#### Exemple (cURL)

```bash
curl -X POST https://api.tissi-mah.com/api/v1/auth/createAccount \
  -H "Authorization: Bearer <firebase_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "Name": "Doe",
    "FirstName": "Samuel",
    "Email": "samuel@example.com",
    "PhoneNumber": "+22890123456"
  }'
```

---

### GET /api/v1/auth/checkEmail

Vérifie si une adresse email est disponible pour l'inscription.

**Authentification :** Aucune (public)
**Rate limit :** `auth` (1/s, 10/min, 100/heure)
**Timeout :** 5s

#### Requête

```http
GET /api/v1/auth/checkEmail?Email=samuel%40example.com HTTP/1.1
Host: api.tissi-mah.com
```

#### Paramètres de query

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `Email` | string | Oui | Email à vérifier (URL-encodé) |

#### Réponse (disponible)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "IsAvailable": true
}
```

#### Réponse (non disponible)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "IsAvailable": false
}
```

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorEmailInvalidFormat` | 400 | Format d'email invalide |
| `ErrorEmailTooLong` | 400 | Email > 254 caractères |
| `request timeout` | 504 | Dépassement de 5s |
| `ErrorInternalServer` | 500 | Erreur DB |

#### Exemple (cURL)

```bash
curl -G "https://api.tissi-mah.com/api/v1/auth/checkEmail" \
  --data-urlencode "Email=samuel@example.com"
```

---

### GET /api/v1/auth/checkPhoneNumber

Vérifie si un numéro de téléphone est disponible pour l'inscription.

**Authentification :** Aucune (public)
**Rate limit :** `auth` (1/s, 10/min, 100/heure)
**Timeout :** 5s

#### Requête

```http
GET /api/v1/auth/checkPhoneNumber?PhoneNumber=%2B22890123456 HTTP/1.1
Host: api.tissi-mah.com
```

#### Paramètres de query

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `PhoneNumber` | string | Oui | Numéro E.164 (URL-encodé, `+` devient `%2B`) |

#### Réponse (disponible)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "IsAvailable": true
}
```

#### Réponse (non disponible)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "ErrorMessage": "",
    "IsAvailable": false
}
```

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorPhoneInvalidFormat` | 400 | Format E.164 invalide |
| `request timeout` | 504 | Dépassement de 5s |
| `ErrorInternalServer` | 500 | Erreur DB |

#### Exemple (cURL)

```bash
curl -G "https://api.tissi-mah.com/api/v1/auth/checkPhoneNumber" \
  --data-urlencode "PhoneNumber=+22890123456"
```

---

### DELETE /api/v1/auth/deleteAccount

Supprime définitivement le compte de l'utilisateur authentifié et anonymise ses données personnelles (GDPR).

**Authentification :** Requise (Firebase JWT)
**Rate limit :** `sensitive` (1/min, 5/heure)
**Timeout :** 10s

**⚠️ Opération irréversible.** Exécute en séquence :
1. Soft-delete + anonymisation du profil dans `user-service`
2. Soft-delete + anonymisation du record auth (avec retry automatique en cas d'échec)

Si la seconde étape échoue définitivement après retry, un log `CRITICAL` est émis (cleanup manuel requis) et l'erreur `ErrorCantDeleteAccount` est renvoyée.

#### Requête

```http
DELETE /api/v1/auth/deleteAccount HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
```

**Body :** Aucun (Firebase UID extrait du JWT — pas d'ID utilisateur dans l'URL pour des raisons de sécurité).

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
| `ErrorUserNotFound` | 404 | Aucun compte associé au Firebase UID |
| `ErrorCantDeleteAccount` | 412 | Échec suppression auth après retry (cleanup manuel requis côté serveur) |
| `request timeout` | 504 | Dépassement de 10s |
| `ErrorInternalServer` | 500 | Échec user-service (profil non supprimé, auth intact) |

#### Exemple (cURL)

```bash
curl -X DELETE https://api.tissi-mah.com/api/v1/auth/deleteAccount \
  -H "Authorization: Bearer <firebase_token>"
```

---

### GET /api/v1/auth/health

Health check du service.

**Authentification :** Aucune (public)
**Rate limit :** `global` (60/min, 1000/heure)
**Timeout :** — (aucun)

#### Requête

```http
GET /api/v1/auth/health HTTP/1.1
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
| `ErrorEmailInvalidFormat` | `domain.ErrEmailInvalidFormat` | Format email invalide (regex) |
| `ErrorEmailTooLong` | `domain.ErrEmailTooLong` | Email > 254 caractères |
| `ErrorPhoneInvalidFormat` | `domain.ErrPhoneInvalidFormat` | Format E.164 invalide |

### Ressources (404 / 409)

| Code | HTTP | Origine | Description |
|------|------|---------|-------------|
| `ErrorUserNotFound` | 404 | `pkg/errors.ErrorUserNotFound` | Utilisateur introuvable |
| `ErrorEmailNotAvailable` | 409 | `pkg/errors.ErrorEmailNotAvailable` | Email déjà utilisé |
| `ErrorPhoneNumberNotAvailable` | 409 | `pkg/errors.ErrorPhoneNumberNotAvailable` | Téléphone déjà utilisé |

### Pré-conditions et internes (412 / 500)

| Code | HTTP | Origine | Description |
|------|------|---------|-------------|
| `ErrorCantDeleteAccount` | 412 | `pkg/errors.ErrorCantDeleteAccount` | Suppression auth impossible après retry |
| `ErrorDataRetrievalFailed` | 500 | `pkg/errors.ErrorDataRetrievalFailed` | Erreur lors de la récupération des données |
| `ErrorInternalServer` | 500 | `pkg/errors.ErrorInternalServer` | Erreur interne (DB, service indisponible, état incohérent) |

### Timeouts et annulations (504 / 499)

| Code | HTTP | Origine | Description |
|------|------|---------|-------------|
| `request timeout` | 504 | `context.DeadlineExceeded` | Handler dépassé |
| `request canceled` | 499 | `context.Canceled` | Client a annulé la requête |

---

## RPCs inter-services (gRPC uniquement)

Non exposés en HTTP. Appelés par les autres services du monorepo via gRPC (port 50051).

### GetAuthInfo

Récupère les informations d'authentification d'un utilisateur par son `AuthID`.

```protobuf
rpc GetAuthInfo(GetAuthInfoRequest) returns (GetAuthInfoResponse);
```

**Requête :**

| Champ | Type | Description |
|-------|------|-------------|
| `AuthID` | string (UUID) | Identifiant auth-service |

**Réponse :**

| Champ | Type | Description |
|-------|------|-------------|
| `AuthID` | string | ID auth-service |
| `Email` | string | Email (vide si non défini) |
| `PhoneNumber` | string | Téléphone E.164 (vide si non défini) |
| `IsActive` | bool | Compte actif |
| `IsSuspended` | bool | Compte suspendu |
| `SuspensionEndDate` | string | Date de fin de suspension (RFC 3339) ou vide |

**Erreurs** : `ErrorUserNotFound` (NOT_FOUND), `ErrorInternalServer` (INTERNAL), timeout 5s.

---

## Résumé des endpoints

| Méthode | Path | Auth | Tier rate-limit | Timeout | Type |
|---------|------|------|-----------------|---------|------|
| `POST` | `/api/v1/auth/createAccount` | JWT | `create` | 10s | Création |
| `GET` | `/api/v1/auth/checkEmail` | — | `auth` | 5s | Lecture |
| `GET` | `/api/v1/auth/checkPhoneNumber` | — | `auth` | 5s | Lecture |
| `DELETE` | `/api/v1/auth/deleteAccount` | JWT | `sensitive` | 10s | Suppression |
| `GET` | `/api/v1/auth/health` | — | `global` | — | Health |
| gRPC | `GetAuthInfo` | — | — | 5s | Inter-service |
