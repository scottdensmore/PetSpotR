package fatethrombus

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// FATEService manages Feline Aortic Thromboembolism (FATE / Saddle Thrombus),
// 5 Ps (Pain, Pallor, Pulselessness, Paresis, Poikilothermy), Paired Glucose/Lactate
// Limb Gradient & Clopidogrel/Rivaroxaban triage operations in a thread-safe store.
type FATEService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.FATEThrombusOperation
	order []string
	seq   int
}

// NewFATEService initializes a FATEService with 3 seeded operations (FATE-OP-101, FATE-OP-102, FATE-OP-103).
func NewFATEService() *FATEService {
	s := &FATEService{
		ops:   make(map[string]*domain.FATEThrombusOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildMetabolicGradientPlan computes the 5 Ps clinical summary, systemic-to-limb glucose delta,
// limb-to-systemic lactate gradient, LA:Ao ratio / left atrial smoke summary, reperfusion hyperkalemia
// washout alert, and passive warming contraindication.
func BuildMetabolicGradientPlan(
	distType domain.ThrombusDistributionType,
	sysGlucose, limbGlucose, glucoseDelta float64,
	sysLactate, limbLactate, lactateDelta float64,
	potassium, limbTempC, la2ao float64,
	concurrentCHF bool,
) domain.MetabolicGradientPlan {
	var fivePs string
	switch distType {
	case domain.ThrombusDistBilateralAorticSaddle:
		fivePs = "Bilateral Aortic Trifurcation Saddle Thrombus — Acute 5 Ps: Vocalizing Ischemic Pain, Cyanotic/Pale Nail Beds (Pallor), Absent Bilateral Femoral/Dorsal Pedal Pulses (Pulselessness), Bilateral Pelvic Limb Lower Motor Neuron Paresis/Paralysis & Firm Cranial Tibialis/Gastrocnemius Gastrocnemius Myopathy, Poikilothermy."
	case domain.ThrombusDistUnilateralIliacFemoral:
		fivePs = "Unilateral External Iliac / Femoral Thromboembolism — Asymmetric Pelvic Limb 5 Ps: Acute Vocalizing Pain, Unilateral Pale/Cyanotic Paw Pads (Pallor), Unilateral Absent Femoral Pulse (Pulselessness), Monoparesis (Paresis) & Distal Limb Poikilothermy (70–90% survival prognosis)."
	case domain.ThrombusDistRightForeBrachial:
		fivePs = "Right Forelimb Brachial Artery Thromboembolism — Right Thoracic Limb 5 Ps: Acute Forelimb Pain, Pale Digital Pads, Absent Carpal/Metacarpal Doppler Flow, Right Forelimb Monoparesis & Distal Poikilothermy."
	default:
		fivePs = "Acute Arterial Thromboembolism — 5 Ps: Pain, Pallor, Pulselessness, Paresis, Poikilothermy."
	}

	glucoseDiagnostic := "Sub-threshold (< 30 mg/dL) or Reperfusing"
	if glucoseDelta >= 30.0 {
		glucoseDiagnostic = "DIAGNOSTIC FOR FATE (> 30 mg/dL Systemic-to-Affected-Limb Glucose Gradient)"
	}
	glucoseSummary := fmt.Sprintf("Systemic %.1f mg/dL vs Affected Limb %.1f mg/dL → ΔGlucose = +%.1f mg/dL (%s).", sysGlucose, limbGlucose, glucoseDelta, glucoseDiagnostic)

	lactateSummary := fmt.Sprintf("Affected Limb %.1f mmol/L vs Systemic %.1f mmol/L → ΔLactate = +%.1f mmol/L (Anaerobic ischemic myonecrosis washout index).", limbLactate, sysLactate, lactateDelta)

	chfLabel := "No acute pulmonary edema"
	if concurrentCHF {
		chfLabel = "CONCURRENT CARDIOGENIC PULMONARY EDEMA / PLEURAL EFFUSION PRESENT"
	}
	laSummary := fmt.Sprintf("LA:Ao Ratio %.2f (> 1.6 indicates severe left atrial enlargement; > 2.0 high FATE risk) with spontaneous echocardiographic contrast ('smoke') — %s.", la2ao, chfLabel)

	hyperkalemiaAlert := fmt.Sprintf("Serum K+ %.1f mmol/L (Distal Limb Temp %.1f°C): Monitor ECG & serial K+ q2–4h during limb reperfusion; ischemic rhabdomyolysis washout releases intracellular K+, H+, and myoglobin causing sudden lethal hyperkalemic bradycardia/asystole (prepare 10%% Calcium Gluconate 0.5–1.0 mL/kg slow IV + Regular Insulin 0.25 U/kg + Dextrose 0.5 g/kg).", potassium, limbTempC)

	passiveWarming := "CONTRAINDICATION GUARD: NEVER apply direct heating pads, hot water bottles, or forced active external heat to ischemic/poikilothermic limbs (T = " + fmt.Sprintf("%.1f°C", limbTempC) + ") — absent arterial perfusion prevents heat dissipation and accelerates cellular metabolic oxygen demand, causing full-thickness thermal burns and rapid ischemic myonecrosis. Use passive ambient room/blanket warming only."

	return domain.MetabolicGradientPlan{
		FivePsClinicalSummary:          fivePs,
		SystemicToLimbGlucoseDelta:     glucoseSummary,
		LimbToSystemicLactateDelta:     lactateSummary,
		LeftAtrialSmokeLA2AoRatio:      laSummary,
		ReperfusionHyperkalemiaAlert:   hyperkalemiaAlert,
		PassiveWarmingContraindication: passiveWarming,
	}
}

// BuildDefaultAntithromboticPlan returns an evidence-based AntithromboticAnalgesiaPlan.
func BuildDefaultAntithromboticPlan(weightKg float64, concurrentCHF bool) domain.AntithromboticAnalgesiaPlan {
	methadoneDoseMg := round1(weightKg * 0.2)
	fentanylBolusMcg := round1(weightKg * 3.0)
	furosemideDoseMg := round1(weightKg * 2.0)
	enoxaparinDoseMg := round1(weightKg * 1.0)

	chfPlan := "Euvolemic non-CHF patient: Avoid aggressive IV crystalloid overload; maintain conservative maintenance hydration only after ruling out left atrial congestion."
	if concurrentCHF {
		chfPlan = fmt.Sprintf("Concurrent Cardiogenic CHF: Furosemide 1–2 mg/kg IV/IM (%.1f mg for %.1f kg cat) + Oxygen Cage (FiO2 40–50%%) + Thoracocentesis if pleural effusion; STRICTLY AVOID IV crystalloid fluid boluses.", furosemideDoseMg, weightKg)
	}

	return domain.AntithromboticAnalgesiaPlan{
		FullMuOpioidCRIProtocol:     fmt.Sprintf("Full-Mu Opioid Priority: Methadone 0.2–0.3 mg/kg IV/IM q4h (%.1f mg) OR Fentanyl 2–5 mcg/kg IV bolus (%.1f mcg) followed by 2–5 mcg/kg/hr CRI (NEVER rely on partial mu-agonist Buprenorphine alone during first 24–48h of severe ischemic neuritis).", methadoneDoseMg, fentanylBolusMcg),
		ClopidogrelLoadingProtocol:  "FAT CAT Trial Protocol: Clopidogrel 75 mg PO one-time oral loading dose (4 × 18.75 mg once nausea/vomiting controlled) to achieve rapid platelet P2Y12 inhibition within 2 hours, followed by 18.75 mg PO q24h maintenance.",
		FactorXaOrHeparinProtocol:   fmt.Sprintf("Dual Antithrombotic Therapy: Enoxaparin (LMWH) 0.8–1.0 mg/kg SC q6–8h (%.1f mg SC) acutely, transitioning to Rivaroxaban (Direct Factor Xa Inhibitor) 2.5 mg PO q24h + Clopidogrel 18.75 mg PO q24h.", enoxaparinDoseMg),
		ConcurrentCHFFurosemidePlan: chfPlan,
		SerialPotassiumMonitoring:   "Serial Venous Blood Gas & Electrolyte Panel (K+, Lactate, pH, iCa++) q2–4h with continuous lead-II ECG telemetry for reperfusion hyperkalemia (tall tented T-waves, absent P-waves, sinoventricular bradycardia).",
		PimobendanLVOTNote:          "Echocardiographic LVOT Guard: Verify absence of Dynamic Left Ventricular Outflow Tract Obstruction (DLVOTO / SAM of mitral valve) before initiating Pimobendan 1.25 mg PO q12h.",
	}
}

func (s *FATEService) seed() {
	now := time.Now().UTC()

	// FATE-OP-101: Jasper — Acute Saddle Thrombus 5Ps Triage
	grad101 := BuildMetabolicGradientPlan(
		domain.ThrombusDistBilateralAorticSaddle,
		185.0, 92.0, 93.0,
		2.4, 11.8, 9.4,
		4.6, 29.2, 2.35,
		true,
	)
	anti101 := BuildDefaultAntithromboticPlan(6.8, true)
	op101 := &domain.FATEThrombusOperation{
		ID:                       "FATE-OP-101",
		CaseID:                   "CASE-FATE-101",
		PetName:                  "Jasper",
		Species:                  "Feline",
		Breed:                    "Maine Coon",
		WeightKg:                 6.8,
		Status:                   domain.FATEStatusAcuteSaddle5PsTriage,
		DistributionType:         domain.ThrombusDistBilateralAorticSaddle,
		SystemicGlucoseMgDL:      185.0,
		AffectedLimbGlucoseMgDL:  92.0,
		GlucoseDeltaMgDL:         93.0,
		SystemicLactateMmolL:     2.4,
		AffectedLimbLactateMmolL: 11.8,
		LactateDeltaMmolL:        9.4,
		SerumPotassiumMmolL:      4.6,
		DistalLimbTempC:          29.2,
		LA2AoRatio:               2.35,
		ConcurrentCardiogenicCHF: true,
		GradientPlan:             grad101,
		AntithromboticPlan:       anti101,
		PerfusionChecks: []domain.LimbPerfusionCheck{
			{
				ID:                  "FATE-CHK-101-1",
				GlucoseDeltaMgDL:    93.0,
				LactateDeltaMmolL:   9.4,
				SerumPotassiumMmolL: 4.3,
				DistalLimbTempC:     28.8,
				FemoralPulseStatus:  "ABSENT_BILATERAL_DOPPLER_SILENT",
				RecordedBy:          "Dr. Maya Lin, DVM, DACVECC",
				ClinicalNote:        "Presentation triage: acute vocalizing hindlimb paralysis, cyanotic pads, paired systemic-to-limb glucose delta 93.0 mg/dL (> 30 mg/dL diagnostic); Methadone 0.25 mg/kg IV + Oxygen Cage 45% + Furosemide 2 mg/kg IV administered.",
				RecordedAt:          now.Add(-45 * time.Minute),
			},
			{
				ID:                  "FATE-CHK-101-2",
				GlucoseDeltaMgDL:    93.0,
				LactateDeltaMmolL:   9.4,
				SerumPotassiumMmolL: 4.6,
				DistalLimbTempC:     29.2,
				FemoralPulseStatus:  "ABSENT_BILATERAL_GASTROCNEMIUS_FIRM",
				RecordedBy:          "Dr. Maya Lin, DVM, DACVECC",
				ClinicalNote:        "POCUS TFAST/Echo confirms LA:Ao 2.35 with dense left atrial spontaneous echo contrast ('smoke'); direct heating pads strictly prohibited on ischemic pelvic limbs.",
				RecordedAt:          now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-50 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// FATE-OP-102: Cleo — Methadone, Clopidogrel 75 mg Loading & Anticoagulant Staged
	grad102 := BuildMetabolicGradientPlan(
		domain.ThrombusDistUnilateralIliacFemoral,
		148.0, 104.0, 44.0,
		2.1, 6.5, 4.4,
		5.1, 34.1, 1.95,
		false,
	)
	anti102 := BuildDefaultAntithromboticPlan(4.9, false)
	op102 := &domain.FATEThrombusOperation{
		ID:                       "FATE-OP-102",
		CaseID:                   "CASE-FATE-102",
		PetName:                  "Cleo",
		Species:                  "Feline",
		Breed:                    "Ragdoll",
		WeightKg:                 4.9,
		Status:                   domain.FATEStatusMethadoneClopidogrelStaged,
		DistributionType:         domain.ThrombusDistUnilateralIliacFemoral,
		SystemicGlucoseMgDL:      148.0,
		AffectedLimbGlucoseMgDL:  104.0,
		GlucoseDeltaMgDL:         44.0,
		SystemicLactateMmolL:     2.1,
		AffectedLimbLactateMmolL: 6.5,
		LactateDeltaMmolL:        4.4,
		SerumPotassiumMmolL:      5.1,
		DistalLimbTempC:          34.1,
		LA2AoRatio:               1.95,
		ConcurrentCardiogenicCHF: false,
		GradientPlan:             grad102,
		AntithromboticPlan:       anti102,
		PerfusionChecks: []domain.LimbPerfusionCheck{
			{
				ID:                  "FATE-CHK-102-1",
				GlucoseDeltaMgDL:    44.0,
				LactateDeltaMmolL:   4.4,
				SerumPotassiumMmolL: 5.1,
				DistalLimbTempC:     34.1,
				FemoralPulseStatus:  "LEFT_FEMORAL_FAINT_DOPPLER_AUDIBLE",
				RecordedBy:          "Dr. Julian Vance, DACVIM (Cardiology)",
				ClinicalNote:        "Fentanyl CRI 3 mcg/kg/hr + Clopidogrel 75 mg PO loading dose + Enoxaparin 0.9 mg/kg SC q8h staged; mild K+ rise to 5.1 mmol/L during early collateral reperfusion.",
				RecordedAt:          now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// FATE-OP-103: Figaro — Reperfused, Normokalemic & Dual Therapy Cleared
	grad103 := BuildMetabolicGradientPlan(
		domain.ThrombusDistRightForeBrachial,
		118.0, 110.0, 8.0,
		1.6, 1.9, 0.3,
		4.1, 37.6, 1.78,
		false,
	)
	anti103 := BuildDefaultAntithromboticPlan(5.2, false)
	dischargedAt := now.Add(-30 * time.Minute)
	op103 := &domain.FATEThrombusOperation{
		ID:                       "FATE-OP-103",
		CaseID:                   "CASE-FATE-103",
		PetName:                  "Figaro",
		Species:                  "Feline",
		Breed:                    "Domestic Longhair",
		WeightKg:                 5.2,
		Status:                   domain.FATEStatusReperfusedNormokalemicCleared,
		DistributionType:         domain.ThrombusDistRightForeBrachial,
		SystemicGlucoseMgDL:      118.0,
		AffectedLimbGlucoseMgDL:  110.0,
		GlucoseDeltaMgDL:         8.0,
		SystemicLactateMmolL:     1.6,
		AffectedLimbLactateMmolL: 1.9,
		LactateDeltaMmolL:        0.3,
		SerumPotassiumMmolL:      4.1,
		DistalLimbTempC:          37.6,
		LA2AoRatio:               1.78,
		ConcurrentCardiogenicCHF: false,
		GradientPlan:             grad103,
		AntithromboticPlan:       anti103,
		PerfusionChecks: []domain.LimbPerfusionCheck{
			{
				ID:                  "FATE-CHK-103-1",
				GlucoseDeltaMgDL:    38.0,
				LactateDeltaMmolL:   3.9,
				SerumPotassiumMmolL: 4.7,
				DistalLimbTempC:     33.4,
				FemoralPulseStatus:  "RIGHT_BRACHIAL_WEAK_DOPPLER_PRESENT",
				RecordedBy:          "Dr. Julian Vance, DACVIM (Cardiology)",
				ClinicalNote:        "Initial right forelimb brachial thromboembolism following Clopidogrel 75 mg PO load and Enoxaparin SC.",
				RecordedAt:          now.Add(-18 * time.Hour),
			},
			{
				ID:                  "FATE-CHK-103-2",
				GlucoseDeltaMgDL:    8.0,
				LactateDeltaMmolL:   0.3,
				SerumPotassiumMmolL: 4.1,
				DistalLimbTempC:     37.6,
				FemoralPulseStatus:  "STRONG_PALPABLE_PULSE_WARM_PINK_PADS",
				RecordedBy:          "Dr. Julian Vance, DACVIM (Cardiology)",
				ClinicalNote:        "Full right forelimb motor recovery, normokalemic K+ 4.1 mmol/L, glucose delta normalized to 8.0 mg/dL, limb temp 37.6°C.",
				RecordedAt:          now.Add(-1 * time.Hour),
			},
		},
		DischargedBy:     "Dr. Julian Vance, DACVIM (Cardiology)",
		DischargeSummary: "Right brachial thromboembolism reperfused with full voluntary motor ambulation, warm pink digital pads (37.6°C), normalized glucose gradient (8.0 mg/dL), and stable normokalemia (4.1 mmol/L). Discharged on lifelong dual antithrombotic therapy: Clopidogrel 18.75 mg PO q24h + Rivaroxaban 2.5 mg PO q24h.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of FATE operations, optionally filtered by statusFilter.
func (s *FATEService) ListOperations(statusFilter string) []domain.FATEThrombusOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.FATEThrombusOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneFATEOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a FATE operation by ID.
func (s *FATEService) GetOperation(id string) (*domain.FATEThrombusOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("fate thrombus operation %q not found", id)
	}
	cp := cloneFATEOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all FATE operations.
func (s *FATEService) GetOverviewMetrics() domain.FATEOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.FATEOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.FATEStatusAcuteSaddle5PsTriage:
			m.AcuteSaddleTriageCount++
		case domain.FATEStatusMethadoneClopidogrelStaged:
			m.AnticoagStagedCount++
		case domain.FATEStatusReperfusedNormokalemicCleared:
			m.ClearedReperfusedCount++
		}
		if op.DistributionType == domain.ThrombusDistBilateralAorticSaddle {
			m.BilateralSaddleCount++
		}
		if op.ConcurrentCardiogenicCHF {
			m.ConcurrentCHFCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new FATE / Saddle Thrombus triage operation.
func (s *FATEService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	distType domain.ThrombusDistributionType,
	systemicGlucose, limbGlucose, systemicLactate, limbLactate, serumPotassium, distalTempC, la2aoRatio float64,
	concurrentCHF bool,
) (*domain.FATEThrombusOperation, error) {
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
		trimmedSpecies = "Feline"
	}
	trimmedBreed := strings.TrimSpace(breed)
	if trimmedBreed == "" {
		trimmedBreed = "Domestic Shorthair"
	}
	if weightKg <= 0 || weightKg > 50.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 50.0], got %v", weightKg)
	}
	if !distType.IsValid() {
		return nil, fmt.Errorf("invalid distribution_type: %q", distType)
	}
	if systemicGlucose <= 0 || limbGlucose <= 0 {
		return nil, fmt.Errorf("systemic_glucose_mg_dl and affected_limb_glucose_mg_dl must be positive")
	}
	if systemicLactate < 0 || limbLactate < 0 {
		return nil, fmt.Errorf("lactate values cannot be negative")
	}
	if serumPotassium < 2.0 || serumPotassium > 12.0 {
		return nil, fmt.Errorf("serum_potassium_mmol_l must be in [2.0, 12.0], got %v", serumPotassium)
	}
	if distalTempC < 20.0 || distalTempC > 42.0 {
		return nil, fmt.Errorf("distal_limb_temp_c must be in [20.0, 42.0], got %v", distalTempC)
	}
	if la2aoRatio < 0.8 || la2aoRatio > 4.5 {
		return nil, fmt.Errorf("la2ao_ratio must be in [0.8, 4.5], got %v", la2aoRatio)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("FATE-OP-%d", s.seq)
	now := time.Now().UTC()

	glucoseDelta := round1(systemicGlucose - limbGlucose)
	lactateDelta := round1(limbLactate - systemicLactate)

	gradPlan := BuildMetabolicGradientPlan(
		distType,
		systemicGlucose, limbGlucose, glucoseDelta,
		systemicLactate, limbLactate, lactateDelta,
		serumPotassium, distalTempC, la2aoRatio,
		concurrentCHF,
	)
	antiPlan := BuildDefaultAntithromboticPlan(weightKg, concurrentCHF)

	op := &domain.FATEThrombusOperation{
		ID:                       opID,
		CaseID:                   trimmedCase,
		PetName:                  trimmedPet,
		Species:                  trimmedSpecies,
		Breed:                    trimmedBreed,
		WeightKg:                 weightKg,
		Status:                   domain.FATEStatusAcuteSaddle5PsTriage,
		DistributionType:         distType,
		SystemicGlucoseMgDL:      systemicGlucose,
		AffectedLimbGlucoseMgDL:  limbGlucose,
		GlucoseDeltaMgDL:         glucoseDelta,
		SystemicLactateMmolL:     systemicLactate,
		AffectedLimbLactateMmolL: limbLactate,
		LactateDeltaMmolL:        lactateDelta,
		SerumPotassiumMmolL:      serumPotassium,
		DistalLimbTempC:          distalTempC,
		LA2AoRatio:               la2aoRatio,
		ConcurrentCardiogenicCHF: concurrentCHF,
		GradientPlan:             gradPlan,
		AntithromboticPlan:       antiPlan,
		PerfusionChecks:          []domain.LimbPerfusionCheck{},
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneFATEOperation(op)
	return &cp, nil
}

// StageAntithrombotic stages the full-mu opioid CRI, Clopidogrel 75 mg PO loading dose (FAT CAT trial),
// Factor Xa inhibitor (Rivaroxaban) or LMWH (Enoxaparin) protocol, concurrent CHF Furosemide plan,
// serial potassium monitoring, and Pimobendan LVOT note, transitioning status to METHADONE_CLOPIDOGREL_ANTICOAG_STAGED.
func (s *FATEService) StageAntithrombotic(
	id, opioidProtocol, clopidogrelLoading, factorXaProtocol, chfFurosemidePlan, potassiumMonitoring, pimobendanNote string,
) (*domain.FATEThrombusOperation, error) {
	trimmedOpioid := strings.TrimSpace(opioidProtocol)
	if trimmedOpioid == "" {
		return nil, fmt.Errorf("full_mu_opioid_cri_protocol is required")
	}
	trimmedClopidogrel := strings.TrimSpace(clopidogrelLoading)
	if trimmedClopidogrel == "" {
		return nil, fmt.Errorf("clopidogrel_loading_protocol is required")
	}
	trimmedFactorXa := strings.TrimSpace(factorXaProtocol)
	if trimmedFactorXa == "" {
		return nil, fmt.Errorf("factor_xa_or_heparin_protocol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("fate thrombus operation %q not found", id)
	}

	defaults := BuildDefaultAntithromboticPlan(op.WeightKg, op.ConcurrentCardiogenicCHF)
	trimmedCHF := strings.TrimSpace(chfFurosemidePlan)
	if trimmedCHF == "" {
		trimmedCHF = defaults.ConcurrentCHFFurosemidePlan
	}
	trimmedK := strings.TrimSpace(potassiumMonitoring)
	if trimmedK == "" {
		trimmedK = defaults.SerialPotassiumMonitoring
	}
	trimmedPimo := strings.TrimSpace(pimobendanNote)
	if trimmedPimo == "" {
		trimmedPimo = defaults.PimobendanLVOTNote
	}

	op.AntithromboticPlan = domain.AntithromboticAnalgesiaPlan{
		FullMuOpioidCRIProtocol:     trimmedOpioid,
		ClopidogrelLoadingProtocol:  trimmedClopidogrel,
		FactorXaOrHeparinProtocol:   trimmedFactorXa,
		ConcurrentCHFFurosemidePlan: trimmedCHF,
		SerialPotassiumMonitoring:   trimmedK,
		PimobendanLVOTNote:          trimmedPimo,
	}
	op.Status = domain.FATEStatusMethadoneClopidogrelStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneFATEOperation(op)
	return &cp, nil
}

// RecordPerfusionCheck appends a LimbPerfusionCheck and updates op.GlucoseDeltaMgDL,
// op.LactateDeltaMmolL, op.SerumPotassiumMmolL, and op.DistalLimbTempC.
func (s *FATEService) RecordPerfusionCheck(
	id string,
	glucoseDeltaMgDL, lactateDeltaMmolL, serumPotassiumMmolL, distalLimbTempC float64,
	femoralPulseStatus, recordedBy, clinicalNote string,
) (*domain.FATEThrombusOperation, error) {
	if glucoseDeltaMgDL < 0 {
		return nil, fmt.Errorf("glucose_delta_mg_dl cannot be negative, got %v", glucoseDeltaMgDL)
	}
	if lactateDeltaMmolL < 0 {
		return nil, fmt.Errorf("lactate_delta_mmol_l cannot be negative, got %v", lactateDeltaMmolL)
	}
	if serumPotassiumMmolL < 2.0 || serumPotassiumMmolL > 12.0 {
		return nil, fmt.Errorf("serum_potassium_mmol_l must be in [2.0, 12.0], got %v", serumPotassiumMmolL)
	}
	if distalLimbTempC < 20.0 || distalLimbTempC > 42.0 {
		return nil, fmt.Errorf("distal_limb_temp_c must be in [20.0, 42.0], got %v", distalLimbTempC)
	}
	trimmedPulse := strings.TrimSpace(femoralPulseStatus)
	if trimmedPulse == "" {
		return nil, fmt.Errorf("femoral_pulse_status is required")
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
		return nil, fmt.Errorf("fate thrombus operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.LimbPerfusionCheck{
		ID:                  fmt.Sprintf("FATE-CHK-%s-%d", op.ID, len(op.PerfusionChecks)+1),
		GlucoseDeltaMgDL:    round1(glucoseDeltaMgDL),
		LactateDeltaMmolL:   round1(lactateDeltaMmolL),
		SerumPotassiumMmolL: round1(serumPotassiumMmolL),
		DistalLimbTempC:     round1(distalLimbTempC),
		FemoralPulseStatus:  trimmedPulse,
		RecordedBy:          trimmedBy,
		ClinicalNote:        trimmedNote,
		RecordedAt:          now,
	}

	op.PerfusionChecks = append(op.PerfusionChecks, chk)
	op.GlucoseDeltaMgDL = chk.GlucoseDeltaMgDL
	op.AffectedLimbGlucoseMgDL = round1(math.Max(20.0, op.SystemicGlucoseMgDL-chk.GlucoseDeltaMgDL))
	op.LactateDeltaMmolL = chk.LactateDeltaMmolL
	op.AffectedLimbLactateMmolL = round1(op.SystemicLactateMmolL + chk.LactateDeltaMmolL)
	op.SerumPotassiumMmolL = chk.SerumPotassiumMmolL
	op.DistalLimbTempC = chk.DistalLimbTempC

	op.GradientPlan = BuildMetabolicGradientPlan(
		op.DistributionType,
		op.SystemicGlucoseMgDL, op.AffectedLimbGlucoseMgDL, op.GlucoseDeltaMgDL,
		op.SystemicLactateMmolL, op.AffectedLimbLactateMmolL, op.LactateDeltaMmolL,
		op.SerumPotassiumMmolL, op.DistalLimbTempC, op.LA2AoRatio,
		op.ConcurrentCardiogenicCHF,
	)
	op.UpdatedAt = now

	cp := cloneFATEOperation(op)
	return &cp, nil
}

// ClearFATEDischarge transitions the operation to REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED
// once distal limb perfusion, normokalemia, and Clopidogrel + Rivaroxaban dual therapy are confirmed.
func (s *FATEService) ClearFATEDischarge(id, dischargedBy, summary string) (*domain.FATEThrombusOperation, error) {
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
		return nil, fmt.Errorf("fate thrombus operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.FATEStatusReperfusedNormokalemicCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneFATEOperation(op)
	return &cp, nil
}

func cloneFATEOperation(op *domain.FATEThrombusOperation) domain.FATEThrombusOperation {
	cp := *op
	if op.PerfusionChecks != nil {
		cp.PerfusionChecks = make([]domain.LimbPerfusionCheck, len(op.PerfusionChecks))
		copy(cp.PerfusionChecks, op.PerfusionChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
