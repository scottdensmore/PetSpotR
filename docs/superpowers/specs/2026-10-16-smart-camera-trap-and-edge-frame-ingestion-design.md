# Smart Camera Trap & Home Security Webhook / Edge Frame Ingestion — Design Specification

- **Milestone:** 12.1
- **Date:** 2026-10-16
- **Status:** Approved
- **Author:** Antigravity & User

---

## 1. Problem Statement & Objectives

Many lost pets—especially skittish cats and displaced dogs—only emerge at dusk, dawn, or overnight when human search parties are asleep. Meanwhile, residential neighborhoods are densely covered by smart home security cameras (Ring, Google Nest, Wyze, Frigate NVR/RTSP) and wilderness search perimeters deploy cellular trail camera traps.

Today, pet owners and volunteers must manually monitor camera apps, download video clips, take screenshots, and manually fill out sighting forms. Furthermore, raw motion alerts from outdoor cameras suffer from high false-positive rates caused by swaying branches, passing headlights, or static background repeats.

**Milestone 12.1** introduces **Smart Camera Trap & Home Security Webhook / Edge Frame Ingestion**:
1. **Multi-Vendor Camera Registry & Cryptographic Authentication:** Volunteers and homeowners register stationary camera nodes (`RING`, `NEST`, `WYZE`, `FRIGATE_RTSP`, `TRAIL_CAM`) with fixed GPS coordinates, field-of-view (FOV) azimuth/angle metadata, and per-camera HMAC-SHA256 webhook secrets.
2. **Two-Tier Pure-Go Edge Frame Pipeline (`pkg/cameratrap`):**
   - **Tier 1 (Deterministic Pure Go):** Computes a 64-bit perceptual difference hash (`dHash`, 9x8 luminance grid) and Hamming distance against the camera's previous frame to suppress duplicate static background triggers (`HammingDistance <= 6`). Evaluates infrared (IR) / low-light characteristics, automatically enhances night-vision contrast via `pkg/imaging`, and executes a luminance/gradient saliency grid detector to isolate the primary Region of Interest (ROI bounding box) and crop a compact JPEG thumbnail.
   - **Tier 2 (Multimodal AI Vision Verification):** Optionally invokes Ollama (`gemma4:e2b`, 1.5s timeout) on the cropped ROI to classify domestic animals (`DOG`, `CAT`, `WILDLIFE`, `NONE`), extract coat/color traits, and compute a composite confidence score, falling back deterministically to Tier 1 saliency confidence when offline.
3. **Automated Sighting Promotion & Real-Time Distribution:** Captures exceeding the camera's auto-promote threshold (or manually promoted by a reviewer in one click) automatically generate a `domain.Sighting` record in `store.SightingsCollection` and broadcast real-time alerts via SSE (`/api/v1/cameras/stream` and `s.reunionHub`) and the off-grid WebRTC mesh (`s.signalingHub`).
4. **Camera Trap Command Center UI (`/cameras`):** An accessible, WCAG AAA (`> 7:1` contrast) command cockpit featuring a camera registration modal, multi-vendor webhook simulator, live motion capture gallery with ROI bounding-box overlays, night-vision IR filter toggle, and one-click sighting promotion.

---

## 2. Domain Models & State Store Collections

### 2.1 Store Collections (`pkg/store/store.go`)
- `CollectionCameraNodes = "camera_nodes"`
- `CollectionCameraCaptures = "camera_captures"`

### 2.2 Domain Types (`pkg/domain/cameratrap.go`)

