# Support Service API

Documentation HTTP/REST des endpoints exposés par `support-service` à travers l'api-gateway. Consommé par le back-office admin/support (interface web interne).

## Base URL

| Environnement | Base URL |
|---------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentification

**Distinct de Firebase.** Le support-service utilise un JWT HS256 propre (`JWT_SECRET` partagé entre support-service et api-gateway via `SUPPORT_JWT_SECRET`).

Après login+OTP réussi, stocker l'`AccessToken` et l'envoyer dans l'en-tête :

```http
Authorization: Bearer <support_access_token>
```

L'api-gateway injecte `x-support-uid` et `x-support-role` en metadata gRPC pour les routes `SupportProtectedRoutes`.

## Flux d'authentification

```
POST /login → {OtpSessionId}
POST /verifyOtp → {AccessToken, RefreshToken, Role, MustChangePassword}
# Si MustChangePassword == true → POST /me/password obligatoire avant toute action admin
POST /refreshToken → {AccessToken, RefreshToken}  (avant expiration)
POST /logout                                        (invalide le refresh token)
```

## Rôles

| Rôle | Permissions |
|------|-------------|
| `admin` | Toutes les opérations, dont gestion des agents |
| `agent` | Lecture seule + opérations support standard |

## Format des erreurs

```json
{
    "ErrorMessage": "ErrorInvalidCredentials"
}
```

> Les champs JSON sont en **PascalCase**.

---

## Endpoints

### POST /api/v1/support/login

Initie le flux d'authentification. Vérifie email + mot de passe, puis envoie un code OTP par email.

**Authentification :** Aucune

#### Corps de la requête

```json
{
    "Email": "admin@tissimah.local",
    "Password": "Admin1234!"
}
```

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `Email` | string | Oui | Email de l'agent support |
| `Password` | string | Oui | Mot de passe |

#### Réponse (succès)

