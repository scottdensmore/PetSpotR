package domain

import (
	"fmt"
	"strings"
	"time"
)

// TetanusBotulismStatus represents the clinical workflow state of an Acute Tetanus
// (Clostridium tetani — Tetanospasmin Renshaw Cell Glycine/GABA Blockade, Risus Sardonicus,
// Trismus & Sawhorse Stance) & Botulism (Clostridium botulinum — Botulinum Neurotoxin Type C
// Presynaptic SNARE/SNAP-25 Acetylcholine Blockade & Ascending Flaccid LMN Quadriplegia)
// triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type TetanusBotulismStatus string

const (
	ClostridialStatusMetronidazoleTATMethocarbamolStaged  TetanusBotulismStatus = "METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED"
	ClostridialStatusSensoryIsolationVentilationMonitored TetanusBotulismStatus = "SENSORY_ISOLATION_VENTILATION_MONITORED"
	ClostridialStatusNormoreflexicSwallowingIntactCleared TetanusBotulismStatus = "NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED"
)

// IsValid returns true when TetanusBotulismStatus is a recognized lifecycle state.
func (s TetanusBotulismStatus) IsValid() bool {
	switch s {
	case ClostridialStatusMetronidazoleTATMethocarbamolStaged,
		ClostridialStatusSensoryIsolationVentilationMonitored,
		ClostridialStatusNormoreflexicSwallowingIntactCleared:
		return true
	default:
		return false
	}
}

// ClostridialSyndrome classifies the Burkitt Tetanus Severity Class (I–IV) or Botulism
// Ascending Flaccid Lower Motor Neuron (LMN) Quadriplegia syndrome.
type ClostridialSyndrome string

const (
	SyndromeTetanusClassIFacialRisusSardonicus      ClostridialSyndrome = "TETANUS_CLASS_I_FACIAL_RISUS_SARDONICUS"
	SyndromeTetanusClassIIGeneralizedSawhorse       ClostridialSyndrome = "TETANUS_CLASS_II_GENERALIZED_SAWHORSE"
	SyndromeTetanusClassIIIRecumbentOpisthotonos    ClostridialSyndrome = "TETANUS_CLASS_III_RECUMBENT_OPISTHOTONOS"
	SyndromeTetanusClassIVAutonomicStormApnea       ClostridialSyndrome = "TETANUS_CLASS_IV_AUTONOMIC_STORM_APNEA"
	SyndromeBotulismAscendingFlaccidLMNQuadriplegia ClostridialSyndrome = "BOTULISM_ASCENDING_FLACCID_LMN_QUADRIPLEGIA"
)

// IsValid returns true when ClostridialSyndrome is a recognized clostridial neurotoxin syndrome.
func (s ClostridialSyndrome) IsValid() bool {
	switch s {
	case SyndromeTetanusClassIFacialRisusSardonicus,
		SyndromeTetanusClassIIGeneralizedSawhorse,
		SyndromeTetanusClassIIIRecumbentOpisthotonos,
		SyndromeTetanusClassIVAutonomicStormApnea,
		SyndromeBotulismAscendingFlaccidLMNQuadriplegia:
		return true
	default:
		return false
	}
}

// ClostridialProtocol represents the primary therapeutic and neuro-isolation regimen staged
// for an Acute Tetanus or Botulism ICU operation.
type ClostridialProtocol string

const (
	ProtocolMetronidazole10MgKgBIDDebridement     ClostridialProtocol = "METRONIDAZOLE_10MG_KG_BID_DEBRIDEMENT"
	ProtocolEquineTAT100To1000IUKgAfterIDTest     ClostridialProtocol = "EQUINE_TAT_100_1000_IU_KG_AFTER_ID_TEST"
	ProtocolMethocarbamolMidazolamAcepromazineCRI ClostridialProtocol = "METHOCARBAMOL_MIDAZOLAM_ACEPROMAZINE_CRI"
	ProtocolDarkQuietCottonEarplugIsolation       ClostridialProtocol = "DARK_QUIET_COTTON_EARPLUG_ISOLATION"
)

