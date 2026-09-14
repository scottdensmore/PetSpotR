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
