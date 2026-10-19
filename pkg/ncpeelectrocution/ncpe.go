package ncpeelectrocution

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// NCPEService manages Acute Non-Cardiogenic Pulmonary Edema (NCPE), Electrocution (Oral Cord Bite),
// Upper Airway Obstruction / Post-Ictal Neurogenic Edema, Low-Dose Furosemide Controversy &
// CPAP/PEEP Mechanical Ventilation triage operations in a thread-safe store.
type NCPEService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.NCPEElectrocutionOperation
	order []string
	seq   int
}

// NewNCPEService initializes an NCPEService with 3 seeded operations (NCPE-OP-101, NCPE-OP-102, NCPE-OP-103).
func NewNCPEService() *NCPEService {
	s := &NCPEService{
		ops:   make(map[string]*domain.NCPEElectrocutionOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildCaudodorsalNCPEAssessmentPlan computes the clinical interpretation and contraindication guards
// for an Acute Non-Cardiogenic Pulmonary Edema (NCPE) & Electrocution operation.
func BuildCaudodorsalNCPEAssessmentPlan(
	etiologyClass domain.NCPEEtiologyClassification,
	paO2FiO2Ratio, spO2Percent float64,
	respiratoryRateBpm, caudodorsalBLinesCount int,
	la2AoRatio float64,
	oralElectricalEscharPresent, pinkFrothyAirwayFluid bool,
) domain.CaudodorsalNCPEAssessmentPlan {
	var patho string
	switch etiologyClass {
	case domain.NCPEEtiologyOralElectricalCordBite:
		patho = "Oral Electrical Cord Bite & Neurogenic Catecholamine Surge NCPE — High-voltage alternating current (110–240V) across wet oral mucosa triggers massive centrally mediated alpha/beta-adrenergic sympathetic discharge, severe systemic & pulmonary venoconstriction, acute pulmonary capillary hydrostatic pressure spike ('blast injury' to alveolar-capillary endothelium), and high-protein permeability pulmonary edema."
	case domain.NCPEEtiologyPostObstructiveChoking:
		patho = "Post-Obstructive (Negative-Pressure) Non-Cardiogenic Pulmonary Edema — Forced inspiratory effort against an occluded upper airway (Müller maneuver from choking, strangulation, laryngeal paralysis, or brachycephalic crisis) generates extreme negative intrathoracic & interstitial hydrostatic pressure (< -50 to -100 cmH2O),increasing venous return and transvascular fluid filtration into caudodorsal alveoli."
	case domain.NCPEEtiologyPostIctalCatecholamineSurge:
		patho = "Post-Ictal Seizure / Traumatic Brain Injury (TBI) Neurogenic Pulmonary Edema — Acute hypothalamic/medullary sympathetic storm following status epilepticus or intracranial hypertension, causing transient pulmonary capillary hypertension and protein-rich alveolar flooding."
	default:
		patho = "Acute Non-Cardiogenic Pulmonary Edema (NCPE) — Alveolar-capillary membrane permeability disruption with protein-rich caudodorsal alveolar flooding."
	}

	frothNote := "No overt pink frothy airway fluid"
	if pinkFrothyAirwayFluid {
		frothNote = "PINK FROTHY HIGH-PROTEIN AIRWAY FLUID PRESENT"
	}
	pocusNote := fmt.Sprintf("VetBLUE Thoracic POCUS (%d Caudodorsal B-Lines | PaO2/FiO2 %.1f | SpO2 %.1f%% | RR %d bpm | %s) — ", caudodorsalBLinesCount, paO2FiO2Ratio, spO2Percent, respiratoryRateBpm, frothNote)
	if caudodorsalBLinesCount >= 3 {
		pocusNote += "BILATERAL CAUDODORSAL & PERIPHERAL LUNG FIELD B-LINES ('Sheet Rockets' / Wet Lung): Classic distribution of Non-Cardiogenic Permeability Edema (contrasting with perihilar/cranioventral distribution + left atrial enlargement seen in cardiogenic CHF)."
	} else {
		pocusNote += "Caudodorsal lung fields clearing (< 3 B-lines per view, return of dry A-lines and normal glide sign)."
	}

	la2aoNote := fmt.Sprintf("Echocardiographic Left-Atrium-to-Aorta Ratio (LA:Ao = %.2f) — ", la2AoRatio)
	if la2AoRatio < 1.50 {
		la2aoNote += "NORMAL LEFT ATRIAL SIZE (LA:Ao < 1.50) RULES OUT LEFT-SIDED CONGESTIVE HEART FAILURE (Cardiogenic Edema): Confirms primary Non-Cardiogenic Pulmonary Edema (NCPE) where left atrial filling pressures are normal at baseline."
	} else {
		la2aoNote += "Borderline/elevated LA:Ao (>= 1.50) — evaluate for concurrent mitral valve disease or volume overload."
	}

	furosemideGuard := "HIGH-DOSE FUROSEMIDE CONTRAINDICATION GUARD: STRICTLY AVOID aggressive cardiogenic-dose Furosemide diuresis (4–8 mg/kg IV q1–2h or high-dose CRI) in NCPE! Unlike cardiogenic CHF, NCPE is a high-protein permeability edema (exudative alveolar leak) in a normovolemic/euvolemic patient; aggressive loop diuresis cannot clear proteinaceous alveolar fluid and rapidly induces severe hypovolemic shock, prerenal AKI, and hemoconcentration. (If used at all, restrict strictly to a single low-dose 0.5–1.0 mg/kg IV bolus solely for transient non-diuretic bronchodilation/venodilation)."

	escharNote := "No oral electrical burn eschar detected."
	if oralElectricalEscharPresent {
		escharNote = "ORAL ELECTRICAL CORD BITE ESCHAR ALERT (Pale Gray/White Coagulative Necrosis on Labial Commissure / Tongue / Hard Palate): Monitor closely for 5–7 day delayed full-thickness tissue sloughing, lingual arterial hemorrhage, and secondary oronasal/palatal fistula formation; avoid premature surgical debridement until necrotic margins demarcate."
	}

	fluidWarning := "CAPILLARY LEAK FLUID OVERLOAD GUARD: Disrupted alveolar-capillary endothelial tight junctions allow intravenous crystalloids to extravasate directly into the pulmonary interstitium; avoid shock fluid boluses unless overt systemic hypotension is documented, and restrict maintenance IV fluids to 50–75% of standard rates."

	return domain.CaudodorsalNCPEAssessmentPlan{
		EtiologyPathophysiology:            patho,
		CaudodorsalVsPerihilarPOCUSNote:    pocusNote,
		NormalLA2AoCardiogenicRuleOut:      la2aoNote,
		HighDoseFurosemideContraindication: furosemideGuard,
		OralElectricalBurnDelayedNecrosis:  escharNote,
		FluidOverloadCapillaryLeakWarning:  fluidWarning,
	}
}

// BuildDefaultOxygenCPAPVentilationPlan returns an evidence-based OxygenCPAPVentilationPlan.
func BuildDefaultOxygenCPAPVentilationPlan(
	weightKg float64,
	etiologyClass domain.NCPEEtiologyClassification,
	paO2FiO2Ratio float64,
	oralElectricalEscharPresent bool,
) domain.OxygenCPAPVentilationPlan {
	butorphanolMg := round2(weightKg * 0.2)
	lowFurosemideMg := round2(weightKg * 0.5)
	fluidMlHr := round1(weightKg * 1.5)

	oxygenPlan := fmt.Sprintf("Temperature/Humidity-Controlled Oxygen Cage (FiO2 40–60%%) or High-Flow Nasal Cannula (HFNC at 1–2 L/kg/min warm humidified O2) for %.1f kg patient; minimize stressful restraint and handling.", weightKg)

	anxiolyticPlan := fmt.Sprintf("Butorphanol 0.2 mg/kg IV/IM (%.2f mg) q4–6h +/- low-dose Acepromazine 0.01–0.02 mg/kg IV/IM for anxiolysis and respiratory center modulation — breaks panicked tachypnea and prevents negative-pressure inspiratory Müller maneuvers.", butorphanolMg)

	furosemideNote := fmt.Sprintf("WITHHOLD high-dose Furosemide (4–8 mg/kg CONTRAINDICATED); optional single low-dose Furosemide 0.5–1.0 mg/kg IV (%.2f mg once) ONLY for transient prostaglandin-mediated bronchodilation and pulmonary venodilation if normovolemic.", lowFurosemideMg)

	cpapPlan := fmt.Sprintf("PaO2/FiO2 %.1f — Maintain HFNC / Helmet CPAP (5 cmH2O) readiness; escalate to Positive End-Expiratory Pressure (PEEP 5–10 cmH2O) mechanical ventilation if PaO2/FiO2 drops < 200 or SpO2 < 90%% despite FiO2 60%%.", paO2FiO2Ratio)
	if paO2FiO2Ratio < 200.0 {
		cpapPlan = fmt.Sprintf("SEVERE HYPOXEMIA (PaO2/FiO2 %.1f < 200): Initiate Helmet CPAP (5–8 cmH2O) or Lung-Protective Mechanical Ventilation with PEEP 5–10 cmH2O and low tidal volume (6–8 mL/kg) to recruit collapsed caudodorsal alveoli and prevent atelectrauma.", paO2FiO2Ratio)
	}

	fluidPlan := fmt.Sprintf("Conservative Euvolemic Fluid Plan: Restrict isotonic crystalloid maintenance to %.1f mL/hr (~1.5 mL/kg/hr) to prevent exacerbating alveolar capillary leak while preserving renal perfusion.", fluidMlHr)

	oralPlan := "No oral electrical burn present; standard airway and mucosal care."
	if oralElectricalEscharPresent || etiologyClass == domain.NCPEEtiologyOralElectricalCordBite {
		oralPlan = "Oral Commissure & Lingual Electrical Burn Care: Methadone/Buprenorphine opioid analgesia, 0.12% Chlorhexidine oral rinse, Sucralfate slurry, soft gruel/esophagostomy nutrition, and 5–7 day delayed eschar sloughing / oronasal fistula surveillance."
	}

	return domain.OxygenCPAPVentilationPlan{
		OxygenFiO2OrHFNCProtocol:            oxygenPlan,
		AnxiolyticButorphanolAcepromazine:   anxiolyticPlan,
		LowDoseFurosemideBronchodilatorNote: furosemideNote,
		CPAPOrPEEPMechanicalVentilation:     cpapPlan,
		ConservativeFluidMaintenancePlan:    fluidPlan,
		OralCommissureBurnAnalgesiaPlan:     oralPlan,
	}
}

func (s *NCPEService) seed() {
	now := time.Now().UTC()

	// NCPE-OP-101: Sparky — Canine — Golden Retriever Puppy, 6.4 kg, ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE
	assess101 := BuildCaudodorsalNCPEAssessmentPlan(
		domain.NCPEEtiologyOralElectricalCordBite,
		165.0, 86.0, 68, 14, 1.22,
		true, true,
	)
	vent101 := BuildDefaultOxygenCPAPVentilationPlan(
		6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, true,
	)
	op101 := &domain.NCPEElectrocutionOperation{
		ID:                          "NCPE-OP-101",
		CaseID:                      "CASE-NCPE-101",
		PetName:                     "Sparky",
		Species:                     "Canine",
		Breed:                       "Golden Retriever Puppy",
		WeightKg:                    6.4,
		Status:                      domain.NCPEStatusAcuteCaudodorsalTriage,
		EtiologyClass:               domain.NCPEEtiologyOralElectricalCordBite,
		PaO2FiO2Ratio:               165.0,
		SpO2Percent:                 86.0,
		RespiratoryRateBpm:          68,
		CaudodorsalBLinesCount:      14,
		LA2AoRatio:                  1.22,
		OralElectricalEscharPresent: true,
		PinkFrothyAirwayFluid:       true,
		AssessmentPlan:              assess101,
		VentilationPlan:             vent101,
		RespiratoryChecks: []domain.NCPERespiratoryCheck{
			{
				ID:                     "NCPE-CHK-101-1",
				PaO2FiO2Ratio:          180.0,
				SpO2Percent:            88.0,
				RespiratoryRateBpm:     64,
				CaudodorsalBLinesCount: 12,
				LA2AoRatio:             1.22,
				RecordedBy:             "Dr. Nadia Varma, DACVECC",
				ClinicalNote:           "Presented 45 min after biting 120V lamp electrical cord: pale gray coagulative eschar at left oral commissure and lateral tongue, orthopnea, RR 64 bpm, bilateral caudodorsal coalescent B-lines (12), normal LA:Ao 1.22 ruling out cardiogenic edema.",
				RecordedAt:             now.Add(-50 * time.Minute),
			},
			{
				ID:                     "NCPE-CHK-101-2",
				PaO2FiO2Ratio:          165.0,
				SpO2Percent:            86.0,
				RespiratoryRateBpm:     68,
				CaudodorsalBLinesCount: 14,
				LA2AoRatio:             1.22,
				RecordedBy:             "Dr. Nadia Varma, DACVECC",
				ClinicalNote:           "Progressive neurogenic NCPE with pink frothy airway fluid, SpO2 86%, PaO2/FiO2 165, 14 caudodorsal B-lines; withholding high-dose furosemide, administering Butorphanol 0.2 mg/kg IV and staging HFNC / Helmet CPAP (PEEP 6 cmH2O).",
				RecordedAt:             now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// NCPE-OP-102: Milo — Canine — Pug, 8.9 kg, OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED
	assess102 := BuildCaudodorsalNCPEAssessmentPlan(
		domain.NCPEEtiologyPostObstructiveChoking,
		235.0, 93.0, 42, 7, 1.18,
		false, false,
	)
	vent102 := BuildDefaultOxygenCPAPVentilationPlan(
		8.9, domain.NCPEEtiologyPostObstructiveChoking, 235.0, false,
	)
	op102 := &domain.NCPEElectrocutionOperation{
		ID:                          "NCPE-OP-102",
		CaseID:                      "CASE-NCPE-102",
		PetName:                     "Milo",
		Species:                     "Canine",
		Breed:                       "Pug",
		WeightKg:                    8.9,
		Status:                      domain.NCPEStatusOxygenCPAPStaged,
		EtiologyClass:               domain.NCPEEtiologyPostObstructiveChoking,
		PaO2FiO2Ratio:               235.0,
		SpO2Percent:                 93.0,
		RespiratoryRateBpm:          42,
		CaudodorsalBLinesCount:      7,
		LA2AoRatio:                  1.18,
		OralElectricalEscharPresent: false,
		PinkFrothyAirwayFluid:       false,
		AssessmentPlan:              assess102,
		VentilationPlan:             vent102,
		RespiratoryChecks: []domain.NCPERespiratoryCheck{
			{
				ID:                     "NCPE-CHK-102-1",
				PaO2FiO2Ratio:          235.0,
				SpO2Percent:            93.0,
				RespiratoryRateBpm:     42,
				CaudodorsalBLinesCount: 7,
				LA2AoRatio:             1.18,
				RecordedBy:             "Dr. Nadia Varma, DACVECC",
				ClinicalNote:           "Post-obstructive negative-pressure NCPE following pharyngeal ball foreign body extraction and BOAS crisis: Butorphanol 0.2 mg/kg IV + HFNC 1.5 L/kg/min staged, SpO2 improved to 93%, LA:Ao 1.18 normal.",
				RecordedAt:             now.Add(-75 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-75 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// NCPE-OP-103: Luna — Feline — Siamese, 3.8 kg, BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED
	assess103 := BuildCaudodorsalNCPEAssessmentPlan(
		domain.NCPEEtiologyPostIctalCatecholamineSurge,
		410.0, 98.0, 26, 0, 1.15,
		false, false,
	)
	vent103 := BuildDefaultOxygenCPAPVentilationPlan(
		3.8, domain.NCPEEtiologyPostIctalCatecholamineSurge, 410.0, false,
	)
	dischargedAt := now.Add(-25 * time.Minute)
	op103 := &domain.NCPEElectrocutionOperation{
		ID:                          "NCPE-OP-103",
		CaseID:                      "CASE-NCPE-103",
		PetName:                     "Luna",
		Species:                     "Feline",
		Breed:                       "Siamese",
		WeightKg:                    3.8,
		Status:                      domain.NCPEStatusBLinesResolvedCleared,
		EtiologyClass:               domain.NCPEEtiologyPostIctalCatecholamineSurge,
		PaO2FiO2Ratio:               410.0,
		SpO2Percent:                 98.0,
		RespiratoryRateBpm:          26,
		CaudodorsalBLinesCount:      0,
		LA2AoRatio:                  1.15,
		OralElectricalEscharPresent: false,
		PinkFrothyAirwayFluid:       false,
		AssessmentPlan:              assess103,
		VentilationPlan:             vent103,
		RespiratoryChecks: []domain.NCPERespiratoryCheck{
			{
				ID:                     "NCPE-CHK-103-1",
				PaO2FiO2Ratio:          220.0,
				SpO2Percent:            91.0,
				RespiratoryRateBpm:     52,
				CaudodorsalBLinesCount: 8,
				LA2AoRatio:             1.15,
				RecordedBy:             "Dr. Nadia Varma, DACVECC",
				ClinicalNote:           "Post-ictal neurogenic pulmonary edema following cluster seizures: 8 caudodorsal B-lines, LA:Ao 1.15, managed in 50% O2 cage with Levetiracetam and Butorphanol anxiolysis.",
				RecordedAt:             now.Add(-24 * time.Hour),
			},
			{
				ID:                     "NCPE-CHK-103-2",
				PaO2FiO2Ratio:          410.0,
				SpO2Percent:            98.0,
				RespiratoryRateBpm:     26,
				CaudodorsalBLinesCount: 0,
				LA2AoRatio:             1.15,
				RecordedBy:             "Dr. Nadia Varma, DACVECC",
				ClinicalNote:           "Complete resolution of caudodorsal B-lines (0 B-lines, crisp dry A-lines), eupneic on room air (FiO2 21%, SpO2 98%, RR 26 bpm, PaO2/FiO2 410).",
				RecordedAt:             now.Add(-40 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Nadia Varma, DACVECC",
		DischargeSummary: "Caudodorsal VetBLUE B-lines completely resolved (0 B-lines), normoxemic and eupneic on room air (SpO2 98%, RR 26 bpm, PaO2/FiO2 410, LA:Ao 1.15); cleared for discharge.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-28 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of NCPE operations, optionally filtered by statusFilter.
func (s *NCPEService) ListOperations(statusFilter string) []domain.NCPEElectrocutionOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.NCPEElectrocutionOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneNCPEOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an NCPE operation by ID.
func (s *NCPEService) GetOperation(id string) (*domain.NCPEElectrocutionOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("ncpe electrocution operation %q not found", id)
	}
	cp := cloneNCPEOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all NCPE & Electrocution operations.
func (s *NCPEService) GetOverviewMetrics() domain.NCPEOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.NCPEOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.NCPEStatusAcuteCaudodorsalTriage:
			m.AcuteNCPETriageCount++
		case domain.NCPEStatusOxygenCPAPStaged:
			m.CPAPOxygenStagedCount++
		case domain.NCPEStatusBLinesResolvedCleared:
			m.ClearedRoomAirCount++
		}
		if op.EtiologyClass == domain.NCPEEtiologyOralElectricalCordBite {
			m.OralCordElectrocutionCount++
		}
		if op.PaO2FiO2Ratio < 200.0 {
			m.SeverePaO2FiO2Under200Count++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Non-Cardiogenic Pulmonary Edema (NCPE) / Electrocution triage operation.
func (s *NCPEService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	etiologyClass domain.NCPEEtiologyClassification,
	paO2FiO2Ratio, spO2Percent float64,
	respiratoryRateBpm, caudodorsalBLinesCount int,
	la2AoRatio float64,
	oralElectricalEscharPresent, pinkFrothyAirwayFluid bool,
) (*domain.NCPEElectrocutionOperation, error) {
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
	if !etiologyClass.IsValid() {
		return nil, fmt.Errorf("invalid etiology_class: %q", etiologyClass)
	}
	if paO2FiO2Ratio <= 0 || paO2FiO2Ratio > 600.0 {
		return nil, fmt.Errorf("pao2_fio2_ratio must be in (0, 600.0], got %v", paO2FiO2Ratio)
	}
	if spO2Percent < 40.0 || spO2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [40.0, 100.0], got %v", spO2Percent)
	}
	if respiratoryRateBpm <= 0 || respiratoryRateBpm > 200 {
		return nil, fmt.Errorf("respiratory_rate_bpm must be in (0, 200], got %d", respiratoryRateBpm)
	}
	if caudodorsalBLinesCount < 0 || caudodorsalBLinesCount > 50 {
		return nil, fmt.Errorf("caudodorsal_blines_count must be in [0, 50], got %d", caudodorsalBLinesCount)
	}
	if la2AoRatio <= 0 || la2AoRatio > 4.0 {
		return nil, fmt.Errorf("la2ao_ratio must be in (0, 4.0], got %v", la2AoRatio)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("NCPE-OP-%d", s.seq)
	now := time.Now().UTC()

	assessPlan := BuildCaudodorsalNCPEAssessmentPlan(
		etiologyClass,
		paO2FiO2Ratio, spO2Percent,
		respiratoryRateBpm, caudodorsalBLinesCount,
		la2AoRatio,
		oralElectricalEscharPresent, pinkFrothyAirwayFluid,
	)
	ventPlan := BuildDefaultOxygenCPAPVentilationPlan(
		weightKg, etiologyClass, paO2FiO2Ratio, oralElectricalEscharPresent,
	)

	op := &domain.NCPEElectrocutionOperation{
		ID:                          opID,
		CaseID:                      trimmedCase,
		PetName:                     trimmedPet,
		Species:                     trimmedSpecies,
		Breed:                       trimmedBreed,
		WeightKg:                    round1(weightKg),
		Status:                      domain.NCPEStatusAcuteCaudodorsalTriage,
		EtiologyClass:               etiologyClass,
		PaO2FiO2Ratio:               round1(paO2FiO2Ratio),
		SpO2Percent:                 round1(spO2Percent),
		RespiratoryRateBpm:          respiratoryRateBpm,
		CaudodorsalBLinesCount:      caudodorsalBLinesCount,
		LA2AoRatio:                  round2(la2AoRatio),
		OralElectricalEscharPresent: oralElectricalEscharPresent,
		PinkFrothyAirwayFluid:       pinkFrothyAirwayFluid,
		AssessmentPlan:              assessPlan,
		VentilationPlan:             ventPlan,
		RespiratoryChecks:           []domain.NCPERespiratoryCheck{},
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneNCPEOperation(op)
	return &cp, nil
}

// StageCPAPVentilation stages Oxygen/HFNC protocol, Butorphanol/Acepromazine anxiolysis,
// low-dose furosemide bronchodilator note (avoiding 4-8 mg/kg diuresis), CPAP/PEEP mechanical
// ventilation protocol, conservative fluid maintenance, and oral burn analgesia,
// transitioning status to OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED.
func (s *NCPEService) StageCPAPVentilation(
	id, oxygenProtocol, anxiolyticPlan, lowDoseFurosemideNote, cpapPeepPlan, conservativeFluidPlan, oralBurnAnalgesia string,
) (*domain.NCPEElectrocutionOperation, error) {
	trimmedOxygen := strings.TrimSpace(oxygenProtocol)
	if trimmedOxygen == "" {
		return nil, fmt.Errorf("oxygen_fio2_or_hfnc_protocol is required")
	}
	trimmedAnxiolytic := strings.TrimSpace(anxiolyticPlan)
	if trimmedAnxiolytic == "" {
		return nil, fmt.Errorf("anxiolytic_butorphanol_acepromazine is required")
	}
	trimmedCPAP := strings.TrimSpace(cpapPeepPlan)
	if trimmedCPAP == "" {
		return nil, fmt.Errorf("cpap_or_peep_mechanical_ventilation is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("ncpe electrocution operation %q not found", id)
	}

	defaults := BuildDefaultOxygenCPAPVentilationPlan(
		op.WeightKg, op.EtiologyClass, op.PaO2FiO2Ratio, op.OralElectricalEscharPresent,
	)
	trimmedFurosemide := strings.TrimSpace(lowDoseFurosemideNote)
	if trimmedFurosemide == "" {
		trimmedFurosemide = defaults.LowDoseFurosemideBronchodilatorNote
	}
	trimmedFluid := strings.TrimSpace(conservativeFluidPlan)
	if trimmedFluid == "" {
		trimmedFluid = defaults.ConservativeFluidMaintenancePlan
	}
	trimmedOral := strings.TrimSpace(oralBurnAnalgesia)
	if trimmedOral == "" {
		trimmedOral = defaults.OralCommissureBurnAnalgesiaPlan
	}

	op.VentilationPlan = domain.OxygenCPAPVentilationPlan{
		OxygenFiO2OrHFNCProtocol:            trimmedOxygen,
		AnxiolyticButorphanolAcepromazine:   trimmedAnxiolytic,
		LowDoseFurosemideBronchodilatorNote: trimmedFurosemide,
		CPAPOrPEEPMechanicalVentilation:     trimmedCPAP,
		ConservativeFluidMaintenancePlan:    trimmedFluid,
		OralCommissureBurnAnalgesiaPlan:     trimmedOral,
	}
	op.Status = domain.NCPEStatusOxygenCPAPStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneNCPEOperation(op)
	return &cp, nil
}

// RecordRespiratoryCheck appends an NCPERespiratoryCheck and updates op.PaO2FiO2Ratio,
// op.SpO2Percent, op.RespiratoryRateBpm, op.CaudodorsalBLinesCount, and op.LA2AoRatio.
func (s *NCPEService) RecordRespiratoryCheck(
	id string,
	paO2FiO2Ratio, spO2Percent float64,
	respiratoryRateBpm, caudodorsalBLinesCount int,
	la2AoRatio float64,
	recordedBy, clinicalNote string,
) (*domain.NCPEElectrocutionOperation, error) {
	if paO2FiO2Ratio <= 0 || paO2FiO2Ratio > 600.0 {
		return nil, fmt.Errorf("pao2_fio2_ratio must be in (0, 600.0], got %v", paO2FiO2Ratio)
	}
	if spO2Percent < 40.0 || spO2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [40.0, 100.0], got %v", spO2Percent)
	}
	if respiratoryRateBpm <= 0 || respiratoryRateBpm > 200 {
		return nil, fmt.Errorf("respiratory_rate_bpm must be in (0, 200], got %d", respiratoryRateBpm)
	}
	if caudodorsalBLinesCount < 0 || caudodorsalBLinesCount > 50 {
		return nil, fmt.Errorf("caudodorsal_blines_count must be in [0, 50], got %d", caudodorsalBLinesCount)
	}
	if la2AoRatio <= 0 || la2AoRatio > 4.0 {
		return nil, fmt.Errorf("la2ao_ratio must be in (0, 4.0], got %v", la2AoRatio)
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
		return nil, fmt.Errorf("ncpe electrocution operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.NCPERespiratoryCheck{
		ID:                     fmt.Sprintf("NCPE-CHK-%s-%d", op.ID, len(op.RespiratoryChecks)+1),
		PaO2FiO2Ratio:          round1(paO2FiO2Ratio),
		SpO2Percent:            round1(spO2Percent),
		RespiratoryRateBpm:     respiratoryRateBpm,
		CaudodorsalBLinesCount: caudodorsalBLinesCount,
		LA2AoRatio:             round2(la2AoRatio),
		RecordedBy:             trimmedBy,
		ClinicalNote:           trimmedNote,
		RecordedAt:             now,
	}

	op.RespiratoryChecks = append(op.RespiratoryChecks, chk)
	op.PaO2FiO2Ratio = chk.PaO2FiO2Ratio
	op.SpO2Percent = chk.SpO2Percent
	op.RespiratoryRateBpm = chk.RespiratoryRateBpm
	op.CaudodorsalBLinesCount = chk.CaudodorsalBLinesCount
	op.LA2AoRatio = chk.LA2AoRatio

	op.AssessmentPlan = BuildCaudodorsalNCPEAssessmentPlan(
		op.EtiologyClass,
		op.PaO2FiO2Ratio, op.SpO2Percent,
		op.RespiratoryRateBpm, op.CaudodorsalBLinesCount,
		op.LA2AoRatio,
		op.OralElectricalEscharPresent, op.PinkFrothyAirwayFluid,
	)
	op.UpdatedAt = now

	cp := cloneNCPEOperation(op)
	return &cp, nil
}

// ClearNCPEDischarge transitions the operation to BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED
// once caudodorsal B-lines resolve and the patient is normoxemic and eupneic on room air.
func (s *NCPEService) ClearNCPEDischarge(id, dischargedBy, summary string) (*domain.NCPEElectrocutionOperation, error) {
	trimmedBy := strings.TrimSpace(dischargedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("discharged_by is required")
	}
	trimmedSummary := strings.TrimSpace(summary)
	if trimmedSummary == "" {
		return nil, fmt.Errorf("discharge_summary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("ncpe electrocution operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.NCPEStatusBLinesResolvedCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneNCPEOperation(op)
	return &cp, nil
}

func cloneNCPEOperation(op *domain.NCPEElectrocutionOperation) domain.NCPEElectrocutionOperation {
	cp := *op
	if op.RespiratoryChecks != nil {
		cp.RespiratoryChecks = make([]domain.NCPERespiratoryCheck, len(op.RespiratoryChecks))
		copy(cp.RespiratoryChecks, op.RespiratoryChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
