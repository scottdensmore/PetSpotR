package heartwormcaval

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDomainHeartwormValidationAndEnums(t *testing.T) {
	validStatuses := []domain.HeartwormCavalStatus{
		domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged,
		domain.HeartwormStatusPulmonaryHypertensionPTEMonitored,
		domain.HeartwormStatusCavalPatentAntigenNegativeCleared,
	}
	for _, st := range validStatuses {
		if !st.IsValid() {
			t.Fatalf("expected status %q to be valid", st)
		}
	}
	if domain.HeartwormCavalStatus("INVALID_STATUS").IsValid() {
		t.Fatal("expected invalid status to return false")
	}

	validClasses := []domain.HeartwormClass{
		domain.HeartwormClassIVCavalSyndrome,
		domain.HeartwormClassIIISeverePTERCHF,
		domain.HeartwormClassIIModerateAntigenPos,
		domain.HeartwormClassFelineHARDDirofilariasis,
	}
	for _, cl := range validClasses {
		if !cl.IsValid() {
			t.Fatalf("expected class %q to be valid", cl)
		}
	}
	if domain.HeartwormClass("INVALID_CLASS").IsValid() {
		t.Fatal("expected invalid class to return false")
	}

	validProtocols := []domain.HeartwormProtocol{
		domain.HeartwormProtocolRightJugularIshiharaExtraction,
		domain.HeartwormProtocolSplitDoseMelarsomineDay609091,
		domain.HeartwormProtocolDoxycyclineWolbachia28Day,
		domain.HeartwormProtocolSildenafilPrednisoneCageRest,
	}
	for _, pr := range validProtocols {
		if !pr.IsValid() {
			t.Fatalf("expected protocol %q to be valid", pr)
		}
	}
	if domain.HeartwormProtocol("INVALID_PROTOCOL").IsValid() {
		t.Fatal("expected invalid protocol to return false")
	}

	baseOp := domain.HeartwormCavalOperation{
		ID:            "HWC-OP-900",
		CaseID:        "CASE-HWC-900",
		PetName:       "Boudreaux",
		WeightKg:      28.5,
		Status:        domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged,
		SeverityClass: domain.HeartwormClassIVCavalSyndrome,
		Protocol:      domain.HeartwormProtocolRightJugularIshiharaExtraction,
	}
	if err := baseOp.Validate(); err != nil {
		t.Fatalf("expected valid operation, got %v", err)
	}

	invalidCases := []struct {
		name   string
		mutate func(op *domain.HeartwormCavalOperation)
	}{
		{"empty ID", func(op *domain.HeartwormCavalOperation) { op.ID = " " }},
		{"empty CaseID", func(op *domain.HeartwormCavalOperation) { op.CaseID = "" }},
		{"empty PetName", func(op *domain.HeartwormCavalOperation) { op.PetName = "" }},
		{"non-positive weight", func(op *domain.HeartwormCavalOperation) { op.WeightKg = 0 }},
		{"invalid status", func(op *domain.HeartwormCavalOperation) { op.Status = "BAD" }},
		{"invalid severity_class", func(op *domain.HeartwormCavalOperation) { op.SeverityClass = "BAD" }},
		{"invalid protocol", func(op *domain.HeartwormCavalOperation) { op.Protocol = "BAD" }},
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

func TestNewHeartwormServiceSeededOperationsAndOverview(t *testing.T) {
	svc := NewHeartwormService()
	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("HWC-OP-101")
	if err != nil {
		t.Fatalf("expected HWC-OP-101, got %v", err)
	}
	if op101.Status != domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged {
		t.Fatalf("unexpected HWC-OP-101 status: %s", op101.Status)
	}
	if !op101.ImmediateMelarsomineContraindicated || !op101.ParallelHyperechoicCavalLines || !op101.FreeHemoglobinuriaPresent {
		t.Fatal("expected Class IV Caval Syndrome safety flags on HWC-OP-101")
	}

	op102, err := svc.GetOperation("HWC-OP-102")
	if err != nil {
		t.Fatalf("expected HWC-OP-102, got %v", err)
	}
	if op102.Status != domain.HeartwormStatusPulmonaryHypertensionPTEMonitored {
		t.Fatalf("unexpected HWC-OP-102 status: %s", op102.Status)
	}

	op103, err := svc.GetOperation("HWC-OP-103")
	if err != nil {
		t.Fatalf("expected HWC-OP-103, got %v", err)
	}
	if op103.Status != domain.HeartwormStatusCavalPatentAntigenNegativeCleared {
		t.Fatalf("unexpected HWC-OP-103 status: %s", op103.Status)
	}

	if _, err := svc.GetOperation("HWC-OP-999"); err == nil {
		t.Fatal("expected error for unknown operation")
	}

	filtered := svc.ListOperations(string(domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged))
	if len(filtered) != 1 || filtered[0].ID != "HWC-OP-101" {
		t.Fatalf("expected 1 filtered operation HWC-OP-101, got %+v", filtered)
	}

	overview := svc.GetOverviewMetrics()
	if overview.TotalOperations != 3 ||
		overview.JugularVenotomyWolbachiaStagedCount != 1 ||
		overview.PulmonaryHypertensionPTEMonitoredCount != 1 ||
		overview.CavalPatentClearedCount != 1 ||
		overview.ClassIVCavalSyndromeCount != 1 ||
		overview.SeverePulmonaryHypertensionCount != 2 ||
		overview.ActiveHemoglobinuriaCount != 1 {
		t.Fatalf("unexpected overview metrics: %+v", overview)
	}
}

func TestCalculateEstimatedRVSPAndNarratives(t *testing.T) {
	if got := CalculateEstimatedRVSPMmHg(0); got != 0 {
		t.Fatalf("expected 0 RVSP for 0 velocity, got %v", got)
	}
	if got := CalculateEstimatedRVSPMmHg(-1.5); got != 0 {
		t.Fatalf("expected 0 RVSP for negative velocity, got %v", got)
	}
	if got := CalculateEstimatedRVSPMmHg(3.5); got != 49.0 {
		t.Fatalf("expected 49.0 mmHg for 3.5 m/s, got %v", got)
	}

	// Test all severity classes and TR jet branches
	classes := []domain.HeartwormClass{
		domain.HeartwormClassIVCavalSyndrome,
		domain.HeartwormClassIIISeverePTERCHF,
		domain.HeartwormClassIIModerateAntigenPos,
		domain.HeartwormClassFelineHARDDirofilariasis,
		domain.HeartwormClass("OTHER"),
	}
	velocities := []float64{4.0, 3.1, 2.2}
	for _, cl := range classes {
		for _, v := range velocities {
			patho, echo, hemolysis := BuildClinicalNarratives(cl, v, 12, 22.0, v > 3.5, v > 3.0)
			if patho == "" || echo == "" || hemolysis == "" {
				t.Fatalf("expected non-empty narratives for %s at %v m/s", cl, v)
			}
		}
	}

	stageFeline := BuildDefaultInterventionStaging(
		4.5,
		domain.HeartwormClassFelineHARDDirofilariasis,
		domain.HeartwormProtocolSildenafilPrednisoneCageRest,
		2.5,
		false,
		false,
		"",
		time.Now().UTC(),
	)
	if !stageFeline.ImmediateMelarsomineContraindicated {
		t.Fatal("expected immediate melarsomine contraindicated for Feline HARD")
	}
	if !strings.Contains(stageFeline.SplitDoseMelarsomineScheduleNote, "FELINE CONTRAINDICATION") {
		t.Fatalf("expected feline contraindication note, got %s", stageFeline.SplitDoseMelarsomineScheduleNote)
	}
}

func TestCreateOperationAndValidationGuards(t *testing.T) {
	svc := NewHeartwormService()

	created, err := svc.CreateOperation(
		"CASE-HWC-104",
		"Beignet",
		"",
		"",
		24.5,
		domain.HeartwormClassIVCavalSyndrome,
		"",
		4.1,
		0,
		19.5,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("expected CreateOperation to succeed, got %v", err)
	}
	if created.ID != "HWC-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Fatalf("unexpected defaults on created operation: %+v", created)
	}
	if !created.ImmediateMelarsomineContraindicated || !created.SildenafilRequiredAlert {
		t.Fatalf("expected contraindication and sildenafil alerts on created operation: %+v", created)
	}

	// Test validation error branches
	if _, err := svc.CreateOperation("", "Pet", "Canine", "Lab", 20, domain.HeartwormClassIVCavalSyndrome, domain.HeartwormProtocolRightJugularIshiharaExtraction, 3.5, 0, 25, true, true); err == nil {
		t.Fatal("expected error for empty case_id")
	}
	if _, err := svc.CreateOperation("CASE-1", "", "Canine", "Lab", 20, domain.HeartwormClassIVCavalSyndrome, domain.HeartwormProtocolRightJugularIshiharaExtraction, 3.5, 0, 25, true, true); err == nil {
		t.Fatal("expected error for empty pet_name")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 0, domain.HeartwormClassIVCavalSyndrome, domain.HeartwormProtocolRightJugularIshiharaExtraction, 3.5, 0, 25, true, true); err == nil {
		t.Fatal("expected error for invalid weight_kg")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 20, "BAD_CLASS", domain.HeartwormProtocolRightJugularIshiharaExtraction, 3.5, 0, 25, true, true); err == nil {
		t.Fatal("expected error for invalid severity_class")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 20, domain.HeartwormClassIVCavalSyndrome, "BAD_PROTO", 3.5, 0, 25, true, true); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 20, domain.HeartwormClassIVCavalSyndrome, domain.HeartwormProtocolRightJugularIshiharaExtraction, 0, 0, 25, true, true); err == nil {
		t.Fatal("expected error for invalid tr_jet_velocity_ms")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 20, domain.HeartwormClassIVCavalSyndrome, domain.HeartwormProtocolRightJugularIshiharaExtraction, 3.5, -1, 25, true, true); err == nil {
		t.Fatal("expected error for negative worms_extracted_count")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Lab", 20, domain.HeartwormClassIVCavalSyndrome, domain.HeartwormProtocolRightJugularIshiharaExtraction, 3.5, 0, 0, true, true); err == nil {
		t.Fatal("expected error for invalid pcv_percent")
	}
}

