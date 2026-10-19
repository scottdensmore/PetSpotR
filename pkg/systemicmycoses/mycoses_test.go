package systemicmycoses

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDomainSystemicMycosesValidationAndEnums(t *testing.T) {
	validStatuses := []domain.SystemicMycosesStatus{
		domain.MycosesStatusAzoleAmphotericinTrephinationStaged,
		domain.MycosesStatusAntigenEIACreaOphthalmicMonitored,
		domain.MycosesStatusTwoNegativeAntigenEIACleared,
	}
	for _, st := range validStatuses {
		if !st.IsValid() {
			t.Fatalf("expected status %q to be valid", st)
		}
	}
	if domain.SystemicMycosesStatus("INVALID_STATUS").IsValid() {
		t.Fatal("expected invalid status to return false")
	}

	validSyndromes := []domain.FungalSyndrome{
		domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		domain.FungalSyndromeHistoplasmosisIntracellular,
		domain.FungalSyndromeCoccidioidomycosisSpheruleValley,
		domain.FungalSyndromeCryptococcosisEncapsulatedCNSOcular,
		domain.FungalSyndromeSinonasalAspergillosisCribriform,
	}
	for _, syn := range validSyndromes {
		if !syn.IsValid() {
			t.Fatalf("expected syndrome %q to be valid", syn)
		}
	}
	if domain.FungalSyndrome("INVALID_SYNDROME").IsValid() {
		t.Fatal("expected invalid syndrome to return false")
	}

	validProtocols := []domain.AntifungalProtocol{
		domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		domain.AntifungalProtocolFluconazole10MgKgBIDCNSOcular,
		domain.AntifungalProtocolAmphotericinBLipidComplexSaline,
		domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion,
	}
	for _, pr := range validProtocols {
		if !pr.IsValid() {
			t.Fatalf("expected protocol %q to be valid", pr)
		}
	}
	if domain.AntifungalProtocol("INVALID_PROTOCOL").IsValid() {
		t.Fatal("expected invalid protocol to return false")
	}

	baseOp := domain.SystemicMycosesOperation{
		ID:       "MYC-OP-900",
		CaseID:   "CASE-MYC-900",
		PetName:  "Northwoods",
		WeightKg: 31.0,
		Status:   domain.MycosesStatusAzoleAmphotericinTrephinationStaged,
		Syndrome: domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		Protocol: domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
	}
	if err := baseOp.Validate(); err != nil {
		t.Fatalf("expected valid operation, got %v", err)
	}

	invalidCases := []struct {
		name   string
		mutate func(op *domain.SystemicMycosesOperation)
	}{
		{"empty ID", func(op *domain.SystemicMycosesOperation) { op.ID = " " }},
		{"empty CaseID", func(op *domain.SystemicMycosesOperation) { op.CaseID = "" }},
		{"empty PetName", func(op *domain.SystemicMycosesOperation) { op.PetName = "" }},
		{"non-positive weight", func(op *domain.SystemicMycosesOperation) { op.WeightKg = 0 }},
		{"invalid status", func(op *domain.SystemicMycosesOperation) { op.Status = "BAD" }},
		{"invalid syndrome", func(op *domain.SystemicMycosesOperation) { op.Syndrome = "BAD" }},
		{"invalid protocol", func(op *domain.SystemicMycosesOperation) { op.Protocol = "BAD" }},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			cp := baseOp
			tc.mutate(&cp)
			if err := cp.Validate(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestNewSystemicMycosesServiceSeededOperationsAndOverview(t *testing.T) {
	svc := NewSystemicMycosesService()
	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("MYC-OP-101")
	if err != nil {
		t.Fatalf("expected MYC-OP-101, got %v", err)
	}
	if op101.Status != domain.MycosesStatusAzoleAmphotericinTrephinationStaged {
		t.Fatalf("unexpected MYC-OP-101 status: %s", op101.Status)
	}
	if !op101.CompoundedBulkItraconazoleBlocked || !op101.DieOffDexamethasoneAntiInflammatoryStaged {
		t.Fatal("expected compounded Itraconazole block and pulmonary die-off Dexamethasone flags on MYC-OP-101")
	}

	op102, err := svc.GetOperation("MYC-OP-102")
	if err != nil {
		t.Fatalf("expected MYC-OP-102, got %v", err)
	}
	if op102.Status != domain.MycosesStatusAntigenEIACreaOphthalmicMonitored {
		t.Fatalf("unexpected MYC-OP-102 status: %s", op102.Status)
	}
	if op102.CribriformPlateIntactOnCT {
		t.Fatal("expected CribriformPlateIntactOnCT to be false on MYC-OP-102")
	}

	op103, err := svc.GetOperation("MYC-OP-103")
	if err != nil {
		t.Fatalf("expected MYC-OP-103, got %v", err)
	}
	if op103.Status != domain.MycosesStatusTwoNegativeAntigenEIACleared {
		t.Fatalf("unexpected MYC-OP-103 status: %s", op103.Status)
	}

	if _, err := svc.GetOperation("MYC-OP-999"); err == nil {
		t.Fatal("expected error for unknown operation")
	}

	filtered := svc.ListOperations(string(domain.MycosesStatusAzoleAmphotericinTrephinationStaged))
	if len(filtered) != 1 || filtered[0].ID != "MYC-OP-101" {
		t.Fatalf("expected 1 filtered operation MYC-OP-101, got %+v", filtered)
	}

	overview := svc.GetOverviewMetrics()
	if overview.TotalOperations != 3 ||
		overview.AzoleAmphotericinTrephinationStagedCount != 1 ||
		overview.AntigenEIACreaOphthalmicMonitoredCount != 1 ||
		overview.TwoNegativeAntigenEIAClearedCount != 1 ||
		overview.BlastomycosisHistoplasmosisCount != 1 ||
		overview.CribriformBreachContraindicatedCount != 1 ||
		overview.AzotemiaNephrotoxicityAlertCount != 0 {
		t.Fatalf("unexpected overview metrics: %+v", overview)
	}
}

func TestBuildClinicalNarrativesAndDefaultStagingBranches(t *testing.T) {
	syndromes := []domain.FungalSyndrome{
		domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		domain.FungalSyndromeHistoplasmosisIntracellular,
		domain.FungalSyndromeCoccidioidomycosisSpheruleValley,
		domain.FungalSyndromeCryptococcosisEncapsulatedCNSOcular,
		domain.FungalSyndromeSinonasalAspergillosisCribriform,
		domain.FungalSyndrome("OTHER"),
	}
	antigens := []float64{15.2, 4.5, 0.0}
	creatinines := []float64{2.9, 1.4}
	spo2s := []float64{88.0, 97.0}

	for _, syn := range syndromes {
		for _, ag := range antigens {
			for _, cr := range creatinines {
				for _, sp := range spo2s {
					patho, diag, safety := BuildClinicalNarratives(syn, ag, cr, 85.0, sp, cr < 2.0, sp > 90.0)
					if patho == "" || diag == "" || safety == "" {
						t.Fatalf("expected non-empty narratives for %s ag=%v cr=%v sp=%v", syn, ag, cr, sp)
					}
				}
			}
		}
	}

	stageSinonasal := BuildDefaultTherapyStaging(
		28.0,
		domain.FungalSyndromeSinonasalAspergillosisCribriform,
		domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion,
		1.2,
		97.0,
		true,
		true,
		true,
		"",
		time.Now().UTC(),
	)
	if !strings.Contains(stageSinonasal.SystemicAzoleAndPolyeneRegimenPlan, "1% Clotrimazole") {
		t.Fatalf("expected Sinonasal Clotrimazole trephination plan, got %s", stageSinonasal.SystemicAzoleAndPolyeneRegimenPlan)
	}
}

func TestCreateOperationAndValidationGuards(t *testing.T) {
	svc := NewSystemicMycosesService()

	created, err := svc.CreateOperation(
		"CASE-MYC-104",
		"Boundary",
		"",
		"",
		29.5,
		domain.FungalSyndromeHistoplasmosisIntracellular,
		"",
		11.4,
		1.4,
		92.0,
		90.5,
		true,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("expected CreateOperation to succeed, got %v", err)
	}
	if created.ID != "MYC-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Fatalf("unexpected defaults on created operation: %+v", created)
	}
	if !created.DieOffDexamethasoneAntiInflammatoryStaged {
		t.Fatal("expected DieOffDexamethasoneAntiInflammatoryStaged to be true when SpO2 < 92%")
	}

	// Test validation error branches
	if _, err := svc.CreateOperation("", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for empty case_id")
	}
	if _, err := svc.CreateOperation("CASE-1", "", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for empty pet_name")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 0, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for invalid weight_kg")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, "BAD_SYN", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for invalid syndrome")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, "BAD_PROTO", 10, 1.2, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, -1, 1.2, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for negative miravista_antigen_ng_ml")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 0, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error for invalid serum_creatinine_mg_dl")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 0, 95, true, true, true); err == nil {
		t.Fatal("expected error for invalid alt_activity_u_l")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 60, 40, true, true, true); err == nil {
		t.Fatal("expected error for invalid spo2_percent")
	}

	// Safety Guard 1: Clotrimazole trephination with breached cribriform plate
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeSinonasalAspergillosisCribriform, domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion, 2.0, 1.2, 60, 96, false, true, true); err == nil {
		t.Fatal("expected error for Clotrimazole trephination when cribriform_plate_intact_on_ct is false")
	}
	// Safety Guard 2: Compounded bulk Itraconazole unblocked
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 10, 1.2, 60, 95, true, false, true); err == nil {
		t.Fatal("expected error when compounded_bulk_itraconazole_blocked is false")
	}
	// Safety Guard 3a: Amphotericin B without saline diuresis
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolAmphotericinBLipidComplexSaline, 10, 1.2, 60, 95, true, true, false); err == nil {
		t.Fatal("expected error when amphotericin_lipid_saline_diuresis_verified is false")
	}
	// Safety Guard 3b: Amphotericin B with Creatinine > 2.5 mg/dL
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 25, domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast, domain.AntifungalProtocolAmphotericinBLipidComplexSaline, 10, 2.9, 60, 95, true, true, true); err == nil {
		t.Fatal("expected error when serum_creatinine_mg_dl > 2.5 mg/dL for Amphotericin B")
	}
}

