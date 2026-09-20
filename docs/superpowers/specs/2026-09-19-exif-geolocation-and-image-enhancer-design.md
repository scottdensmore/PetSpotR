# Milestone 8.2: EXIF Geolocation Auto-Extraction & AI Image Quality Enhancer Design Specification

- **Date:** 2026-09-19
- **Status:** Approved
- **Author:** Antigravity (Pair Programming Assistant)
- **Target Release:** PetSpotR Milestone 8.2

---

## 1. Executive Summary & Problem Statement

When a pet goes missing or is found by a community member, time is of the essence. PetSpotR users frequently upload photos taken on their mobile devices, but manually entering the exact location and time of a sighting can be tedious and error-prone. In many cases, these photos already contain precise GPS coordinates and timestamps embedded in their EXIF metadata. Furthermore, photos of fleeing or hidden pets are often taken hastily, resulting in poor lighting, low contrast, or incorrect orientation, making identification difficult.

**Milestone 8.2: EXIF Geolocation Auto-Extraction & AI Image Quality Enhancer** solves these friction points by introducing a seamless, privacy-first imaging pipeline. The system provides a pure Go EXIF reader to automatically extract geographic and chronological data to pre-fill report fields. Additionally, an on-the-fly Image Quality Analyzer evaluates underexposed photos and applies dynamic range normalization and orientation corrections. The entire process requires explicit user consent for applying location data and strips away sensitive PII metadata before final storage.

---

## 2. Architecture & Subsystem Boundaries

The imaging pipeline is decomposed into strict layers, prioritizing memory safety and separation of concerns:

```
                  ┌─────────────────────────────────────────────────────────┐
                  │                 Browser Clients / Mobile                │
                  │  (Dropzone, EXIF Pre-fill Prompt, Auto-Enhance Slider)  │
                  └────────────────────────────┬────────────────────────────┘
                                               │
                                       HTTP Multipart
                                               │
                  ┌────────────────────────────▼────────────────────────────┐
                  │           internal/app/webfrontend                      │
                  │  - POST /api/v1/images/extract-metadata (Rate-limited)  │
                  │  - POST /api/v1/images/enhance          (Rate-limited)  │
                  └──────────────┬───────────────────────────┬──────────────┘
                                 │                           │
                   In-memory byte parsing          Image byte processing
                                 │                           │
                  ┌──────────────▼────────────┐┌─────────────▼──────────────┐
                  │        pkg/imaging        ││        pkg/imaging         │
                  │     (Pure Go EXIF)        ││   (Quality & Enhancement)  │
                  └───────────────────────────┘└────────────────────────────┘
```

---

## 3. Domain Models & Core Engine Logic (`pkg/imaging`)

The `pkg/imaging` package handles robust image parsing and manipulation entirely in Go, avoiding external CGO dependencies (like libvips or ImageMagick) to ensure simplified cross-compilation and maximum security.

### 3.1 Pure Go EXIF Metadata Engine

- **Format Support:** Parses TIFF and JPEG EXIF header tags.
- **GPS Extraction:**
  - Reads `GPSLatitude`, `GPSLatitudeRef`, `GPSLongitude`, and `GPSLongitudeRef`.
  - Converts Degrees, Minutes, and Seconds (DMS) rational numbers to accurate `float64` decimal degrees.
- **Time Parsing:**
  - Extracts `DateTimeOriginal` and `DateTimeDigitized`.
  - Parses string timestamps to native `time.Time` structs in UTC.
- **Image Properties:**
  - Extracts image dimensions (`width` and `height`).
  - Reads `Orientation` (standard EXIF flags 1-8).

### 3.2 Image Quality Analyzer & Auto-Enhancer

- **Histogram Evaluation:** Computes a brightness histogram to evaluate global contrast.
- **Dynamic Range Normalization:** Applies contrast stretching to normalize underexposed or low-light pet photos, improving visibility of fur patterns and distinct markings.
- **Orientation Normalization:** Automatically rotates transposed or inverted images based on EXIF orientation flags, ensuring uniform display across all clients.
- **Memory Safety Guards:** Enforces strict limitations on maximum parsed image dimensions to prevent zero-division errors, out-of-memory panics, or decompression bombs.

---

## 4. Web Frontend Endpoints & REST API

