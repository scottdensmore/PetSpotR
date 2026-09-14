# Milestone 5.2: Web Push Alert Preferences & Notification Dashboard Design Spec

- **Author**: Antigravity & Scott Densmore
- **Date**: 2026-09-13
- **Status**: Proposed
- **Target Epic**: Milestone 5.2 (`docs/ROADMAP.md`)

---

## 1. Context & Objectives

PetSpotR's backend includes multi-channel notification dispatchers (`internal/app/notification`) supporting Email (SendGrid), SMS (Twilio), and Web Push (VAPID), along with a radius-based `GeoBroadcastEngine`. However, users currently lack a unified frontend interface to:
1. Manage their alert channel preferences (Email, SMS, Web Push).
2. Configure a personalized geographic alert zone (e.g. radius of 5, 10, 25 miles around their home or neighborhood).
3. View and acknowledge incoming match notifications, broadcast alerts, and report status updates in an in-app Notification Center.

### 1.1 Goals
1. **In-App Notification Center Drawer**: Add an accessible sliding drawer opened from a header notification bell icon with a real-time unread badge counter.
2. **Alert Channel Preferences**: Provide a dedicated settings panel enabling users to toggle Email, SMS, and Web Push notifications with input validation.
3. **Geographic Alert Zone with Interactive Mini-Map**: Allow users to set an alert center (via "📍 Use My Location" or clicking on an embedded Leaflet mini-map) and choose an alert radius (5, 10, 25, 50 miles) with an interactive proximity circle overlay.
4. **Dual Identity Support**:
   - **Authenticated Users**: Save alert preferences and in-app notifications directly to the database tied to their user account.
   - **Guest Volunteers**: Allow anonymous visitors to enable Web Push and configure local neighborhood alert zones stored on their device (`localStorage`), associating their push subscription with the chosen alert radius.
5. **Strict Security & CSP Compliance**: Keep all scripts, styles, and templates origin-bound, enforcing double-submit CSRF protection on authenticated preference mutations.

### 1.2 Non-Goals
- Real-time WebSockets or Server-Sent Events (standard HTTP polling/fetching upon page load and drawer interaction satisfies scalability and cost constraints).
- Replacing existing third-party delivery providers (SendGrid, Twilio).

---

## 2. Architecture & Data Flow

```
+---------------------------------------------------------------------------------+
| Browser Client                                                                  |
|                                                                                 |
|   +-------------------------------------------------------------------------+   |
|   | Navigation Bar                                                          |   |
|   | [ Brand ]   [ Directory ]   [ Matches ]   [ 🔔 (Unread: 2) ] [ Theme ]  |   |
|   +--------------------------------------------------+----------------------+   |
|                                                      |                          |
|                                           (Click Bell opens Drawer)             |
|                                                      v                          |
|   +--------------------------------------------------+----------------------+   |
|   | Sliding Notification Drawer (#notification-drawer)                      |   |
|   | - Unread count badge                                                    |   |
|   | - Action buttons: [ ✓ Mark all read ]  [ ⚙️ Preferences ]               |   |
|   | - Feed items:                                                           |   |
|   |   * 🐾 92% Match Found: "Milo (Beagle)" -> links to match                |   |
|   |   * 📍 Lost Pet Alert in your zone: "Bella (Lab)" -> links to report     |   |
|   |   * ✓ Report Resolved: "Shadow (Cat)" reunited                          |   |
|   +--------------------------------------------------+----------------------+   |
|                                                      |                          |
|                                           (Click Settings opens Modal)          |
|                                                      v                          |
|   +-------------------------------------------------------------------------+   |
|   | Notification Preferences Modal (#notification-preferences-modal)        |   |
|   | - Channels: [x] Email   [x] SMS   [x] Web Push (🔔 Enable Push)         |   |
|   | - Neighborhood Zone: [ 📍 Use My Location ]  Radius: [ 10 miles v ]     |   |
|   | - Embedded Leaflet Mini-Map (#zone-mini-map) with blue radius circle     |   |
|   | - [ Save Preferences ]                                                  |   |
|   +-------------------------------------------------------------------------+   |
|            |                                            |                       |
|   (Auth: PUT /api/v1/notifications/preferences)         | (Guest: localStorage) |
|            v                                            v                       |
+------------+--------------------------------------------+-----------------------+
             |
             v
+---------------------------------------------------------------------------------+
| Go Web Frontend Service (server.go / notifications.go)                          |
| - Verified Principal & CSRF Validation                                          |
| - store.NotificationPreferencesCollection                                       |
| - store.InAppNotificationsCollection                                            |
| - store.PushSubscriptionsCollection                                             |
+---------------------------------------------------------------------------------+
```

