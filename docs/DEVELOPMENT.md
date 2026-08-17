# PetSpotR Development & Deployment Guide

Welcome to the PetSpotR Go & Google Cloud Platform codebase. This document
outlines how to build, test, run locally, and deploy PetSpotR.

---

## 1. Architecture Overview

PetSpotR is an event-driven microservice system targeting **Go 1.25.8** and
built with the pinned **Go 1.26.5** toolchain:

- **`web-frontend`** (`cmd/web-frontend`): Modern web application UI at `http://localhost:8082`,
  offering Lost Pet wizards, Found Pet dropzone, AI visual match comparison dashboard, and Web Push.
- **`lostpet-service`** (`cmd/lostpet-service`): Exposes REST API
  `POST /lostPet`, persists lost pet events, and emits `lostPet` events.
- **`foundpet-service`** (`cmd/foundpet-service`): Exposes REST API
  `POST /foundPet/uploads` and `POST /foundPet`, validates private image
  uploads, persists found pet events, and emits `foundPet` events.
- **`pet-matcher`** (`cmd/pet-matcher`): Private HTTP service receiving
  authenticated `lostPet` and `foundPet` Pub/Sub push requests, persisting
  lost-image traits, performing found-image feature extraction using **Ollama**
  and **Gemma 4** models (`gemma4:e2b`), scoring similarity against lost pets,
  and emitting `matchFound` events.
- **`notification-service`** (`cmd/notification-service`): Private HTTP service
  receiving authenticated `matchFound` Pub/Sub push requests and generating
  owner alert emails, SMS, and Web Push notifications.

---

## 2. Local Development

### Prerequisites

- Go 1.26.5 installed locally, or available through `GOTOOLCHAIN=go1.26.5`.
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
starting the stack. Compose exposes matcher and notification health and push
endpoints on ports 8083 and 8084. Their default static push token is
development-only; set `PUBSUB_PUSH_DEV_TOKEN` to override it.

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
export PUBSUB_EMULATOR_HOST=127.0.0.1:8086
go run ./cmd/lostpet-service
```

The lost- and found-report binaries initialize both managed adapters, so their
`local-emulator` processes require both emulator endpoints. State-package tests
that instantiate only `StateRuntime` still require Firestore alone.

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
PUBSUB_EMULATOR_HOST=127.0.0.1:8086 \
  go test ./e2e \
  -run TestFirestoreStateCrossesServiceProcessesAndSurvivesRestart
```

### Messaging Runtime and Pub/Sub Push

The lost- and found-report publishers, pet matcher, and notification service
use the same runtime-mode selection as state:

Managed and emulator publishing uses `cloud.google.com/go/pubsub/v2`. The
runtime reuses one publisher handle per topic, waits for Pub/Sub to acknowledge
each event, and closes every publisher handle before closing the shared client.
Topics and subscriptions must already exist; production infrastructure and the
emulator contracts create them explicitly.

Event envelope and payload versions evolve independently. Readers must accept
the legacy raw payload and payload version 1 while those messages can remain in
flight. Found-pet producers emit additive payload version 2. Lost-pet producers
emit payload version 4, which adds an optional finalized private `imageObject`
to the contact-redacted version-3 shape. Its decoder continues to accept raw
and enveloped payload version 1, the prior contact-bearing payload version 2,
and contact-redacted payload version 3. Private phone and finder-contact data
are never copied into report events, and current lost-pet events also omit
reporter email. Removing or renaming a published field requires a new payload
version and a tolerant decoder for every supported prior shape.

Deploy the lost-pet payload-version-4 consumers first: update
`notification-service` to accept lost-pet payload versions 1 through 4, and
update `pet-matcher` with its distinct `lost-pet-matcher-analysis`
subscription. Only after both revisions are serving should `lostpet-service`
and `web-frontend` be deployed to publish version 4. The consumer-first order
keeps every in-flight prior version readable and prevents old consumers from
rejecting the private-image payload.

The matcher and notification workers decode report events through the
canonical payload-version readers. Those readers normalize raw legacy and
payload-version-1 messages without trusting fields that did not exist in that
schema. Community broadcasts require `verified` geocoding plus validated
coordinates; older messages and pending locations are accepted but do not
trigger a broadcast or receive an invented fallback location.