func TestStageTherapyAndSafetyGuards(t *testing.T) {
	svc := NewSystemicMycosesService()

	// Safety Guard: Attempting Clotrimazole trephination on MYC-OP-102 (cribriform plate breach) must fail
	_, err := svc.StageTherapy(
		"MYC-OP-102",
		domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion,
		5.0, 20.0, 2.0,
		true, false, true, false,
		"Attempt topical 1% Clotrimazole sinus infusion despite cribriform breach",
		"", "",
		6,
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "strictly contraindicated") {
		t.Fatalf("expected Clotrimazole cribriform plate contraindication error, got %v", err)
	}

	// Safety Guard: Attempting compounded bulk Itraconazole (compoundedBulkItraconazoleBlocked=false) must fail
	_, err = svc.StageTherapy(
		"MYC-OP-101",
		domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		5.0, 20.0, 2.0,
		false, true, true, true,
		"Attempt compounded bulk powder Itraconazole",
		"", "",
		6,
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "compounded_bulk_itraconazole_blocked") {
		t.Fatalf("expected compounded bulk itraconazole block error, got %v", err)
	}

	// Safety Guard: Attempting Amphotericin B without 0.9% NaCl pre/post-diuresis must fail
	_, err = svc.StageTherapy(
		"MYC-OP-101",
		domain.AntifungalProtocolAmphotericinBLipidComplexSaline,
		5.0, 20.0, 2.0,
		true, true, false, true,
		"Attempt Amphotericin B without saline diuresis",
		"", "",
		6,
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "amphotericin_lipid_saline_diuresis_verified") {
		t.Fatalf("expected saline diuresis guard error, got %v", err)
	}

	// Valid staging on MYC-OP-101 with defaults
	staged, err := svc.StageTherapy(
		"MYC-OP-101",
		"",
		0, 0, 0,
		true, true, true, false,
		"FDA-approved innovator Itraconazole 5 mg/kg/day PO with food + Amphotericin B Lipid Complex 2 mg/kg IV 3x/wk with 0.9% NaCl diuresis",
		"", "",
		0,
		"",
	)
	if err != nil {
		t.Fatalf("expected StageTherapy to succeed, got %v", err)
	}
	if staged.Status != domain.MycosesStatusAntigenEIACreaOphthalmicMonitored {
		t.Fatalf("expected status %s, got %s", domain.MycosesStatusAntigenEIACreaOphthalmicMonitored, staged.Status)
	}
	if !staged.DieOffDexamethasoneAntiInflammatoryStaged {
		t.Fatal("expected DieOffDexamethasoneAntiInflammatoryStaged to be auto-enabled for Blastomycosis with SpO2 < 92%")
	}

	// Validation error branches on StageTherapy
	if _, err := svc.StageTherapy("MYC-OP-101", "BAD_PROTO", 5, 20, 2, true, true, true, true, "Plan", "", "", 6, "Dr"); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.StageTherapy("MYC-OP-101", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 1.0, 20, 2, true, true, true, true, "Plan", "", "", 6, "Dr"); err == nil {
		t.Fatal("expected error for low itraconazole_mg_kg_day")
	}
	if _, err := svc.StageTherapy("MYC-OP-101", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 5.0, 2.0, 2, true, true, true, true, "Plan", "", "", 6, "Dr"); err == nil {
		t.Fatal("expected error for low fluconazole_mg_kg_day")
	}
	if _, err := svc.StageTherapy("MYC-OP-101", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 5.0, 20.0, 0.1, true, true, true, true, "Plan", "", "", 6, "Dr"); err == nil {
		t.Fatal("expected error for low amphotericin_b_lipid_mg_kg_dose")
	}
	if _, err := svc.StageTherapy("MYC-OP-101", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 5.0, 20.0, 2.0, true, true, true, true, "Plan", "", "", 3, "Dr"); err == nil {
		t.Fatal("expected error for minimum_azole_duration_months < 6")
	}
	if _, err := svc.StageTherapy("MYC-OP-101", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 5.0, 20.0, 2.0, true, true, true, true, "", "", "", 6, "Dr"); err == nil {
		t.Fatal("expected error for empty systemic_azole_and_polyene_regimen_plan")
	}
	if _, err := svc.StageTherapy("UNKNOWN-ID", domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution, 5.0, 20.0, 2.0, true, true, true, true, "Plan", "", "", 6, "Dr"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}
}

