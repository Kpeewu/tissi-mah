# Geolocation Service API

Routing self-hosted (OSRM) + geocoding (Nominatim) + tuiles vectorielles
(TileServer GL) sur Togo, Ghana, Bénin, Burkina Faso. Appelé par le front
mobile pendant la **création d'un trajet** pour afficher le tracé entre les
waypoints, calculer distance/durée totales, et proposer une heure d'arrivée
estimée pour chaque waypoint.

> **Important** : `trips-service` n'appelle PAS ce service. Le front interroge
> `/api/v1/geolocation/route` pendant l'édition des waypoints, reçoit le
> tracé + ETAs, et inclut ces valeurs dans le payload `POST /api/v1/trip/driver/createTrip`.
> trips-service stocke tel quel.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local       | `http://localhost:8080` |
| VPS-Dev     | `https://dev.tissi-mah.com` |
| Staging     | `https://staging.tissi-mah.com` |
| Production  | `https://api.tissi-mah.com` |

Les **tuiles vectorielles** ont un domaine dédié (bypass api-gateway pour
servir efficacement du binaire) :

| Environment | Tile Server |
|-------------|-------------|
| VPS-Dev     | `https://tiles.tissimah.kpeewu.dev` |
| Production  | `https://tiles.tissimah.kpeewu.dev` |

## Service Discovery (gRPC inter-service)

| Environment | Address |
|-------------|---------|
| Local       | `localhost:50064` |
| Kubernetes  | `geolocation-service.dev.svc.cluster.local:50064` |

## Authentication

Toutes les routes `/api/v1/geolocation/*` requièrent un **JWT Firebase valide**
sauf `/health`. Le JWT est validé par l'api-gateway qui injecte le `x-firebase-uid`
en metadata gRPC — le service le journalise mais ne l'utilise pas (les
opérations sont purement géographiques, pas liées à un user spécifique).

```
Authorization: Bearer <firebase_id_token>
```

## Error Response Format

```json
{ "ErrorMessage": "ErrorRouteNotFound" }
```

| HTTP Code | Meaning |
|-----------|---------|
| 200 | Success (le payload contient `ErrorMessage` si erreur logique côté backend) |
| 400 | Invalid request (`INVALID_ARGUMENT`) |
| 401 | Missing or invalid token (`UNAUTHENTICATED`) |
| 503 | Backend OSRM/Nominatim indisponible (`UNAVAILABLE`) |

---

## Endpoints

### POST /api/v1/geolocation/route

Calcule la distance, la durée et le tracé (polyline) entre une séquence de
waypoints. Si `DepartureTime` est fourni, retourne aussi l'heure d'arrivée
estimée à chaque waypoint intermédiaire.

**Authentication** : Firebase JWT requis.

#### Request

```http
POST /api/v1/geolocation/route HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
Content-Type: application/json

{
  "Waypoints": [
    { "Lat": 6.1319, "Lng": 1.2228 },
    { "Lat": 6.5,    "Lng": 1.0 },
    { "Lat": 6.9269, "Lng": 0.6266 }
  ],
  "Profile": "driving",
  "DepartureTime": "2026-04-26T08:00:00Z"
}
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Waypoints` | array | Yes | Au moins 2 coordonnées en ordre départ → … → arrivée |
| `Waypoints[].Lat` | float | Yes | Latitude (-90 à 90) |
| `Waypoints[].Lng` | float | Yes | Longitude (-180 à 180) |
| `Profile` | string | No | `driving` (V1, défaut). Autres valeurs rejetées. |
| `DepartureTime` | string | No | RFC3339. Si fourni, calcule `EstimatedArrivalTime` par leg. |

#### Response (Success)

