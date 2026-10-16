package domain

import (
	"errors"
	"strings"
	"time"
)

type CameraVendor string

const (
	CameraVendorRing        CameraVendor = "RING"
	CameraVendorNest        CameraVendor = "NEST"
	CameraVendorWyze        CameraVendor = "WYZE"
	CameraVendorFrigateRTSP CameraVendor = "FRIGATE_RTSP"
	CameraVendorTrailCam    CameraVendor = "TRAIL_CAM"
)

func (v CameraVendor) IsValid() bool {
	switch v {
	case CameraVendorRing, CameraVendorNest, CameraVendorWyze, CameraVendorFrigateRTSP, CameraVendorTrailCam:
		return true
	default:
		return false
	}
}

type CameraNodeStatus string

const (
	CameraStatusActive  CameraNodeStatus = "ACTIVE"
	CameraStatusPaused  CameraNodeStatus = "PAUSED"
	CameraStatusOffline CameraNodeStatus = "OFFLINE"
)

type CaptureVerificationStatus string

const (
	CaptureStatusPendingReview CaptureVerificationStatus = "PENDING_REVIEW"
	CaptureStatusPromoted      CaptureVerificationStatus = "PROMOTED"
	CaptureStatusDismissed     CaptureVerificationStatus = "DISMISSED"
	CaptureStatusDuplicate     CaptureVerificationStatus = "DUPLICATE_SUPPRESSED"
)

type ROIBoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type CameraNode struct {
	ID                   string           `json:"id"`
	Name                 string           `json:"name"`
	Vendor               CameraVendor     `json:"vendor"`
	OwnerName            string           `json:"ownerName,omitempty"`
	TargetPetID          string           `json:"targetPetId,omitempty"`
	Latitude             float64          `json:"latitude"`
	Longitude            float64          `json:"longitude"`
	HeadingDegrees       float64          `json:"headingDegrees"`
	FOVDegrees           float64          `json:"fovDegrees"`
	NightVisionIR        bool             `json:"nightVisionIr"`
	AutoPromoteThreshold float64          `json:"autoPromoteThreshold"`
	WebhookSecretHash    string           `json:"webhookSecretHash,omitempty"`
	WebhookTokenPreview  string           `json:"webhookTokenPreview,omitempty"`
	LastFrameDHash       uint64           `json:"lastFrameDHash"`
	LastFrameDHashHex    string           `json:"lastFrameDHashHex,omitempty"`
	TotalCaptures        int              `json:"totalCaptures"`
	SuppressedDuplicates int              `json:"suppressedDuplicates"`
	Status               CameraNodeStatus `json:"status"`
	CreatedAt            time.Time        `json:"createdAt"`
	LastSeenAt           *time.Time       `json:"lastSeenAt,omitempty"`
}

func (c *CameraNode) Validate() error {
	if c == nil {
		return errors.New("camera node is nil")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("camera name is required")
	}
	if !c.Vendor.IsValid() {
		return errors.New("invalid camera vendor")
	}
	if c.Latitude < -90 || c.Latitude > 90 {
		return errors.New("latitude must be between -90 and 90")
	}
	if c.Longitude < -180 || c.Longitude > 180 {
		return errors.New("longitude must be between -180 and 180")
	}
	if c.AutoPromoteThreshold < 0 || c.AutoPromoteThreshold > 1 {
		return errors.New("autoPromoteThreshold must be between 0.0 and 1.0")
	}
	return nil
}

type CameraCapture struct {
	ID                 string                    `json:"id"`
	CameraID           string                    `json:"cameraId"`
	CameraName         string                    `json:"cameraName"`
	Vendor             CameraVendor              `json:"vendor"`
	TargetPetID        string                    `json:"targetPetId,omitempty"`
	Latitude           float64                   `json:"latitude"`
	Longitude          float64                   `json:"longitude"`
	CapturedAt         time.Time                 `json:"capturedAt"`
	FrameDHash         uint64                    `json:"frameDHash"`
	FrameDHashHex      string                    `json:"frameDHashHex"`
	HammingDistance    int                       `json:"hammingDistance"`
	IsDuplicate        bool                      `json:"isDuplicate"`
	IsNightVisionIR    bool                      `json:"isNightVisionIr"`
	IREnhanced         bool                      `json:"irEnhanced"`
	ROI                ROIBoundingBox            `json:"roi"`
	DetectedSpecies    string                    `json:"detectedSpecies"`
	ExtractedTraits    []string                  `json:"extractedTraits"`
	SaliencyScore      float64                   `json:"saliencyScore"`
	ConfidenceScore    float64                   `json:"confidenceScore"`
	SnapshotDataURI    string                    `json:"snapshotDataUri"`
	CropThumbnailURI   string                    `json:"cropThumbnailUri,omitempty"`
	VerificationStatus CaptureVerificationStatus `json:"verificationStatus"`
	PromotedSightingID string                    `json:"promotedSightingId,omitempty"`
}
