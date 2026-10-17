# Volunteer Credentialing, SAR Gamification & Incident Command (ICS) Shift Rosters — Design Specification

- **Milestone:** 12.3
- **Date:** 2026-10-17
- **Status:** Approved
- **Author:** Antigravity & User

---

## 1. Problem Statement & Objectives

Across Milestones 8.1 through 12.2, PetSpotR introduced community search parties, GPS breadcrumb trails, drone reconnaissance, off-grid P2P mesh coordination, emergency veterinary triage, and smart camera traps. However, large-scale wilderness and disaster search-and-rescue (SAR) operations require:
1. **Verifiable Responder Credentials:** Ensuring that specialized roles—such as FAA Part 107 UAV Pilots (`UAV_PART107_PILOT`), Veterinary Field Medics (`VET_MEDIC`), Certified K9 Handlers (`K9_HANDLER`), and Incident Commanders (`ICS_COMMANDER`)—carry cryptographically verifiable qualifications rather than unverified self-assertions.
2. **NIMS / FEMA Incident Command System (ICS-211 Check-In & ICS-204 Assignment) Shift Rosters with Fatigue Guardrails:** Tracking when volunteers check into a search operation, which sector/task they are assigned to, and how long they have been continuously on shift—automatically flagging `FATIGUE_WARNING` (`>= 6.0` hours) and enforcing `MANDATORY_REST` (`>= 8.0` hours) to prevent responder exhaustion and unsafe wilderness deployment.
3. **Deterministic SAR Gamification, XP & Merit Badges:** Recognizing and motivating sustained community participation through deterministic XP progression, responder levels, and verifiable merit badges (`FIRST_RESPONDER`, `NIGHT_OWL_IR`, `GRID_MASTER_50KM`, `CANINE_TRACKER`, `AERIAL_SPOTTER`, `TRIAGE_LIFESAVER`, `REUNION_HERO`).

---

## 2. Domain Models & State Store Collections

### 2.1 State Store Collections (`pkg/store/store.go`)
- `CollectionVolunteerProfiles = "volunteer_profiles"`
- `CollectionICSShiftEntries = "ics_shift_entries"`

### 2.2 Domain Models (`pkg/domain/ics_roster.go`)

```go
package domain

import "time"

type SARRoleCredential string

const (
	RoleFieldSearcher   SARRoleCredential = "FIELD_SEARCHER"
	RoleK9Handler       SARRoleCredential = "K9_HANDLER"
	RoleUAVPart107Pilot SARRoleCredential = "UAV_PART107_PILOT"
	RoleVetMedic        SARRoleCredential = "VET_MEDIC"
	RoleICSCommander    SARRoleCredential = "ICS_COMMANDER"
)

type SARMeritBadge string

const (
	BadgeFirstResponder  SARMeritBadge = "FIRST_RESPONDER"
	BadgeNightOwlIR      SARMeritBadge = "NIGHT_OWL_IR"
	BadgeGridMaster50Km  SARMeritBadge = "GRID_MASTER_50KM"
	BadgeCanineTracker   SARMeritBadge = "CANINE_TRACKER"
	BadgeAerialSpotter   SARMeritBadge = "AERIAL_SPOTTER"
	BadgeTriageLifesaver SARMeritBadge = "TRIAGE_LIFESAVER"
	BadgeReunionHero     SARMeritBadge = "REUNION_HERO"
)

type ICSShiftStatus string

const (
	ShiftStatusCheckedIn      ICSShiftStatus = "CHECKED_IN"
	ShiftStatusOnSectorPatrol ICSShiftStatus = "ON_SECTOR_PATROL"
	ShiftStatusMandatoryRest  ICSShiftStatus = "MANDATORY_REST"
	ShiftStatusCheckedOut     ICSShiftStatus = "CHECKED_OUT"
)

type FatigueAlertLevel string

const (
	FatigueLevelNormal       FatigueAlertLevel = "NORMAL_FIT"        // < 6.0 hours
	FatigueLevelWarning      FatigueAlertLevel = "FATIGUE_WARNING"   // 6.0 to < 8.0 hours
	FatigueLevelMandatoryRest FatigueAlertLevel = "MANDATORY_REST"   // >= 8.0 hours
)

type SignedCredential struct {
	CredentialID  string            `json:"credentialId"`
	Role          SARRoleCredential `json:"role"`
	IssuedBy      string            `json:"issuedBy"`
	LicenseNumber string            `json:"licenseNumber,omitempty"`
	IssuedAt      time.Time         `json:"issuedAt"`
	ExpiresAt     time.Time         `json:"expiresAt"`
	SignatureHex  string            `json:"signatureHex"`
}

type VolunteerProfile struct {
	VolunteerID       string              `json:"volunteerId"`
	Name              string              `json:"name"`
	Callsign          string              `json:"callsign"`
	PrimaryRole       SARRoleCredential   `json:"primaryRole"`
	Credentials       []SignedCredential  `json:"credentials"`
	EarnedBadges      []SARMeritBadge     `json:"earnedBadges"`
	TotalXP           int                 `json:"totalXp"`
	RankTitle         string              `json:"rankTitle"`
	CompletedShifts   int                 `json:"completedShifts"`
	TotalDistanceKm   float64             `json:"totalDistanceKm"`
	TotalHoursLogged  float64             `json:"totalHoursLogged"`
	VerifiedSightings int                 `json:"verifiedSightings"`
	ReunionsAssisted  int                 `json:"reunionsAssisted"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
}

