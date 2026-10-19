package rabiesquarantine

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDomainRabiesValidationAndEnums(t *testing.T) {
	validStatuses := []domain.RabiesQuarantineStatus{
		domain.RabiesStatusBiosecurityIsolationRCOHoldStaged,
		domain.RabiesStatusSerialNeuroSalivarySheddingMonitored,
		domain.RabiesStatusStatutoryQuarantineSurvivedCleared,
	}
	for _, st := range validStatuses {
		if !st.IsValid() {
			t.Fatalf("expected status %q to be valid", st)
		}
	}
	if domain.RabiesQuarantineStatus("INVALID_STATUS").IsValid() {
		t.Fatal("expected invalid status to return false")
	}

	validCategories := []domain.RabiesStatutoryCategory{
		domain.RabiesCategoryTenDayDomesticBiteConfinement,
		domain.RabiesCategoryFortyFiveDayVaccinatedObservation,
		domain.RabiesCategoryFourMonthStrictDoubleBarrier,
		domain.RabiesCategorySixMonthOverdueSerologyQuarantine,
	}
	for _, cat := range validCategories {
		if !cat.IsValid() {
			t.Fatalf("expected category %q to be valid", cat)
		}
	}
	if domain.RabiesStatutoryCategory("INVALID_CATEGORY").IsValid() {
		t.Fatal("expected invalid category to return false")
	}

	validProtocols := []domain.RabiesBiosecurityProtocol{
		domain.RabiesProtocolImmediateIMBooster96HrWoundLavage,
		domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
		domain.RabiesProtocolDoubleBarrierLockedCagePPEHandling,
		domain.RabiesProtocolProspectiveSerologicRFFITBooster,
	}
	for _, pr := range validProtocols {
		if !pr.IsValid() {
			t.Fatalf("expected protocol %q to be valid", pr)
		}
	}
	if domain.RabiesBiosecurityProtocol("INVALID_PROTOCOL").IsValid() {
		t.Fatal("expected invalid protocol to return false")
	}

	baseOp := domain.RabiesQuarantineOperation{
		ID:                  "RBQ-OP-900",
		CaseID:              "CASE-RBQ-900",
		PetName:             "Koda",
		WeightKg:            34.0,
		Status:              domain.RabiesStatusBiosecurityIsolationRCOHoldStaged,
		StatutoryCategory:   domain.RabiesCategoryTenDayDomesticBiteConfinement,
		Protocol:            domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
		RCOCaseFilingNumber: "RCO-STAT-2026-900",
	}
	if err := baseOp.Validate(); err != nil {
		t.Fatalf("expected valid operation, got %v", err)
	}

	invalidCases := []struct {
		name   string
		mutate func(op *domain.RabiesQuarantineOperation)
	}{
		{"empty ID", func(op *domain.RabiesQuarantineOperation) { op.ID = " " }},
		{"empty CaseID", func(op *domain.RabiesQuarantineOperation) { op.CaseID = "" }},
		{"empty PetName", func(op *domain.RabiesQuarantineOperation) { op.PetName = "" }},
		{"non-positive weight", func(op *domain.RabiesQuarantineOperation) { op.WeightKg = 0 }},
		{"invalid status", func(op *domain.RabiesQuarantineOperation) { op.Status = "BAD" }},
		{"invalid statutory_category", func(op *domain.RabiesQuarantineOperation) { op.StatutoryCategory = "BAD" }},
		{"invalid protocol", func(op *domain.RabiesQuarantineOperation) { op.Protocol = "BAD" }},
		{"empty rco_case_filing_number", func(op *domain.RabiesQuarantineOperation) { op.RCOCaseFilingNumber = "" }},
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

func TestNewRabiesQuarantineServiceSeededOperationsAndOverview(t *testing.T) {
	svc := NewRabiesQuarantineService()
	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("RBQ-OP-101")
	if err != nil {
		t.Fatalf("expected RBQ-OP-101, got %v", err)
	}
	if op101.Status != domain.RabiesStatusBiosecurityIsolationRCOHoldStaged {
		t.Fatalf("unexpected RBQ-OP-101 status: %s", op101.Status)
	}
	if !op101.NoVaccineDuring10DayBiteHoldVerified || !op101.DoubleBarrierLockedEnclosureVerified || !op101.BrainStemHippocampusPreservedGuard {
		t.Fatal("expected biosecurity and 10-day bite hold safety flags on RBQ-OP-101")
	}

	op102, err := svc.GetOperation("RBQ-OP-102")
	if err != nil {
		t.Fatalf("expected RBQ-OP-102, got %v", err)
	}
	if op102.Status != domain.RabiesStatusSerialNeuroSalivarySheddingMonitored {
		t.Fatalf("unexpected RBQ-OP-102 status: %s", op102.Status)
	}

	op103, err := svc.GetOperation("RBQ-OP-103")
	if err != nil {
		t.Fatalf("expected RBQ-OP-103, got %v", err)
	}
	if op103.Status != domain.RabiesStatusStatutoryQuarantineSurvivedCleared {
		t.Fatalf("unexpected RBQ-OP-103 status: %s", op103.Status)
	}

	if _, err := svc.GetOperation("RBQ-OP-999"); err == nil {
		t.Fatal("expected error for unknown operation")
	}

	filtered := svc.ListOperations(string(domain.RabiesStatusBiosecurityIsolationRCOHoldStaged))
	if len(filtered) != 1 || filtered[0].ID != "RBQ-OP-101" {
		t.Fatalf("expected 1 filtered operation RBQ-OP-101, got %+v", filtered)
	}

	overview := svc.GetOverviewMetrics()
	if overview.TotalOperations != 3 ||
		overview.BiosecurityIsolationRCOHoldStagedCount != 1 ||
		overview.SerialNeuroSalivaryMonitoredCount != 1 ||
		overview.StatutoryQuarantineSurvivedClearedCount != 1 ||
		overview.TenDayDomesticBiteHoldCount != 1 ||
		overview.FourMonthStrictUnvaccinatedCount != 1 ||
		overview.ProtectiveRFFITTiterCount != 2 {
		t.Fatalf("unexpected overview metrics: %+v", overview)
	}
}

func TestDetermineRequiredQuarantineDaysAndNarratives(t *testing.T) {
	if got := DetermineRequiredQuarantineDays(domain.RabiesCategoryTenDayDomesticBiteConfinement); got != 10 {
		t.Fatalf("expected 10 days, got %d", got)
	}
	if got := DetermineRequiredQuarantineDays(domain.RabiesCategoryFortyFiveDayVaccinatedObservation); got != 45 {
		t.Fatalf("expected 45 days, got %d", got)
	}
	if got := DetermineRequiredQuarantineDays(domain.RabiesCategoryFourMonthStrictDoubleBarrier); got != 120 {
		t.Fatalf("expected 120 days, got %d", got)
	}
	if got := DetermineRequiredQuarantineDays(domain.RabiesCategorySixMonthOverdueSerologyQuarantine); got != 180 {
		t.Fatalf("expected 180 days, got %d", got)
	}
	if got := DetermineRequiredQuarantineDays(domain.RabiesStatutoryCategory("OTHER")); got != 10 {
		t.Fatalf("expected default 10 days, got %d", got)
	}

	for _, validDays := range []int{10, 45, 120, 180} {
		if !IsValidStatutoryQuarantineDays(validDays) {
			t.Fatalf("expected %d to be valid statutory days", validDays)
		}
	}
	if IsValidStatutoryQuarantineDays(30) {
		t.Fatal("expected 30 to be invalid statutory days")
	}

	categories := []domain.RabiesStatutoryCategory{
		domain.RabiesCategoryTenDayDomesticBiteConfinement,
		domain.RabiesCategoryFortyFiveDayVaccinatedObservation,
		domain.RabiesCategoryFourMonthStrictDoubleBarrier,
		domain.RabiesCategorySixMonthOverdueSerologyQuarantine,
		domain.RabiesStatutoryCategory("OTHER"),
	}
	for _, cat := range categories {
		p1, r1, n1 := BuildClinicalNarratives(cat, "RCO-1", 5, 10, 0.75, true, true, true, true)
		p2, r2, n2 := BuildClinicalNarratives(cat, "RCO-1", 5, 10, 0.20, false, false, false, false)
		if p1 == "" || r1 == "" || n1 == "" || p2 == "" || r2 == "" || n2 == "" {
			t.Fatalf("expected non-empty narratives for %s", cat)
		}
	}

	stageDefaults := BuildDefaultQuarantineStaging(
		20.0,
		domain.RabiesCategoryFourMonthStrictDoubleBarrier,
		domain.RabiesProtocolDoubleBarrierLockedCagePPEHandling,
		"",
		0,
		0.65,
		true,
		true,
		"",
		time.Now().UTC(),
	)
	if stageDefaults.RequiredQuarantineDays != 120 || stageDefaults.RCOCaseFilingNumber == "" || stageDefaults.StagedBy == "" {
		t.Fatalf("unexpected stageDefaults: %+v", stageDefaults)
	}
}

func TestCreateOperationAndValidationGuards(t *testing.T) {
	svc := NewRabiesQuarantineService()

	created, err := svc.CreateOperation(
		"CASE-RBQ-104",
		"Fenrir",
		"",
		"",
		28.0,
		domain.RabiesCategoryTenDayDomesticBiteConfinement,
		"",
		"RCO-STAT-2026-104",
		1,
		0,
		0.35,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("expected CreateOperation to succeed, got %v", err)
	}
	if created.ID != "RBQ-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" || created.RequiredQuarantineDays != 10 {
		t.Fatalf("unexpected defaults on created operation: %+v", created)
	}

	// Test validation error branches
	if _, err := svc.CreateOperation("", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for empty case_id")
	}
	if _, err := svc.CreateOperation("CASE-1", "", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for empty pet_name")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 0, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for invalid weight_kg")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, "BAD_CAT", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for invalid statutory_category")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, "BAD_PROTO", "RCO-1", 1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "", 1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for empty rco_case_filing_number")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 30, 0.5, true, true); err == nil {
		t.Fatal("expected error for invalid required_quarantine_days")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", -1, 10, 0.5, true, true); err == nil {
		t.Fatal("expected error for negative quarantine_day_elapsed")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, -0.5, true, true); err == nil {
		t.Fatal("expected error for negative rffit_titer_iu_ml")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, 0.5, false, true); err == nil {
		t.Fatal("expected error for NASPHV 10-day bite hold vaccine violation")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.RabiesCategoryTenDayDomesticBiteConfinement, domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 1, 10, 0.5, true, false); err == nil {
		t.Fatal("expected error when double_barrier_locked_enclosure_verified is false")
	}
}

