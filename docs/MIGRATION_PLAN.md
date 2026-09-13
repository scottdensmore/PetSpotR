# PetSpotR Architecture Migration Plan

This document details the architectural migration of PetSpotR from its legacy
prototype (Python, Azure, and Blazor) to an event-driven, production-grade Go
microservices platform hosted on Google Cloud Platform (GCP) with Gemma 4 AI
vision intelligence.

---

## 1. Executive Summary & Migration Objectives

The legacy PetSpotR implementation relied on Azure services (AKS, Cosmos DB, Azure
Service Bus, Azure Blob Storage) and Python/Blazor application code. The modern
architecture standardizes on:

1. **Go Toolchain**: High-performance, concurrent, and statically typed services
   targeting Go 1.25+ with the pinned Go 1.26.5 toolchain.
2. **Canonical Google Cloud Stack**:
   - **Google Cloud Run**: Serverless container execution with automated concurrency
     management, instance scaling caps, and internal ingress security.
   - **Google Cloud Pub/Sub**: Real-time event broker with push subscriptions,
     Google OIDC identity verification, dead-letter queues (DLQ), and exponential
     backoff retries.
   - **Google Cloud Firestore**: Scalable NoSQL document store with ACID multi-document
     transactions, transactional event outbox pattern, and candidate indexing.
   - **Google Cloud Storage (GCS)**: Private, encrypted object storage for pet images
     with direct client signed POST policies and capability token verification.
   - **Google Secret Manager**: Secure storage and runtime injection of sensitive
     credentials (session encryption keys, notification provider tokens).
3. **Gemma 4 Vision AI**: Local inference via Ollama (`gemma4:e2b`) for local
   development, with transition path to managed GCP private inference.
4. **Native Cloud & Local Emulators**: Replaced third-party sidecars (Dapr) with
   native Go SDK adapters (`cloud.google.com/go`) that seamlessly target local
   GCP emulators during development and managed Google services in staging and
   production.

---

## 2. Architecture Comparison

| Architectural Layer | Legacy Architecture (Python / Azure) | Canonical Architecture (Go / GCP / Gemma 4) |
| :--- | :--- | :--- |
| **Language & Runtime** | Python 3.8 & .NET Blazor | Go 1.25+ (Pinned Go 1.26.5) |
| **Web Frontend** | .NET Blazor WebAssembly (`src/frontend`) | Native Go Web UI (`cmd/web-frontend`) with WCAG 2.1 AA Accessibility |
| **Microservices** | Python / Flask monolith (`src/backend`) | 5 Decoupled Go Services (`lostpet-service`, `foundpet-service`, `pet-matcher`, `notification-service`, `web-frontend`) |
| **AI Vision Inference**| Azure ML Compute | Gemma 4 Vision AI (`gemma4:e2b` via Ollama / GCP Private Inference) |
| **Container Hosting** | Azure Kubernetes Service (AKS) | Google Cloud Run (Serverless, Concurrency & Scaling Caps) |
| **Object Storage** | Azure Blob Storage | Google Cloud Storage (GCS) Private Buckets + Signed Uploads |
| **State & Document DB**| Azure Cosmos DB / Redis | Google Cloud Firestore (ACID Transactions & Transactional Outbox) |
| **Event Messaging** | Azure Service Bus / Redis PubSub | Google Cloud Pub/Sub (OIDC Push Subscriptions & DLQ Routing) |
| **Secret Management** | Azure Key Vault | Google Secret Manager |
| **Infrastructure as Code**| Azure Bicep (`iac/*.bicep`) | OpenTofu (`infra/opentofu`) |
| **Local Development** | Dapr CLI + Docker | Docker Compose + Google Cloud Emulators + Ollama |

---

## 3. Canonical Service Decomposition

PetSpotR is structured into five cohesive microservices communicating via asynchronous
CloudEvents over Pub/Sub and RESTful HTTP APIs:

1. **`web-frontend`** (`cmd/web-frontend`):
   - Serves the modern, accessible web interface (`http://localhost:8082`).
   - Handles interactive lost pet reporting, found pet dropzones, and public directory search.
   - Integrates Google Identity Platform sessions for report ownership and match mediation.
   - Enforces token-bucket rate limiting and double-submit CSRF protection on public routes.

2. **`lostpet-service`** (`cmd/lostpet-service`):
   - Exposes `POST /lostPet` and `POST /lostPet/uploads`.
   - Generates secure GCS signed upload policies for pet photos.
   - Transactionally commits lost pet reports and durable `lostPet` outbox events.
   - Dispatches events via periodic asynchronous outbox polling relay.

3. **`foundpet-service`** (`cmd/foundpet-service`):
   - Exposes `POST /foundPet` and `POST /foundPet/uploads`.
   - Validates cryptographic upload capability tokens and finalizes private GCS objects.
   - Transactionally commits found pet reports and durable `foundPet` outbox events.
   - Runs periodic orphan image reconciliation to recover abandoned uploads.

