# Milestone 5.2: Web Push Alert Preferences & Notification Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver an accessible in-app Notification Center drawer and an interactive Alert Preferences modal with multi-channel toggles (Email, SMS, Web Push) and geographic neighborhood alert zones on `/pets`.

**Architecture:** Extend backend store with `notificationPreferences` and `inAppNotifications` collections, expose authenticated & guest REST endpoints (`/api/v1/notifications`, `/api/v1/notifications/preferences`, `/api/v1/notifications/mark-read`), and implement an accessible sliding drawer and Leaflet mini-map zone picker on the frontend.

**Tech Stack:** Go 1.26, Leaflet.js 1.9.4 (vendored), Vanilla ES6+ JavaScript, CSS3 Glassmorphism, html/template.

**Spec:** [`docs/superpowers/specs/2026-09-13-web-push-alert-preferences-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-13-web-push-alert-preferences-design.md)

## Global Constraints

- Defense-in-depth CSP (`script-src 'self'`, `style-src 'self' https://fonts.googleapis.com`); zero external CDNs.
- Maintain OpenStreetMap tile image allowance (`https://*.tile.openstreetmap.org`).
- Enforce double-submit CSRF cookie checks on authenticated mutation endpoints (`POST /api/v1/notifications/mark-read`, `PUT /api/v1/notifications/preferences`).
- Support dual identity: authenticated users persist to database; guest visitors persist preferences locally (`localStorage`) and receive validated server responses.
- Every commit must adhere to Conventional Commits formatting (`feat(...)`, `fix(...)`, `test(...)`, `style(...)`).
- Must pass `make verify` (`go vet`, `golangci-lint`, `tofu validate`, `yamllint`, `go test -race -cover ./...`).

---

### Task 1: Store Collection Constants & Domain Models

**Files:**
- Modify: `pkg/store/names.go`
- Create: `pkg/domain/notification_preferences.go`
- Create: `pkg/domain/notification_preferences_test.go`

**Interfaces:**
- Consumes: `domain.LocationPoint`
- Produces: `store.NotificationPreferencesCollection`, `store.InAppNotificationsCollection`, `domain.NotificationPreferences`, `domain.InAppNotification`, `domain.NotificationsFeedResponse`, `domain.MarkNotificationsReadRequest`

- [ ] **Step 1: Write the failing tests in `pkg/domain/notification_preferences_test.go`**

```go
package domain

import (
	"testing"
	"time"
)

func TestNotificationPreferences_Validation(t *testing.T) {
	tests := []struct {
		name    string
		pref    NotificationPreferences
		wantErr bool
	}{
		{
			name: "valid email and geo zone",
			pref: NotificationPreferences{
				UserID:         "user-123",
				EmailEnabled:   true,
				Email:          "volunteer@example.com",
				GeoZoneEnabled: true,
				Coordinates:    LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
				RadiusMiles:    10.0,
			},
			wantErr: false,
		},
		{
			name: "invalid email when email enabled",
			pref: NotificationPreferences{
				UserID:       "user-123",
				EmailEnabled: true,
				Email:        "not-an-email",
			},
			wantErr: true,
		},
		{
			name: "invalid phone when sms enabled",
			pref: NotificationPreferences{
				UserID:     "user-123",
				SMSEnabled: true,
				Phone:      "12345", // missing E.164 + prefix
			},
			wantErr: true,
		},
		{
			name: "invalid coordinates when geo zone enabled",
			pref: NotificationPreferences{
				UserID:         "user-123",
				GeoZoneEnabled: true,
				Coordinates:    LocationPoint{Latitude: 120.0, Longitude: 0},
				RadiusMiles:    10.0,
			},
			wantErr: true,
		},
		{
			name: "non-positive radius when geo zone enabled",
			pref: NotificationPreferences{
				UserID:         "user-123",
				GeoZoneEnabled: true,
				Coordinates:    LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
				RadiusMiles:    0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pref.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInAppNotification_Defaults(t *testing.T) {
	now := time.Now().UTC()
	n := InAppNotification{
		ID:        "notif-1",
		UserID:    "user-123",
		Type:      "match",
		Title:     "New Match Found",
		Message:   "Potential match for Milo (92%)",
		CreatedAt: now,
		Read:      false,
	}

	if n.Read {
		t.Errorf("expected new notification to be unread")
	}
	if n.ReadAt != nil {
		t.Errorf("expected ReadAt to be nil initially")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./pkg/domain/notification_preferences_test.go ./pkg/domain/...`
Expected: FAIL with undefined `NotificationPreferences` or `InAppNotification`.

- [ ] **Step 3: Implement domain types and store constants**

1. In `pkg/store/names.go`, add:
```go
	NotificationPreferencesCollection = "notificationPreferences"
	InAppNotificationsCollection      = "inAppNotifications"
```

2. Create `pkg/domain/notification_preferences.go`:
```go
package domain

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var e164Regex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// NotificationPreferences defines user alert channels and geographic alert zone coordinates.
type NotificationPreferences struct {
	UserID         string        `json:"userId"`
	EmailEnabled   bool          `json:"emailEnabled"`
	Email          string        `json:"email,omitempty"`
	SMSEnabled     bool          `json:"smsEnabled"`
	Phone          string        `json:"phone,omitempty"`
	PushEnabled    bool          `json:"pushEnabled"`
	PushEndpoint   string        `json:"pushEndpoint,omitempty"`
	GeoZoneEnabled bool          `json:"geoZoneEnabled"`
	Coordinates    LocationPoint `json:"coordinates"`
	RadiusMiles    float64       `json:"radiusMiles"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

// Validate ensures preference fields meet format requirements.
func (p *NotificationPreferences) Validate() error {
	if p.EmailEnabled {
		if strings.TrimSpace(p.Email) == "" {
			return errors.New("email address is required when email alerts are enabled")
		}
		if _, err := mail.ParseAddress(p.Email); err != nil {
			return errors.New("invalid email address format")
		}
	}

	if p.SMSEnabled {
		trimmedPhone := strings.TrimSpace(p.Phone)
		if trimmedPhone == "" {
			return errors.New("phone number is required when SMS alerts are enabled")
		}
		if !e164Regex.MatchString(trimmedPhone) {
			return errors.New("phone number must be in valid E.164 format (e.g. +12065550100)")
		}
	}

	if p.GeoZoneEnabled {
		if err := p.Coordinates.Validate(); err != nil {
			return err
		}
		if p.RadiusMiles <= 0 || p.RadiusMiles > 100 {
			return errors.New("radiusMiles must be greater than 0 and less than or equal to 100")
		}
	}

	return nil
}

// InAppNotification represents an alert entry displayed in the sliding drawer.
type InAppNotification struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Type      string     `json:"type"` // "match", "broadcast", "status"
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Link      string     `json:"link,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	Read      bool       `json:"read"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
}

