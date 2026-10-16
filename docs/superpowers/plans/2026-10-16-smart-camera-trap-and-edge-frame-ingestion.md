# Smart Camera Trap & Home Security Webhook / Edge Frame Ingestion — Implementation Plan

- **Milestone:** 12.1
- **Spec:** `docs/superpowers/specs/2026-10-16-smart-camera-trap-and-edge-frame-ingestion-design.md`
- **Toolchain:** `export GOTOOLCHAIN=go1.26.5`
- **Worktree:** `.worktrees/feat-smart-camera-trap-ingestion` (`feat/smart-camera-trap-ingestion`)

---

## Task Breakdown (Subagent-Driven TDD)

### Task 1: Domain Models, State Store Collections & Validation
- **Files:**
  - Create `pkg/domain/cameratrap.go`
  - Create `pkg/domain/cameratrap_test.go`
  - Modify `pkg/store/store.go`
- **Details:**
  - Define `CameraVendor`, `CameraNodeStatus`, `CaptureVerificationStatus`, `CameraNode`, `ROIBoundingBox`, `CameraCapture`.
  - Add `CollectionCameraNodes = "camera_nodes"` and `CollectionCameraCaptures = "camera_captures"` to `pkg/store/store.go`.
  - Implement JSON round-trip and validation unit tests in `pkg/domain/cameratrap_test.go`.

### Task 2: Pure-Go Perceptual dHash, Night-Vision IR & Saliency ROI Pipeline (`pkg/cameratrap`)
- **Files:**
  - Create `pkg/cameratrap/dhash.go`
  - Create `pkg/cameratrap/processor.go`
  - Create `pkg/cameratrap/verifier.go`
  - Create `pkg/cameratrap/cameratrap_test.go`
- **Details:**
  - `ComputeDHash(img image.Image) uint64` and `FormatDHashHex(h uint64) string`.
  - `HammingDistance(a, b uint64) int` and `IsDuplicateFrame(prev, curr uint64, hasPrev bool, maxDist int) (bool, int)`.
  - `DetectNightVisionIR(img image.Image) bool`: detects monochromatic/low-chrominance IR frames.
  - `DetectMotionROI(img image.Image) (domain.ROIBoundingBox, float64)`: `8x8` grid luminance/gradient saliency detector isolating foreground subject bounding box and returning normalized `[0.0, 1.0]` coordinates + `SaliencyScore`.
  - `ProcessEdgeFrame(img image.Image, prevHash uint64, hasPrev bool) (*FrameAnalysisResult, error)`: runs `dHash`, deduplication, IR auto-enhancement (`imaging.AutoEnhance`), ROI bounding box detection, and cropped JPEG thumbnail encoding.
  - `VerifyCaptureWithOllama(ctx context.Context, client *ollama.Client, capture domain.CameraCapture, vendorHintSpecies string, vendorHintTraits []string) domain.CameraCapture`: optional `gemma4:e2b` vision verification with 1.5s timeout and deterministic fallback.

### Task 3: WebFrontend REST API, Multi-Vendor Webhook Ingestion & Sighting Promotion
- **Files:**
  - Create `internal/app/webfrontend/cameratrap_handlers.go`
  - Create `internal/app/webfrontend/cameratrap_handlers_test.go`
  - Modify `internal/app/webfrontend/server.go`
- **Details:**
  - Register routes in `server.go`:
    - `GET /cameras` -> `s.handleCamerasPage`
    - `POST /api/v1/cameras` -> `s.handleCreateCameraNode`
    - `GET /api/v1/cameras` -> `s.handleListCameraNodes`
    - `POST /api/v1/cameras/{id}/webhook` -> `s.handleCameraWebhook`
    - `GET /api/v1/cameras/captures` -> `s.handleListCameraCaptures`
    - `POST /api/v1/cameras/captures/{id}/promote` -> `s.handlePromoteCameraCapture`
    - `POST /api/v1/cameras/captures/{id}/dismiss` -> `s.handleDismissCameraCapture`
    - `GET /api/v1/cameras/stream` -> `s.handleCamerasStream`
  - Authenticate camera webhook tokens via `X-Camera-Token` or `Authorization: Bearer` against SHA-256 `WebhookSecretHash`.
  - Support auto-promotion when `!capture.IsDuplicate && camera.AutoPromoteThreshold > 0 && capture.ConfidenceScore >= camera.AutoPromoteThreshold`, creating a verified `domain.Sighting` in `store.SightingsCollection`.
  - Broadcast SSE events on topic `"cameras"` and WebRTC mesh notifications via `s.signalingHub`.

### Task 4: Camera Trap Command Center UI, ROI Overlay & WCAG AAA CSS
- **Files:**
  - Create `internal/app/webfrontend/templates/cameras.html`
  - Create `internal/app/webfrontend/static/js/camera-trap.js`
  - Modify `internal/app/webfrontend/templates/layout.html`
  - Modify `internal/app/webfrontend/static/css/styles.css`
  - Create `tests/playwright/unit/camera-trap.spec.ts`
- **Details:**
  - Build `/cameras` Command Center with Camera Node Registry (`#camera-register-form`), Edge Frame Webhook Simulator (`#webhook-simulator-form`), Night-Vision IR Phosphor toggle (`#toggle-ir-phosphor`), and Live Capture Review Grid (`#camera-captures-grid`) with normalized ROI bounding-box overlays (`.roi-bounding-box`) and one-click Sighting Promotion (`.btn-promote-capture`).
  - Enforce WCAG AAA (`> 7:1`) contrast across light and dark themes and strict CSP compliance (zero inline `<script>` or `eval()`).

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- **Files:**
  - Create `tests/playwright/e2e/camera-trap-journey.spec.ts`
- **Details:**
  - End-to-end 6-step user journey verifying:
    1. Camera Node Registration (`RING` / `TRAIL_CAM`) and webhook token issuance.
    2. Edge Frame Ingestion via `/api/v1/cameras/{id}/webhook` with ROI bounding-box detection and night-vision IR auto-enhancement.
    3. Perceptual `dHash` duplicate frame suppression when sending an identical static frame.
    4. Live UI inspection on `/cameras`, ROI overlay rendering, and IR phosphor toggle.
    5. One-click promotion of a `PENDING_REVIEW` capture to a verified Pet Sighting (`PROMOTED`) and verification in `/api/v1/sightings`.
    6. WCAG AAA contrast & CSP compliance verification.

### Task 6: Final Whole-Branch Code Review (`pro` model) & Verification Gate
- **Details:**
  - Run comprehensive whole-branch code review, remediate any findings, verify `export GOTOOLCHAIN=go1.26.5 && make verify` and 100% of Playwright tests pass.
