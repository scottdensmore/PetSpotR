package hepaticencephalopathy_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hepaticencephalopathy"
)

func TestHepaticSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := hepaticencephalopathy.NewHepaticService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded Hepatic operations, got %d", len(ops))
	}

	// Verify HEP-OP-101 ("Kona")
	op101, err := svc.GetOperation("HEP-OP-101")
	if err != nil {
		t.Fatalf("expected HEP-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Kona" || op101.Species != "Canine" || op101.Breed != "Labrador Retriever" || op101.WeightKg != 26.4 {
		t.Errorf("unexpected HEP-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.HepaticStatusFulminantHETriage || op101.EtiologyType != domain.HepatotoxinEtiologySagoPalmCycasin {
		t.Errorf("unexpected HEP-OP-101 status/etiology: %s / %s", op101.Status, op101.EtiologyType)
	}
	if op101.ALTIUL != 2840 || op101.TotalBilirubinMgDL != 6.8 || op101.BloodAmmoniaUmolL != 245.0 || op101.BloodGlucoseMgDL != 48.0 || op101.PTSeconds != 26.4 {
		t.Errorf("unexpected HEP-OP-101 metabolic values: %+v", op101)
	}
	if op101.EncephalopathyGrade != 3 || !op101.HeadPressingSeizuresPresent {
		t.Errorf("unexpected HEP-OP-101 neuro status: grade=%d seizures=%v", op101.EncephalopathyGrade, op101.HeadPressingSeizuresPresent)
	}
	if len(op101.MetabolicChecks) != 2 {
		t.Errorf("expected 2 seeded MetabolicChecks on HEP-OP-101, got %d", len(op101.MetabolicChecks))
	}
	if !strings.Contains(op101.EncephalopathyPlan.BenzodiazepineMethionineWarning, "Benzodiazepines") {
		t.Errorf("expected benzodiazepine warning on HEP-OP-101, got %q", op101.EncephalopathyPlan.BenzodiazepineMethionineWarning)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected HEP-OP-101 to validate, got %v", err)
	}

	// Verify HEP-OP-102 ("Marmalade")
	op102, err := svc.GetOperation("HEP-OP-102")
	if err != nil {
		t.Fatalf("expected HEP-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Marmalade" || op102.Species != "Feline" || op102.Breed != "Domestic Shorthair" || op102.WeightKg != 5.1 {
		t.Errorf("unexpected HEP-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.HepaticStatusNACLactuloseStaged || op102.EtiologyType != domain.HepatotoxinEtiologyFelineLipidosisPSS {
		t.Errorf("unexpected HEP-OP-102 status/etiology: %s / %s", op102.Status, op102.EtiologyType)
	}
	if op102.ALTIUL != 620 || op102.TotalBilirubinMgDL != 8.4 || op102.BloodAmmoniaUmolL != 138.0 || op102.BloodGlucoseMgDL != 82.0 || op102.PTSeconds != 19.2 || op102.EncephalopathyGrade != 2 || op102.HeadPressingSeizuresPresent {
		t.Errorf("unexpected HEP-OP-102 metrics: %+v", op102)
	}
	if len(op102.MetabolicChecks) != 1 {
		t.Errorf("expected 1 seeded MetabolicCheck on HEP-OP-102, got %d", len(op102.MetabolicChecks))
	}

	// Verify HEP-OP-103 ("Scout")
	op103, err := svc.GetOperation("HEP-OP-103")
	if err != nil {
		t.Fatalf("expected HEP-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Scout" || op103.Species != "Canine" || op103.Breed != "Australian Shepherd" || op103.WeightKg != 18.2 {
		t.Errorf("unexpected HEP-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.HepaticStatusAmmoniaNormalizedCleared || op103.EtiologyType != domain.HepatotoxinEtiologyBlueGreenAlgaeAmanita {
		t.Errorf("unexpected HEP-OP-103 status/etiology: %s / %s", op103.Status, op103.EtiologyType)
	}
	if op103.ALTIUL != 210 || op103.TotalBilirubinMgDL != 0.9 || op103.BloodAmmoniaUmolL != 32.0 || op103.BloodGlucoseMgDL != 98.0 || op103.PTSeconds != 11.5 || op103.EncephalopathyGrade != 0 || op103.HeadPressingSeizuresPresent {
		t.Errorf("unexpected HEP-OP-103 values: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Priya Nair, DACVECC" || op103.DischargedAt == nil || len(op103.MetabolicChecks) != 2 {
		t.Errorf("expected HEP-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.HepaticStatusFulminantHETriage))
	if len(triageOps) != 1 || triageOps[0].ID != "HEP-OP-101" {
		t.Errorf("expected 1 ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.FulminantHETriageCount != 1 ||
		metrics.NACLactuloseStagedCount != 1 ||
		metrics.ClearedHepaticCount != 1 ||
		metrics.SagoPalmMicrocystinCount != 2 ||
		metrics.HyperammonemicEncephalopathyCount != 2 {
		t.Errorf("unexpected HepaticOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown Hepatic operation ID")
	}
}

func TestHepaticCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := hepaticencephalopathy.NewHepaticService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-HEP-104",
		"Bodie",
		"",
		"",
		24.0,
		domain.HepatotoxinEtiologySagoPalmCycasin,
		3120,
		7.2,
		260.0,
		52.0,
		27.8,
		3,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "HEP-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.ALTIUL != 3120 || created.BloodAmmoniaUmolL != 260.0 || created.BloodGlucoseMgDL != 52.0 || created.PTSeconds != 27.8 {
		t.Errorf("unexpected created operation metabolic values: %+v", created)
	}

	// Stage NAC/Lactulose support protocol with empty optional fields (tests fallback defaults)
	staged, err := svc.StageNACLactuloseSupport(
		created.ID,
		"NAC 140 mg/kg IV loading dose via 0.2-micron filter, then 70 mg/kg IV q6h x 7 doses",
		"",
		"Lactulose 1:3 warm retention enema + oral Lactulose 0.5 mL/kg PO q8h + Metronidazole 7.5 mg/kg IV q12h",
		"Levetiracetam 60 mg/kg IV loading dose, then 20 mg/kg IV q8h (avoid Diazepam/Midazolam)",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageNACLactuloseSupport error: %v", err)
	}
	if staged.Status != domain.HepaticStatusNACLactuloseStaged {
		t.Errorf("expected status %s, got %s", domain.HepaticStatusNACLactuloseStaged, staged.Status)
	}
	if staged.SupportPlan.SAMeSilybinHepatoprotectant == "" ||
		staged.SupportPlan.DextroseVitaminK1FFPPlan == "" ||
		staged.SupportPlan.PotassiumPhosphateThiaminePlan == "" {
		t.Errorf("expected default SAMe/Silybin, Dextrose/VitK1/FFP, and K+/PO4/Thiamine notes to be populated: %+v", staged.SupportPlan)
	}

	// Stage NAC/Lactulose support protocol with explicit optional fields
	stagedExplicit, err := svc.StageNACLactuloseSupport(
		created.ID,
		"NAC 140 mg/kg IV load",
		"SAMe 20 mg/kg PO q24h + Silybin 50 mg/kg IV",
		"Lactulose warm enema q6h",
		"Levetiracetam 60 mg/kg IV",
		"5% Dextrose CRI + Vitamin K1 2 mg/kg SC q12h + FFP 15 mL/kg IV",
		"KCl 30 mEq/L + KPO4 0.02 mmol/kg/hr + Thiamine 100 mg SC q12h",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageNACLactuloseSupport error: %v", err)
	}
	if stagedExplicit.SupportPlan.SAMeSilybinHepatoprotectant != "SAMe 20 mg/kg PO q24h + Silybin 50 mg/kg IV" {
		t.Errorf("unexpected explicit SAMe/Silybin plan: %q", stagedExplicit.SupportPlan.SAMeSilybinHepatoprotectant)
	}

	// Record metabolic check and verify operation metrics update
	checked, err := svc.RecordMetabolicCheck(
		created.ID,
		410,
		1.4,
		38.0,
		102.0,
		12.2,
		"Dr. Priya Nair, DACVECC",
		"Post-NAC & Lactulose retention enemas: blood ammonia normalized to 38.0 umol/L, ALT reduced to 410 IU/L, euglycemic 102.0 mg/dL, PT 12.2 sec.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordMetabolicCheck error: %v", err)
	}
	if len(checked.MetabolicChecks) != 1 {
		t.Fatalf("expected 1 MetabolicCheck on HEP-OP-104, got %d", len(checked.MetabolicChecks))
	}
	if checked.ALTIUL != 410 || checked.TotalBilirubinMgDL != 1.4 || checked.BloodAmmoniaUmolL != 38.0 || checked.BloodGlucoseMgDL != 102.0 || checked.PTSeconds != 12.2 {
		t.Errorf("expected operation vitals updated from MetabolicCheck, got %+v", checked)
	}

	// Clear Hepatic discharge
	cleared, err := svc.ClearHepaticDischarge(
		created.ID,
		"Dr. Priya Nair, DACVECC",
		"Bodie recovered from Sago Palm hepatotoxicity; ammonia normalized (38.0 umol/L), HE resolved, PT 12.2 sec, discharged on oral SAMe/Silybin + Lactulose.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearHepaticDischarge error: %v", err)
	}
	if cleared.Status != domain.HepaticStatusAmmoniaNormalizedCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Priya Nair, DACVECC" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestHepaticValidationErrors(t *testing.T) {
	svc := hepaticencephalopathy.NewHepaticService()

	// CreateOperation validation errors
	createCases := []struct {
		name      string
		caseID    string
		petName   string
		weightKg  float64
		etiology  domain.HepatotoxinEtiologyType
		alt       int
		bilirubin float64
		ammonia   float64
		glucose   float64
		pt        float64
		heGrade   int
	}{
		{"empty case_id", "", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 60, 20, 2},
		{"empty pet_name", "CASE-1", "", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 60, 20, 2},
		{"weight <= 0", "CASE-1", "Kona", 0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 60, 20, 2},
		{"weight > 120", "CASE-1", "Kona", 130, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 60, 20, 2},
		{"invalid etiology", "CASE-1", "Kona", 25.0, "INVALID_ETIOLOGY", 2000, 5.0, 180, 60, 20, 2},
		{"alt <= 0", "CASE-1", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 0, 5.0, 180, 60, 20, 2},
		{"bilirubin < 0", "CASE-1", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, -1.0, 180, 60, 20, 2},
		{"ammonia <= 0", "CASE-1", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 0, 60, 20, 2},
		{"glucose out of range", "CASE-1", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 10, 20, 2},
		{"pt out of range", "CASE-1", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 60, 2, 2},
		{"heGrade out of range", "CASE-1", "Kona", 25.0, domain.HepatotoxinEtiologySagoPalmCycasin, 2000, 5.0, 180, 60, 20, 5},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Labrador Retriever",
				tc.weightKg, tc.etiology,
				tc.alt, tc.bilirubin, tc.ammonia, tc.glucose, tc.pt, tc.heGrade,
				true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageNACLactuloseSupport validation errors
	if _, err := svc.StageNACLactuloseSupport("HEP-OP-101", "", "SAMe", "Lactulose", "Levetiracetam", "", ""); err == nil {
		t.Errorf("expected error for empty nacProtocol")
	}
	if _, err := svc.StageNACLactuloseSupport("HEP-OP-101", "NAC", "SAMe", "", "Levetiracetam", "", ""); err == nil {
		t.Errorf("expected error for empty lactuloseProtocol")
	}
	if _, err := svc.StageNACLactuloseSupport("HEP-OP-101", "NAC", "SAMe", "Lactulose", "", "", ""); err == nil {
		t.Errorf("expected error for empty levetiracetamProtocol")
	}
	if _, err := svc.StageNACLactuloseSupport("NON-EXISTENT", "NAC", "SAMe", "Lactulose", "Levetiracetam", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageNACLactuloseSupport")
	}

	// RecordMetabolicCheck validation errors
	if _, err := svc.RecordMetabolicCheck("HEP-OP-101", 0, 2.0, 50.0, 90.0, 12.0, "Dr. Nair", "note"); err == nil {
		t.Errorf("expected error for non-positive ALT")
	}
	if _, err := svc.RecordMetabolicCheck("HEP-OP-101", 300, -1.0, 50.0, 90.0, 12.0, "Dr. Nair", "note"); err == nil {
		t.Errorf("expected error for negative bilirubin")
	}
	if _, err := svc.RecordMetabolicCheck("HEP-OP-101", 300, 2.0, 0, 90.0, 12.0, "Dr. Nair", "note"); err == nil {
		t.Errorf("expected error for non-positive ammonia")
	}
	if _, err := svc.RecordMetabolicCheck("HEP-OP-101", 300, 2.0, 50.0, 5.0, 12.0, "Dr. Nair", "note"); err == nil {
		t.Errorf("expected error for glucose out of range")
	}
	if _, err := svc.RecordMetabolicCheck("HEP-OP-101", 300, 2.0, 50.0, 90.0, 2.0, "Dr. Nair", "note"); err == nil {
		t.Errorf("expected error for PT out of range")
	}
	if _, err := svc.RecordMetabolicCheck("HEP-OP-101", 300, 2.0, 50.0, 90.0, 12.0, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordMetabolicCheck("NON-EXISTENT", 300, 2.0, 50.0, 90.0, 12.0, "Dr. Nair", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordMetabolicCheck")
	}

	// ClearHepaticDischarge validation errors
	if _, err := svc.ClearHepaticDischarge("HEP-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearHepaticDischarge("HEP-OP-101", "Dr. Nair", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearHepaticDischarge("NON-EXISTENT", "Dr. Nair", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearHepaticDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("HEP-OP-101")
	badOps := []func(op *domain.HepaticOperation){
		func(op *domain.HepaticOperation) { op.ID = "" },
		func(op *domain.HepaticOperation) { op.CaseID = "" },
		func(op *domain.HepaticOperation) { op.PetName = "" },
		func(op *domain.HepaticOperation) { op.WeightKg = 0 },
		func(op *domain.HepaticOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.HepaticOperation) { op.EtiologyType = "BAD_ETIOLOGY" },
		func(op *domain.HepaticOperation) { op.EncephalopathyGrade = -1 },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildEncephalopathyPlan default switch branch coverage
	planDefault := hepaticencephalopathy.BuildEncephalopathyPlan("CUSTOM_ETIOLOGY", 500, 2.5, 45.0, 95.0, 12.0, 0, false)
	if !strings.Contains(planDefault.EtiologyPathophysiology, "Fulminant Hepatotoxicity") {
		t.Errorf("expected default etiology summary, got %q", planDefault.EtiologyPathophysiology)
	}
}
