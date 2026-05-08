# TissiMah — Intégration API Gateway

Guide technique pour communiquer avec l'api-gateway depuis une application web ou mobile.

---

## Table des matières

1. [URLs de base](#1-urls-de-base)
2. [Headers obligatoires](#2-headers-obligatoires)
3. [Authentification](#3-authentification)
4. [Format des erreurs](#4-format-des-erreurs)
5. [Rate limiting](#5-rate-limiting)
6. [CORS (Web uniquement)](#6-cors-web-uniquement)
7. [Client HTTP recommandé — Web (TypeScript)](#7-client-http-recommandé--web-typescript)
8. [Client HTTP recommandé — Mobile (Flutter/Dart)](#8-client-http-recommandé--mobile-flutterdart)
9. [Debugging](#9-debugging)

---

## 1. URLs de base

| Environnement | URL |
|---|---|
| **VPS-Dev** | `https://api.tissimah.kpeewu.dev` |
| **Local** | `http://localhost:8080` |

**Limite de body :** 10 Mo par requête (`Content-Length` maximal : 10 485 760 bytes).

---

## 2. Headers obligatoires

Chaque requête vers une route protégée doit inclure **les trois headers suivants**.

| Header | Valeur | Obligatoire sur |
|---|---|---|
| `Authorization` | `Bearer <token>` | Toutes les routes protégées |
| `X-App-ID` | UUID fourni par l'équipe backend | Toutes les routes protégées |
| `Content-Type` | `application/json` | Requêtes avec body (POST, PATCH, PUT) |

> **Routes publiques** (health checks, recherche de trajets, géolocalisation, vérification email/téléphone) : aucun header d'authentification requis.

### X-App-ID

C'est un UUID statique bundlé dans le build de l'application. Il permet à la gateway de distinguer le trafic légitime des bots.

- **App mobile (passager/conducteur)** → whitelist `MOBILE_APP_IDS`
- **Back-office support** → whitelist `SUPPORT_APP_IDS`

En l'absence ou en cas de valeur invalide : **HTTP 401** `{"message": "missing X-App-ID header"}`.

La valeur est une chaîne UUID fixe, **à ne pas confondre avec le Firebase UID** qui change par utilisateur.

### X-Request-ID (optionnel mais recommandé)

UUID de corrélation pour le debugging. Si absent, la gateway en génère un automatiquement. La valeur est toujours retournée dans le header de réponse `X-Request-ID`.

```
X-Request-ID: 550e8400-e29b-41d4-a716-446655440000
```

---

## 3. Authentification

### 3.1 Utilisateurs — Firebase JWT

Les passagers et conducteurs s'authentifient avec un **Firebase ID Token** obtenu via le SDK Firebase.

**Format du header :**
```
Authorization: Bearer <firebase_id_token>
```

**Durée de vie :** 1 heure. Rafraîchir proactivement avant expiration pour éviter un 401 en cours d'usage.

**Ce que la gateway fait avec le token :**
1. Vérifie la signature RS256 via Firebase Admin SDK (issuer + audience + expiry)
2. Extrait le Firebase UID
3. Propage le UID aux microservices via le header interne `x-firebase-uid`

Le UID Firebase est disponible dans les services sans que l'application cliente ait à le transmettre explicitement dans le body des requêtes — la gateway le fait automatiquement.

---

### 3.2 Back-office — Support JWT

Les agents support utilisent un JWT HS256 émis par le support-service après vérification OTP.

**Flux complet :**

```
POST /api/v1/support/login
  → { Email, Password }
  ← { OtpSessionId, ExpiresInSeconds }

POST /api/v1/support/verifyOtp
  → { OtpSessionId, Code }
  ← { AccessToken, AccessExpiresAt, RefreshToken, RefreshExpiresAt, Role, MustChangePassword }

// Utiliser AccessToken dans Authorization pour toutes les routes /api/v1/support/*

POST /api/v1/support/refreshToken   (quand AccessToken arrive à expiration)
  → { RefreshToken }
  ← { AccessToken, AccessExpiresAt, RefreshToken, RefreshExpiresAt }
```

**Durée de vie par défaut :**
- Access token : 12 heures
- Refresh token : 30 jours

---

## 4. Format des erreurs

La gateway produit deux formats d'erreur selon l'origine.

### Erreurs de middleware (401, 429, 503)

Produites par la gateway elle-même avant d'atteindre le service :

```json
{ "message": "invalid or expired token" }
```

Cas déclencheurs :
- Token Firebase absent ou invalide → 401 `missing authorization header` / `invalid or expired token`
- X-App-ID absent ou invalide → 401 `missing X-App-ID header` / `invalid X-App-ID`
- Rate limit dépassé → 429 `API rate limit exceeded`
- Redis indisponible (prod, fail-closed) → 503 `service temporarily unavailable`
- Body > 10 Mo → 413 (pas de body JSON)

### Erreurs métier (4xx, 5xx via gRPC)

Produites par les microservices, transcrites par grpc-gateway :

```json
{ "ErrorMessage": "code_erreur_metier" }
```

### Tableau de correspondance HTTP → gRPC

| HTTP | gRPC | Cas typique |
|---|---|---|
| 200 | OK | Succès |
| 400 | INVALID_ARGUMENT | Champ manquant ou invalide |
| 401 | UNAUTHENTICATED | Token invalide (middleware) |
| 403 | PERMISSION_DENIED | Action non autorisée |
| 404 | NOT_FOUND | Ressource introuvable |
| 409 | ALREADY_EXISTS | Email/téléphone déjà utilisé |
| 412 | FAILED_PRECONDITION | Précondition non remplie |
| 413 | — | Body trop grand |
| 429 | — | Rate limit dépassé |
| 500 | INTERNAL | Erreur serveur |
| 503 | UNAVAILABLE | Service indisponible |

**Stratégie de gestion recommandée :**

```
401 → rafraîchir le token et réessayer une fois
429 → lire Retry-After, attendre, puis exponential backoff
5xx → retry avec backoff (max 3 tentatives)
4xx (autres) → erreur définitive, afficher à l'utilisateur
```

Le header `Retry-After` est présent sur les réponses 429 et 503 (valeur en secondes).

---

## 5. Rate limiting

Sliding window Redis par Firebase UID (authentifié) ou par IP (public).

| Tier | Routes | /seconde | /minute | /heure |
|---|---|---|---|---|
| **global** | Toutes les autres | — | 60 | 1 000 |
| **auth** | `checkEmail`, `checkPhoneNumber` | 1 | 10 | 100 |
| **create** | `createAccount`, `POST .../messages` (chat) | — | 3 | 10 |
| **sensitive** | `deleteAccount`, `POST .../flag` (chat) | — | 1 | 5 |

**Headers retournés sur chaque réponse (fenêtre minute et heure) :**

```
X-RateLimit-Limit-Minute: 60
X-RateLimit-Remaining-Minute: 58
X-RateLimit-Limit-Hour: 1000
X-RateLimit-Remaining-Hour: 997
```

Exposés dans CORS (`Access-Control-Expose-Headers`) — lisibles depuis le navigateur.

---

## 6. CORS (Web uniquement)

| Environnement | Origines autorisées |
|---|---|
| Production | `https://tissi-mah.com`, `https://www.tissi-mah.com` |
| VPS-Dev | `*` |

**Méthodes autorisées :** `GET POST PUT PATCH DELETE OPTIONS HEAD`

**Headers autorisés en requête :**
```
Accept, Accept-Language, Accept-Encoding, Authorization, Content-Type,
Content-Length, Origin, X-Request-ID, X-Requested-With, X-Device-ID,
X-App-Version, X-Platform, X-Timezone, Cache-Control, Pragma
```

**Headers exposés en réponse :**
```
X-Request-ID, X-RateLimit-Limit-Minute, X-RateLimit-Remaining-Minute,
X-RateLimit-Limit-Hour, X-RateLimit-Remaining-Hour, Content-Disposition
```

**Preflight :** mis en cache 3 600 secondes (`Access-Control-Max-Age: 3600`).

> `Access-Control-Allow-Credentials: true` est toujours positionné — ne pas utiliser `credentials: 'omit'` côté fetch.

---

## 7. Client HTTP recommandé — Web (TypeScript)

### Installation des dépendances Firebase

```bash
npm install firebase
```

### `src/lib/api/client.ts`

```typescript
import { getAuth } from 'firebase/auth';

const BASE_URL = import.meta.env.VITE_API_URL ?? 'https://api.tissimah.kpeewu.dev';
const MOBILE_APP_ID = import.meta.env.VITE_APP_ID; // UUID fourni par le backend

// ─── Types ───────────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public readonly message: string,
    public readonly status: number,
    public readonly isGatewayError: boolean, // true = middleware, false = métier
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

async function getFirebaseToken(forceRefresh = false): Promise<string | null> {
  const user = getAuth().currentUser;
  if (!user) return null;
  return user.getIdToken(forceRefresh);
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// ─── Requête de base ──────────────────────────────────────────────────────────

async function request<T>(
  path: string,
  options: RequestInit & { skipAuth?: boolean } = {},
  retryCount = 0,
): Promise<T> {
  const { skipAuth = false, ...fetchOptions } = options;
  const headers = new Headers(fetchOptions.headers);

  // Content-Type par défaut
  if (!headers.has('Content-Type') && fetchOptions.body) {
    headers.set('Content-Type', 'application/json');
  }

  // X-App-ID (obligatoire sur routes protégées)
  headers.set('X-App-ID', MOBILE_APP_ID);

  // Authorization (Firebase JWT)
  if (!skipAuth) {
    const token = await getFirebaseToken();
    if (token) {
      headers.set('Authorization', `Bearer ${token}`);
    }
  }

  const res = await fetch(`${BASE_URL}${path}`, { ...fetchOptions, headers });

  // ── 401 : token expiré → refresh + retry (une seule fois) ────────────────
  if (res.status === 401 && retryCount === 0 && !skipAuth) {
    const freshToken = await getFirebaseToken(true);
    if (freshToken) {
      headers.set('Authorization', `Bearer ${freshToken}`);
      return request<T>(path, { ...options, headers }, 1);
    }
  }

  // ── 429 : rate limit → attendre Retry-After puis retry ───────────────────
  if (res.status === 429 && retryCount < 3) {
    const retryAfter = Number(res.headers.get('Retry-After') ?? 60);
    const backoff = retryAfter * 1000 * Math.pow(2, retryCount);
    await sleep(backoff);
    return request<T>(path, options, retryCount + 1);
  }

  // ── 5xx → retry avec backoff exponentiel (max 3 fois) ────────────────────
  if (res.status >= 500 && retryCount < 3) {
    await sleep(1000 * Math.pow(2, retryCount));
    return request<T>(path, options, retryCount + 1);
  }

  // ── Pas de body (413, 204…) ───────────────────────────────────────────────
  const contentType = res.headers.get('Content-Type') ?? '';
  if (!contentType.includes('application/json')) {
    if (!res.ok) throw new ApiError(`HTTP ${res.status}`, res.status, true);
    return undefined as T;
  }

  const data = await res.json();

  // ── Erreur middleware → champ "message" ───────────────────────────────────
  if (!res.ok && data.message) {
    throw new ApiError(data.message, res.status, true);
  }

  // ── Erreur métier → champ "ErrorMessage" ─────────────────────────────────
  if (data.ErrorMessage) {
    throw new ApiError(data.ErrorMessage, res.status, false);
  }

  if (!res.ok) {
    throw new ApiError(`HTTP ${res.status}`, res.status, true);
  }

  return data as T;
}

// ─── Méthodes publiques ───────────────────────────────────────────────────────

export const api = {
  get: <T>(path: string, options?: RequestInit & { skipAuth?: boolean }) =>
    request<T>(path, { ...options, method: 'GET' }),

  post: <T>(path: string, body: unknown, options?: RequestInit & { skipAuth?: boolean }) =>
    request<T>(path, {
      ...options,
      method: 'POST',
      body: JSON.stringify(body),
    }),

  patch: <T>(path: string, body: unknown, options?: RequestInit & { skipAuth?: boolean }) =>
    request<T>(path, {
      ...options,
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  put: <T>(path: string, body: unknown, options?: RequestInit & { skipAuth?: boolean }) =>
    request<T>(path, {
      ...options,
      method: 'PUT',
      body: JSON.stringify(body),
    }),

  delete: <T>(path: string, options?: RequestInit & { skipAuth?: boolean }) =>
    request<T>(path, { ...options, method: 'DELETE' }),
};
```

### Exemples d'utilisation

```typescript
import { api, ApiError } from '@/lib/api/client';

// Route publique (pas de token nécessaire)
const trips = await api.get('/trip/passenger/getScheduledTripsPreviews?...', {
  skipAuth: true,
});

// Route protégée Firebase
const me = await api.get<{ User: { UserID: string } }>('/api/v1/user/me');

// Gestion d'erreur
try {
  const booking = await api.post('/booking/createBooking', {
    PassengerId: userId,
    TripId: tripId,
    // ...
  });
} catch (err) {
  if (err instanceof ApiError) {
    if (err.status === 409) {
      // Réservation déjà existante
    } else if (err.status === 412) {
      // Précondition non remplie (ex: siège non disponible)
    }
    console.error(err.message); // code métier ou message middleware
  }
}
```

### `src/lib/api/support-client.ts` (back-office uniquement)

```typescript
const SUPPORT_APP_ID = import.meta.env.VITE_SUPPORT_APP_ID;
let accessToken: string | null = null;
let refreshToken: string | null = null;

export async function supportLogin(email: string, password: string) {
  const res = await fetch(`${BASE_URL}/api/v1/support/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-App-ID': SUPPORT_APP_ID,
    },
    body: JSON.stringify({ Email: email, Password: password }),
  });
  return res.json(); // { OtpSessionId, ExpiresInSeconds }
}

export async function supportVerifyOtp(otpSessionId: string, code: string) {
  const res = await fetch(`${BASE_URL}/api/v1/support/verifyOtp`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-App-ID': SUPPORT_APP_ID },
    body: JSON.stringify({ OtpSessionId: otpSessionId, Code: code }),
  });
  const data = await res.json();
  accessToken = data.AccessToken;
  refreshToken = data.RefreshToken;
  return data;
}

async function supportRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set('Authorization', `Bearer ${accessToken}`);
  headers.set('X-App-ID', SUPPORT_APP_ID);
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }

  let res = await fetch(`${BASE_URL}${path}`, { ...options, headers });

  // Refresh automatique si token expiré
  if (res.status === 401 && refreshToken) {
    const refreshRes = await fetch(`${BASE_URL}/api/v1/support/refreshToken`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-App-ID': SUPPORT_APP_ID },
      body: JSON.stringify({ RefreshToken: refreshToken }),
    });
    const refreshData = await refreshRes.json();
    accessToken = refreshData.AccessToken;
    refreshToken = refreshData.RefreshToken;
    headers.set('Authorization', `Bearer ${accessToken}`);
    res = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  }

  const data = await res.json();
  if (!res.ok) throw new Error(data.ErrorMessage ?? data.message);
  return data;
}

export const supportApi = {
  get: <T>(path: string) => supportRequest<T>(path, { method: 'GET' }),
  post: <T>(path: string, body: unknown) =>
    supportRequest<T>(path, { method: 'POST', body: JSON.stringify(body) }),
};
```

---

## 8. Client HTTP recommandé — Mobile (Flutter/Dart)

### `pubspec.yaml`

```yaml
dependencies:
  firebase_auth: ^4.0.0
  http: ^1.2.0
  # ou dio: ^5.0.0 si tu préfères les intercepteurs
```

### `lib/core/api/api_client.dart`

```dart
import 'dart:convert';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:http/http.dart' as http;

// ─── Config ──────────────────────────────────────────────────────────────────

const String _baseUrl = String.fromEnvironment(
  'API_URL',
  defaultValue: 'https://api.tissimah.kpeewu.dev',
);

// UUID fourni par le backend, bundlé dans le build via --dart-define
const String _mobileAppId = String.fromEnvironment('MOBILE_APP_ID');

// ─── Erreur API ───────────────────────────────────────────────────────────────

class ApiException implements Exception {
  const ApiException(this.message, this.statusCode, {this.isGatewayError = false});

  final String message;
  final int statusCode;
  final bool isGatewayError;

  @override
  String toString() => 'ApiException($statusCode): $message';
}

// ─── Client ───────────────────────────────────────────────────────────────────

class ApiClient {
  ApiClient._();
  static final ApiClient instance = ApiClient._();

  final _http = http.Client();

  // ── Helpers ─────────────────────────────────────────────────────────────────

  Future<String?> _getToken({bool forceRefresh = false}) async {
    final user = FirebaseAuth.instance.currentUser;
    if (user == null) return null;
    return user.getIdToken(forceRefresh);
  }

  Map<String, String> _baseHeaders({String? token}) => {
    'X-App-ID': _mobileAppId,
    if (token != null) 'Authorization': 'Bearer $token',
  };

  Future<T> _request<T>(
    String method,
    String path, {
    Object? body,
    bool skipAuth = false,
    int retryCount = 0,
  }) async {
    final token = skipAuth ? null : await _getToken();

    final headers = {
      ..._baseHeaders(token: token),
      if (body != null) 'Content-Type': 'application/json',
    };

    final uri = Uri.parse('$_baseUrl$path');
    final encodedBody = body != null ? jsonEncode(body) : null;

    http.Response res;
    switch (method) {
      case 'GET':
        res = await _http.get(uri, headers: headers);
      case 'POST':
        res = await _http.post(uri, headers: headers, body: encodedBody);
      case 'PATCH':
        res = await _http.patch(uri, headers: headers, body: encodedBody);
      case 'PUT':
        res = await _http.put(uri, headers: headers, body: encodedBody);
      case 'DELETE':
        res = await _http.delete(uri, headers: headers);
      default:
        throw ArgumentError('Unknown method: $method');
    }

    // ── 401 → refresh token + retry (une seule fois) ─────────────────────────
    if (res.statusCode == 401 && retryCount == 0 && !skipAuth) {
      final freshToken = await _getToken(forceRefresh: true);
      if (freshToken != null) {
        return _request<T>(method, path,
            body: body, skipAuth: skipAuth, retryCount: 1);
      }
    }

    // ── 429 → respecter Retry-After + backoff ────────────────────────────────
    if (res.statusCode == 429 && retryCount < 3) {
      final retryAfter = int.tryParse(
            res.headers['retry-after'] ?? '60',
          ) ??
          60;
      final delay = Duration(seconds: retryAfter * (1 << retryCount));
      await Future.delayed(delay);
      return _request<T>(method, path,
          body: body, skipAuth: skipAuth, retryCount: retryCount + 1);
    }

    // ── 5xx → retry exponentiel (max 3) ─────────────────────────────────────
    if (res.statusCode >= 500 && retryCount < 3) {
      await Future.delayed(Duration(seconds: 1 << retryCount));
      return _request<T>(method, path,
          body: body, skipAuth: skipAuth, retryCount: retryCount + 1);
    }

    // ── Pas de body JSON ─────────────────────────────────────────────────────
    final contentType = res.headers['content-type'] ?? '';
    if (!contentType.contains('application/json')) {
      if (res.statusCode >= 400) {
        throw ApiException('HTTP ${res.statusCode}', res.statusCode,
            isGatewayError: true);
      }
      return null as T;
    }

    final data = jsonDecode(res.body) as Map<String, dynamic>;

    // ── Erreur middleware → champ "message" ──────────────────────────────────
    if (res.statusCode >= 400 && data.containsKey('message')) {
      throw ApiException(
        data['message'] as String,
        res.statusCode,
        isGatewayError: true,
      );
    }

    // ── Erreur métier → champ "ErrorMessage" ─────────────────────────────────
    final errorMsg = data['ErrorMessage'] as String?;
    if (errorMsg != null && errorMsg.isNotEmpty) {
      throw ApiException(errorMsg, res.statusCode);
    }

    if (res.statusCode >= 400) {
      throw ApiException('HTTP ${res.statusCode}', res.statusCode);
    }

    return data as T;
  }

  // ── Interface publique ────────────────────────────────────────────────────

  Future<Map<String, dynamic>> get(String path, {bool skipAuth = false}) =>
      _request('GET', path, skipAuth: skipAuth);

  Future<Map<String, dynamic>> post(String path, Object body,
          {bool skipAuth = false}) =>
      _request('POST', path, body: body, skipAuth: skipAuth);

  Future<Map<String, dynamic>> patch(String path, Object body) =>
      _request('PATCH', path, body: body);

  Future<Map<String, dynamic>> put(String path, Object body) =>
      _request('PUT', path, body: body);

  Future<Map<String, dynamic>> delete(String path) =>
      _request('DELETE', path);
}
```

### Exemples d'utilisation

```dart
import 'package:tissimah/core/api/api_client.dart';

final api = ApiClient.instance;

// Route publique
final trips = await api.get(
  '/trip/passenger/getScheduledTripsPreviews'
  '?DepartureLocationName=Lomé&ArrivalLocationName=Accra'
  '&PassengerPositionLat=6.1375&PassengerPositionLng=1.2317'
  '&DistanceRange=5000&TripStartDate=2024-12-01'
  '&TripStartHour=0&TripArrivalHour=23&Index=0',
  skipAuth: true,
);

// Route protégée Firebase
final me = await api.get('/api/v1/user/me');
final userId = me['User']['UserID'] as String;

// Gestion d'erreur
try {
  await api.post('/booking/createBooking', {
    'PassengerId': userId,
    'TripId': tripId,
    'SeatsBooked': 1,
    'PaymentMethod': 'mobileMoney',
    // ...
  });
} on ApiException catch (e) {
  switch (e.statusCode) {
    case 409:
      // Conflit — réservation déjà existante
    case 412:
      // Précondition non remplie
    case 401:
      // Token invalide — rediriger vers login
    default:
      // Afficher e.message à l'utilisateur
  }
}

// Enregistrement token FCM (au démarrage de l'app)
final fcmToken = await FirebaseMessaging.instance.getToken();
await api.post('/api/v1/notifications/deviceToken', {
  'FcmToken': fcmToken,
  'Platform': Platform.isAndroid ? 'android' : 'ios',
  'DeviceName': await _getDeviceName(),
});
```

### Lancement avec les variables d'environnement

```bash
# Dev
flutter run \
  --dart-define=API_URL=https://api.tissimah.kpeewu.dev \
  --dart-define=MOBILE_APP_ID=<uuid-dev>

# Prod
flutter build apk \
  --dart-define=API_URL=https://api.tissimah.kpeewu.dev \
  --dart-define=MOBILE_APP_ID=<uuid-prod>
```

---

## 9. Debugging

### Lire le X-Request-ID

Chaque réponse de la gateway contient un `X-Request-ID`. Inclure cet identifiant dans les rapports de bug pour permettre au backend de retrouver les logs.

```typescript
// Web
const res = await fetch(url, options);
const requestId = res.headers.get('X-Request-ID');
console.error(`[${requestId}] Erreur`, err);
```

```dart
// Flutter — si tu utilises dio ou accès direct à la Response
final requestId = res.headers['x-request-id'];
debugPrint('[$requestId] Erreur: ${e.message}');
```

### Envoyer un X-Request-ID personnalisé

Pratique pour corréler les logs côté client et côté serveur avant même la réponse :

```typescript
import { v4 as uuidv4 } from 'uuid';

const reqId = uuidv4();
const res = await fetch(url, {
  headers: { 'X-Request-ID': reqId, ... },
});
// Si la requête échoue avant d'atteindre le serveur,
// tu as quand même l'ID pour le debugging local.
```

### Rate limit headers

Surveiller `X-RateLimit-Remaining-Minute` en développement pour détecter les boucles de requêtes involontaires. Exposés dans `Access-Control-Expose-Headers` — lisibles depuis le navigateur sans configuration supplémentaire.
