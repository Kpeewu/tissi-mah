# KYC Service API

Ce document décrit l'API HTTP/REST exposée par le kyc-service via l'api-gateway (grpc-gateway).

La vérification d'identité est **100 % manuelle** : les documents sont soumis via le
file-service, puis validés par un agent support depuis le back-office. Il n'y a plus
de fournisseur d'identité externe.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local | `http://localhost:8080` |
| VPS-Dev | `https://dev.tissi-mah.com` |
| Staging | `https://staging.tissi-mah.com` |
| Production | `https://api.tissi-mah.com` |

## Authentication

| Routes | Authentification |
|--------|------------------|
| `/api/v1/kyc/me/*` | **JWT Firebase** (utilisateur mobile) |
| `/api/v1/kyc/admin/*` | **JWT support** (back-office) — rôle `support` ou `admin` |
| `/api/v1/kyc/health` | publique |

`POST /api/v1/kyc/admin/reviews/override` exige en plus le rôle **`admin`** :
annuler un rejet est une action sensible.

```http
Authorization: Bearer <firebase-id-token>   # routes /me
Authorization: Bearer <support-jwt>         # routes /admin
```

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Oui (POST) | `application/json` |
| `Authorization` | Oui (routes protégées) | `Bearer <token>` |
| `X-App-ID` | Oui | UUID de l'app (mobile ou back-office support) |

## Error Response Format

```json
{
    "ErrorMessage": "ErrorDocumentAlreadyReviewed"
}
```

---

## Règles de vérification

Les flags sont calculés à partir des **documents courants** (`is_current = true`) et de
leur statut — jamais depuis l'historique des reviews. Toute resoumission, tout
remplacement de selfie et tout override sont donc pris en compte uniformément.

```
selfieOK   = selfie approuvé
idProofOK  = (idCardFront + idCardBack) | passport | (driverLicenceFront + driverLicenceBack) approuvés
licenceOK  = driverLicenceFront + driverLicenceBack approuvés
vehicleOK(v) = insurance(v) ET registrationCard(v) approuvées

IdentityVerified (passager) = selfieOK ET idProofOK
DriverVerified              = selfieOK ET licenceOK ET ∃ véhicule v : vehicleOK(v)
vehicles.is_verified[v]     = vehicleOK(v)
```

**Points clés**

- Le **selfie est obligatoire** : une pièce approuvée seule ne suffit pas. Le support
  compare le selfie à la pièce d'identité. Le selfie sert aussi de photo de profil.
- Le **permis a un double rôle explicite** : il vaut pièce d'identité *et* preuve du
  droit de conduire. Une seule décision support débloque les deux ; il appartient donc
  aux deux catégories (`passenger` et `driver`) dans la file de validation.
- **Vérification par véhicule** : `DriverVerified` exige au moins un véhicule
  entièrement validé. Chaque véhicule doit être individuellement validé
  (`is_verified`) pour être utilisable dans `POST /api/v1/trip/driver/createTrip`.
- **Unité de validation = document logique** : un recto-verso (CNI, permis) est un
  seul document. Le support voit et valide les deux faces ensemble ; une décision
  s'applique aux deux.

---

## Endpoints

### GET /api/v1/kyc/me/getStatus

Statut KYC de l'utilisateur connecté.

**Authentification :** JWT Firebase

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "IdentityVerified": true,
    "DriverVerified": false,
    "PendingReviews": [
        {
            "ReviewId": "rev-550e8400-e29b-41d4-a716-446655440000",
            "Status": "pending",
            "AttemptNumber": 1,
            "DocumentType": "insurance"
        }
    ],
    "LatestRejection": {
        "ReviewId": "rev-1c2d…",
        "ReasonRejection": "document_illegible",
        "RejectionDetails": "Photo floue",
        "ReviewType": "manual",
        "ReviewedAt": "2026-04-15T09:12:00Z"
    },
    "ErrorMessage": ""
}
```

`PendingReviews` liste les documents en attente de décision support (une entrée par
document logique). `LatestRejection` est le rejet le plus récent, pour afficher le
motif à l'utilisateur.

---

### GET /api/v1/kyc/admin/manualReviews/requests

File de validation, groupée par utilisateur.

**Authentification :** JWT support

#### Query Parameters

Les paramètres suivent la casse des champs proto (**PascalCase**).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `Page` | integer | Non | Page 0-based (défaut 20/page) |
| `PageSize` | integer | Non | Taille de page |
| `Status` | string | Non | Garde les users dont `PassengerStatus` OU `DriverStatus` vaut ce statut |
| `Name` / `FirstName` | string | Non | Recherche partielle, insensible à la casse |
| `DepositFrom` / `DepositTo` | string | Non | Bornes du dernier dépôt (ISO 8601 ou `YYYY-MM-DD`) |

#### Response (Success)

```json
{
    "Requests": [
        {
            "UserId": "usr-550e…",
            "Name": "Diallo",
            "FirstName": "Amadou",
            "Email": "amadou@example.com",
            "PhoneNumber": "+22890000000",
            "ProfileImageURL": "https://…",
            "PassengerStatus": "pending",
            "DriverStatus": "rejected",
            "TotalDocuments": 4,
            "LastDepositAt": "2026-04-15T08:00:00Z"
        }
    ],
    "Total": 1,
    "ErrorMessage": ""
}
```

- `PassengerStatus` agrège selfie + pièces d'identité (**permis inclus**).
- `DriverStatus` agrège permis + documents véhicule.
- `TotalDocuments` compte les documents **logiques** (un recto-verso = 1) ; la photo
  de profil historique n'est pas comptée.
- Agrégation par précédence : `rejected` > `underReview` > `pending` > `expired` > `approved`.

---

### GET /api/v1/kyc/admin/manualReviews/requestDetail

Détail d'une demande : tous les documents soumis par l'utilisateur.

**Authentification :** JWT support

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserId` | string | Oui | UserID interne |