// NotificationsFeedResponse is the payload returned by GET /api/v1/notifications.
type NotificationsFeedResponse struct {
	Notifications []InAppNotification `json:"notifications"`
	UnreadCount   int                 `json:"unreadCount"`
}

// MarkNotificationsReadRequest is the payload for POST /api/v1/notifications/mark-read.
type MarkNotificationsReadRequest struct {
	All             bool     `json:"all"`
	NotificationIDs []string `json:"notificationIds,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./pkg/domain/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/store/names.go pkg/domain/notification_preferences.go pkg/domain/notification_preferences_test.go
git commit -m "feat(domain): add notification preferences and in-app notification domain models"
```

---

### Task 2: Backend REST Endpoints in `webfrontend`

**Files:**
- Create: `internal/app/webfrontend/notifications.go`
- Create: `internal/app/webfrontend/notifications_test.go`
- Modify: `internal/app/webfrontend/server.go:230`

**Interfaces:**
- Consumes: `domain.NotificationPreferences`, `domain.InAppNotification`, `store.NotificationPreferencesCollection`, `store.InAppNotificationsCollection`, `s.verifiedRequestPrincipal`
- Produces:
  - `GET /api/v1/notifications`
  - `POST /api/v1/notifications/mark-read`
  - `GET /api/v1/notifications/preferences`
  - `PUT /api/v1/notifications/preferences`

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/notifications_test.go`**

```go
package webfrontend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestNotifications_GuestFeedAndDefaultPreferences(t *testing.T) {
	srv := NewServer()

	// 1. GET /api/v1/notifications for guest returns 200 with feed
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/notifications returned %d, want 200", rec.Code)
	}

	var feed domain.NotificationsFeedResponse
	if err := json.NewDecoder(rec.Body).Decode(&feed); err != nil {
		t.Fatalf("failed to decode notifications feed: %v", err)
	}

	// 2. GET /api/v1/notifications/preferences for guest returns defaults
	reqPref := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/preferences", nil)
	recPref := httptest.NewRecorder()
	srv.ServeHTTP(recPref, reqPref)

	if recPref.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/notifications/preferences returned %d, want 200", recPref.Code)
	}

	var pref domain.NotificationPreferences
	if err := json.NewDecoder(recPref.Body).Decode(&pref); err != nil {
		t.Fatalf("failed to decode preferences: %v", err)
	}
	if pref.RadiusMiles <= 0 {
		t.Errorf("expected default radius to be positive, got %f", pref.RadiusMiles)
	}
}

