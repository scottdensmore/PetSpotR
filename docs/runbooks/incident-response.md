# Operational Runbook: Incident Response Protocol

This runbook defines the end-to-end incident management framework for PetSpotR production services, covering severity classifications, Incident Commander (IC) operational duties, stakeholder communication templates, and post-incident review checklists.

---

## 1. Incident Severity Classifications

Every production anomaly or alert in PetSpotR is triaged into one of four severity levels:

| Severity | Definition & Impact | Response SLA | Target Update Cadence | Primary Channels |
| :--- | :--- | :--- | :--- | :--- |
| **P1 - Critical** | Catastrophic outage, critical data loss risk, complete failure of core services (`web-frontend`, report submission, or matching pipeline), DLQ message accumulation, or Cloud Run 5xx rate > 2%. | **&le; 5 minutes** | Every **15 minutes** | PagerDuty (Voice/SMS), War Room (Google Meet), `#incident-p1` Slack |
| **P2 - High** | Major performance degradation (API p95 > 1s), AI inference failure rate > 10%, Firestore error rate > 1%, outbox backlog > 50 records, or partial loss of non-critical functionality. | **&le; 30 minutes** | Every **30 minutes** | PagerDuty, `#incident-p2` Slack, Email notification to engineering leads |
| **P3 - Medium** | Minor degradation, storage growth anomalies, noncurrent object version spikes, or internal admin tool impairment. Core user journeys remain functional. | **Next business day** (or &le; 4 hrs during business hours) | Daily / On resolution | `#ops-alerts` Slack, GitHub issue tracking |
| **P4 - Low** | Cosmetic issues, low-priority metric drift, non-impacting informational alerts, or questions requiring investigation without user impact. | **Planned Sprint** | As status changes | GitHub issue backlog |

---

## 2. Roles & Responsibilities

During a P1 or P2 incident, clear separation of operational roles prevents duplicate effort and ensures transparent communication:

### Incident Commander (IC)
* **Designation**: The on-call engineer who acknowledges the initial page or a designated senior lead who assumes the role.
* **Responsibilities**:
  - Declares the incident and assigns severity.
  - Establishes the incident bridge (War Room Meet link and dedicated Slack channel).
  - Directs triage, assigns investigation threads to responders, and authorizes mitigations (e.g., triggering rollbacks or database restores).
  - Owns external and internal status communications.
  - **Does NOT** perform low-level debugging while acting as IC to preserve global situational awareness.

### Operations / SRE Lead
* **Responsibilities**:
  - Executes specific operational runbooks (e.g., [Cloud Run Rollback](file:///docs/runbooks/rollback.md), [DLQ Drain](file:///docs/runbooks/dlq-drain.md), or [Firestore Restore](file:///docs/runbooks/firestore-restore.md)).
  - Inspects Google Cloud Monitoring, Cloud Trace, and structured JSON logs.
  - Formulates tactical remediation options and presents them to the IC.

### Communications Lead (for extended P1 incidents)
* **Responsibilities**:
  - Maintains public status page updates.
  - Broadcasts executive status updates at designated intervals.

---

## 3. Incident Lifecycle Workflow

```
[Alert Fired / Page Triggered]
             │
             ▼
      1. TRIAGE & DECLARE
         - Acknowledge within SLA (5m for P1, 30m for P2)
         - Assume IC role and create `#incident-YYYYMMDD-<name>` channel
         - Open Google Meet Incident War Room
             │
             ▼
      2. COMMUNICATE (Initial)
         - Send Initial Notification within 15 minutes
             │
             ▼
      3. MITIGATE & STABILIZE
         - Focus on restoring service, NOT finding permanent fix:
           * If recent deployment: Execute Cloud Run Rollback
           * If poison-pill: Inspect & isolate via DLQ Drain
           * If state corruption: Prepare Firestore PITR Restore
             │
             ▼
      4. VERIFY RESTORATION
         - Confirm metrics return to baseline (/healthz, error rates < 0.1%)
         - Monitor for 15 minutes
             │
             ▼
      5. RESOLVE & CLOSE
         - Broadcast Resolution Communication
         - Schedule Post-Incident Review (PIR) within 3 business days
```

---

## 4. Communication Templates

### A. Initial Broadcast Template (Sent within 15 mins of P1 declaration)

```markdown
**[INCIDENT-P1] Initial Notification: degraded service on PetSpotR**

- **Incident ID**: INC-YYYYMMDD-01
- **Severity**: P1 - Critical
- **Current Status**: Investigating
- **Affected Services**: [web-frontend / lostpet-service / foundpet-service / pet-matcher / notification-service]
- **Customer Impact**: Users may experience [5xx errors when submitting reports / delays in pet match notifications]
- **Incident Commander**: @<slack-handle>
- **War Room Bridge**: https://meet.google.com/xyz-abcd-efg
- **Dedicated Channel**: #incident-YYYYMMDD-01
- **Next Update**: In 15 minutes (HH:MM UTC)
```

### B. Periodic Update Template (Sent every 15m for P1, 30m for P2)

```markdown
**[INCIDENT-P1] Status Update #<number>**

- **Incident ID**: INC-YYYYMMDD-01
- **Current Status**: [Investigating | Identified | Mitigating | Monitoring]
- **Mitigation In Progress**: [e.g., Rolling back web-frontend to revision 00042-xyz]
- **Observed Metric Trend**: [e.g., 5xx rate dropped from 4.2% to 0.5%]
- **Next Steps**: [e.g., Validating /readyz probes across instances]
- **Next Update**: In 15 minutes (HH:MM UTC)
```

### C. Resolution Broadcast Template

```markdown
**[INCIDENT-P1] Resolution Notice**

- **Incident ID**: INC-YYYYMMDD-01
- **Severity**: P1 - Critical
- **Final Status**: Resolved
- **Impact Duration**: XX minutes (from HH:MM to HH:MM UTC)
- **Root Cause Summary**: [e.g., Incompatible schema change deployed in revision 00043 caused unhandled nil panic in matcher]
- **Resolution**: [e.g., Traffic rolled back to previous stable revision 00042]
- **Follow-up Actions**: Post-Incident Review scheduled for [Date/Time].
```

---

## 5. Root Cause Analysis (RCA) & Post-Incident Review (PIR) Checklist

Within **3 business days** following a P1 or P2 incident, the Incident Commander leads a blameless Post-Incident Review.

### Post-Incident Checklist

- [ ] **Data Gathering & Artifact Preservation**:
  - [ ] Cloud Trace distributed traces captured for anomalous transactions.
  - [ ] Error stack traces and structured log entries archived.
  - [ ] Cloud Monitoring metric charts exported (traffic, latency, 5xx rate, queue depth).
- [ ] **Timeline Reconstruction**:
  - [ ] Exact time of code commit or configuration change.
  - [ ] Exact time alert fired and first responder acknowledged.
  - [ ] Exact time mitigation command was executed.
  - [ ] Exact time metrics normalized.
- [ ] **Blameless 5-Whys Analysis**:
  - [ ] Focus on process, testing, tooling, and safety guardrails, never human blame.
  - [ ] Identify why automated test suites did not catch the issue prior to merge.
  - [ ] Identify why canary deployments or startup probes did not isolate the faulty revision.
- [ ] **Action Items Assignment**:
  - [ ] Every action item must have a single DRI (Directly Responsible Individual) and a GitHub issue link.
  - [ ] P1 action items must be scheduled for completion within **14 calendar days**.
  - [ ] P2 action items must be scheduled within **30 calendar days**.
  - [ ] Add new automated tests or alert policies to prevent direct recurrence.