---

## 3. Data Models & API Specifications

### 3.1 State Collections in `pkg/store/names.go`
```go
const (
    NotificationPreferencesCollection = "notificationPreferences"
    InAppNotificationsCollection      = "inAppNotifications"
)
```

### 3.2 Domain Models
```go
// NotificationPreferences defines user alert channels and geographic alert zone coordinates.
type NotificationPreferences struct {
    UserID         string               `json:"userId"`
    EmailEnabled   bool                 `json:"emailEnabled"`
    Email          string               `json:"email,omitempty"`
    SMSEnabled     bool                 `json:"smsEnabled"`
    Phone          string               `json:"phone,omitempty"`
    PushEnabled    bool                 `json:"pushEnabled"`
    PushEndpoint   string               `json:"pushEndpoint,omitempty"`
    GeoZoneEnabled bool                 `json:"geoZoneEnabled"`
    Coordinates    domain.LocationPoint `json:"coordinates"`
    RadiusMiles    float64              `json:"radiusMiles"`
    UpdatedAt      time.Time            `json:"updatedAt"`
}

// InAppNotification represents a notification entry displayed in the sliding drawer.
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

### 3.3 REST Endpoints

1. **`GET /api/v1/notifications`**:
   - Query: `?lat=&lng=&radiusMiles=` (optional for guests).
   - If authenticated (`verifiedRequestPrincipal`):
     - Loads notifications from `store.InAppNotificationsCollection` where `UserID == principal.Subject`.
     - Computes `UnreadCount`.
   - If guest:
     - Returns public community alerts matching the provided coordinates and radius, plus seeded onboarding notifications.
   - Status: `200 OK`.

2. **`POST /api/v1/notifications/mark-read`**:
   - Payload: `{"all": true}` or `{"notificationIds": ["..."]}`.
   - For authenticated users: updates `Read: true` and `ReadAt: now` in `store.InAppNotificationsCollection`.
   - Status: `200 OK` (`{"status": "ok", "markedCount": N}`).

3. **`GET /api/v1/notifications/preferences`**:
   - For authenticated users: loads preferences from `store.NotificationPreferencesCollection`. If none saved, returns defaults (`EmailEnabled: true`, `Email: principal.Email`, `RadiusMiles: 10.0`).
   - For guests: returns default preference structure.
   - Status: `200 OK`.

4. **`PUT /api/v1/notifications/preferences`**:
   - Headers: `Content-Type: application/json`, `X-CSRF-Token` (for authenticated users).
   - Validation:
     - If `EmailEnabled == true`, validates non-empty email syntax.
     - If `SMSEnabled == true`, validates E.164 phone format (`^\+[1-9]\d{1,14}$`).
     - If `GeoZoneEnabled == true`, validates `Coordinates` (`lat [-90, 90]`, `lng [-180, 180]`) and `RadiusMiles > 0 && RadiusMiles <= 100`.
   - If authenticated: saves to `store.NotificationPreferencesCollection`.
   - If guest: returns validated JSON payload for `localStorage` saving.
   - Status: `200 OK`.

---

## 4. Frontend UI & Component Architecture

### 4.1 Header Trigger (`templates/pets.html` and common header)
- Rendered in `.nav-actions`:
  ```html
  <button id="btn-notification-drawer" class="btn-icon notification-bell-btn" aria-label="Open notifications" aria-expanded="false" title="Notifications">
    <svg class="bell-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
      <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
    </svg>
    <span id="notification-badge" class="badge-unread hidden" aria-live="polite">0</span>
  </button>
  ```

### 4.2 Sliding Drawer (`#notification-drawer`)
- HTML structure with backdrop:
  - Header: title, unread count pill, "✓ Read all" button, "⚙️ Settings" button, and "✕" close button.
  - Content: feed of notification items (`.notification-item`), with status icons:
    - 🐾 **Match**: Amber icon and badge, linking to `/matches`.
    - 📍 **Broadcast**: Azure icon and badge, linking to `/pets?lat=...&lng=...`.
    - ✓ **Status**: Emerald icon and badge.
  - Empty state (`#notification-empty`): *"You're all caught up! 🐾"*.