```json
{
    "OtpSessionId": "sess-uuid-001",
    "ExpiresInSeconds": 300
}
```

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorInvalidCredentials` | 401 | Email ou mot de passe incorrect |
| `ErrorAccountLocked` | 403 | Compte bloqué après trop d'échecs |
| `ErrorAccountInactive` | 403 | Compte désactivé |

---

### POST /api/v1/support/verifyOtp

Valide le code OTP et retourne les tokens d'accès.

**Authentification :** Aucune

#### Corps de la requête

```json
{
    "OtpSessionId": "sess-uuid-001",
    "Code": "123456"
}
```

#### Réponse (succès)

```json
{
    "AccessToken": "<jwt>",
    "AccessExpiresAt": 1746737400,
    "RefreshToken": "<refresh>",
    "RefreshExpiresAt": 1749329400,
    "Role": "admin",
    "MustChangePassword": false
}
```

| Champ | Type | Description |
|-------|------|-------------|
| `AccessToken` | string | JWT HS256, durée configurable (`JWT_TTL_HOURS`, défaut 12h) |
| `AccessExpiresAt` | int64 | Timestamp Unix d'expiration de l'access token |
| `RefreshToken` | string | Token de renouvellement, durée `REFRESH_TOKEN_TTL_HOURS` (défaut 30j) |
| `RefreshExpiresAt` | int64 | Timestamp Unix d'expiration du refresh token |
| `Role` | string | `admin` ou `agent` |
| `MustChangePassword` | bool | `true` si changement de mot de passe obligatoire au 1er login |

#### Erreurs

| ErrorMessage | HTTP | Cause |
|--------------|------|-------|
| `ErrorInvalidOTP` | 400 | Code OTP incorrect |
| `ErrorOTPExpired` | 400 | Session OTP expirée |
| `ErrorOTPMaxAttempts` | 403 | Trop de tentatives OTP |

---

### POST /api/v1/support/resendOtp

Renvoie un nouveau code OTP (soumis à cooldown).

**Authentification :** Aucune

#### Corps de la requête

```json
{ "OtpSessionId": "sess-uuid-001" }
```

#### Réponse (succès)

```json
{
    "OtpSessionId": "sess-uuid-002",
    "ExpiresInSeconds": 300
}
```

---

### POST /api/v1/support/refreshToken

Renouvelle l'access token à partir du refresh token.

**Authentification :** Aucune

#### Corps de la requête

```json
{ "RefreshToken": "<refresh>" }
```

#### Réponse (succès)

```json
{
    "AccessToken": "<new_jwt>",
    "AccessExpiresAt": 1746780600,
    "RefreshToken": "<new_refresh>",
    "RefreshExpiresAt": 1749372600
}
```

---

### POST /api/v1/support/logout

Invalide le refresh token côté serveur (Redis).

**Authentification :** Bearer (support JWT)

#### Corps de la requête

```json
{ "RefreshToken": "<refresh>" }
```

#### Réponse (succès)

```json
{}
```

---

### GET /api/v1/support/me

Retourne les informations de l'agent connecté.

**Authentification :** Bearer (support JWT)

#### Réponse (succès)

```json
{
    "UserId": "agent-uuid-001",
    "Email": "admin@tissimah.local",
    "FirstName": "Admin",
    "LastName": "TissiMah",
    "Role": "admin",
    "MustChangePassword": false,
    "CreatedAt": 1714000000
}
```

---

### POST /api/v1/support/me/password

Change le mot de passe de l'agent connecté. **Obligatoire si `MustChangePassword == true`.**

**Authentification :** Bearer (support JWT)

#### Corps de la requête

```json
{
    "CurrentPassword": "Admin1234!",
    "NewPassword": "NewSecure456!"
}
```

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `CurrentPassword` | string | Oui | Mot de passe actuel |
| `NewPassword` | string | Oui | Nouveau mot de passe (min 8 car., maj + chiffre) |

#### Réponse (succès)

```json
{}
```

---

### POST /api/v1/support/me/email

Change l'adresse email de l'agent connecté.

**Authentification :** Bearer (support JWT)

#### Corps de la requête

```json
{
    "NewEmail": "newadmin@tissimah.local",
    "CurrentPassword": "Admin1234!"
}
```

#### Réponse (succès)

```json
{}
```

---

### POST /api/v1/support/admin/agents

Crée un nouvel agent support. **Rôle `admin` requis.**

**Authentification :** Bearer (support JWT, rôle admin)

#### Corps de la requête

```json
{
    "Email": "agent@tissimah.local",
    "FirstName": "Kouassi",
    "LastName": "Mensah"
}
```

> Un mot de passe provisoire est généré et envoyé par email. `MustChangePassword` est `true` au 1er login.

#### Réponse (succès)

```json
{ "UserId": "agent-uuid-002" }
```

---

### GET /api/v1/support/admin/agents

Liste les agents support. **Rôle `admin` requis.**

**Authentification :** Bearer (support JWT, rôle admin)

#### Paramètres de query

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `Limit` | integer | Non | Nombre max de résultats (défaut 20) |
| `Offset` | integer | Non | Décalage pour pagination (défaut 0) |

#### Réponse (succès)

```json
{
    "Agents": [
        {
            "UserId": "agent-uuid-001",
            "Email": "admin@tissimah.local",
            "FirstName": "Admin",
            "LastName": "TissiMah",
            "Role": "admin",
            "IsActive": true,
            "CreatedAt": 1714000000
        }
    ],
    "Total": 1
}
```

---

### POST /api/v1/support/admin/agents/deactivate

Désactive un agent support (soft-disable). **Rôle `admin` requis.**

**Authentification :** Bearer (support JWT, rôle admin)

#### Corps de la requête

```json
{ "UserId": "agent-uuid-002" }
```

#### Réponse (succès)

```json
{}
```

---

### GET /api/v1/support/health

Health check public.

```json
{
    "Status": "ok",
    "Version": "1.0.0",
    "Service": "support-service"
}
```