| Mode | Required configuration | Messaging backend |
| --- | --- | --- |
| `memory` | Consumer: `PUBSUB_PUSH_SUBSCRIPTION`, `PUBSUB_PUSH_DEV_TOKEN` | Process-local test broker and static push authentication |
| `local-emulator` | `GOOGLE_CLOUD_PROJECT`, `PUBSUB_EMULATOR_HOST`; consumer push variables above | Google Pub/Sub emulator |
| `gcp` | Application Default Credentials; consumer: `PUBSUB_PUSH_SUBSCRIPTION`, `PUBSUB_PUSH_SERVICE_ACCOUNT` | Managed Pub/Sub and verified Google OIDC push identity |

The matcher and notification services also require
`PUBSUB_LOST_SUBSCRIPTION`. Within each service it must be different from
`PUBSUB_PUSH_SUBSCRIPTION` so each route is bound to exactly one managed
subscription. The matcher binds it to `lost-pet-matcher-analysis`; the
notification service binds it to `lost-pet-notification`.

In GCP mode each consumer accepts only a valid Google-signed ID token whose
verified email is that consumer's configured invocation service account. The
subscription name in the wrapped push body must also match. A handler returns
`204` only after processing succeeds. Transient provider failures and invalid
event payloads return `500` for redelivery and eventual DLQ routing; malformed
Pub/Sub wrappers and mismatched delivery metadata return `400` and are also
unacknowledged.

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

The notification-specific contract runs the real worker behind its HTTP push
endpoint. It verifies a transient channel failure is retried without repeating
completed channels, then confirms a poison event is redelivered:

```bash
PUBSUB_EMULATOR_HOST=127.0.0.1:8086 \
  go test ./internal/app/notification \
  -run TestNotificationPubSubEmulatorDeliversRetriesAndRetainsPoison
```

Run the equivalent `lostPet` community-broadcast contract with:

```bash
PUBSUB_EMULATOR_HOST=127.0.0.1:8086 \
  go test ./internal/app/notification \
  -run TestNotificationLostPetPubSubEmulatorDeliversRetriesAndRetainsPoison
```

The Pub/Sub emulator does not implement IAM, so this contract uses the static
development token on the push URL. Unit tests independently enforce exact OIDC
service-account identity, verified-email, audience, and invalid-signature
handling. Never configure `PUBSUB_PUSH_DEV_TOKEN` in GCP mode.
Cloud Run rejects both local runtime modes even when they are explicitly set.
Managed `lostPet`, `foundPet`, and `matchFound` subscriptions own transport
retry and DLQ routing; issue #91 retains provider circuit-breaker work.

### Private image storage

The lost-pet service, found-pet service, and pet matcher select image storage
with the same `PETSPOTR_RUNTIME_MODE` contract:

| Mode | Required configuration | Image backend |
| --- | --- | --- |
| `memory` | Optional `PETSPOTR_IMAGE_BASE_URL` | Process-local compatibility store |
| `local-emulator` | `PETSPOTR_IMAGE_BUCKET`, `STORAGE_EMULATOR_HOST` as an HTTP(S) URL | GCS-compatible local emulator |
| `gcp` | `PETSPOTR_IMAGE_BUCKET`, Application Default Credentials | Private managed GCS bucket |

Storage follows `PETSPOTR_RUNTIME_MODE` by default. Mixed local contracts that
exercise Firestore or Pub/Sub without a GCS emulator may set the explicit
`PETSPOTR_STORAGE_MODE=memory` component override. Cloud Run still rejects
every non-GCP component mode, so this override cannot weaken managed startup.

Managed mode rejects an emulator endpoint and never falls back to memory.
Cloud Run receives the bucket name from OpenTofu. The lost- and found-pet
identities have object-user access and may sign short-lived policies through
IAM Credentials; the matcher has read-only object access.

The secure found-pet flow is:

1. `POST /foundPet/uploads` with `purpose: "found-pet"` and a JPEG or PNG
   content type. The service ignores the caller filename and returns a
   cryptographically generated report ID, temporary object name, and V4 POST
   policy fields. It also returns a separate `finalizeToken`; only its SHA-256
   digest is signed into object metadata.
2. Multipart POST the file and every returned field to `uploadUrl`. The policy
   expires after 15 minutes and binds the exact object, media type, report
   metadata, finalization deadline, and a 1 byte through 10 MiB size range.
