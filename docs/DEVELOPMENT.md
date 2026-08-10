# PetSpotR Development & Deployment Guide

Welcome to the PetSpotR Go & Google Cloud Platform codebase. This document
outlines how to build, test, run locally, and deploy PetSpotR.

---

## 1. Architecture Overview

PetSpotR is an event-driven microservice system written in **Go 1.22+**:

- **`web-frontend`** (`cmd/web-frontend`): Modern web application UI at `http://localhost:8082`,
  offering Lost Pet wizards, Found Pet dropzone, AI visual match comparison dashboard, and Web Push.
- **`lostpet-service`** (`cmd/lostpet-service`): Exposes REST API
  `POST /lostPet`, persists lost pet events, and emits `lostPet` events.
- **`foundpet-service`** (`cmd/foundpet-service`): Exposes REST API
  `POST /foundPet`, saves image blobs, persists found pet events, and emits
  `foundPet` events.
- **`pet-matcher`** (`cmd/pet-matcher`): Background worker subscribing to
  `foundPet` events, performing visual feature extraction using **Ollama** and
  **Gemma 4** models (`gemma4:e2b`), scoring similarity against lost pets, and
  emitting `matchFound` events.
- **`notification-service`** (`cmd/notification-service`): Background worker
  subscribing to `matchFound` events, generating owner alert emails, SMS, and Web Push notifications.

---

## 2. Local Development

### Prerequisites

- Go 1.22+ installed locally.
- Docker & Docker Compose installed.
- Ollama installed locally or running via Docker Compose.

### Running Unit Tests

To run all package unit tests and static analysis:

```bash
# Run unit tests across all packages
go test -v ./...

# Run tests with race detector and coverage
go test -race -v -cover ./...

# Run static analysis
go vet ./...
```

### Running Locally with Docker Compose

To launch all 5 microservices (`web-frontend`, `lostpet-service`, `foundpet-service`,
`pet-matcher`, `notification-service`) and an Ollama instance running Gemma 4 locally:

```bash
# Start the stack and automatically download Gemma 4 on first use
docker compose up --build
```

Compose waits for Ollama to become healthy and downloads `gemma4:e2b` into a
persistent volume before starting the pet matcher. Subsequent starts reuse the
downloaded model. Ollama stays inside the Compose network to avoid conflicting
with a host installation. To use another model, set `OLLAMA_MODEL` before
starting the stack.

Access the application in your browser at `http://localhost:8082`.

### State Runtime Modes

The four stateful processes select their state backend with
`PETSPOTR_RUNTIME_MODE`:

| Mode | Required configuration | State backend | Intended use |
| --- | --- | --- | --- |
| `memory` | None | Process-local memory | Unit tests and demo-only development |
| `local-emulator` | `GOOGLE_CLOUD_PROJECT`, `FIRESTORE_EMULATOR_HOST` | Firestore emulator | Shared local development and integration tests |
| `gcp` | Application Default Credentials; optional explicit `GOOGLE_CLOUD_PROJECT` | Managed Firestore | Deployed environments |

Outside Cloud Run, an unset mode defaults to `memory` for compatibility with
the current Compose demo. Cloud Run selects `gcp` automatically, rejects an
explicit `memory` mode, and detects the project ID through Application Default
Credentials or the metadata server when `GOOGLE_CLOUD_PROJECT` is not set. A
deployed service therefore cannot silently start with ephemeral state.

Managed emulator and GCP servers return redacted public lost-pet records.
Contact, match/reunion transition, and push-subscription endpoints remain
disabled with `403 Forbidden` outside explicit `memory` demo mode until
issue #110 adds authentication and ownership enforcement.

For example, after starting a Firestore emulator on port 8085:

```bash
export PETSPOTR_RUNTIME_MODE=local-emulator
export GOOGLE_CLOUD_PROJECT=petspotr-local
export FIRESTORE_EMULATOR_HOST=127.0.0.1:8085
go run ./cmd/lostpet-service
```

Run the cross-client persistence contract against that emulator with:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./pkg/runtimeconfig \
  -run TestStateRuntimeSharesAndRetainsStateWithFirestoreEmulator
```

The separate-process contract builds the real lost-pet and web binaries,
writes through the first process, reads through the second, restarts it, and
verifies retention:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./e2e \
  -run TestFirestoreStateCrossesServiceProcessesAndSurvivesRestart
```

This runtime slice covers shared state only. Pub/Sub delivery and GCS uploads
remain in-memory until issues #108 and #109 are completed, so it does not yet
provide a complete cross-process event cascade.

---

## 3. Google Cloud Platform (GCP) Deployment

### OpenTofu / Terraform Infrastructure Setup

Infrastructure is defined as code under `infra/opentofu`:

- GCS Bucket for image storage (`modules/storage`)
- Cloud Pub/Sub topics (`modules/pubsub`)
- Cloud Firestore database (`modules/firestore`)
- Cloud Run v2 services (`modules/cloudrun`)

To apply infrastructure:

```bash
cd infra/opentofu
tofu init
tofu plan -var="project_id=your-gcp-project-id"
tofu apply -var="project_id=your-gcp-project-id"
```

### Deploying to Cloud Run

Knative-compatible service manifests are located under `deploy/cloudrun/`:

- `lostpet-service.yaml`
- `foundpet-service.yaml`
- `pet-matcher.yaml`
- `notification-service.yaml`

To deploy a service:

```bash
gcloud run services replace deploy/cloudrun/lostpet-service.yaml \
  --region=us-central1
```
