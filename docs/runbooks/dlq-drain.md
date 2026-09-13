# Operational Runbook: Dead-Letter Queue (DLQ) Drain and Triage

This runbook guides operators through inspecting poison-pill messages trapped in PetSpotR Dead-Letter Queues (DLQs), analyzing root-cause failures encoded in `DeadLetterEnvelope` payloads, and safely replaying or purging quarantined messages.

---

## 1. Dead-Letter Queues Overview

PetSpotR routes failed messages to dedicated dead-letter topics when retry policies or downstream workers fail repeatedly (after 10 delivery attempts or explicit poison-pill encapsulation in `pkg/pubsub/dlq.go`):

| Canonical DLQ Topic | Source Topic | Primary Consumer | DLQ Retention Subscription |
| :--- | :--- | :--- | :--- |
| `lostPet-dlq` | `lostPet` | `pet-matcher`, `notification-service` | `lost-pet-dead-letter-retention` |
| `foundPet-dlq` | `foundPet` | `pet-matcher` | `found-pet-dead-letter-retention` |
| `matchFound-dlq` | `matchFound` | `notification-service` | `match-found-dead-letter-retention` |
| `petStatusChanged-dlq` | `petStatusChanged` | `web-frontend`, reporting | `pet-status-changed-backlog` |

Messages in DLQs are wrapped in a structured `DeadLetterEnvelope` struct containing failure diagnostic metadata.

---

## 2. Step-by-Step Inspection Procedure

### Step 1: Pull Messages Without Acknowledging
To inspect quarantined messages without deleting them from the retention subscription, pull messages using `--auto-ack=false`:

```bash
gcloud pubsub subscriptions pull <RETENTION_SUBSCRIPTION> \
  --limit=5 \
  --auto-ack=false \
  --format="json" > /tmp/dlq_sample.json
```

Example for `foundPet-dlq`:
```bash
gcloud pubsub subscriptions pull found-pet-dead-letter-retention \
  --limit=5 \
  --auto-ack=false \
  --format="json" > /tmp/foundpet_dlq.json
```

---

### Step 2: Extract and Analyze the `DeadLetterEnvelope`

As defined in [`pkg/pubsub/dlq.go`](file:///pkg/pubsub/dlq.go), the message body contains a JSON-serialized `DeadLetterEnvelope`:

```json
{
  "originalPayload": "eyJwZXRJZCI6IjEyMyIsInNwZWNpZXMiOiJEb2cifQ==",
  "originalTopic": "foundPet",
  "failedAt": "2026-09-12T20:15:30.123456Z",
  "attempts": 10,
  "lastError": "vision: inference endpoint timeout after 10000ms",
  "messageId": "msg-89723412"
}
```

#### Diagnostic Breakdown:
* **`originalTopic`**: The originating topic (`lostPet`, `foundPet`, `matchFound`, `petStatusChanged`).
* **`lastError`**: The exact Go error string returned by the subscriber handler before quarantine.
* **`attempts`**: Total delivery attempts prior to DLQ routing.
* **`failedAt`**: UTC timestamp when the poison pill was quarantined.
* **`originalPayload`**: Base64-encoded raw bytes of the original event before envelope encapsulation.

#### Decode Original Event Payload
To view the underlying domain event JSON:
```bash
jq -r '.[0].message.data' /tmp/foundpet_dlq.json | base64 -d | jq -r '.originalPayload' | base64 -d | jq .
```

---

## 3. Failure Classification

Determine whether the failure is **transient** (safe to replay) or a **permanent poison pill** (requires purge or code fix):

1. **Transient / Upstream Dependency Failures** (Candidate for Replay):
   - `lastError` contains network timeouts, Firestore `Unavailable`, rate limiting, or temporary Vision AI downtime.
   - Once the dependent service is healthy, these messages can be safely replayed.
2. **Permanent Schema / Validation Failures** (Poison Pill):
   - `lastError` contains JSON unmarshal syntax error, missing required domain fields (`petId`, `ownerEmail`), or data integrity violations.
   - Replaying without a code patch will simply re-pollute the pipeline and trigger another P1 alert.
3. **Idempotency & Stale Messages**:
   - Check `failedAt`. If the message is several days old, verify whether the pet report was already resolved manually before replaying.

---

## 4. Replay and Purge Procedures

### Option A: Safely Replaying Messages

Once root-cause fixes or dependency recovery is confirmed, replay messages back to their `originalTopic`.

#### 1. Archive Messages Locally Before Replay
```bash
mkdir -p /var/log/petspotr/dlq-backup
cp /tmp/dlq_sample.json /var/log/petspotr/dlq-backup/dlq-$(date +%Y%m%d%H%M%S).json
```

#### 2. Replay Extracted Payloads
Using a bash/python extraction script, read each message, unwrap `originalPayload`, and republish to `originalTopic`:

```bash
python3 - << 'PY_EOF'
import json, base64, subprocess

with open("/tmp/dlq_sample.json") as f:
    messages = json.load(f)

for item in messages:
    msg_data = base64.b64decode(item["message"]["data"]).decode("utf-8")
    envelope = json.loads(msg_data)
    
    orig_topic = envelope["originalTopic"]
    orig_payload = base64.b64decode(envelope["originalPayload"])
    
    print(f"Replaying message {envelope.get('messageId')} to topic {orig_topic}...")
    
    cmd = [
        "gcloud", "pubsub", "topics", "publish", orig_topic,
        f"--message={orig_payload.decode('utf-8', errors='ignore')}"
    ]
    subprocess.run(cmd, check=True)
PY_EOF
```

#### 3. Acknowledge and Clear From DLQ Subscription
After successful republishing, drain the processed messages from the DLQ subscription using the ack IDs:
```bash
gcloud pubsub subscriptions ack <RETENTION_SUBSCRIPTION> \
  --ack-ids=<ACK_ID_1>,<ACK_ID_2>
```

---

### Option B: Purging Unprocessable Messages

If messages represent corrupted, abusive, or obsolete data that should not be re-published:

#### 1. Persist to Cloud Storage Long-Term Archive
Always archive discarded poison pills to GCS for compliance and audit logs:
```bash
gsutil cp /tmp/dlq_sample.json "gs://petspotr-dlq-archive/$(date +%Y/%m/%d)/purged-dlq-$(date +%s).json"
```

#### 2. Seek Subscription to Head or Purge Messages
To acknowledge all messages without replaying:
```bash
# Purge all pending messages up to the current timestamp
gcloud pubsub subscriptions seek <RETENTION_SUBSCRIPTION> --time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
```

---

## 5. Verification & Health Confirmation

1. Verify the DLQ subscription backlog is zero:
   ```bash
   gcloud pubsub subscriptions describe <RETENTION_SUBSCRIPTION> \
     --format="value(numUndeliveredMessages)"
   ```
2. Verify Cloud Monitoring:
   - Alert `Dead-Letter Queue message count` transitions from Firing to OK.
   - Consumer services (`pet-matcher`, `notification-service`) process replayed events without error.
