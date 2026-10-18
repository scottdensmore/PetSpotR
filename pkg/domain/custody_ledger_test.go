package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestCustodyLedgerDomainValidationAndSerialization(t *testing.T) {
	if store.CollectionPetCustodyChains != "pet_custody_chains" {
		t.Fatalf("expected CollectionPetCustodyChains to be pet_custody_chains, got %q", store.CollectionPetCustodyChains)
	}

	validStages := []domain.CustodyStage{
		domain.CustodyStageFinderIntake,
		domain.CustodyStageFieldSAREscort,
		domain.CustodyStageVetERHold,
		domain.CustodyStageShelterHold,
		domain.CustodyStageOwnerEscrow,
		domain.CustodyStageReunited,
	}
	for _, stage := range validStages {
		if !stage.IsValid() {
			t.Errorf("expected stage %q to be valid", stage)
		}
	}
	if domain.CustodyStage("UNKNOWN_STAGE").IsValid() {
		t.Error("expected UNKNOWN_STAGE to be invalid")
	}

	validProofs := []domain.OwnershipProofMethod{
		domain.ProofMicrochipMatch,
		domain.ProofVetRecordDeed,
		domain.ProofPhotoBiometric,
		domain.ProofGovIDAffidavit,
	}
	for _, proof := range validProofs {
		if !proof.IsValid() {
			t.Errorf("expected ownership proof %q to be valid", proof)
		}
	}
	if domain.OwnershipProofMethod("HANDSHAKE").IsValid() {
		t.Error("expected HANDSHAKE ownership proof to be invalid")
	}

	now := time.Date(2026, 10, 17, 14, 30, 0, 0, time.UTC)
	validHop := domain.CustodyTransferHop{
		HopIndex:         1,
		Stage:            domain.CustodyStageFinderIntake,
		FromCustodian:    "FIELD_RECOVERY_ORIGIN",
		ToCustodian:      "Unit SAR-4 (Elena Rostova)",
		ToRole:           "FIELD_RESCUER",
		LocationLabel:    "Sector 7 Creek Bed, Portland OR",
		ConditionNotes:   "Mild dehydration, scanning collar intact",
		TransferredAt:    now,
		PrevHashHex:      strings.Repeat("0", 64),
		EntryHashHex:     strings.Repeat("a", 64),
		HmacSignatureHex: strings.Repeat("b", 64),
	}
	if err := validHop.Validate(); err != nil {
		t.Fatalf("expected valid hop to pass validation, got: %v", err)
	}

	hopCases := []struct {
		name    string
		mutate  func(*domain.CustodyTransferHop)
		wantErr string
	}{
		{
			name:    "invalid hopIndex",
			mutate:  func(h *domain.CustodyTransferHop) { h.HopIndex = 0 },
			wantErr: "hopIndex must be >= 1",
		},
		{
			name:    "invalid stage",
			mutate:  func(h *domain.CustodyTransferHop) { h.Stage = "BAD" },
			wantErr: "invalid custody stage",
		},
		{
			name:    "missing fromCustodian",
			mutate:  func(h *domain.CustodyTransferHop) { h.FromCustodian = "  " },
			wantErr: "fromCustodian is required",
		},
		{
			name:    "missing toCustodian",
			mutate:  func(h *domain.CustodyTransferHop) { h.ToCustodian = "" },
			wantErr: "toCustodian is required",
		},
		{
			name:    "missing locationLabel",
			mutate:  func(h *domain.CustodyTransferHop) { h.LocationLabel = " " },
			wantErr: "locationLabel is required",
		},
	}

	for _, tc := range hopCases {
		t.Run("hop_"+tc.name, func(t *testing.T) {
			hop := validHop
			tc.mutate(&hop)
			err := hop.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}

	validChain := domain.PetCustodyChain{
		ChainID:              "chain-101",
		PetID:                "pet-901",
		PetName:              "Cooper",
		Species:              "DOG",
		MicrochipID:          "985-112-004918273",
		CurrentStage:         domain.CustodyStageFinderIntake,
		CurrentCustodian:     "Unit SAR-4 (Elena Rostova)",
		CurrentLocation:      "Sector 7 Creek Bed, Portland OR",
		HandoverPinCode:      "482910",
		VerifiedOwnerName:    "Marcus Vance",
		OwnershipProof:       domain.ProofMicrochipMatch,
		WaiverSigned:         true,
		ReceiptCertificateID: "CERT-REUNION-101",
		ChainVerifiedIntact:  true,
		LatestEntryHashHex:   validHop.EntryHashHex,
		Hops:                 []domain.CustodyTransferHop{validHop},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := validChain.Validate(); err != nil {
		t.Fatalf("expected valid chain to pass validation, got: %v", err)
	}

	chainCases := []struct {
		name    string
		mutate  func(*domain.PetCustodyChain)
		wantErr string
	}{
		{
			name:    "missing chainId",
			mutate:  func(c *domain.PetCustodyChain) { c.ChainID = " " },
			wantErr: "chainId is required",
		},
		{
			name:    "missing petId",
			mutate:  func(c *domain.PetCustodyChain) { c.PetID = "" },
			wantErr: "petId is required",
		},
		{
			name:    "missing petName",
			mutate:  func(c *domain.PetCustodyChain) { c.PetName = "  " },
			wantErr: "petName is required",
		},
		{
			name:    "invalid currentStage",
			mutate:  func(c *domain.PetCustodyChain) { c.CurrentStage = "INVALID" },
			wantErr: "invalid currentStage",
		},
		{
			name:    "missing currentCustodian",
			mutate:  func(c *domain.PetCustodyChain) { c.CurrentCustodian = "" },
			wantErr: "currentCustodian is required",
		},
		{
			name:    "missing currentLocation",
			mutate:  func(c *domain.PetCustodyChain) { c.CurrentLocation = "" },
			wantErr: "currentLocation is required",
		},
		{
			name:    "empty hops",
			mutate:  func(c *domain.PetCustodyChain) { c.Hops = nil },
			wantErr: "custody chain must contain at least one genesis hop",
		},
		{
			name: "invalid nested hop",
			mutate: func(c *domain.PetCustodyChain) {
				badHop := validHop
				badHop.ToCustodian = ""
				c.Hops = []domain.CustodyTransferHop{badHop}
			},
			wantErr: "toCustodian is required",
		},
	}

	for _, tc := range chainCases {
		t.Run("chain_"+tc.name, func(t *testing.T) {
			chain := validChain
			tc.mutate(&chain)
			err := chain.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}

	raw, err := json.Marshal(validChain)
	if err != nil {
		t.Fatalf("failed to marshal validChain: %v", err)
	}
	var decoded domain.PetCustodyChain
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal validChain: %v", err)
	}
	if decoded.ChainID != validChain.ChainID || len(decoded.Hops) != 1 || decoded.Hops[0].EntryHashHex != validHop.EntryHashHex {
		t.Fatalf("round-trip mismatch: got %+v", decoded)
	}
}