3. `POST /foundPet` using the returned `reportId` as `petId` and temporary
   `objectName` as `imageObject`, and send the raw capability in
   `X-PetSpotR-Upload-Token`. Before creating report state or its outbox, the
   service checks the capability in constant time plus immutable GCS
   generation, size, metadata, detected media type, decoded dimensions, and
   report binding. The signed finalization deadline remains enforced after the
   object is copied, and the whole report operation has a two-minute server
   deadline. It then copies the object to the private finalized namespace and
   deletes the temporary object. The raw capability is never persisted in GCS
   metadata or the durable event.

Durable events store only the finalized object name. The matcher reads that
private object with its service identity and base64-encodes the bytes for
Ollama; no public or expiring URL is placed in an event. Private read URLs are
V4-signed and capped at 15 minutes. Existing URL events remain readable during
the schema migration, but managed found-pet report creation rejects them.

The bucket enforces public access prevention and uniform access. Set
`image_cors_allowed_origins` to exact deployed web origins; the default empty
list permits no browser origin. Unfinalized `uploads/` objects are deleted
after one day. The found-pet process also reconciles at most 100 finalized
objects per pass with a cursor. After a 24-hour grace period—well beyond the
15-minute capability and two-minute report deadlines—it deletes an object only
when two durable-state checks find no report referencing the exact name and the
object generation is unchanged. This recovers the crash window between GCS
finalization and the Firestore report transaction without deleting active,
referenced, or replaced objects. The capability authenticates the generated
report upload; tying that capability to a signed-in user remains tracked by
issue #110. Routing the browser through the canonical service remains tracked
by issue #113.

The optional lost-pet flow uses the same capability and validation lifecycle
under the separate `uploads/lost-pets/` and `images/lost-pets/` namespaces:

1. `POST /lostPet/uploads` with `purpose: "lost-pet"` and a JPEG or PNG
   content type, then upload using the returned V4 POST policy.
2. `POST /lostPet` with the generated `reportId`, temporary `imageObject`, and
   `X-PetSpotR-Upload-Token`. The service finalizes and verifies the object
   before atomically creating report state and its payload-v4 outbox event.

Reports may omit `imageObject`; they remain valid but have no visual evidence
for asynchronous analysis. The matcher receives image-bearing reports through
its authenticated `lost-pet-matcher-analysis` subscription, verifies the object
belongs to the report's private lost-pet namespace, reads it with the matcher
identity, and persists parsed traits plus model, analysis version, source event,
source object, and verification time. A transactional ten-minute delivery lease
admits one model invocation; a retry after state persistence reuses the durable
analysis instead of invoking the model again. Payload versions 1 through 3 and
version 4 reports without images are acknowledged as no-ops. The private object
name remains in persisted internal state and the integration event; trait
provenance exists only in persisted internal state. Neither is exposed by the
unauthenticated lost-pet listing DTO. Lost-pet orphan reconciliation is
purpose-scoped so it cannot delete found-pet objects, and vice versa.

An opt-in deployed contract exercises signing, upload, finalization, private
service reads, and a signed read URL against a real bucket:

```bash
PETSPOTR_GCS_INTEGRATION_BUCKET=your-disposable-test-bucket \
  go test ./pkg/blob -run TestGCSDeployedSecureImageLifecycle
```

The test creates uniquely named objects and deletes its finalized object. Use
only a disposable bucket configured with the same private-access policy.

### Live Ollama scoring

An independent opt-in contract exercises the scoring pipeline against a live
Ollama server. Start Ollama so it is reachable at `http://localhost:11434` (or
set `OLLAMA_HOST` to its base URL) and pull the model required by this test:

```bash
ollama pull gemma2:2b
```

Then run the focused contract with the CI-pinned Go toolchain:

```bash
GO_INTEGRATION_OLLAMA=1 \
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}" \
GOTOOLCHAIN=go1.26.5 \
  go test ./pkg/scoring \
  -run '^TestScoringPipeline_LiveOllamaIntegration$' -v
```

The test specifically requests `gemma2:2b`; the Compose stack's default
`gemma4:e2b` model does not satisfy this contract. When
`GO_INTEGRATION_OLLAMA` is unset or not `1`, the test skips and the package
still reports `ok`. This live contract is not part of the default verifier or
CI battery, so do not claim live Ollama coverage unless the command above ran.

### Transactional report state and outbox