func TestNotifications_PreferencesValidation(t *testing.T) {
	srv := NewServer()

	invalidPayload := domain.NotificationPreferences{
		EmailEnabled: true,
		Email:        "invalid-email",
	}
	bodyBytes, _ := json.Marshal(invalidPayload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid preferences returned %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestNotifications_" ./internal/app/webfrontend`
Expected: FAIL with 404 on unhandled endpoints.

- [ ] **Step 3: Implement handlers in `notifications.go` and register in `server.go`**

1. In `server.go`, register routes:
```go
	s.mux.HandleFunc("/api/v1/notifications", s.handleApiNotifications)
	s.mux.HandleFunc("/api/v1/notifications/mark-read", s.handleApiNotificationsMarkRead)
	s.mux.HandleFunc("/api/v1/notifications/preferences", s.handleApiNotificationsPreferences)
```

2. Create `internal/app/webfrontend/notifications.go` implementing:
   - `handleApiNotifications`: handles `GET /api/v1/notifications`. If authenticated, queries user notifications and counts unread items; if guest, provides sample/broadcast items.
   - `handleApiNotificationsMarkRead`: handles `POST /api/v1/notifications/mark-read`. Validates request, updates `Read: true` and `ReadAt: now` in `store.InAppNotificationsCollection`.
   - `handleApiNotificationsPreferences`: handles `GET` and `PUT`. Validates payload using `pref.Validate()`. If authenticated, persists in `store.NotificationPreferencesCollection`. Returns validated JSON response.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race -run "TestNotifications_" ./internal/app/webfrontend`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/notifications.go internal/app/webfrontend/notifications_test.go internal/app/webfrontend/server.go
git commit -m "feat(api): implement notification feed and alert preferences REST endpoints"
```

---

### Task 3: Template Markup for Notification Bell, Drawer, and Preferences Modal

**Files:**
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/notifications_test.go`

**Interfaces:**
- Consumes: `#btn-notification-drawer`, `#notification-badge`, `#notification-drawer`, `#notification-preferences-modal`
- Produces: Rendered HTML containing notification trigger, slide-over drawer structure, and preferences modal.

- [ ] **Step 1: Write failing test in `notifications_test.go`**

```go
func TestNotifications_TemplateElementsPresent(t *testing.T) {
	srv := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/pets", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	requiredElements := []string{
		`id="btn-notification-drawer"`,
		`id="notification-badge"`,
		`id="notification-drawer"`,
		`id="notification-list"`,
		`id="notification-preferences-modal"`,
		`id="zone-mini-map"`,
		`/static/js/notification-center.js`,
	}

	for _, elem := range requiredElements {
		if !strings.Contains(body, elem) {
			t.Errorf("expected pets.html to contain %s", elem)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestNotifications_TemplateElementsPresent" ./internal/app/webfrontend`
Expected: FAIL with missing elements.

- [ ] **Step 3: Update `templates/pets.html`**

1. In `<div class="nav-actions">`, add notification bell button:
```html
        <button id="btn-notification-drawer" class="btn-icon notification-bell-btn" aria-label="Open notifications" aria-expanded="false" title="Notifications">
          <svg class="bell-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
            <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
          </svg>
          <span id="notification-badge" class="badge-unread hidden" aria-live="polite">0</span>
        </button>
```
2. Below `<main>`, add `#notification-drawer` and backdrop:
```html
  <div id="drawer-backdrop" class="drawer-backdrop hidden" tabindex="-1"></div>
  <aside id="notification-drawer" class="notification-drawer" role="dialog" aria-label="Notifications" aria-modal="true" aria-hidden="true">
    <div class="drawer-header">
      <div class="drawer-title-group">
        <h2>Notifications</h2>
        <span id="drawer-unread-count" class="badge badge-secondary">0 unread</span>
      </div>
      <div class="drawer-actions">
        <button type="button" class="btn btn-ghost btn-sm" id="btn-mark-all-read" title="Mark all as read">✓ Read all</button>
        <button type="button" class="btn btn-ghost btn-sm" id="btn-open-preferences" title="Notification preferences">⚙️ Settings</button>
        <button type="button" class="btn btn-icon btn-sm" id="btn-close-drawer" aria-label="Close notifications">✕</button>
      </div>
    </div>
    <div class="drawer-content" id="notification-list-container">
      <div id="notification-list" class="notification-list" role="feed" aria-busy="false"></div>
      <div id="notification-empty" class="empty-state notification-empty hidden">
        <div class="empty-icon" aria-hidden="true">🐾</div>
        <p class="empty-state-title">You're all caught up!</p>
        <p class="text-secondary">New match alerts and neighborhood updates will appear here.</p>
      </div>
    </div>
  </aside>
```
3. Add `#notification-preferences-modal` and backdrop:
```html
  <div id="preferences-backdrop" class="modal-backdrop hidden" tabindex="-1"></div>
  <div id="notification-preferences-modal" class="glass-card preferences-modal hidden" role="dialog" aria-modal="true" aria-labelledby="pref-modal-title">
    <div class="modal-header">
      <h2 id="pref-modal-title">Alert & Zone Preferences</h2>
      <button type="button" class="btn-icon" id="btn-close-preferences" aria-label="Close settings">✕</button>
    </div>
    <form id="preferences-form" class="preferences-form">
      <fieldset class="pref-fieldset">
        <legend class="pref-legend">Delivery Channels</legend>
        <div class="pref-channel-row">
          <label class="pref-toggle-label">
            <input type="checkbox" id="pref-email-enabled" name="emailEnabled">
            <span>Email Alerts</span>
          </label>
          <input type="email" id="pref-email-address" name="email" class="form-control" placeholder="your.email@example.com">
        </div>
        <div class="pref-channel-row">
          <label class="pref-toggle-label">
            <input type="checkbox" id="pref-sms-enabled" name="smsEnabled">
            <span>SMS Text Alerts</span>
          </label>
          <input type="tel" id="pref-phone-number" name="phone" class="form-control" placeholder="+12065550199">
        </div>
        <div class="pref-channel-row">
          <label class="pref-toggle-label">
            <input type="checkbox" id="pref-push-enabled" name="pushEnabled">
            <span>Instant Web Push</span>
          </label>
          <button type="button" class="btn btn-secondary btn-sm" id="btn-enable-push">Enable Push Alerts</button>
        </div>
      </fieldset>
      <fieldset class="pref-fieldset">
        <legend class="pref-legend">Neighborhood Alert Zone</legend>
        <label class="pref-toggle-label">
          <input type="checkbox" id="pref-geozone-enabled" name="geoZoneEnabled" checked>
          <span>Notify me of lost pets reported in my neighborhood</span>
        </label>
        <div class="pref-geo-controls">
          <button type="button" class="btn btn-secondary btn-sm" id="btn-pref-geolocation">📍 Use My Location</button>
          <label for="pref-radius">Alert Radius:</label>
          <select id="pref-radius" name="radiusMiles" class="form-control">
            <option value="5">5 miles</option>
            <option value="10" selected>10 miles</option>
            <option value="25">25 miles</option>
            <option value="50">50 miles</option>
          </select>
        </div>
        <input type="hidden" id="pref-lat" name="lat">
        <input type="hidden" id="pref-lng" name="lng">
        <div id="zone-mini-map" class="zone-mini-map" role="region" aria-label="Interactive alert zone mini map"></div>
      </fieldset>
      <div class="modal-footer">
        <button type="button" class="btn btn-secondary" id="btn-cancel-preferences">Cancel</button>
        <button type="submit" class="btn btn-primary" id="btn-save-preferences">Save Preferences</button>
      </div>
    </form>
  </div>
```
4. Link script at bottom:
```html
  <script src="/static/js/push-notifier.js"></script>
  <script src="/static/js/notification-center.js"></script>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run "TestNotifications_TemplateElementsPresent" ./internal/app/webfrontend`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/pets.html internal/app/webfrontend/notifications_test.go
git commit -m "feat(frontend): add notification bell, sliding drawer, and preferences modal markup"
```

---

### Task 4: CSS Styles for Sliding Drawer, Badges, Cards & Preferences Modal

**Files:**
- Modify: `internal/app/webfrontend/static/css/styles.css`

**Interfaces:**
- Consumes: HTML class names `.notification-bell-btn`, `.badge-unread`, `.notification-drawer`, `.drawer-backdrop`, `.notification-item`, `.preferences-modal`, `.zone-mini-map`
- Produces: Smooth sliding drawer animations, glassmorphic styling, responsive layouts, and modal transitions.

- [ ] **Step 1: Add CSS rules to `internal/app/webfrontend/static/css/styles.css`**

```css
/* Notification Bell & Unread Badge */
.notification-bell-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.5rem;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  transition: all 0.2s ease;
}

.notification-bell-btn:hover {
  color: var(--text-primary);
  background: var(--bg-card);
}

.badge-unread {
  position: absolute;
  top: 0;
  right: 0;
  background: var(--accent-primary);
  color: #ffffff;
  font-size: 0.72rem;
  font-weight: 700;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 2px solid var(--bg-surface);
}

/* Sliding Notification Drawer */
.drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  z-index: 1000;
  transition: opacity 0.3s ease;
}

