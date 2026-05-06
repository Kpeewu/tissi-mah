# Notification Service API

Documentation HTTP/REST des endpoints exposés par `notification-service` à travers l'api-gateway. Consommé par les clients mobiles (iOS/Android).

## Base URL

| Environnement | Base URL |
|---------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://api.tissimah.kpeewu.dev` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentification

Tous les endpoints nécessitent un **Firebase JWT** valide :

```http
Authorization: Bearer <firebase_id_token>
```

## En-têtes communs

| En-tête | Requis | Description |
|---------|--------|-------------|
| `Authorization` | Oui | `Bearer <firebase_id_token>` |
| `Content-Type` | Oui (PUT/POST avec body) | `application/json` |

## Format des erreurs

```json
{
    "ErrorMessage": "ErrorUnauthorized"
}
```

> Les champs JSON sont en **PascalCase**.

---

## Endpoints

### GET /api/v1/notifications/inbox

Récupère la liste paginée des notifications de l'utilisateur authentifié.

**Authentification :** Firebase JWT requis

#### Requête

```http
GET /api/v1/notifications/inbox?Page=1&PageSize=20 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
```

#### Paramètres de query

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `Page` | integer | Non | Numéro de page (défaut 1) |
| `PageSize` | integer | Non | Nombre d'entrées par page (défaut 20) |

#### Réponse (succès)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "Entries": [
        {
            "InboxId": "notif-uuid-001",
            "EventType": "BOOKING_APPROVED",
            "Title": "Réservation confirmée",
            "Body": "Votre réservation pour Lomé → Kpalimé a été acceptée.",
            "ActionType": "booking",
            "ActionId": "booking-uuid-001",
            "IsRead": false,
            "CreatedAt": "2026-05-01T10:30:00Z"
        }
    ],
    "TotalCount": 5
}
```

#### Champs de la réponse

| Champ | Type | Description |
|-------|------|-------------|
| `Entries` | array | Liste des notifications |
| `Entries[].InboxId` | string | UUID de l'entrée |
| `Entries[].EventType` | string | Type d'événement (ex: `BOOKING_APPROVED`) |
| `Entries[].Title` | string | Titre de la notification |
| `Entries[].Body` | string | Corps du message |
| `Entries[].ActionType` | string | Type de ressource liée (`booking`, `trip`, `refund`…) |
| `Entries[].ActionId` | string | UUID de la ressource liée |
| `Entries[].IsRead` | bool | `true` si déjà lue |
| `Entries[].CreatedAt` | string | Timestamp ISO 8601 |
| `TotalCount` | integer | Nombre total d'entrées |

---

### PUT /api/v1/notifications/inbox/{InboxId}/read

Marque une notification spécifique comme lue.

**Authentification :** Firebase JWT requis

#### Requête

```http
PUT /api/v1/notifications/inbox/notif-uuid-001/read HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
```

#### Réponse (succès)

```json
{ "Success": true }
```

---

### PUT /api/v1/notifications/inbox/readAll

Marque toutes les notifications non lues de l'utilisateur comme lues.

**Authentification :** Firebase JWT requis

#### Requête

```http
PUT /api/v1/notifications/inbox/readAll HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{}
```

#### Réponse (succès)

```json
{ "UpdatedCount": 3 }
```

---

### GET /api/v1/notifications/inbox/unreadCount

Retourne le nombre de notifications non lues.

**Authentification :** Firebase JWT requis

#### Réponse (succès)

```json
{ "Count": 3 }
```

---

### GET /api/v1/notifications/preferences

Retourne les préférences de notification de l'utilisateur.

**Authentification :** Firebase JWT requis

#### Réponse (succès)

```json
{
    "PushEnabled": true,
    "EmailEnabled": true
}
```

---

### PUT /api/v1/notifications/preferences

Met à jour les préférences de notification.

**Authentification :** Firebase JWT requis

#### Corps de la requête

```json
{
    "PushEnabled": true,
    "EmailEnabled": false
}
```

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `PushEnabled` | bool | Oui | Activer les notifications push |
| `EmailEnabled` | bool | Oui | Activer les notifications email |

#### Réponse (succès)

```json
{ "Success": true }
```

---

### POST /api/v1/notifications/deviceToken

Enregistre un token FCM pour envoyer des push notifications à cet appareil.

**Authentification :** Firebase JWT requis

#### Corps de la requête

```json
{
    "FcmToken": "dGhpcyBpcyBhIHRlc3QgdG9rZW4",
    "Platform": "android",
    "DeviceName": "Pixel 7"
}
```

| Champ | Type | Requis | Description |
|-------|------|--------|-------------|
| `FcmToken` | string | Oui | Token FCM de l'appareil |
| `Platform` | string | Oui | `android` ou `ios` |
| `DeviceName` | string | Non | Nom lisible de l'appareil |

#### Réponse (succès)

```json
{
    "Success": true,
    "TokenId": "token-uuid-001"
}
```

---

### DELETE /api/v1/notifications/deviceToken

Désenregistre un token FCM (ex: déconnexion).

**Authentification :** Firebase JWT requis

#### Paramètres de query

| Paramètre | Type | Requis | Description |
|-----------|------|--------|-------------|
| `FcmToken` | string | Oui | Token FCM à supprimer |

#### Réponse (succès)

```json
{ "Success": true }
```

---

### GET /api/v1/notifications/health

Health check public.

```json
{
    "Status": "ok",
    "Version": "1.0.0",
    "Service": "notification-service"
}
```
