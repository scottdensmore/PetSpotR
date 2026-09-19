package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestMicrochipDomainExtensions(t *testing.T) {
	t.Parallel()

	lost := domain.LostPetRecord{
		PetID:             "lost-pet-1",
		PetName:           "Max",
		Species:           domain.SpeciesDog,
		MicrochipID:       "985141000123456",
		MicrochipRegistry: "HomeAgain",
	}

	data, err := json.Marshal(lost)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var roundtrip domain.LostPetRecord
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if roundtrip.MicrochipID != "985141000123456" || roundtrip.MicrochipRegistry != "HomeAgain" {
		t.Errorf("microchip fields not preserved in LostPetRecord")
	}

	// Verify Public projection masks the microchip
	pubLost := lost.Public()
	if pubLost.MicrochipID != "HomeAgain ••••3456" {
		t.Errorf("expected masked microchip 'HomeAgain ••••3456', got %q", pubLost.MicrochipID)
	}
	if pubLost.MicrochipRegistry != "HomeAgain" {
		t.Errorf("expected registry 'HomeAgain', got %q", pubLost.MicrochipRegistry)
	}

	found := domain.FoundPetRecord{
		PetID:             "found-shelter-sea-01-99",
		Species:           domain.SpeciesDog,
		MicrochipID:       "985141000123456",
		MicrochipRegistry: "HomeAgain",
		ShelterID:         "shelter-sea-01",
		ShelterName:       "Seattle Animal Shelter",
		IntakeID:          "INT-2026-8819",
		CustodyStatus:     "Shelter Care",
	}

	foundData, _ := json.Marshal(found)
	var foundRoundtrip domain.FoundPetRecord
	_ = json.Unmarshal(foundData, &foundRoundtrip)

	if foundRoundtrip.ShelterName != "Seattle Animal Shelter" || foundRoundtrip.IntakeID != "INT-2026-8819" {
		t.Errorf("shelter fields not preserved in FoundPetRecord")
	}

	pubFound := found.Public()
	if pubFound.MicrochipID != "HomeAgain ••••3456" {
		t.Errorf("expected masked microchip on PublicFoundPetReport, got %q", pubFound.MicrochipID)
	}
	if pubFound.ShelterName != "Seattle Animal Shelter" || pubFound.CustodyStatus != "Shelter Care" {
		t.Errorf("shelter fields not preserved in PublicFoundPetReport")
	}

	match := domain.MatchRecord{
		MatchID:            "match-1",
		LostPetID:          lost.PetID,
		FoundPetID:         found.PetID,
		OverallScore:       1.0,
		DeterministicMatch: true,
		MatchType:          "deterministic_microchip",
		MatchedMicrochip:   "HomeAgain ••••3456",
	}

	matchData, _ := json.Marshal(match)
	var matchRoundtrip domain.MatchRecord
	_ = json.Unmarshal(matchData, &matchRoundtrip)

	if !matchRoundtrip.DeterministicMatch || matchRoundtrip.MatchType != "deterministic_microchip" {
		t.Errorf("deterministic match fields not preserved in MatchRecord")
	}
	if matchRoundtrip.MatchedMicrochip != "HomeAgain ••••3456" {
		t.Errorf("expected MatchedMicrochip 'HomeAgain ••••3456', got %q", matchRoundtrip.MatchedMicrochip)
	}
}

func TestNormalizeLostPetReport_MicrochipRegistryLookup(t *testing.T) {
	t.Parallel()

	report := domain.LostPetReport{
		PetID:       "lost-pet-2",
		PetName:     "Buddy",
		MicrochipID: "  985141000123456  ",
	}

	normalized := domain.NormalizeLostPetReport(report)
	if normalized.MicrochipID != "985141000123456" {
		t.Errorf("expected normalized MicrochipID '985141000123456', got %q", normalized.MicrochipID)
	}
	if normalized.MicrochipRegistry != "HomeAgain" {
		t.Errorf("expected resolved MicrochipRegistry 'HomeAgain', got %q", normalized.MicrochipRegistry)
	}
}

func TestNormalizeFoundPetReport_MicrochipAndShelterSanitization(t *testing.T) {
	t.Parallel()

	record := domain.FoundPetRecord{
		PetID:         "found-shelter-1",
		MicrochipID:   " 981010000123456 ",
		ShelterID:     " shelter-1 ",
		ShelterName:   " Seattle Animal Shelter ",
		IntakeID:      " INT-123 ",
		CustodyStatus: "Shelter Care",
	}

	normalized := domain.NormalizeFoundPetRecord(record)
	if normalized.MicrochipID != "981010000123456" {
		t.Errorf("expected normalized MicrochipID '981010000123456', got %q", normalized.MicrochipID)
	}
	if normalized.MicrochipRegistry != "AKC Reunite" {
		t.Errorf("expected resolved MicrochipRegistry 'AKC Reunite', got %q", normalized.MicrochipRegistry)
	}
	if normalized.ShelterID != "shelter-1" {
		t.Errorf("expected sanitized ShelterID 'shelter-1', got %q", normalized.ShelterID)
	}
	if normalized.ShelterName != "Seattle Animal Shelter" {
		t.Errorf("expected sanitized ShelterName 'Seattle Animal Shelter', got %q", normalized.ShelterName)
	}
	if normalized.IntakeID != "INT-123" {
		t.Errorf("expected sanitized IntakeID 'INT-123', got %q", normalized.IntakeID)
	}
	if normalized.CustodyStatus != domain.CustodyShelterCare {
		t.Errorf("expected CustodyStatus %q, got %q", domain.CustodyShelterCare, normalized.CustodyStatus)
	}
}