The image extraction and enhancement logic is exposed via stateless HTTP endpoints in `internal/app/webfrontend`. Both endpoints are protected by `ratelimit.ModerateLimit`.

### 4.1 `POST /api/v1/images/extract-metadata`
- **Purpose:** Multipart upload endpoint that parses image bytes purely in memory to extract metadata. **Does not store the image permanently.**
- **Response:**
  ```json
  {
    "gps": {
      "latitude": 47.613,
      "longitude": -122.337
    },
    "captureTime": "2026-09-19T18:15:00Z",
    "orientation": 6,
    "width": 4032,
    "height": 3024
  }
  ```

### 4.2 `POST /api/v1/images/enhance`
- **Purpose:** Multipart upload endpoint that accepts a raw image, applies auto-contrast and low-light compensation, and returns the optimized image.
- **Response:**
  - Returns enhanced JPEG bytes.
  - Includes custom HTTP header: `X-Enhancement-Applied: true`.

---

## 5. Frontend UI & Interactive Visualization

The frontend seamlessly integrates the endpoints into existing workflows to streamline report creation.

### 5.1 Dropzone Integration
- Implemented within `#photo-dropzone` across:
  - `report-lost.html`
  - `report-found.html`
  - `#modal-report-sighting`
- **Behavior:** When a user selects or drops a photo, the client asynchronously inspects the metadata via the `/api/v1/images/extract-metadata` endpoint.

### 5.2 Intelligent Pre-fill Prompt
- If valid GPS coordinates and timestamps are found, the UI presents an actionable, non-intrusive toast or inline prompt:
  > *"📍 Found photo location: 47.61, -122.33 from 10 minutes ago. Pre-fill report? [Apply] / [Keep Manual]"*

### 5.3 Auto-Enhance UI
- Displays an optional **"✨ Auto-Enhance"** toggle button adjacent to the uploaded thumbnail.
- Upon toggling, calls `/api/v1/images/enhance`.
- Presents a side-by-side or sliding comparison view (before/after), allowing users to accept or discard the enhancement based on visual preference.

---

## 6. Zero-PII Wire Contract & Security

Privacy and strict data safety are paramount when handling user-provided media:

1. **Explicit Location Consent:** EXIF GPS coordinates are *never* applied to public-facing report fields automatically. The user must explicitly click "Apply" to opt-in to location sharing.
2. **Metadata Sanitization:** The system meticulously strips all sensitive device metadata (e.g., camera serial number, specific device model, software versions, and EXIF owner name tags) before saving the image to the permanent `petimages` storage.
3. **In-Memory Lifespan:** Temporary metadata extraction logic runs completely in-memory. If the user abandons the form, the extracted metadata and the raw photo evaporate immediately with no residual disk footprint.
4. **Memory and CPU Bounding:** Both the EXIF parser and the auto-enhancer utilize strict buffer limits and maximum image dimensions to mitigate malicious payloads.

---

## 7. Verification & Testing Strategy

1. **Unit Tests (`pkg/imaging`)**:
   - `exif_test.go`: Verify extraction logic using synthetic JPEG and TIFF byte fixtures. Assert accurate rational-to-decimal coordinate conversion and accurate timestamp parsing across timezones.
   - `enhance_test.go`: Assert successful contrast stretching, correct orientation rotation (testing all 8 EXIF flags), and proper bounds checking/panic recovery.
2. **Web Frontend Integration Tests (`internal/app/webfrontend/imaging_test.go`)**:
   - Verify `POST /api/v1/images/extract-metadata` handling of valid and invalid multipart payloads.
   - Assert `ratelimit.ModerateLimit` is correctly enforced.
   - Verify `POST /api/v1/images/enhance` returns the `X-Enhancement-Applied` header and valid JPEG bytes.
3. **Playwright E2E User Journey**:
   - Create mock device photo uploads to test the drag-and-drop flow.
   - Verify that the UI correctly displays the "📍 Found photo location" prompt.
   - Confirm that selecting "Apply" updates the map and form fields, and "Keep Manual" ignores the data.
   - Test the "✨ Auto-Enhance" slider interaction.
4. **Full Verification**:
   - Run `export GOTOOLCHAIN=go1.26.5 && make verify` to ensure clean execution across all static checks and test suites.
