package tetanusbotulism

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TetanusBotulismService manages Acute Tetanus (Clostridium tetani), Botulism (Clostridium botulinum),
// Risus Sardonicus / Sawhorse Stance vs. Ascending Flaccid Lower Motor Neuron Paralysis,
// Equine Tetanus Antitoxin (TAT) Intradermal Test, Metronidazole (vs. Penicillin GABA Antagonism) &
// Dark/Quiet Sensory-Isolation Cockpit operations in a thread-safe store.
type TetanusBotulismService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.TetanusBotulismOperation
	order []string
	seq   int
}

// NewTetanusBotulismService initializes a TetanusBotulismService with 3 seeded operations
// (CLB-OP-101, CLB-OP-102, CLB-OP-103) across all 3 lifecycle statuses.
func NewTetanusBotulismService() *TetanusBotulismService {
	s := &TetanusBotulismService{
		ops:   make(map[string]*domain.TetanusBotulismOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildClostridialClinicalAlerts computes the syndrome pathophysiology summary, Penicillin G GABA-A
// antagonism warning vs. Metronidazole, Equine Tetanus Antitoxin (TAT) intradermal skin test guard,
// aminoglycoside neuromuscular blockade contraindication in Botulism, and dark/quiet sensory-isolation plan.
func BuildClostridialClinicalAlerts(
	weightKg float64,
	syndrome domain.ClostridialSyndrome,
	burkittClass int,
	metronidazoleMgKgBID, equineTATUnitsKg, methocarbamolMgKgIV float64,
	intradermalVerified, penicillinAcknowledged, earplugsActive, aminoglycosideContraindicated bool,
) (patho, penicillinAlert, tatGuard, aminoglycosideAlert, isolationPlan string) {
	switch syndrome {
	case domain.SyndromeTetanusClassIFacialRisusSardonicus:
		patho = "Clostridium tetani Localized / Cranial Tetanus (Burkitt Class I) — Retrograde axonal transport of tetanospasmin metalloprotease cleaving synaptobrevin (VAMP) in inhibitory spinal/brainstem interneurons (Renshaw cells), blocking glycine and GABA release and producing facial trismus (lockjaw), Risus Sardonicus (retracted commissures), prolapsed nictitans, and erect ears while patient remains ambulatory."
	case domain.SyndromeTetanusClassIIGeneralizedSawhorse:
		patho = "Clostridium tetani Generalized Tetanus (Burkitt Class II) — Diffuse spinal Renshaw cell glycine/GABA blockade causing generalized appendicular extensor rigidity, classic 'Sawhorse Stance', stiff tail, dysphagia/pharyngeal spasm, and stimulus-evoked reflex muscle spasms."
	case domain.SyndromeTetanusClassIIIRecumbentOpisthotonos:
		patho = "Clostridium tetani Severe Recumbent Tetanus (Burkitt Class III) — Lateral recumbency with severe opisthotonos, frequent violent stimulus-induced tetanic spasms, loss of pharyngeal gag reflex (high aspiration pneumonia risk), and neurogenic hyperthermia."
	case domain.SyndromeTetanusClassIVAutonomicStormApnea:
		patho = "Clostridium tetani Fulminant Tetanus with Autonomic Storm & Apnea (Burkitt Class IV) — Severe opisthotonos complicated by sympathetic/parasympathetic autonomic storm (labile tachyarrhythmias/bradycardia, hypertension/hypotension), laryngospasm, and diaphragmatic/intercostal tetanic paralysis requiring tracheostomy and positive-pressure mechanical ventilation."
	case domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia:
		patho = "Clostridium botulinum Neurotoxin (BoNT Type C) Ascending Flaccid Lower Motor Neuron (LMN) Quadriplegia — Carrion/raw meat ingestion of preformed BoNT cleaving presynaptic SNARE (SNAP-25/syntaxin) at neuromuscular junctions, blocking acetylcholine (ACh) exocytosis and causing symmetric ascending flaccid LMN paralysis, hyporeflexia/areflexia, cranial nerve VII–XII dysphagia, megaesophagus, and phrenic ventilatory failure with preserved nociception."
	default:
		patho = fmt.Sprintf("Acute Clostridial Neurotoxin Triage (Burkitt Class %d) — Differentiate Tetanus upper/inhibitory interneuron spastic rigidity vs. Botulism presynaptic NMJ flaccid LMN paralysis.", burkittClass)
	}

	metroTotalMg := round1(weightKg * metronidazoleMgKgBID)
	penicillinAlert = fmt.Sprintf(
		"METRONIDAZOLE %.1f mg/kg IV q12h (%.1f mg/dose for %.1f kg patient) vs. PENICILLIN G GABA-A ANTAGONISM WARNING (Acknowledged: %t): Metronidazole is the bactericidal drug of choice for Clostridium tetani with superior anaerobic necrotic tissue penetration. STRICTLY AVOID Penicillin G because its beta-lactam ring is a structural GABA-A receptor antagonist that synergizes with tetanospasmin-induced Renshaw cell disinhibition to precipitate refractory tetanic spasms and seizures.",
		metronidazoleMgKgBID, metroTotalMg, weightKg, penicillinAcknowledged,
	)

	rawTATUnits := round1(weightKg * equineTATUnitsKg)
	cappedTATUnits := rawTATUnits
	if cappedTATUnits > 20000.0 {
		cappedTATUnits = 20000.0
	}
	tatGuard = fmt.Sprintf(
		"EQUINE TETANUS ANTITOXIN (TAT) %.0f IU/kg IV (Calculated %.0f IU, Capped at %.0f IU <= 20,000 IU max | Intradermal Skin Test Negative Verified: %t): Equine TAT ONLY neutralizes unbound circulating tetanospasmin (cannot reverse toxin already bound inside CNS axons; takes 2–4 weeks for new axonal terminal sprouting). MANDATORY 0.1–0.2 mL intradermal test observed 20–30 min for wheal/flare anaphylaxis BEFORE slow IV infusion, with local perilesional TAT infiltration performed 30–60 min PRIOR to surgical wound debridement.",
		equineTATUnitsKg, rawTATUnits, cappedTATUnits, intradermalVerified,
	)

	aminoglycosideAlert = fmt.Sprintf(
		"AMINOGLYCOSIDE NEUROMUSCULAR BLOCKADE CONTRAINDICATION GUARD (Verified: %t): STRICTLY CONTRAINDICATED in Clostridium botulinum LMN paralysis and Clostridial neuropathies — Aminoglycosides (Gentamicin, Amikacin, Neomycin), Fluoroquinolones, and Magnesium salts inhibit presynaptic voltage-gated calcium channels and acetylcholine release, fatally potentiating neuromuscular junction blockade and acute diaphragmatic apnea.",
		aminoglycosideContraindicated,
	)

	methoTotalMg := round1(weightKg * methocarbamolMgKgIV)
	isolationPlan = fmt.Sprintf(
		"DARK, QUIET SENSORY-ISOLATION COCKPIT & COTTON EARPLUGS (Active: %t | Methocarbamol %.1f mg/kg = %.1f mg IV): House patient in a darkened, sound-dampened ICU isolation suite with sterile cotton balls placed in both external ear canals, clustered nursing care every 4–6h, padded turning mattress q4h to prevent decubital ulcers, and Methocarbamol + Midazolam/Diazepam + low-dose Acepromazine CRI prior to any tactile or auditory handling.",
		earplugsActive, methocarbamolMgKgIV, methoTotalMg,
	)

	return patho, penicillinAlert, tatGuard, aminoglycosideAlert, isolationPlan
}

func (s *TetanusBotulismService) seed() {
	now := time.Now().UTC()

	// CLB-OP-101: Rex — Canine, German Shepherd, 32.0 kg, METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED
	patho101, pen101, tat101, amino101, iso101 := BuildClostridialClinicalAlerts(
		32.0,
		domain.SyndromeTetanusClassIIIRecumbentOpisthotonos,
		3,
		10.0, 500.0, 55.0,
		true, true, true, true,
	)
	op101 := &domain.TetanusBotulismOperation{
		ID:                                      "CLB-OP-101",
		CaseID:                                  "CASE-CLB-101",
		PetName:                                 "Rex",
		Species:                                 "Canine",
		Breed:                                   "German Shepherd",
		WeightKg:                                32.0,
		Status:                                  domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged,
		Syndrome:                                domain.SyndromeTetanusClassIIIRecumbentOpisthotonos,
		Protocol:                                domain.ProtocolEquineTAT100To1000IUKgAfterIDTest,
		BurkittSeverityClass:                    3,
		GagReflexIntact:                         false,
		SpasmFrequencyPerHr:                     6,
		CoreTempCelsius:                         39.8,
		PaCO2mmHg:                               49.0,
		HeartRateBPM:                            148,
		MetronidazoleMgKgBID:                    10.0,
		EquineTATUnitsKg:                        500.0,
		MethocarbamolMgKgIV:                     55.0,
		IntradermalSkinTestNegativeVerified:     true,
		PenicillinGABAWarningAcknowledged:       true,
		DarkQuietCottonEarplugsActive:           true,
		AminoglycosideContraindicatedInBotulism: true,
		PathophysiologySummary:                  patho101,
		PenicillinGABAAntagonismAlert:           pen101,
		EquineTATIntradermalTestGuard:           tat101,
		AminoglycosideBotulismAlert:             amino101,
		SensoryIsolationCockpitPlan:             iso101,
		TherapyStagings: []domain.TetanusBotulismTherapyStaging{
			{
				ID:                                      "CLB-STG-101-1",
				Protocol:                                domain.ProtocolEquineTAT100To1000IUKgAfterIDTest,
				MetronidazoleMgKgBID:                    10.0,
				EquineTATUnitsKg:                        500.0,
				MethocarbamolMgKgIV:                     55.0,
				IntradermalSkinTestNegativeVerified:     true,
				PenicillinGABAWarningAcknowledged:       true,
				DarkQuietCottonEarplugsActive:           true,
				AminoglycosideContraindicatedInBotulism: true,
				WoundInfiltrationPriorToDebridementNote: "0.2 mL intradermal Equine TAT skin test negative at 30 min; 2,000 IU infiltrated perilesionally around deep interdigital puncture wound 45 min prior to surgical debridement + H2O2 aeration, remaining 14,000 IU infused IV slowly over 60 min.",
				PenicillinGABAAntagonismAlert:           pen101,
				EquineTATIntradermalTestGuard:           tat101,
				SensoryIsolationCockpitPlan:             iso101,
				StagedBy:                                "Dr. Ingrid Vance, DACVIM (Neurology)",
				StagedAt:                                now.Add(-45 * time.Minute),
			},
		},
		NeuroChecks: []domain.TetanusBotulismNeuroCheck{
			{
				ID:                            "CLB-CHK-101-1",
				BurkittSeverityClass:          3,
				SpasmFrequencyPerHr:           6,
				GagReflexIntact:               false,
				CoreTempCelsius:               39.8,
				PaCO2mmHg:                     49.0,
				HeartRateBPM:                  148,
				DarkQuietCottonEarplugsActive: true,
				RecordedBy:                    "Dr. Ingrid Vance, DACVIM (Neurology)",
				ClinicalNote:                  "Burkitt Class III Generalized Tetanus: Risus sardonicus, trismus, sawhorse extensor rigidity with lateral recumbency, 6 stimulus-induced spasms/hr, core temp 39.8°C, PaCO2 49.0 mmHg; Metronidazole 10 mg/kg IV q12h (320 mg), Equine TAT 16,000 IU IV after negative ID test, and Methocarbamol 55 mg/kg IV + Midazolam CRI staged.",
				RecordedAt:                    now.Add(-20 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-20 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// CLB-OP-102: Otis — Canine, Basset Hound, 24.5 kg, SENSORY_ISOLATION_VENTILATION_MONITORED
	patho102, pen102, tat102, amino102, iso102 := BuildClostridialClinicalAlerts(
		24.5,
		domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia,
		3,
		10.0, 0.0, 0.0,
		true, true, true, true,
	)
	op102 := &domain.TetanusBotulismOperation{
		ID:                                      "CLB-OP-102",
		CaseID:                                  "CASE-CLB-102",
		PetName:                                 "Otis",
		Species:                                 "Canine",
		Breed:                                   "Basset Hound",
		WeightKg:                                24.5,
		Status:                                  domain.ClostridialStatusSensoryIsolationVentilationMonitored,
		Syndrome:                                domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia,
		Protocol:                                domain.ProtocolDarkQuietCottonEarplugIsolation,
		BurkittSeverityClass:                    3,
		GagReflexIntact:                         false,
		SpasmFrequencyPerHr:                     0,
		CoreTempCelsius:                         38.4,
		PaCO2mmHg:                               51.5,
		HeartRateBPM:                            92,
		MetronidazoleMgKgBID:                    10.0,
		EquineTATUnitsKg:                        0.0,
		MethocarbamolMgKgIV:                     0.0,
		IntradermalSkinTestNegativeVerified:     true,
		PenicillinGABAWarningAcknowledged:       true,
		DarkQuietCottonEarplugsActive:           true,
		AminoglycosideContraindicatedInBotulism: true,
		PathophysiologySummary:                  patho102,
		PenicillinGABAAntagonismAlert:           pen102,
		EquineTATIntradermalTestGuard:           tat102,
		AminoglycosideBotulismAlert:             amino102,
		SensoryIsolationCockpitPlan:             iso102,
		TherapyStagings: []domain.TetanusBotulismTherapyStaging{
			{
				ID:                                      "CLB-STG-102-1",
				Protocol:                                domain.ProtocolDarkQuietCottonEarplugIsolation,
				MetronidazoleMgKgBID:                    10.0,
				EquineTATUnitsKg:                        0.0,
				MethocarbamolMgKgIV:                     0.0,
				IntradermalSkinTestNegativeVerified:     true,
				PenicillinGABAWarningAcknowledged:       true,
				DarkQuietCottonEarplugsActive:           true,
				AminoglycosideContraindicatedInBotulism: true,
				WoundInfiltrationPriorToDebridementNote: "Carrion-associated Clostridium botulinum Type C ascending LMN flaccid quadriplegia; Aminoglycosides strictly contraindicated; sternal recumbency sling, corneal lubrication, urinary catheterization, and capnography/PaCO2 ventilatory monitoring active.",
				PenicillinGABAAntagonismAlert:           pen102,
				EquineTATIntradermalTestGuard:           tat102,
				SensoryIsolationCockpitPlan:             iso102,
				StagedBy:                                "Dr. Ingrid Vance, DACVIM (Neurology)",
				StagedAt:                                now.Add(-4 * time.Hour),
			},
		},
		NeuroChecks: []domain.TetanusBotulismNeuroCheck{
			{
				ID:                            "CLB-CHK-102-1",
				BurkittSeverityClass:          3,
				SpasmFrequencyPerHr:           0,
				GagReflexIntact:               false,
				CoreTempCelsius:               38.4,
				PaCO2mmHg:                     51.5,
				HeartRateBPM:                  92,
				DarkQuietCottonEarplugsActive: true,
				RecordedBy:                    "Dr. Ingrid Vance, DACVIM (Neurology)",
				ClinicalNote:                  "Ascending flaccid LMN quadriplegia with absent patellar/withdrawal reflexes, intact tail wag & nociception, reduced gag reflex, mild hypoventilation (PaCO2 51.5 mmHg); Aminoglycosides held, NPO + parenteral nutrition & ventilatory watch in progress.",
				RecordedAt:                    now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-5 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// CLB-OP-103: Bella — Canine, Golden Retriever, 27.0 kg, NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED
	patho103, pen103, tat103, amino103, iso103 := BuildClostridialClinicalAlerts(
		27.0,
		domain.SyndromeTetanusClassIFacialRisusSardonicus,
		1,
		10.0, 300.0, 44.0,
		true, true, true, true,
	)
	clearedAt := now.Add(-30 * time.Minute)
	clearance103 := &domain.TetanusBotulismClearance{
		ClearedBy:                      "Dr. Ingrid Vance, DACVIM (Neurology)",
		SpasmFrequencyPerHr:            0,
		GagReflexIntact:                true,
		PaCO2mmHg:                      38.0,
		AmbulatoryNormothermicVerified: true,
		ClearanceSummary:               "Day 18 Tetanus recovery complete: Risus sardonicus and trismus resolved, 0 spasms/hr off Methocarbamol/Midazolam, intact pharyngeal gag reflex and voluntary swallowing, normocapnic (PaCO2 38.0 mmHg), normothermic (38.5°C), and fully ambulatory.",
		ClearedAt:                      clearedAt,
	}
	op103 := &domain.TetanusBotulismOperation{
		ID:                                      "CLB-OP-103",
		CaseID:                                  "CASE-CLB-103",
		PetName:                                 "Bella",
		Species:                                 "Canine",
		Breed:                                   "Golden Retriever",
		WeightKg:                                27.0,
		Status:                                  domain.ClostridialStatusNormoreflexicSwallowingIntactCleared,
		Syndrome:                                domain.SyndromeTetanusClassIFacialRisusSardonicus,
		Protocol:                                domain.ProtocolMetronidazole10MgKgBIDDebridement,
		BurkittSeverityClass:                    1,
		GagReflexIntact:                         true,
		SpasmFrequencyPerHr:                     0,
		CoreTempCelsius:                         38.5,
		PaCO2mmHg:                               38.0,
		HeartRateBPM:                            84,
		MetronidazoleMgKgBID:                    10.0,
		EquineTATUnitsKg:                        300.0,
		MethocarbamolMgKgIV:                     44.0,
		IntradermalSkinTestNegativeVerified:     true,
		PenicillinGABAWarningAcknowledged:       true,
		DarkQuietCottonEarplugsActive:           true,
		AminoglycosideContraindicatedInBotulism: true,
		PathophysiologySummary:                  patho103,
		PenicillinGABAAntagonismAlert:           pen103,
		EquineTATIntradermalTestGuard:           tat103,
		AminoglycosideBotulismAlert:             amino103,
		SensoryIsolationCockpitPlan:             iso103,
		TherapyStagings: []domain.TetanusBotulismTherapyStaging{
			{
				ID:                                      "CLB-STG-103-1",
				Protocol:                                domain.ProtocolMetronidazole10MgKgBIDDebridement,
				MetronidazoleMgKgBID:                    10.0,
				EquineTATUnitsKg:                        300.0,
				MethocarbamolMgKgIV:                     44.0,
				IntradermalSkinTestNegativeVerified:     true,
				PenicillinGABAWarningAcknowledged:       true,
				DarkQuietCottonEarplugsActive:           true,
				AminoglycosideContraindicatedInBotulism: true,
				WoundInfiltrationPriorToDebridementNote: "Negative 0.15 mL intradermal Equine TAT skin test at 25 min; 8,100 IU Equine TAT administered slow IV + local infiltration prior to carnassial tooth root abscess debridement; Metronidazole 10 mg/kg IV q12h x 10 days.",
				PenicillinGABAAntagonismAlert:           pen103,
				EquineTATIntradermalTestGuard:           tat103,
				SensoryIsolationCockpitPlan:             iso103,
				StagedBy:                                "Dr. Ingrid Vance, DACVIM (Neurology)",
				StagedAt:                                now.Add(-14 * 24 * time.Hour),
			},
		},
		NeuroChecks: []domain.TetanusBotulismNeuroCheck{
			{
				ID:                            "CLB-CHK-103-1",
				BurkittSeverityClass:          1,
				SpasmFrequencyPerHr:           0,
				GagReflexIntact:               true,
				CoreTempCelsius:               38.5,
				PaCO2mmHg:                     38.0,
				HeartRateBPM:                  84,
				DarkQuietCottonEarplugsActive: true,
				RecordedBy:                    "Dr. Ingrid Vance, DACVIM (Neurology)",
				ClinicalNote:                  "Final neurological check: Burkitt Class I resolved, 0 spasms/hr, brisk gag reflex, voluntary water/food intake without regurgitation, PaCO2 38.0 mmHg, core temp 38.5°C.",
				RecordedAt:                    now.Add(-45 * time.Minute),
			},
		},
		Clearance:        clearance103,
		ClearedBy:        clearance103.ClearedBy,
		ClearanceSummary: clearance103.ClearanceSummary,
		ClearedAt:        &clearedAt,
		CreatedAt:        now.Add(-14 * 24 * time.Hour),
		UpdatedAt:        clearedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Acute Tetanus & Botulism operations, optionally filtered by statusFilter.
func (s *TetanusBotulismService) ListOperations(statusFilter string) []domain.TetanusBotulismOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.TetanusBotulismOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneClostridialOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an Acute Tetanus & Botulism operation by ID.
func (s *TetanusBotulismService) GetOperation(id string) (*domain.TetanusBotulismOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("tetanus botulism operation %q not found", id)
	}
	cp := cloneClostridialOperation(op)
	return &cp, nil
}

// CalculateOverview computes summary telemetry counts across all Acute Tetanus & Botulism operations.
func (s *TetanusBotulismService) CalculateOverview() domain.TetanusBotulismOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.TetanusBotulismOverview
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged:
			m.MetronidazoleTATStagedCount++
		case domain.ClostridialStatusSensoryIsolationVentilationMonitored:
			m.SensoryIsolationMonitoredCount++
		case domain.ClostridialStatusNormoreflexicSwallowingIntactCleared:
			m.NormoreflexicClearedCount++
		}
		if op.BurkittSeverityClass >= 3 ||
			op.Syndrome == domain.SyndromeTetanusClassIIIRecumbentOpisthotonos ||
			op.Syndrome == domain.SyndromeTetanusClassIVAutonomicStormApnea {
			m.SevereBurkittClassIIIIVCount++
		}
		if op.Syndrome == domain.SyndromeBotulismAscendingFlaccidLMNQuadriplegia {
			m.BotulismLMNParalysisCount++
		}
	}
	return m
}

// GetOverviewMetrics is an alias for CalculateOverview.
func (s *TetanusBotulismService) GetOverviewMetrics() domain.TetanusBotulismOverview {
	return s.CalculateOverview()
}

// CreateOperation validates clinical parameters and safety guards and creates a new
// Acute Tetanus or Botulism triage operation.
func (s *TetanusBotulismService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	syndrome domain.ClostridialSyndrome,
	protocol domain.ClostridialProtocol,
	burkittSeverityClass int,
	gagReflexIntact bool,
	spasmFrequencyPerHr int,
	coreTempCelsius, paCO2mmHg float64,
	heartRateBPM int,
	metronidazoleMgKgBID, equineTATUnitsKg, methocarbamolMgKgIV float64,
	intradermalSkinTestNegativeVerified, penicillinGABAWarningAcknowledged, darkQuietCottonEarplugsActive, aminoglycosideContraindicatedInBotulism bool,
) (*domain.TetanusBotulismOperation, error) {
	trimmedCase := strings.TrimSpace(caseID)
	if trimmedCase == "" {
		return nil, fmt.Errorf("case_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	trimmedSpecies := strings.TrimSpace(species)
	if trimmedSpecies == "" {
		trimmedSpecies = "Canine"
	}
	trimmedBreed := strings.TrimSpace(breed)
	if trimmedBreed == "" {
		trimmedBreed = "Mixed Breed"
	}
	if weightKg <= 0 || weightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 120.0], got %v", weightKg)
	}
	if !syndrome.IsValid() {
		return nil, fmt.Errorf("invalid syndrome: %q", syndrome)
	}
	if protocol == "" {
		protocol = domain.ProtocolMetronidazole10MgKgBIDDebridement
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if burkittSeverityClass < 1 || burkittSeverityClass > 4 {
		return nil, fmt.Errorf("burkitt_severity_class must be in [1, 4], got %d", burkittSeverityClass)
	}
	if spasmFrequencyPerHr < 0 || spasmFrequencyPerHr > 60 {
		return nil, fmt.Errorf("spasm_frequency_per_hr must be in [0, 60], got %d", spasmFrequencyPerHr)
	}
	if coreTempCelsius < 34.0 || coreTempCelsius > 43.5 {
		return nil, fmt.Errorf("core_temp_celsius must be in [34.0, 43.5], got %v", coreTempCelsius)
	}
	if paCO2mmHg < 15.0 || paCO2mmHg > 110.0 {
		return nil, fmt.Errorf("paco2_mmhg must be in [15.0, 110.0], got %v", paCO2mmHg)
	}
	if heartRateBPM <= 0 {
		heartRateBPM = 110
	}
	if heartRateBPM < 20 || heartRateBPM > 300 {
		return nil, fmt.Errorf("heart_rate_bpm must be in [20, 300], got %d", heartRateBPM)
	}
	if metronidazoleMgKgBID <= 0 {
		metronidazoleMgKgBID = 10.0
	}
	if metronidazoleMgKgBID > 25.0 {
		return nil, fmt.Errorf("metronidazole_mg_kg_bid must be in (0, 25.0], got %v", metronidazoleMgKgBID)
	}
	if equineTATUnitsKg < 0 || equineTATUnitsKg > 1000.0 {
		return nil, fmt.Errorf("equine_tat_units_kg must be in [0, 1000.0], got %v", equineTATUnitsKg)
	}
	if methocarbamolMgKgIV < 0 || methocarbamolMgKgIV > 150.0 {
		return nil, fmt.Errorf("methocarbamol_mg_kg_iv must be in [0, 150.0], got %v", methocarbamolMgKgIV)
	}
	if equineTATUnitsKg > 0 && !intradermalSkinTestNegativeVerified {
		return nil, fmt.Errorf("intradermal_skin_test_negative_verified must be true (0.1–0.2 mL intradermal test observed 20–30 min for wheal/flare) before administering Equine Tetanus Antitoxin IV")
	}
	if !penicillinGABAWarningAcknowledged {
		return nil, fmt.Errorf("penicillin_gaba_warning_acknowledged must be true (Metronidazole 10 mg/kg IV q12h preferred over Penicillin G GABA-A antagonism)")
	}
	if !darkQuietCottonEarplugsActive {
		return nil, fmt.Errorf("dark_quiet_cotton_earplugs_active must be true (dark, quiet sensory-isolation room with cotton earplugs required)")
	}
	if !aminoglycosideContraindicatedInBotulism {
		return nil, fmt.Errorf("aminoglycoside_contraindicated_in_botulism must be true (Aminoglycosides block presynaptic acetylcholine release and fatally worsen Botulism neuromuscular blockade)")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("CLB-OP-%d", s.seq)
	now := time.Now().UTC()

	patho, penAlert, tatGuard, aminoAlert, isoPlan := BuildClostridialClinicalAlerts(
		weightKg, syndrome, burkittSeverityClass,
		metronidazoleMgKgBID, equineTATUnitsKg, methocarbamolMgKgIV,
		intradermalSkinTestNegativeVerified, penicillinGABAWarningAcknowledged,
		darkQuietCottonEarplugsActive, aminoglycosideContraindicatedInBotulism,
	)

	op := &domain.TetanusBotulismOperation{
		ID:                                      opID,
		CaseID:                                  trimmedCase,
		PetName:                                 trimmedPet,
		Species:                                 trimmedSpecies,
		Breed:                                   trimmedBreed,
		WeightKg:                                round1(weightKg),
		Status:                                  domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged,
		Syndrome:                                syndrome,
		Protocol:                                protocol,
		BurkittSeverityClass:                    burkittSeverityClass,
		GagReflexIntact:                         gagReflexIntact,
		SpasmFrequencyPerHr:                     spasmFrequencyPerHr,
		CoreTempCelsius:                         round1(coreTempCelsius),
		PaCO2mmHg:                               round1(paCO2mmHg),
		HeartRateBPM:                            heartRateBPM,
		MetronidazoleMgKgBID:                    round1(metronidazoleMgKgBID),
		EquineTATUnitsKg:                        round1(equineTATUnitsKg),
		MethocarbamolMgKgIV:                     round1(methocarbamolMgKgIV),
		IntradermalSkinTestNegativeVerified:     intradermalSkinTestNegativeVerified,
		PenicillinGABAWarningAcknowledged:       penicillinGABAWarningAcknowledged,
		DarkQuietCottonEarplugsActive:           darkQuietCottonEarplugsActive,
		AminoglycosideContraindicatedInBotulism: aminoglycosideContraindicatedInBotulism,
		PathophysiologySummary:                  patho,
		PenicillinGABAAntagonismAlert:           penAlert,
		EquineTATIntradermalTestGuard:           tatGuard,
		AminoglycosideBotulismAlert:             aminoAlert,
		SensoryIsolationCockpitPlan:             isoPlan,
		TherapyStagings:                         []domain.TetanusBotulismTherapyStaging{},
		NeuroChecks:                             []domain.TetanusBotulismNeuroCheck{},
		CreatedAt:                               now,
		UpdatedAt:                               now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneClostridialOperation(op)
	return &cp, nil
}

// StageTherapy validates mandatory clinical safety guards (intradermal skin test negative prior to
// Equine TAT IV infusion, Penicillin G GABA-A antagonism acknowledgment vs. Metronidazole 10 mg/kg BID,
// dark/quiet sensory-isolation room with cotton earplugs, and aminoglycoside neuromuscular blockade
// contraindication) and stages the therapeutic regimen on the operation.
func (s *TetanusBotulismService) StageTherapy(
	id string,
	protocol domain.ClostridialProtocol,
	metronidazoleMgKgBID, equineTATUnitsKg, methocarbamolMgKgIV float64,
	intradermalSkinTestNegativeVerified, penicillinGABAWarningAcknowledged, darkQuietCottonEarplugsActive, aminoglycosideContraindicatedInBotulism bool,
	woundInfiltrationNote, stagedBy string,
) (*domain.TetanusBotulismOperation, error) {
	if protocol == "" {
		protocol = domain.ProtocolEquineTAT100To1000IUKgAfterIDTest
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if metronidazoleMgKgBID <= 0 || metronidazoleMgKgBID > 25.0 {
		return nil, fmt.Errorf("metronidazole_mg_kg_bid must be in (0, 25.0], got %v", metronidazoleMgKgBID)
	}
	if equineTATUnitsKg < 0 || equineTATUnitsKg > 1000.0 {
		return nil, fmt.Errorf("equine_tat_units_kg must be in [0, 1000.0], got %v", equineTATUnitsKg)
	}
	if methocarbamolMgKgIV < 0 || methocarbamolMgKgIV > 150.0 {
		return nil, fmt.Errorf("methocarbamol_mg_kg_iv must be in [0, 150.0], got %v", methocarbamolMgKgIV)
	}
	if equineTATUnitsKg > 0 && !intradermalSkinTestNegativeVerified {
		return nil, fmt.Errorf("intradermal_skin_test_negative_verified must be true (0.1–0.2 mL intradermal skin test observed 20–30 min for wheal/flare) before administering Equine Tetanus Antitoxin IV")
	}
	if !penicillinGABAWarningAcknowledged {
		return nil, fmt.Errorf("penicillin_gaba_warning_acknowledged must be true (Metronidazole 10 mg/kg IV q12h preferred over Penicillin G GABA-A antagonism)")
	}
	if !darkQuietCottonEarplugsActive {
		return nil, fmt.Errorf("dark_quiet_cotton_earplugs_active must be true (dark, quiet sensory-isolation room with cotton earplugs required)")
	}
	if !aminoglycosideContraindicatedInBotulism {
		return nil, fmt.Errorf("aminoglycoside_contraindicated_in_botulism must be true (Aminoglycosides block presynaptic acetylcholine release and fatally worsen Botulism neuromuscular blockade)")
	}
	trimmedBy := strings.TrimSpace(stagedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("staged_by is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("tetanus botulism operation %q not found", id)
	}

	now := time.Now().UTC()
	trimmedNote := strings.TrimSpace(woundInfiltrationNote)
	if trimmedNote == "" {
		trimmedNote = "0.1–0.2 mL Equine TAT intradermal skin test negative at 30 min; local perilesional TAT infiltration performed prior to surgical wound debridement + Metronidazole 10 mg/kg IV q12h + Methocarbamol/Midazolam CRI in dark/quiet cotton-earplug isolation."
	}

	patho, penAlert, tatGuard, aminoAlert, isoPlan := BuildClostridialClinicalAlerts(
		op.WeightKg, op.Syndrome, op.BurkittSeverityClass,
		metronidazoleMgKgBID, equineTATUnitsKg, methocarbamolMgKgIV,
		intradermalSkinTestNegativeVerified, penicillinGABAWarningAcknowledged,
		darkQuietCottonEarplugsActive, aminoglycosideContraindicatedInBotulism,
	)

	stg := domain.TetanusBotulismTherapyStaging{
		ID:                                      fmt.Sprintf("CLB-STG-%s-%d", op.ID, len(op.TherapyStagings)+1),
		Protocol:                                protocol,
		MetronidazoleMgKgBID:                    round1(metronidazoleMgKgBID),
		EquineTATUnitsKg:                        round1(equineTATUnitsKg),
		MethocarbamolMgKgIV:                     round1(methocarbamolMgKgIV),
		IntradermalSkinTestNegativeVerified:     intradermalSkinTestNegativeVerified,
		PenicillinGABAWarningAcknowledged:       penicillinGABAWarningAcknowledged,
		DarkQuietCottonEarplugsActive:           darkQuietCottonEarplugsActive,
		AminoglycosideContraindicatedInBotulism: aminoglycosideContraindicatedInBotulism,
		WoundInfiltrationPriorToDebridementNote: trimmedNote,
		PenicillinGABAAntagonismAlert:           penAlert,
		EquineTATIntradermalTestGuard:           tatGuard,
		SensoryIsolationCockpitPlan:             isoPlan,
		StagedBy:                                trimmedBy,
		StagedAt:                                now,
	}

	op.TherapyStagings = append(op.TherapyStagings, stg)
	op.Protocol = protocol
	op.MetronidazoleMgKgBID = stg.MetronidazoleMgKgBID
	op.EquineTATUnitsKg = stg.EquineTATUnitsKg
	op.MethocarbamolMgKgIV = stg.MethocarbamolMgKgIV
	op.IntradermalSkinTestNegativeVerified = intradermalSkinTestNegativeVerified
	op.PenicillinGABAWarningAcknowledged = penicillinGABAWarningAcknowledged
	op.DarkQuietCottonEarplugsActive = darkQuietCottonEarplugsActive
	op.AminoglycosideContraindicatedInBotulism = aminoglycosideContraindicatedInBotulism
	op.PathophysiologySummary = patho
	op.PenicillinGABAAntagonismAlert = penAlert
	op.EquineTATIntradermalTestGuard = tatGuard
	op.AminoglycosideBotulismAlert = aminoAlert
	op.SensoryIsolationCockpitPlan = isoPlan
	op.Status = domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged
	op.Clearance = nil
	op.ClearedBy = ""
	op.ClearanceSummary = ""
	op.ClearedAt = nil
	op.UpdatedAt = now

	cp := cloneClostridialOperation(op)
	return &cp, nil
}

// RecordNeuroCheck logs a serial neurological, autonomic, and ventilatory assessment and transitions
// the operation to SENSORY_ISOLATION_VENTILATION_MONITORED.
func (s *TetanusBotulismService) RecordNeuroCheck(
	id string,
	burkittSeverityClass, spasmFrequencyPerHr int,
	gagReflexIntact bool,
	coreTempCelsius, paCO2mmHg float64,
	heartRateBPM int,
	darkQuietCottonEarplugsActive bool,
	recordedBy, clinicalNote string,
) (*domain.TetanusBotulismOperation, error) {
	if burkittSeverityClass < 1 || burkittSeverityClass > 4 {
		return nil, fmt.Errorf("burkitt_severity_class must be in [1, 4], got %d", burkittSeverityClass)
	}
	if spasmFrequencyPerHr < 0 || spasmFrequencyPerHr > 60 {
		return nil, fmt.Errorf("spasm_frequency_per_hr must be in [0, 60], got %d", spasmFrequencyPerHr)
	}
	if coreTempCelsius < 34.0 || coreTempCelsius > 43.5 {
		return nil, fmt.Errorf("core_temp_celsius must be in [34.0, 43.5], got %v", coreTempCelsius)
	}
	if paCO2mmHg < 15.0 || paCO2mmHg > 110.0 {
		return nil, fmt.Errorf("paco2_mmhg must be in [15.0, 110.0], got %v", paCO2mmHg)
	}
	if heartRateBPM <= 0 {
		heartRateBPM = 96
	}
	if heartRateBPM < 20 || heartRateBPM > 300 {
		return nil, fmt.Errorf("heart_rate_bpm must be in [20, 300], got %d", heartRateBPM)
	}
	trimmedBy := strings.TrimSpace(recordedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("recorded_by is required")
	}
	trimmedNote := strings.TrimSpace(clinicalNote)

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("tetanus botulism operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.TetanusBotulismNeuroCheck{
		ID:                            fmt.Sprintf("CLB-CHK-%s-%d", op.ID, len(op.NeuroChecks)+1),
		BurkittSeverityClass:          burkittSeverityClass,
		SpasmFrequencyPerHr:           spasmFrequencyPerHr,
		GagReflexIntact:               gagReflexIntact,
		CoreTempCelsius:               round1(coreTempCelsius),
		PaCO2mmHg:                     round1(paCO2mmHg),
		HeartRateBPM:                  heartRateBPM,
		DarkQuietCottonEarplugsActive: darkQuietCottonEarplugsActive,
		RecordedBy:                    trimmedBy,
		ClinicalNote:                  trimmedNote,
		RecordedAt:                    now,
	}

	op.NeuroChecks = append(op.NeuroChecks, chk)
	op.BurkittSeverityClass = chk.BurkittSeverityClass
	op.SpasmFrequencyPerHr = chk.SpasmFrequencyPerHr
	op.GagReflexIntact = chk.GagReflexIntact
	op.CoreTempCelsius = chk.CoreTempCelsius
	op.PaCO2mmHg = chk.PaCO2mmHg
	op.HeartRateBPM = chk.HeartRateBPM
	op.DarkQuietCottonEarplugsActive = chk.DarkQuietCottonEarplugsActive
	op.Status = domain.ClostridialStatusSensoryIsolationVentilationMonitored

	patho, penAlert, tatGuard, aminoAlert, isoPlan := BuildClostridialClinicalAlerts(
		op.WeightKg, op.Syndrome, op.BurkittSeverityClass,
		op.MetronidazoleMgKgBID, op.EquineTATUnitsKg, op.MethocarbamolMgKgIV,
		op.IntradermalSkinTestNegativeVerified, op.PenicillinGABAWarningAcknowledged,
		op.DarkQuietCottonEarplugsActive, op.AminoglycosideContraindicatedInBotulism,
	)
	op.PathophysiologySummary = patho
	op.PenicillinGABAAntagonismAlert = penAlert
	op.EquineTATIntradermalTestGuard = tatGuard
	op.AminoglycosideBotulismAlert = aminoAlert
	op.SensoryIsolationCockpitPlan = isoPlan
	op.UpdatedAt = now

	cp := cloneClostridialOperation(op)
	return &cp, nil
}

// EvaluateClearance verifies that tetanic spasms have resolved (spasm_frequency_per_hr == 0),
// pharyngeal gag/swallowing reflex is intact (gag_reflex_intact == true), normocapnia is maintained
// (paco2_mmhg <= 45.0), and ambulatory normothermia is verified (ambulatory_normothermic_verified == true),
// transitioning the operation to NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED.
func (s *TetanusBotulismService) EvaluateClearance(
	id string,
	spasmFrequencyPerHr int,
	gagReflexIntact bool,
	paCO2mmHg float64,
	ambulatoryNormothermicVerified bool,
	clearedBy, clearanceSummary string,
) (*domain.TetanusBotulismOperation, error) {
	if spasmFrequencyPerHr != 0 {
		return nil, fmt.Errorf("spasm_frequency_per_hr must be 0 for clearance, got %d", spasmFrequencyPerHr)
	}
	if !gagReflexIntact {
		return nil, fmt.Errorf("gag_reflex_intact must be true (intact pharyngeal swallowing reflex required before oral intake/discharge)")
	}
	if paCO2mmHg < 15.0 || paCO2mmHg > 45.0 {
		return nil, fmt.Errorf("paco2_mmhg must be in [15.0, 45.0] mmHg (normocapnic spontaneous ventilation required for clearance), got %v", paCO2mmHg)
	}
	if !ambulatoryNormothermicVerified {
		return nil, fmt.Errorf("ambulatory_normothermic_verified must be true for clearance")
	}
	trimmedBy := strings.TrimSpace(clearedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("cleared_by is required")
	}
	trimmedSummary := strings.TrimSpace(clearanceSummary)
	if trimmedSummary == "" {
		return nil, fmt.Errorf("clearance_summary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("tetanus botulism operation %q not found", id)
	}

	now := time.Now().UTC()
	cl := &domain.TetanusBotulismClearance{
		ClearedBy:                      trimmedBy,
		SpasmFrequencyPerHr:            0,
		GagReflexIntact:                true,
		PaCO2mmHg:                      round1(paCO2mmHg),
		AmbulatoryNormothermicVerified: true,
		ClearanceSummary:               trimmedSummary,
		ClearedAt:                      now,
	}

	op.Status = domain.ClostridialStatusNormoreflexicSwallowingIntactCleared
	op.BurkittSeverityClass = 1
	op.SpasmFrequencyPerHr = 0
	op.GagReflexIntact = true
	op.PaCO2mmHg = cl.PaCO2mmHg
	if op.CoreTempCelsius > 39.2 {
		op.CoreTempCelsius = 38.5
	}
	op.Clearance = cl
	op.ClearedBy = trimmedBy
	op.ClearanceSummary = trimmedSummary
	op.ClearedAt = &now
	op.UpdatedAt = now

	cp := cloneClostridialOperation(op)
	return &cp, nil
}

func cloneClostridialOperation(op *domain.TetanusBotulismOperation) domain.TetanusBotulismOperation {
	cp := *op
	if op.TherapyStagings != nil {
		cp.TherapyStagings = make([]domain.TetanusBotulismTherapyStaging, len(op.TherapyStagings))
		copy(cp.TherapyStagings, op.TherapyStagings)
	}
	if op.NeuroChecks != nil {
		cp.NeuroChecks = make([]domain.TetanusBotulismNeuroCheck, len(op.NeuroChecks))
		copy(cp.NeuroChecks, op.NeuroChecks)
	}
	if op.Clearance != nil {
		cl := *op.Clearance
		cp.Clearance = &cl
	}
	if op.ClearedAt != nil {
		t := *op.ClearedAt
		cp.ClearedAt = &t
	}
	return cp
}
