# PetSpotR Service Level Objectives (SLOs), Alert Policies & Observability Specification

This specification establishes production Service Level Indicators (SLIs), Service Level Objectives (SLOs), Error Budget governance, alert tier classifications, and monitoring architecture for the PetSpotR microservices platform running on Google Cloud Platform.

---

## 1. Service Level Indicators (SLIs) & Objectives (SLOs)

PetSpotR measures reliability, latency, and end-to-end event freshness against four core user-facing SLOs calculated over a **rolling 30-day window**.

### Summary Matrix

| Metric | SLI (Measurement Specification) | Target (SLO) | Window |
| :--- | :--- | :--- | :--- |
| **Public API Availability** | Proportion of successful HTTP requests (`2xx`, `3xx`, `4xx` excluding `5xx`) served by the `web-frontend` and ingress services | **&ge; 99.9%** | Rolling 30 Days |
| **Public API Latency** | Proportion of HTTP transactions served in under 500 ms (excluding asynchronous AI vision inference endpoints) | **&ge; 95.0%** | Rolling 30 Days |
| **AI Vision Inference Latency** | Proportion of Gemma 4 multimodal feature extraction tasks completed in under 5.0 seconds | **&ge; 95.0%** | Rolling 30 Days |
| **Report-to-Notification Freshness** | Proportion of candidate matches where notifications are delivered to owners/finders within 60 seconds of report creation | **&ge; 95.0%** | Rolling 30 Days |

---

### Detailed SLI/SLO Definitions

#### 1. Public API Availability
* **Definition**: Measures whether public end-users and client integrations can reliably interact with the PetSpotR portal.
* **Good Events**: HTTP responses with status code `< 500` (includes client errors `4xx` as valid service responses, e.g., validation failures).
* **Total Events**: All HTTP requests received by Cloud Run services (`web-frontend`, `lostpet-service`, `foundpet-service`).
* **Formula**:
  $$\text{SLI}_{\text{Availability}} = \frac{\sum \text{Requests with status } < 500}{\sum \text{Total HTTP Requests}} \ge 99.9\%$$
* **Error Budget**: $0.1\%$ allowable failure rate over 30 days (approx. $43.2$ minutes of total downtime per month).

#### 2. Public API Latency
* **Definition**: Ensures real-time web application responsiveness for browse, view, and report-submission user interactions.
* **Good Events**: HTTP requests served with latency $T_{\text{response}} < 500\text{ ms}$.
* **Total Events**: Total HTTP requests excluding requests to `/api/feature-extract` (which trigger heavyweight multimodal vision models).
* **Formula**:
  $$\text{SLI}_{\text{Latency}} = \frac{\sum \text{Eligible Requests with } T_{\text{latency}} < 500\text{ms}}{\sum \text{Total Eligible Requests}} \ge 95.0\%$$
* **Error Budget**: $5.0\%$ of requests may exceed 500 ms without breaching policy.

#### 3. AI Vision Inference Latency
* **Definition**: Measures inference execution speed for Gemma 4 visual trait extraction (species, breed, primary color, distinctive markings).
* **Good Events**: Image analysis requests completing feature extraction in $T_{\text{inference}} < 5000\text{ ms}$ (5.0s).
* **Total Events**: Total feature extraction inference calls processed by `pet-matcher` or the image inference pipeline.
* **Formula**:
  $$\text{SLI}_{\text{Inference}} = \frac{\sum \text{Inference tasks with } T_{\text{infer}} < 5\text{s}}{\sum \text{Total Inference tasks}} \ge 95.0\%$$
* **Error Budget**: $5.0\%$ of inference executions may take $\ge 5$ seconds.

#### 4. Report-to-Notification Freshness
* **Definition**: Measures end-to-end system freshness from the instant a lost or found pet report is registered to the moment notification delivery is dispatched for candidate matches.
* **Good Events**: High-confidence match pairs ($S \ge 0.70$) where notification dispatch timestamp $T_{\text{notified}} - T_{\text{reported}} \le 60\text{ seconds}$.
* **Total Events**: Total eligible candidate match events processed.
* **Formula**:
  $$\text{SLI}_{\text{Freshness}} = \frac{\sum \text{Matches notified in } \le 60\text{s}}{\sum \text{Total Candidate Matches}} \ge 95.0\%$$
* **Error Budget**: $5.0\%$ of candidate matches taking $> 60$ seconds.

---

## 2. Error Budget Policy & Release Governance

The monthly error budget represents the difference between $100\%$ availability/timeliness and the agreed SLO (e.g., $0.1\%$ for availability).

### Error Budget Depletion Rules

```
Remaining Error Budget (Monthly)
  ├── 100% - 50%: Normal Operations. Feature deployment continues with standard CI/CD gates.
  ├── 50% - 20%: Warning State. Tech leads review stability and flake rates in retrospectives.
  ├── 20% - 10%: Critical State. Heightened scrutiny; non-urgent deployments require peer SRE sign-off.
  └── < 10%: RELEASE FREEZE TRIGGERED.
```

