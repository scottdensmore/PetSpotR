package diccoagulopathy_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/diccoagulopathy"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDICSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := diccoagulopathy.NewDICService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded DIC operations, got %d", len(ops))
	}

	// Verify DIC-OP-101 ("Flash")
	op101, err := svc.GetOperation("DIC-OP-101")
	if err != nil {
		t.Fatalf("expected DIC-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Flash" || op101.Species != "Canine" || op101.Breed != "Retired Racing Greyhound" || op101.WeightKg != 31.0 {
		t.Errorf("unexpected DIC-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.DICStatusAcuteOvertDICTriage || op101.PhasePhenotype != domain.DICPhenotypeHypoCoagulableConsumptive {
		t.Errorf("unexpected DIC-OP-101 status/phenotype: %s / %s", op101.Status, op101.PhasePhenotype)
	}
	if op101.PlateletCountKUl != 38.0 || op101.PTSeconds != 24.5 || op101.APTTSeconds != 142.0 {
		t.Errorf("unexpected DIC-OP-101 platelets/PT/aPTT: plt=%.1f pt=%.1f aptt=%.1f", op101.PlateletCountKUl, op101.PTSeconds, op101.APTTSeconds)
	}
	if op101.DDimerNgML != 1850.0 || op101.FibrinogenMgDL != 68.0 || op101.AntithrombinPercent != 48.0 {
		t.Errorf("unexpected DIC-OP-101 D-Dimer/Fibrinogen/ATIII: ddimer=%.1f fib=%.1f at3=%.1f", op101.DDimerNgML, op101.FibrinogenMgDL, op101.AntithrombinPercent)
	}
	if !op101.SchistocytesPresent || !op101.SpontaneousMucosalBleeding {
		t.Errorf("expected SchistocytesPresent=true and SpontaneousMucosalBleeding=true on DIC-OP-101")
	}
	if len(op101.CoagChecks) != 2 {
		t.Errorf("expected 2 seeded CoagChecks on DIC-OP-101, got %d", len(op101.CoagChecks))
	}
	if !strings.Contains(op101.ProfilePlan.JugularVenipunctureContraindication, "jugular venipuncture") {
		t.Errorf("expected jugular venipuncture contraindication note on DIC-OP-101, got %q", op101.ProfilePlan.JugularVenipunctureContraindication)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected DIC-OP-101 to validate, got %v", err)
	}

	// Verify DIC-OP-102 ("Rocket")
	op102, err := svc.GetOperation("DIC-OP-102")
	if err != nil {
		t.Fatalf("expected DIC-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Rocket" || op102.Breed != "Doberman Pinscher" || op102.WeightKg != 34.5 {
		t.Errorf("unexpected DIC-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.DICStatusFFPCryoprecipitateTXAStaged || op102.PhasePhenotype != domain.DICPhenotypeHyperfibrinolysisGreyhound {
		t.Errorf("unexpected DIC-OP-102 status/phenotype: %s / %s", op102.Status, op102.PhasePhenotype)
	}
	if op102.PlateletCountKUl != 92.0 || op102.PTSeconds != 14.2 || op102.APTTSeconds != 88.0 || op102.DDimerNgML != 940.0 || op102.FibrinogenMgDL != 115.0 || op102.AntithrombinPercent != 72.0 {
		t.Errorf("unexpected DIC-OP-102 metrics: %+v", op102)
	}
	if op102.SchistocytesPresent || !op102.SpontaneousMucosalBleeding {
		t.Errorf("unexpected DIC-OP-102 flags: %+v", op102)
	}
	if len(op102.CoagChecks) != 1 {
		t.Errorf("expected 1 seeded CoagCheck on DIC-OP-102, got %d", len(op102.CoagChecks))
	}

	// Verify DIC-OP-103 ("Willow")
	op103, err := svc.GetOperation("DIC-OP-103")
	if err != nil {
		t.Fatalf("expected DIC-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Willow" || op103.Species != "Feline" || op103.Breed != "Domestic Medium Hair" || op103.WeightKg != 4.4 {
		t.Errorf("unexpected DIC-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.DICStatusHemostasisNormocoagulableClear || op103.PhasePhenotype != domain.DICPhenotypeHyperCoagulableEarly {
		t.Errorf("unexpected DIC-OP-103 status/phenotype: %s / %s", op103.Status, op103.PhasePhenotype)
	}
	if op103.PlateletCountKUl != 215.0 || op103.PTSeconds != 10.8 || op103.APTTSeconds != 64.0 || op103.DDimerNgML != 180.0 || op103.FibrinogenMgDL != 210.0 || op103.AntithrombinPercent != 94.0 {
		t.Errorf("unexpected DIC-OP-103 metrics: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Callum Vance, DACVECC" || op103.DischargedAt == nil || len(op103.CoagChecks) != 2 {
		t.Errorf("expected DIC-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.DICStatusAcuteOvertDICTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "DIC-OP-101" {
		t.Errorf("expected 1 ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.OvertDICTriageCount != 1 ||
		metrics.FFPTXAStagedCount != 1 ||
		metrics.ClearedNormocoagulableCount != 1 ||
		metrics.ConsumptiveHemorrhagicCount != 1 ||
		metrics.HyperfibrinolysisGreyhoundCount != 1 {
		t.Errorf("unexpected DICOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown DIC operation ID")
	}
}

func TestDICCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := diccoagulopathy.NewDICService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-DIC-104",
		"Zephyr",
		"",
		"",
		28.5,
		domain.DICPhenotypeHypoCoagulableConsumptive,
		42.0,
		22.4,
		136.0,
		1740.0,
		72.0,
		52.0,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "DIC-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.DDimerNgML != 1740.0 || created.FibrinogenMgDL != 72.0 {
		t.Errorf("expected D-Dimer=1740.0 and Fibrinogen=72.0, got %.1f / %.1f", created.DDimerNgML, created.FibrinogenMgDL)
	}

	// Stage component therapy with empty optional fields (tests fallback defaults)
	staged, err := svc.StageComponentTherapy(
		created.ID,
		"Fresh Frozen Plasma (FFP) 15 mL/kg IV over 4 hours",
		"Cryoprecipitate 3 units IV (1 unit/10 kg for Fibrinogen < 100 mg/dL)",
		"Tranexamic Acid (TXA) 10 mg/kg IV slow over 20 min q8h",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageComponentTherapy error: %v", err)
	}
	if staged.Status != domain.DICStatusFFPCryoprecipitateTXAStaged {
		t.Errorf("expected status %s, got %s", domain.DICStatusFFPCryoprecipitateTXAStaged, staged.Status)
	}
	if staged.ComponentPlan.AntithrombinHeparinPhasePlan == "" ||
		staged.ComponentPlan.UnderlyingTriggerControlPlan == "" ||
		staged.ComponentPlan.SerialCoagPanelSchedule == "" {
		t.Errorf("expected default heparin, trigger control, and coag schedule notes to be populated: %+v", staged.ComponentPlan)
	}

	// Stage component therapy with explicit optional fields
	stagedExplicit, err := svc.StageComponentTherapy(
		created.ID,
		"FFP 18 mL/kg IV",
		"Cryoprecipitate 3 units IV",
		"TXA 10 mg/kg IV q8h",
		"Supplement ATIII via FFP prior to low-dose Dalteparin",
		"Surgical splenectomy for ruptured hemangiosarcoma",
		"Repeat PT/aPTT/Fibrinogen/D-Dimer q4h",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageComponentTherapy error: %v", err)
	}
	if stagedExplicit.ComponentPlan.UnderlyingTriggerControlPlan != "Surgical splenectomy for ruptured hemangiosarcoma" {
		t.Errorf("unexpected explicit trigger control plan: %q", stagedExplicit.ComponentPlan.UnderlyingTriggerControlPlan)
	}

	// Record coagulation panel check and verify operation metrics update
	checked, err := svc.RecordCoagCheck(
		created.ID,
		148.0,
		11.4,
		72.0,
		260.0,
		195.0,
		"Dr. Callum Vance, DACVECC",
		"Post-FFP, Cryoprecipitate & TXA infusion: mucosal bleeding arrested, fibrinogen restored to 195 mg/dL, D-Dimer down to 260 ng/mL.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordCoagCheck error: %v", err)
	}
	if len(checked.CoagChecks) != 1 {
		t.Fatalf("expected 1 CoagCheck on DIC-OP-104, got %d", len(checked.CoagChecks))
	}
	if checked.PlateletCountKUl != 148.0 || checked.PTSeconds != 11.4 || checked.APTTSeconds != 72.0 || checked.DDimerNgML != 260.0 || checked.FibrinogenMgDL != 195.0 {
		t.Errorf("expected operation vitals updated from CoagCheck, got %+v", checked)
	}

	// Clear DIC discharge
	cleared, err := svc.ClearDICDischarge(
		created.ID,
		"Dr. Callum Vance, DACVECC",
		"Hemostasis restored, PT/aPTT normalized (11.4s / 72.0s), Fibrinogen 195.0 mg/dL, D-Dimer 260.0 ng/mL, cleared for step-down recovery.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearDICDischarge error: %v", err)
	}
	if cleared.Status != domain.DICStatusHemostasisNormocoagulableClear || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Callum Vance, DACVECC" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestDICValidationErrors(t *testing.T) {
	svc := diccoagulopathy.NewDICService()

	createCases := []struct {
		name       string
		caseID     string
		petName    string
		weightKg   float64
		phenotype  domain.DICPhasePhenotype
		platelets  float64
		pt         float64
		aptt       float64
		dDimer     float64
		fibrinogen float64
		atIII      float64
	}{
		{"empty case_id", "", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, 1500, 70, 50},
		{"empty pet_name", "CASE-1", "", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, 1500, 70, 50},
		{"weight <= 0", "CASE-1", "Flash", 0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, 1500, 70, 50},
		{"weight > 120", "CASE-1", "Flash", 130, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, 1500, 70, 50},
		{"invalid phenotype", "CASE-1", "Flash", 30.0, "INVALID_PHENOTYPE", 40, 22, 130, 1500, 70, 50},
		{"platelets <= 0", "CASE-1", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 0, 22, 130, 1500, 70, 50},
		{"pt <= 0", "CASE-1", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 0, 130, 1500, 70, 50},
		{"aptt <= 0", "CASE-1", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 0, 1500, 70, 50},
		{"ddimer < 0", "CASE-1", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, -10, 70, 50},
		{"fibrinogen <= 0", "CASE-1", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, 1500, 0, 50},
		{"atIII <= 0", "CASE-1", "Flash", 30.0, domain.DICPhenotypeHypoCoagulableConsumptive, 40, 22, 130, 1500, 70, 0},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Greyhound",
				tc.weightKg, tc.phenotype,
				tc.platelets, tc.pt, tc.aptt, tc.dDimer, tc.fibrinogen, tc.atIII,
				true, true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageComponentTherapy validation errors
	if _, err := svc.StageComponentTherapy("DIC-OP-101", "", "Cryo", "TXA", "", "", ""); err == nil {
		t.Errorf("expected error for empty ffpProtocol")
	}
	if _, err := svc.StageComponentTherapy("DIC-OP-101", "FFP", "", "TXA", "", "", ""); err == nil {
		t.Errorf("expected error for empty cryoprecipitateProtocol")
	}
	if _, err := svc.StageComponentTherapy("DIC-OP-101", "FFP", "Cryo", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty txaPlan")
	}
	if _, err := svc.StageComponentTherapy("NON-EXISTENT", "FFP", "Cryo", "TXA", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageComponentTherapy")
	}

	// RecordCoagCheck validation errors
	if _, err := svc.RecordCoagCheck("DIC-OP-101", 0, 12.0, 75.0, 300.0, 180.0, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for non-positive platelet count")
	}
	if _, err := svc.RecordCoagCheck("DIC-OP-101", 100, 0, 75.0, 300.0, 180.0, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for non-positive PT")
	}
	if _, err := svc.RecordCoagCheck("DIC-OP-101", 100, 12.0, 0, 300.0, 180.0, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for non-positive aPTT")
	}
	if _, err := svc.RecordCoagCheck("DIC-OP-101", 100, 12.0, 75.0, -5.0, 180.0, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for negative D-Dimer")
	}
	if _, err := svc.RecordCoagCheck("DIC-OP-101", 100, 12.0, 75.0, 300.0, 0, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for non-positive fibrinogen")
	}
	if _, err := svc.RecordCoagCheck("DIC-OP-101", 100, 12.0, 75.0, 300.0, 180.0, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordCoagCheck("NON-EXISTENT", 100, 12.0, 75.0, 300.0, 180.0, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordCoagCheck")
	}

	// ClearDICDischarge validation errors
	if _, err := svc.ClearDICDischarge("DIC-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearDICDischarge("DIC-OP-101", "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearDICDischarge("NON-EXISTENT", "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearDICDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("DIC-OP-101")
	badOps := []func(op *domain.DICCoagulopathyOperation){
		func(op *domain.DICCoagulopathyOperation) { op.ID = "" },
		func(op *domain.DICCoagulopathyOperation) { op.CaseID = "" },
		func(op *domain.DICCoagulopathyOperation) { op.PetName = "" },
		func(op *domain.DICCoagulopathyOperation) { op.WeightKg = 0 },
		func(op *domain.DICCoagulopathyOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.DICCoagulopathyOperation) { op.PhasePhenotype = "BAD_PHENOTYPE" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildCoagulationTEGProfilePlan default switch branch coverage
	planDefault := diccoagulopathy.BuildCoagulationTEGProfilePlan("CUSTOM_PHENOTYPE", 150, 12, 70, 200, 180, 90, false, false)
	if !strings.Contains(planDefault.PhenotypePathophysiology, "Hemostatic") {
		t.Errorf("expected default pathophysiology summary, got %q", planDefault.PhenotypePathophysiology)
	}
}
