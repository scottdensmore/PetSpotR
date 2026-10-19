package vectorbornerickettsial_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/vectorbornerickettsial"
)

func TestVectorBorneSeededOperationsAndOverview(t *testing.T) {
	svc := vectorbornerickettsial.NewVectorBorneService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	// Verify VBR-OP-101 ("Yukon")
	op101, err := svc.GetOperation("VBR-OP-101")
	if err != nil {
		t.Fatalf("expected VBR-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Yukon" || op101.Species != "Canine" || op101.Breed != "Golden Retriever" || op101.WeightKg != 31.5 {
		t.Errorf("unexpected VBR-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.VectorBorneStatusAntimicrobialStaged ||
		op101.Syndrome != domain.VectorBorneSyndromeLymeNephritis ||
		op101.Assay != domain.VectorBorneAssaySNAP4DxPlusC6 ||
		op101.AntimicrobialProtocol != domain.VectorBorneAntimicrobialMycophenolateLymePLN {
		t.Errorf("unexpected VBR-OP-101 classification: %+v", op101)
	}
	if op101.PlateletCountKUL != 42.0 || op101.PCVPercent != 26.5 || op101.UrinePCRatio != 4.8 || op101.SerumAlbuminGDL != 1.6 {
		t.Errorf("unexpected VBR-OP-101 metrics: %+v", op101)
	}
	if !op101.LymeNephritisUPCPositive || !op101.EsophagealWaterChaseVerified || len(op101.HematologyChecks) != 2 {
		t.Errorf("unexpected VBR-OP-101 flags/checks: %+v", op101)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected VBR-OP-101 to validate, got %v", err)
	}

	// Verify VBR-OP-102 ("Willow")
	op102, err := svc.GetOperation("VBR-OP-102")
	if err != nil {
		t.Fatalf("expected VBR-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Willow" || op102.Species != "Feline" || op102.Status != domain.VectorBorneStatusThrombocytopeniaMonitored ||
		op102.Syndrome != domain.VectorBorneSyndromeCytauxzoonosis {
		t.Errorf("unexpected VBR-OP-102 state: %+v", op102)
	}
	if op102.AtovaquoneMgKgTID != 15.0 || len(op102.HematologyChecks) != 1 {
		t.Errorf("unexpected VBR-OP-102 atovaquone/checks: %+v", op102)
	}

	// Verify VBR-OP-103 ("Boone")
	op103, err := svc.GetOperation("VBR-OP-103")
	if err != nil {
		t.Fatalf("expected VBR-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Boone" || op103.Status != domain.VectorBorneStatusPCRNegativeCleared ||
		op103.Syndrome != domain.VectorBorneSyndromeBabesiosis || op103.Clearance == nil {
		t.Errorf("unexpected VBR-OP-103 state: %+v", op103)
	}
	if !op103.AtropinePremedVerified || op103.ImidocarbMgKgIM != 6.6 || len(op103.HematologyChecks) != 2 {
		t.Errorf("unexpected VBR-OP-103 imidocarb/checks: %+v", op103)
	}

	// Filter by status
	stagedOps := svc.ListOperations(string(domain.VectorBorneStatusAntimicrobialStaged))
	if len(stagedOps) != 1 || stagedOps[0].ID != "VBR-OP-101" {
		t.Errorf("expected 1 DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED op, got %+v", stagedOps)
	}

	ov := svc.CalculateOverview()
	ovAlias := svc.GetOverviewMetrics()
	if ov != ovAlias {
		t.Errorf("expected CalculateOverview and GetOverviewMetrics to match: %+v vs %+v", ov, ovAlias)
	}
	if ov.TotalOperations != 3 ||
		ov.AntimicrobialStagedCount != 1 ||
		ov.ThrombocytopeniaMonitoredCount != 1 ||
		ov.PCRNegativeClearedCount != 1 ||
		ov.LymeNephritisProteinuriaCount != 1 ||
		ov.SevereThrombocytopeniaCount != 2 ||
		ov.BabesiaCytauxzoonProtozoalCount != 2 {
		t.Errorf("unexpected VectorBorneOverview: %+v", ov)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestVectorBorneCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := vectorbornerickettsial.NewVectorBorneService()

	// Create operation with default fallbacks for species, breed, assay, and antimicrobial protocol
	created, err := svc.CreateOperation(
		"CASE-VBR-104",
		"Scout",
		"",
		"",
		18.2,
		domain.VectorBorneSyndromeEhrlichiaAnaplasma,
		"",
		"",
		38.0, 27.0, 0.3, 2.6,
		10.0, 0.0, 0.0,
		true, false,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "VBR-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" ||
		created.Assay != domain.VectorBorneAssaySNAP4DxPlusC6 ||
		created.AntimicrobialProtocol != domain.VectorBorneAntimicrobialDoxycyclineWaterChase {
		t.Errorf("unexpected created operation defaults: %+v", created)
	}

	// Create Cytauxzoonosis operation with default species/protocol
	createdCytaux, err := svc.CreateOperation(
		"CASE-VBR-105",
		"Cleo",
		"",
		"Siamese",
		3.8,
		domain.VectorBorneSyndromeCytauxzoonosis,
		domain.VectorBorneAssayBuffyCoatSmear,
		"",
		64.0, 24.0, 0.2, 2.4,
		10.0, 15.0, 0.0,
		true, false,
	)
	if err != nil {
		t.Fatalf("unexpected Cytaux CreateOperation error: %v", err)
	}
	if createdCytaux.Species != "Feline" || createdCytaux.AntimicrobialProtocol != domain.VectorBorneAntimicrobialAtovaquoneAzithromycin {
		t.Errorf("unexpected Cytaux defaults: %+v", createdCytaux)
	}

	// Create Lyme Nephritis and Babesiosis operations with default protocols
	createdLyme, err := svc.CreateOperation(
		"CASE-VBR-106",
		"Bodie",
		"Canine",
		"Labrador Retriever",
		29.0,
		domain.VectorBorneSyndromeLymeNephritis,
		domain.VectorBorneAssayUrinePCRatio,
		"",
		85.0, 31.0, 3.6, 1.9,
		10.0, 0.0, 0.0,
		true, false,
	)
	if err != nil || createdLyme.AntimicrobialProtocol != domain.VectorBorneAntimicrobialMycophenolateLymePLN {
		t.Fatalf("unexpected Lyme CreateOperation result: %+v, err=%v", createdLyme, err)
	}

	createdBabesia, err := svc.CreateOperation(
		"CASE-VBR-107",
		"Rex",
		"Canine",
		"Greyhound",
		27.0,
		domain.VectorBorneSyndromeBabesiosis,
		domain.VectorBorneAssayQuantitativePCR,
		"",
		55.0, 22.0, 0.4, 2.8,
		10.0, 13.5, 6.6,
		true, true,
	)
	if err != nil || createdBabesia.AntimicrobialProtocol != domain.VectorBorneAntimicrobialImidocarbAtropine {
		t.Fatalf("unexpected Babesia CreateOperation result: %+v, err=%v", createdBabesia, err)
	}

	// Stage antimicrobial with custom notes and default doses
	staged, err := svc.StageAntimicrobial(
		created.ID,
		domain.VectorBorneAntimicrobialDoxycyclineWaterChase,
		0, 0, 0,
		true, false,
		"Administer with meal",
		"Monitor UPC weekly",
		"",
		"Custom rationale for Ehrlichia morulae",
	)
	if err != nil {
		t.Fatalf("unexpected StageAntimicrobial error: %v", err)
	}
	if staged.Status != domain.VectorBorneStatusAntimicrobialStaged || staged.DoxycyclineMgKgDay != 10.0 {
		t.Errorf("unexpected staged state: %+v", staged)
	}
	if staged.AntimicrobialStaging.FattyMealBioavailabilityNote != "Administer with meal" ||
		staged.AntimicrobialStaging.LymeNephritisImmunomodulationNote != "Monitor UPC weekly" ||
		staged.AntimicrobialStaging.ClinicalRationale != "Custom rationale for Ehrlichia morulae" {
		t.Errorf("unexpected custom staging notes: %+v", staged.AntimicrobialStaging)
	}

	// Stage Atovaquone & Imidocarb default doses
	stagedAtov, err := svc.StageAntimicrobial(
		createdCytaux.ID,
		domain.VectorBorneAntimicrobialAtovaquoneAzithromycin,
		0, 0, 0,
		true, false,
		"", "", "Dr. Mara Vance", "",
	)
	if err != nil || stagedAtov.AtovaquoneMgKgTID != 15.0 {
		t.Errorf("expected default Atovaquone 15.0 mg/kg TID, got %+v, err=%v", stagedAtov, err)
	}

	stagedImido, err := svc.StageAntimicrobial(
		createdBabesia.ID,
		domain.VectorBorneAntimicrobialImidocarbAtropine,
		0, 0, 0,
		true, true,
		"", "", "Dr. Mara Vance", "",
	)
	if err != nil || stagedImido.ImidocarbMgKgIM != 6.6 {
		t.Errorf("expected default Imidocarb 6.6 mg/kg IM, got %+v, err=%v", stagedImido, err)
	}

	// Record hematology check and verify status transitions to THROMBOCYTOPENIA_PROTEINURIA_MONITORED
	checked, err := svc.RecordHematologyCheck(
		created.ID,
		168.0, 38.5, 0.2, 3.2,
		true,
		"Dr. Mara Vance, DACVIM (Infectious Disease)",
		"Day 14 recheck: Morulae cleared on buffy-coat smear, platelets rebounded to 168 K/uL, PCV 38.5%, UPC 0.2.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordHematologyCheck error: %v", err)
	}
	if checked.Status != domain.VectorBorneStatusThrombocytopeniaMonitored || len(checked.HematologyChecks) != 1 {
		t.Errorf("expected status %s and 1 check, got %s (%d)", domain.VectorBorneStatusThrombocytopeniaMonitored, checked.Status, len(checked.HematologyChecks))
	}
	if checked.PlateletCountKUL != 168.0 || checked.PCVPercent != 38.5 || checked.UrinePCRatio != 0.2 || checked.SerumAlbuminGDL != 3.2 {
		t.Errorf("unexpected updated vitals after check: %+v", checked)
	}

	// Evaluate clearance and verify status transitions to PCR_NEGATIVE_PLATELETS_RENAL_CLEARED
	cleared, err := svc.EvaluateClearance(
		created.ID,
		195.0, 40.0, 0.18,
		true, true,
		"Dr. Mara Vance, DACVIM (Infectious Disease)",
		"Quantitative PCR negative for Ehrlichia/Anaplasma, platelets 195 K/uL (>= 150), PCV 40.0% (>= 35%), UPC 0.18 (< 0.5), normothermic and eating.",
	)
	if err != nil {
		t.Fatalf("unexpected EvaluateClearance error: %v", err)
	}
	if cleared.Status != domain.VectorBorneStatusPCRNegativeCleared || cleared.Clearance == nil || cleared.DischargedAt == nil {
		t.Errorf("expected cleared state, got %+v", cleared)
	}
}

func TestVectorBorneValidationAndSafetyGuards(t *testing.T) {
	svc := vectorbornerickettsial.NewVectorBorneService()

	createErrCases := []struct {
		name       string
		caseID     string
		petName    string
		species    string
		weightKg   float64
		syndrome   domain.VectorBorneSyndrome
		assay      domain.VectorBorneAssay
		protocol   domain.VectorBorneAntimicrobial
		platelets  float64
		pcv        float64
		upc        float64
		albumin    float64
		doxy       float64
		atovaquone float64
		imidocarb  float64
		waterChase bool
		atropine   bool
	}{
		{"empty case_id", "", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"empty pet_name", "CASE-1", "", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"weight <= 0", "CASE-1", "Scout", "Canine", 0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"invalid syndrome", "CASE-1", "Scout", "Canine", 15.0, "BAD_SYNDROME", domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"invalid assay", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, "BAD_ASSAY", domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"invalid protocol", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, "BAD_PROTOCOL", 45.0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"platelets <= 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 0, 30.0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"pcv <= 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 0, 0.2, 2.8, 10.0, 0, 0, true, false},
		{"upc < 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, -0.1, 2.8, 10.0, 0, 0, true, false},
		{"albumin <= 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 0, 10.0, 0, 0, true, false},
		{"doxy < 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, -1.0, 0, 0, true, false},
		{"atovaquone < 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, -1.0, 0, true, false},
		{"imidocarb < 0", "CASE-1", "Scout", "Canine", 15.0, domain.VectorBorneSyndromeRMSFVasculitis, domain.VectorBorneAssaySNAP4DxPlusC6, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 45.0, 30.0, 0.2, 2.8, 10.0, 0, -1.0, true, false},
		{"feline doxy without water chase", "CASE-1", "Willow", "Feline", 4.2, domain.VectorBorneSyndromeEhrlichiaAnaplasma, domain.VectorBorneAssayQuantitativePCR, domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 60.0, 28.0, 0.2, 2.6, 10.0, 0, 0, false, false},
		{"imidocarb without atropine premed", "CASE-1", "Boone", "Canine", 24.0, domain.VectorBorneSyndromeBabesiosis, domain.VectorBorneAssayQuantitativePCR, domain.VectorBorneAntimicrobialImidocarbAtropine, 55.0, 23.0, 0.3, 2.7, 10.0, 13.5, 6.6, true, false},
	}

	for _, tc := range createErrCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, tc.species, "Breed",
				tc.weightKg, tc.syndrome, tc.assay, tc.protocol,
				tc.platelets, tc.pcv, tc.upc, tc.albumin,
				tc.doxy, tc.atovaquone, tc.imidocarb,
				tc.waterChase, tc.atropine,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageAntimicrobial errors
	if _, err := svc.StageAntimicrobial("VBR-OP-101", "BAD_PROTOCOL", 10, 0, 0, true, false, "", "", "", ""); err == nil {
		t.Errorf("expected error for invalid protocol in StageAntimicrobial")
	}
	if _, err := svc.StageAntimicrobial("VBR-OP-101", domain.VectorBorneAntimicrobialDoxycyclineWaterChase, -1, 0, 0, true, false, "", "", "", ""); err == nil {
		t.Errorf("expected error for negative doxy dose")
	}
	if _, err := svc.StageAntimicrobial("VBR-OP-101", domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 10, -1, 0, true, false, "", "", "", ""); err == nil {
		t.Errorf("expected error for negative atovaquone dose")
	}
	if _, err := svc.StageAntimicrobial("VBR-OP-101", domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 10, 0, -1, true, false, "", "", "", ""); err == nil {
		t.Errorf("expected error for negative imidocarb dose")
	}
	if _, err := svc.StageAntimicrobial("UNKNOWN", domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 10, 0, 0, true, false, "", "", "", ""); err == nil {
		t.Errorf("expected error for unknown ID in StageAntimicrobial")
	}
	// Feline VBR-OP-102 without water chase
	if _, err := svc.StageAntimicrobial("VBR-OP-102", domain.VectorBorneAntimicrobialDoxycyclineWaterChase, 10, 0, 0, false, false, "", "", "", ""); err == nil {
		t.Errorf("expected feline water chase guard error in StageAntimicrobial")
	}
	// Imidocarb without atropine premed
	if _, err := svc.StageAntimicrobial("VBR-OP-101", domain.VectorBorneAntimicrobialImidocarbAtropine, 10, 0, 6.6, true, false, "", "", "", ""); err == nil {
		t.Errorf("expected atropine premedication guard error in StageAntimicrobial")
	}

	// RecordHematologyCheck errors
	if _, err := svc.RecordHematologyCheck("VBR-OP-101", 0, 30, 0.2, 2.8, false, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for invalid platelets")
	}
	if _, err := svc.RecordHematologyCheck("VBR-OP-101", 100, 0, 0.2, 2.8, false, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for invalid pcv")
	}
	if _, err := svc.RecordHematologyCheck("VBR-OP-101", 100, 30, -1, 2.8, false, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for invalid upc")
	}
	if _, err := svc.RecordHematologyCheck("VBR-OP-101", 100, 30, 0.2, 0, false, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for invalid albumin")
	}
	if _, err := svc.RecordHematologyCheck("VBR-OP-101", 100, 30, 0.2, 2.8, false, "", ""); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordHematologyCheck("UNKNOWN", 100, 30, 0.2, 2.8, false, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordHematologyCheck")
	}

	// EvaluateClearance errors
	if _, err := svc.EvaluateClearance("VBR-OP-101", 120.0, 38.0, 0.2, true, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for platelet_count_kul < 150")
	}
	if _, err := svc.EvaluateClearance("VBR-OP-101", 180.0, 30.0, 0.2, true, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for pcv_percent < 35")
	}
	if _, err := svc.EvaluateClearance("VBR-OP-101", 180.0, 38.0, 0.6, true, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for urine_pc_ratio >= 0.5")
	}
	if _, err := svc.EvaluateClearance("VBR-OP-101", 180.0, 38.0, 0.2, false, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for pcr_negative_verified == false")
	}
	if _, err := svc.EvaluateClearance("VBR-OP-101", 180.0, 38.0, 0.2, true, false, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for normothermic_eating_verified == false")
	}
	if _, err := svc.EvaluateClearance("VBR-OP-101", 180.0, 38.0, 0.2, true, true, "", "summary"); err == nil {
		t.Errorf("expected error for empty cleared_by")
	}
	if _, err := svc.EvaluateClearance("VBR-OP-101", 180.0, 38.0, 0.2, true, true, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for empty clearance_summary")
	}
	if _, err := svc.EvaluateClearance("UNKNOWN", 180.0, 38.0, 0.2, true, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in EvaluateClearance")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("VBR-OP-101")
	mutations := []func(op *domain.VectorBorneOperation){
		func(op *domain.VectorBorneOperation) { op.ID = "" },
		func(op *domain.VectorBorneOperation) { op.CaseID = "" },
		func(op *domain.VectorBorneOperation) { op.PetName = "" },
		func(op *domain.VectorBorneOperation) { op.WeightKg = 0 },
		func(op *domain.VectorBorneOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.VectorBorneOperation) { op.Syndrome = "BAD_SYNDROME" },
		func(op *domain.VectorBorneOperation) { op.Assay = "BAD_ASSAY" },
		func(op *domain.VectorBorneOperation) { op.AntimicrobialProtocol = "BAD_PROTOCOL" },
	}
	for _, m := range mutations {
		cp := *validOp
		m(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildPathophysiologySummary coverage for all syndromes + default
	for _, syn := range []domain.VectorBorneSyndrome{
		domain.VectorBorneSyndromeLymeNephritis,
		domain.VectorBorneSyndromeEhrlichiaAnaplasma,
		domain.VectorBorneSyndromeRMSFVasculitis,
		domain.VectorBorneSyndromeBabesiosis,
		domain.VectorBorneSyndromeCytauxzoonosis,
		"CUSTOM_SYNDROME",
	} {
		summary := vectorbornerickettsial.BuildPathophysiologySummary(syn, domain.VectorBorneAssaySNAP4DxPlusC6)
		if !strings.Contains(summary, "SNAP_4DX_PLUS_C6_PEPTIDE") {
			t.Errorf("expected assay in pathophysiology summary for %s, got %q", syn, summary)
		}
	}
}
