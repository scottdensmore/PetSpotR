# Milestone 9.2: Automated Neighborhood SMS/RCS Broadcast & Two-Way Interactive Dispatch Design Specification

- **Date:** 2026-09-20
- **Status:** Approved
- **Author:** Antigravity (Pair Programming Assistant)
- **Target Release:** PetSpotR Milestone 9.2

---

## 1. Executive Summary & Problem Statement

While Web Push and email notifications reach active app users, emergency lost pet situations require immediate, high-visibility alerting to local neighbors who may not have PetSpotR open. Furthermore, volunteers in the field need frictionless two-way interaction without needing to install an app or navigate complex web forms while outdoors.

**Milestone 9.2: Automated Neighborhood SMS/RCS Broadcast & Two-Way Interactive Dispatch** introduces:
1. **Zero-PII SMS Provider Abstraction (`pkg/sms`):** Pluggable provider interface with strict cryptographic phone number tokenization/hashing (`HMAC-SHA256`) and automated opt-out compliance (`STOP`, `UNSUBSCRIBE`, `START`).
2. **Hyper-Local Radius Broadcast:** Event listener reacting to `pet_lost` events that queries opted-in neighborhood phone numbers within a 2-mile geographic radius, sending instant concise alerts.
3. **Two-Way Inbound Webhook (`POST /api/v1/webhooks/sms/inbound`):** Receives SMS replies, parsing commands:
   - `CLAIM <SectorID>`: Claims the corresponding sector for the search party and sends an SMS confirmation with briefing URL.
   - `SIGHTED <Details>`: Immediately records a community sighting for the pet, recalculates trajectory, and broadcasts to the Reunion Room.
   - `STATUS`: Returns summary of active search progress and open sectors.

---

## 2. Architecture & Subsystem Boundaries

```text
               ┌───────────────────────────────────────────────────────────┐
               │              Volunteer Mobile Phone (SMS/MMS)             │
               └──────────────┬─────────────────────────────▲──────────────┘
                              │ Inbound SMS                 │ Outbound SMS
               ┌──────────────▼─────────────────────────────┴──────────────┐
               │             SMS Gateway (Mock / Twilio / Telnyx)          │
               └──────────────┬─────────────────────────────▲──────────────┘
                              │ Webhook                     │ REST
               ┌──────────────▼─────────────────────────────┴──────────────┐
               │               internal/app/webfrontend                    │
               │  - POST /api/v1/webhooks/sms/inbound                      │
               │  - Command Parser: CLAIM, SIGHTED, STATUS, STOP           │
               └──────────────┬─────────────────────────────▲──────────────┘
                              │                             │
               ┌──────────────▼────────────┐  ┌─────────────┴──────────────┐
               │         pkg/sms           │  │   internal/app/notification│
               │  - HMAC Tokenizer (No PII)│  │   - Geo-Radius Dispatch    │
               │  - Opt-Out State Manager  │  │   - Rate Limit Guards      │
               └───────────────────────────┘  └────────────────────────────┘
```

---

## 3. Data Models & API Contracts

### 3.1 Domain Models (`pkg/sms`)
```go
type InboundMessage struct {
    FromNumber string    `json:"from"`
    ToNumber   string    `json:"to"`
    Body       string    `json:"body"`
    Timestamp  time.Time `json:"timestamp"`
}

type CommandType string
const (
    CommandClaim   CommandType = "CLAIM"
    CommandSighted CommandType = "SIGHTED"
    CommandStatus  CommandType = "STATUS"
    CommandOptOut  CommandType = "STOP"
    CommandOptIn   CommandType = "START"
)
```

### 3.2 Inbound Webhook (`internal/app/webfrontend`)
- `POST /api/v1/webhooks/sms/inbound`:
  - Validates signature, parses body into `InboundMessage`.
  - Executes corresponding action:
    - If `CLAIM <Sector>`: assigns sector in search party, emits SSE update, returns TwiML/SMS reply.
    - If `SIGHTED <Notes>`: logs sighting, triggers trajectory recalculation, replies with confirmation.
    - If `STOP`: records opt-out in `store.SMSOptOutCollection`.

---

## 4. Compliance & Privacy
- Phone numbers stored only as salted HMAC-SHA256 hashes (`TokenizedPhone`).
- Clear opt-out messages provided in every outbound alert.
- Rate limiting per phone token to prevent SMS spam or abuse.
