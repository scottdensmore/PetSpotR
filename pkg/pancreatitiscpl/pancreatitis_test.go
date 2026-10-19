package pancreatitiscpl_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pancreatitiscpl"
)

func TestPancreatitisCPLSeededOperationsAndMetrics(t *testing.T) {
	svc := pancreatitiscpl.NewPancreatitisService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("PANC-OP-101")
	if err != nil {
		t.Fatalf("expected PANC-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Schnitzel" || op101.Breed != "Miniature Schnauzer" || op101.WeightKg != 8.2 {
		t.Errorf("unexpected PANC-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.PancreatitisStatusAcuteNecrotizingTriage || op101.SeverityClass != domain.PancreatitisSeveritySevereNecrotizingSIRSMODS {
		t.Errorf("unexpected PANC-OP-101 status/severity: %s / %s", op101.Status, op101.SeverityClass)
	}
	if op101.SpecPLUgL != 940 || op101.ModifiedGlasgowPainScore != 14 || op101.EnteralRERPercent != 0 {
		t.Errorf("unexpected PANC-OP-101 telemetry: specPL=%d pain=%d rer=%d", op101.SpecPLUgL, op101.ModifiedGlasgowPainScore, op101.EnteralRERPercent)
	}
	if !op101.POCUSHypoechoicPancreas || !op101.HyperechoicPeripancreaticFat || !op101.PrayerPositionPresent || op101.FelineTriaditisSuspected {
		t.Errorf("unexpected PANC-OP-101 boolean flags: %+v", op101)
	}
	if len(op101.EnteralChecks) != 2 {
		t.Errorf("expected 2 seeded EnteralChecks on PANC-OP-101, got %d", len(op101.EnteralChecks))
	}
	if !strings.Contains(strings.ToLower(op101.DiagnosticPlan.OutdatedNPOMythWarning), "npo") {
		t.Errorf("expected OutdatedNPOMythWarning to warn against prolonged NPO, got %q", op101.DiagnosticPlan.OutdatedNPOMythWarning)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected PANC-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("PANC-OP-102")
	if err != nil {
		t.Fatalf("expected PANC-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Miso" || op102.Species != "Feline" || op102.WeightKg != 4.3 {
		t.Errorf("unexpected PANC-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.PancreatitisStatusFuzapladibEnteralStaged || op102.SeverityClass != domain.PancreatitisSeverityFelineTriaditis {
		t.Errorf("unexpected PANC-OP-102 status/severity: %s / %s", op102.Status, op102.SeverityClass)
	}
	if op102.SpecPLUgL != 28 || op102.ModifiedGlasgowPainScore != 6 || op102.EnteralRERPercent != 50 || !op102.FelineTriaditisSuspected {
		t.Errorf("unexpected PANC-OP-102 telemetry: %+v", op102)
	}
	if len(op102.EnteralChecks) != 1 {
		t.Errorf("expected 1 seeded EnteralCheck on PANC-OP-102, got %d", len(op102.EnteralChecks))
	}

	op103, err := svc.GetOperation("PANC-OP-103")
	if err != nil {
		t.Fatalf("expected PANC-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Biscuit" || op103.Breed != "Yorkshire Terrier" || op103.WeightKg != 3.6 {
		t.Errorf("unexpected PANC-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.PancreatitisStatusToleratingEnteralCleared || op103.SeverityClass != domain.PancreatitisSeverityModerateAcute {
		t.Errorf("unexpected PANC-OP-103 status/severity: %s / %s", op103.Status, op103.SeverityClass)
	}
	if op103.SpecPLUgL != 190 || op103.ModifiedGlasgowPainScore != 2 || op103.EnteralRERPercent != 100 || op103.DischargedBy != "Dr. Marcus Vance, DACVECC" || op103.DischargedAt == nil {
		t.Errorf("unexpected PANC-OP-103 clearance state: %+v", op103)
	}
	if len(op103.EnteralChecks) != 2 {
		t.Errorf("expected 2 seeded EnteralChecks on PANC-OP-103, got %d", len(op103.EnteralChecks))
	}

	triageOps := svc.ListOperations(string(domain.PancreatitisStatusAcuteNecrotizingTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "PANC-OP-101" {
		t.Errorf("expected 1 triage op PANC-OP-101, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcutePancreatitisTriageCount != 1 ||
		metrics.EnteralNutritionStagedCount != 1 ||
		metrics.ClearedEnteralCount != 1 ||
		metrics.SevereSpecPLElevatedCount != 2 ||
		metrics.FelineTriaditisCount != 1 {
		t.Errorf("unexpected overview metrics: %+v", metrics)
	}

	aliasMetrics := svc.ComputeMetrics()
	if aliasMetrics != metrics {
		t.Errorf("expected ComputeMetrics to match GetOverviewMetrics")
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestPancreatitisCPLCreateStageRecordAndClearLifecycle(t *testing.T) {
	svc := pancreatitiscpl.NewPancreatitisService()

	// Create new operation with auto-inferred severity and defaults
	created, err := svc.CreateOperation(
		"",
		"Barnaby",
		"",
		"",
		11.4,
		"",
		680,
		12,
		0,
		true,
		true,
		true,
		false,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "PANC-OP-104" || created.CaseID != "CASE-PANC-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created defaults: %+v", created)
	}
	if created.Status != domain.PancreatitisStatusAcuteNecrotizingTriage || created.SeverityClass != domain.PancreatitisSeveritySevereNecrotizingSIRSMODS {
		t.Errorf("unexpected created status/severity: %s / %s", created.Status, created.SeverityClass)
	}

	// Also test creating feline triaditis & moderate canine cases for branch coverage
	felineCreated, err := svc.CreateOperation("CASE-F-1", "Cleo", "Feline", "Siamese", 3.9, "", 3, 5, 25, true, false, false, true)
	if err != nil {
		t.Fatalf("unexpected feline CreateOperation error: %v", err)
	}
	if felineCreated.SeverityClass != domain.PancreatitisSeverityFelineTriaditis {
		t.Errorf("expected FELINE_TRIADITIS_CHOLANGIOHEPATITIS, got %s", felineCreated.SeverityClass)
	}

	modCreated, err := svc.CreateOperation("CASE-M-1", "Toby", "Canine", "Beagle", 9.5, "", 280, 6, 25, false, false, false, false)
	if err != nil {
		t.Fatalf("unexpected moderate CreateOperation error: %v", err)
	}
	if modCreated.SeverityClass != domain.PancreatitisSeverityModerateAcute {
		t.Errorf("expected MODERATE_ACUTE_PANCREATITIS, got %s", modCreated.SeverityClass)
	}

	// Stage enteral nutrition with empty strings (uses defaults) and with explicit strings
	staged, err := svc.StageEnteralNutrition(created.ID, "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected StageEnteralNutrition error: %v", err)
	}
	if staged.Status != domain.PancreatitisStatusFuzapladibEnteralStaged || staged.EnteralRERPercent != 25 {
		t.Errorf("expected FUZAPLADIB_MAROPITANT_ENTERAL_STAGED and 25%% RER, got %s and %d%%", staged.Status, staged.EnteralRERPercent)
	}

	stagedCustom, err := svc.StageEnteralNutrition(
		created.ID,
		"Fuzapladib 0.4 mg/kg IV q24h x 3d",
		"Maropitant 1 mg/kg IV q24h + Ondansetron 0.5 mg/kg IV q8h",
		"Fentanyl 4 mcg/kg/hr + Ketamine 3 mcg/kg/min CRI",
		"8 Fr Nasoesophageal tube trickle feeding",
		"25% -> 50% -> 75% -> 100% RER over 72h",
		"LRS + 20 mEq/L KCl at 1.5x maintenance",
	)
	if err != nil {
		t.Fatalf("unexpected custom StageEnteralNutrition error: %v", err)
	}
	if stagedCustom.NutritionPlan.FuzapladibProtocol != "Fuzapladib 0.4 mg/kg IV q24h x 3d" {
		t.Errorf("unexpected custom FuzapladibProtocol: %q", stagedCustom.NutritionPlan.FuzapladibProtocol)
	}

	// Record enteral check with empty vomitingStatus and empty clinicalNote (auto-populated)
	checked1, err := svc.RecordEnteralCheck(created.ID, 420, 6, 50, "", "Dr. Rostova", "")
	if err != nil {
		t.Fatalf("unexpected RecordEnteralCheck error: %v", err)
	}
	if checked1.SpecPLUgL != 420 || checked1.ModifiedGlasgowPainScore != 6 || checked1.EnteralRERPercent != 50 || len(checked1.EnteralChecks) != 1 {
		t.Errorf("unexpected state after check 1: %+v", checked1)
	}

	// Record enteral check with painScore <= 4 (clears PrayerPositionPresent)
	checked2, err := svc.RecordEnteralCheck(
		created.ID,
		180,
		2,
		70,
		"ZERO_EMESIS_36H",
		"Dr. Rostova",
		"Spec cPL normalized to 180 ug/L; pain-free on oral transition.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordEnteralCheck 2 error: %v", err)
	}
	if checked2.SpecPLUgL != 180 || checked2.ModifiedGlasgowPainScore != 2 || checked2.EnteralRERPercent != 70 || checked2.PrayerPositionPresent {
		t.Errorf("unexpected state after check 2: %+v", checked2)
	}

	// Clear pancreatitis discharge with empty summary (auto-populated, and EnteralRERPercent < 75 bumped to 100)
	cleared, err := svc.ClearPancreatitisDischarge(created.ID, "Dr. Rostova, DACVECC", "")
	if err != nil {
		t.Fatalf("unexpected ClearPancreatitisDischarge error: %v", err)
	}
	if cleared.Status != domain.PancreatitisStatusToleratingEnteralCleared || cleared.DischargedBy != "Dr. Rostova, DACVECC" || cleared.DischargedAt == nil || cleared.DischargeSummary == "" || cleared.EnteralRERPercent != 100 {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestPancreatitisCPLValidationErrors(t *testing.T) {
	svc := pancreatitiscpl.NewPancreatitisService()

	if _, err := svc.CreateOperation("C-1", "", "Canine", "Poodle", 5.0, domain.PancreatitisSeverityModerateAcute, 300, 5, 0, true, false, false, false); err == nil {
		t.Errorf("expected error for empty pet_name")
	}
	if _, err := svc.CreateOperation("C-1", "Max", "Canine", "Poodle", 0, domain.PancreatitisSeverityModerateAcute, 300, 5, 0, true, false, false, false); err == nil {
		t.Errorf("expected error for weight_kg <= 0")
	}
	if _, err := svc.CreateOperation("C-1", "Max", "Canine", "Poodle", 130, domain.PancreatitisSeverityModerateAcute, 300, 5, 0, true, false, false, false); err == nil {
		t.Errorf("expected error for weight_kg > 120")
	}
	if _, err := svc.CreateOperation("C-1", "Max", "Canine", "Poodle", 10, "INVALID_SEVERITY", 300, 5, 0, true, false, false, false); err == nil {
		t.Errorf("expected error for invalid severity_class")
	}
	if _, err := svc.CreateOperation("C-1", "Max", "Canine", "Poodle", 10, domain.PancreatitisSeverityModerateAcute, -1, 5, 0, true, false, false, false); err == nil {
		t.Errorf("expected error for negative spec_pl_ug_l")
	}
	if _, err := svc.CreateOperation("C-1", "Max", "Canine", "Poodle", 10, domain.PancreatitisSeverityModerateAcute, 300, 25, 0, true, false, false, false); err == nil {
		t.Errorf("expected error for painScore > 24")
	}
	if _, err := svc.CreateOperation("C-1", "Max", "Canine", "Poodle", 10, domain.PancreatitisSeverityModerateAcute, 300, 5, 160, true, false, false, false); err == nil {
		t.Errorf("expected error for enteralRERPercent > 150")
	}

	if _, err := svc.StageEnteralNutrition("UNKNOWN", "", "", "", "", "", ""); err == nil {
		t.Errorf("expected error staging nutrition on unknown op")
	}

	if _, err := svc.RecordEnteralCheck("UNKNOWN", 300, 5, 50, "NONE", "Dr. X", "note"); err == nil {
		t.Errorf("expected error recording check on unknown op")
	}
	if _, err := svc.RecordEnteralCheck("PANC-OP-101", -5, 5, 50, "NONE", "Dr. X", "note"); err == nil {
		t.Errorf("expected error for negative specPLUgL on check")
	}
	if _, err := svc.RecordEnteralCheck("PANC-OP-101", 300, -1, 50, "NONE", "Dr. X", "note"); err == nil {
		t.Errorf("expected error for negative painScore on check")
	}
	if _, err := svc.RecordEnteralCheck("PANC-OP-101", 300, 5, -10, "NONE", "Dr. X", "note"); err == nil {
		t.Errorf("expected error for negative enteralRERPercent on check")
	}
	if _, err := svc.RecordEnteralCheck("PANC-OP-101", 300, 5, 50, "NONE", "   ", "note"); err == nil {
		t.Errorf("expected error for empty recordedBy on check")
	}

	if _, err := svc.ClearPancreatitisDischarge("UNKNOWN", "Dr. X", "summary"); err == nil {
		t.Errorf("expected error clearing unknown op")
	}
	if _, err := svc.ClearPancreatitisDischarge("PANC-OP-101", "   ", "summary"); err == nil {
		t.Errorf("expected error clearing with empty discharged_by")
	}

	// Domain Validate error paths
	validOp, _ := svc.GetOperation("PANC-OP-101")
	badMutations := []func(op *domain.PancreatitisOperation){
		func(op *domain.PancreatitisOperation) { op.ID = "" },
		func(op *domain.PancreatitisOperation) { op.CaseID = "" },
		func(op *domain.PancreatitisOperation) { op.PetName = "" },
		func(op *domain.PancreatitisOperation) { op.Species = "" },
		func(op *domain.PancreatitisOperation) { op.WeightKg = 0 },
		func(op *domain.PancreatitisOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.PancreatitisOperation) { op.SeverityClass = "BAD_SEVERITY" },
		func(op *domain.PancreatitisOperation) { op.SpecPLUgL = -1 },
		func(op *domain.PancreatitisOperation) { op.ModifiedGlasgowPainScore = 30 },
		func(op *domain.PancreatitisOperation) { op.EnteralRERPercent = 200 },
	}
	for _, mutate := range badMutations {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected Validate error for mutated op: %+v", cp)
		}
	}
}
