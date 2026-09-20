# Milestone 8.4: Multilingual Localization (i18n), Voice Sighting Notes & WCAG AAA Accessibility Design Specification

- **Date:** 2026-09-20
- **Status:** Approved
- **Author:** Antigravity (Pair Programming Assistant)
- **Target Release:** PetSpotR Milestone 8.4

---

## 1. Executive Summary & Problem Statement

PetSpotR serves diverse urban communities where timely reporting of lost pets and sightings is critical. Language barriers and physical mobility limitations can significantly delay sightings. Furthermore, typing extensive text descriptions on mobile devices while tracking an animal on the run is cumbersome and hazardous.

**Milestone 8.4: Multilingual Localization (i18n), Voice Sighting Notes & WCAG AAA Accessibility** solves these issues:
1. **Pure Go Localization Engine (`pkg/i18n`):** Fast, zero-dependency translation catalog supporting English (`en`), Spanish (`es`), Vietnamese (`vi`), Simplified Chinese (`zh-CN`), and Tagalog (`tl`). Resolves language via query param (`?lang=`), cookie (`petspotr_locale`), or `Accept-Language` headers.
2. **Audio Voice Memo Sighting Ingestion:** Uses browser `MediaRecorder` API to capture up to 15 seconds of audio, uploading to the web frontend with MIME type validation, audio duration verification, and waveform amplitude extraction.
3. **WCAG 2.1 AAA Accessibility Enhancements:** Implements an ARIA live region announcer for real-time SSE events (search party claims, sighting alerts), full keyboard traversability for modals and maps, high-contrast modes, and visible focus indicators.

---

## 2. Architecture & Subsystem Boundaries

```text
               ┌───────────────────────────────────────────────────────────┐
               │                  Browser Client / Mobile                  │
               │  - Language Selector Dropdown                             │
               │  - HTML5 MediaRecorder (15s voice memo)                   │
               │  - ARIA Live Announcer (#aria-announcer)                  │
               └─────────────────────────────┬─────────────────────────────┘
                                             │
                                     HTTP / REST / Audio
                                             │
               ┌─────────────────────────────▼─────────────────────────────┐
               │               internal/app/webfrontend                    │
               │  - Middleware: Locale Resolution (Query, Cookie, Header)  │
               │  - POST /api/v1/lost-pets/{petID}/sightings/{id}/voice-memo │
               │  - GET  /api/v1/lost-pets/{petID}/sightings/{id}/voice-memo │
               │  - Template FuncMap: {{ t .Locale "key" }}                │
               └──────────────┬─────────────────────────────┬──────────────┘
                              │                             │
               ┌──────────────▼────────────┐  ┌─────────────▼──────────────┐
               │         pkg/i18n          │  │       pkg/blob / Store     │
               │  - Catalog & Pluralization│  │  - Audio Blob Persistence  │
               │  - Safe String Templates  │  │  - Waveform Metadata       │
               └───────────────────────────┘  └────────────────────────────┘
```

---

## 3. Data Models & API Contracts

### 3.1 `pkg/i18n` Catalog
- Locales: `en` (default), `es`, `vi`, `zh-CN`, `tl`.
- Fallback: Any unknown locale falls back to `en`.
- Translation Function: `Translate(locale, key string, args ...any) string`.
- Pluralization: `TranslatePlural(locale, key string, count int, args ...any) string`.

### 3.2 Voice Memo Endpoints
- `POST /api/v1/lost-pets/{petID}/sightings/{sightingID}/voice-memo`:
  - Request: `multipart/form-data` with `audio` file.
  - Allowed content-types: `audio/webm`, `audio/ogg`, `audio/mp4`, `audio/wav`.
  - Max size: 5 MB.
  - Generates 30-sample amplitude array (`waveform`) normalized between 0.0 and 1.0.
  - Returns: `201 Created` with `{"voiceMemoUrl": "...", "durationSeconds": 14.2, "waveform": [0.1, 0.4, ...]}`.
- `GET /api/v1/lost-pets/{petID}/sightings/{sightingID}/voice-memo`:
  - Streams stored audio with proper `Content-Type` and `Accept-Ranges: bytes`.

---

## 4. UI & Accessibility Enhancements

1. **Language Selector:** Seamless selector in navigation header that updates preference cookie and reloads localized content.
2. **Audio Voice Memo Widget:** Record/Stop button with 15-second progress bar, audio preview player, and fallback text note.
3. **WCAG 2.1 AAA Live Announcer:** Hidden `<div id="aria-announcer" aria-live="polite" aria-atomic="true">` receiving dynamic text announcements on real-time events.
