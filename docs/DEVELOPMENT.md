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
- **`pet-matcher`** (`cmd/pet-matcher`): Private HTTP service receiving
  authenticated `foundPet` Pub/Sub push requests, performing visual feature
  extraction using **Ollama** and **Gemma 4** models (`gemma4:e2b`), scoring
  similarity against lost pets, and emitting `matchFound` events.
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
starting the stack. Compose exposes the matcher health and push endpoint on
port 8083. Its default static push token is development-only; set
`PUBSUB_PUSH_DEV_TOKEN` to override it.

The default Compose stack still uses process-local state and messaging. It is
useful for UI and isolated-service development, but it is not a shared
cross-container event cascade. Use the Firestore and Pub/Sub emulator contracts
below to exercise durable state and real push delivery. A single-command shared
emulator stack remains tracked separately in issue #117.

Access the application in your browser at `http://localhost:8082`.

### State Runtime Modes

The five stateful processes select their state backend with
`PETSPOTR_RUNTIME_MODE`:

| Mode | Required configuration | State backend | Intended use |
| --- | --- | --- | --- |
| `memory` | None | Process-local memory | Unit tests and demo-only development |
| `local-emulator` | `GOOGLE_CLOUD_PROJECT`, `FIRESTORE_EMULATOR_HOST` | Firestore emulator | Shared local development and integration tests |
| `gcp` | Application Default Credentials; optional explicit `GOOGLE_CLOUD_PROJECT` | Managed Firestore | Deployed environments |

Outside Cloud Run, an unset mode defaults to `memory` for compatibility with
the current Compose demo. Cloud Run selects `gcp` automatically, rejects an
explicit `memory` or `local-emulator` mode, and detects the project ID through
Application Default Credentials or the metadata server when
`GOOGLE_CLOUD_PROJECT` is not set. A deployed service therefore cannot silently
start with ephemeral state or emulator credentials.

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

### Messaging Runtime and Pub/Sub Push

The found-report publisher and pet matcher use the same runtime-mode selection
as state:

| Mode | Required configuration | Messaging backend |
| --- | --- | --- |
| `memory` | Matcher: `PUBSUB_FOUND_SUBSCRIPTION`, `PUBSUB_PUSH_DEV_TOKEN` | Process-local test broker and static push authentication |
| `local-emulator` | `GOOGLE_CLOUD_PROJECT`, `PUBSUB_EMULATOR_HOST`; matcher push variables above | Google Pub/Sub emulator |
| `gcp` | Application Default Credentials; matcher: `PUBSUB_FOUND_SUBSCRIPTION`, `PUBSUB_PUSH_SERVICE_ACCOUNT` | Managed Pub/Sub and verified Google OIDC push identity |

In GCP mode the matcher accepts only a valid Google-signed ID token whose
verified email is the configured invocation service account. The subscription
name in the wrapped push body must also match. A handler returns `204` only
after processing succeeds; transient processing failures return `500` for
redelivery, while malformed poison messages return `400`.

Run the Pub/Sub emulator in Docker from one terminal:

```bash
docker run --rm --network host \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start \
  --host-port=127.0.0.1:8086 \
  --project=petspotr-push-contract
```

Then run the real publish-to-push contract from another terminal:

```bash
PUBSUB_EMULATOR_HOST=127.0.0.1:8086 \
  go test ./e2e \
  -run TestPubSubEmulatorRedeliversWrappedMessageUntilHandlerSucceeds
```

The Pub/Sub emulator does not implement IAM, so this contract uses the static
development token on the push URL. Unit tests independently enforce exact OIDC
service-account identity, verified-email, audience, and invalid-signature
handling. Never configure `PUBSUB_PUSH_DEV_TOKEN` in GCP mode.
Cloud Run rejects both local runtime modes even when they are explicitly set.

### Transactional report outbox

The lost- and found-report services create aggregate state and a durable
`eventOutbox` record in one transaction. Each outbox payload uses envelope
version 1 with a stable event ID, type, occurrence time, correlation and trace
IDs, aggregate ID and version, and payload version. Consumers accept both this
envelope and the legacy raw event payload so messages already in flight remain
readable.

