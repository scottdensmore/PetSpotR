package webfrontend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
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
	if len(feed.Notifications) == 0 {
		t.Fatal("expected at least one onboarding/community notification for guest")
	}
	if feed.UnreadCount != len(feed.Notifications) {
		t.Fatalf("guest unreadCount = %d, want %d", feed.UnreadCount, len(feed.Notifications))
	}

	// 2. GET /api/v1/notifications/preferences for guest returns defaults
	reqPref := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/preferences", nil)
	recPref := httptest.NewRecorder()
	srv.ServeHTTP(recPref, reqPref)

	if recPref.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/notifications/preferences returned %d, want 200", recPref.Code)
	}
	if got := recPref.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("GET preferences Cache-Control = %q, want %q", got, "no-store")
	}

	var pref domain.NotificationPreferences
	if err := json.NewDecoder(recPref.Body).Decode(&pref); err != nil {
		t.Fatalf("failed to decode preferences: %v", err)
	}
	if pref.RadiusMiles <= 0 {
		t.Errorf("expected default radius to be positive, got %f", pref.RadiusMiles)
	}

	// 3. POST /api/v1/notifications/mark-read for guest returns 200 with markedCount: 0
	markReq := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/mark-read", bytes.NewReader([]byte(`{"all":true}`)))
	markReq.Header.Set("Content-Type", "application/json")
	markRec := httptest.NewRecorder()
	srv.ServeHTTP(markRec, markReq)

	if markRec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/notifications/mark-read for guest returned %d, want 200", markRec.Code)
	}
	var markResp map[string]any
	if err := json.NewDecoder(markRec.Body).Decode(&markResp); err != nil {
		t.Fatalf("failed to decode mark-read response: %v", err)
	}
	if markResp["status"] != "ok" || int(markResp["markedCount"].(float64)) != 0 {
		t.Fatalf("unexpected mark-read response for guest: %#v", markResp)
	}

	// 4. PUT /api/v1/notifications/preferences for guest returns 200 with validated payload and blank userId
	validPref := domain.NotificationPreferences{
		UserID:      "spoofed-user-id",
		RadiusMiles: 15.0,
	}
	prefBytes, _ := json.Marshal(validPref)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader(prefBytes))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/notifications/preferences for guest returned %d, want 200", putRec.Code)
	}
	if got := putRec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("PUT preferences Cache-Control = %q, want %q", got, "no-store")
	}
	var putResult domain.NotificationPreferences
	if err := json.NewDecoder(putRec.Body).Decode(&putResult); err != nil {
		t.Fatalf("failed to decode put preferences: %v", err)
	}
	if putResult.RadiusMiles != 15.0 {
		t.Errorf("expected radius 15.0, got %f", putResult.RadiusMiles)
	}
	if putResult.UserID != "" {
		t.Errorf("expected guest UserID to be blanked, got %q", putResult.UserID)
	}
}

func TestNotifications_PreferencesValidation(t *testing.T) {
	srv := NewServer()

	tests := []struct {
		name    string
		payload domain.NotificationPreferences
	}{
		{
			name: "invalid email syntax",
			payload: domain.NotificationPreferences{
				EmailEnabled: true,
				Email:        "invalid-email",
			},
		},
		{
			name: "empty email when enabled",
			payload: domain.NotificationPreferences{
				EmailEnabled: true,
				Email:        "",
			},
		},
		{
			name: "invalid phone number",
			payload: domain.NotificationPreferences{
				SMSEnabled: true,
				Phone:      "not-a-phone",
			},
		},
		{
			name: "invalid coordinates",
			payload: domain.NotificationPreferences{
				GeoZoneEnabled: true,
				Coordinates: domain.LocationPoint{
					Latitude: 95.0, // out of range
				},
				RadiusMiles: 10.0,
			},
		},
		{
			name: "invalid zero radius",
			payload: domain.NotificationPreferences{
				GeoZoneEnabled: true,
				Coordinates: domain.LocationPoint{
					Latitude:  47.6062,
					Longitude: -122.3321,
				},
				RadiusMiles: 0,
			},
		},
		{
			name: "radius exceeds max 100",
			payload: domain.NotificationPreferences{
				GeoZoneEnabled: true,
				Coordinates: domain.LocationPoint{
					Latitude:  47.6062,
					Longitude: -122.3321,
				},
				RadiusMiles: 105.0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("PUT invalid preferences returned %d, want 400", rec.Code)
			}
		})
	}

	t.Run("invalid json body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader([]byte(`{not valid json`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("PUT invalid json returned %d, want 400", rec.Code)
		}
	})
}