func TestStageQuarantineAndNASPHVSafetyGuards(t *testing.T) {
	svc := NewRabiesQuarantineService()

	// Safety Guard: Attempting to violate NASPHV 10-day bite hold vaccine prohibition on RBQ-OP-101 must fail
	_, err := svc.StageQuarantine(
		"RBQ-OP-101",
		domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
		"RCO-STAT-2026-101",
		10,
		false,
		true,
		"Padlocked double-barrier isolation run + PPE",
		"15-minute virucidal povidone-iodine lavage",
		"",
		true,
		"",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "NASPHV Compendium 10-Day Bite Hold Violation") {
		t.Fatalf("expected NASPHV 10-day bite hold vaccine error, got %v", err)
	}

	// Valid staging on RBQ-OP-101 with defaults
	staged, err := svc.StageQuarantine(
		"RBQ-OP-101",
		"",
		"RCO-STAT-2026-101",
		0,
		true,
		true,
		"Padlocked double-barrier isolation run + Kevlar gauntlets + face shield + catch-pole",
		"15-minute virucidal 1% povidone-iodine wound lavage; withhold canine rabies vaccine until Day 10 release",
		"",
		true,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("expected StageQuarantine to succeed, got %v", err)
	}
	if staged.Status != domain.RabiesStatusSerialNeuroSalivarySheddingMonitored {
		t.Fatalf("expected status %s, got %s", domain.RabiesStatusSerialNeuroSalivarySheddingMonitored, staged.Status)
	}

	// Validation error branches on StageQuarantine
	if _, err := svc.StageQuarantine("RBQ-OP-101", "BAD_PROTO", "RCO-1", 10, true, true, "PPE", "Lavage", "", true, "", ""); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.StageQuarantine("RBQ-OP-101", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "", 10, true, true, "PPE", "Lavage", "", true, "", ""); err == nil {
		t.Fatal("expected error for empty rco_case_filing_number")
	}
	if _, err := svc.StageQuarantine("RBQ-OP-101", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 10, true, false, "PPE", "Lavage", "", true, "", ""); err == nil {
		t.Fatal("expected error when double_barrier_locked_enclosure_verified is false")
	}
	if _, err := svc.StageQuarantine("RBQ-OP-101", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 10, true, true, "PPE", "Lavage", "", false, "", ""); err == nil {
		t.Fatal("expected error when brain_stem_hippocampus_preserved_guard is false")
	}
	if _, err := svc.StageQuarantine("RBQ-OP-101", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 10, true, true, "", "Lavage", "", true, "", ""); err == nil {
		t.Fatal("expected error for empty double_barrier_biosecurity_ppe_plan")
	}
	if _, err := svc.StageQuarantine("RBQ-OP-101", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 10, true, true, "PPE", "", "", true, "", ""); err == nil {
		t.Fatal("expected error for empty virucidal_wound_lavage_booster_pep_plan")
	}
	if _, err := svc.StageQuarantine("RBQ-OP-101", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 30, true, true, "PPE", "Lavage", "", true, "", ""); err == nil {
		t.Fatal("expected error for invalid required_quarantine_days")
	}
	if _, err := svc.StageQuarantine("UNKNOWN-ID", domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold, "RCO-1", 10, true, true, "PPE", "Lavage", "", true, "", ""); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}
}