.notification-drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 100%;
  max-width: 420px;
  background: var(--bg-surface);
  border-left: 1px solid var(--border-glass);
  box-shadow: -10px 0 30px rgba(0, 0, 0, 0.5);
  z-index: 1001;
  display: flex;
  flex-direction: column;
  transform: translateX(100%);
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.notification-drawer.is-open {
  transform: translateX(0);
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--border-glass);
  background: var(--bg-card);
}

.drawer-title-group {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.drawer-title-group h2 {
  font-size: 1.25rem;
  margin: 0;
}

.drawer-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.drawer-content {
  flex: 1;
  overflow-y: auto;
  padding: 1rem 1.25rem;
}

.notification-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.notification-item {
  display: block;
  text-decoration: none;
  background: var(--bg-card);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 0.85rem 1rem;
  color: var(--text-primary);
  transition: all 0.2s ease;
}

.notification-item:hover {
  border-color: var(--accent-primary);
  transform: translateY(-1px);
}

.notification-item.is-unread {
  border-left: 3px solid var(--accent-primary);
}

.notif-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.35rem;
}

.notif-badge-type {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.2rem 0.5rem;
  border-radius: var(--radius-sm);
}

.type-match { background: rgba(245, 158, 11, 0.15); color: #f59e0b; }
.type-broadcast { background: rgba(59, 130, 246, 0.15); color: #3b82f6; }
.type-status { background: rgba(16, 185, 129, 0.15); color: #10b981; }

.notif-time {
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.notif-title {
  font-size: 0.95rem;
  font-weight: 600;
  margin: 0 0 0.25rem 0;
}

.notif-msg {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.4;
}

/* Preferences Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  z-index: 1050;
}

.preferences-modal {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 90%;
  max-width: 580px;
  max-height: 90vh;
  overflow-y: auto;
  z-index: 1051;
  padding: 1.75rem;
  border-radius: var(--radius-lg);
}

.pref-fieldset {
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 1rem;
  margin-bottom: 1.25rem;
}

.pref-legend {
  font-weight: 600;
  padding: 0 0.5rem;
  color: var(--text-primary);
}

.pref-channel-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}

.pref-channel-row .form-control {
  max-width: 260px;
}

.pref-toggle-label {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
  cursor: pointer;
}

.pref-geo-controls {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 0.75rem 0;
  flex-wrap: wrap;
}

.zone-mini-map {
  width: 100%;
  height: 240px;
  border-radius: var(--radius-sm);
  background: var(--bg-surface);
  margin-top: 0.75rem;
  border: 1px solid var(--border-glass);
}
```

- [ ] **Step 2: Run verification**

Run: `go test ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/css/styles.css
git commit -m "style(frontend): add styles for notification bell, sliding drawer, and preferences modal"
```

---

### Task 5: Client-Side Notification Center & Zone Mini-Map Controller

**Files:**
- Create: `internal/app/webfrontend/static/js/notification-center.js`

**Interfaces:**
- Consumes: `GET /api/v1/notifications`, `POST /api/v1/notifications/mark-read`, `GET /api/v1/notifications/preferences`, `PUT /api/v1/notifications/preferences`, Leaflet `L` global object
- Produces: Dynamic notification badge updates, drawer open/close animations, mark all as read action, preferences modal open/save, and interactive Leaflet mini-map with click-to-pin and radius circle visualization.

- [ ] **Step 1: Implement `internal/app/webfrontend/static/js/notification-center.js`**

Implement:
1. `initNotificationDrawer()`:
   - Wire `#btn-notification-drawer` to toggle `#notification-drawer` (add/remove `.is-open`) and `#drawer-backdrop`.
   - Update `aria-expanded` and `aria-hidden`.
   - Wire close button (`#btn-close-drawer`) and backdrop click to close.
   - Wire `Escape` key to close drawer.
2. `fetchNotifications()`:
   - Request `GET /api/v1/notifications`.
   - Update `#notification-badge` with unread count; if > 0 remove `.hidden`, else add `.hidden`.
   - Render notification cards into `#notification-list`. If empty, display `#notification-empty`.
3. `initMarkAllRead()`:
   - Wire `#btn-mark-all-read`: sends `POST /api/v1/notifications/mark-read` with CSRF header, updates UI cards to read, sets unread badge to 0.
4. `initPreferencesModal()`:
   - Wire `#btn-open-preferences` to show `#notification-preferences-modal` and `#preferences-backdrop`.
   - Lazily initialize Leaflet mini-map `#zone-mini-map`.
   - In mini-map: draw center pin and `L.circle` proximity radius. Clicking anywhere on the mini-map updates `#pref-lat`, `#pref-lng`, moves the center marker, and redraws the radius circle.
   - Wire `#btn-pref-geolocation`: uses `navigator.geolocation.getCurrentPosition` to set `#pref-lat` / `#pref-lng` and re-center mini-map.
   - Wire form submission (`#preferences-form`): sends `PUT /api/v1/notifications/preferences`. If authenticated, saves to server; if guest, validates and saves to `localStorage.setItem('petspotr_alert_preferences', ...)`.
   - Connect `#btn-enable-push` with `push-notifier.js` subscription flow.

- [ ] **Step 2: Syntax and Go tests verification**

Run: `node -c internal/app/webfrontend/static/js/notification-center.js && go test -race ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/js/notification-center.js
git commit -m "feat(frontend): implement notification center drawer and alert preferences client controller"
```

---

### Task 6: Full Verification & Integration Validation

**Files:**
- Run complete test suite and linters across the entire workspace.

- [ ] **Step 1: Execute `make verify`**

Run: `make verify`
Expected:
- `go vet ./...` exits 0.
- `golangci-lint run` exits 0.
- `tofu validate` exits 0.
- `yamllint .` exits 0.
- `go test -race -cover ./...` passes 100%.

- [ ] **Step 2: Verify git status and clean working tree**

Run: `git status`
Expected: Clean working tree on `feat/notification-dashboard-alert-preferences`.