func TestRecordAntigenRenalCheckAndEvaluateClearance(t *testing.T) {
	svc := NewSystemicMycosesService()

	// Record check with Creatinine > 2.5 mg/dL to verify auto-hold of Amphotericin B and disallow staging Amphotericin B
	azotemic, err := svc.RecordAntigenRenalCheck(
		"MYC-OP-101",
		3.8,
		2.8,
		95.0,
		95.5,
		true,
		false,
		false,
		"Dr. Arlette Thibodeaux, DACVIM",
		"Serial renal check after cumulative Amphotericin B Lipid Complex: Creatinine rose to 2.8 mg/dL — holding Amphotericin B.",
	)
	if err != nil {
		t.Fatalf("expected RecordAntigenRenalCheck to succeed, got %v", err)
	}
	if !azotemic.AmphotericinNephrotoxicityHoldAlert {
		t.Fatal("expected AmphotericinNephrotoxicityHoldAlert to be true when Creatinine > 2.5 mg/dL")
	}
	if ov := svc.CalculateOverview(); ov.AzotemiaNephrotoxicityAlertCount != 1 {
		t.Fatalf("expected AzotemiaNephrotoxicityAlertCount=1, got %d", ov.AzotemiaNephrotoxicityAlertCount)
	}

	// Staging Amphotericin B on azotemic patient (Creatinine 2.8 > 2.5) must fail
	if _, err := svc.StageTherapy(
		"MYC-OP-101",
		domain.AntifungalProtocolAmphotericinBLipidComplexSaline,
		5.0, 20.0, 2.0,
		true, true, true, false,
		"Attempt Amphotericin B while Creatinine is 2.8 mg/dL",
		"", "",
		6, "Dr. Thibodeaux",
	); err == nil {
		t.Fatal("expected StageTherapy to reject Amphotericin B when SerumCreatinineMgDL > 2.5 mg/dL")
	}

	// Record check showing antigen clearance (0.0 ng/mL) and renal recovery (1.2 mg/dL)
	recovered, err := svc.RecordAntigenRenalCheck(
		"MYC-OP-101",
		0.0,
		1.2,
		62.0,
		98.0,
		true,
		false,
		true,
		"Dr. Arlette Thibodeaux, DACVIM",
		"Month 6 & Month 7 MiraVista Blastomyces urine antigen EIA both 0.0 ng/mL, Creatinine normalized to 1.2 mg/dL, chorioretinitis resolved.",
	)
	if err != nil {
		t.Fatalf("expected second RecordAntigenRenalCheck to succeed, got %v", err)
	}
	if recovered.MiraVistaAntigenNgML != 0.0 || recovered.AmphotericinNephrotoxicityHoldAlert {
		t.Fatalf("unexpected fields after recovery check: %+v", recovered)
	}

	// Validation errors for RecordAntigenRenalCheck
	if _, err := svc.RecordAntigenRenalCheck("MYC-OP-101", -1, 1.2, 60, 96, true, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for negative miravista_antigen_ng_ml")
	}
	if _, err := svc.RecordAntigenRenalCheck("MYC-OP-101", 0.0, 0, 60, 96, true, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid serum_creatinine_mg_dl")
	}
	if _, err := svc.RecordAntigenRenalCheck("MYC-OP-101", 0.0, 1.2, 0, 96, true, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid alt_activity_u_l")
	}
	if _, err := svc.RecordAntigenRenalCheck("MYC-OP-101", 0.0, 1.2, 60, 40, true, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid spo2_percent")
	}
	if _, err := svc.RecordAntigenRenalCheck("MYC-OP-101", 0.0, 1.2, 60, 96, true, false, true, "", "note"); err == nil {
		t.Fatal("expected error for empty recorded_by")
	}
	if _, err := svc.RecordAntigenRenalCheck("UNKNOWN-ID", 0.0, 1.2, 60, 96, true, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}

	// EvaluateClearance safety guards
	if _, err := svc.EvaluateClearance("MYC-OP-101", 0.4, 1.2, true, true, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when miravista_antigen_ng_ml != 0.0")
	}
	if _, err := svc.EvaluateClearance("MYC-OP-101", 0.0, 2.1, true, true, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when serum_creatinine_mg_dl > 1.8")
	}
	if _, err := svc.EvaluateClearance("MYC-OP-101", 0.0, 1.2, false, true, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when two_consecutive_negative_eia_verified is false")
	}
	if _, err := svc.EvaluateClearance("MYC-OP-101", 0.0, 1.2, true, false, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when minimum_6month_azole_course_verified is false")
	}
	if _, err := svc.EvaluateClearance("MYC-OP-101", 0.0, 1.2, true, true, "", "Summary"); err == nil {
		t.Fatal("expected clearance error when cleared_by is empty")
	}
	if _, err := svc.EvaluateClearance("MYC-OP-101", 0.0, 1.2, true, true, "Dr. Thibodeaux", ""); err == nil {
		t.Fatal("expected clearance error when clearance_summary is empty")
	}
	if _, err := svc.EvaluateClearance("UNKNOWN-ID", 0.0, 1.2, true, true, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error for unknown operation ID")
	}

	// Successful clearance
	cleared, err := svc.EvaluateClearance(
		"MYC-OP-101",
		0.0,
		1.2,
		true,
		true,
		"Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		"Two consecutive negative MiraVista Blastomyces urine antigen EIA tests (0.0 ng/mL) 1 month apart, Serum Creatinine 1.2 mg/dL (<= 1.8 mg/dL), and 6-month FDA-approved Itraconazole 5 mg/kg/day course verified; cleared.",
	)
	if err != nil {
		t.Fatalf("expected EvaluateClearance to succeed, got %v", err)
	}
	if cleared.Status != domain.MycosesStatusTwoNegativeAntigenEIACleared {
		t.Fatalf("expected status %s, got %s", domain.MycosesStatusTwoNegativeAntigenEIACleared, cleared.Status)
	}
	if cleared.Clearance == nil || cleared.DischargedAt == nil {
		t.Fatal("expected clearance and discharged_at to be populated")
	}
}

func TestConcurrentSystemicMycosesServiceOperations(t *testing.T) {
	svc := NewSystemicMycosesService()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			op, err := svc.CreateOperation(
				fmt.Sprintf("CASE-CONC-%d", idx),
				fmt.Sprintf("Ozark-%d", idx),
				"Canine",
				"Pointer",
				23.0,
				domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
				domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
				9.5,
				1.2,
				70.0,
				94.0,
				true,
				true,
				true,
			)
			if err != nil {
				t.Errorf("concurrent CreateOperation failed: %v", err)
				return
			}
			_, _ = svc.RecordAntigenRenalCheck(op.ID, 0.0, 1.1, 58.0, 98.0, true, false, true, "Dr. Concurrent", "Negative antigen EIA check")
			_, _ = svc.EvaluateClearance(op.ID, 0.0, 1.1, true, true, "Dr. Concurrent", "Cleared")
			_ = svc.ListOperations("")
			_ = svc.CalculateOverview()
		}(i)
	}
	wg.Wait()
}