4. **`pet-matcher`** (`cmd/pet-matcher`):
   - Private HTTP service receiving authenticated Pub/Sub push deliveries.
   - Performs asynchronous visual trait extraction using Gemma 4 AI vision model.
   - Executes multi-stage matching: species matching, Haversine geospatial radius filtering,
     and multi-trait similarity scoring.
   - Atomically persists match candidate records and publishes `matchFound` outbox events.

5. **`notification-service`** (`cmd/notification-service`):
   - Private HTTP service receiving authenticated Pub/Sub push deliveries for `matchFound`
     and `lostPet` community broadcast events.
   - Dispatches multi-channel alerts: HTML Email (SendGrid), SMS alerts (Twilio), and
     Web Push (VAPID).
   - Employs transactional delivery leases and idempotency keys to guarantee at-least-once
     delivery without duplicate notifications.

---

## 4. Phased Migration Status

### Phase 1: Go Core & Domain Models
- [x] **#2** Define Go module structure, domain entities, and unit test suite.
- [x] **#3** Implement state store and pub/sub abstraction interfaces in Go.

### Phase 2: AI Model Integration & Scoring
- [x] **#4** Implement Go Ollama API client module supporting Gemma models.
- [x] **#15** Gemma prompt engineering and structured JSON trait extraction.
- [x] **#16** Pet similarity scoring engine with weighted feature comparison.
- [x] **#6** Initial pet match notification engine interface.

### Phase 3: Service Foundation & Web Frontend
- [x] **#17** Go HTTP server bootstrap and `/lostPet` event handling.
- [x] **#18** `/foundPet` event handling with scoring integration and CloudEvents.
- [x] **#19** Go web frontend skeleton with responsive layout.
- [x] **#20** Lost pet report submission form with image preview.
- [x] **#21** Found pet search and match results view.

### Phase 4: GCP Infrastructure & Local Development
- [x] **#22** OpenTofu modules for Google Cloud Storage (GCS) and Cloud Pub/Sub.
- [x] **#23** OpenTofu modules for Google Cloud Run and Cloud Firestore.
- [x] **#11** Local development setup with Docker Compose, GCP emulators, and Ollama.
*(Note: Legacy Dapr component work #10 was superseded by native GCP SDK adapters).*

### Phase 5: Verification & CI/CD Pipelines
- [x] **#12** End-to-end test suite migration for Go web frontend.
- [x] **#13** GitHub Actions CI/CD workflow for automated validation and Cloud Run deployment.

### Phase 6: Modern UI, Accessibility & Directory
- [x] **#50** Implement modern design system with glassmorphism and theme switching.
- [x] **#51** Interactive lost pet report wizard with image upload.
- [x] **#52** Found pet reporting interface with AI trait auto-extraction.
- [x] **#53** Pet match comparison dashboard with visual side-by-side scoring breakdown.
- [x] **#54** Pet reunion and resolution workflow modal.
- [x] **#55** Interactive lost and found pet directory.
- [x] **#124** Public pet directory with deterministic pagination and search filters.
- [x] **#125** Deterministic filtering and pagination on directory API endpoints.
- [x] **#126** Accessible dropzones, focus management, and WCAG 2.1 AA keyboard navigation.

### Phase 7: Automated E2E User Journeys
- [x] **#56** Playwright user journey for lost pet reporting and photo upload.
- [x] **#57** Playwright user journey for found pet reporting and AI matching cascade.
- [x] **#58** Playwright user journey for match notification alert and email verification.
- [x] **#59** Playwright user journey for match confirmation and reunion resolution.
- [x] **#60** Playwright user journey for search, geospatial radius filtering, and pagination.
- [x] **#67** GitHub Actions workflow running Playwright API journey test suite.

### Phase 8: Backend Production Readiness & Security
- [x] **#61** Geospatial location indexing and distance-weighted matching.
- [x] **#62** GCS signed upload URL pipeline with cryptographic capability tokens.
- [x] **#63** Authentication, user sessions, and listing management endpoints.
- [x] **#65** Multi-channel notification engine (SendGrid Email, Twilio SMS, VAPID Web Push).
- [x] **#110** Enforce identity ownership boundaries, participant-only match reads, and contact privacy.
- [x] **#120** Typed runtime configuration validation and non-secret `.env.example`.
- [x] **#122** Cloud Run concurrency and scaling limits.
- [x] **#129** Abuse controls and token-bucket rate limiting on public endpoints.
- [x] **#150** Native deployment artifact and Knative manifest validation.

---

## 5. Engineering Standards & Quality Gates

All development on the PetSpotR platform must adhere to the following quality standards:

1. **Test-Driven Development (TDD)**: Unit and contract tests written before implementation.
2. **Deterministic Verification**: Every change must pass:
   - `export GOTOOLCHAIN=go1.26.5`
   - `go vet ./...`
   - `go test -race -cover ./...`
   - `golangci-lint run`
   - Playwright API journeys against local services.
3. **Branching & Commit Discipline**: Work is developed on descriptive feature/fix branches
   and committed using Conventional Commits.
