# Operational Runbook: Cloud Firestore Point-In-Time Recovery (PITR) & Disaster Recovery

This runbook documents the disaster recovery procedure for restoring PetSpotR's Cloud Firestore Native database using Point-In-Time Recovery (PITR), verifying data integrity and composite index alignment, and conducting annual disaster recovery drills.

---

## 1. Cloud Firestore PITR Overview

PetSpotR utilizes Google Cloud Firestore Native mode with Point-In-Time Recovery (PITR) enabled:
- **Recovery Window**: Continuous continuous backup retained for **7 rolling days** (168 hours).
- **Granularity**: Restoration to any exact microsecond within the 7-day window.
- **Restore Destination**: Restoring PITR creates a **new Firestore database** in the same GCP project or a designated DR project. It cannot overwrite an existing live database in place.

---

## 2. Emergency Restore Procedure

Follow these steps when severe data corruption, unauthorized bulk deletion, or catastrophic administrative error requires point-in-time recovery.

### Step 1: Halt Incoming Write Traffic
Before initiating restoration, stop writer traffic to prevent split-brain data states:

```bash
# Update Cloud Run services to a maintenance container or temporarily pause traffic
gcloud run services update-traffic web-frontend --to-tags=maintenance=100 --region=us-central1
gcloud run services update-traffic lostpet-service --to-tags=maintenance=100 --region=us-central1
gcloud run services update-traffic foundpet-service --to-tags=maintenance=100 --region=us-central1
```

---

### Step 2: Identify Target Recovery Timestamp
Determine the exact UTC timestamp $T_{\text{restore}}$ immediately prior to the corrupting event. Timestamp must be formatted in ISO 8601 (e.g., `2026-09-12T20:45:00Z`).

Validate the timestamp falls within the database PITR earliest restoration time:
```bash
gcloud firestore databases describe --database="(default)" \
  --format="value(earliestVersionTime)"
```

---

### Step 3: Execute Point-in-Time Restore Command

Invoke the `gcloud firestore databases restore` command to restore from source `(default)` into a new destination database (e.g., `petspotr-restored-YYYYMMDD`):

```bash
export PROJECT_ID=$(gcloud config get-value project)
export RESTORE_TIMESTAMP="2026-09-12T20:45:00Z"
export DESTINATION_DB="petspotr-restored-$(date +%Y%m%d%H%M)"

gcloud firestore databases restore \
  --source-database="projects/${PROJECT_ID}/databases/(default)" \
  --destination-database="projects/${PROJECT_ID}/databases/${DESTINATION_DB}" \
  --restore-time="${RESTORE_TIMESTAMP}"
```

#### Monitor the Long-Running Restore Operation
The restore command returns an asynchronous operation identifier. Track its progress:
```bash
gcloud firestore operations list \
  --database="${DESTINATION_DB}" \
  --filter="metadata.@type:RestoreDatabaseMetadata"

# Describe specific operation
gcloud firestore operations describe <OPERATION_NAME>
```
Wait until `done: true` is reported.

---

## 3. Verifying State Integrity and Composite Indexes

A restored Firestore database initially contains collection data, but composite indexes must be verified and built to support PetSpotR queries.

### Step 1: Verify and Provision Composite Indexes

List composite indexes on the newly restored database:
```bash
gcloud firestore indexes composite list --database="${DESTINATION_DB}"
```

Ensure all composite indexes defined in [`infra/opentofu/modules/firestore/main.tf`](file:///infra/opentofu/modules/firestore/main.tf) exist:
1. **`eventOutbox` Index**:
   - Fields: `topic` (ASC), `status` (ASC), `createdAt` (ASC), `key` (ASC)
2. **`lostPets` Geospatial & Status Candidates**:
   - Fields: `lostStatus` (ASC), `lostGeocodingStatus` (ASC), `lostReportedAt` (ASC), `lostLatitude` (ASC), `lostLongitude` (ASC), `key` (ASC)
3. **`lostPets` Species-Filtered Candidates**:
   - Fields: `lostStatus` (ASC), `lostGeocodingStatus` (ASC), `lostSpecies` (ASC), `lostReportedAt` (ASC), `lostLatitude` (ASC), `lostLongitude` (ASC), `key` (ASC)

If any indexes are missing, deploy them via OpenTofu targeting the restored database or create them via CLI:
```bash
# Example CLI index creation if needed:
gcloud firestore indexes composite create \
  --database="${DESTINATION_DB}" \
  --collection-group=eventOutbox \
  --field-config field-path=topic,order=ascending \
  --field-config field-path=status,order=ascending \
  --field-config field-path=createdAt,order=ascending \
  --field-config field-path=key,order=ascending
```

---

### Step 2: Validate Data Integrity

Run sample validation checks against the restored database:
- **Pet Reports Count**: Verify total record count matches pre-incident baseline.
- **Outbox Reconciliation**: Inspect pending outbox records to ensure no stale or phantom events:
  ```bash
  # Check for uncommitted or pending records
  curl -s "https://firestore.googleapis.com/v1/projects/${PROJECT_ID}/databases/${DESTINATION_DB}/documents/eventOutbox?pageSize=10" \
    -H "Authorization: Bearer $(gcloud auth print-access-token)"
  ```
- **Audit Trails**: Check role assignment records and ensure `revision` sequence integrity.

---

## 4. Switching Service Traffic to the Restored Database

Once verification is confirmed, re-point Cloud Run microservices to the new restored database:

```bash
# Update Cloud Run service environment variables
for SERVICE in web-frontend lostpet-service foundpet-service pet-matcher notification-service; do
  gcloud run services update ${SERVICE} \
    --region=us-central1 \
    --set-env-vars="FIRESTORE_DATABASE=${DESTINATION_DB}"
done
```

After deployment:
1. Verify `/readyz` probes return HTTP 200 on all services.
2. Route 100% of production traffic back to active revisions.
3. Archive or delete the corrupted original database after a 30-day quarantine period.

---

## 5. Annual Disaster Recovery Drill Protocol

To validate operational readiness and ensure Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO) are met, PetSpotR conducts a mandatory **Annual Disaster Recovery Drill**.

### Target Metrics
* **RTO (Recovery Time Objective)**: $\le 120\text{ minutes}$ (2 hours) from disaster declaration to full service traffic restoration.
* **RPO (Recovery Point Objective)**: $\le 5\text{ minutes}$ of data loss.

### Drill Execution Protocol

1. **Pre-Drill Preparation (T - 1 Week)**:
   - Announce drill window to engineering team.
   - Designate the Drill Commander and Observability Observer.
   - Seed test data in staging/demo environment.
2. **Drill Day Simulation (T = 0)**:
   - Inject simulated incident: trigger accidental deletion or schema corruption on the test Firestore database.
   - Declare simulated P1 incident in `#incident-dr-drill`.
   - Execute PITR restore to target timestamp $T - 10\text{m}$.
   - Build composite indexes and verify data consistency.
   - Reconfigure staging microservices to use the restored database.
3. **Drill Verification & Sign-Off**:
   - Run automated end-to-end integration test suite (`e2e/firestore_process_test.go`).
   - Validate that Playwright UI tests pass against the restored environment.
   - Document total elapsed time (RTO actual) and data delta (RPO actual).
4. **Post-Drill Reporting**:
   - Submit Disaster Recovery Drill Report to engineering leadership.
   - Log any latency bottlenecks or tooling issues as P2 tracking tasks.