### Policy Enforcement: Non-Critical Release Freeze
1. **Trigger**: If the rolling monthly error budget for any Tier-1 SLO drops below **10%**, an immediate automated freeze on non-critical releases is enacted.
2. **Impact**:
   - Continuous deployment pipelines automatically block regular feature pull requests.
   - All engineering capacity pivots exclusively to reliability engineering, root cause remediation, bug fixes, automated test stabilization, and performance tuning.
3. **Exceptions (P0 Security & Critical Fixes)**:
   - Only emergency patches addressing security vulnerabilities (CVEs) or fixes directly targeting the active SLO degradation are permitted.
   - Requires joint approval from the **Incident Commander** and **Engineering Tech Lead**.
4. **Unfreezing Criteria**:
   - The remaining error budget trend must remain positive for **7 consecutive days**.
   - Completed Root Cause Analysis (RCA) and verified post-incident action items are deployed to production.

---

## 3. Production Alert Policies

PetSpotR alerts are organized into three actionable tiers (P1, P2, P3) based on urgency, customer blast radius, and required response times. Every alert links directly to a documented operational runbook.

### Alert Severity Matrix

| Tier | Urgency | Notification Channels | Response SLA | Runbook |
| :--- | :--- | :--- | :--- | :--- |
| **P1** | Critical (Immediate) | PagerDuty (Voice/SMS), Push, Escalation Tree | &le; 5 minutes | [Incident Response](file:///docs/runbooks/incident-response.md) |
| **P2** | High (Escalated) | PagerDuty, Slack (`#alerts-high`), Email | &le; 30 minutes | [Incident Response](file:///docs/runbooks/incident-response.md) |
| **P3** | Medium (Business Hours) | Slack (`#alerts-ops`), Jira/GitHub Issues | Next Business Day | [Incident Response](file:///docs/runbooks/incident-response.md) |

---

### Tier 1: P1 Critical Alerts (Immediate 24/7 Paging)

P1 alerts indicate catastrophic user-facing degradation, potential data loss, or core event pipeline blockage. The on-call engineer is paged immediately.

#### 1. Cloud Run 5xx Error Rate Spike
* **Condition**: Total HTTP 5xx responses across any Cloud Run service (`web-frontend`, `lostpet-service`, `foundpet-service`, `pet-matcher`, `notification-service`) exceed **2%** of total traffic for **3 consecutive minutes**.
* **Metric Filter**:
  ```
  resource.type = "cloud_run_revision"
  metric.type = "run.googleapis.com/request_count"
  metric.labels.response_code_class = "5xx"
  ```
* **Impact**: Users experiencing service errors, inability to submit reports or search lost pets.
* **Runbook**: [Cloud Run Rollback Runbook](file:///docs/runbooks/rollback.md)

#### 2. Dead-Letter Queue (DLQ) Message Count Spike
* **Condition**: Message count **> 0** on any canonical dead-letter queue topic:
  - `lostPet-dlq`
  - `foundPet-dlq`
  - `matchFound-dlq`
  - `petStatusChanged-dlq`
* **Metric Filter**:
  ```
  resource.type = "pubsub_topic"
  metric.type = "pubsub.googleapis.com/topic/send_message_operation_count"
  metric.labels.topic_id = one_of("lostPet-dlq", "foundPet-dlq", "matchFound-dlq", "petStatusChanged-dlq")
  ```
* **Impact**: Unprocessable poison-pill events or persistent downstream failures causing lost events.
* **Runbook**: [DLQ Drain Runbook](file:///docs/runbooks/dlq-drain.md)

#### 3. Container Crash-Looping or Startup Probe Failure
* **Condition**: Cloud Run container instances restarting repeatedly or failing startup/readiness health probes (`/healthz` or `/readyz`) resulting in zero available healthy instances.
* **Metric Filter**:
  ```
  resource.type = "cloud_run_revision"
  metric.type = "run.googleapis.com/container/instance_latencies"
  metric.labels.response_code = "503"
  ```
* **Impact**: Service unavailability, traffic dropping, incoming connections rejected.
* **Runbook**: [Cloud Run Rollback Runbook](file:///docs/runbooks/rollback.md) & [Incident Response Runbook](file:///docs/runbooks/incident-response.md)

#### 4. Pub/Sub Oldest Unacked Message Age Exceeded
* **Condition**: Oldest unacknowledged message age in any subscription exceeds **5 minutes** (300 seconds).
* **Metric Filter**:
  ```
  resource.type = "pubsub_subscription"
  metric.type = "pubsub.googleapis.com/subscription/oldest_unacked_message_age"
  value > 300s
  ```
* **Impact**: Pipeline stalls; pet match computations and critical owner notifications are severely delayed.
* **Runbook**: [Incident Response Runbook](file:///docs/runbooks/incident-response.md)

---

### Tier 2: P2 High Alerts (30-Minute Escalation)

P2 alerts indicate substantial latency degradation or non-catastrophic subsystem failures. If unacknowledged within 30 minutes, they escalate to P1 status.

#### 1. Public API p95 Latency Degradation
* **Condition**: 95th percentile HTTP response latency exceeds **1000 ms** for **10 consecutive minutes** (excluding `/api/feature-extract`).
* **Metric Filter**:
  ```
  resource.type = "cloud_run_revision"
  metric.type = "run.googleapis.com/request_latencies"
  percentile = 95
  value > 1000ms
  ```
* **Impact**: Extreme UI sluggishness and degraded customer experience.
* **Runbook**: [Cloud Run Rollback Runbook](file:///docs/runbooks/rollback.md)

#### 2. AI Vision Inference Failure Rate
* **Condition**: Gemma 4 vision model feature extraction failure rate exceeds **10%** over a 15-minute evaluation window.
* **Metric Filter**:
  ```
  custom.googleapis.com/petspotr/inference_errors / custom.googleapis.com/petspotr/inference_total > 0.10
  ```
* **Impact**: Automated image trait matching fails; system falls back to basic geospatial/text-only search.
* **Runbook**: [Incident Response Runbook](file:///docs/runbooks/incident-response.md)

#### 3. Firestore Read/Write Error Rate
* **Condition**: Cloud Firestore API operations encounter an error rate **> 1%** over a 5-minute rolling window.
* **Metric Filter**:
  ```
  resource.type = "firestore_instance"
  metric.type = "firestore.googleapis.com/document/read_count"
  metric.labels.status != "OK"
  ```
* **Impact**: Flaky report persistence, outbox commit failures, identity session drops.
* **Runbook**: [Firestore Restore Runbook](file:///docs/runbooks/firestore-restore.md)

#### 4. Outbox Relay Backlog Accumulation
* **Condition**: Pending outbox records backlog in collection `eventOutbox` exceeds **50 records** with status `pending` older than 2 minutes.
* **Metric Filter**:
  ```
  custom.googleapis.com/petspotr/outbox_pending_count > 50
  ```
* **Impact**: Asynchronous decoupling pipeline lagging behind transactional updates.
* **Runbook**: [Incident Response Runbook](file:///docs/runbooks/incident-response.md)

---

### Tier 3: P3 Medium Alerts (Business Hours Triage)

P3 alerts represent non-urgent capacity shifts, storage lifecycle anomalies, or gradual drifts requiring investigation during regular working hours.

#### 1. Cloud Storage Growth Rate Anomaly
* **Condition**: Storage consumption in the pet photo bucket (`petspotr-*-images`) grows by **> 20% within 24 hours** without corresponding increase in report submissions.
* **Metric Filter**:
  ```
  resource.type = "gcs_bucket"
  metric.type = "storage.googleapis.com/storage/total_bytes"
  rate_of_change(24h) > 0.20
  ```
* **Impact**: Potential resource leak, image upload abuse, or missing client thumbnail compression.
* **Runbook**: [Incident Response Runbook](file:///docs/runbooks/incident-response.md)

#### 2. Noncurrent Object Version Count Spike
* **Condition**: GCS bucket noncurrent object count increases by **> 5,000 objects** in a single day, indicating lifecycle policy delays or excessive image overwrite cycles.
* **Metric Filter**:
  ```
  resource.type = "gcs_bucket"
  metric.type = "storage.googleapis.com/storage/object_count"
  metric.labels.storage_class = "NONCURRENT"
  ```
* **Impact**: Cloud storage bill inflation, retention policy non-compliance.
* **Runbook**: [Incident Response Runbook](file:///docs/runbooks/incident-response.md)

---

## 4. Monitoring Dashboards & Telemetry Architecture

Cloud Monitoring dashboards in GCP provide real-time visualization of these SLOs:

1. **Executive SLO & Error Budget Dashboard**:
   - 30-day rolling burn rate for Availability (99.9%) and Latency (95% < 500ms).
   - Report-to-Notification end-to-end freshness heatmap.
2. **Service Operations & Container Health**:
   - Container CPU/Memory utilization, active instances, and concurrency across services.
   - Liveness (`/healthz`) and readiness (`/readyz`) probe pass/fail history.
3. **Event Stream & Pub/Sub Pipeline**:
   - Ingress message volume, subscription acknowledgment latency, and oldest unacked message age.
   - Dead-letter queue ingress counters for `lostPet-dlq`, `foundPet-dlq`, `matchFound-dlq`, `petStatusChanged-dlq`.
4. **Data Layer & Storage**:
   - Firestore transaction duration, conflict rate, and pending outbox queue depth.
   - GCS bucket size, object count, and noncurrent version lifecycle metrics.