An exact retry is a successful no-op and cannot reset a completed outbox
record. A competing create with the same pet ID returns `409 Conflict`; report
creation is aggregate version 1 and does not use last-write-wins ordering. The
relay serializes publication within one process. The found-report service also
polls a bounded, indexed set of pending `foundPet` records every five seconds,
so failed publication stays pending across a restart and is recovered. Before
polling, a cursor-backed bounded compatibility sweep adds the query fields
missing from legacy key/data-only Firestore outbox documents. Incomplete sweeps
advance from the durable cursor. Completed sweeps restart after one minute so a
late write from an old Cloud Run revision cannot remain behind the old cursor.

The found-report Cloud Run service temporarily runs exactly one minimum
instance with CPU available outside requests. This is required for the
five-second relay and prevents unclaimed multi-instance publication. Issue #122
owns later load/cost tuning. A
crash after broker publication but before the completion write can publish the
record again, which is the expected at-least-once boundary.

Run the live atomic-write contract against a Firestore emulator with:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./pkg/store \
  -run TestFirestoreCreateStateAndOutboxTransaction
```

This slice wires the durable `foundPet` outbox to managed Pub/Sub and migrates
the pet matcher to authenticated push delivery. The OpenTofu module retains
unconsumed `matchFound` events in a backlog subscription until the notification
consumer is migrated in the next issue #108 slice. Lost-pet publication and
notification push delivery are not yet managed paths. Multi-instance outbox
claiming and the remaining consumer paths stay in issue #116. GCS uploads
remain in-memory until issue #109 is completed.

### Idempotent owner-notification delivery

The `matchFound` owner-notification path derives one opaque delivery operation
for each event, recipient, and channel. Current envelopes use their verified
stable event ID. Legacy raw payloads receive a stable compatibility ID derived
from their event type and exact payload bytes, so an in-flight message remains
safe under redelivery.

Each operation is claimed in a Firestore transaction with a one-minute lease
and monotonically increasing attempt fence. A concurrent handler returns an
error while the lease is active. An expired or failed operation can be
reclaimed, but a stale worker cannot complete the newer attempt. Completed
channels are durable no-ops, so a later channel failure retries only unfinished
work.

The operation ID is also passed to every channel provider as its idempotency
key. This closes the crash window where provider delivery succeeds but writing
the completed operation fails: after the lease expires, the retry uses the same
provider key. Every production sender must preserve that contract when
issue #65 replaces the current development senders.

Run the live concurrent-claim and fencing contract against a Firestore emulator
with:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./pkg/store \
  -run TestFirestoreDeliveryOperationClaimAndFencing
```

Focused worker tests also cover duplicate envelopes, partial multi-channel
failure, and provider success followed by completion-store failure. This slice
does not yet make lost-pet community broadcasts idempotent or migrate the
notification worker to authenticated Cloud Run push; those remain in
issues #116 and #108 respectively.

---

## 3. Google Cloud Platform (GCP) Deployment

### OpenTofu / Terraform Infrastructure Setup

Infrastructure is defined as code under `infra/opentofu`:

- GCS Bucket for image storage (`modules/storage`)
- Cloud Pub/Sub topics, authenticated `foundPet` push subscription, retry and
  dead-letter policies, dedicated matcher invocation identity, and retained
  `matchFound` backlog (`modules/pubsub`)
- Cloud Firestore database and pending-outbox composite index
  (`modules/firestore`)
- Cloud Run v2 services, including private matcher ingress, push identity
  configuration, and the single always-CPU found-report relay instance
  (`modules/cloudrun`)

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

OpenTofu is the authoritative deployment path for the authenticated matcher
subscription and its least-privilege IAM grants. The standalone manifests
contain example project-qualified identity values and do not create Pub/Sub or
IAM resources; replace those placeholders if applying a manifest directly.

To deploy a service:

```bash
gcloud run services replace deploy/cloudrun/lostpet-service.yaml \
  --region=us-central1
```