func TestRecordNeuroObservationCheckAndEvaluateClearance(t *testing.T) {
	svc := NewRabiesQuarantineService()

	updated, err := svc.RecordNeuroObservationCheck(
		"RBQ-OP-101",
		10,
		0.65,
		true,
		true,
		true,
		true,
		"Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		"Day 10 of 10 statutory human-bite confinement: zero cranial nerve or bulbar palsy, eating and drinking normally, no hypersalivation.",
	)
	if err != nil {
		t.Fatalf("expected RecordNeuroObservationCheck to succeed, got %v", err)
	}
	if updated.QuarantineDayElapsed != 10 || !updated.SeroprotectiveTiterVerified {
		t.Fatalf("unexpected updated fields: %+v", updated)
	}
	if updated.Status != domain.RabiesStatusSerialNeuroSalivarySheddingMonitored {
		t.Fatalf("expected status transition to SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED, got %s", updated.Status)
	}

	// Validation errors for RecordNeuroObservationCheck
	if _, err := svc.RecordNeuroObservationCheck("RBQ-OP-101", -1, 0.5, true, true, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid quarantine_day_elapsed")
	}
	if _, err := svc.RecordNeuroObservationCheck("RBQ-OP-101", 5, -0.5, true, true, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid rffit_titer_iu_ml")
	}
	if _, err := svc.RecordNeuroObservationCheck("RBQ-OP-101", 5, 0.5, true, true, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error when double_barrier_locked_enclosure_verified is false")
	}
	if _, err := svc.RecordNeuroObservationCheck("RBQ-OP-101", 5, 0.5, true, true, true, false, "Dr", "note"); err == nil {
		t.Fatal("expected error when 10-day bite hold vaccine guard is violated before Day 10")
	}
	if _, err := svc.RecordNeuroObservationCheck("RBQ-OP-101", 5, 0.5, true, true, true, true, "", "note"); err == nil {
		t.Fatal("expected error for empty recorded_by")
	}
	if _, err := svc.RecordNeuroObservationCheck("UNKNOWN-ID", 5, 0.5, true, true, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}

	// EvaluateClearance safety guards
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 10, false, true, true, 1.2, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when cranial_nerve_bulbar_palsy_absent is false")
	}
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 10, true, false, true, 1.2, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when rco_written_release_signed is false")
	}
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 10, true, true, false, 1.2, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when exit_rabies_vaccination_verified is false")
	}
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 10, true, true, true, -1.0, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when rffit_titer_iu_ml < 0")
	}
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 10, true, true, true, 1.2, "", "Summary"); err == nil {
		t.Fatal("expected clearance error when cleared_by is empty")
	}
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 10, true, true, true, 1.2, "Dr. Thibodeaux", ""); err == nil {
		t.Fatal("expected clearance error when clearance_summary is empty")
	}
	if _, err := svc.EvaluateClearance("RBQ-OP-101", 5, true, true, true, 1.2, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error when quarantine_day_elapsed < required_quarantine_days")
	}
	if _, err := svc.EvaluateClearance("UNKNOWN-ID", 10, true, true, true, 1.2, "Dr. Thibodeaux", "Summary"); err == nil {
		t.Fatal("expected clearance error for unknown operation ID")
	}

	// Successful clearance
	cleared, err := svc.EvaluateClearance(
		"RBQ-OP-101",
		10,
		true,
		true,
		true,
		1.45,
		"Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		"Completed full 10-day NASPHV domestic bite observation (Day 10/10), cranial nerve/bulbar palsy absent, RCO statutory release signed, and exit rabies vaccination administered on Day 10; cleared.",
	)
	if err != nil {
		t.Fatalf("expected EvaluateClearance to succeed, got %v", err)
	}
	if cleared.Status != domain.RabiesStatusStatutoryQuarantineSurvivedCleared {
		t.Fatalf("expected status %s, got %s", domain.RabiesStatusStatutoryQuarantineSurvivedCleared, cleared.Status)
	}
	if cleared.Clearance == nil || cleared.DischargedAt == nil {
		t.Fatal("expected clearance and discharged_at to be populated")
	}
}

func TestConcurrentRabiesQuarantineServiceOperations(t *testing.T) {
	svc := NewRabiesQuarantineService()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			op, err := svc.CreateOperation(
				fmt.Sprintf("CASE-CONC-%d", idx),
				fmt.Sprintf("Koda-%d", idx),
				"Canine",
				"Malamute",
				30.0,
				domain.RabiesCategoryTenDayDomesticBiteConfinement,
				domain.RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
				fmt.Sprintf("RCO-CONC-%d", idx),
				2,
				10,
				0.60,
				true,
				true,
			)
			if err != nil {
				t.Errorf("concurrent CreateOperation failed: %v", err)
				return
			}
			_, _ = svc.RecordNeuroObservationCheck(op.ID, 10, 1.10, true, true, true, true, "Dr. Concurrent", "Day 10 check")
			_, _ = svc.EvaluateClearance(op.ID, 10, true, true, true, 1.10, "Dr. Concurrent", "Cleared")
			_ = svc.ListOperations("")
			_ = svc.CalculateOverview()
		}(i)
	}
	wg.Wait()
}
