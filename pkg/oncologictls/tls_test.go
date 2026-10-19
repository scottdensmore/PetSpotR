package oncologictls_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/oncologictls"
)

func TestTLSSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := oncologictls.NewTLSService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded TLS operations, got %d", len(ops))
	}

	// Verify TLS-OP-101 ("Bauer")
	op101, err := svc.GetOperation("TLS-OP-101")
	if err != nil {
		t.Fatalf("expected TLS-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Bauer" || op101.Species != "Canine" || op101.Breed != "Boxer" || op101.WeightKg != 32.0 {
		t.Errorf("unexpected TLS-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.TLSStatusAcuteMetabolicTriage || op101.EmergencyType != domain.OncologicEmergencyTumorLysisLymphoma {
		t.Errorf("unexpected TLS-OP-101 status/emergency_type: %s / %s", op101.Status, op101.EmergencyType)
	}
	if op101.SerumPotassiumMmolL != 7.2 || op101.SerumPhosphorusMgDL != 11.8 || op101.IonizedCalciumMmolL != 0.78 {
		t.Errorf("unexpected TLS-OP-101 K+/P/iCa: k=%.1f p=%.1f ica=%.2f", op101.SerumPotassiumMmolL, op101.SerumPhosphorusMgDL, op101.IonizedCalciumMmolL)
	}
	if op101.UricAcidMgDL != 9.4 || op101.SerumCreatinineMgDL != 3.1 || op101.CaXPProduct != 98.4 {
		t.Errorf("unexpected TLS-OP-101 UricAcid/Creatinine/CaXP: ua=%.1f cr=%.1f caxp=%.1f", op101.UricAcidMgDL, op101.SerumCreatinineMgDL, op101.CaXPProduct)
	}
	if !op101.DefinitiveCytologyConfirmed {
		t.Errorf("expected DefinitiveCytologyConfirmed=true on TLS-OP-101")
	}
	if len(op101.ElectrolyteChecks) != 2 {
		t.Errorf("expected 2 seeded ElectrolyteChecks on TLS-OP-101, got %d", len(op101.ElectrolyteChecks))
	}
	if !strings.Contains(op101.MetabolicPlan.SodiumBicarbonateUrineNote, "Sodium Bicarbonate") {
		t.Errorf("expected sodium bicarbonate warning on TLS-OP-101, got %q", op101.MetabolicPlan.SodiumBicarbonateUrineNote)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected TLS-OP-101 to validate, got %v", err)
	}

	// Verify TLS-OP-102 ("Penny")
	op102, err := svc.GetOperation("TLS-OP-102")
	if err != nil {
		t.Fatalf("expected TLS-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Penny" || op102.Breed != "English Cocker Spaniel" || op102.WeightKg != 13.5 {
		t.Errorf("unexpected TLS-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.TLSStatusDiuresisStaged || op102.EmergencyType != domain.OncologicEmergencyHypercalcemiaPTHrP {
		t.Errorf("unexpected TLS-OP-102 status/emergency_type: %s / %s", op102.Status, op102.EmergencyType)
	}
	if op102.SerumPotassiumMmolL != 4.1 || op102.SerumPhosphorusMgDL != 4.2 || op102.IonizedCalciumMmolL != 1.92 || op102.UricAcidMgDL != 1.4 || op102.SerumCreatinineMgDL != 2.2 || op102.CaXPProduct != 66.8 {
		t.Errorf("unexpected TLS-OP-102 metrics: %+v", op102)
	}
	if op102.DefinitiveCytologyConfirmed {
		t.Errorf("expected DefinitiveCytologyConfirmed=false on TLS-OP-102")
	}
	if len(op102.ElectrolyteChecks) != 1 {
		t.Errorf("expected 1 seeded ElectrolyteCheck on TLS-OP-102, got %d", len(op102.ElectrolyteChecks))
	}

	// Verify TLS-OP-103 ("Otis")
	op103, err := svc.GetOperation("TLS-OP-103")
	if err != nil {
		t.Fatalf("expected TLS-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Otis" || op103.Species != "Feline" || op103.Breed != "Domestic Shorthair" || op103.WeightKg != 4.8 {
		t.Errorf("unexpected TLS-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.TLSStatusNormoelectrolyticCleared || op103.EmergencyType != domain.OncologicEmergencyFebrileNeutropenia {
		t.Errorf("unexpected TLS-OP-103 status/emergency_type: %s / %s", op103.Status, op103.EmergencyType)
	}
	if op103.SerumPotassiumMmolL != 4.3 || op103.SerumPhosphorusMgDL != 4.5 || op103.IonizedCalciumMmolL != 1.28 || op103.UricAcidMgDL != 1.1 || op103.SerumCreatinineMgDL != 1.2 || op103.CaXPProduct != 44.1 {
		t.Errorf("unexpected TLS-OP-103 metrics: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Soren Lindqvist, DACVIM (Oncology)" || op103.DischargedAt == nil || len(op103.ElectrolyteChecks) != 2 {
		t.Errorf("expected TLS-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.TLSStatusAcuteMetabolicTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "TLS-OP-101" {
		t.Errorf("expected 1 ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteMetabolicTriageCount != 1 ||
		metrics.DiuresisStagedCount != 1 ||
		metrics.ClearedNormoelectrolyticCount != 1 ||
		metrics.TumorLysisSyndromeCount != 1 ||
		metrics.HypercalcemiaMalignancyCount != 1 {
		t.Errorf("unexpected OncologicTLSOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown TLS operation ID")
	}
}

func TestTLSCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := oncologictls.NewTLSService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-TLS-104",
		"Apollo",
		"",
		"",
		28.0,
		domain.OncologicEmergencyTumorLysisLymphoma,
		6.9,
		10.8,
		0.82,
		8.8,
		2.8,
		92.5,
		false,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "TLS-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.SerumPotassiumMmolL != 6.9 || created.IonizedCalciumMmolL != 0.82 {
		t.Errorf("expected K+=6.9 and iCa=0.82, got %.1f / %.2f", created.SerumPotassiumMmolL, created.IonizedCalciumMmolL)
	}

	// Stage diuresis therapy with empty optional fields (tests fallback defaults)
	staged, err := svc.StageDiuresisTherapy(
		created.ID,
		"0.9% NaCl Calcium-Free Saline Diuresis at 140 mL/hr IV",
		"Rasburicase 0.2 mg/kg IV over 30 min + Allopurinol 10 mg/kg PO q8h",
		"Zoledronate 0.25 mg/kg IV over 15 min + withhold Prednisone pending cytology",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageDiuresisTherapy error: %v", err)
	}
	if staged.Status != domain.TLSStatusDiuresisStaged {
		t.Errorf("expected status %s, got %s", domain.TLSStatusDiuresisStaged, staged.Status)
	}
	if staged.DiuresisPlan.HyperkalemiaCardioprotectionPlan == "" ||
		staged.DiuresisPlan.PhosphateBinderOrLoopDiuretic == "" ||
		staged.DiuresisPlan.SerialElectrolyteECGSchedule == "" {
		t.Errorf("expected default cardioprotection, binder/diuretic, and schedule notes to be populated: %+v", staged.DiuresisPlan)
	}

	// Stage diuresis therapy with explicit optional fields
	stagedExplicit, err := svc.StageDiuresisTherapy(
		created.ID,
		"0.9% NaCl 150 mL/hr IV",
		"Rasburicase 0.2 mg/kg IV once",
		"Zoledronate 0.25 mg/kg IV over 15m",
		"Regular Insulin 0.5 U/kg IV + Dextrose 2 g/U",
		"Furosemide 2 mg/kg IV post-euvolemia + Aluminum Hydroxide 60 mg/kg/day PO",
		"Recheck K+, iCa, Phosphorus, Creatinine & Lead II ECG q4h",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageDiuresisTherapy error: %v", err)
	}
	if stagedExplicit.DiuresisPlan.HyperkalemiaCardioprotectionPlan != "Regular Insulin 0.5 U/kg IV + Dextrose 2 g/U" {
		t.Errorf("unexpected explicit cardioprotection plan: %q", stagedExplicit.DiuresisPlan.HyperkalemiaCardioprotectionPlan)
	}

	// Record electrolyte check and verify operation metrics update
	checked, err := svc.RecordElectrolyteCheck(
		created.ID,
		4.4,
		4.6,
		1.24,
		1.6,
		1.3,
		"Dr. Soren Lindqvist, DACVIM (Oncology)",
		"Post-0.9% NaCl diuresis & Rasburicase: K+ normalized to 4.4 mmol/L, Phosphorus 4.6 mg/dL, iCa restored to 1.24 mmol/L, Uric Acid 1.6 mg/dL.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordElectrolyteCheck error: %v", err)
	}
	if len(checked.ElectrolyteChecks) != 1 {
		t.Fatalf("expected 1 ElectrolyteCheck on TLS-OP-104, got %d", len(checked.ElectrolyteChecks))
	}
	if checked.SerumPotassiumMmolL != 4.4 || checked.SerumPhosphorusMgDL != 4.6 || checked.IonizedCalciumMmolL != 1.24 || checked.UricAcidMgDL != 1.6 || checked.SerumCreatinineMgDL != 1.3 {
		t.Errorf("expected operation vitals updated from ElectrolyteCheck, got %+v", checked)
	}

	// Clear TLS discharge
	cleared, err := svc.ClearTLSDischarge(
		created.ID,
		"Dr. Soren Lindqvist, DACVIM (Oncology)",
		"Electrolytes normalized (K+ 4.4 mmol/L, iCa 1.24 mmol/L, Phosphorus 4.6 mg/dL), renal tubular function preserved (Creatinine 1.3 mg/dL), cleared for oncology ward.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearTLSDischarge error: %v", err)
	}
	if cleared.Status != domain.TLSStatusNormoelectrolyticCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Soren Lindqvist, DACVIM (Oncology)" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestTLSValidationErrors(t *testing.T) {
	svc := oncologictls.NewTLSService()

	createCases := []struct {
		name          string
		caseID        string
		petName       string
		weightKg      float64
		emergencyType domain.OncologicEmergencyType
		potassium     float64
		phosphorus    float64
		iCal          float64
		uricAcid      float64
		creatinine    float64
		caxp          float64
	}{
		{"empty case_id", "", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, 9.0, 3.0, 95.0},
		{"empty pet_name", "CASE-1", "", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, 9.0, 3.0, 95.0},
		{"weight <= 0", "CASE-1", "Bauer", 0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, 9.0, 3.0, 95.0},
		{"weight > 120", "CASE-1", "Bauer", 130, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, 9.0, 3.0, 95.0},
		{"invalid emergency_type", "CASE-1", "Bauer", 30.0, "INVALID_EMERGENCY", 7.0, 11.0, 0.80, 9.0, 3.0, 95.0},
		{"potassium <= 0", "CASE-1", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 0, 11.0, 0.80, 9.0, 3.0, 95.0},
		{"phosphorus <= 0", "CASE-1", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 0, 0.80, 9.0, 3.0, 95.0},
		{"iCal <= 0", "CASE-1", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0, 9.0, 3.0, 95.0},
		{"uricAcid < 0", "CASE-1", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, -1.0, 3.0, 95.0},
		{"creatinine <= 0", "CASE-1", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, 9.0, 0, 95.0},
		{"caxp <= 0", "CASE-1", "Bauer", 30.0, domain.OncologicEmergencyTumorLysisLymphoma, 7.0, 11.0, 0.80, 9.0, 3.0, 0},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Boxer",
				tc.weightKg, tc.emergencyType,
				tc.potassium, tc.phosphorus, tc.iCal, tc.uricAcid, tc.creatinine, tc.caxp,
				true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageDiuresisTherapy validation errors
	if _, err := svc.StageDiuresisTherapy("TLS-OP-101", "", "Rasburicase", "Zoledronate", "", "", ""); err == nil {
		t.Errorf("expected error for empty salineDiuresisPlan")
	}
	if _, err := svc.StageDiuresisTherapy("TLS-OP-101", "Saline", "", "Zoledronate", "", "", ""); err == nil {
		t.Errorf("expected error for empty rasburicaseProtocol")
	}
	if _, err := svc.StageDiuresisTherapy("TLS-OP-101", "Saline", "Rasburicase", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty zoledronateProtocol")
	}
	if _, err := svc.StageDiuresisTherapy("NON-EXISTENT", "Saline", "Rasburicase", "Zoledronate", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageDiuresisTherapy")
	}

	// RecordElectrolyteCheck validation errors
	if _, err := svc.RecordElectrolyteCheck("TLS-OP-101", 0, 4.5, 1.2, 1.5, 1.2, "Dr. Lindqvist", "note"); err == nil {
		t.Errorf("expected error for non-positive potassium")
	}
	if _, err := svc.RecordElectrolyteCheck("TLS-OP-101", 4.5, 0, 1.2, 1.5, 1.2, "Dr. Lindqvist", "note"); err == nil {
		t.Errorf("expected error for non-positive phosphorus")
	}
	if _, err := svc.RecordElectrolyteCheck("TLS-OP-101", 4.5, 4.5, 0, 1.5, 1.2, "Dr. Lindqvist", "note"); err == nil {
		t.Errorf("expected error for non-positive ionized calcium")
	}
	if _, err := svc.RecordElectrolyteCheck("TLS-OP-101", 4.5, 4.5, 1.2, -1.0, 1.2, "Dr. Lindqvist", "note"); err == nil {
		t.Errorf("expected error for negative uric acid")
	}
	if _, err := svc.RecordElectrolyteCheck("TLS-OP-101", 4.5, 4.5, 1.2, 1.5, 0, "Dr. Lindqvist", "note"); err == nil {
		t.Errorf("expected error for non-positive creatinine")
	}
	if _, err := svc.RecordElectrolyteCheck("TLS-OP-101", 4.5, 4.5, 1.2, 1.5, 1.2, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordElectrolyteCheck("NON-EXISTENT", 4.5, 4.5, 1.2, 1.5, 1.2, "Dr. Lindqvist", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordElectrolyteCheck")
	}

	// ClearTLSDischarge validation errors
	if _, err := svc.ClearTLSDischarge("TLS-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearTLSDischarge("TLS-OP-101", "Dr. Lindqvist", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearTLSDischarge("NON-EXISTENT", "Dr. Lindqvist", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearTLSDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("TLS-OP-101")
	badOps := []func(op *domain.OncologicTLSOperation){
		func(op *domain.OncologicTLSOperation) { op.ID = "" },
		func(op *domain.OncologicTLSOperation) { op.CaseID = "" },
		func(op *domain.OncologicTLSOperation) { op.PetName = "" },
		func(op *domain.OncologicTLSOperation) { op.WeightKg = 0 },
		func(op *domain.OncologicTLSOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.OncologicTLSOperation) { op.EmergencyType = "BAD_EMERGENCY" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildMetabolicNephroprotectionPlan default switch branch coverage
	planDefault := oncologictls.BuildMetabolicNephroprotectionPlan("CUSTOM_EMERGENCY", 4.5, 4.5, 1.25, 1.5, 1.2, 45.0, false)
	if !strings.Contains(planDefault.EmergencyPathophysiology, "Oncologic") {
		t.Errorf("expected default pathophysiology summary, got %q", planDefault.EmergencyPathophysiology)
	}
}