#### Response (extrait)

```json
{
    "UserId": "usr-550e…",
    "Documents": [
        {
            "DocumentId": "doc-front",
            "DocumentType": "driverLicenceFront",
            "LogicalDocumentType": "driverLicence",
            "Status": "pending",
            "OwnerKind": "user",
            "Category": "driver",
            "Categories": ["passenger", "driver"],
            "DocumentUrl": "https://…/recto.jpg",
            "SecondDocumentId": "doc-back",
            "SecondDocumentUrl": "https://…/verso.jpg",
            "LatestReview": {
                "ReviewId": "rev-…",
                "Status": "pending",
                "Decision": "pending",
                "Notes": "",
                "AttemptNumber": 2,
                "PreviousReviewId": "rev-precedente"
            }
        }
    ]
}
```

Chaque document recto-verso est **une seule entrée** portant les deux faces
(`DocumentUrl` + `SecondDocumentUrl`) : l'agent les compare ensemble. Le selfie
apparaît comme document `selfie` (catégorie `passenger`).

---

### POST /api/v1/kyc/admin/validateDocument

Décision d'un agent support sur un document.

**Authentification :** JWT support

#### Request

```json
{
    "DocumentId": "doc-550e…",
    "VehicleId": "",
    "Decision": "approved",
    "ReasonRejection": "",
    "RejectionDetails": "",
    "Notes": "Selfie conforme à la pièce"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DocumentId` | string | Oui | ID du document — **n'importe quelle face** d'un recto-verso |