func TestNotifications_AuthenticatedPreferences(t *testing.T) {
	memStore := store.NewMemoryStore()
	sessionMgr := &stubSessionManager{
		verified: identity.Principal{
			Subject:       "user-42",
			Email:         "user42@example.com",
			EmailVerified: true,
		},
	}
	srv := NewServerWithOptions(memStore, ServerOptions{
		IdentitySessions: sessionMgr,
	})

	sessionCookie := &http.Cookie{
		Name:  srv.sessionCookieName(),
		Value: "valid-session",
	}

	// 1. GET preferences without prior saved preferences returns defaults with user identity
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/preferences", nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET authenticated preferences returned %d, want 200", getRec.Code)
	}
	if got := getRec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("GET authenticated preferences Cache-Control = %q, want %q", got, "no-store")
	}
	var defPref domain.NotificationPreferences
	if err := json.NewDecoder(getRec.Body).Decode(&defPref); err != nil {
		t.Fatalf("failed to decode default preferences: %v", err)
	}
	if defPref.UserID != "user-42" {
		t.Errorf("expected UserID user-42, got %q", defPref.UserID)
	}
	if !defPref.EmailEnabled || defPref.Email != "user42@example.com" {
		t.Errorf("expected EmailEnabled with user42@example.com, got %v / %q", defPref.EmailEnabled, defPref.Email)
	}
	if defPref.RadiusMiles != 10.0 {
		t.Errorf("expected RadiusMiles 10.0, got %f", defPref.RadiusMiles)
	}

	// 2. PUT without CSRF returns 403 Forbidden
	prefUpdate := domain.NotificationPreferences{
		EmailEnabled:   true,
		Email:          "updated@example.com",
		SMSEnabled:     true,
		Phone:          "+12065550199",
		GeoZoneEnabled: true,
		Coordinates: domain.LocationPoint{
			Latitude:  47.6062,
			Longitude: -122.3321,
		},
		RadiusMiles: 25.0,
	}
	updateBytes, _ := json.Marshal(prefUpdate)
	noCSRFReq := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader(updateBytes))
	noCSRFReq.Header.Set("Content-Type", "application/json")
	noCSRFReq.AddCookie(sessionCookie)
	noCSRFRec := httptest.NewRecorder()
	srv.ServeHTTP(noCSRFRec, noCSRFReq)

	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("PUT preferences without CSRF returned %d, want 403", noCSRFRec.Code)
	}

	// 3. Obtain CSRF token
	csrfReq := httptest.NewRequest(http.MethodGet, "/api/v1/session/csrf", nil)
	csrfRec := httptest.NewRecorder()
	srv.ServeHTTP(csrfRec, csrfReq)
	if csrfRec.Code != http.StatusOK {
		t.Fatalf("GET CSRF returned %d, want 200", csrfRec.Code)
	}
	csrfCookie := responseCookie(t, csrfRec, srv.csrfCookieName())
	csrfToken := jsonStringField(t, csrfRec.Body.String(), "csrfToken")

	// 4. PUT with CSRF saves preferences
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader(updateBytes))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(csrfHeaderName, csrfToken)
	putReq.AddCookie(sessionCookie)
	putReq.AddCookie(csrfCookie)
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT preferences with CSRF returned %d, want 200; body: %s", putRec.Code, putRec.Body.String())
	}
	if got := putRec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("PUT preferences Cache-Control = %q, want %q", got, "no-store")
	}
	var savedPref domain.NotificationPreferences
	if err := json.NewDecoder(putRec.Body).Decode(&savedPref); err != nil {
		t.Fatalf("failed to decode saved preferences: %v", err)
	}
	if savedPref.UserID != "user-42" {
		t.Errorf("expected UserID user-42, got %q", savedPref.UserID)
	}
	if savedPref.Email != "updated@example.com" || savedPref.Phone != "+12065550199" || savedPref.RadiusMiles != 25.0 {
		t.Errorf("saved preferences mismatch: %+v", savedPref)
	}

	// 5. Subsequent GET returns saved preferences
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/preferences", nil)
	getReq2.AddCookie(sessionCookie)
	getRec2 := httptest.NewRecorder()
	srv.ServeHTTP(getRec2, getReq2)

	if getRec2.Code != http.StatusOK {
		t.Fatalf("GET preferences returned %d, want 200", getRec2.Code)
	}
	if got := getRec2.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("GET saved preferences Cache-Control = %q, want %q", got, "no-store")
	}
	var retrievedPref domain.NotificationPreferences
	if err := json.NewDecoder(getRec2.Body).Decode(&retrievedPref); err != nil {
		t.Fatalf("failed to decode retrieved preferences: %v", err)
	}
	if retrievedPref.Phone != "+12065550199" || retrievedPref.RadiusMiles != 25.0 {
		t.Errorf("retrieved preferences mismatch: %+v", retrievedPref)
	}
}

