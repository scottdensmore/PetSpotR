# Milestone 13.3: Humane Live-Trap (Box/Drop Trap) IoT Telemetry, Bait Decay & Rapid Dispatch Cockpit — Design Specification

## 1. Problem Statement & Goals
Timid, panicked, or feral-reverted lost pets (especially indoor cats and skittish dogs in survival mode) rarely approach humans when called and are most safely recovered using humane box traps (e.g., Tomahawk / Havahart) or remote drop traps equipped with cellular/LoRa door-trip sensors. However, unattended live traps pose severe animal welfare risks:
1. **Delayed Trip Response:** Once a trap door trips (`DOOR_TRIPPED`), a trapped animal can suffer extreme panic, self-injury, or predation if a responder does not arrive within the **15-minute Humane Check SLA Window**.
2. **Thermal & Weather Exposure:** Traps deployed in direct sun (`> 32.0°C` internal enclosure temperature) or freezing temperatures (`< 2.0°C`) become life-threatening if not disarmed or shaded immediately (`THERMAL_LOCKOUT_HAZARD`).
3. **Olfactory Bait Decay:** Wet food, sardines,rotisserie chicken, or KFC bait loses volatile scent dispersion or spoils over time depending on ambient temperature, reducing capture probability unless refreshed on schedule.
4. **Non-Target Wildlife / Bycatch:** Raccoons, opossums, or stray cats frequently trip traps and must be safely released while re-arming and re-baiting the trap, whereas target pet captures must immediately promote a verified high-confidence Community Sighting and dispatch a recovery runner.

### Key Objectives
- **Domain Model & State Store (`pkg/domain/humane_trap.go`, `pkg/store/store.go`):**
  - Model `HumaneTrapType` (`BOX_TRAP_GRAVITY`, `DROP_TRAP_REMOTE`, `ENCLOSURE_PEN_CELLULAR`), `HumaneTrapStatus` (`ARMED_ACTIVE`, `DOOR_TRIPPED_URGENT`, `RESPONDER_DISPATCHED`, `TARGET_CAPTURED_SECURED`, `THERMAL_LOCKOUT_DISARMED`), `BaitFreshnessState` (`FRESH_PEAK_SCENT`, `DEGRADING_REFRESH_SOON`, `SPOILED_EXPIRED`), and `HumaneTrapDeployment` in `store.CollectionHumaneTrapDeployments = "humane_trap_deployments"`.
- **Pure-Go Humane Trap Telemetry, Bait Decay, Thermal Safety & SLA Engine (`pkg/humanetrap`):**
  - Compute Arrhenius-inspired olfactory bait freshness percentage (`0–100%`) and `BaitFreshnessState` from bait type (`SARDINES_IN_OIL`, `ROTISSERIE_CHICKEN`, `WET_RECOVERY_PATE`, `CATNIP_MACKEREL_BLEND`), hours elapsed since baiting, and enclosure temperature (`°C`).
  - Evaluate thermal safety limits (`InternalTempC >= 32.0` hyperthermia risk or `InternalTempC <= 2.0` hypothermia risk) and low battery (`BatteryPct <= 15%`), generating structured `SafetyAlerts`.
  - Process IoT door-trip webhook payloads (reed switch + accelerometer G-force spike + weight plate `EstimatedWeightKg`), setting a strict **15-minute Humane Check SLA Deadline (`CheckSLADeadline`)** and classifying whether `EstimatedWeightKg` matches the target pet's expected weight range or indicates likely non-target wildlife bycatch.
  - Handle rapid responder dispatch (`AssignTrapResponder`) and field resolution (`ResolveTrapOutcome`: `TARGET_PET_CAPTURED` vs `BYCATCH_RELEASED_REARMED`).
- **WebFrontend REST API, Auto-Sighting Promotion & Real-Time SSE/Mesh Stream (`internal/app/webfrontend/humane_trap_handlers.go`):**
  - `GET /humane-traps`: Renders the Humane Live-Trap IoT Telemetry & Rapid Dispatch Cockpit UI.
  - `GET /api/v1/humane-traps`: Lists all deployed humane traps (seeding 3 realistic traps on first access: `trap-ht-101` `DOOR_TRIPPED_URGENT`, `trap-ht-102` `ARMED_ACTIVE`, `trap-ht-103` `THERMAL_LOCKOUT_DISARMED`).
  - `POST /api/v1/humane-traps`: Registers and arms a new humane trap deployment with initial bait, telemetry, and thermal evaluation.
  - `POST /api/v1/humane-traps/{id}/telemetry`: Ingests IoT sensor heartbeat or door-trip alert (`doorClosed`, `internalTempC`, `batteryPct`, `rssiDbm`, `accelerometerPeakG`, `estimatedWeightKg`, `baitReset`), computing SLA deadlines and thermal lockouts.
  - `POST /api/v1/humane-traps/{id}/dispatch`: Dispatches a field volunteer/handler to a tripped or hazardous trap (`responderId`, `responderName`, `etaMinutes`) or resolves the field check (`outcome`: `TARGET_PET_CAPTURED` | `BYCATCH_RELEASED_REARMED` | `THERMAL_SHADED_REARMED`, `notes`). When `outcome == "TARGET_PET_CAPTURED"`, automatically promotes a verified `domain.Sighting` in `store.SightingsCollection` (`PromotedSightingID`).
  - `GET /api/v1/humane-traps/stream`: SSE stream broadcasting `"humane_trap_updated"` events on topic `"humane_traps"` (and WebRTC mesh signaling hub).
- **Accessible WCAG AAA Cockpit UI (`/humane-traps`):**
  - Strict CSP compliance (zero inline `<script>` or `style=`), semantic landmarks, keyboard navigation, ARIA live announcements, and verified `> 7.1:1` WCAG AAA contrast ratios across dark and light themes.
- **Verification:**
  - `make verify` (0 linter issues, `-race` tests pass) and 100% Playwright unit (`tests/playwright/unit/humane-traps.spec.ts`) & E2E (`tests/playwright/e2e/humane-traps-journey.spec.ts`) test pass rate.
