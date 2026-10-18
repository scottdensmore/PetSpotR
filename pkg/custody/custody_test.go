package custody_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/custody"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestAppendCustodyHopAndVerifyIntegrity(t *testing.T) {
	secretKey := []byte("petspotr-test-secret-key-2026")
	t0 := time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)

	chain := domain.PetCustodyChain{
		ChainID:     "CHAIN-SEATTLE-01",
		PetID:       "PET-KODA-99",
		PetName:     "Koda",
		Species:     "DOG",
		MicrochipID: "985-112-0049281",
		CreatedAt:   t0,
	}

	// Genesis hop: FINDER_INTAKE
	var err error
	chain, err = custody.AppendCustodyHop(chain, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageFinderIntake,
		FromCustodian:  "UNATTENDED_FIELD_DISCOVERY",
		ToCustodian:    "Maya Lin (Good Samaritan Finder)",
		ToRole:         "GOOD_SAMARITAN_FINDER",
		LocationLabel:  "Discovery Park North Trailhead, Seattle",
		ConditionNotes: "Found shivering near lighthouse trail, wearing blue harness, no visible injuries",
		TransferredAt:  t0,
	}, secretKey)
	if err != nil {
		t.Fatalf("failed to append genesis hop: %v", err)
	}

	if len(chain.Hops) != 1 {
		t.Fatalf("expected 1 hop, got %d", len(chain.Hops))
	}
	if chain.Hops[0].HopIndex != 1 {
		t.Errorf("expected hop index 1, got %d", chain.Hops[0].HopIndex)
	}
	if chain.Hops[0].PrevHashHex != custody.GenesisPrevHash {
		t.Errorf("expected genesis prev hash %s, got %s", custody.GenesisPrevHash, chain.Hops[0].PrevHashHex)
	}
	if len(chain.Hops[0].EntryHashHex) != 64 {
		t.Errorf("expected 64-char sha256 entry hash, got %q", chain.Hops[0].EntryHashHex)
	}
	if len(chain.Hops[0].HmacSignatureHex) != 64 {
		t.Errorf("expected 64-char hmac signature, got %q", chain.Hops[0].HmacSignatureHex)
	}

	// Hop 2: FIELD_SAR_ESCORT
	chain, err = custody.AppendCustodyHop(chain, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageFieldSAREscort,
		FromCustodian:  chain.CurrentCustodian,
		ToCustodian:    "Ranger Marcus Vance (K9 SAR Unit 4)",
		ToRole:         "SAR_FIELD_RANGER",
		LocationLabel:  "Discovery Park South Parking Relay",
		ConditionNotes: "Scanned microchip 985-112-0049281; loaded into climate-controlled K9 transport crate",
		TransferredAt:  t0.Add(25 * time.Minute),
	}, secretKey)
	if err != nil {
		t.Fatalf("failed to append hop 2: %v", err)
	}

	// Hop 3: VET_ER_HOLD
	chain, err = custody.AppendCustodyHop(chain, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageVetERHold,
		FromCustodian:  chain.CurrentCustodian,
		ToCustodian:    "Dr. Elena Rostova (Emerald City Vet ER)",
		ToRole:         "EMERGENCY_VET_TRIAGE",
		LocationLabel:  "Emerald City Vet ER — Ballard",
		ConditionNotes: "Vitals normal, mild dehydration treated with subcutaneous fluids, cleared for owner release",
		TransferredAt:  t0.Add(65 * time.Minute),
	}, secretKey)
	if err != nil {
		t.Fatalf("failed to append hop 3: %v", err)
	}

	// Hop 4: OWNER_REUNIFICATION_ESCROW
	chain, err = custody.AppendCustodyHop(chain, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageOwnerEscrow,
		FromCustodian:  chain.CurrentCustodian,
		ToCustodian:    "Ballard Reunification Desk (Escrow Lock)",
		ToRole:         "REUNIFICATION_ESCROW_OFFICER",
		LocationLabel:  "Emerald City Vet ER — Release Lobby",
		ConditionNotes: "Awaiting dual-party 6-digit PIN verification & microchip deed confirmation",
		TransferredAt:  t0.Add(95 * time.Minute),
	}, secretKey)
	if err != nil {
		t.Fatalf("failed to append hop 4: %v", err)
	}

	if chain.CurrentStage != domain.CustodyStageOwnerEscrow {
		t.Errorf("expected stage %s, got %s", domain.CustodyStageOwnerEscrow, chain.CurrentStage)
	}

	intact, verifiedHops, detail := custody.VerifyCustodyChainIntegrity(chain, secretKey)
	if !intact {
		t.Fatalf("expected intact chain, got false: %s", detail)
	}
	if verifiedHops != 4 {
		t.Errorf("expected 4 verified hops, got %d", verifiedHops)
	}

	// Test tampering: modify hop 2 ToCustodian
	tamperedCustodian := chain
	tamperedCustodian.Hops = append([]domain.CustodyTransferHop(nil), chain.Hops...)
	tamperedCustodian.Hops[1].ToCustodian = "Unauthorized Actor"
	intact, badHop, detail := custody.VerifyCustodyChainIntegrity(tamperedCustodian, secretKey)
	if intact || badHop != 2 || !strings.Contains(detail, "entryHash tamper") {
		t.Errorf("expected tamper detection at hop 2 for ToCustodian change, got intact=%v badHop=%d detail=%q", intact, badHop, detail)
	}

	// Test tampering: modify hop 3 PrevHashHex
	tamperedPrev := chain
	tamperedPrev.Hops = append([]domain.CustodyTransferHop(nil), chain.Hops...)
	tamperedPrev.Hops[2].PrevHashHex = custody.GenesisPrevHash
	intact, badHop, detail = custody.VerifyCustodyChainIntegrity(tamperedPrev, secretKey)
	if intact || badHop != 3 || !strings.Contains(detail, "prevHash mismatch") {
		t.Errorf("expected prevHash mismatch at hop 3, got intact=%v badHop=%d detail=%q", intact, badHop, detail)
	}

	// Test tampering: modify hop 4 EntryHashHex
	tamperedEntry := chain
	tamperedEntry.Hops = append([]domain.CustodyTransferHop(nil), chain.Hops...)
	tamperedEntry.Hops[3].EntryHashHex = strings.Repeat("a", 64)
	intact, badHop, detail = custody.VerifyCustodyChainIntegrity(tamperedEntry, secretKey)
	if intact || badHop != 4 || !strings.Contains(detail, "entryHash tamper") {
		t.Errorf("expected entryHash tamper at hop 4, got intact=%v badHop=%d detail=%q", intact, badHop, detail)
	}

	// Test tampering: modify hop 1 HmacSignatureHex
	tamperedHMAC := chain
	tamperedHMAC.Hops = append([]domain.CustodyTransferHop(nil), chain.Hops...)
	tamperedHMAC.Hops[0].HmacSignatureHex = strings.Repeat("f", 64)
	intact, badHop, detail = custody.VerifyCustodyChainIntegrity(tamperedHMAC, secretKey)
	if intact || badHop != 1 || !strings.Contains(detail, "HMAC-SHA256 signature verification failed") {
		t.Errorf("expected HMAC failure at hop 1, got intact=%v badHop=%d detail=%q", intact, badHop, detail)
	}
}

