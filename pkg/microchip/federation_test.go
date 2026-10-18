package microchip

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDecodeTransponderManufacturer(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantNorm   string
		wantMask   string
		wantStd    domain.MicrochipStandardType
		wantFreq   float64
		wantMfg    string
		wantMfgNm  string
		wantLot    string
		wantErrSub string
	}{
		{
			name:      "ISO 15 HomeAgain 985",
			raw:       "985-1120-0458-9120",
			wantNorm:  "985112004589120",
			wantMask:  "985 ••••9120",
			wantStd:   domain.ChipStandardISO11784_15,
			wantFreq:  134.2,
			wantMfg:   "985",
			wantMfgNm: "HomeAgain / Merck Animal Health",
			wantLot:   "LOT-MERCK-1120",
		},
		{
			name:      "ISO 15 AKC Reunite 981",
			raw:       "981020019283746",
			wantNorm:  "981020019283746",
			wantMask:  "981 ••••3746",
			wantStd:   domain.ChipStandardISO11784_15,
			wantFreq:  134.2,
			wantMfg:   "981",
			wantMfgNm: "AKC Reunite / Datamars",
			wantLot:   "LOT-DATAMARS-0200",
		},
		{
			name:      "ISO 15 24PetWatch 982",
			raw:       "982000405511223",
			wantNorm:  "982000405511223",
			wantMask:  "982 ••••1223",
			wantStd:   domain.ChipStandardISO11784_15,
			wantFreq:  134.2,
			wantMfg:   "982",
			wantMfgNm: "24PetWatch / Allflex",
			wantLot:   "LOT-ALLFLEX-0004",
		},
		{
			name:      "ISO 15 Michelson Found Animals 941",
			raw:       "941000024681357",
			wantNorm:  "941000024681357",
			wantMask:  "941 ••••1357",
			wantStd:   domain.ChipStandardISO11784_15,
			wantFreq:  134.2,
			wantMfg:   "941",
			wantMfgNm: "Michelson Found Animals / Felixcan",
			wantLot:   "LOT-FELIX-0000",
		},
		{
			name:      "ISO 15 Unallocated Wholesale 900",
			raw:       "900881200334455",
			wantNorm:  "900881200334455",
			wantMask:  "900 ••••4455",
			wantStd:   domain.ChipStandardISO11784_15,
			wantFreq:  134.2,
			wantMfg:   "900",
			wantMfgNm: "Unallocated ISO Shared Lot (Wholesale)",
			wantLot:   "LOT-ISO900-8812",
		},
		{
			name:      "ISO 15 Other Global Manufacturer 956",
			raw:       "956000011223344",
			wantNorm:  "956000011223344",
			wantMask:  "956 ••••3344",
			wantStd:   domain.ChipStandardISO11784_15,
			wantFreq:  134.2,
			wantMfg:   "956",
			wantMfgNm: "ISO 11784/11785 Global Manufacturer",
			wantLot:   "LOT-ISO-0000",
		},
		{
			name:      "AVID 9-digit encrypted",
			raw:       "041-298-776",
			wantNorm:  "041298776",
			wantMask:  "AVID ••••8776",
			wantStd:   domain.ChipStandardAVID9,
			wantFreq:  125.0,
			wantMfg:   "AVID",
			wantMfgNm: "AVID Identification Systems (PETtrac)",
			wantLot:   "LOT-AVID-0412",
		},
		{
			name:      "Euro 10-digit hex FDX-A",
			raw:       "7f-00-4a-9b-1c",
			wantNorm:  "7F004A9B1C",
			wantMask:  "FDX-A ••••9B1C",
			wantStd:   domain.ChipStandardHex10Legacy,
			wantFreq:  128.0,
			wantMfg:   "FDX-A",
			wantMfgNm: "Legacy FDX-A / Trovan-Banfield Hex",
			wantLot:   "LOT-FDXA-7F00",
		},
		{
			name:       "Invalid length",
			raw:        "12345",
			wantErrSub: "invalid microchip length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			norm, mask, std, freq, mfg, mfgNm, lot, err := DecodeTransponderManufacturer(tc.raw)
			if tc.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if norm != tc.wantNorm || mask != tc.wantMask || std != tc.wantStd || freq != tc.wantFreq || mfg != tc.wantMfg || mfgNm != tc.wantMfgNm || lot != tc.wantLot {
				t.Fatalf("got (%q, %q, %q, %v, %q, %q, %q), want (%q, %q, %q, %v, %q, %q, %q)",
					norm, mask, std, freq, mfg, mfgNm, lot,
					tc.wantNorm, tc.wantMask, tc.wantStd, tc.wantFreq, tc.wantMfg, tc.wantMfgNm, tc.wantLot)
			}
		})
	}
}