// IsValid returns true when ClostridialProtocol is a recognized therapeutic regimen.
func (p ClostridialProtocol) IsValid() bool {
	switch p {
	case ProtocolMetronidazole10MgKgBIDDebridement,
		ProtocolEquineTAT100To1000IUKgAfterIDTest,
		ProtocolMethocarbamolMidazolamAcepromazineCRI,
		ProtocolDarkQuietCottonEarplugIsolation:
		return true
	default:
		return false
	}
}

// TetanusBotulismTherapyStaging captures the staged antimicrobial, antitoxin, muscle relaxant,
// and dark/quiet sensory-isolation regimen with mandatory clinical safety guards.
type TetanusBotulismTherapyStaging struct {
	ID                                      string              `json:"id"`
	Protocol                                ClostridialProtocol `json:"protocol"`
	MetronidazoleMgKgBID                    float64             `json:"metronidazole_mg_kg_bid"`
	EquineTATUnitsKg                        float64             `json:"equine_tat_units_kg"`
	MethocarbamolMgKgIV                     float64             `json:"methocarbamol_mg_kg_iv"`
	IntradermalSkinTestNegativeVerified     bool                `json:"intradermal_skin_test_negative_verified"`
	PenicillinGABAWarningAcknowledged       bool                `json:"penicillin_gaba_warning_acknowledged"`
	DarkQuietCottonEarplugsActive           bool                `json:"dark_quiet_cotton_earplugs_active"`
	AminoglycosideContraindicatedInBotulism bool                `json:"aminoglycoside_contraindicated_in_botulism"`
	WoundInfiltrationPriorToDebridementNote string              `json:"wound_infiltration_prior_to_debridement_note"`
	PenicillinGABAAntagonismAlert           string              `json:"penicillin_gaba_antagonism_alert"`
	EquineTATIntradermalTestGuard           string              `json:"equine_tat_intradermal_test_guard"`
	SensoryIsolationCockpitPlan             string              `json:"sensory_isolation_cockpit_plan"`
	StagedBy                                string              `json:"staged_by"`
	StagedAt                                time.Time           `json:"staged_at"`
}

// TetanusBotulismNeuroCheck records a serial neurological, autonomic, and ventilatory assessment
// (Burkitt severity class 1–4, tetanic spasm frequency/hr, pharyngeal gag reflex, core temperature,
// arterial/venous PaCO2 mmHg, heart rate, and dark/quiet cotton earplug sensory isolation status).
type TetanusBotulismNeuroCheck struct {
	ID                            string    `json:"id"`
	BurkittSeverityClass          int       `json:"burkitt_severity_class"`
	SpasmFrequencyPerHr           int       `json:"spasm_frequency_per_hr"`
	GagReflexIntact               bool      `json:"gag_reflex_intact"`
	CoreTempCelsius               float64   `json:"core_temp_celsius"`
	PaCO2mmHg                     float64   `json:"paco2_mmhg"`
	HeartRateBPM                  int       `json:"heart_rate_bpm"`
	DarkQuietCottonEarplugsActive bool      `json:"dark_quiet_cotton_earplugs_active"`
	RecordedBy                    string    `json:"recorded_by"`
	ClinicalNote                  string    `json:"clinical_note"`
	RecordedAt                    time.Time `json:"recorded_at"`
}

// TetanusBotulismClearance records final ICU step-down / discharge clearance once tetanic spasms
// resolve (0/hr), pharyngeal gag/swallowing reflex is intact, normocapnia (PaCO2 <= 45 mmHg) is
// maintained, and ambulatory normothermia is verified.
type TetanusBotulismClearance struct {
	ClearedBy                      string    `json:"cleared_by"`
	SpasmFrequencyPerHr            int       `json:"spasm_frequency_per_hr"`
	GagReflexIntact                bool      `json:"gag_reflex_intact"`
	PaCO2mmHg                      float64   `json:"paco2_mmhg"`
	AmbulatoryNormothermicVerified bool      `json:"ambulatory_normothermic_verified"`
	ClearanceSummary               string    `json:"clearance_summary"`
	ClearedAt                      time.Time `json:"cleared_at"`
}

