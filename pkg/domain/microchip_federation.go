package domain

import (
	"fmt"
	"strings"
	"time"
)

type MicrochipStandardType string

const (
	ChipStandardISO11784_15 MicrochipStandardType = "ISO_11784_15"
	ChipStandardAVID9       MicrochipStandardType = "AVID_9_ENCRYPTED"
	ChipStandardHex10Legacy MicrochipStandardType = "HEX_10_LEGACY"
)

func ValidMicrochipStandardType(s MicrochipStandardType) bool {
	switch s {
	case ChipStandardISO11784_15, ChipStandardAVID9, ChipStandardHex10Legacy:
		return true
	default:
		return false
	}
}

func (s MicrochipStandardType) IsValid() bool {
	return ValidMicrochipStandardType(s)
}

type FederationResolutionStatus string

const (
	ChipStatusRegisteredOwnerFound      FederationResolutionStatus = "REGISTERED_OWNER_FOUND"
	ChipStatusUnregisteredBatchTraced   FederationResolutionStatus = "UNREGISTERED_BATCH_TRACED"
	ChipStatusSecondaryContactEscalated FederationResolutionStatus = "SECONDARY_CONTACT_ESCALATED"
	ChipStatusReunionDispatchedVerified FederationResolutionStatus = "REUNION_DISPATCHED_VERIFIED"
)

func ValidFederationResolutionStatus(s FederationResolutionStatus) bool {
	switch s {
	case ChipStatusRegisteredOwnerFound,
		ChipStatusUnregisteredBatchTraced,
		ChipStatusSecondaryContactEscalated,
		ChipStatusReunionDispatchedVerified:
		return true
	default:
		return false
	}
}

func (s FederationResolutionStatus) IsValid() bool {
	return ValidFederationResolutionStatus(s)
}

type RegistryQueryHop struct {
	RegistryName    string `json:"registryName"`
	ManufacturerISO string `json:"manufacturerIso"`
	Responded       bool   `json:"responded"`
	RecordFound     bool   `json:"recordFound"`
	LastUpdatedYear int    `json:"lastUpdatedYear,omitempty"`
	StatusNote      string `json:"statusNote"`
}

type MicrochipScanCase struct {
	CaseID                 string                     `json:"caseId"`
	PetID                  string                     `json:"petId"`
	PetName                string                     `json:"petName"`
	Species                string                     `json:"species"` // "DOG" or "CAT"
	RawChipInput           string                     `json:"rawChipInput"`
	NormalizedChipID       string                     `json:"normalizedChipId"`
	MaskedChipDisplay      string                     `json:"maskedChipDisplay"`
	ChipStandard           MicrochipStandardType      `json:"chipStandard"`
	FrequencyKhz           float64                    `json:"frequencyKhz"`     // e.g. 134.2, 125.0, 128.0
	ManufacturerCode       string                     `json:"manufacturerCode"` // e.g. "985", "981", "982", "900", "AVID", "FDX-A"
	ManufacturerName       string                     `json:"manufacturerName"`
	ManufacturerLotID      string                     `json:"manufacturerLotId"` // e.g. "LOT-MERCK-2024-88A"
	Status                 FederationResolutionStatus `json:"status"`
	ScannerLocation        GeoPoint                   `json:"scannerLocation"`
	ScannedByAgency        string                     `json:"scannedByAgency"` // e.g. "Seattle Animal Shelter Intake #2", "SAR Field Unit K9-04"
	WinningRegistry        string                     `json:"winningRegistry,omitempty"`
	RegistryHops           []RegistryQueryHop         `json:"registryHops"`
	ImplantingClinicName   string                     `json:"implantingClinicName,omitempty"`
	ImplantingClinicPhone  string                     `json:"implantingClinicPhone,omitempty"`
	ImplantPatientChartID  string                     `json:"implantPatientChartId,omitempty"`
	ResolvedOwnerName      string                     `json:"resolvedOwnerName,omitempty"`
	ResolvedOwnerPhoneMask string                     `json:"resolvedOwnerPhoneMask,omitempty"`
	ResolutionNotes        string                     `json:"resolutionNotes"`
	PromotedSightingID     string                     `json:"promotedSightingId,omitempty"`
	CreatedAt              time.Time                  `json:"createdAt"`
	UpdatedAt              time.Time                  `json:"updatedAt"`
}

func (c MicrochipScanCase) Validate() error {
	if strings.TrimSpace(c.CaseID) == "" {
		return fmt.Errorf("domain: caseId is required")
	}
	if strings.TrimSpace(c.PetID) == "" {
		return fmt.Errorf("domain: petId is required")
	}
	if strings.TrimSpace(c.PetName) == "" {
		return fmt.Errorf("domain: petName is required")
	}
	species := strings.TrimSpace(c.Species)
	if species == "" || species != strings.ToUpper(species) {
		return fmt.Errorf("domain: species must be non-empty uppercase (e.g. DOG or CAT)")
	}
	if strings.TrimSpace(c.RawChipInput) == "" {
		return fmt.Errorf("domain: rawChipInput is required")
	}
	if strings.TrimSpace(c.NormalizedChipID) == "" {
		return fmt.Errorf("domain: normalizedChipId is required")
	}
	if strings.TrimSpace(c.MaskedChipDisplay) == "" {
		return fmt.Errorf("domain: maskedChipDisplay is required")
	}
	if !ValidMicrochipStandardType(c.ChipStandard) {
		return fmt.Errorf("domain: invalid chipStandard %q", c.ChipStandard)
	}
	if c.FrequencyKhz <= 100.0 || c.FrequencyKhz > 150.0 {
		return fmt.Errorf("domain: frequencyKhz must be in (100.0, 150.0], got %f", c.FrequencyKhz)
	}
	if strings.TrimSpace(c.ManufacturerCode) == "" {
		return fmt.Errorf("domain: manufacturerCode is required")
	}
	if strings.TrimSpace(c.ManufacturerName) == "" {
		return fmt.Errorf("domain: manufacturerName is required")
	}
	if strings.TrimSpace(c.ManufacturerLotID) == "" {
		return fmt.Errorf("domain: manufacturerLotId is required")
	}
	if !ValidFederationResolutionStatus(c.Status) {
		return fmt.Errorf("domain: invalid status %q", c.Status)
	}
	if err := c.ScannerLocation.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.ScannedByAgency) == "" {
		return fmt.Errorf("domain: scannedByAgency is required")
	}
	if strings.TrimSpace(c.ResolutionNotes) == "" {
		return fmt.Errorf("domain: resolutionNotes is required")
	}
	if len(c.RegistryHops) < 1 {
		return fmt.Errorf("domain: registryHops must contain at least 1 hop")
	}
	for i, hop := range c.RegistryHops {
		if strings.TrimSpace(hop.RegistryName) == "" {
			return fmt.Errorf("domain: registryHops[%d].registryName is required", i)
		}
		if strings.TrimSpace(hop.StatusNote) == "" {
			return fmt.Errorf("domain: registryHops[%d].statusNote is required", i)
		}
	}
	if c.Status == ChipStatusReunionDispatchedVerified {
		if strings.TrimSpace(c.ResolvedOwnerName) == "" || strings.TrimSpace(c.ResolvedOwnerPhoneMask) == "" {
			return fmt.Errorf("domain: resolvedOwnerName and resolvedOwnerPhoneMask are required when status is REUNION_DISPATCHED_VERIFIED")
		}
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("domain: createdAt is required")
	}
	if c.UpdatedAt.IsZero() {
		return fmt.Errorf("domain: updatedAt is required")
	}
	return nil
}