### 4.3 Preferences Modal (`#notification-preferences-modal`)
- Settings dialog opened from drawer:
  - Channel switches: Email, SMS (with phone input), and Web Push (with "Enable Instant Push" button integrating `push-notifier.js`).
  - Neighborhood zone:
    - Checkbox: *"Receive alerts for lost pets in my neighborhood"*
    - "📍 Use My Location" button (requests GPS coordinates via `navigator.geolocation`)
    - Radius selector: `5 miles`, `10 miles` (default), `25 miles`, `50 miles`.
    - Embedded Leaflet mini-map (`#zone-mini-map`, height: 260px) with interactive center pin and dynamic `L.circle` radius overlay. Clicking anywhere on the mini-map updates the alert center and moves the radius circle.
  - "Save Preferences" button (`#btn-save-preferences`).

### 4.4 Client Controller (`static/js/notification-center.js`)
- Initializes on `DOMContentLoaded`.
- Fetches `GET /api/v1/notifications` on page load; updates badge count and populates drawer.
- Manages drawer open/close animations, keyboard traps, and `Escape` key handling.
- Manages preferences modal open/close and lazy initialization of the `#zone-mini-map` Leaflet instance with `map.invalidateSize()`.
- Synchronizes with `push-notifier.js` for Web Push subscription registration.

---

## 5. Security & Progressive Enhancement

1. **Strict CSP Adherence**:
   - All script and style execution remains origin-local (`script-src 'self'`, `style-src 'self' https://fonts.googleapis.com`).
   - Leaflet mini-map tiles load from `https://*.tile.openstreetmap.org` as permitted in `img-src`.
   - Zero inline `style="..."` attributes or inline `onclick` handlers.
2. **CSRF & Authentication**:
   - Double-submit CSRF cookie validation applied to `POST /api/v1/notifications/mark-read` and `PUT /api/v1/notifications/preferences`.
   - Verified principal scoping ensures users can only read and modify their own notifications and preferences.
3. **Graceful Fallbacks**:
   - If JavaScript is disabled, navigation continues to function normally.
   - If geolocation is denied, users can set coordinates by clicking on the mini-map.

---

## 6. Testing & Verification Strategy

### 6.1 Backend Tests (`internal/app/webfrontend/notifications_test.go`)
1. **`TestNotifications_ListFeed`**: Verify feed retrieval, JSON schema, and unread count for authenticated users and guests.
2. **`TestNotifications_MarkRead`**: Verify marking specific notifications and all notifications as read.
3. **`TestNotifications_PreferencesValidation`**: Verify validation rules for email, phone (E.164), coordinates, and radius limits.
4. **`TestNotifications_PreferencesPersistence`**: Verify persistence in `store.NotificationPreferencesCollection` with CSRF checks.

### 6.2 Frontend & Full Verification
1. Verify template element rendering in `directory_test.go` and `server_test.go`.
2. Syntax check `notification-center.js` via `node -c`.
3. Run comprehensive `make verify` (`go vet`, `golangci-lint`, `tofu validate`, `yamllint`, `go test -race -cover ./...`).
