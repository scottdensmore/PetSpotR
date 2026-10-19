package pyometrasirs_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pyometrasirs"
)

func TestPyometraSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := pyometrasirs.NewPyometraService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded Pyometra operations, got %d", len(ops))
	}

	// Verify PYO-OP-101 ("Bella")
	op101, err := svc.GetOperation("PYO-OP-101")
	if err != nil {
		t.Fatalf("expected PYO-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Bella" || op101.Species != "Canine" || op101.Breed != "Golden Retriever" || op101.WeightKg != 28.5 {
		t.Errorf("unexpected PYO-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.PyometraStatusClosedCervixSIRSTriage || op101.CervixType != domain.CervixPatencyClosedRuptureRisk {
		t.Errorf("unexpected PYO-OP-101 status/cervix: %s / %s", op101.Status, op101.CervixType)
	}
	if op101.UterineHornDiameterCm != 5.4 || op101.BloodLactateMmolL != 4.8 || op101.WBCCountKUl != 34.2 || op101.BandNeutrophilPercent != 18.0 || op101.MAPMmHg != 58 {
		t.Errorf("unexpected PYO-OP-101 metrics: %+v", op101)
	}
	if op101.POCUSFreePeritonealFluid || !op101.PolyuriaPolydipsiaPresent {
		t.Errorf("unexpected PYO-OP-101 flags: fluid=%v pupd=%v", op101.POCUSFreePeritonealFluid, op101.PolyuriaPolydipsiaPresent)
	}
	if len(op101.HemodynamicChecks) != 2 {
		t.Errorf("expected 2 seeded HemodynamicChecks on PYO-OP-101, got %d", len(op101.HemodynamicChecks))
	}
	if !strings.Contains(op101.SIRSPlan.SIRSLactateShockSummary, "DECOMPENSATED ENDOTOXIC SIRS") {
		t.Errorf("expected decompensated SIRS note on PYO-OP-101, got %q", op101.SIRSPlan.SIRSLactateShockSummary)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected PYO-OP-101 to validate, got %v", err)
	}

	// Verify PYO-OP-102 ("Duchess")
	op102, err := svc.GetOperation("PYO-OP-102")
	if err != nil {
		t.Fatalf("expected PYO-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Duchess" || op102.Species != "Feline" || op102.Breed != "Persian" || op102.WeightKg != 3.9 {
		t.Errorf("unexpected PYO-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.PyometraStatusResuscitatedOHEStaged || op102.CervixType != domain.CervixPatencyOpenPurulentDischarge {
		t.Errorf("unexpected PYO-OP-102 status/cervix: %s / %s", op102.Status, op102.CervixType)
	}
	if op102.UterineHornDiameterCm != 2.8 || op102.BloodLactateMmolL != 2.1 || op102.WBCCountKUl != 22.5 || op102.BandNeutrophilPercent != 8.0 || op102.MAPMmHg != 76 {
		t.Errorf("unexpected PYO-OP-102 metrics: %+v", op102)
	}
	if len(op102.HemodynamicChecks) != 1 {
		t.Errorf("expected 1 seeded HemodynamicCheck on PYO-OP-102, got %d", len(op102.HemodynamicChecks))
	}

	// Verify PYO-OP-103 ("Daisy")
	op103, err := svc.GetOperation("PYO-OP-103")
	if err != nil {
		t.Fatalf("expected PYO-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Daisy" || op103.Species != "Canine" || op103.Breed != "English Springer Spaniel" || op103.WeightKg != 19.4 {
		t.Errorf("unexpected PYO-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.PyometraStatusPostopOHECleared || op103.CervixType != domain.CervixPatencyStumpOvarianRemnant {
		t.Errorf("unexpected PYO-OP-103 status/cervix: %s / %s", op103.Status, op103.CervixType)
	}
	if op103.DischargedBy != "Dr. Hannah Lin, DACVS" || op103.DischargedAt == nil || len(op103.HemodynamicChecks) != 2 {
		t.Errorf("expected PYO-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.PyometraStatusClosedCervixSIRSTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "PYO-OP-101" {
		t.Errorf("expected 1 CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.ClosedCervixSIRSTriageCount != 1 ||
		metrics.OHEStagedCount != 1 ||
		metrics.ClearedPostopOHECount != 1 ||
		metrics.ClosedCervixRuptureRiskCount != 1 ||
		metrics.PeritonealEffusionAlertCount != 0 {
		t.Errorf("unexpected PyometraOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown Pyometra operation ID")
	}
}

func TestPyometraCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := pyometrasirs.NewPyometraService()

	// Create operation with default species/breed fallback and free peritoneal fluid alert
	created, err := svc.CreateOperation(
		"CASE-PYO-104",
		"Sadie",
		"",
		"",
		24.0,
		domain.CervixPatencyClosedRuptureRisk,
		6.2,
		5.4,
		38.5,
		21.0,
		56,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "PYO-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if !created.POCUSFreePeritonealFluid || !strings.Contains(created.SIRSPlan.POCUSUterineHornFinding, "FREE ECHOGENIC PERITONEAL FLUID") {
		t.Errorf("expected peritoneal effusion alert in SIRSPlan: %+v", created.SIRSPlan)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.PeritonealEffusionAlertCount != 1 {
		t.Errorf("expected PeritonealEffusionAlertCount=1, got %d", metrics.PeritonealEffusionAlertCount)
	}

	// Stage OHE stabilization protocol with empty optional fields (tests fallback defaults)
	staged, err := svc.StageOHEStabilization(
		created.ID,
		"Normosol-R 20 mL/kg IV shock bolus + Norepinephrine 0.1 mcg/kg/min CRI",
		"Enrofloxacin 10 mg/kg slow IV diluted q24h + Ampicillin-Sulbactam 30 mg/kg IV q8h",
		"Emergent midline laparotomy OHE with moistened laparotomy sponge isolation and double-ligated cervical stump",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageOHEStabilization error: %v", err)
	}
	if staged.Status != domain.PyometraStatusResuscitatedOHEStaged {
		t.Errorf("expected status %s, got %s", domain.PyometraStatusResuscitatedOHEStaged, staged.Status)
	}
	if staged.OHEPlan.AbdominalLavagePlan == "" ||
		staged.OHEPlan.PerioperativeAnalgesiaPlan == "" ||
		staged.OHEPlan.PostopSepsisMonitoring == "" {
		t.Errorf("expected default lavage, analgesia, and sepsis monitoring plans to be populated: %+v", staged.OHEPlan)
	}

	// Stage OHE stabilization with explicit optional fields
	stagedExplicit, err := svc.StageOHEStabilization(
		created.ID,
		"Plasmalyte 20 mL/kg IV bolus",
		"Enrofloxacin + Ampicillin-Sulbactam IV",
		"Atraumatic OHE without traction",
		"Warm 0.9% NaCl 250 mL/kg lavage + closed-suction JP drain",
		"Fentanyl CRI 4 mcg/kg/hr; NSAIDs strictly avoided",
		"Serial lactate and MAP q2h in ICU",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageOHEStabilization error: %v", err)
	}
	if stagedExplicit.OHEPlan.AbdominalLavagePlan != "Warm 0.9% NaCl 250 mL/kg lavage + closed-suction JP drain" {
		t.Errorf("unexpected explicit lavage plan: %q", stagedExplicit.OHEPlan.AbdominalLavagePlan)
	}

	// Record hemodynamic check and verify operation metrics update
	checked, err := svc.RecordHemodynamicCheck(
		created.ID,
		0.0,
		1.4,
		13.8,
		2.0,
		84,
		"Dr. Hannah Lin, DACVS",
		"Post-OHE ICU check: distended uterine horns excised (0.0 cm), lactate cleared to 1.4 mmol/L, band neutrophilia resolving (2.0%), MAP 84 mmHg.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordHemodynamicCheck error: %v", err)
	}
	if len(checked.HemodynamicChecks) != 1 {
		t.Fatalf("expected 1 HemodynamicCheck on PYO-OP-104, got %d", len(checked.HemodynamicChecks))
	}
	if checked.UterineHornDiameterCm != 0.0 || checked.BloodLactateMmolL != 1.4 || checked.WBCCountKUl != 13.8 || checked.BandNeutrophilPercent != 2.0 || checked.MAPMmHg != 84 {
		t.Errorf("expected operation vitals updated from HemodynamicCheck, got %+v", checked)
	}

	// Clear Pyometra discharge
	cleared, err := svc.ClearPyometraDischarge(
		created.ID,
		"Dr. Hannah Lin, DACVS",
		"Post-op Ovariohysterectomy recovery complete: normotensive (MAP 84 mmHg), afebrile, lactate 1.4 mmol/L, resolved PU/PD, discharged on oral Amoxicillin-Clavulanate + Enrofloxacin.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearPyometraDischarge error: %v", err)
	}
	if cleared.Status != domain.PyometraStatusPostopOHECleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Hannah Lin, DACVS" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestPyometraValidationErrors(t *testing.T) {
	svc := pyometrasirs.NewPyometraService()

	createCases := []struct {
		name     string
		caseID   string
		petName  string
		weightKg float64
		cervix   domain.CervixPatencyType
		diamCm   float64
		lactate  float64
		wbc      float64
		bands    float64
		mapMmHg  int
	}{
		{"empty case_id", "", "Bella", 25.0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 30.0, 15.0, 60},
		{"empty pet_name", "CASE-1", "", 25.0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 30.0, 15.0, 60},
		{"weight <= 0", "CASE-1", "Bella", 0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 30.0, 15.0, 60},
		{"weight > 120", "CASE-1", "Bella", 130.0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 30.0, 15.0, 60},
		{"invalid cervixType", "CASE-1", "Bella", 25.0, "INVALID_CERVIX", 5.0, 4.0, 30.0, 15.0, 60},
		{"horn diameter < 0", "CASE-1", "Bella", 25.0, domain.CervixPatencyClosedRuptureRisk, -1.0, 4.0, 30.0, 15.0, 60},
		{"lactate < 0", "CASE-1", "Bella", 25.0, domain.CervixPatencyClosedRuptureRisk, 5.0, -1.0, 30.0, 15.0, 60},
		{"wbc <= 0", "CASE-1", "Bella", 25.0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 0.0, 15.0, 60},
		{"bands out of range", "CASE-1", "Bella", 25.0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 30.0, 110.0, 60},
		{"map out of range", "CASE-1", "Bella", 25.0, domain.CervixPatencyClosedRuptureRisk, 5.0, 4.0, 30.0, 15.0, 10},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Golden Retriever",
				tc.weightKg, tc.cervix,
				tc.diamCm, tc.lactate, tc.wbc, tc.bands, tc.mapMmHg,
				false, true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageOHEStabilization validation errors
	if _, err := svc.StageOHEStabilization("PYO-OP-101", "", "Abx", "OHE", "", "", ""); err == nil {
		t.Errorf("expected error for empty crystalloidPlan")
	}
	if _, err := svc.StageOHEStabilization("PYO-OP-101", "Crystalloid", "", "OHE", "", "", ""); err == nil {
		t.Errorf("expected error for empty empiricAbxPlan")
	}
	if _, err := svc.StageOHEStabilization("PYO-OP-101", "Crystalloid", "Abx", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty oheHandlingProtocol")
	}
	if _, err := svc.StageOHEStabilization("NON-EXISTENT", "Crystalloid", "Abx", "OHE", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageOHEStabilization")
	}

	// RecordHemodynamicCheck validation errors
	if _, err := svc.RecordHemodynamicCheck("PYO-OP-101", -1.0, 2.0, 15.0, 5.0, 75, "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for negative horn diameter")
	}
	if _, err := svc.RecordHemodynamicCheck("PYO-OP-101", 2.0, -1.0, 15.0, 5.0, 75, "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for negative blood lactate")
	}
	if _, err := svc.RecordHemodynamicCheck("PYO-OP-101", 2.0, 2.0, 0.0, 5.0, 75, "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for zero WBC count")
	}
	if _, err := svc.RecordHemodynamicCheck("PYO-OP-101", 2.0, 2.0, 15.0, -1.0, 75, "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for negative band neutrophils")
	}
	if _, err := svc.RecordHemodynamicCheck("PYO-OP-101", 2.0, 2.0, 15.0, 5.0, 10, "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for MAP out of range")
	}
	if _, err := svc.RecordHemodynamicCheck("PYO-OP-101", 2.0, 2.0, 15.0, 5.0, 75, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordHemodynamicCheck("NON-EXISTENT", 2.0, 2.0, 15.0, 5.0, 75, "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordHemodynamicCheck")
	}

	// ClearPyometraDischarge validation errors
	if _, err := svc.ClearPyometraDischarge("PYO-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearPyometraDischarge("PYO-OP-101", "Dr. Lin", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearPyometraDischarge("NON-EXISTENT", "Dr. Lin", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearPyometraDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("PYO-OP-101")
	badOps := []func(op *domain.PyometraOperation){
		func(op *domain.PyometraOperation) { op.ID = "" },
		func(op *domain.PyometraOperation) { op.CaseID = "" },
		func(op *domain.PyometraOperation) { op.PetName = "" },
		func(op *domain.PyometraOperation) { op.WeightKg = 0 },
		func(op *domain.PyometraOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.PyometraOperation) { op.CervixType = "BAD_CERVIX" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildEndotoxemiaSIRSPlan default switch branch coverage
	planDefault := pyometrasirs.BuildEndotoxemiaSIRSPlan("CUSTOM_CERVIX", 3.0, 2.0, 15.0, 4.0, 80, false, false)
	if !strings.Contains(planDefault.CervixPatencySummary, "Acute Pyometra") {
		t.Errorf("expected default cervix summary, got %q", planDefault.CervixPatencySummary)
	}
}
