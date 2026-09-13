# Operational Runbook: Cloud Run Fast Rollback

This runbook describes the procedure to instantly shift traffic back to a known-healthy Cloud Run revision during an active outage, along with verification steps and critical database/outbox backward compatibility safeguards.

---

## 1. When to Trigger a Rollback

Initiate a Cloud Run traffic rollback immediately if any of the following conditions occur following a deployment:
- **P1 Alert fired**: Cloud Run 5xx rate > 2% for 3 consecutive minutes.
- **P1 Alert fired**: Container instances fail startup or liveness probes (`/healthz`).
- **P2 Alert fired**: 95th percentile latency spikes > 1000 ms.
- **Regression detected**: Critical user workflows (report filing, photo upload, search) are broken in production.

> [!IMPORTANT]
> In production outages, **mitigate first by rolling back** before attempting in-place debugging or hotfixing.

---

## 2. Fast Rollback Command

Cloud Run maintains immutable revision histories. Rolling back is instantaneous because previous container images remain provisioned and warm in Google Cloud Run.

### Step 1: Identify the Known-Healthy Revision

List recent revisions for the impacted service to identify the previous stable revision name:

```bash
gcloud run revisions list \
  --service=<SERVICE_NAME> \
  --region=<REGION> \
  --project=<PROJECT_ID> \
  --sort-by="~metadata.creationTimestamp" \
  --format="table(name, active, traffic_percent, creation_timestamp)"
```

Example Output:
```
NAME                          ACTIVE  TRAFFIC_PERCENT  CREATION_TIMESTAMP
web-frontend-00045-abc        yes     100              2026-09-12T20:45:00Z  <-- FAULTY CURRENT
web-frontend-00044-xyz        yes     0                2026-09-12T18:30:00Z  <-- PREVIOUS HEALTHY
```

---

### Step 2: Instant 100% Traffic Reversion

Shift 100% of user traffic to the target previous healthy revision using the canonical fast rollback command:

```bash
gcloud run services update-traffic <SERVICE> \
  --to-revisions=<PREVIOUS_REVISION>=100 \
  --region=<REGION> \
  --project=<PROJECT_ID>
```

#### Service-Specific Invocations

* **Web Frontend**:
  ```bash
  gcloud run services update-traffic web-frontend \
    --to-revisions=web-frontend-00044-xyz=100 \
    --region=us-central1
  ```
* **Lost Pet Service**:
  ```bash
  gcloud run services update-traffic lostpet-service \
    --to-revisions=lostpet-service-00032-xyz=100 \
    --region=us-central1
  ```
* **Found Pet Service**:
  ```bash
  gcloud run services update-traffic foundpet-service \
    --to-revisions=foundpet-service-00030-xyz=100 \
    --region=us-central1
  ```
* **Pet Matcher Service**:
  ```bash
  gcloud run services update-traffic pet-matcher \
    --to-revisions=pet-matcher-00028-xyz=100 \
    --region=us-central1
  ```
* **Notification Service**:
  ```bash
  gcloud run services update-traffic notification-service \
    --to-revisions=notification-service-00019-xyz=100 \
    --region=us-central1
  ```

---

## 3. Safe Rollback Verification Steps

After executing the traffic shift, complete these verification steps:

### 1. Confirm Active Traffic Split
Ensure 100% of traffic is allocated to the previous revision:
```bash
gcloud run services describe <SERVICE> \
  --region=<REGION> \
  --format="value(status.traffic)"
```
Verify the output shows `percent: 100` for `<PREVIOUS_REVISION>`.

### 2. Verify Health and Readiness Probes
Directly curl the service endpoint (or custom domain) `/healthz` and `/readyz` probes:
```bash
SERVICE_URL=$(gcloud run services describe <SERVICE> --region=<REGION> --format="value(status.url)")

# Liveness probe (HTTP 200 {"status":"ok"})
curl -sf -o /dev/null -w "%{http_code}\n" "${SERVICE_URL}/healthz"

# Readiness probe (HTTP 200 {"status":"ready"})
curl -sf -o /dev/null -w "%{http_code}\n" "${SERVICE_URL}/readyz"
```

### 3. Monitor Telemetry and Error Rates
Monitor Google Cloud Monitoring for **15 consecutive minutes**:
- Cloud Run HTTP 5xx error rate drops to $< 0.1\%$.
- Request latency returns to normal baseline ($p95 < 500\text{ ms}$).
- Pub/Sub subscription ack rate recovers and DLQ message count remains at $0$.

---

## 4. Database Schema & Outbox Compatibility Rules

A rollback can be catastrophic if the preceding release made backward-incompatible changes to Firestore documents or outbox event schemas. PetSpotR mandates strict compatibility protocols:

### Rule 1: Strict Two-Phase Expand/Contract Schema Changes
- **Phase 1 (Expand)**: Additive changes only. New code can read and write new fields, but must continue to accept old field formats with sensible defaults.
- **Phase 2 (Contract)**: Old code can be safely rolled back because the database still contains compatible representations. Unused legacy fields are only removed in a subsequent release after verification.
- **NEVER** rename or delete database fields in the same release that introduces their replacements.

### Rule 2: Outbox Record Envelope Immutability
Outbox records stored in Firestore collection `eventOutbox` (as defined in `pkg/outbox/relay.go`) adhere to the standard schema:
```json
{
  "id": "evt-uuid",
  "topic": "lostPet",
  "payload": "...base64 or json...",
  "status": "pending",
  "attempts": 1,
  "createdAt": "2026-09-12T20:00:00Z"
}
```
- When rolling back `lostpet-service`, `foundpet-service`, or `web-frontend`, previous revisions must be able to read and publish existing pending records.
- If the failed revision wrote events with an updated payload schema, verify whether downstream subscribers (`pet-matcher`, `notification-service`) or the rolled-back publisher can parse them without panicking.
- If payload schema changes are backward-incompatible, isolate unprocessable messages via the [DLQ Drain Runbook](file:///docs/runbooks/dlq-drain.md).

### Rule 3: Active Outbox Leases
Outbox relay workers use distributed leases (`leaseUntil`) to prevent duplicate delivery.
- If a rollback occurs while outbox records have active leases held by the faulty revision, the lease will naturally expire after `defaultPublishLease` (10 minutes).
- To accelerate outbox recovery without waiting for lease timeouts, trigger the outbox recovery loop via `internal/app/outboxrecovery` or invoke the admin flush endpoint.
