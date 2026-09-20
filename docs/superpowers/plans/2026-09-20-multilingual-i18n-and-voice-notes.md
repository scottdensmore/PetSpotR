# Milestone 8.4: Multilingual Localization (i18n), Voice Sighting Notes & WCAG AAA Accessibility Implementation Plan

- **Target Branch:** `feat/i18n-voice-notes`
- **Worktree:** `.worktrees/track-a`

---

## Proposed Tasks

### Task 1: Pure Go Localization Catalog & Pluralization Engine (`pkg/i18n`)
- Implement `pkg/i18n/catalog.go` and `pkg/i18n/catalog_test.go`.
- Define dictionaries for `en`, `es`, `vi`, `zh-CN`, `tl`.
- Provide `Translate`, `TranslatePlural`, `ResolveLocale`, and fallback handling.
- Verify 100% test coverage on locale parsing and key interpolation.

### Task 2: Voice Memo Audio Ingestion & Waveform Analysis (`pkg/imaging` or `pkg/audio`)
- Implement `pkg/audio/voicememo.go` and `pkg/audio/voicememo_test.go`.
- MIME type validation, audio byte header check, duration calculation, and mockable/pure Go amplitude envelope extraction.
- Unit tests validating max duration rejection, size bounds, and valid waveform normalization.

### Task 3: Web Frontend Middleware, Template FuncMap & REST Endpoints (`internal/app/webfrontend`)
- Add locale detection middleware in `webfrontend` reading query param `lang`, cookie `petspotr_locale`, or `Accept-Language`.
- Add template helper `{{ t .Locale "key" }}` in HTML renderer.
- Implement `POST /api/v1/lost-pets/{petID}/sightings/{id}/voice-memo` and `GET /api/v1/lost-pets/{petID}/sightings/{id}/voice-memo`.
- Write unit tests in `internal/app/webfrontend/i18n_test.go` and `internal/app/webfrontend/voice_memo_test.go`.

### Task 4: Frontend UI Widgets: Language Switcher, Voice Recorder & ARIA Announcer
- Update `internal/app/webfrontend/templates/base.html` with Language Picker and `#aria-announcer`.
- Implement `static/js/audio-memo.js` managing `navigator.mediaDevices.getUserMedia` and `MediaRecorder`.
- Add responsive waveform player styles and WCAG high-contrast focus rings in `styles.css`.

### Task 5: Playwright End-to-End User Journey Tests & Full Verification
- Create `tests/playwright/e2e/i18n-voice-notes-journey.spec.ts`.
- Test language switching on `/pets` and `/p/{id}`, localized strings, voice memo recording/playback, and ARIA announcements.
- Run `make verify` ensuring all linters and tests pass.
