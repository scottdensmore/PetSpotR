package megaesophagusfb_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/megaesophagusfb"
)

func TestMegaesophagusSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := megaesophagusfb.NewMegaesophagusService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded Megaesophagus operations, got %d", len(ops))
	}

	// Verify MEGA-OP-101 ("Waffles")
	op101, err := svc.GetOperation("MEGA-OP-101")
	if err != nil {
		t.Fatalf("expected MEGA-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Waffles" || op101.Species != "Canine" || op101.Breed != "West Highland White Terrier" || op101.WeightKg != 8.6 {
		t.Errorf("unexpected MEGA-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.MegaStatusAcuteObstructionAspirationTriage || op101.PathologyType != domain.PathologyAcuteBoneFishhookForeignBody {
		t.Errorf("unexpected MEGA-OP-101 status/pathology: %s / %s", op101.Status, op101.PathologyType)
	}
	if op101.SpO2Percent != 91.0 || op101.AlveolarArterialGradientMmHg != 34.0 || op101.RegurgitationEpisodes24h != 6 || op101.UprightHoldMinutes != 0 || op101.MucosalEsophagitisGrade != 3 {
		t.Errorf("unexpected MEGA-OP-101 metrics: %+v", op101)
	}
	if op101.AChRAntibodyTiterNmolL != 0.12 || !op101.AspirationPneumoniaPresent {
		t.Errorf("unexpected MEGA-OP-101 AChR/pneumonia: %+v", op101)
	}
	if len(op101.EsophagealChecks) != 2 {
		t.Errorf("expected 2 seeded EsophagealChecks on MEGA-OP-101, got %d", len(op101.EsophagealChecks))
	}
	if !strings.Contains(op101.AspirationPlan.BariumSwallowContraindication, "Barium") {
		t.Errorf("expected barium contraindication guard on MEGA-OP-101, got %q", op101.AspirationPlan.BariumSwallowContraindication)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected MEGA-OP-101 to validate, got %v", err)
	}

	// Verify MEGA-OP-102 ("Freya")
	op102, err := svc.GetOperation("MEGA-OP-102")
	if err != nil {
		t.Fatalf("expected MEGA-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Freya" || op102.Breed != "German Shepherd Dog" || op102.WeightKg != 29.4 {
		t.Errorf("unexpected MEGA-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.MegaStatusEndoscopicPyridostigmineStaged || op102.PathologyType != domain.PathologyAcquiredMyastheniaGravis {
		t.Errorf("unexpected MEGA-OP-102 status/pathology: %s / %s", op102.Status, op102.PathologyType)
	}
	if op102.SpO2Percent != 95.0 || op102.AlveolarArterialGradientMmHg != 18.0 || op102.RegurgitationEpisodes24h != 2 || op102.UprightHoldMinutes != 25 || op102.MucosalEsophagitisGrade != 1 {
		t.Errorf("unexpected MEGA-OP-102 metrics: %+v", op102)
	}
	if op102.AChRAntibodyTiterNmolL != 2.85 || !op102.AspirationPneumoniaPresent || len(op102.EsophagealChecks) != 1 {
		t.Errorf("unexpected MEGA-OP-102 AChR/pneumonia/checks: %+v", op102)
	}

	// Verify MEGA-OP-103 ("Pippin")
	op103, err := svc.GetOperation("MEGA-OP-103")
	if err != nil {
		t.Fatalf("expected MEGA-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Pippin" || op103.Breed != "Miniature Schnauzer" || op103.WeightKg != 6.8 {
		t.Errorf("unexpected MEGA-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.MegaStatusUprightBaileyChairCleared || op103.PathologyType != domain.PathologyCongenitalPRAAIdiopathic {
		t.Errorf("unexpected MEGA-OP-103 status/pathology: %s / %s", op103.Status, op103.PathologyType)
	}
	if op103.SpO2Percent != 99.0 || op103.AlveolarArterialGradientMmHg != 8.0 || op103.RegurgitationEpisodes24h != 0 || op103.UprightHoldMinutes != 30 || op103.MucosalEsophagitisGrade != 0 || op103.AChRAntibodyTiterNmolL != 0.08 {
		t.Errorf("unexpected MEGA-OP-103 metrics: %+v", op103)
	}
	if op103.AspirationPneumoniaPresent || op103.DischargedBy != "Dr. Clara Sterling, DACVIM (SAIM)" || op103.DischargedAt == nil || len(op103.EsophagealChecks) != 2 {
		t.Errorf("unexpected MEGA-OP-103 discharge state: %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.MegaStatusAcuteObstructionAspirationTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "MEGA-OP-101" {
		t.Errorf("expected 1 ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteObstructionTriageCount != 1 ||
		metrics.EndoscopyPyridostigmineStagedCount != 1 ||
		metrics.ClearedBaileyChairCount != 1 ||
		metrics.ForeignBodyObstructionCount != 1 ||
		metrics.AspirationPneumoniaActiveCount != 2 {
		t.Errorf("unexpected MegaesophagusOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown Megaesophagus operation ID")
	}
}

func TestMegaesophagusCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := megaesophagusfb.NewMegaesophagusService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-MEGA-104",
		"Paddington",
		"",
		"",
		14.2,
		domain.PathologyAcuteBoneFishhookForeignBody,
		92.0,
		28.0,
		5,
		0,
		2,
		0.15,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "MEGA-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.SpO2Percent != 92.0 || created.RegurgitationEpisodes24h != 5 {
		t.Errorf("expected SpO2=92.0 and RegurgEpisodes=5, got %.1f / %d", created.SpO2Percent, created.RegurgitationEpisodes24h)
	}

	// Stage endoscopy & Bailey Chair plan with empty optional fields (tests fallback defaults)
	staged, err := svc.StageEndoscopyBaileyPlan(
		created.ID,
		"Flexible esophagoscopy under cuffed endotracheal tube anesthesia; alligator forceps extraction of thoracic inlet bone foreign body",
		"Sucralfate 0.5 g PO slurry q8h + Omeprazole 1 mg/kg PO q12h",
		"",
		"",
		"Upright Bailey Chair 90-degree vertical feeding of Knox gelatin meatballs with 30-minute post-prandial hold",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageEndoscopyBaileyPlan error: %v", err)
	}
	if staged.Status != domain.MegaStatusEndoscopicPyridostigmineStaged {
		t.Errorf("expected status %s, got %s", domain.MegaStatusEndoscopicPyridostigmineStaged, staged.Status)
	}
	if staged.BaileyPlan.PyridostigmineAnticholinesterase == "" ||
		staged.BaileyPlan.AspirationPneumoniaAntibioticPlan == "" ||
		staged.BaileyPlan.SildenafilLESRelaxationNote == "" {
		t.Errorf("expected default pyridostigmine, antibiotic, and sildenafil notes to be populated: %+v", staged.BaileyPlan)
	}

	// Stage endoscopy & Bailey Chair plan with explicit optional fields
	stagedExplicit, err := svc.StageEndoscopyBaileyPlan(
		created.ID,
		"Endoscopic snare retrieval of bone foreign body",
		"Sucralfate 1 g PO q8h + Omeprazole 1 mg/kg IV q12h",
		"Pyridostigmine Bromide 1.5 mg/kg PO q8h with Atropine 0.02 mg/kg IV ready",
		"Ampicillin-Sulbactam 30 mg/kg IV q8h + Enrofloxacin 10 mg/kg IV q24h + saline nebulization",
		"Upright Bailey Chair hold 30 min post-prandially",
		"Sildenafil 1 mg/kg PO q8h for LES relaxation",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageEndoscopyBaileyPlan error: %v", err)
	}
	if stagedExplicit.BaileyPlan.SildenafilLESRelaxationNote != "Sildenafil 1 mg/kg PO q8h for LES relaxation" {
		t.Errorf("unexpected explicit sildenafil note: %q", stagedExplicit.BaileyPlan.SildenafilLESRelaxationNote)
	}

	// Record esophageal & aspiration check and verify operation metrics update
	checked, err := svc.RecordEsophagealCheck(
		created.ID,
		98.5,
		9.5,
		0,
		30,
		1,
		"Dr. Clara Sterling, DACVIM (SAIM)",
		"Post-endoscopic bone extraction, Sucralfate/Omeprazole & 30-min Upright Bailey Chair feeding: SpO2 98.5%, A-a gradient 9.5 mmHg, 0 regurgitation episodes over 24h.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordEsophagealCheck error: %v", err)
	}
	if len(checked.EsophagealChecks) != 1 {
		t.Fatalf("expected 1 EsophagealCheck on MEGA-OP-104, got %d", len(checked.EsophagealChecks))
	}
	if checked.SpO2Percent != 98.5 || checked.AlveolarArterialGradientMmHg != 9.5 || checked.RegurgitationEpisodes24h != 0 || checked.UprightHoldMinutes != 30 || checked.MucosalEsophagitisGrade != 1 {
		t.Errorf("expected operation vitals updated from EsophagealCheck, got %+v", checked)
	}

	// Clear Megaesophagus discharge
	cleared, err := svc.ClearMegaesophagusDischarge(
		created.ID,
		"Dr. Clara Sterling, DACVIM (SAIM)",
		"Esophageal foreign body retrieved, mucosal esophagitis healing (Grade 1), 0 regurgitation episodes over 24h with 30-min Upright Bailey Chair feeding, SpO2 98.5% on room air.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearMegaesophagusDischarge error: %v", err)
	}
	if cleared.Status != domain.MegaStatusUprightBaileyChairCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Clara Sterling, DACVIM (SAIM)" || cleared.AspirationPneumoniaPresent {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestMegaesophagusValidationErrors(t *testing.T) {
	svc := megaesophagusfb.NewMegaesophagusService()

	createCases := []struct {
		name             string
		caseID           string
		petName          string
		weightKg         float64
		pathology        domain.EsophagealPathologyType
		spO2             float64
		aaGradient       float64
		regurgEpisodes   int
		uprightHold      int
		esophagitisGrade int
		achrTiter        float64
	}{
		{"empty case_id", "", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 2, 0.1},
		{"empty pet_name", "CASE-1", "", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 2, 0.1},
		{"weight <= 0", "CASE-1", "Waffles", 0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 2, 0.1},
		{"weight > 120", "CASE-1", "Waffles", 130, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 2, 0.1},
		{"invalid pathology", "CASE-1", "Waffles", 10.0, "INVALID_PATHOLOGY", 92.0, 30.0, 4, 0, 2, 0.1},
		{"spo2 < 50", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 40.0, 30.0, 4, 0, 2, 0.1},
		{"spo2 > 100", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 105.0, 30.0, 4, 0, 2, 0.1},
		{"aaGradient < 0", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, -5.0, 4, 0, 2, 0.1},
		{"aaGradient > 80", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 90.0, 4, 0, 2, 0.1},
		{"regurgEpisodes < 0", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, -1, 0, 2, 0.1},
		{"regurgEpisodes > 50", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 60, 0, 2, 0.1},
		{"uprightHold < 0", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, -5, 2, 0.1},
		{"uprightHold > 120", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 150, 2, 0.1},
		{"esophagitisGrade < 0", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, -1, 0.1},
		{"esophagitisGrade > 4", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 5, 0.1},
		{"achrTiter < 0", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 2, -0.5},
		{"achrTiter > 50", "CASE-1", "Waffles", 10.0, domain.PathologyAcuteBoneFishhookForeignBody, 92.0, 30.0, 4, 0, 2, 60.0},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Westie",
				tc.weightKg, tc.pathology,
				tc.spO2, tc.aaGradient,
				tc.regurgEpisodes, tc.uprightHold, tc.esophagitisGrade,
				tc.achrTiter, true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageEndoscopyBaileyPlan validation errors
	if _, err := svc.StageEndoscopyBaileyPlan("MEGA-OP-101", "", "Sucralfate", "", "", "Bailey Chair", ""); err == nil {
		t.Errorf("expected error for empty retrievalPlan")
	}
	if _, err := svc.StageEndoscopyBaileyPlan("MEGA-OP-101", "Retrieval", "", "", "", "Bailey Chair", ""); err == nil {
		t.Errorf("expected error for empty sucralfateOmeprazole")
	}
	if _, err := svc.StageEndoscopyBaileyPlan("MEGA-OP-101", "Retrieval", "Sucralfate", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty baileyChairProtocol")
	}
	if _, err := svc.StageEndoscopyBaileyPlan("NON-EXISTENT", "Retrieval", "Sucralfate", "", "", "Bailey Chair", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageEndoscopyBaileyPlan")
	}

	// RecordEsophagealCheck validation errors
	if _, err := svc.RecordEsophagealCheck("MEGA-OP-101", 40.0, 15.0, 1, 25, 1, "Dr. Sterling", "note"); err == nil {
		t.Errorf("expected error for spo2 < 50")
	}
	if _, err := svc.RecordEsophagealCheck("MEGA-OP-101", 95.0, -2.0, 1, 25, 1, "Dr. Sterling", "note"); err == nil {
		t.Errorf("expected error for negative aaGradient")
	}
	if _, err := svc.RecordEsophagealCheck("MEGA-OP-101", 95.0, 15.0, -1, 25, 1, "Dr. Sterling", "note"); err == nil {
		t.Errorf("expected error for negative regurgEpisodes24h")
	}
	if _, err := svc.RecordEsophagealCheck("MEGA-OP-101", 95.0, 15.0, 1, -5, 1, "Dr. Sterling", "note"); err == nil {
		t.Errorf("expected error for negative uprightHoldMinutes")
	}
	if _, err := svc.RecordEsophagealCheck("MEGA-OP-101", 95.0, 15.0, 1, 25, 6, "Dr. Sterling", "note"); err == nil {
		t.Errorf("expected error for invalid esophagitisGrade")
	}
	if _, err := svc.RecordEsophagealCheck("MEGA-OP-101", 95.0, 15.0, 1, 25, 1, "", "note"); err == nil {
		t.Errorf("expected error for empty recordedBy")
	}
	if _, err := svc.RecordEsophagealCheck("NON-EXISTENT", 95.0, 15.0, 1, 25, 1, "Dr. Sterling", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordEsophagealCheck")
	}

	// ClearMegaesophagusDischarge validation errors
	if _, err := svc.ClearMegaesophagusDischarge("MEGA-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearMegaesophagusDischarge("MEGA-OP-101", "Dr. Sterling", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearMegaesophagusDischarge("NON-EXISTENT", "Dr. Sterling", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearMegaesophagusDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("MEGA-OP-101")
	badOps := []func(op *domain.MegaesophagusOperation){
		func(op *domain.MegaesophagusOperation) { op.ID = "" },
		func(op *domain.MegaesophagusOperation) { op.CaseID = "" },
		func(op *domain.MegaesophagusOperation) { op.PetName = "" },
		func(op *domain.MegaesophagusOperation) { op.WeightKg = 0 },
		func(op *domain.MegaesophagusOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.MegaesophagusOperation) { op.PathologyType = "BAD_PATHOLOGY" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildRegurgitationAspirationPlan default switch branch coverage
	planDefault := megaesophagusfb.BuildRegurgitationAspirationPlan("CUSTOM_PATHOLOGY", 96.0, 12.0, 1, 1, 0.2, false)
	if !strings.Contains(planDefault.RegurgitationVsVomitingNote, "Esophageal Dysmotility") {
		t.Errorf("expected default regurgitation note, got %q", planDefault.RegurgitationVsVomitingNote)
	}
}