| `VehicleId` | string | Conditionnel | Requis pour un document véhicule (assurance, carte grise) |
| `Decision` | string | Oui | `approved` \| `rejected` \| `resubmission` — `pending` refusé |
| `ReasonRejection` | string | Conditionnel | Requis si `rejected` |
| `RejectionDetails` | string | Non | Commentaire libre |
| `Notes` | string | Non | Note interne (visible dans le détail et l'historique) |

Motifs de rejet : `document_expired`, `document_incomplete`, `document_illegible`,
`photo_missmatch`, `information_missmatch`, `wrong_document_type`, `other`.

#### Response (Success)

```json
{
    "ReviewId": "rev-…",
    "Decision": "approved",
    "ReviewedBy": "support-uid",
    "ReviewType": "manual",
    "ReviewedAt": "2026-04-15T09:30:00Z",
    "Notes": "Selfie conforme à la pièce",
    "DocumentId": "doc-recto",
    "SecondDocumentId": "doc-verso",
    "LogicalDocumentType": "idCard"
}
```

#### Effets

1. Le statut des documents concernés (**les deux faces**) est synchronisé sur la décision.
2. Les flags de vérification sont recalculés depuis les documents courants et poussés
   vers user-service ; `is_verified` est poussé par véhicule vers vehicle-service.
3. L'utilisateur est notifié (push + email). Sur bascule `false → true` : notification
   « identité vérifiée » / « conducteur vérifié ».

#### Errors

| Error | HTTP | Description |
|-------|------|-------------|
| `ErrorInvalidDecision` | 400 | Décision invalide (dont `pending`) ou motif manquant sur un rejet |
| `ErrorDocumentMismatch` | 400 | Le document véhicule n'appartient pas au `VehicleId` fourni |
| `ErrorDocumentNotFound` | 404 | Document inexistant |
| `ErrorDocumentAlreadyReviewed` | 412 | Document déjà validé → passer par `override` |
| `ErrorCompanionDocumentMissing` | 412 | Recto-verso incomplet : l'autre face manque |
| `ErrorFileServiceUnavailable` | 503 | file-service injoignable |

---

### POST /api/v1/kyc/admin/reviews/override

Annule un **rejet** en créant une nouvelle review chaînée (l'historique est conservé).

**Authentification :** JWT support — **rôle `admin` requis**

#### Request

```json
{
    "ReviewId": "rev-550e…",
    "Decision": "approved",
    "ReasonRejection": "",
    "RejectionDetails": "",
    "Notes": "Pièce vérifiée manuellement auprès de l'usager"
}
```

Seules les reviews `completed` **et** `rejected` sont overridables. `Decision` accepte
`approved` | `rejected` | `resubmission` (jamais `pending`).

#### Errors

| Error | HTTP | Description |
|-------|------|-------------|
| `ErrorReviewNotFound` | 404 | Review inexistante |
| `ErrorReviewNotOverridable` | 412 | Review non terminée |
| `ErrorOnlyRejectionOverridable` | 412 | Seul un rejet peut être overridé |

---

### GET /api/v1/kyc/admin/reviews/getReviews

Liste paginée des reviews (filtres `UserId`, `Status`, `Decision`, `Index` — 20/page).

**Authentification :** JWT support

### GET /api/v1/kyc/admin/reviews/getReview

Détail complet d'une review (`ReviewId` en query).

**Authentification :** JWT support

### GET /api/v1/kyc/admin/documentHistory

Historique chronologique complet d'un document logique.

**Authentification :** JWT support

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `UserId` | string | Oui | Propriétaire |
| `LogicalDocumentType` | string | Oui | `idCard` \| `passport` \| `driverLicence` \| `selfie` \| `insurance` \| `registrationCard` |

Chaque entrée expose la décision, le motif, les notes, `AttemptNumber`,
`PreviousReviewId`, l'agent ayant tranché, et les IDs des documents concernés
(`DocumentId` — y compris pour les documents véhicule — et `SecondDocumentId`).

---

### GET /api/v1/kyc/health

Health check (public).

```json
{
    "Status": "SERVING",
    "Version": "1.0.0",
    "Timestamp": 1742040000
}
```

---

## Statuts

### Statut d'un document

| Statut | Description |
|--------|-------------|
| `pending` | Déposé, en attente de décision support |
| `approved` | Validé par le support |
| `rejected` | Refusé (ou resoumission demandée) — resoumission possible via `changeDocument` |
| `expired` | Expiré |
| `underReview` | Hérité, plus écrit |

### Statut d'une review

| Statut | Description |
|--------|-------------|
| `pending` | Créée à l'upload, aucune décision |
| `completed` | Décision prise (`approved`, `rejected` ou `resubmission`) |
| `expired` / `failed` | États terminaux résiduels |

Une review `pending` n'a **pas** de `ReviewedAt` : cette date n'est renseignée qu'au
moment de la décision.

## Error Reference

| Error | HTTP | gRPC | Description |
|-------|------|------|-------------|
| `ErrorInvalidDecision` | 400 | INVALID_ARGUMENT (3) | Décision invalide ou motif de rejet manquant |
| `ErrorMissingUserID` | 400 | INVALID_ARGUMENT (3) | Identifiant utilisateur absent |
| `ErrorMissingDocumentID` | 400 | INVALID_ARGUMENT (3) | `DocumentId` absent |
| `ErrorMissingReviewID` | 400 | INVALID_ARGUMENT (3) | `ReviewId` absent |
| `ErrorDocumentMismatch` | 400 | INVALID_ARGUMENT (3) | Document véhicule / `VehicleId` incohérents |
| `ErrorInvalidDateRange` | 400 | INVALID_ARGUMENT (3) | Bornes `DepositFrom`/`DepositTo` invalides |
| `ErrorDocumentNotFound` | 404 | NOT_FOUND (5) | Document inexistant |
| `ErrorReviewNotFound` | 404 | NOT_FOUND (5) | Review inexistante |
| `ErrorUserNotFound` | 404 | NOT_FOUND (5) | Utilisateur inexistant |
| `ErrorUnauthorized` | 403 | PERMISSION_DENIED (7) | Non autorisé (rôle support/admin) |
| `ErrorDocumentAlreadyReviewed` | 412 | FAILED_PRECONDITION (9) | Document déjà validé |
| `ErrorReviewNotOverridable` | 412 | FAILED_PRECONDITION (9) | Review non overridable |
| `ErrorOnlyRejectionOverridable` | 412 | FAILED_PRECONDITION (9) | Seul un rejet est overridable |
| `ErrorCompanionDocumentMissing` | 412 | FAILED_PRECONDITION (9) | Face compagnon manquante |
| `ErrorInternalServer` | 500 | INTERNAL (13) | Erreur interne |
| `ErrorFileServiceUnavailable` | 503 | UNAVAILABLE (14) | file-service injoignable |