func TestEvaluateFederatedScan(t *testing.T) {
	now := time.Date(2026, 10, 17, 15, 0, 0, 0, time.UTC)
	loc := domain.GeoPoint{Latitude: 47.6062, Longitude: -122.3321}

	t.Run("Registered primary owner match (985 default)", func(t *testing.T) {
		sc, err := EvaluateFederatedScan(FederatedScanInput{
			CaseID:          "SCAN-001",
			PetID:           "PET-001",
			PetName:         "Buster",
			Species:         "", // should default to DOG
			RawChipInput:    "985-1120-0458-9120",
			ScannerLocation: loc,
			ScannedByAgency: "", // should default to Universal RFID Field Scanner
		}, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.Status != domain.ChipStatusRegisteredOwnerFound {
			t.Fatalf("expected status %s, got %s", domain.ChipStatusRegisteredOwnerFound, sc.Status)
		}
		if sc.WinningRegistry != "HomeAgain" {
			t.Fatalf("expected WinningRegistry HomeAgain, got %q", sc.WinningRegistry)
		}
		if sc.Species != "DOG" {
			t.Fatalf("expected default species DOG, got %q", sc.Species)
		}
		if sc.ScannedByAgency != "Universal RFID Field Scanner" {
			t.Fatalf("expected default ScannedByAgency, got %q", sc.ScannedByAgency)
		}
		if sc.ResolvedOwnerName != "Jordan & Taylor Brooks" {
			t.Fatalf("unexpected owner %q", sc.ResolvedOwnerName)
		}
		if len(sc.RegistryHops) != 4 {
			t.Fatalf("expected 4 registry hops, got %d", len(sc.RegistryHops))
		}
	})

	t.Run("Unregistered dead-end batch trace (900 or UNREGISTERED_DEAD_END)", func(t *testing.T) {
		sc, err := EvaluateFederatedScan(FederatedScanInput{
			CaseID:          "SCAN-002",
			PetID:           "PET-002",
			PetName:         "Luna",
			Species:         "CAT",
			RawChipInput:    "900881200334455",
			ScannerLocation: loc,
			ScannedByAgency: "Seattle Animal Shelter Intake #2",
		}, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.Status != domain.ChipStatusUnregisteredBatchTraced {
			t.Fatalf("expected status %s, got %s", domain.ChipStatusUnregisteredBatchTraced, sc.Status)
		}
		if sc.ImplantingClinicName != "Emerald City Spay/Neuter & Wellness Clinic" {
			t.Fatalf("unexpected clinic %q", sc.ImplantingClinicName)
		}
		if sc.ImplantPatientChartID != "PIMS-CHART-4455" {
			t.Fatalf("unexpected chart ID %q", sc.ImplantPatientChartID)
		}
		for _, hop := range sc.RegistryHops {
			if hop.RecordFound {
				t.Fatalf("expected 0 records found in consumer registries, but %s matched", hop.RegistryName)
			}
		}
	})

	t.Run("Outdated primary phone secondary contact escalation (AVID or OUTDATED_PRIMARY_PHONE)", func(t *testing.T) {
		sc, err := EvaluateFederatedScan(FederatedScanInput{
			CaseID:          "SCAN-003",
			PetID:           "PET-003",
			PetName:         "Milo",
			Species:         "DOG",
			RawChipInput:    "041-298-776",
			ScannerLocation: loc,
			ScannedByAgency: "SAR Field Unit K9-04",
		}, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.Status != domain.ChipStatusSecondaryContactEscalated {
			t.Fatalf("expected status %s, got %s", domain.ChipStatusSecondaryContactEscalated, sc.Status)
		}
		if sc.WinningRegistry != "AVID PETtrac" {
			t.Fatalf("expected WinningRegistry AVID PETtrac, got %q", sc.WinningRegistry)
		}
		if !strings.Contains(sc.ResolvedOwnerName, "Elena Vance") {
			t.Fatalf("unexpected owner %q", sc.ResolvedOwnerName)
		}
	})
}

func TestTraceManufacturerBatchLotAndResolveOwnership(t *testing.T) {
	now := time.Date(2026, 10, 17, 15, 0, 0, 0, time.UTC)
	loc := domain.GeoPoint{Latitude: 47.6062, Longitude: -122.3321}

	sc, err := EvaluateFederatedScan(FederatedScanInput{
		CaseID:          "SCAN-004",
		PetID:           "PET-004",
		PetName:         "Shadow",
		Species:         "CAT",
		RawChipInput:    "900881200334455",
		ScannerLocation: loc,
		ScannedByAgency: "King County Shelter",
		SimulateMode:    "UNREGISTERED_DEAD_END",
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	later := now.Add(10 * time.Minute)
	if err := TraceManufacturerBatchLot(sc, LotTraceInput{}, later); err != nil {
		t.Fatalf("TraceManufacturerBatchLot failed: %v", err)
	}
	if sc.Status != domain.ChipStatusUnregisteredBatchTraced {
		t.Fatalf("expected status to remain %s, got %s", domain.ChipStatusUnregisteredBatchTraced, sc.Status)
	}
	if sc.ResolvedOwnerName != "Devon & Priya Patel (Recovered via Clinic PIMS)" {
		t.Fatalf("unexpected recovered owner %q", sc.ResolvedOwnerName)
	}
	if !strings.Contains(sc.ResolutionNotes, "LOT TRACE COMPLETE") {
		t.Fatalf("unexpected resolution notes: %q", sc.ResolutionNotes)
	}

	dispatchTime := later.Add(5 * time.Minute)
	if err := ResolveMicrochipOwnership(sc, OwnershipResolutionInput{
		VerifiedOwnerName:      "Devon Patel",
		VerifiedOwnerPhoneMask: "+1 (206) •••-7724",
		VerificationMethod:     "CLINIC_PIMS_LOT_TRACE",
		Notes:                  "Confirmed rabies tag and implant certificate with Northwest Veterinary Specialists.",
	}, dispatchTime); err != nil {
		t.Fatalf("ResolveMicrochipOwnership failed: %v", err)
	}
	if sc.Status != domain.ChipStatusReunionDispatchedVerified {
		t.Fatalf("expected status %s, got %s", domain.ChipStatusReunionDispatchedVerified, sc.Status)
	}
	if sc.ResolvedOwnerName != "Devon Patel" {
		t.Fatalf("expected ResolvedOwnerName Devon Patel, got %q", sc.ResolvedOwnerName)
	}
	if !strings.Contains(sc.ResolutionNotes, "REUNION DISPATCHED (CLINIC_PIMS_LOT_TRACE)") {
		t.Fatalf("unexpected resolution notes: %q", sc.ResolutionNotes)
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("expected valid scanCase after ownership resolution, got: %v", err)
	}

	if err := TraceManufacturerBatchLot(nil, LotTraceInput{}, later); err == nil {
		t.Fatalf("expected error for nil scanCase in TraceManufacturerBatchLot")
	}
	if err := ResolveMicrochipOwnership(nil, OwnershipResolutionInput{Notes: "ok"}, dispatchTime); err == nil {
		t.Fatalf("expected error for nil scanCase in ResolveMicrochipOwnership")
	}
	if err := ResolveMicrochipOwnership(sc, OwnershipResolutionInput{Notes: "   "}, dispatchTime); err == nil {
		t.Fatalf("expected error for empty notes in ResolveMicrochipOwnership")
	}
}
