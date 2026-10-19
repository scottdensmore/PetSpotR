package masscasualtyvmat

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDomainMassCasualtyVMATValidationAndEnums(t *testing.T) {
	validStatuses := []domain.MassCasualtyVMATStatus{
		domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged,
		domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored,
		domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared,
	}
	for _, st := range validStatuses {
		if !st.IsValid() {
			t.Fatalf("expected status %q to be valid", st)
		}
	}
	if domain.MassCasualtyVMATStatus("INVALID_STATUS").IsValid() {
		t.Fatal("expected invalid status to return false")
	}

	validTags := []domain.VeterinarySTARTTag{
		domain.STARTTagRedImmediatePriority1LifeThreat,
		domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		domain.STARTTagGreenMinimalPriority3WalkingWounded,
		domain.STARTTagBlackExpectantAnalgesiaPalliative,
	}
	for _, tg := range validTags {
		if !tg.IsValid() {
			t.Fatalf("expected tag %q to be valid", tg)
		}
	}
	if domain.VeterinarySTARTTag("INVALID_TAG").IsValid() {
		t.Fatal("expected invalid tag to return false")
	}

	validProtocols := []domain.VMATSurgeProtocol{
		domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		domain.VMATProtocolDamageControlResuscitationTourniquetO2,
		domain.VMATProtocolICS214MicrochipBandCohortIsolation,
		domain.VMATProtocolRegionalTertiaryVetMedevacTransfer,
	}
	for _, pr := range validProtocols {
		if !pr.IsValid() {
			t.Fatalf("expected protocol %q to be valid", pr)
		}
	}
	if domain.VMATSurgeProtocol("INVALID_PROTOCOL").IsValid() {
		t.Fatal("expected invalid protocol to return false")
	}

	baseOp := domain.MassCasualtyVMATOperation{
		ID:                   "MCI-OP-900",
		CaseID:               "CASE-MCI-900",
		PetName:              "Kodiak",
		WeightKg:             31.5,
		Status:               domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged,
		TriageTag:            domain.STARTTagRedImmediatePriority1LifeThreat,
		Protocol:             domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		AnimalTriageScoreATS: 17,
	}
	if err := baseOp.Validate(); err != nil {
		t.Fatalf("expected valid operation, got %v", err)
	}

	invalidCases := []struct {
		name   string
		mutate func(op *domain.MassCasualtyVMATOperation)
	}{
		{"empty ID", func(op *domain.MassCasualtyVMATOperation) { op.ID = " " }},
		{"empty CaseID", func(op *domain.MassCasualtyVMATOperation) { op.CaseID = "" }},
		{"empty PetName", func(op *domain.MassCasualtyVMATOperation) { op.PetName = "" }},
		{"non-positive weight", func(op *domain.MassCasualtyVMATOperation) { op.WeightKg = 0 }},
		{"invalid status", func(op *domain.MassCasualtyVMATOperation) { op.Status = "BAD" }},
		{"invalid triage_tag", func(op *domain.MassCasualtyVMATOperation) { op.TriageTag = "BAD" }},
		{"invalid protocol", func(op *domain.MassCasualtyVMATOperation) { op.Protocol = "BAD" }},
		{"negative ATS", func(op *domain.MassCasualtyVMATOperation) { op.AnimalTriageScoreATS = -1 }},
		{"ATS > 25", func(op *domain.MassCasualtyVMATOperation) { op.AnimalTriageScoreATS = 26 }},
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

func TestNewMassCasualtyVMATServiceSeededOperationsAndOverview(t *testing.T) {
	svc := NewMassCasualtyVMATService()
	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("MCI-OP-101")
	if err != nil {
		t.Fatalf("expected MCI-OP-101, got %v", err)
	}
	if op101.Status != domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged {
		t.Fatalf("unexpected MCI-OP-101 status: %s", op101.Status)
	}
	if !op101.GrossDecontaminationPriorToColdZoneVerified || !op101.ReverseTriageSurgeTriggered || !op101.ContagiousCohortSegregated {
		t.Fatal("expected safety and surge flags on MCI-OP-101")
	}

	op102, err := svc.GetOperation("MCI-OP-102")
	if err != nil {
		t.Fatalf("expected MCI-OP-102, got %v", err)
	}
	if op102.Status != domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored {
		t.Fatalf("unexpected MCI-OP-102 status: %s", op102.Status)
	}

	op103, err := svc.GetOperation("MCI-OP-103")
	if err != nil {
		t.Fatalf("expected MCI-OP-103, got %v", err)
	}
	if op103.Status != domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared {
		t.Fatalf("unexpected MCI-OP-103 status: %s", op103.Status)
	}

	if _, err := svc.GetOperation("MCI-OP-999"); err == nil {
		t.Fatal("expected error for unknown operation")
	}

	filtered := svc.ListOperations(string(domain.MassCasualtyVMATStatusStartSaltTaggedDeconStaged))
	if len(filtered) != 1 || filtered[0].ID != "MCI-OP-101" {
		t.Fatalf("expected 1 filtered operation MCI-OP-101, got %+v", filtered)
	}

	overview := svc.GetOverviewMetrics()
	if overview.TotalOperations != 3 ||
		overview.StartSaltTaggedDeconStagedCount != 1 ||
		overview.DamageControlSurgeMonitoredCount != 1 ||
		overview.ReunifiedEvacClearedCount != 1 ||
		overview.RedImmediatePriority1Count != 1 ||
		overview.ReverseTriageSurgeTriggeredCount != 2 ||
		overview.ColdZoneDeconVerifiedCount != 3 {
		t.Fatalf("unexpected overview metrics: %+v", overview)
	}
}

func TestNarrativesAndDefaultStagingBranches(t *testing.T) {
	if !IsReverseTriageSurgeTriggered(85.0) {
		t.Fatal("expected reverse triage surge triggered at 85.0%")
	}
	if IsReverseTriageSurgeTriggered(84.9) {
		t.Fatal("expected reverse triage surge false below 85.0%")
	}

	tags := []domain.VeterinarySTARTTag{
		domain.STARTTagRedImmediatePriority1LifeThreat,
		domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		domain.STARTTagGreenMinimalPriority3WalkingWounded,
		domain.STARTTagBlackExpectantAnalgesiaPalliative,
		domain.VeterinarySTARTTag("OTHER"),
	}
	for _, tg := range tags {
		s1, d1, c1 := BuildClinicalNarratives(tg, 14, 5.2, 89.0, 90.0, false, false, "RFID-1", "ICS-1")
		if s1 == "" || d1 == "" || c1 == "" {
			t.Fatalf("expected non-empty narratives for %s", tg)
		}
		s2, d2, c2 := BuildClinicalNarratives(tg, 4, 1.8, 97.0, 70.0, true, true, "RFID-2", "ICS-2")
		if s2 == "" || d2 == "" || c2 == "" {
			t.Fatalf("expected non-empty narratives for %s", tg)
		}
	}

	stgBlocked := BuildDefaultFieldTriageStaging(
		25.0,
		domain.STARTTagRedImmediatePriority1LifeThreat,
		domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		false,
		false,
		"",
		"",
		72.0,
		"",
		time.Now().UTC(),
	)
	if !strings.Contains(stgBlocked.ColdZoneCrossContaminationGuardNote, "CRITICAL HOT-TO-COLD ZONE CROSS-CONTAMINATION BLOCK") {
		t.Fatalf("expected cross-contamination block note, got %s", stgBlocked.ColdZoneCrossContaminationGuardNote)
	}
	if stgBlocked.ReverseTriageSurgeTriggered {
		t.Fatal("expected ReverseTriageSurgeTriggered false for 72.0% occupancy")
	}
}

func TestCreateOperationAndValidationGuards(t *testing.T) {
	svc := NewMassCasualtyVMATService()

	created, err := svc.CreateOperation(
		"CASE-MCI-104",
		"Cascade",
		"",
		"",
		27.5,
		domain.STARTTagRedImmediatePriority1LifeThreat,
		"",
		16,
		5.9,
		88.5,
		89.0,
		true,
		"",
		"",
		true,
	)
	if err != nil {
		t.Fatalf("expected CreateOperation to succeed, got %v", err)
	}
	if created.ID != "MCI-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Fatalf("unexpected defaults on created operation: %+v", created)
	}
	if !created.ReverseTriageSurgeTriggered || created.WaterproofRFIDTriageCollarNumber == "" || created.ICS214IncidentNumber == "" {
		t.Fatalf("expected reverse triage and default RFID/ICS numbers on created operation: %+v", created)
	}

	// Validation error branches
	if _, err := svc.CreateOperation("", "Pet", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 4.0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for empty case_id")
	}
	if _, err := svc.CreateOperation("CASE-1", "", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 4.0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for empty pet_name")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 0, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 4.0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid weight_kg")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, "BAD_TAG", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 4.0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid triage_tag")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, "BAD_PROTO", 10, 4.0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 26, 4.0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid animal_triage_score_ats")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 0, 92.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid lactate_mmol_l")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 4.0, 40.0, 80.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid spo2_percent")
	}
	if _, err := svc.CreateOperation("CASE-1", "Pet", "Canine", "Breed", 20, domain.STARTTagRedImmediatePriority1LifeThreat, domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, 10, 4.0, 92.0, 105.0, true, "R1", "I1", true); err == nil {
		t.Fatal("expected error for invalid surge_bed_occupancy_percent")
	}
}