```go
package domain

import "time"

type CameraVendor string

const (
	CameraVendorRing        CameraVendor = "RING"
	CameraVendorNest        CameraVendor = "NEST"
	CameraVendorWyze        CameraVendor = "WYZE"
	CameraVendorFrigateRTSP CameraVendor = "FRIGATE_RTSP"
	CameraVendorTrailCam    CameraVendor = "TRAIL_CAM"
)

type CameraNodeStatus string

const (
	CameraStatusActive  CameraNodeStatus = "ACTIVE"
	CameraStatusPaused  CameraNodeStatus = "PAUSED"
	CameraStatusOffline CameraNodeStatus = "OFFLINE"
)

type CaptureVerificationStatus string

const (
	CaptureStatusPendingReview CaptureVerificationStatus = "PENDING_REVIEW"
	CaptureStatusPromoted      CaptureVerificationStatus = "PROMOTED"
	CaptureStatusDismissed     CaptureVerificationStatus = "DISMISSED"
	CaptureStatusDuplicate     CaptureVerificationStatus = "DUPLICATE_SUPPRESSED"
)

type CameraNode struct {
	ID                   string           `json:"id"`
	Name                 string           `json:"name"`
	Vendor               CameraVendor     `json:"vendor"`
	OwnerName            string           `json:"ownerName,omitempty"`
	TargetPetID          string           `json:"targetPetId,omitempty"`
	Latitude             float64          `json:"latitude"`
	Longitude            float64          `json:"longitude"`
	HeadingDegrees       float64          `json:"headingDegrees"`
	FOVDegrees           float64          `json:"fovDegrees"`
	NightVisionIR        bool             `json:"nightVisionIr"`
	AutoPromoteThreshold float64          `json:"autoPromoteThreshold"`
	WebhookSecretHash    string           `json:"webhookSecretHash,omitempty"`
	WebhookTokenPreview  string           `json:"webhookTokenPreview,omitempty"`
	LastFrameDHash       uint64           `json:"lastFrameDHash"`
	LastFrameDHashHex    string           `json:"lastFrameDHashHex,omitempty"`
	TotalCaptures        int              `json:"totalCaptures"`
	SuppressedDuplicates int              `json:"suppressedDuplicates"`
	Status               CameraNodeStatus `json:"status"`
	CreatedAt            time.Time        `json:"createdAt"`
	LastSeenAt           *time.Time       `json:"lastSeenAt,omitempty"`
}

type ROIBoundingBox struct {
	X      float64 `json:"x"`      // Normalized [0.0, 1.0]
	Y      float64 `json:"y"`      // Normalized [0.0, 1.0]
	Width  float64 `json:"width"`  // Normalized [0.0, 1.0]
	Height float64 `json:"height"` // Normalized [0.0, 1.0]
}

type CameraCapture struct {
	ID                  string                    `json:"id"`
	CameraID            string                    `json:"cameraId"`
	CameraName          string                    `json:"cameraName"`
	Vendor              CameraVendor              `json:"vendor"`
	TargetPetID         string                    `json:"targetPetId,omitempty"`
	Latitude            float64                   `json:"latitude"`
	Longitude           float64                   `json:"longitude"`
	CapturedAt          time.Time                 `json:"capturedAt"`
	FrameDHash          uint64                    `json:"frameDHash"`
	FrameDHashHex       string                    `json:"frameDHashHex"`
	HammingDistance     int                       `json:"hammingDistance"`
	IsDuplicate         bool                      `json:"isDuplicate"`
	IsNightVisionIR     bool                      `json:"isNightVisionIr"`
	IREnhanced          bool                      `json:"irEnhanced"`
	ROI                 ROIBoundingBox            `json:"roi"`
	DetectedSpecies     string                    `json:"detectedSpecies"`     // "DOG", "CAT", "WILDLIFE", "UNKNOWN"
	ExtractedTraits     []string                  `json:"extractedTraits"`     // e.g. ["Golden coat", "Pointed ears", "Reflective collar"]
	SaliencyScore       float64                   `json:"saliencyScore"`       // [0.0, 1.0]
	ConfidenceScore     float64                   `json:"confidenceScore"`     // [0.0, 1.0]
	SnapshotDataURI     string                    `json:"snapshotDataUri"`
	CropThumbnailURI    string                    `json:"cropThumbnailUri,omitempty"`
	VerificationStatus  CaptureVerificationStatus `json:"verificationStatus"`
	PromotedSightingID  string                    `json:"promotedSightingId,omitempty"`
}
```

---

## 3. Pure-Go Edge Frame Pipeline (`pkg/cameratrap`)

### 3.1 Perceptual Difference Hash (`dHash`) & Deduplication (`pkg/cameratrap/dhash.go`)
- `ComputeDHash(img image.Image) uint64`:
  - Samples the image onto a `9 x 8` grayscale luminance grid (`Y = 0.299R + 0.587G + 0.114B`).
  - For each of the 8 rows and 8 adjacent horizontal pairs `(col, col+1)`, sets bit `(row*8 + col)` to `1` if `lum[row][col] > lum[row][col+1]`, producing a deterministic 64-bit structural fingerprint.
- `HammingDistance(hashA, hashB uint64) int`:
  - Uses `bits.OnesCount64(hashA ^ hashB)` from `math/bits`.
- `IsDuplicateFrame(prevHash, currHash uint64, hasPrev bool, maxDistance int) (bool, int)`:
  - When `hasPrev` is true and `HammingDistance(prevHash, currHash) <= maxDistance` (default `6`), classifies the frame as a static background repeat.

### 3.2 Night-Vision IR Detection, Saliency ROI & Cropping (`pkg/cameratrap/processor.go`)
- `DetectNightVisionIR(img image.Image) bool`:
  - Samples pixels across the frame and computes average RGB channel divergence (`|R-G| + |G-B| + |R-B|`). Near-monochromatic frames (low chrominance variance or low luminance) are flagged as `IsNightVisionIR = true`.
- `DetectMotionROI(img image.Image) (domain.ROIBoundingBox, float64)`:
  - Divides the image into an `8 x 8` spatial grid.
  - Computes local luminance contrast and edge gradient energy relative to the perimeter background border cells.
  - Identifies the highest-energy contiguous cell cluster, returning normalized `ROIBoundingBox{X, Y, Width, Height}` clamped to `[0.0, 1.0]` and a deterministic `SaliencyScore` in `[0.0, 1.0]`.
