package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCameraVendor_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		vendor CameraVendor
		want   bool
	}{
		{"Ring", CameraVendorRing, true},
		{"Nest", CameraVendorNest, true},
		{"Wyze", CameraVendorWyze, true},
		{"Frigate RTSP", CameraVendorFrigateRTSP, true},
		{"Trail Cam", CameraVendorTrailCam, true},
		{"Empty", CameraVendor(""), false},
		{"Unknown", CameraVendor("ARLO"), false},
		{"Lowercase", CameraVendor("ring"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vendor.IsValid(); got != tt.want {
				t.Errorf("CameraVendor(%q).IsValid() = %v, want %v", tt.vendor, got, tt.want)
			}
		})
	}
}

func TestCameraNode_Validate(t *testing.T) {
	validNode := func() *CameraNode {
		return &CameraNode{
			ID:                   "cam-1",
			Name:                 "Backyard Feeder Cam",
			Vendor:               CameraVendorRing,
			Latitude:             47.6062,
			Longitude:            -122.3321,
			HeadingDegrees:       180.0,
			FOVDegrees:           110.0,
			NightVisionIR:        true,
			AutoPromoteThreshold: 0.85,
			Status:               CameraStatusActive,
		}
	}

	t.Run("valid node", func(t *testing.T) {
		node := validNode()
		if err := node.Validate(); err != nil {
			t.Fatalf("expected valid node, got error: %v", err)
		}
	})

	t.Run("nil node", func(t *testing.T) {
		var node *CameraNode
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "camera node is nil") {
			t.Fatalf("expected nil error, got: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		node := validNode()
		node.Name = "   "
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "camera name is required") {
			t.Fatalf("expected name error, got: %v", err)
		}
	})

	t.Run("invalid vendor", func(t *testing.T) {
		node := validNode()
		node.Vendor = CameraVendor("INVALID")
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid camera vendor") {
			t.Fatalf("expected vendor error, got: %v", err)
		}
	})

	t.Run("latitude out of range low", func(t *testing.T) {
		node := validNode()
		node.Latitude = -90.1
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "latitude must be between -90 and 90") {
			t.Fatalf("expected latitude error, got: %v", err)
		}
	})

	t.Run("latitude out of range high", func(t *testing.T) {
		node := validNode()
		node.Latitude = 90.1
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "latitude must be between -90 and 90") {
			t.Fatalf("expected latitude error, got: %v", err)
		}
	})

	t.Run("longitude out of range low", func(t *testing.T) {
		node := validNode()
		node.Longitude = -180.1
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "longitude must be between -180 and 180") {
			t.Fatalf("expected longitude error, got: %v", err)
		}
	})

	t.Run("longitude out of range high", func(t *testing.T) {
		node := validNode()
		node.Longitude = 180.1
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "longitude must be between -180 and 180") {
			t.Fatalf("expected longitude error, got: %v", err)
		}
	})

	t.Run("autoPromoteThreshold out of range low", func(t *testing.T) {
		node := validNode()
		node.AutoPromoteThreshold = -0.01
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "autoPromoteThreshold must be between 0.0 and 1.0") {
			t.Fatalf("expected threshold error, got: %v", err)
		}
	})

	t.Run("autoPromoteThreshold out of range high", func(t *testing.T) {
		node := validNode()
		node.AutoPromoteThreshold = 1.01
		err := node.Validate()
		if err == nil || !strings.Contains(err.Error(), "autoPromoteThreshold must be between 0.0 and 1.0") {
			t.Fatalf("expected threshold error, got: %v", err)
		}
	})
}