func TestStageFieldTriageAndSafetyGuards(t *testing.T) {
	svc := NewMassCasualtyVMATService()

	// Safety Guard 1: Hot-to-Cold Zone Cross-Contamination Guard (grossDeconVerified=false must fail)
	_, err := svc.StageFieldTriage(
		"MCI-OP-101",
		domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		domain.STARTTagRedImmediatePriority1LifeThreat,
		false,
		"Warm-zone Dawn surfactant wash",
		"Damage-control resuscitation + TXA",
		"VMAT-RFID-9101",
		"ICS214-FEMA-2026-101",
		"",
		true,
		"",
		91.0,
		"",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "hot-to-cold zone cross-contamination guard") {
		t.Fatalf("expected hot-to-cold zone cross-contamination error, got %v", err)
	}

	// Safety Guard 2: Contagious Biosecurity Segregation Guard (contagiousCohortSegregated=false must fail)
	_, err = svc.StageFieldTriage(
		"MCI-OP-101",
		domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		domain.STARTTagRedImmediatePriority1LifeThreat,
		true,
		"Warm-zone Dawn surfactant wash",
		"Damage-control resuscitation + TXA",
		"VMAT-RFID-9101",
		"ICS214-FEMA-2026-101",
		"",
		false,
		"",
		91.0,
		"",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "biosecurity isolation guard") {
		t.Fatalf("expected biosecurity isolation guard error, got %v", err)
	}

	// Valid staging on MCI-OP-101 with defaults
	staged, err := svc.StageFieldTriage(
		"MCI-OP-101",
		"",
		"",
		true,
		"Warm-zone tepid water + Dawn dish detergent dermal/paw flush and sterile 0.9% NaCl ocular irrigation",
		"Direct arterial pressure bandage + TXA 10 mg/kg IV + Hypertonic Saline 4 mL/kg IV + Methadone 0.2 mg/kg IV",
		"VMAT-RFID-9101",
		"ICS214-FEMA-2026-101",
		"",
		true,
		"",
		0,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("expected StageFieldTriage to succeed, got %v", err)
	}
	if staged.Status != domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored {
		t.Fatalf("expected status %s, got %s", domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored, staged.Status)
	}
	if !staged.GrossDecontaminationPriorToColdZoneVerified || !staged.ReverseTriageSurgeTriggered {
		t.Fatal("expected decon verified and reverse triage surge triggered on MCI-OP-101")
	}

	// Validation error branches on StageFieldTriage
	if _, err := svc.StageFieldTriage("MCI-OP-101", "BAD_PROTO", "", true, "Decon", "DCR", "RFID", "ICS", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if _, err := svc.StageFieldTriage("MCI-OP-101", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "BAD_TAG", true, "Decon", "DCR", "RFID", "ICS", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for invalid triage_tag")
	}
	if _, err := svc.StageFieldTriage("MCI-OP-101", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "", true, "", "DCR", "RFID", "ICS", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for empty warm_zone_decontamination_corridor_plan")
	}
	if _, err := svc.StageFieldTriage("MCI-OP-101", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "", true, "Decon", "", "RFID", "ICS", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for empty damage_control_resuscitation_plan")
	}
	if _, err := svc.StageFieldTriage("MCI-OP-101", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "", true, "Decon", "DCR", "", "ICS", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for empty waterproof_rfid_triage_collar_number")
	}
	if _, err := svc.StageFieldTriage("MCI-OP-101", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "", true, "Decon", "DCR", "RFID", "", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for empty ics214_incident_number")
	}
	if _, err := svc.StageFieldTriage("MCI-OP-101", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "", true, "Decon", "DCR", "RFID", "ICS", "", true, "", 105.0, "", "Dr"); err == nil {
		t.Fatal("expected error for invalid surge_bed_occupancy_percent")
	}
	if _, err := svc.StageFieldTriage("UNKNOWN-ID", domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor, "", true, "Decon", "DCR", "RFID", "ICS", "", true, "", 85.0, "", "Dr"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}
}

func TestRecordReassessmentCheckAndEvaluateClearance(t *testing.T) {
	svc := NewMassCasualtyVMATService()

	updated, err := svc.RecordReassessmentCheck(
		"MCI-OP-101",
		domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		6,
		1.85,
		97.5,
		78.0,
		true,
		true,
		"Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		"Post-resuscitation reassessment: arterial hemorrhage controlled, ATS improved from 17 to 6, lactate cleared to 1.85 mmol/L, SpO2 97.5%, surge bed occupancy down to 78.0%.",
	)
	if err != nil {
		t.Fatalf("expected RecordReassessmentCheck to succeed, got %v", err)
	}
	if updated.AnimalTriageScoreATS != 6 || updated.ReverseTriageSurgeTriggered {
		t.Fatalf("unexpected updated check fields: %+v", updated)
	}
	if updated.Status != domain.MassCasualtyVMATStatusDamageControlSurgeICUMonitored {
		t.Fatalf("expected status transition to DAMAGE_CONTROL_SURGE_ICU_MONITORED, got %s", updated.Status)
	}

	// Test default tag and occupancy fallback on RecordReassessmentCheck
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", 5, 1.8, 98.0, 0, true, true, "Dr. Vance", "Follow-up"); err != nil {
		t.Fatalf("expected RecordReassessmentCheck with empty tag/0 occupancy to succeed, got %v", err)
	}

	// Validation errors for RecordReassessmentCheck
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "BAD_TAG", 5, 1.8, 98.0, 75.0, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid triage_tag")
	}
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", -1, 1.8, 98.0, 75.0, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid ATS")
	}
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", 5, 0, 98.0, 75.0, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid lactate")
	}
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", 5, 1.8, 45.0, 75.0, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid SpO2")
	}
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", 5, 1.8, 98.0, 110.0, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for invalid surge_bed_occupancy_percent")
	}
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", 5, 1.8, 98.0, 75.0, false, true, "Dr", "note"); err == nil {
		t.Fatal("expected error when grossDeconVerified is false")
	}
	if _, err := svc.RecordReassessmentCheck("MCI-OP-101", "", 5, 1.8, 98.0, 75.0, true, true, "", "note"); err == nil {
		t.Fatal("expected error for empty recorded_by")
	}
	if _, err := svc.RecordReassessmentCheck("UNKNOWN-ID", "", 5, 1.8, 98.0, 75.0, true, true, "Dr", "note"); err == nil {
		t.Fatal("expected error for unknown operation ID")
	}

	// EvaluateClearance safety guards
	if _, err := svc.EvaluateClearance("MCI-OP-101", 3.5, 97.5, true, true, "Dr. Vance", "Summary"); err == nil {
		t.Fatal("expected clearance error when lactate_mmol_l > 2.5")
	}
	if _, err := svc.EvaluateClearance("MCI-OP-101", 1.8, 91.0, true, true, "Dr. Vance", "Summary"); err == nil {
		t.Fatal("expected clearance error when spo2_percent < 95.0")
	}
	if _, err := svc.EvaluateClearance("MCI-OP-101", 1.8, 97.5, false, true, "Dr. Vance", "Summary"); err == nil {
		t.Fatal("expected clearance error when waterproof_rfid_matched_verified is false")
	}
	if _, err := svc.EvaluateClearance("MCI-OP-101", 1.8, 97.5, true, false, "Dr. Vance", "Summary"); err == nil {
		t.Fatal("expected clearance error when ics214_chain_of_custody_signed is false")
	}
	if _, err := svc.EvaluateClearance("MCI-OP-101", 1.8, 97.5, true, true, "", "Summary"); err == nil {
		t.Fatal("expected clearance error when cleared_by is empty")
	}
	if _, err := svc.EvaluateClearance("MCI-OP-101", 1.8, 97.5, true, true, "Dr. Vance", ""); err == nil {
		t.Fatal("expected clearance error when clearance_summary is empty")
	}
	if _, err := svc.EvaluateClearance("UNKNOWN-ID", 1.8, 97.5, true, true, "Dr. Vance", "Summary"); err == nil {
		t.Fatal("expected clearance error for unknown operation ID")
	}

	// Test unverified decon guard on EvaluateClearance
	unverifiedOp, err := svc.CreateOperation(
		"CASE-UNDECON-1",
		"Ash",
		"Canine",
		"Shepherd",
		24.0,
		domain.STARTTagYellowDelayedPriority2UrgentSurgery,
		domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
		9,
		2.0,
		96.0,
		75.0,
		false,
		"VMAT-RFID-9999",
		"ICS214-FEMA-9999",
		true,
	)
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if _, err := svc.EvaluateClearance(unverifiedOp.ID, 1.9, 97.0, true, true, "Dr. Vance", "Summary"); err == nil {
		t.Fatal("expected clearance error when gross decontamination was not verified")
	}

	// Successful clearance on MCI-OP-101
	cleared, err := svc.EvaluateClearance(
		"MCI-OP-101",
		1.85,
		97.5,
		true,
		true,
		"Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		"Perfusion restored (lactate 1.85 mmol/L <= 2.5 mmol/L), SpO2 97.5% (>= 95.0%), warm-zone decon verified, RFID collar VMAT-RFID-9101 matched, and FEMA ICS-214 chain of custody signed; cleared.",
	)
	if err != nil {
		t.Fatalf("expected EvaluateClearance to succeed, got %v", err)
	}
	if cleared.Status != domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared {
		t.Fatalf("expected status %s, got %s", domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared, cleared.Status)
	}
	if cleared.Clearance == nil || cleared.DischargedAt == nil {
		t.Fatal("expected clearance and discharged_at to be populated")
	}
}