func TestDualPartyPINAndReunionHandover(t *testing.T) {
	secretKey := []byte("petspotr-test-secret-key-2026")
	t0 := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)

	pin1 := custody.GenerateHandoverPIN("CHAIN-SEATTLE-01", "PET-KODA-99", secretKey)
	pin2 := custody.GenerateHandoverPIN("CHAIN-SEATTLE-01", "PET-KODA-99", secretKey)
	if len(pin1) != 6 || pin1 != pin2 {
		t.Fatalf("expected deterministic 6-digit PIN, got %q and %q", pin1, pin2)
	}

	chain := domain.PetCustodyChain{
		ChainID:         "CHAIN-SEATTLE-01",
		PetID:           "PET-KODA-99",
		PetName:         "Koda",
		Species:         "DOG",
		HandoverPinCode: pin1,
		CreatedAt:       t0,
	}
	var err error
	chain, err = custody.AppendCustodyHop(chain, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageFinderIntake,
		FromCustodian:  "UNATTENDED_FIELD_DISCOVERY",
		ToCustodian:    "Maya Lin",
		ToRole:         "GOOD_SAMARITAN_FINDER",
		LocationLabel:  "Discovery Park",
		ConditionNotes: "Intake",
		TransferredAt:  t0,
	}, secretKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reject wrong PIN
	_, err = custody.VerifyAndCompleteReunionHandover(
		chain,
		"000000",
		"Devon Mercer",
		"Emerald City Vet ER Lobby",
		domain.ProofMicrochipMatch,
		true,
		secretKey,
		t0.Add(time.Hour),
	)
	if !errors.Is(err, custody.ErrInvalidHandoverPIN) {
		t.Fatalf("expected ErrInvalidHandoverPIN, got %v", err)
	}

	// Reject unsigned waiver
	_, err = custody.VerifyAndCompleteReunionHandover(
		chain,
		pin1,
		"Devon Mercer",
		"Emerald City Vet ER Lobby",
		domain.ProofMicrochipMatch,
		false,
		secretKey,
		t0.Add(time.Hour),
	)
	if err == nil || !strings.Contains(err.Error(), "waiver") {
		t.Fatalf("expected waiver error, got %v", err)
	}

	// Succeed with valid PIN & signed waiver
	reunited, err := custody.VerifyAndCompleteReunionHandover(
		chain,
		pin1,
		"Devon Mercer",
		"Emerald City Vet ER Lobby",
		domain.ProofMicrochipMatch,
		true,
		secretKey,
		t0.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("expected reunion handover success, got %v", err)
	}
	if reunited.CurrentStage != domain.CustodyStageReunited {
		t.Errorf("expected stage %s, got %s", domain.CustodyStageReunited, reunited.CurrentStage)
	}
	if reunited.VerifiedOwnerName != "Devon Mercer" {
		t.Errorf("expected owner Devon Mercer, got %s", reunited.VerifiedOwnerName)
	}
	if !reunited.WaiverSigned {
		t.Errorf("expected WaiverSigned=true")
	}
	if !strings.HasPrefix(reunited.ReceiptCertificateID, "CERT-REUNION-") {
		t.Errorf("expected CERT-REUNION-... certificate, got %s", reunited.ReceiptCertificateID)
	}

	intact, hops, _ := custody.VerifyCustodyChainIntegrity(reunited, secretKey)
	if !intact || hops != 2 {
		t.Errorf("expected intact chain with 2 hops after reunion, got intact=%v hops=%d", intact, hops)
	}
}