type ICSShiftEntry struct {
	ShiftID            string            `json:"shiftId"`
	IncidentID         string            `json:"incidentId"`
	VolunteerID        string            `json:"volunteerId"`
	VolunteerName      string            `json:"volunteerName"`
	Callsign           string            `json:"callsign"`
	AssignedRole       SARRoleCredential `json:"assignedRole"`
	AssignedSectorID   string            `json:"assignedSectorId"`
	Status             ICSShiftStatus    `json:"status"`
	ActiveHours        float64           `json:"activeHours"`
	DistanceCoveredKm  float64           `json:"distanceCoveredKm"`
	FatigueLevel       FatigueAlertLevel `json:"fatigueLevel"`
	FatigueNotes       string            `json:"fatigueNotes,omitempty"`
	CheckedInAt        time.Time         `json:"checkedInAt"`
	LastHeartbeatAt    time.Time         `json:"lastHeartbeatAt"`
	CheckedOutAt       *time.Time        `json:"checkedOutAt,omitempty"`
}
```

---

## 3. Pure-Go Credential Signer, Fatigue Evaluator & Gamification Engine (`pkg/ics`)

1. **Cryptographic Credential Signing & Role Gate (`pkg/ics/credentials.go`):**
   - `SignRoleCredential(volunteerID string, cred domain.SignedCredential, secretKey []byte) (domain.SignedCredential, error)`: computes canonical HMAC-SHA256 signature over `volunteerID|credentialId|role|issuedBy|licenseNumber|issuedAt|expiresAt`.
   - `VerifyRoleCredential(volunteerID string, cred domain.SignedCredential, secretKey []byte, now time.Time) (bool, error)`: verifies constant-time HMAC-SHA256 match and `!now.After(cred.ExpiresAt)`.
   - `HasVerifiedRole(profile domain.VolunteerProfile, requiredRole domain.SARRoleCredential, secretKey []byte, now time.Time) bool`: returns `true` for `FIELD_SEARCHER` baseline or when `profile.Credentials` contains an unexpired, cryptographically valid `SignedCredential` for `requiredRole`.

2. **Fatigue Guardrails & ICS-204 CSV Exporter (`pkg/ics/fatigue.go`):**
   - `EvaluateShiftFatigue(activeHours float64, distanceKm float64) (domain.FatigueAlertLevel, domain.ICSShiftStatus, string)`:
     - `activeHours >= 8.0` or `distanceKm >= 25.0`: returns `FatigueLevelMandatoryRest`, `ShiftStatusMandatoryRest`, `"Mandatory 8h/25km SAR safety ceiling reached — stand down for rehab & rest"`
     - `activeHours >= 6.0` or `distanceKm >= 18.0`: returns `FatigueLevelWarning`, `ShiftStatusOnSectorPatrol`, `"Approaching 6h/18km fatigue threshold — schedule relief team"`
     - Otherwise: returns `FatigueLevelNormal`, `ShiftStatusOnSectorPatrol`, `"Fit for active sector patrol"`
   - `ExportICS204RosterCSV(shifts []domain.ICSShiftEntry) (string, error)`: exports NIMS ICS-204 / ICS-211 compliant shift roster CSV.

3. **SAR Gamification, XP & Merit Badge Engine (`pkg/ics/gamification.go`):**
   - `EvaluateVolunteerProgression(profile domain.VolunteerProfile, nightShift bool) domain.VolunteerProfile`:
     - Calculates `TotalXP`: `CompletedShifts*100 + int(TotalDistanceKm*20) + int(TotalHoursLogged*25) + VerifiedSightings*150 + ReunionsAssisted*500 + len(Credentials)*200`.
     - Computes `RankTitle`:
       - `< 500 XP`: `"Rookie Spotter (Tier 1)"`
       - `500–1499 XP`: `"Field Tracker (Tier 2)"`
       - `1500–2999 XP`: `"Senior SAR Specialist (Tier 3)"`
       - `>= 3000 XP`: `"Master Incident Vanguard (Tier 4)"`
     - Deterministically awards merit badges (`EarnedBadges`) without duplicates:
       - `FIRST_RESPONDER`: `CompletedShifts >= 1`
       - `NIGHT_OWL_IR`: `nightShift` (or existing badge)
       - `GRID_MASTER_50KM`: `TotalDistanceKm >= 50.0`
       - `CANINE_TRACKER`: holds `K9_HANDLER` credential
       - `AERIAL_SPOTTER`: holds `UAV_PART107_PILOT` credential
       - `TRIAGE_LIFESAVER`: holds `VET_MEDIC` credential
       - `REUNION_HERO`: `ReunionsAssisted >= 1`

---

## 4. REST API & ICS Command Cockpit (`internal/app/webfrontend/ics_handlers.go`)

1. `GET /ics-command` — Renders the Incident Command (ICS) Shift Roster, Volunteer Credentialing & SAR Gamification Cockpit (`templates/ics_command.html`).
2. `GET /api/v1/ics/volunteers` — Lists volunteer profiles sorted by `TotalXP` descending (seeds 3 default credentialed SAR responders if empty).
3. `POST /api/v1/ics/volunteers` — Registers a new volunteer profile (and issues initial signed role credential), awarding baseline XP/rank.
4. `POST /api/v1/ics/volunteers/{id}/credentials` — Issues a cryptographically signed `SignedCredential` (`K9_HANDLER`, `UAV_PART107_PILOT`, `VET_MEDIC`, `ICS_COMMANDER`) and recalculates XP & merit badges.
5. `POST /api/v1/ics/shifts` — Checks a volunteer into an ICS-211 shift (`ICSShiftEntry`), enforcing the role qualification gate (returns `403 Forbidden` if the volunteer lacks a verified credential for a specialized role like `UAV_PART107_PILOT` or `VET_MEDIC`), evaluating fatigue level, and broadcasting `ics_shift_updated` via SSE (`/api/v1/ics/stream`) and WebRTC mesh.
6. `GET /api/v1/ics/shifts` — Lists active and historical ICS shifts (optional `?incidentId=`, `?status=`, or `?format=csv` for ICS-204 CSV export).
7. `POST /api/v1/ics/shifts/{id}/progress` — Updates shift hours, distance covered (`km`), verified sightings, or checkout status (`checkout: true`), automatically triggering `FATIGUE_WARNING` / `MANDATORY_REST` when thresholds are crossed and updating the volunteer's cumulative XP, rank, and merit badges.
8. `GET /api/v1/ics/stream` — Public SSE stream on topic `"ics"` for live Incident Command updates.
