package domain

import (
	"errors"
	"strings"
	"time"
)

type CustodyStage string

const (
	CustodyStageFinderIntake   CustodyStage = "FINDER_INTAKE"
	CustodyStageFieldSAREscort CustodyStage = "FIELD_SAR_ESCORT"
	CustodyStageVetERHold      CustodyStage = "VET_ER_HOLD"
	CustodyStageShelterHold    CustodyStage = "SHELTER_SAFEKEEPING"
	CustodyStageOwnerEscrow    CustodyStage = "OWNER_REUNIFICATION_ESCROW"
	CustodyStageReunited       CustodyStage = "REUNITED_RELEASED"
)

func (s CustodyStage) IsValid() bool {
	switch s {
	case CustodyStageFinderIntake,
		CustodyStageFieldSAREscort,
		CustodyStageVetERHold,
		CustodyStageShelterHold,
		CustodyStageOwnerEscrow,
		CustodyStageReunited:
		return true
	default:
		return false
	}
}

type OwnershipProofMethod string

const (
	ProofMicrochipMatch OwnershipProofMethod = "MICROCHIP_SCAN_MATCH"
	ProofVetRecordDeed  OwnershipProofMethod = "VET_RECORD_DEED"
	ProofPhotoBiometric OwnershipProofMethod = "PHOTO_BIOMETRIC_MATCH"
	ProofGovIDAffidavit OwnershipProofMethod = "GOVERNMENT_ID_AFFIDAVIT"
)

func (m OwnershipProofMethod) IsValid() bool {
	switch m {
	case ProofMicrochipMatch, ProofVetRecordDeed, ProofPhotoBiometric, ProofGovIDAffidavit:
		return true
	default:
		return false
	}
}

type CustodyTransferHop struct {
	HopIndex         int          `json:"hopIndex"`
	Stage            CustodyStage `json:"stage"`
	FromCustodian    string       `json:"fromCustodian"`
	ToCustodian      string       `json:"toCustodian"`
	ToRole           string       `json:"toRole"`
	LocationLabel    string       `json:"locationLabel"`
	ConditionNotes   string       `json:"conditionNotes"`
	TransferredAt    time.Time    `json:"transferredAt"`
	PrevHashHex      string       `json:"prevHashHex"`
	EntryHashHex     string       `json:"entryHashHex"`
	HmacSignatureHex string       `json:"hmacSignatureHex"`
}

func (h CustodyTransferHop) Validate() error {
	if h.HopIndex < 1 {
		return errors.New("hopIndex must be >= 1")
	}
	if !h.Stage.IsValid() {
		return errors.New("invalid custody stage")
	}
	if strings.TrimSpace(h.FromCustodian) == "" {
		return errors.New("fromCustodian is required")
	}
	if strings.TrimSpace(h.ToCustodian) == "" {
		return errors.New("toCustodian is required")
	}
	if strings.TrimSpace(h.LocationLabel) == "" {
		return errors.New("locationLabel is required")
	}
	return nil
}

type PetCustodyChain struct {
	ChainID              string               `json:"chainId"`
	PetID                string               `json:"petId"`
	PetName              string               `json:"petName"`
	Species              string               `json:"species"`
	MicrochipID          string               `json:"microchipId,omitempty"`
	CurrentStage         CustodyStage         `json:"currentStage"`
	CurrentCustodian     string               `json:"currentCustodian"`
	CurrentLocation      string               `json:"currentLocation"`
	HandoverPinCode      string               `json:"handoverPinCode"`
	VerifiedOwnerName    string               `json:"verifiedOwnerName,omitempty"`
	OwnershipProof       OwnershipProofMethod `json:"ownershipProof,omitempty"`
	WaiverSigned         bool                 `json:"waiverSigned"`
	ReceiptCertificateID string               `json:"receiptCertificateId,omitempty"`
	ChainVerifiedIntact  bool                 `json:"chainVerifiedIntact"`
	LatestEntryHashHex   string               `json:"latestEntryHashHex"`
	Hops                 []CustodyTransferHop `json:"hops"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`
}

func (c PetCustodyChain) Validate() error {
	if strings.TrimSpace(c.ChainID) == "" {
		return errors.New("chainId is required")
	}
	if strings.TrimSpace(c.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(c.PetName) == "" {
		return errors.New("petName is required")
	}
	if !c.CurrentStage.IsValid() {
		return errors.New("invalid currentStage")
	}
	if strings.TrimSpace(c.CurrentCustodian) == "" {
		return errors.New("currentCustodian is required")
	}
	if strings.TrimSpace(c.CurrentLocation) == "" {
		return errors.New("currentLocation is required")
	}
	if len(c.Hops) == 0 {
		return errors.New("custody chain must contain at least one genesis hop")
	}
	for _, h := range c.Hops {
		if err := h.Validate(); err != nil {
			return err
		}
	}
	return nil
}
