package polytraumaortho_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/polytraumaortho"
)

func TestOrthoSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := polytraumaortho.NewOrthoService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded Ortho operations, got %d", len(ops))
	}

	// Verify ORTHO-OP-101 ("Ranger")
	op101, err := svc.GetOperation("ORTHO-OP-101")
	if err != nil {
		t.Fatalf("expected ORTHO-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Ranger" || op101.Species != "Canine" || op101.Breed != "Border Collie" || op101.WeightKg != 19.5 {
		t.Errorf("unexpected ORTHO-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.OrthoStatusAcuteOpenFractureTriage || op101.GustiloClass != domain.GustiloClassGradeIIIShearingDegloving {
		t.Errorf("unexpected ORTHO-OP-101 status/gustilo_class: %s / %s", op101.Status, op101.GustiloClass)
	}
	if op101.AnimalTraumaTriageScore != 9 || op101.LavageVolumeML != 1500 || op101.NPWTVacuumMmHg != 0 {
		t.Errorf("unexpected ORTHO-OP-101 ATT/Lavage/NPWT: att=%d lavage=%d npwt=%d", op101.AnimalTraumaTriageScore, op101.LavageVolumeML, op101.NPWTVacuumMmHg)
	}
	if op101.BloodLactateMmolL != 4.6 || op101.DistalToeTempDeltaC != 1.8 {
		t.Errorf("unexpected ORTHO-OP-101 Lactate/ToeDelta: lactate=%.1f delta=%.1f", op101.BloodLactateMmolL, op101.DistalToeTempDeltaC)
	}
	if !op101.ExposedCorticalBoneTendon || op101.ThoracicWallPenetrationSuspected {
		t.Errorf("unexpected ORTHO-OP-101 boolean flags: exposed=%v thoracic=%v", op101.ExposedCorticalBoneTendon, op101.ThoracicWallPenetrationSuspected)
	}
	if len(op101.WoundChecks) != 2 {
		t.Errorf("expected 2 seeded WoundChecks on ORTHO-OP-101, got %d", len(op101.WoundChecks))
	}
	if !strings.Contains(op101.AssessmentPlan.HighPressureLavage7to8PSINote, "7–8 PSI") {
		t.Errorf("expected 7-8 PSI lavage note on ORTHO-OP-101, got %q", op101.AssessmentPlan.HighPressureLavage7to8PSINote)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected ORTHO-OP-101 to validate, got %v", err)
	}

	// Verify ORTHO-OP-102 ("Peanut")
	op102, err := svc.GetOperation("ORTHO-OP-102")
	if err != nil {
		t.Fatalf("expected ORTHO-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Peanut" || op102.Breed != "Chihuahua Mix" || op102.WeightKg != 4.2 {
		t.Errorf("unexpected ORTHO-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.OrthoStatusNPWTSplintStaged || op102.GustiloClass != domain.GustiloClassBDLDThoracicBiteFlail {
		t.Errorf("unexpected ORTHO-OP-102 status/gustilo_class: %s / %s", op102.Status, op102.GustiloClass)
	}
	if op102.AnimalTraumaTriageScore != 7 || op102.LavageVolumeML != 2000 || op102.NPWTVacuumMmHg != -125 || op102.BloodLactateMmolL != 2.3 || op102.DistalToeTempDeltaC != 0.4 {
		t.Errorf("unexpected ORTHO-OP-102 metrics: %+v", op102)
	}
	if op102.ExposedCorticalBoneTendon || !op102.ThoracicWallPenetrationSuspected {
		t.Errorf("unexpected ORTHO-OP-102 flags: exposed=%v thoracic=%v", op102.ExposedCorticalBoneTendon, op102.ThoracicWallPenetrationSuspected)
	}
	if len(op102.WoundChecks) != 1 {
		t.Errorf("expected 1 seeded WoundCheck on ORTHO-OP-102, got %d", len(op102.WoundChecks))
	}

	// Verify ORTHO-OP-103 ("Toby")
	op103, err := svc.GetOperation("ORTHO-OP-103")
	if err != nil {
		t.Fatalf("expected ORTHO-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Toby" || op103.Species != "Feline" || op103.Breed != "Domestic Shorthair" || op103.WeightKg != 5.1 {
		t.Errorf("unexpected ORTHO-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.OrthoStatusGranulationFixationCleared || op103.GustiloClass != domain.GustiloClassGradeIOpenLongBoneFracture {
		t.Errorf("unexpected ORTHO-OP-103 status/gustilo_class: %s / %s", op103.Status, op103.GustiloClass)
	}
	if op103.AnimalTraumaTriageScore != 2 || op103.LavageVolumeML != 2500 || op103.NPWTVacuumMmHg != 0 || op103.BloodLactateMmolL != 1.1 || op103.DistalToeTempDeltaC != 0.2 {
		t.Errorf("unexpected ORTHO-OP-103 metrics: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Caleb Thorne, DACVS (Orthopedics)" || op103.DischargedAt == nil || len(op103.WoundChecks) != 2 {
		t.Errorf("expected ORTHO-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.OrthoStatusAcuteOpenFractureTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "ORTHO-OP-101" {
		t.Errorf("expected 1 ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteOpenFractureTriageCount != 1 ||
		metrics.NPWTSplintStagedCount != 1 ||
		metrics.ClearedFixationCount != 1 ||
		metrics.GustiloGradeIIIShearingCount != 1 ||
		metrics.ThoracicBitePenetrationCount != 1 {
		t.Errorf("unexpected PolytraumaOrthoOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown Ortho operation ID")
	}
}

func TestOrthoCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := polytraumaortho.NewOrthoService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-ORTHO-104",
		"Bandit",
		"",
		"",
		24.0,
		domain.GustiloClassGradeIIIShearingDegloving,
		8,
		1800,
		0,
		3.9,
		1.6,
		true,
		false,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "ORTHO-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.AnimalTraumaTriageScore != 8 || created.LavageVolumeML != 1800 {
		t.Errorf("expected ATT=8 and Lavage=1800, got %d / %d", created.AnimalTraumaTriageScore, created.LavageVolumeML)
	}

	// Stage debridement & fixation with empty optional fields (tests fallback defaults)
	staged, err := svc.StageDebridementFixation(
		created.ID,
		"Pack open shearing wound with sterile K-Y water-soluble gel before wide #40 surgical clip",
		"Copious 7–8 PSI warm isotonic LRS lavage (2,500 mL via 1 L bag at 300 mmHg + 18G needle)",
		"NPWT Vacuum-Assisted Closure at -125 mmHg continuous suction over polyurethane foam",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageDebridementFixation error: %v", err)
	}
	if staged.Status != domain.OrthoStatusNPWTSplintStaged {
		t.Errorf("expected status %s, got %s", domain.OrthoStatusNPWTSplintStaged, staged.Status)
	}
	if staged.FixationPlan.EmpiricAbxCultureBeforeLavage == "" ||
		staged.FixationPlan.RobertJonesOrSpicaSplintPlan == "" ||
		staged.FixationPlan.DefinitiveExternalSkeletalFixator == "" {
		t.Errorf("expected default abx, splint, and ESF plans to be populated: %+v", staged.FixationPlan)
	}

	// Stage debridement & fixation with explicit optional fields
	stagedExplicit, err := svc.StageDebridementFixation(
		created.ID,
		"Sterile K-Y gel wound protection + wide clip",
		"3,000 mL warm LRS at 7-8 PSI via 35 mL syringe + 18G needle",
		"NPWT at -125 mmHg continuous vacuum",
		"Deep tissue culture post-lavage + Cefazolin 22 mg/kg IV q8h",
		"Modified Robert Jones splint immobilizing stifle and tarsus (joint above & joint below)",
		"Type II Transarticular External Skeletal Fixator (ESF)",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageDebridementFixation error: %v", err)
	}
	if stagedExplicit.FixationPlan.EmpiricAbxCultureBeforeLavage != "Deep tissue culture post-lavage + Cefazolin 22 mg/kg IV q8h" {
		t.Errorf("unexpected explicit abx plan: %q", stagedExplicit.FixationPlan.EmpiricAbxCultureBeforeLavage)
	}

	// Record wound check and verify operation metrics update
	checked, err := svc.RecordWoundCheck(
		created.ID,
		3,
		3000,
		-125,
		1.4,
		0.3,
		"Dr. Caleb Thorne, DACVS (Orthopedics)",
		"72h post-NPWT (-125 mmHg) & transarticular ESF: robust granulation bed covering exposed talus, ATT score improved to 3, Lactate 1.4 mmol/L, toe-web ΔT 0.3°C.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordWoundCheck error: %v", err)
	}
	if len(checked.WoundChecks) != 1 {
		t.Fatalf("expected 1 WoundCheck on ORTHO-OP-104, got %d", len(checked.WoundChecks))
	}
	if checked.AnimalTraumaTriageScore != 3 || checked.LavageVolumeML != 3000 || checked.NPWTVacuumMmHg != -125 || checked.BloodLactateMmolL != 1.4 || checked.DistalToeTempDeltaC != 0.3 {
		t.Errorf("expected operation vitals updated from WoundCheck, got %+v", checked)
	}

	// Clear Ortho discharge
	cleared, err := svc.ClearOrthoDischarge(
		created.ID,
		"Dr. Caleb Thorne, DACVS (Orthopedics)",
		"Healthy granulation bed covering exposed cortical bone, transarticular ESF stable, ATT score 3, Lactate 1.4 mmol/L, distal toe-web ΔT 0.3°C; cleared for orthopedic step-down.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearOrthoDischarge error: %v", err)
	}
	if cleared.Status != domain.OrthoStatusGranulationFixationCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Caleb Thorne, DACVS (Orthopedics)" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestOrthoValidationErrors(t *testing.T) {
	svc := polytraumaortho.NewOrthoService()

	createCases := []struct {
		name         string
		caseID       string
		petName      string
		weightKg     float64
		gustiloClass domain.GustiloInjuryClassification
		attScore     int
		lavageML     int
		npwtMmHg     int
		lactate      float64
		toeDeltaC    float64
	}{
		{"empty case_id", "", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 0, 4.6, 1.8},
		{"empty pet_name", "CASE-1", "", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 0, 4.6, 1.8},
		{"weight <= 0", "CASE-1", "Ranger", 0, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 0, 4.6, 1.8},
		{"weight > 120", "CASE-1", "Ranger", 130, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 0, 4.6, 1.8},
		{"invalid gustilo_class", "CASE-1", "Ranger", 19.5, "INVALID_GUSTILO", 9, 1500, 0, 4.6, 1.8},
		{"attScore < 0", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, -1, 1500, 0, 4.6, 1.8},
		{"attScore > 18", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 19, 1500, 0, 4.6, 1.8},
		{"lavageML < 0", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, -100, 0, 4.6, 1.8},
		{"npwtMmHg < -250", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, -300, 4.6, 1.8},
		{"npwtMmHg > 0", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 50, 4.6, 1.8},
		{"lactate <= 0", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 0, 0, 1.8},
		{"toeDeltaC < 0", "CASE-1", "Ranger", 19.5, domain.GustiloClassGradeIIIShearingDegloving, 9, 1500, 0, 4.6, -0.5},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Border Collie",
				tc.weightKg, tc.gustiloClass,
				tc.attScore, tc.lavageML, tc.npwtMmHg,
				tc.lactate, tc.toeDeltaC,
				true, false,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageDebridementFixation validation errors
	if _, err := svc.StageDebridementFixation("ORTHO-OP-101", "", "Lavage", "NPWT", "", "", ""); err == nil {
		t.Errorf("expected error for empty gelClipPlan")
	}
	if _, err := svc.StageDebridementFixation("ORTHO-OP-101", "GelClip", "", "NPWT", "", "", ""); err == nil {
		t.Errorf("expected error for empty lavagePlan")
	}
	if _, err := svc.StageDebridementFixation("ORTHO-OP-101", "GelClip", "Lavage", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty npwtOrTieOverPlan")
	}
	if _, err := svc.StageDebridementFixation("NON-EXISTENT", "GelClip", "Lavage", "NPWT", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageDebridementFixation")
	}

	// RecordWoundCheck validation errors
	if _, err := svc.RecordWoundCheck("ORTHO-OP-101", -1, 1500, -125, 2.0, 0.5, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for negative attScore")
	}
	if _, err := svc.RecordWoundCheck("ORTHO-OP-101", 5, -1, -125, 2.0, 0.5, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for negative lavageVolumeML")
	}
	if _, err := svc.RecordWoundCheck("ORTHO-OP-101", 5, 1500, 20, 2.0, 0.5, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for positive npwtVacuumMmHg")
	}
	if _, err := svc.RecordWoundCheck("ORTHO-OP-101", 5, 1500, -125, 0, 0.5, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for non-positive bloodLactateMmolL")
	}
	if _, err := svc.RecordWoundCheck("ORTHO-OP-101", 5, 1500, -125, 2.0, -0.5, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for negative distalToeTempDeltaC")
	}
	if _, err := svc.RecordWoundCheck("ORTHO-OP-101", 5, 1500, -125, 2.0, 0.5, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordWoundCheck("NON-EXISTENT", 5, 1500, -125, 2.0, 0.5, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordWoundCheck")
	}

	// ClearOrthoDischarge validation errors
	if _, err := svc.ClearOrthoDischarge("ORTHO-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearOrthoDischarge("ORTHO-OP-101", "Dr. Thorne", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearOrthoDischarge("NON-EXISTENT", "Dr. Thorne", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearOrthoDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("ORTHO-OP-101")
	badOps := []func(op *domain.PolytraumaOrthoOperation){
		func(op *domain.PolytraumaOrthoOperation) { op.ID = "" },
		func(op *domain.PolytraumaOrthoOperation) { op.CaseID = "" },
		func(op *domain.PolytraumaOrthoOperation) { op.PetName = "" },
		func(op *domain.PolytraumaOrthoOperation) { op.WeightKg = 0 },
		func(op *domain.PolytraumaOrthoOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.PolytraumaOrthoOperation) { op.GustiloClass = "BAD_GUSTILO" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildGustiloWoundAssessmentPlan default switch branch coverage
	planDefault := polytraumaortho.BuildGustiloWoundAssessmentPlan("CUSTOM_GUSTILO", 4, 1500, -125, 1.8, 0.4, false, false)
	if !strings.Contains(planDefault.GustiloGradePathophysiology, "Polytrauma") {
		t.Errorf("expected default pathophysiology summary, got %q", planDefault.GustiloGradePathophysiology)
	}
}
