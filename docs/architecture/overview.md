# Architecture Overview

This document provides a comprehensive overview of the Tissi-Mah platform architecture. It is intended for both technical and non-technical stakeholders.

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [High-Level Architecture](#high-level-architecture)
3. [Microservices](#microservices)
4. [Communication](#communication)
5. [Data Storage](#data-storage)
6. [External Services](#external-services)
7. [Environments](#environments)
8. [Security](#security)

---

## Executive Summary

### What is Tissi-Mah?

Tissi-Mah is a ride-sharing platform specifically designed for West Africa. It connects drivers and passengers for intercity travel, supporting both complete trips and partial segments between cities like Abidjan, Yamoussoukro, and Bouaké.

### Key Features

- **Intercity ride-sharing**: Connecting passengers with drivers for long-distance travel
- **Multi-segment bookings**: Passengers can book partial trips (e.g., Abidjan → Yamoussoukro only on an Abidjan → Bouaké trip)
- **Recurring trips**: Drivers can create recurring trip patterns (daily, weekly, custom)
- **Manual approval**: Drivers manually approve or reject passenger booking requests
- **Flexible cancellation**: Configurable refund policies based on cancellation timing
- **KYC verification**: Identity verification for drivers and passengers
- **Mobile payments**: Integration with local payment providers (Mobile Money, cards)

### Technology Stack

| Layer | Technologies |
|-------|--------------|
| **Backend** | Go, gRPC |
| **API Gateway** | Kong |
| **Orchestration** | Kubernetes (EKS, K3s) |
| **Databases** | PostgreSQL, MongoDB, Redis |
| **Cloud** | AWS (EKS, RDS, DocumentDB, ElastiCache, S3) |
| **Authentication** | Firebase Authentication |
| **Notifications** | Firebase Cloud Messaging |
| **Mobile Clients** | SwiftUI (iOS), Kotlin (Android) |

---

## High-Level Architecture

### Architecture Diagram

![Tissi-Mah Architecture](../diagrams/architecture-overview.png)

### Components Overview

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                   CLIENTS                                        │
│                        ┌─────────────┐  ┌─────────────┐                         │
│                        │   iOS App   │  │ Android App │                         │
│                        │  (SwiftUI)  │  │  (Kotlin)   │                         │
│                        └──────┬──────┘  └──────┬──────┘                         │
└────────────────────────────────┼────────────────┼────────────────────────────────┘
                                 │    HTTPS       │
                                 ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              CLOUD PROVIDER (AWS)                                │
│  ┌───────────────────────────────────────────────────────────────────────────┐  │
│  │                         KUBERNETES CLUSTER (EKS)                          │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐  │  │
│  │  │                    INGRESS (Kong API Gateway)                       │  │  │
│  │  │                      HTTPS → gRPC transformation                    │  │  │
│  │  └────────────────────────────────┬────────────────────────────────────┘  │  │
│  │                                   │ gRPC                                  │  │
│  │  ┌────────────────────────────────┼────────────────────────────────────┐  │  │
│  │  │                         MICROSERVICES                               │  │  │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │  │  │
│  │  │  │  Auth   │ │  User   │ │  Trips  │ │ Booking │ │ Payment │       │  │  │
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘       │  │  │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │  │  │
│  │  │  │ Rating  │ │ Vehicle │ │  File   │ │  Notif  │ │  Stats  │       │  │  │
│  │  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘       │  │  │
│  │  │  ┌─────────┐                                                        │  │  │
│  │  │  │ Geoloc  │                                                        │  │  │
│  │  │  └─────────┘                                                        │  │  │
│  │  └─────────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                      │                                           │
│         ┌────────────────────────────┼────────────────────────────┐             │
│         ▼                            ▼                            ▼             │
│  ┌─────────────┐            ┌─────────────┐              ┌─────────────┐        │
│  │ PostgreSQL  │            │   MongoDB   │              │    Redis    │        │
│  │   (RDS)     │            │ (DocumentDB)│              │(ElastiCache)│        │
│  └─────────────┘            └─────────────┘              └─────────────┘        │
│                                                                                  │
│  ┌─────────────┐                                                                │
│  │  S3 Bucket  │                                                                │
│  │  (Files)    │                                                                │
│  └─────────────┘                                                                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          ▼                           ▼                           ▼
┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐
│    Firebase     │        │  External APIs  │        │      OSRM       │
│ (Auth + FCM)    │        │   (Payments)    │        │   (Routing)     │
└─────────────────┘        └─────────────────┘        └─────────────────┘
```

### Request Flow

1. **Client** sends HTTPS request to Kong API Gateway
2. **Kong** validates JWT token (Firebase), applies rate limiting, transforms HTTPS → gRPC
3. **Service** receives gRPC request, processes business logic
4. **Service** communicates with other services via gRPC if needed
5. **Service** reads/writes to appropriate database
6. **Response** flows back through Kong, transformed to HTTPS/JSON

---

## Microservices

Tissi-Mah is composed of 11 microservices, each responsible for a specific domain.

### auth-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Authentication and token validation |
| **Functions** | Firebase JWT validation, session management, token refresh |
| **Dependencies** | user-service |
| **Database** | PostgreSQL |
| **Redis Usage** | Cache validated tokens, blacklist revoked tokens |

**Key Operations:**
- Verify Firebase ID tokens
- Create user session after first login
- Manage token refresh flow
- Handle logout (token blacklisting)

---

### user-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | User profile and identity management |
| **Functions** | Profile CRUD, KYC status, driver/passenger roles, preferences |
| **Dependencies** | auth-service, file-service |
| **Database** | MongoDB |
| **Redis Usage** | Cache frequently accessed user profiles |

**Key Operations:**
- Create/update user profiles
- Manage driver and passenger roles
- Track KYC verification status
- Store user preferences (notifications, language)
- Link profile pictures (via file-service)

---

### vehicle-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Vehicle management |
| **Functions** | Vehicle CRUD, document management, verification status |
| **Dependencies** | user-service, file-service |
| **Database** | PostgreSQL |
| **Redis Usage** | Cache vehicle information |

**Key Operations:**
- Register vehicles for drivers
- Store vehicle details (brand, model, color, plate)
- Manage vehicle documents (insurance, registration)
- Track document expiration dates

---

### trips-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Trip creation and management |
| **Functions** | Trip CRUD, waypoints, recurring patterns, search |
| **Dependencies** | user-service, vehicle-service |
| **Database** | PostgreSQL |
| **Redis Usage** | Cache trip searches, popular routes |

**Key Operations:**
- Create trips with departure, arrival, and waypoints
- Define pricing per seat and per segment
- Create recurring trip patterns (daily, weekly, custom)
- Search available trips by route and date
- Manage trip status (draft, scheduled, in-progress, completed, cancelled)

---

### booking-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Reservation management |
| **Functions** | Booking CRUD, segment management, status workflow, cancellation |
| **Dependencies** | user-service, trips-service, payment-service, notification-service |
| **Database** | PostgreSQL |
| **Redis Usage** | Cache active booking statuses |

**Key Operations:**
- Create booking requests (full trip or segments)
- Driver approval/rejection workflow
- Manage booking segments (pickup/dropoff points)
- Handle cancellations with refund calculation
- Track booking status history

**Booking Status Flow:**
```
pending → approved → completed
    ↓         ↓
rejected   cancelled
```

---

### payment-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Payment processing and financial operations |
| **Functions** | Payment collection, refunds, driver payouts |
| **Dependencies** | booking-service, user-service, notification-service |
| **Database** | PostgreSQL |
| **Redis Usage** | Cache pending payment statuses |

**Key Operations:**
- Process passenger payments (Mobile Money, card)
- Hold funds in escrow until trip completion
- Calculate and process refunds on cancellation
- Execute driver payouts (24-72h after trip completion)
- Track all financial transactions

**Payment Flow:**
```
pending → processing → completed
              ↓
           failed
```

---

### rating-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | User ratings and reviews |
| **Functions** | Rating submission, average calculation, review display |
| **Dependencies** | user-service, booking-service |
| **Database** | PostgreSQL |
| **Redis Usage** | Cache user rating averages |

**Key Operations:**
- Submit ratings after trip completion
- Calculate and update user averages
- Retrieve ratings for display on profiles
- Enforce one rating per booking per user

---

### file-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | File storage and retrieval |
| **Functions** | Upload, download, signed URLs, document verification |
| **Dependencies** | None (independent service) |
| **Database** | PostgreSQL (metadata) |
| **Redis Usage** | Cache signed S3 URLs |
| **Storage** | AWS S3 |

**Key Operations:**
- Upload files to S3 (images, documents)
- Generate signed URLs for secure access
- Store file metadata (type, size, owner)
- Track document verification status
- Manage document reviews (KYC verification)

**Supported Document Types:**
- Profile pictures
- ID cards (front/back)
- Passports
- Driver licenses
- Vehicle insurance
- Vehicle registration
- Vehicle photos

---

### notification-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Push notifications and messaging |
| **Functions** | Push notifications, email, SMS |
| **Dependencies** | user-service |
| **Database** | MongoDB |
| **Redis Usage** | Notification queue, rate limiting |

**Key Operations:**
- Send push notifications via Firebase Cloud Messaging
- Send email notifications
- Send SMS notifications
- Manage notification preferences
- Track delivery status

**Notification Triggers:**
- Booking request received (to driver)
- Booking approved/rejected (to passenger)
- Payment confirmation
- Trip reminder (24h before)
- Trip started/completed
- Refund processed

---

### stats-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Analytics and reporting |
| **Functions** | Data aggregation, dashboards, reports |
| **Dependencies** | All services (read-only) |
| **Database** | MongoDB |
| **Redis Usage** | Cache computed aggregations |

**Key Operations:**
- Aggregate booking statistics
- Calculate revenue metrics
- Track user growth
- Monitor platform health
- Generate reports for stakeholders

---

### geolocation-service

| Attribute | Value |
|-----------|-------|
| **Responsibility** | Real-time location tracking |
| **Functions** | Driver location updates, proximity search |
| **Dependencies** | user-service |
| **Database** | Redis (primary storage) |
| **Redis Usage** | Store real-time driver positions |

**Key Operations:**
- Receive and store driver location updates
- Query drivers near a location
- Track driver movement during trips
- Calculate ETAs

---

## Communication

### Internal Communication (Service-to-Service)

All internal service communication uses **gRPC** over HTTP/2.

**Benefits:**
- Strong typing with Protocol Buffers
- Efficient binary serialization
- Built-in code generation
- Streaming support
- Lower latency than REST

**Example:**
```
booking-service ──gRPC──► trips-service.GetTrip()
booking-service ──gRPC──► payment-service.CreatePayment()
booking-service ──gRPC──► notification-service.SendNotification()
```

### External Communication (Client-to-Backend)

Clients communicate via **HTTPS/JSON** through Kong API Gateway.

```
Mobile App ──HTTPS/JSON──► Kong ──gRPC──► Services
```

**Kong Responsibilities:**
- TLS termination
- JWT validation (Firebase tokens)
- Rate limiting
- HTTPS to gRPC transformation
- Request routing
- CORS handling

### Service Discovery

Services discover each other via **Kubernetes DNS**:

```
<service-name>.<namespace>.svc.cluster.local:<port>
```

**Examples:**
```
auth-service.tissi-mah.svc.cluster.local:50051
user-service.tissi-mah.svc.cluster.local:50051
trips-service.tissi-mah.svc.cluster.local:50051
```

---

## Data Storage

### Database Strategy

Each service owns its data and chooses the appropriate database type:

| Database | Type | Use Case | Services |
|----------|------|----------|----------|
| **PostgreSQL** | Relational | Transactional data, strong consistency | auth, trips, booking, payment, rating, vehicle, file |
| **MongoDB** | Document | Flexible schemas, nested data | user, notification, stats |
| **Redis** | Key-Value | Caching, real-time data | All services (cache), geolocation (primary) |

### PostgreSQL (Amazon RDS)

Used for services requiring:
- ACID transactions (payments, bookings)
- Complex queries with joins
- Strong data consistency
- Relational data models

**Services:** auth-service, trips-service, booking-service, payment-service, rating-service, vehicle-service, file-service

### MongoDB (Amazon DocumentDB)

Used for services requiring:
- Flexible, evolving schemas
- Nested document structures
- High write throughput
- JSON-like data storage

**Services:** user-service, notification-service, stats-service

### Redis (Amazon ElastiCache)

**As Cache (all services):**
- Reduce database load
- Speed up frequent queries
- Store session data
- Rate limiting counters

**As Primary Storage (geolocation-service):**
- Real-time driver positions
- Geospatial queries
- TTL-based expiration

### File Storage (Amazon S3)

All files (images, documents) are stored in S3:

- **Bucket structure:** `tissi-mah-{env}-files`
- **Access:** Only via file-service
- **Security:** Signed URLs with expiration
- **Organization:** `/{document_type}/{user_id}/{file_id}`

---

## External Services

> **Note:** This section will be completed with detailed integration documentation.

### Firebase

- **Firebase Authentication**: Client-side authentication (email, phone, social)
- **Firebase Cloud Messaging**: Push notifications to mobile devices

### Payment Providers

- Details to be added

### Routing Service

- **OSRM**: Route calculation and distance estimation

---

## Environments

### Environment Overview

| Environment | Infrastructure | Purpose | Deployment |
|-------------|---------------|---------|------------|
| **Local** | Docker Compose | Development | Manual |
| **VPS-Dev** | K3s on VPS | Integration testing | Automatic (on merge to develop) |
| **Staging** | AWS EKS | Pre-production testing | Manual |
| **Production** | AWS EKS | Live platform | Manual |

### Local Environment

```
Developer Machine
├── Docker Compose
│   ├── All 11 services
│   ├── PostgreSQL
│   ├── MongoDB
│   ├── Redis
│   └── Kong (optional)
```

**Setup:** See [Local Setup Guide](../deployment/local-setup.md)

### VPS-Dev Environment

```
VPS (Hetzner)
├── K3s (lightweight Kubernetes)
│   ├── All 11 services
│   └── Kong
├── PostgreSQL (single instance)
├── MongoDB (single instance)
└── Redis (single instance)
```

**Purpose:** Shared development environment for team integration testing

**Setup:** See [VPS Dev Setup Guide](../deployment/vps-dev-setup.md)

### Staging Environment

```
AWS
├── EKS Cluster
│   ├── All 11 services (2 replicas each)
│   └── Kong
├── RDS PostgreSQL (db.t3.medium)
├── DocumentDB (db.t3.medium)
├── ElastiCache Redis (cache.t3.micro)
└── S3 Bucket
```

**Purpose:** Pre-production testing with production-like infrastructure

**Setup:** See [Staging Deployment Guide](../deployment/staging-deployment.md)

### Production Environment

```
AWS
├── EKS Cluster (multi-AZ)
│   ├── All 11 services (3+ replicas each)
│   ├── Kong (3 replicas)
│   └── HPA (auto-scaling)
├── RDS PostgreSQL (db.r5.large, Multi-AZ)
├── DocumentDB (3-node cluster)
├── ElastiCache Redis (cluster mode)
└── S3 Bucket (versioning enabled)
```

**Purpose:** Live production platform

**Setup:** See [Production Deployment Guide](../deployment/prod-deployment.md)

---

## Security

### Authentication Flow

```
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│  Mobile  │──1──►│ Firebase │──2──►│  Mobile  │──3──►│   Kong   │
│   App    │◄──2──│   Auth   │      │   App    │      │          │
└──────────┘      └──────────┘      └──────────┘      └────┬─────┘
                                                           │ 4
                                                           ▼
                                                    ┌──────────┐
                                                    │   Auth   │
                                                    │ Service  │
                                                    └──────────┘

1. User authenticates with Firebase (email/phone/social)
2. Firebase returns ID token to mobile app
3. Mobile app includes token in Authorization header
4. Kong validates token and forwards request to auth-service
```

### Security Layers

| Layer | Security Measure |
|-------|------------------|
| **Transport** | TLS 1.3 (HTTPS) for all external communication |
| **Authentication** | Firebase JWT tokens |
| **Authorization** | Role-based access (driver, passenger, admin) |
| **API Gateway** | Rate limiting, request validation |
| **Data** | Encryption at rest (RDS, S3), field-level encryption for sensitive data |
| **Secrets** | Kubernetes Secrets, AWS Secrets Manager |

### KYC Verification

Tissi-Mah implements Know Your Customer (KYC) verification:

**For Passengers:**
- Phone number verification (required)
- ID document verification (optional but encouraged)

**For Drivers:**
- Phone number verification (required)
- ID document verification (required)
- Driver license verification (required)
- Vehicle documents verification (required)

**Verification Flow:**
```
Document Uploaded → Pending Review → Under Review → Approved/Rejected
                                          ↓
                                    Request Resubmission
```

### Data Protection

- **Personal data**: Encrypted at rest, access logged
- **Payment data**: PCI-compliant, tokenized
- **Location data**: Retained only for active trips, anonymized for analytics
- **Documents**: Encrypted in S3, signed URLs with short expiration

---

## Quick Reference

### Service Ports

| Service | gRPC Port |
|---------|-----------|
| auth-service | 50051 |
| user-service | 50052 |
| vehicle-service | 50053 |
| trips-service | 50054 |
| booking-service | 50055 |
| payment-service | 50056 |
| rating-service | 50054 |
| file-service | 50058 |
| notification-service | 50059 |
| stats-service | 50060 |
| geolocation-service | 50061 |

### Service Dependencies Matrix

```
                    auth  user  vehicle  trips  booking  payment  rating  file  notif  stats  geoloc
auth-service         -     →      -       -       -        -        -      -      -      -       -
user-service         →     -      -       -       -        -        -      →      -      -       -
vehicle-service      -     →      -       -       -        -        -      →      -      -       -
trips-service        -     →      →       -       -        -        -      -      -      -       →
booking-service      -     →      -       →       -        →        -      -      →      -       -
payment-service      -     →      -       -       →        -        -      →      →      -       -
rating-service       -     →      -       -       →        -        -      -      →      -       -
file-service         -     -      -       -       -        -        -      -      -      -       -
notification-service -     →      -       -       -        -        -      -      -      -       -
stats-service        →     →      →       →       →        →        →      →      →      -       →
geolocation-service  -     →      -       -       -        -        -      -      -      -       -
```

### Database Mapping

| Service | PostgreSQL | MongoDB | Redis (Cache) | Redis (Primary) | S3 |
|---------|------------|---------|---------------|-----------------|-----|
| auth-service | ✓ | - | ✓ | - | - |
| user-service | - | ✓ | ✓ | - | - |
| vehicle-service | ✓ | - | ✓ | - | - |
| trips-service | ✓ | - | ✓ | - | - |
| booking-service | ✓ | - | ✓ | - | - |
| payment-service | ✓ | - | ✓ | - | - |
| rating-service | ✓ | - | ✓ | - | - |
| file-service | ✓ | - | ✓ | - | ✓ |
| notification-service | - | ✓ | ✓ | - | - |
| stats-service | - | ✓ | ✓ | - | - |
| geolocation-service | - | - | - | ✓ | - |