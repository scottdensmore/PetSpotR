package webfrontend

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// authenticatedPrincipal returns the verified principal if a valid session cookie exists, or nil for guests.
func (s *Server) authenticatedPrincipal(r *http.Request) *identity.Principal {
	if s.identitySessions == nil {
		return nil
	}
	cookie, err := r.Cookie(s.sessionCookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil
	}
	principal, err := s.identitySessions.VerifySession(r.Context(), cookie.Value)
	if err != nil {
		return nil
	}
	return &principal
}

// handleApiNotifications handles GET /api/v1/notifications for both authenticated users and guests.
func (s *Server) handleApiNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	principal := s.authenticatedPrincipal(r)
	if principal != nil {
		rawItems, err := s.stateStore.ListState(r.Context(), store.InAppNotificationsCollection)
		if err != nil {
			http.Error(w, "Failed to load notifications", http.StatusInternalServerError)
			return
		}

		notifications := make([]domain.InAppNotification, 0)
		for _, data := range rawItems {
			var notif domain.InAppNotification
			if err := json.Unmarshal(data, &notif); err != nil {
				continue
			}
			if notif.UserID == principal.Subject {
				notifications = append(notifications, notif)
			}
		}

		sort.Slice(notifications, func(i, j int) bool {
			if notifications[i].CreatedAt.Equal(notifications[j].CreatedAt) {
				return notifications[i].ID < notifications[j].ID
			}
			return notifications[i].CreatedAt.After(notifications[j].CreatedAt)
		})

		unreadCount := 0
		for _, notif := range notifications {
			if !notif.Read {
				unreadCount++
			}
		}

		_ = json.NewEncoder(w).Encode(domain.NotificationsFeedResponse{
			Notifications: notifications,
			UnreadCount:   unreadCount,
		})
		return
	}

	// Guest feed: return community/broadcast notifications or onboarding welcome alert
	notifications := make([]domain.InAppNotification, 0)
	rawItems, err := s.stateStore.ListState(r.Context(), store.InAppNotificationsCollection)
	if err == nil {
		for _, data := range rawItems {
			var notif domain.InAppNotification
			if err := json.Unmarshal(data, &notif); err != nil {
				continue
			}
			if notif.UserID == "" || notif.UserID == "guest" || notif.Type == "broadcast" {
				notifications = append(notifications, notif)
			}
		}
	}

	if len(notifications) == 0 {
		notifications = append(notifications, domain.InAppNotification{
			ID:        "welcome-onboarding",
			UserID:    "",
			Type:      "broadcast",
			Title:     "Welcome to PetSpotR Alerts",
			Message:   "Enable push notifications or configure your geographic zone in Preferences to receive real-time lost & found pet alerts.",
			Link:      "/pets",
			CreatedAt: time.Now().UTC(),
			Read:      false,
		})
	}

	sort.Slice(notifications, func(i, j int) bool {
		if notifications[i].CreatedAt.Equal(notifications[j].CreatedAt) {
			return notifications[i].ID < notifications[j].ID
		}
		return notifications[i].CreatedAt.After(notifications[j].CreatedAt)
	})

	unreadCount := 0
	for _, notif := range notifications {
		if !notif.Read {
			unreadCount++
		}
	}

	_ = json.NewEncoder(w).Encode(domain.NotificationsFeedResponse{
		Notifications: notifications,
		UnreadCount:   unreadCount,
	})
}

// handleApiNotificationsMarkRead handles POST /api/v1/notifications/mark-read.
func (s *Server) handleApiNotificationsMarkRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.MarkNotificationsReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	principal := s.authenticatedPrincipal(r)
	if principal == nil {
		// Guests manage notification read state locally in the client
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":      "ok",
			"markedCount": 0,
		})
		return
	}

	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	rawItems, err := s.stateStore.ListState(r.Context(), store.InAppNotificationsCollection)
	if err != nil {
		http.Error(w, "Failed to load notifications", http.StatusInternalServerError)
		return
	}

	targetIDs := make(map[string]bool)
	for _, id := range req.NotificationIDs {
		targetIDs[id] = true
	}

	now := time.Now().UTC()
	markedCount := 0

	for key, data := range rawItems {
		var notif domain.InAppNotification
		if err := json.Unmarshal(data, &notif); err != nil {
			continue
		}
		if notif.UserID != principal.Subject {
			continue
		}

		shouldMark := false
		if req.All {
			shouldMark = !notif.Read
		} else if targetIDs[notif.ID] || targetIDs[key] {
			shouldMark = !notif.Read
		}

		if shouldMark {
			notif.Read = true
			notif.ReadAt = &now
			updatedData, err := json.Marshal(notif)
			if err != nil {
				continue
			}
			if err := s.stateStore.SaveState(r.Context(), store.InAppNotificationsCollection, key, updatedData); err == nil {
				markedCount++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"markedCount": markedCount,
	})
}

// handleApiNotificationsPreferences handles GET and PUT for /api/v1/notifications/preferences.
func (s *Server) handleApiNotificationsPreferences(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	switch r.Method {
	case http.MethodGet:
		principal := s.authenticatedPrincipal(r)
		if principal == nil {
			// Guest: return default preferences
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(domain.NotificationPreferences{
				RadiusMiles: 10.0,
			})
			return
		}

		data, err := s.stateStore.GetState(r.Context(), store.NotificationPreferencesCollection, principal.Subject)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(domain.NotificationPreferences{
					UserID:       principal.Subject,
					EmailEnabled: true,
					Email:        principal.Email,
					RadiusMiles:  10.0,
				})
				return
			}
			http.Error(w, "Failed to load preferences", http.StatusInternalServerError)
			return
		}

		var pref domain.NotificationPreferences
		if err := json.Unmarshal(data, &pref); err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(domain.NotificationPreferences{
				UserID:       principal.Subject,
				EmailEnabled: true,
				Email:        principal.Email,
				RadiusMiles:  10.0,
			})
			return
		}
		if pref.UserID == "" {
			pref.UserID = principal.Subject
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pref)

	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 1048576)
		var pref domain.NotificationPreferences
		if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		if err := pref.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		principal := s.authenticatedPrincipal(r)
		if principal != nil {
			if !s.validCSRF(r) {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			pref.UserID = principal.Subject
			pref.UpdatedAt = time.Now().UTC()

			data, err := json.Marshal(pref)
			if err != nil {
				http.Error(w, "Failed to encode preferences", http.StatusInternalServerError)
				return
			}

			if err := s.stateStore.SaveState(r.Context(), store.NotificationPreferencesCollection, principal.Subject, data); err != nil {
				http.Error(w, "Failed to save preferences", http.StatusInternalServerError)
				return
			}
		} else {
			pref.UserID = ""
			pref.UpdatedAt = time.Now().UTC()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pref)

	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