```json
{
  "DistanceMeters": 120000,
  "DurationSeconds": 7200,
  "PolylineEncoded": "_p~iF~ps|U_ulLnnqC...",
  "Legs": [
    {
      "DistanceMeters": 60000,
      "DurationSeconds": 3000,
      "MinutesFromDeparture": 50,
      "EstimatedArrivalTime": "2026-04-26T08:50:00Z"
    },
    {
      "DistanceMeters": 60000,
      "DurationSeconds": 4200,
      "MinutesFromDeparture": 120,
      "EstimatedArrivalTime": "2026-04-26T10:00:00Z"
    }
  ],
  "ErrorMessage": ""
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `DistanceMeters` | float | Distance totale en mètres |
| `DurationSeconds` | int64 | Durée totale en secondes |
| `PolylineEncoded` | string | Google encoded polyline (compact, ~1 KB pour un trajet inter-villes). Décodable côté front via maplibre-polyline-codec ou équivalent. |
| `Legs` | array | `len(Waypoints) - 1` entrées. Index N = segment entre `Waypoints[N]` et `Waypoints[N+1]`. |
| `Legs[].MinutesFromDeparture` | int32 | Cumul depuis `Waypoints[0]` (utilisé pour pré-remplir `MinutesFromDeparture` du waypoint dans `CreateTrip`) |
| `Legs[].EstimatedArrivalTime` | string | RFC3339, vide si `DepartureTime` non fourni. À utiliser pour pré-remplir `ScheduledPickupDatetime` du waypoint. |

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorInvalidWaypoints` | 400 | < 2 waypoints, ou coordonnées hors plage |
| `ErrorInvalidProfile` | 400 | Profile autre que `driving` |
| `ErrorInvalidDepartureTime` | 400 | DepartureTime fourni mais format RFC3339 invalide |
| `ErrorRouteNotFound` | 200 (logique) | OSRM n'a pas trouvé de route entre les points (ex: trop loin du réseau routier) |
| `ErrorRoutingOverloaded` | 503 | Trop de requêtes simultanées sur ce pod (semaphore plein). Le client peut retry après backoff. |
| `ErrorRoutingUnavailable` | 503 | OSRM injoignable ou circuit breaker ouvert |

#### Exemple cURL

```bash
curl -X POST https://api.tissi-mah.com/api/v1/geolocation/route \
  -H "Authorization: Bearer $FIREBASE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "Waypoints": [
      {"Lat":6.1319,"Lng":1.2228},
      {"Lat":6.9269,"Lng":0.6266}
    ],
    "Profile":"driving",
    "DepartureTime":"2026-04-26T08:00:00Z"
  }'
```

---

### GET /api/v1/geolocation/geocode

Résout un texte (adresse, lieu) en coordonnées via Nominatim. Filtré par
défaut sur les 4 pays cibles (Togo, Ghana, Bénin, Burkina).

**Authentication** : Firebase JWT requis.

#### Request

```http
GET /api/v1/geolocation/geocode?Query=Marché+de+Lomé&Limit=5 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Query` | string | Yes | Texte à résoudre |
| `CountryFilter` | string | No | ISO codes séparés par virgules. Défaut serveur : `tg,gh,bj,bf`. |
| `Limit` | int32 | No | Nombre max de résultats (défaut 5) |

#### Response (Success)