// TetanusBotulismOperation represents an Acute Tetanus (Clostridium tetani) or Botulism
// (Clostridium botulinum) neurotoxin triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type TetanusBotulismOperation struct {
	ID                                      string                          `json:"id"`
	CaseID                                  string                          `json:"case_id"`
	PetName                                 string                          `json:"pet_name"`
	Species                                 string                          `json:"species"`
	Breed                                   string                          `json:"breed"`
	WeightKg                                float64                         `json:"weight_kg"`
	Status                                  TetanusBotulismStatus           `json:"status"`
	Syndrome                                ClostridialSyndrome             `json:"syndrome"`
	Protocol                                ClostridialProtocol             `json:"protocol"`
	BurkittSeverityClass                    int                             `json:"burkitt_severity_class"`
	GagReflexIntact                         bool                            `json:"gag_reflex_intact"`
	SpasmFrequencyPerHr                     int                             `json:"spasm_frequency_per_hr"`
	CoreTempCelsius                         float64                         `json:"core_temp_celsius"`
	PaCO2mmHg                               float64                         `json:"paco2_mmhg"`
	HeartRateBPM                            int                             `json:"heart_rate_bpm"`
	MetronidazoleMgKgBID                    float64                         `json:"metronidazole_mg_kg_bid"`
	EquineTATUnitsKg                        float64                         `json:"equine_tat_units_kg"`
	MethocarbamolMgKgIV                     float64                         `json:"methocarbamol_mg_kg_iv"`
	IntradermalSkinTestNegativeVerified     bool                            `json:"intradermal_skin_test_negative_verified"`
	PenicillinGABAWarningAcknowledged       bool                            `json:"penicillin_gaba_warning_acknowledged"`
	DarkQuietCottonEarplugsActive           bool                            `json:"dark_quiet_cotton_earplugs_active"`
	AminoglycosideContraindicatedInBotulism bool                            `json:"aminoglycoside_contraindicated_in_botulism"`
	PathophysiologySummary                  string                          `json:"pathophysiology_summary"`
	PenicillinGABAAntagonismAlert           string                          `json:"penicillin_gaba_antagonism_alert"`
	EquineTATIntradermalTestGuard           string                          `json:"equine_tat_intradermal_test_guard"`
	AminoglycosideBotulismAlert             string                          `json:"aminoglycoside_botulism_alert"`
	SensoryIsolationCockpitPlan             string                          `json:"sensory_isolation_cockpit_plan"`
	TherapyStagings                         []TetanusBotulismTherapyStaging `json:"therapy_stagings"`
	NeuroChecks                             []TetanusBotulismNeuroCheck     `json:"neuro_checks"`
	Clearance                               *TetanusBotulismClearance       `json:"clearance,omitempty"`
	ClearedBy                               string                          `json:"cleared_by,omitempty"`
	ClearanceSummary                        string                          `json:"clearance_summary,omitempty"`
	ClearedAt                               *time.Time                      `json:"cleared_at,omitempty"`
	CreatedAt                               time.Time                       `json:"created_at"`
	UpdatedAt                               time.Time                       `json:"updated_at"`
}

// Validate checks domain invariants on TetanusBotulismOperation.
func (op TetanusBotulismOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.CaseID) == "" {
		return fmt.Errorf("case_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if op.WeightKg <= 0 {
		return fmt.Errorf("weight_kg must be positive, got %v", op.WeightKg)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if !op.Syndrome.IsValid() {
		return fmt.Errorf("invalid syndrome: %q", op.Syndrome)
	}
	if !op.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %q", op.Protocol)
	}
	if op.BurkittSeverityClass < 1 || op.BurkittSeverityClass > 4 {
		return fmt.Errorf("burkitt_severity_class must be in [1, 4], got %d", op.BurkittSeverityClass)
	}
	return nil
}

// TetanusBotulismOverview summarizes aggregate counts across Acute Tetanus & Botulism operations.
type TetanusBotulismOverview struct {
	TotalOperations                int `json:"total_operations"`
	MetronidazoleTATStagedCount    int `json:"metronidazole_tat_staged_count"`
	SensoryIsolationMonitoredCount int `json:"sensory_isolation_monitored_count"`
	NormoreflexicClearedCount      int `json:"normoreflexic_cleared_count"`
	SevereBurkittClassIIIIVCount   int `json:"severe_burkitt_class_iii_iv_count"`
	BotulismLMNParalysisCount      int `json:"botulism_lmn_paralysis_count"`
}
