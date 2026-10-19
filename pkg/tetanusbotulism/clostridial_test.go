package tetanusbotulism_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/tetanusbotulism"
)

func TestClostridialSeededOperationsAndOverview(t *testing.T) {
	svc := tetanusbotulism.NewTetanusBotulismService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded Tetanus/Botulism operations, got %d", len(ops))
	}

	// Verify CLB-OP-101 ("Rex")
	op101, err := svc.GetOperation("CLB-OP-101")
	if err != nil {
		t.Fatalf("expected CLB-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Rex" || op101.Species != "Canine" || op101.Breed != "German Shepherd" || op101.WeightKg != 32.0 {
		t.Errorf("unexpected CLB-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged ||
		op101.Syndrome != domain.SyndromeTetanusClassIIIRecumbentOpisthotonos ||
		op101.Protocol != domain.ProtocolEquineTAT100To1000IUKgAfterIDTest {
		t.Errorf("unexpected CLB-OP-101 status/syndrome/protocol: %s / %s / %s", op101.Status, op101.Syndrome, op101.Protocol)
	}
	if op101.BurkittSeverityClass != 3 || op101.GagReflexIntact || op101.SpasmFrequencyPerHr != 6 || op101.CoreTempCelsius != 39.8 || op101.PaCO2mmHg != 49.0 {
		t.Errorf("unexpected CLB-OP-101 vitals: %+v", op101)
	}
	if op101.MetronidazoleMgKgBID != 10.0 || op101.EquineTATUnitsKg != 500.0 || op101.MethocarbamolMgKgIV != 55.0 {
		t.Errorf("unexpected CLB-OP-101 dosing: %+v", op101)
	}
	if !op101.IntradermalSkinTestNegativeVerified || !op101.PenicillinGABAWarningAcknowledged || !op101.DarkQuietCottonEarplugsActive || !op101.AminoglycosideContraindicatedInBotulism {
		t.Errorf("expected all safety guards verified on CLB-OP-101, got %+v", op101)
	}
	if len(op101.TherapyStagings) != 1 || len(op101.NeuroChecks) != 1 {
		t.Errorf("expected 1 staging and 1 neuro check on CLB-OP-101, got %d / %d", len(op101.TherapyStagings), len(op101.NeuroChecks))
	}
	if !strings.Contains(op101.PenicillinGABAAntagonismAlert, "GABA-A") || !strings.Contains(op101.EquineTATIntradermalTestGuard, "0.1–0.2 mL") {
		t.Errorf("expected GABA-A and intradermal test guard text on CLB-OP-101")
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected CLB-OP-101 to validate, got %v", err)
	}

	// Verify CLB-OP-102 ("Otis")
	op102, err := svc.GetOperation("CLB-OP-102")
	if err != nil {
		t.Fatalf("expected CLB-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Otis" || op102.Status != domain.ClostridialStatusSensoryIsolationVentilationMonitored ||
		op102.Syndrome != domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia {
		t.Errorf("unexpected CLB-OP-102 state: %+v", op102)
	}

	// Verify CLB-OP-103 ("Bella")
	op103, err := svc.GetOperation("CLB-OP-103")
	if err != nil {
		t.Fatalf("expected CLB-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Bella" || op103.Status != domain.ClostridialStatusNormoreflexicSwallowingIntactCleared ||
		op103.Clearance == nil || op103.ClearedAt == nil || !op103.GagReflexIntact {
		t.Errorf("unexpected CLB-OP-103 cleared state: %+v", op103)
	}

	// Filter by status
	stagedOps := svc.ListOperations(string(domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged))
	if len(stagedOps) != 1 || stagedOps[0].ID != "CLB-OP-101" {
		t.Errorf("expected 1 METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED operation, got %+v", stagedOps)
	}

	overview := svc.CalculateOverview()
	metrics := svc.GetOverviewMetrics()
	if overview != metrics {
		t.Errorf("expected CalculateOverview and GetOverviewMetrics to match")
	}
	if overview.TotalOperations != 3 ||
		overview.MetronidazoleTATStagedCount != 1 ||
		overview.SensoryIsolationMonitoredCount != 1 ||
		overview.NormoreflexicClearedCount != 1 ||
		overview.SevereBurkittClassIIIIVCount != 2 ||
		overview.BotulismLMNParalysisCount != 1 {
		t.Errorf("unexpected TetanusBotulismOverview: %+v", overview)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestClostridialCreateStageNeuroCheckAndClearLifecycle(t *testing.T) {
	svc := tetanusbotulism.NewTetanusBotulismService()

	// Create operation with default species/breed/protocol/HR/metronidazole fallbacks and TAT cap > 20,000 IU
	created, err := svc.CreateOperation(
		"CASE-CLB-104",
		"Apollo",
		"",
		"",
		30.0,
		domain.SyndromeTetanusClassIIGeneralizedSawhorse,
		"",
		2,
		false,
		4,
		39.5,
		46.0,
		0,
		0,
		800.0, // 30 kg * 800 = 24,000 IU -> capped at 20,000 IU
		50.0,
		true,
		true,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "CLB-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.Protocol != domain.ProtocolMetronidazole10MgKgBIDDebridement || created.MetronidazoleMgKgBID != 10.0 || created.HeartRateBPM != 110 {
		t.Errorf("expected fallback defaults on created operation, got %+v", created)
	}
	if !strings.Contains(created.EquineTATIntradermalTestGuard, "Capped at 20000 IU") {
		t.Errorf("expected 20,000 IU cap in TAT guard note, got %q", created.EquineTATIntradermalTestGuard)
	}

	// Stage therapy with empty protocol and empty note (tests fallbacks)
	staged, err := svc.StageTherapy(
		created.ID,
		"",
		10.0,
		500.0,
		55.0,
		true,
		true,
		true,
		true,
		"",
		"Dr. Ingrid Vance, DACVIM (Neurology)",
	)
	if err != nil {
		t.Fatalf("unexpected StageTherapy error: %v", err)
	}
	if staged.Status != domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged || len(staged.TherapyStagings) != 1 {
		t.Errorf("expected status METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED and 1 staging, got %+v", staged)
	}

	// Stage therapy with explicit protocol and note
	stagedExplicit, err := svc.StageTherapy(
		created.ID,
		domain.ProtocolMethocarbamolMidazolamAcepromazineCRI,
		10.0,
		400.0,
		60.0,
		true,
		true,
		true,
		true,
		"Explicit wound infiltration 45 min prior to debridement",
		"Dr. Ingrid Vance, DACVIM (Neurology)",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageTherapy error: %v", err)
	}
	if stagedExplicit.Protocol != domain.ProtocolMethocarbamolMidazolamAcepromazineCRI || len(stagedExplicit.TherapyStagings) != 2 {
		t.Errorf("unexpected explicit staged state: %+v", stagedExplicit)
	}

	// Record neuro check and verify transition to SENSORY_ISOLATION_VENTILATION_MONITORED
	checked, err := svc.RecordNeuroCheck(
		created.ID,
		2,
		1,
		true,
		39.4,
		41.0,
		0, // fallback HR 96
		true,
		"Dr. Ingrid Vance, DACVIM (Neurology)",
		"Spasms reduced to 1/hr in dark/quiet room with cotton earplugs; gag reflex recovering.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordNeuroCheck error: %v", err)
	}
	if checked.Status != domain.ClostridialStatusSensoryIsolationVentilationMonitored || len(checked.NeuroChecks) != 1 || checked.HeartRateBPM != 96 {
		t.Errorf("unexpected checked operation state: %+v", checked)
	}

	// Evaluate clearance and verify transition to NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED
	cleared, err := svc.EvaluateClearance(
		created.ID,
		0,
		true,
		39.0,
		true,
		"Dr. Ingrid Vance, DACVIM (Neurology)",
		"0 spasms/hr, intact pharyngeal gag reflex and swallowing, PaCO2 39.0 mmHg, normothermic (38.5°C), ambulatory.",
	)
	if err != nil {
		t.Fatalf("unexpected EvaluateClearance error: %v", err)
	}
	if cleared.Status != domain.ClostridialStatusNormoreflexicSwallowingIntactCleared ||
		cleared.Clearance == nil ||
		cleared.ClearedAt == nil ||
		cleared.CoreTempCelsius != 38.5 {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestClostridialSafetyGuardsAndValidationErrors(t *testing.T) {
	svc := tetanusbotulism.NewTetanusBotulismService()

	// All syndrome branches in BuildClostridialClinicalAlerts
	for _, syn := range []domain.ClostridialSyndrome{
		domain.SyndromeTetanusClassIFacialRisusSardonicus,
		domain.SyndromeTetanusClassIIGeneralizedSawhorse,
		domain.SyndromeTetanusClassIIIRecumbentOpisthotonos,
		domain.SyndromeTetanusClassIVAutonomicStormApnea,
		domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia,
		"CUSTOM_SYNDROME",
	} {
		patho, _, _, _, _ := tetanusbotulism.BuildClostridialClinicalAlerts(25.0, syn, 2, 10.0, 300.0, 50.0, true, true, true, true)
		if patho == "" {
			t.Errorf("expected non-empty pathophysiology for %s", syn)
		}
	}

	createErrCases := []struct {
		name               string
		caseID             string
		petName            string
		weightKg           float64
		syndrome           domain.ClostridialSyndrome
		protocol           domain.ClostridialProtocol
		burkittClass       int
		spasmsPerHr        int
		tempC              float64
		paCO2              float64
		hrBPM              int
		metroMgKg          float64
		tatUnitsKg         float64
		methoMgKg          float64
		idVerified         bool
		penicillinAck      bool
		earplugsActive     bool
		aminoglycosideHold bool
	}{
		{"empty case_id", "", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"empty pet_name", "CASE-1", "", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"weight <= 0", "CASE-1", "Rex", 0, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"weight > 120", "CASE-1", "Rex", 125, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"invalid syndrome", "CASE-1", "Rex", 30, "BAD_SYN", domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"invalid protocol", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, "BAD_PROTO", 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"burkitt < 1", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 0, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"burkitt > 4", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 5, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"spasms < 0", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, -1, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"spasms > 60", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 65, 39.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"temp < 34", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 32.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"temp > 43.5", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 44.0, 42.0, 110, 10.0, 300, 50, true, true, true, true},
		{"paco2 < 15", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 10.0, 110, 10.0, 300, 50, true, true, true, true},
		{"paco2 > 110", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 120.0, 110, 10.0, 300, 50, true, true, true, true},
		{"hr < 20", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 10, 10.0, 300, 50, true, true, true, true},
		{"hr > 300", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 320, 10.0, 300, 50, true, true, true, true},
		{"metro > 25", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 30.0, 300, 50, true, true, true, true},
		{"tat < 0", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, -10, 50, true, true, true, true},
		{"tat > 1000", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 1200, 50, true, true, true, true},
		{"metho < 0", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, -5, true, true, true, true},
		{"metho > 150", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 160, true, true, true, true},
		{"unverified intradermal test with TAT > 0", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, false, true, true, true},
		{"unacknowledged penicillin GABA warning", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, false, true, true},
		{"inactive dark quiet cotton earplugs", "CASE-1", "Rex", 30, domain.SyndromeTetanusClassIIGeneralizedSawhorse, domain.ProtocolMetronidazole10MgKgBIDDebridement, 2, 4, 39.0, 42.0, 110, 10.0, 300, 50, true, true, false, true},
		{"unverified aminoglycoside contraindication", "CASE-1", "Rex", 30, domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia, domain.ProtocolDarkQuietCottonEarplugIsolation, 3, 0, 38.5, 50.0, 90, 10.0, 0, 0, true, true, true, false},
	}

	for _, tc := range createErrCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "German Shepherd",
				tc.weightKg, tc.syndrome, tc.protocol,
				tc.burkittClass, false, tc.spasmsPerHr,
				tc.tempC, tc.paCO2, tc.hrBPM,
				tc.metroMgKg, tc.tatUnitsKg, tc.methoMgKg,
				tc.idVerified, tc.penicillinAck, tc.earplugsActive, tc.aminoglycosideHold,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageTherapy validation & safety guard errors
	if _, err := svc.StageTherapy("CLB-OP-101", "BAD_PROTO", 10, 300, 50, true, true, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for invalid protocol in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 0, 300, 50, true, true, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for metronidazole <= 0 in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, -1, 50, true, true, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for equine TAT < 0 in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, -1, true, true, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for methocarbamol < 0 in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, 50, false, true, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for unverified intradermal skin test in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, 50, true, false, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for unacknowledged penicillin GABA warning in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, 50, true, true, false, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for inactive cotton earplugs in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, 50, true, true, true, false, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for unverified aminoglycoside guard in StageTherapy")
	}
	if _, err := svc.StageTherapy("CLB-OP-101", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, 50, true, true, true, true, "note", ""); err == nil {
		t.Errorf("expected error for empty staged_by in StageTherapy")
	}
	if _, err := svc.StageTherapy("NON-EXISTENT", domain.ProtocolEquineTAT100To1000IUKgAfterIDTest, 10, 300, 50, true, true, true, true, "note", "Dr. Vance"); err == nil {
		t.Errorf("expected error for unknown operation ID in StageTherapy")
	}

	// RecordNeuroCheck validation errors
	if _, err := svc.RecordNeuroCheck("CLB-OP-101", 0, 2, true, 38.5, 40.0, 90, true, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for burkitt < 1 in RecordNeuroCheck")
	}
	if _, err := svc.RecordNeuroCheck("CLB-OP-101", 2, -1, true, 38.5, 40.0, 90, true, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for spasms < 0 in RecordNeuroCheck")
	}
	if _, err := svc.RecordNeuroCheck("CLB-OP-101", 2, 2, true, 30.0, 40.0, 90, true, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for temp < 34 in RecordNeuroCheck")
	}
	if _, err := svc.RecordNeuroCheck("CLB-OP-101", 2, 2, true, 38.5, 10.0, 90, true, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for paco2 < 15 in RecordNeuroCheck")
	}
	if _, err := svc.RecordNeuroCheck("CLB-OP-101", 2, 2, true, 38.5, 40.0, 10, true, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for hr < 20 in RecordNeuroCheck")
	}
	if _, err := svc.RecordNeuroCheck("CLB-OP-101", 2, 2, true, 38.5, 40.0, 90, true, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by in RecordNeuroCheck")
	}
	if _, err := svc.RecordNeuroCheck("NON-EXISTENT", 2, 2, true, 38.5, 40.0, 90, true, "Dr. Vance", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordNeuroCheck")
	}

	// EvaluateClearance validation errors
	if _, err := svc.EvaluateClearance("CLB-OP-101", 2, true, 38.0, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for spasm_frequency_per_hr != 0 in EvaluateClearance")
	}
	if _, err := svc.EvaluateClearance("CLB-OP-101", 0, false, 38.0, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for gag_reflex_intact == false in EvaluateClearance")
	}
	if _, err := svc.EvaluateClearance("CLB-OP-101", 0, true, 50.0, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for paco2 > 45 in EvaluateClearance")
	}
	if _, err := svc.EvaluateClearance("CLB-OP-101", 0, true, 38.0, false, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for ambulatory_normothermic_verified == false in EvaluateClearance")
	}
	if _, err := svc.EvaluateClearance("CLB-OP-101", 0, true, 38.0, true, "", "summary"); err == nil {
		t.Errorf("expected error for empty cleared_by in EvaluateClearance")
	}
	if _, err := svc.EvaluateClearance("CLB-OP-101", 0, true, 38.0, true, "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for empty clearance_summary in EvaluateClearance")
	}
	if _, err := svc.EvaluateClearance("NON-EXISTENT", 0, true, 38.0, true, "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in EvaluateClearance")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("CLB-OP-101")
	badOps := []func(op *domain.TetanusBotulismOperation){
		func(op *domain.TetanusBotulismOperation) { op.ID = "" },
		func(op *domain.TetanusBotulismOperation) { op.CaseID = "" },
		func(op *domain.TetanusBotulismOperation) { op.PetName = "" },
		func(op *domain.TetanusBotulismOperation) { op.WeightKg = 0 },
		func(op *domain.TetanusBotulismOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.TetanusBotulismOperation) { op.Syndrome = "BAD_SYNDROME" },
		func(op *domain.TetanusBotulismOperation) { op.Protocol = "BAD_PROTOCOL" },
		func(op *domain.TetanusBotulismOperation) { op.BurkittSeverityClass = 0 },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}
}