func TestNotifications_AuthenticatedFeedAndMarkRead(t *testing.T) {
	memStore := store.NewMemoryStore()
	sessionMgr := &stubSessionManager{
		verified: identity.Principal{
			Subject:       "user-99",
			Email:         "user99@example.com",
			EmailVerified: true,
		},
	}
	srv := NewServerWithOptions(memStore, ServerOptions{
		IdentitySessions: sessionMgr,
	})

	sessionCookie := &http.Cookie{
		Name:  srv.sessionCookieName(),
		Value: "valid-session",
	}

	// Seed notifications
	now := time.Now().UTC()
	notif1 := domain.InAppNotification{
		ID:        "n-1",
		UserID:    "user-99",
		Type:      "match",
		Title:     "Match found: Max",
		Message:   "A potential match was identified.",
		CreatedAt: now.Add(-2 * time.Hour),
		Read:      false,
	}
	notif2 := domain.InAppNotification{
		ID:        "n-2",
		UserID:    "user-99",
		Type:      "status",
		Title:     "Status updated: Bella",
		Message:   "Bella was reunited!",
		CreatedAt: now.Add(-1 * time.Hour),
		Read:      false,
	}
	notifOther := domain.InAppNotification{
		ID:        "n-other",
		UserID:    "user-other",
		Type:      "match",
		Title:     "Other match",
		Message:   "Not for user-99",
		CreatedAt: now,
		Read:      false,
	}

	for _, n := range []domain.InAppNotification{notif1, notif2, notifOther} {
		data, _ := json.Marshal(n)
		if err := memStore.SaveState(context.Background(), store.InAppNotificationsCollection, n.ID, data); err != nil {
			t.Fatalf("failed to seed notification: %v", err)
		}
	}

	// 1. GET feed for user-99
	feedReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	feedReq.AddCookie(sessionCookie)
	feedRec := httptest.NewRecorder()
	srv.ServeHTTP(feedRec, feedReq)

	if feedRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/notifications returned %d, want 200", feedRec.Code)
	}
	var feed domain.NotificationsFeedResponse
	if err := json.NewDecoder(feedRec.Body).Decode(&feed); err != nil {
		t.Fatalf("failed to decode feed: %v", err)
	}

	if len(feed.Notifications) != 2 {
		t.Fatalf("expected 2 notifications for user-99, got %d", len(feed.Notifications))
	}
	if feed.UnreadCount != 2 {
		t.Fatalf("expected unreadCount 2, got %d", feed.UnreadCount)
	}
	// Verify sorting: n-2 (1 hr ago) before n-1 (2 hr ago)
	if feed.Notifications[0].ID != "n-2" || feed.Notifications[1].ID != "n-1" {
		t.Fatalf("expected notifications sorted desc by CreatedAt, got [%s, %s]",
			feed.Notifications[0].ID, feed.Notifications[1].ID)
	}

	// 2. Mark specific notification as read without CSRF fails
	markPayload := domain.MarkNotificationsReadRequest{
		NotificationIDs: []string{"n-2"},
	}
	markBytes, _ := json.Marshal(markPayload)
	noCSRFReq := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/mark-read", bytes.NewReader(markBytes))
	noCSRFReq.Header.Set("Content-Type", "application/json")
	noCSRFReq.AddCookie(sessionCookie)
	noCSRFRec := httptest.NewRecorder()
	srv.ServeHTTP(noCSRFRec, noCSRFReq)

	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("POST mark-read without CSRF returned %d, want 403", noCSRFRec.Code)
	}

	// 3. Mark specific notification as read with CSRF
	csrfReq := httptest.NewRequest(http.MethodGet, "/api/v1/session/csrf", nil)
	csrfRec := httptest.NewRecorder()
	srv.ServeHTTP(csrfRec, csrfReq)
	csrfCookie := responseCookie(t, csrfRec, srv.csrfCookieName())
	csrfToken := jsonStringField(t, csrfRec.Body.String(), "csrfToken")

	markReq := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/mark-read", bytes.NewReader(markBytes))
	markReq.Header.Set("Content-Type", "application/json")
	markReq.Header.Set(csrfHeaderName, csrfToken)
	markReq.AddCookie(sessionCookie)
	markReq.AddCookie(csrfCookie)
	markRec := httptest.NewRecorder()
	srv.ServeHTTP(markRec, markReq)

	if markRec.Code != http.StatusOK {
		t.Fatalf("POST mark-read with CSRF returned %d, want 200", markRec.Code)
	}
	var markResp map[string]any
	if err := json.NewDecoder(markRec.Body).Decode(&markResp); err != nil {
		t.Fatalf("failed to decode mark-read resp: %v", err)
	}
	if int(markResp["markedCount"].(float64)) != 1 {
		t.Fatalf("expected markedCount 1, got %v", markResp["markedCount"])
	}

	// 4. Verify feed now has unreadCount 1 and n-2 is Read == true
	feedRec2 := httptest.NewRecorder()
	srv.ServeHTTP(feedRec2, feedReq)
	var feed2 domain.NotificationsFeedResponse
	_ = json.NewDecoder(feedRec2.Body).Decode(&feed2)
	if feed2.UnreadCount != 1 {
		t.Fatalf("expected unreadCount 1, got %d", feed2.UnreadCount)
	}
	if !feed2.Notifications[0].Read || feed2.Notifications[0].ReadAt == nil {
		t.Fatalf("expected n-2 to be marked Read with ReadAt timestamp, got %+v", feed2.Notifications[0])
	}
	if feed2.Notifications[1].Read {
		t.Fatalf("expected n-1 to remain unread")
	}

	// 5. Mark all as read with all: true
	allPayload := domain.MarkNotificationsReadRequest{
		All: true,
	}
	allBytes, _ := json.Marshal(allPayload)
	allReq := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/mark-read", bytes.NewReader(allBytes))
	allReq.Header.Set("Content-Type", "application/json")
	allReq.Header.Set(csrfHeaderName, csrfToken)
	allReq.AddCookie(sessionCookie)
	allReq.AddCookie(csrfCookie)
	allRec := httptest.NewRecorder()
	srv.ServeHTTP(allRec, allReq)

	if allRec.Code != http.StatusOK {
		t.Fatalf("POST mark-read all returned %d, want 200", allRec.Code)
	}
	var allResp map[string]any
	_ = json.NewDecoder(allRec.Body).Decode(&allResp)
	if int(allResp["markedCount"].(float64)) != 1 {
		t.Fatalf("expected markedCount 1 (only n-1 was unread), got %v", allResp["markedCount"])
	}

	// Feed should now have unreadCount 0
	feedRec3 := httptest.NewRecorder()
	srv.ServeHTTP(feedRec3, feedReq)
	var feed3 domain.NotificationsFeedResponse
	_ = json.NewDecoder(feedRec3.Body).Decode(&feed3)
	if feed3.UnreadCount != 0 {
		t.Fatalf("expected unreadCount 0, got %d", feed3.UnreadCount)
	}
}

func TestNotifications_MethodNotAllowed(t *testing.T) {
	srv := NewServer()

	disallowed := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/notifications"},
		{http.MethodDelete, "/api/v1/notifications"},
		{http.MethodGet, "/api/v1/notifications/mark-read"},
		{http.MethodDelete, "/api/v1/notifications/mark-read"},
		{http.MethodDelete, "/api/v1/notifications/preferences"},
	}

	for _, d := range disallowed {
		req := httptest.NewRequest(d.method, d.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s returned %d, want 405", d.method, d.path, rec.Code)
		}
	}
}

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
