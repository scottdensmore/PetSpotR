package webhook

import (
	"time"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// Canonical collection names for webhooks.
const (
	WebhooksCollection          = store.WebhooksCollection
	WebhookDeliveriesCollection = store.WebhookDeliveriesCollection
)

// WebhookSubscription defines a partner's webhook registration.
type WebhookSubscription struct {
	ID           string    `json:"id"`
	PartnerID    string    `json:"partnerId"`
	TargetURL    string    `json:"targetUrl"`
	Secret       string    `json:"secret"` // Used for HMAC-SHA256 signatures
	FilterEvents []string  `json:"filterEvents"`
	GeoFence     *GeoFence `json:"geoFence,omitempty"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
}

// GeoFence restricts events to a specific geographic radius.
type GeoFence struct {
	CenterLat   float64 `json:"centerLat"`
	CenterLng   float64 `json:"centerLng"`
	RadiusMiles float64 `json:"radiusMiles"`
}

// WebhookDeliveryRecord records the outcome of a webhook delivery attempt.
type WebhookDeliveryRecord struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscriptionId"`
	EventID        string    `json:"eventId,omitempty"`
	EventType      string    `json:"eventType"`
	TargetURL      string    `json:"targetUrl"`
	StatusCode     int       `json:"statusCode"`
	Success        bool      `json:"success"`
	Error          string    `json:"error,omitempty"`
	Attempt        int       `json:"attempt"`
	ResponseTimeMs int64     `json:"responseTimeMs"`
	CreatedAt      time.Time `json:"createdAt"`
}
