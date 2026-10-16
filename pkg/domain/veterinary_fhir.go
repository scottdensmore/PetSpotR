package domain

import (
	"errors"
	"strings"
	"time"
)

type PIMSProvider string

const (
	PIMSCornerstone PIMSProvider = "IDEXX_CORNERSTONE"
	PIMSEzyVet      PIMSProvider = "EZYVET"
	PIMSAvimark     PIMSProvider = "AVIMARK"
	PIMSShelterluv  PIMSProvider = "SHELTERLUV"
	PIMSGenericFHIR PIMSProvider = "HL7_FHIR_R4"
)

func (p PIMSProvider) IsValid() bool {
	switch p {
	case PIMSCornerstone, PIMSEzyVet, PIMSAvimark, PIMSShelterluv, PIMSGenericFHIR:
		return true
	default:
		return false
	}
}

type HandoffStatus string

const (
	HandoffStatusEnRoute                 HandoffStatus = "EN_ROUTE_PRE_ARRIVAL"
	HandoffStatusReceivedInTreatment     HandoffStatus = "IN_CLINIC_TREATMENT"
	HandoffStatusStabilized              HandoffStatus = "STABILIZED"
	HandoffStatusClearedForReunification HandoffStatus = "CLEARED_FOR_REUNIFICATION"
)

func (s HandoffStatus) IsValid() bool {
	switch s {
	case HandoffStatusEnRoute, HandoffStatusReceivedInTreatment, HandoffStatusStabilized, HandoffStatusClearedForReunification:
		return true
	default:
		return false
	}
}

type VeterinaryClinic struct {
	ClinicID          string       `json:"clinicId"`
	Name              string       `json:"name"`
	PIMS              PIMSProvider `json:"pims"`
	Emergency24Hr     bool         `json:"emergency24Hr"`
	Phone             string       `json:"phone"`
	Address           string       `json:"address"`
	Latitude          float64      `json:"latitude"`
	Longitude         float64      `json:"longitude"`
	AcceptingPatients bool         `json:"acceptingPatients"`
	CreatedAt         time.Time    `json:"createdAt"`
}

func (c *VeterinaryClinic) Validate() error {
	if c == nil {
		return errors.New("veterinary clinic is nil")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("clinic name is required")
	}
	if !c.PIMS.IsValid() {
		return errors.New("invalid PIMS provider")
	}
	if c.Latitude < -90 || c.Latitude > 90 {
		return errors.New("latitude must be between -90 and 90")
	}
	if c.Longitude < -180 || c.Longitude > 180 {
		return errors.New("longitude must be between -180 and 180")
	}
	return nil
}

type FHIRResourceEntry struct {
	FullURL  string         `json:"fullUrl"`
	Resource map[string]any `json:"resource"`
}

type FHIRVeterinaryBundle struct {
	ResourceType    string              `json:"resourceType"`
	ID              string              `json:"id"`
	Type            string              `json:"type"`
	Timestamp       time.Time           `json:"timestamp"`
	BundleDigestHex string              `json:"bundleDigestHex"`
	Entry           []FHIRResourceEntry `json:"entry"`
}

type PIMSIntakeRecord struct {
	HandoffID          string         `json:"handoffId"`
	ClinicID           string         `json:"clinicId"`
	ClinicName         string         `json:"clinicName"`
	PIMS               PIMSProvider   `json:"pims"`
	PetID              string         `json:"petId"`
	PetName            string         `json:"petName"`
	Species            string         `json:"species"`
	Breed              string         `json:"breed"`
	WeightKg           float64        `json:"weightKg"`
	MicrochipID        string         `json:"microchipId,omitempty"`
	TriageCategory     TriageCategory `json:"triageCategory"`
	HeartRateBPM       int            `json:"heartRateBpm"`
	RespiratoryRateBPM int            `json:"respiratoryRateBpm"`
	TemperatureF       float64        `json:"temperatureF"`
	CriticalAllergies  []string       `json:"criticalAllergies"`
	TreatmentsGiven    []string       `json:"treatmentsGiven"`
	AttendingVet       string         `json:"attendingVet,omitempty"`
	DischargeSummary   string         `json:"dischargeSummary,omitempty"`
}

type ClinicalHandoff struct {
	HandoffID           string               `json:"handoffId"`
	AssessmentID        string               `json:"assessmentId"`
	PetID               string               `json:"petId"`
	PassportID          string               `json:"passportId,omitempty"`
	ClinicID            string               `json:"clinicId"`
	ClinicName          string               `json:"clinicName"`
	PIMS                PIMSProvider         `json:"pims"`
	TriageCategory      TriageCategory       `json:"triageCategory"`
	TransportUnit       string               `json:"transportUnit"`
	ETAMinutes          int                  `json:"etaMinutes"`
	Status              HandoffStatus        `json:"status"`
	VerifiedMicrochipID string               `json:"verifiedMicrochipId,omitempty"`
	AttendingVet        string               `json:"attendingVet,omitempty"`
	DischargeSummary    string               `json:"dischargeSummary,omitempty"`
	ReadyForReunion     bool                 `json:"readyForReunion"`
	FHIRBundle          FHIRVeterinaryBundle `json:"fhirBundle"`
	PIMSRecord          PIMSIntakeRecord     `json:"pimsRecord"`
	DispatchedAt        time.Time            `json:"dispatchedAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}