func TestConcurrentMassCasualtyVMATServiceOperations(t *testing.T) {
	svc := NewMassCasualtyVMATService()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			op, err := svc.CreateOperation(
				fmt.Sprintf("CASE-CONC-%d", idx),
				fmt.Sprintf("Responder-%d", idx),
				"Canine",
				"Malinois",
				28.0,
				domain.STARTTagRedImmediatePriority1LifeThreat,
				domain.VMATProtocolHotWarmColdZoneDawnDeconCorridor,
				15,
				5.4,
				89.0,
				88.0,
				true,
				fmt.Sprintf("VMAT-RFID-8%d", idx),
				fmt.Sprintf("ICS214-FEMA-8%d", idx),
				true,
			)
			if err != nil {
				t.Errorf("concurrent CreateOperation failed: %v", err)
				return
			}
			_, _ = svc.RecordReassessmentCheck(op.ID, domain.STARTTagYellowDelayedPriority2UrgentSurgery, 6, 1.9, 97.0, 79.0, true, true, "Dr. Concurrent", "Stabilized")
			_, _ = svc.EvaluateClearance(op.ID, 1.9, 97.0, true, true, "Dr. Concurrent", "Cleared")
			_ = svc.ListOperations("")
			_ = svc.CalculateOverview()
		}(i)
	}
	wg.Wait()
}