- `ProcessEdgeFrame(img image.Image, prevHash uint64, hasPrev bool) (*FrameAnalysisResult, error)`:
  - Computes `dHash` and `HammingDistance`.
  - Detects `IsNightVisionIR`; if low-light/IR is detected, invokes `imaging.AutoEnhance(img)` to normalize contrast.
  - Extracts `ROI` and crops the sub-image, encoding a compact JPEG data URI (`CropThumbnailURI`).

### 3.3 Multimodal AI Vision Verifier (`pkg/cameratrap/verifier.go`)
- `VerifyCaptureWithOllama(ctx context.Context, client *ollama.Client, capture domain.CameraCapture, vendorHintSpecies string, vendorHintTraits []string) domain.CameraCapture`:
  - When `client != nil`, sends the ROI crop (or full snapshot) to `gemma4:e2b` with a `1500ms` timeout requesting structured JSON: `{"species": "DOG|CAT|WILDLIFE|NONE", "confidence": 0.92, "traits": ["..."]}`.
  - When `client == nil` or Ollama times out, gracefully falls back to vendor payload hints and deterministic `SaliencyScore`.

---

## 4. REST API & Multi-Vendor Webhook Endpoints (`internal/app/webfrontend/cameratrap_handlers.go`)

1. `POST /api/v1/cameras` — Register a new `CameraNode`, generating a cryptographically random webhook token (`cam_tok_...`), storing its SHA-256 hash (`WebhookSecretHash`) and preview (`WebhookTokenPreview`), and returning the one-time token in the creation response.
2. `GET /api/v1/cameras` — List registered `CameraNode`s (optional `?targetPetId=` filter).
3. `POST /api/v1/cameras/{id}/webhook` — Ingest an edge motion frame from a registered camera (`RING`, `NEST`, `WYZE`, `FRIGATE_RTSP`, `TRAIL_CAM`).
   - Validates the camera token via `X-Camera-Token` header, `Authorization: Bearer`, or HMAC-SHA256 `X-Camera-Signature` header (when `WebhookSecretHash` is configured on the node).
   - Runs `ProcessEdgeFrame` and `VerifyCaptureWithOllama`.
   - Updates `CameraNode` (`TotalCaptures`, `SuppressedDuplicates`, `LastFrameDHash`, `LastSeenAt`) atomically via `UpdateState`.
   - If `!capture.IsDuplicate` and `capture.ConfidenceScore >= camera.AutoPromoteThreshold` (when `AutoPromoteThreshold > 0`), automatically creates a `domain.Sighting` in `store.SightingsCollection` and sets `VerificationStatus = CaptureStatusPromoted`.
   - Broadcasts `camera_capture_received` on SSE (`/api/v1/cameras/stream` and `s.reunionHub`) and WebRTC mesh (`s.signalingHub`).
4. `GET /api/v1/cameras/captures` — List captured motion events (sorted newest-first, optional `?cameraId=`, `?status=`, `?includeDuplicates=true` filters).
5. `POST /api/v1/cameras/captures/{id}/promote` — Promote a `CameraCapture` to a verified `domain.Sighting` in `store.SightingsCollection` (or dismiss via `POST /api/v1/cameras/captures/{id}/dismiss`).
6. `GET /api/v1/cameras/stream` — Dedicated public SSE stream on topic `"cameras"` for real-time cockpit updates.

---

## 5. Camera Trap Command Center UI & Accessibility

- **Route:** `GET /cameras` (`templates/cameras.html`, `static/js/camera-trap.js`)
- **Navigation Link:** Added to `templates/layout.html` header nav.
- **Core UI Components:**
  1. **Camera Registry Panel & Registration Form (`#camera-register-form`):** Register Ring, Nest, Wyze, Frigate RTSP, or Trail Cam nodes with GPS coordinates, heading, FOV, night-vision IR mode, and auto-promote confidence threshold. Displays generated webhook endpoint and token.
  2. **Edge Webhook Simulator (`#webhook-simulator-form`):** Allows field coordinators to test/simulate motion frames from registered cameras with customizable synthetic canvas snapshots (daylight vs. night-vision IR, moving pet subject vs. static background repeat to demonstrate `dHash` duplicate suppression).
  3. **Live Capture Feed (`#camera-captures-grid`):** Displays incoming captures with:
     - Visual snapshot viewport with normalized ROI bounding-box overlay (`.roi-bounding-box`).
     - Night-vision IR high-contrast green phosphor filter toggle (`.ir-phosphor-mode`).
     - Badges for vendor, `dHash` fingerprint, confidence percentage, detected species, and verification status (`PENDING_REVIEW`, `PROMOTED`, `DUPLICATE_SUPPRESSED`, `DISMISSED`).
     - Action buttons: **Promote to Sighting** (`.btn-promote-capture`) and **Dismiss** (`.btn-dismiss-capture`).
- **Accessibility & Security:**
  - Strict CSP compliance: Zero inline `<script>` or `eval()`.
  - Strict WCAG AAA contrast (`> 7:1`) for all status badges (`.badge-capture-promoted`, `.badge-capture-pending`, `.badge-capture-duplicate`, `.badge-capture-dismissed`) across light and dark themes.