The lost- and found-report services create a contact-free aggregate, its
private `reportContacts` record, and a durable `eventOutbox` record in one
transaction. The aggregate links to contact through a stable, report-scoped
owner or finder identity reference; public DTOs omit both the reference and the
private record. Each outbox payload uses envelope version 1 with a stable event
ID, type, occurrence time, correlation and trace IDs, aggregate ID and version,
and payload version. Consumers accept both this envelope and the legacy raw
event payload so messages already in flight remain readable. Exact retries of
the prior contact-bearing aggregate shape are also accepted without rewriting
legacy state.

An exact retry is a successful no-op and cannot reset a completed outbox
record. A competing create with the same pet ID returns `409 Conflict`; report
creation is aggregate version 1 and does not use last-write-wins ordering. Each
relay serializes publication within one process. The lost- and found-report
services poll a bounded, indexed set of their pending records every five
seconds, so failed publication stays pending across a restart and is recovered.
Before polling, a cursor-backed bounded compatibility sweep adds the query
fields missing from legacy key/data-only Firestore outbox documents. Incomplete
sweeps advance from the durable cursor. Completed sweeps restart after one
minute so a late write from an old Cloud Run revision cannot remain behind the
old cursor.

The lost- and found-report Cloud Run services temporarily run exactly one
minimum instance each with CPU available outside requests. This is required for
their five-second relays. Issue #122 owns later load/cost tuning. A crash after
broker publication but before the completion write can publish the record
again, which is the expected at-least-once boundary.

Before broker I/O, every relay instance transactionally leases the pending
outbox record for ten minutes and increments its fenced attempt. An active lease
is a successful no-op for a competing relay. Provider failure releases the
lease immediately; a process crash leaves it recoverable after expiry. Only the
winning attempt can persist completion or failure, so a stale process cannot
overwrite a newer retry. The stable event envelope lets consumers deduplicate
the unavoidable crash window after broker acceptance but before the completion
transaction.

Run the live atomic-write contract against a Firestore emulator with:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./pkg/store \
  -run TestFirestoreCreateStateAndOutboxTransaction
```

Run the two-client relay claim contract against the same emulator with:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./pkg/outbox \
  -run TestFirestoreConcurrentRelaysClaimOnePublication
```

The durable `lostPet` and `foundPet` outboxes publish to managed Pub/Sub. The
pet matcher receives independent authenticated `foundPet` matching and
`lostPet` image-analysis deliveries. Its durable `matchFound` output reaches
the notification service through a separate authenticated subscription, and
the notification service independently receives `lostPet` community-broadcast
pushes. All four subscriptions have retry, retention, expiration, and DLQ
policy. Multi-instance outbox claiming is durable. Found-pet image events carry
a private finalized GCS object name in managed mode and retain legacy
`imageUrl` decoding for messages already in flight.

### Idempotent matcher result publication

The pet matcher derives one durable processing operation from the verified
`foundPet` envelope ID, or from a stable digest of an exact legacy payload. A
ten-minute transactional lease admits only one concurrent model invocation.
Completed inputs are no-ops, while failed and expired attempts can be reclaimed
without allowing a stale attempt to record completion.

Matching begins only after the found report has verified coordinates. The
worker queries Firestore's lost-pet candidate indexes for active reports within
30 days of the found timestamp and the latitude/longitude bounding box around
its 15-mile radius. It includes unknown species but excludes mismatched known
species in the indexed query, then applies exact Haversine filtering. It never
parses a user-entered location into fallback coordinates. It sorts eligible
candidates by ID before scoring, then chooses the highest score with distance
and candidate ID as deterministic tie-breakers. Pending or unavailable
geocoding completes without invoking the found-image model or creating a match.
After those geographic and species filters, image-less lost reports and records
with invalid analysis provenance are ineligible. If any otherwise eligible,
image-bearing report is still waiting for analysis, the fenced found-event
operation fails before found-image model access so Pub/Sub can retry after the
lost analysis lands. This prevents a ready candidate from winning prematurely
while another candidate's traits are pending. Once every eligible image-bearing
candidate is ready, scoring uses only its validated model-derived breed, colors,
markings, and eye color; reporter-entered visual fields are never a scoring
fallback.