func TestCameraTrap_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 16, 22, 0, 0, 0, time.UTC)
	lastSeen := now.Add(5 * time.Minute)

	node := CameraNode{
		ID:                   "cam-123",
		Name:                 "Alleyway Trail Cam",
		Vendor:               CameraVendorTrailCam,
		OwnerName:            "Alice",
		TargetPetID:          "pet-99",
		Latitude:             47.6101,
		Longitude:            -122.3421,
		HeadingDegrees:       90.0,
		FOVDegrees:           120.0,
		NightVisionIR:        true,
		AutoPromoteThreshold: 0.80,
		WebhookSecretHash:    "hash123",
		WebhookTokenPreview:  "pst_...abcd",
		LastFrameDHash:       0xabcdef0123456789,
		LastFrameDHashHex:    "abcdef0123456789",
		TotalCaptures:        12,
		SuppressedDuplicates: 4,
		Status:               CameraStatusPaused,
		CreatedAt:            now,
		LastSeenAt:           &lastSeen,
	}

	nodeData, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal CameraNode: %v", err)
	}

	var decodedNode CameraNode
	if err := json.Unmarshal(nodeData, &decodedNode); err != nil {
		t.Fatalf("unmarshal CameraNode: %v", err)
	}

	if decodedNode.ID != node.ID || decodedNode.Vendor != CameraVendorTrailCam || decodedNode.Status != CameraStatusPaused {
		t.Errorf("decoded node mismatch: %+v", decodedNode)
	}
	if decodedNode.LastSeenAt == nil || !decodedNode.LastSeenAt.Equal(lastSeen) {
		t.Errorf("expected LastSeenAt %v, got %v", lastSeen, decodedNode.LastSeenAt)
	}
	if CameraStatusOffline != "OFFLINE" {
		t.Errorf("unexpected CameraStatusOffline: %s", CameraStatusOffline)
	}

	capture := CameraCapture{
		ID:              "cap-456",
		CameraID:        node.ID,
		CameraName:      node.Name,
		Vendor:          node.Vendor,
		TargetPetID:     node.TargetPetID,
		Latitude:        node.Latitude,
		Longitude:       node.Longitude,
		CapturedAt:      lastSeen,
		FrameDHash:      0xabcdef0123456789,
		FrameDHashHex:   "abcdef0123456789",
		HammingDistance: 18,
		IsDuplicate:     false,
		IsNightVisionIR: true,
		IREnhanced:      true,
		ROI: ROIBoundingBox{
			X:      0.15,
			Y:      0.25,
			Width:  0.40,
			Height: 0.35,
		},
		DetectedSpecies:    "DOG",
		ExtractedTraits:    []string{"pointed-ears", "reflective-collar"},
		SaliencyScore:      0.91,
		ConfidenceScore:    0.88,
		SnapshotDataURI:    "data:image/png;base64,iVBORw0KGgo=",
		CropThumbnailURI:   "data:image/png;base64,iVBORw0KGgo=",
		VerificationStatus: CaptureStatusPromoted,
		PromotedSightingID: "sight-789",
	}

	capData, err := json.Marshal(capture)
	if err != nil {
		t.Fatalf("marshal CameraCapture: %v", err)
	}

	var decodedCap CameraCapture
	if err := json.Unmarshal(capData, &decodedCap); err != nil {
		t.Fatalf("unmarshal CameraCapture: %v", err)
	}

	if decodedCap.ID != capture.ID || decodedCap.VerificationStatus != CaptureStatusPromoted {
		t.Errorf("decoded capture mismatch: %+v", decodedCap)
	}
	if decodedCap.ROI != capture.ROI {
		t.Errorf("ROI mismatch: got %+v, want %+v", decodedCap.ROI, capture.ROI)
	}
	if len(decodedCap.ExtractedTraits) != 2 || decodedCap.ExtractedTraits[0] != "pointed-ears" {
		t.Errorf("ExtractedTraits mismatch: %v", decodedCap.ExtractedTraits)
	}
	if CaptureStatusPendingReview != "PENDING_REVIEW" || CaptureStatusDismissed != "DISMISSED" || CaptureStatusDuplicate != "DUPLICATE_SUPPRESSED" {
		t.Errorf("unexpected capture verification status constants")
	}
}