```json
{
  "Results": [
    {
      "DisplayName": "Marché de Lomé, Togo",
      "Lat": 6.13,
      "Lng": 1.22,
      "Country": "tg",
      "City": "Lomé",
      "Type": "amenity"
    }
  ],
  "ErrorMessage": ""
}
```

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorEmptyQuery` | 400 | Query vide |
| `ErrorAddressNotFound` | 200 (logique) | Aucun résultat pour cette query |
| `ErrorGeocodingOverloaded` | 503 | Semaphore Nominatim plein |
| `ErrorGeocodingUnavailable` | 503 | Nominatim injoignable, circuit breaker ouvert, ou `NOMINATIM_URL` non configuré côté serveur |

---

### GET /api/v1/geolocation/reverse

Résout des coordonnées en adresse via Nominatim.

**Authentication** : Firebase JWT requis.

#### Request

```http
GET /api/v1/geolocation/reverse?Lat=6.13&Lng=1.22 HTTP/1.1
Host: api.tissi-mah.com
Authorization: Bearer <firebase_id_token>
```

#### Response (Success)

```json
{
  "Result": {
    "DisplayName": "Lomé, Togo",
    "Lat": 6.13,
    "Lng": 1.22,
    "Country": "tg",
    "City": "Lomé",
    "Type": "city"
  },
  "ErrorMessage": ""
}
```

#### Errors

| ErrorMessage | HTTP | Description |
|--------------|------|-------------|
| `ErrorInvalidWaypoints` | 400 | Lat/Lng hors plage |
| `ErrorAddressNotFound` | 200 | Pas de match Nominatim |
| `ErrorGeocodingOverloaded` / `ErrorGeocodingUnavailable` | 503 | idem Geocode |

---

### GET /api/v1/geolocation/health

Health check public.

```json
{
  "Status": "SERVING",
  "Version": "1.0.0",
  "Timestamp": 1745647200
}
```

---

## Tuiles vectorielles (TileServer GL)

**Pas via api-gateway.** Le mobile appelle directement le TileServer pour
récupérer les tuiles MapLibre.

### GET https://tiles.tissi-mah.com/data/west-africa/{z}/{x}/{y}.pbf

Tuile vectorielle Mapbox Vector Tile (format `.pbf`). Compatible MapLibre GL.

```bash
curl -I https://tiles.tissi-mah.com/data/west-africa/13/4012/3851.pbf
# Content-Type: application/x-protobuf
# Cache-Control: public, max-age=604800, immutable
```

### GET https://tiles.tissi-mah.com/data/west-africa.json

Métadonnées TileJSON du dataset (zoom min/max, attribution, URL pattern…).
À fournir à MapLibre GL :

```js
const map = new maplibregl.Map({
  container: 'map',
  style: 'https://tiles.tissi-mah.com/data/west-africa.json',
  center: [1.2228, 6.1319],
  zoom: 8,
});
```

---

## Cache et performance

Le service applique trois couches de protection (cf. plan d'implémentation) :

1. **Cache Redis** — clés hashées sur les waypoints. Hit ratio attendu > 80%
   en régime normal (preview UI = beaucoup de re-queries identiques pendant
   qu'un chauffeur drague un waypoint). TTL : 1h pour les routes, 24h pour
   les geocodings.
2. **Semaphore par pod** — 50 appels OSRM concurrents max, 30 Nominatim. Au-delà,
   `ErrorRoutingOverloaded` immédiat.
3. **Circuit breaker** — 5 échecs consécutifs → `ErrorRoutingUnavailable`
   pendant 30s, sans frapper le backend. Évite l'effet retry-storm.

Latences typiques mesurées (vps-dev) :
- Cache hit : 3-5 ms
- Cache miss → OSRM : 50-200 ms (selon distance)
- Cache miss → Nominatim : 100-400 ms

## Error Reference

| Error | HTTP | gRPC | Description |
|-------|------|------|-------------|
| `ErrorInvalidWaypoints` | 400 | INVALID_ARGUMENT | Validation locale des coordonnées |
| `ErrorInvalidProfile` | 400 | INVALID_ARGUMENT | Profile non supporté |
| `ErrorInvalidDepartureTime` | 400 | INVALID_ARGUMENT | DepartureTime mal formé |
| `ErrorEmptyQuery` | 400 | INVALID_ARGUMENT | Geocode appelé sans query |
| `ErrorRouteNotFound` | 200 | NOT_FOUND | OSRM `code=NoRoute` |
| `ErrorAddressNotFound` | 200 | NOT_FOUND | Nominatim sans résultat |
| `ErrorRoutingOverloaded` | 503 | RESOURCE_EXHAUSTED | Semaphore OSRM plein |
| `ErrorRoutingUnavailable` | 503 | UNAVAILABLE | OSRM down / breaker open |
| `ErrorGeocodingOverloaded` | 503 | RESOURCE_EXHAUSTED | Semaphore Nominatim plein |
| `ErrorGeocodingUnavailable` | 503 | UNAVAILABLE | Nominatim down / breaker open / non configuré |
| `ErrorInternalServer` | 500 | INTERNAL | Erreur non catégorisée |