The lost-image consumer derives a separate durable operation from the verified
`lostPet` envelope ID, or from a stable digest for exact legacy payloads. Reports
without images complete without storage or model access. For image-bearing
reports, the event object must match both the exact lost-pet namespace and the
durable aggregate before analysis. Successful model output is bounded,
normalized, and transactionally added to the existing lost record. Model,
parsing, storage, or persistence failures request Pub/Sub redelivery.

When a found event references a finalized private object, the same matching
operation validates that object against the durable found record, persists its
bounded analysis and provenance before scoring, and reuses those verified
traits after a failed delivery attempt or worker restart. Exact reporter
retries ignore but preserve this matcher-owned enrichment. Legacy found events
that carry only `imageUrl` remain readable and use the prior inline analysis
path without treating an external URL as verified private-object provenance.

Lost-pet writes now duplicate only query metadata beside the opaque state blob:
status, geocoding status, normalized species, report timestamp, and verified
coordinates. On startup, `pet-matcher` completes a cursor-backed migration of
legacy `lostPets` documents before accepting Pub/Sub deliveries. During a
rolling deployment, deploy the lost-pet producers before the matcher so new
writes carry those fields while the compatibility migration catches prior
records. The OpenTofu Firestore module owns both candidate composite indexes;
apply them before deploying the matcher revision.

Run the indexed-query and legacy-migration contract against a Firestore
emulator with:

```bash
GOTOOLCHAIN=go1.26.5 go test ./pkg/store -count=1 -v \
  -run '^TestFirestoreQueriesAndBackfillsBoundedLostPetCandidates$'
```

When scoring produces a match, the additive `sourceEventId` field keeps ordered
input versions distinct. The worker derives a stable match ID from that source
event and the two report IDs, then atomically creates the canonical `matches`
record, its `matcherResults` deduplication record, and the exact `matchFound`
`eventOutbox` payload before broker I/O. The match record preserves the score
components, distance, model, threshold version, timestamp, and explanation
used for the decision without copying private reporter contact. A retry loads
that winning result and publishes its existing outbox record rather than
invoking Ollama again. Broker failure releases the outbox lease for an
immediate retry. A crash after broker acceptance can publish the same stable
event again after lease expiry; the notification delivery operations below
deduplicate that expected at-least-once boundary. No-candidate and no-match
outcomes complete the processing operation without creating an event.

Focused worker tests cover duplicate inputs, concurrent handlers, broker
failure after the atomic write, and consumer-completion failure after successful
publication. Run the two-client recovery contract against a Firestore emulator
with:

```bash
FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 \
  go test ./internal/app/petmatcher \
  -run 'TestFirestore(MatcherRecoversPersistedResult|(Lost|Found)ImageAnalysisSurvivesCompletionRetry)AcrossWorkers'
```

### Idempotent notification delivery

The `matchFound` owner-notification and `lostPet` community-broadcast paths
derive one opaque delivery operation for each event, recipient, and channel.
Current envelopes use their verified stable event ID. Legacy raw payloads
receive a stable compatibility ID derived from their event type and exact
payload bytes, so an in-flight message remains safe under redelivery.

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

Focused worker tests also cover current and legacy duplicate events, concurrent
handlers, partial subscriber/channel failure, and provider success followed by
completion-store failure. Both managed notification routes are authenticated
private Cloud Run push endpoints with exact subscription binding.

---

## 3. Google Cloud Platform (GCP) Deployment

### OpenTofu / Terraform Infrastructure Setup

Infrastructure is defined as code under `infra/opentofu`:

- GCS Bucket for image storage (`modules/storage`)
- Cloud Pub/Sub topics, four authenticated matcher and notification push
  subscriptions for `lostPet`, `foundPet`, and `matchFound`, retry and
  dead-letter policies, and a dedicated invocation identity for each consumer
  (`modules/pubsub`)
- Cloud Firestore database plus pending-outbox and matcher-candidate composite
  indexes
  (`modules/firestore`)
- Cloud Run v2 services, including private matcher and notification ingress,
  push identity configuration, and always-CPU lost- and found-report relay
  instances (`modules/cloudrun`)

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
and notification subscriptions and their least-privilege IAM grants. The
standalone manifests contain example project-qualified identity values and do
not create Pub/Sub or IAM resources; replace those placeholders if applying a
manifest directly.

To deploy a service:

```bash
gcloud run services replace deploy/cloudrun/lostpet-service.yaml \
  --region=us-central1
```