func TestStageInterventionAndMelarsomineSafetyGuard(t *testing.T) {
	svc := NewHeartwormService()

	// Safety Guard: Attempting immediate Melarsomine on HWC-OP-101 (active Class IV Caval Syndrome with parallel hyperechoic lines & hemoglobinuria) with immediateMelarsomineContraindicated=false must fail
	_, err := svc.StageIntervention(
		"HWC-OP-101",
		domain.HeartwormProtocolSplitDoseMelarsomineDay609091,
		"Attempt immediate adulticide without extraction",
		false,
		20.0,
		"Doxycycline 10 mg/kg PO q12h x 28 days",
		"Macrocyclic lactone Day 1, 30, 60",
		1.0,
		"", "", "",
		8,
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "strictly contraindicated") {
		t.Fatalf("expected immediate melarsomine contraindication error, got %v", err)
	}

	// Valid staging on HWC-OP-101 with right jugular Ishihara extraction & defaults
	staged, err := svc.StageIntervention(
		"HWC-OP-101",
		"",
		"Right external jugular venotomy with Ishihara flexible alligator forceps under fluoroscopy",
		true,
		0,
		"Doxycycline 10 mg/kg PO q12h for 28 days",
		"Ivermectin/Milbemycin preventive + Diphenhydramine/Prednisone pretreatment",
		0,
		"", "", "",
		0,
		"",
	)
	if err != nil {
		t.Fatalf("expected StageIntervention to succeed, got %v", err)
	}
	if staged.Status != domain.HeartwormStatusPulmonaryHypertensionPTEMonitored {
		t.Fatalf("expected status %s, got %s", domain.HeartwormStatusPulmonaryHypertensionPTEMonitored, staged.Status)
	}
	if !staged.ImmediateMelarsomineContraindicated {
		t.Fatal("expected ImmediateMelarsomineContraindicated to remain true on active Class IV Caval Syndrome")
	}

	// Validation error branches on StageIntervention
	if _, err := svc.StageIntervention("HWC-OP-101", "BAD_PROTO", "Plan", true, 20, "Doxy", "Lactone", 1.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.StageIntervention("HWC-OP-101", domain.HeartwormProtocolRightJugularIshiharaExtraction, "", true, 20, "Doxy", "Lactone", 1.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for empty jugular plan")
	}
	if _, err := svc.StageIntervention("HWC-OP-101", domain.HeartwormProtocolRightJugularIshiharaExtraction, "Plan", true, 20, "", "Lactone", 1.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for empty doxycycline plan")
	}
	if _, err := svc.StageIntervention("HWC-OP-101", domain.HeartwormProtocolRightJugularIshiharaExtraction, "Plan", true, 20, "Doxy", "", 1.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for empty lactone plan")
	}
	if _, err := svc.StageIntervention("HWC-OP-101", domain.HeartwormProtocolRightJugularIshiharaExtraction, "Plan", true, 5.0, "Doxy", "Lactone", 1.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for low doxycycline dose < 10 mg/kg/day")
	}
	if _, err := svc.StageIntervention("HWC-OP-101", domain.HeartwormProtocolRightJugularIshiharaExtraction, "Plan", true, 20.0, "Doxy", "Lactone", 5.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for excessive prednisone dose > 4.0 mg/kg/day")
	}
	if _, err := svc.StageIntervention("HWC-OP-101", domain.HeartwormProtocolRightJugularIshiharaExtraction, "Plan", true, 20.0, "Doxy", "Lactone", 1.0, "", "", "", 2, "Dr"); err == nil {
		t.Fatal("expected error for insufficient strict cage rest < 6 weeks")
	}
	if _, err := svc.StageIntervention("UNKNOWN-ID", domain.HeartwormProtocolRightJugularIshiharaExtraction, "Plan", true, 20.0, "Doxy", "Lactone", 1.0, "", "", "", 8, "Dr"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}
}

func TestRecordHemodynamicCheckAndEvaluateClearance(t *testing.T) {
	svc := NewHeartwormService()

	// Record check with TR jet > 3.4 m/s and 0 sildenafil auto-triggers 2.0 mg/kg Sildenafil
	updated, err := svc.RecordHemodynamicCheck(
		"HWC-OP-101",
		3.6,
		31,
		30.5,
		false,
		false,
		0.0,
		"Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		"Extracted 31 adult Dirofilaria via right jugular Ishihara forceps; caval lumen now patent and hemoglobinuria clearing.",
	)
	if err != nil {
		t.Fatalf("expected RecordHemodynamicCheck to succeed, got %v", err)
	}
	if updated.WormsExtractedCount != 31 || !updated.SildenafilRequiredAlert {
		t.Fatalf("unexpected updated hemodynamic check fields: %+v", updated)
	}
	if updated.Status != domain.HeartwormStatusPulmonaryHypertensionPTEMonitored {
		t.Fatalf("expected status transition to PULMONARY_HYPERTENSION_PTE_MONITORED, got %s", updated.Status)
	}

	// Validation errors for RecordHemodynamicCheck
	if _, err := svc.RecordHemodynamicCheck("HWC-OP-101", 0, 10, 30, false, false, 1.0, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid tr_jet_velocity_ms")
	}
	if _, err := svc.RecordHemodynamicCheck("HWC-OP-101", 2.5, -1, 30, false, false, 1.0, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid worms_extracted_count")
	}
	if _, err := svc.RecordHemodynamicCheck("HWC-OP-101", 2.5, 10, 0, false, false, 1.0, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid pcv_percent")
	}
	if _, err := svc.RecordHemodynamicCheck("HWC-OP-101", 2.5, 10, 30, false, false, -1.0, "Dr", "note"); err == nil {
		t.Fatal("expected error for negative sildenafil dose")
	}
	if _, err := svc.RecordHemodynamicCheck("HWC-OP-101", 2.5, 10, 30, false, false, 1.0, "", "note"); err == nil {
		t.Fatal("expected error for empty recorded_by")
	}
	if _, err := svc.RecordHemodynamicCheck("UNKNOWN-ID", 2.5, 10, 30, false, false, 1.0, "Dr", "note"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}

	// EvaluateClearance safety guards
	if _, err := svc.EvaluateClearance("HWC-OP-101", true, false, 2.2, true, "Dr. Broussard", "Summary"); err == nil {
		t.Fatal("expected clearance error when parallel_hyperechoic_caval_lines is true")
	}
	if _, err := svc.EvaluateClearance("HWC-OP-101", false, true, 2.2, true, "Dr. Broussard", "Summary"); err == nil {
		t.Fatal("expected clearance error when free_hemoglobinuria_present is true")
	}
	if _, err := svc.EvaluateClearance("HWC-OP-101", false, false, 3.2, true, "Dr. Broussard", "Summary"); err == nil {
		t.Fatal("expected clearance error when tr_jet_velocity_ms > 2.8")
	}
	if _, err := svc.EvaluateClearance("HWC-OP-101", false, false, 2.2, false, "Dr. Broussard", "Summary"); err == nil {
		t.Fatal("expected clearance error when strict_cage_rest_verified is false")
	}
	if _, err := svc.EvaluateClearance("HWC-OP-101", false, false, 2.2, true, "", "Summary"); err == nil {
		t.Fatal("expected clearance error when cleared_by is empty")
	}
	if _, err := svc.EvaluateClearance("HWC-OP-101", false, false, 2.2, true, "Dr. Broussard", ""); err == nil {
		t.Fatal("expected clearance error when clearance_summary is empty")
	}
	if _, err := svc.EvaluateClearance("UNKNOWN-ID", false, false, 2.2, true, "Dr. Broussard", "Summary"); err == nil {
		t.Fatal("expected clearance error for unknown operation ID")
	}

	// Successful clearance
	cleared, err := svc.EvaluateClearance(
		"HWC-OP-101",
		false,
		false,
		2.2,
		true,
		"Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		"Caval lumen patent, hemoglobinuria resolved, TR jet 2.2 m/s (<= 2.8 m/s), 8-week strict cage rest verified; cleared.",
	)
	if err != nil {
		t.Fatalf("expected EvaluateClearance to succeed, got %v", err)
	}
	if cleared.Status != domain.HeartwormStatusCavalPatentAntigenNegativeCleared {
		t.Fatalf("expected status %s, got %s", domain.HeartwormStatusCavalPatentAntigenNegativeCleared, cleared.Status)
	}
	if cleared.Clearance == nil || cleared.DischargedAt == nil {
		t.Fatal("expected clearance and discharged_at to be populated")
	}
}

func TestConcurrentHeartwormServiceOperations(t *testing.T) {
	svc := NewHeartwormService()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			op, err := svc.CreateOperation(
				fmt.Sprintf("CASE-CONC-%d", idx),
				fmt.Sprintf("Bayou-%d", idx),
				"Canine",
				"Catahoula",
				22.0,
				domain.HeartwormClassIVCavalSyndrome,
				domain.HeartwormProtocolRightJugularIshiharaExtraction,
				3.8,
				0,
				22.0,
				true,
				true,
			)
			if err != nil {
				t.Errorf("concurrent CreateOperation failed: %v", err)
				return
			}
			_, _ = svc.RecordHemodynamicCheck(op.ID, 2.4, 20, 38.0, false, false, 1.5, "Dr. Concurrent", "Post-venotomy check")
			_, _ = svc.EvaluateClearance(op.ID, false, false, 2.4, true, "Dr. Concurrent", "Cleared")
			_ = svc.ListOperations("")
			_ = svc.CalculateOverview()
		}(i)
	}
	wg.Wait()
}
