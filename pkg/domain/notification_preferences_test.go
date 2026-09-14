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
		{
			name: "radius exceeds max 100 when geo zone enabled",
			pref: NotificationPreferences{
				UserID:         "user-123",
				GeoZoneEnabled: true,
				Coordinates:    LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
				RadiusMiles:    101.0,
			},
			wantErr: true,
		},
		{
			name: "valid sms preference",
			pref: NotificationPreferences{
				UserID:     "user-123",
				SMSEnabled: true,
				Phone:      "+12065550100",
			},
			wantErr: false,
		},
		{
			name: "invalid email and phone ignored when channels disabled",
			pref: NotificationPreferences{
				UserID:       "user-123",
				EmailEnabled: false,
				Email:        "invalid",
				SMSEnabled:   false,
				Phone:        "invalid",
			},
			wantErr: false,
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
