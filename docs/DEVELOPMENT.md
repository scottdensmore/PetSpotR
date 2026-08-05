# PetSpotR Development & Deployment Guide

Welcome to the PetSpotR Go & Google Cloud Platform codebase. This document
outlines how to build, test, run locally, and deploy PetSpotR.

---

## 1. Architecture Overview

PetSpotR is an event-driven microservice system written in **Go 1.22+**:

- **`lostpet-service`** (`cmd/lostpet-service`): Exposes REST API
  `POST /lostPet`, persists lost pet events, and emits `lostPet` events.
- **`foundpet-service`** (`cmd/foundpet-service`): Exposes REST API
  `POST /foundPet`, saves image blobs, persists found pet events, and emits
  `foundPet` events.
- **`pet-matcher`** (`cmd/pet-matcher`): Background worker subscribing to
  `foundPet` events, performing visual feature extraction using **Ollama** and
  **Gemma 4** models (`gemma4:2b`, with `gemma2:2b` fallback), scoring similarity
  against lost pets, and emitting `matchFound` events when the score is at
  least `0.70`.
- **`notification-service`** (`cmd/notification-service`): Background worker
  subscribing to `matchFound` events, generating owner alert emails, and
  logging notifications.

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

To launch all 4 Go microservices and an Ollama instance running Gemma 4
locally:

```bash
# Pull Gemma 4 model in Ollama
ollama pull gemma4:2b

# Start local microservice stack
docker-compose up --build
```

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
