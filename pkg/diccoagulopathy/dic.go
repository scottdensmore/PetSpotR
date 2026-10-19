package diccoagulopathy

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// DICService manages Acute Coagulopathy & Disseminated Intravascular Coagulation (DIC),
// PT/aPTT Prolongation, D-Dimer/FDP Elevation, Fresh Frozen Plasma (FFP) &
// Tranexamic Acid (TXA) triage operations in a thread-safe store.
type DICService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.DICCoagulopathyOperation
	order []string
	seq   int
}

// NewDICService initializes a DICService with 3 seeded operations (DIC-OP-101, DIC-OP-102, DIC-OP-103).
func NewDICService() *DICService {
	s := &DICService{
		ops:   make(map[string]*domain.DICCoagulopathyOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildCoagulationTEGProfilePlan computes the clinical interpretation for a DIC / Coagulopathy operation.
func BuildCoagulationTEGProfilePlan(
	phenotype domain.DICPhasePhenotype,
	platelets, ptSec, apttSec, dDimer, fibrinogen, atIII float64,
	schistocytes, mucosalBleeding bool,
) domain.CoagulationTEGProfilePlan {
	var patho string
	switch phenotype {
	case domain.DICPhenotypeHypoCoagulableConsumptive:
		patho = "Hypocoagulable Hemorrhagic Consumptive DIC — Severe systemic thrombin-plasmin activation with secondary exhaustion of platelets, soluble coagulation factors (II, V, VII, VIII, IX, X, XIII), fibrinogen (< 100 mg/dL), and Antithrombin III (< 65%), producing spontaneous mucosal/cavitary bleeding and microvascular schistocytosis."
	case domain.DICPhenotypeHyperCoagulableEarly:
		patho = "Hypercoagulable Microthrombotic Early/Non-Overt DIC — Unchecked tissue factor (TF) expression and pro-inflammatory cytokine storm driving diffuse microvascular fibrin deposition, falling Antithrombin III, and organ micro-infarction prior to overt factor exhaustion."
	case domain.DICPhenotypeHyperfibrinolysisGreyhound:
		patho = "Delayed Post-Operative / Trauma Hyperfibrinolysis (Greyhound Sighthound Syndrome & Acute Traumatic Coagulopathy) — Normal primary hemostasis and initial clot formation followed by premature plasmin-mediated clot lysis (TEG LY30 > 3%) 24–72h post-trauma or surgery."
	default:
		patho = "Acute Hemostatic & Fibrinolytic Dysregulation — Combined primary/secondary coagulation factor consumption and fibrinolysis."
	}

	bleedingNote := "No active spontaneous mucosal hemorrhage"
	if mucosalBleeding {
		bleedingNote = "ACTIVE SPONTANEOUS MUCOSAL / PETECHIAL / ECCHYMOTIC HEMORRHAGE PRESENT"
	}
	schistocyteNote := "No peripheral blood smear schistocytes"
	if schistocytes {
		schistocyteNote = "Microangiopathic RBC fragmentation (Schistocytes / Acanthocytes) confirmed on blood smear"
	}

	ptApttSummary := fmt.Sprintf("Platelets %.1f K/µL | PT %.1f sec & aPTT %.1f sec (> 1.5× reference upper limit indicates severe extrinsic/intrinsic factor depletion) — %s; %s.", platelets, ptSec, apttSec, bleedingNote, schistocyteNote)

	dDimerSummary := fmt.Sprintf("D-Dimer %.1f ng/mL (> 500 ng/mL confirms cross-linked fibrin degradation) | Fibrinogen %.1f mg/dL (< 100 mg/dL critical hypofibrinogenemia) | Antithrombin III %.1f%% (< 65%% indicates severe ATIII consumption and high mortality risk).", dDimer, fibrinogen, atIII)

	var tegNote string
	switch phenotype {
	case domain.DICPhenotypeHypoCoagulableConsumptive:
		tegNote = "TEG/ROTEM Hypocoagulable Tracing: Prolonged R-time (clotting factor deficiency), prolonged K-time & decreased alpha-angle (hypofibrinogenemia), severely reduced Maximum Amplitude MA < 45 mm (thrombocytopenia/platelet dysfunction)."
	case domain.DICPhenotypeHyperCoagulableEarly:
		tegNote = "TEG/ROTEM Hypercoagulable Tracing: Shortened R-time and K-time, increased alpha-angle (> 75°), and elevated Maximum Amplitude MA > 68 mm; initiate thromboprophylaxis before consumptive transition."
	case domain.DICPhenotypeHyperfibrinolysisGreyhound:
		tegNote = "TEG/ROTEM Hyperfibrinolytic Tracing: Normal R-time and initial MA followed by rapid amplitude decay with LY30 > 3% (or CL30 < 90%) diagnostic for primary/secondary hyperfibrinolysis."
	default:
		tegNote = "TEG/ROTEM Viscoelastic Monitoring: Evaluate R-time (FFP need), K-time/alpha-angle (Cryoprecipitate/Fibrinogen need), MA (platelet function), and LY30 (antifibrinolytic TXA/EACA need)."
	}

	txaNote := fmt.Sprintf("Antifibrinolytic Indication: Administer Tranexamic Acid (TXA 10 mg/kg IV slow over 15–20 min q8h) or Epsilon-Aminocaproic Acid (EACA 50–100 mg/kg IV/PO q8h) for Hyperfibrinolysis (TEG LY30 > 3%%), Greyhound post-operative bleeding, or severe hemorrhagic consumptive DIC (D-Dimer %.1f ng/mL); use caution in pure early microthrombotic DIC without hemorrhage.", dDimer)

	jugularGuard := "CONTRAINDICATION GUARD: STRICTLY AVOID jugular venipuncture, central line placement in non-compressible neck vessels, cystocentesis, and intramuscular (IM) injections in coagulopathic/DIC patients (Platelets < 60 K/µL or PT/aPTT > 1.5× normal) — risk of lethal cervical hematoma causing upper airway obstruction. Use peripheral saphenous/cephalic veins with 5–10 min manual compression wraps."

	return domain.CoagulationTEGProfilePlan{
		PhenotypePathophysiology:            patho,
		PTaPTTProlongationSummary:           ptApttSummary,
		DDimerFibrinogenATIIINote:           dDimerSummary,
		TEGViscoelasticInterpretation:       tegNote,
		TXAFibrinolysisIndicationNote:       txaNote,
		JugularVenipunctureContraindication: jugularGuard,
	}
}

// BuildDefaultFFPAntifibrinolyticPlan returns an evidence-based FFPAntifibrinolyticPlan.
func BuildDefaultFFPAntifibrinolyticPlan(weightKg float64, phenotype domain.DICPhasePhenotype, fibrinogen float64) domain.FFPAntifibrinolyticPlan {
	ffpLowMl := round1(weightKg * 10.0)
	ffpHighMl := round1(weightKg * 20.0)
	cryoUnits := math.Max(1.0, round1(weightKg/10.0))
	txaDoseMg := round1(weightKg * 10.0)
	eacaDoseMg := round1(weightKg * 75.0)

	cryoIndication := fmt.Sprintf("Cryoprecipitate 1 unit per 10 kg (%.1f units IV for %.1f kg patient) indicated if Fibrinogen < 100 mg/dL (current %.1f mg/dL) or concomitant von Willebrand Disease (vWD Factor VIII/vWF deficiency).", cryoUnits, weightKg, fibrinogen)

	heparinPlan := "Consumptive Hemorrhagic Phase: Withhold systemic unfractionated/LMWH heparin during active spontaneous mucosal hemorrhage until clotting factors and Antithrombin III (> 65%) are replenished via FFP."
	if phenotype == domain.DICPhenotypeHyperCoagulableEarly {
		heparinPlan = "Early Hypercoagulable Microthrombotic Phase: Enoxaparin 0.8 mg/kg SC q8h or Dalteparin 150 U/kg SC q8h (requires adequate Antithrombin III cofactor > 65%, supplemented via FFP if depleted)."
	}

	return domain.FFPAntifibrinolyticPlan{
		FreshFrozenPlasmaProtocol:      fmt.Sprintf("Fresh Frozen Plasma (FFP) 10–20 mL/kg IV (%.0f–%.0f mL over 3–4h via 170–260 µm blood filter; start 0.5–1 mL/kg/hr for 15 min transfusion reaction check) to replenish labile/stable Factors II, V, VII, VIII, IX, X, fibrinogen, and Antithrombin III.", ffpLowMl, ffpHighMl),
		CryoprecipitatevWFProtocol:     cryoIndication,
		TranexamicAcidAminocaproicPlan: fmt.Sprintf("Tranexamic Acid (TXA) 10 mg/kg IV slow infusion over 15–20 min q8h (%.1f mg IV — infuse slowly to prevent emesis/hypotension) OR Epsilon-Aminocaproic Acid (EACA) 50–100 mg/kg IV/PO q8h (%.0f mg) for Greyhound/trauma hyperfibrinolysis.", txaDoseMg, eacaDoseMg),
		AntithrombinHeparinPhasePlan:   heparinPlan,
		UnderlyingTriggerControlPlan:   "Primary Trigger Eradication: Rapid source control of underlying sepsis, heatstroke, hemangiosarcoma hemoabdomen, pancreatitis, pit-viper envenomation, or immune-mediated hemolysis plus microvascular perfusion restoration.",
		SerialCoagPanelSchedule:        "Serial Coagulation & Viscoelastic Panel (PT, aPTT, Platelet count, Fibrinogen, D-Dimer, Antithrombin III %, PCV/TS) q4–6h post-FFP/TXA infusion; peripheral venipuncture only.",
	}
}

func (s *DICService) seed() {
	now := time.Now().UTC()

	// DIC-OP-101: Flash — Canine, Retired Racing Greyhound, 31.0 kg, ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE
	prof101 := BuildCoagulationTEGProfilePlan(
		domain.DICPhenotypeHypoCoagulableConsumptive,
		38.0, 24.5, 142.0, 1850.0, 68.0, 48.0,
		true, true,
	)
	comp101 := BuildDefaultFFPAntifibrinolyticPlan(31.0, domain.DICPhenotypeHypoCoagulableConsumptive, 68.0)
	op101 := &domain.DICCoagulopathyOperation{
		ID:                         "DIC-OP-101",
		CaseID:                     "CASE-DIC-101",
		PetName:                    "Flash",
		Species:                    "Canine",
		Breed:                      "Retired Racing Greyhound",
		WeightKg:                   31.0,
		Status:                     domain.DICStatusAcuteOvertDICTriage,
		PhasePhenotype:             domain.DICPhenotypeHypoCoagulableConsumptive,
		PlateletCountKUl:           38.0,
		PTSeconds:                  24.5,
		APTTSeconds:                142.0,
		DDimerNgML:                 1850.0,
		FibrinogenMgDL:             68.0,
		AntithrombinPercent:        48.0,
		SchistocytesPresent:        true,
		SpontaneousMucosalBleeding: true,
		ProfilePlan:                prof101,
		ComponentPlan:              comp101,
		CoagChecks: []domain.CoagulationPanelCheck{
			{
				ID:               "DIC-CHK-101-1",
				PlateletCountKUl: 44.0,
				PTSeconds:        22.8,
				APTTSeconds:      135.0,
				DDimerNgML:       1620.0,
				FibrinogenMgDL:   78.0,
				RecordedBy:       "Dr. Callum Vance, DACVECC",
				ClinicalNote:     "Presentation triage following exertional heatstroke: petechiae/ecchymoses on gingival mucosa, blood smear schistocytes +, severe PT/aPTT prolongation (> 1.5x normal), D-Dimer 1620 ng/mL.",
				RecordedAt:       now.Add(-50 * time.Minute),
			},
			{
				ID:               "DIC-CHK-101-2",
				PlateletCountKUl: 38.0,
				PTSeconds:        24.5,
				APTTSeconds:      142.0,
				DDimerNgML:       1850.0,
				FibrinogenMgDL:   68.0,
				RecordedBy:       "Dr. Callum Vance, DACVECC",
				ClinicalNote:     "Progression to overt consumptive DIC with hypofibrinogenemia (68 mg/dL) and ATIII 48%; jugular venipuncture strictly prohibited, thawing DEA 1.1-negative FFP 15 mL/kg + Cryoprecipitate + TXA 10 mg/kg IV.",
				RecordedAt:       now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// DIC-OP-102: Rocket — Canine, Doberman Pinscher, 34.5 kg, FFP_CRYOPRECIPITATE_TXA_STAGED
	prof102 := BuildCoagulationTEGProfilePlan(
		domain.DICPhenotypeHyperfibrinolysisGreyhound,
		92.0, 14.2, 88.0, 940.0, 115.0, 72.0,
		false, true,
	)
	comp102 := BuildDefaultFFPAntifibrinolyticPlan(34.5, domain.DICPhenotypeHyperfibrinolysisGreyhound, 115.0)
	op102 := &domain.DICCoagulopathyOperation{
		ID:                         "DIC-OP-102",
		CaseID:                     "CASE-DIC-102",
		PetName:                    "Rocket",
		Species:                    "Canine",
		Breed:                      "Doberman Pinscher",
		WeightKg:                   34.5,
		Status:                     domain.DICStatusFFPCryoprecipitateTXAStaged,
		PhasePhenotype:             domain.DICPhenotypeHyperfibrinolysisGreyhound,
		PlateletCountKUl:           92.0,
		PTSeconds:                  14.2,
		APTTSeconds:                88.0,
		DDimerNgML:                 940.0,
		FibrinogenMgDL:             115.0,
		AntithrombinPercent:        72.0,
		SchistocytesPresent:        false,
		SpontaneousMucosalBleeding: true,
		ProfilePlan:                prof102,
		ComponentPlan:              comp102,
		CoagChecks: []domain.CoagulationPanelCheck{
			{
				ID:               "DIC-CHK-102-1",
				PlateletCountKUl: 92.0,
				PTSeconds:        14.2,
				APTTSeconds:      88.0,
				DDimerNgML:       940.0,
				FibrinogenMgDL:   115.0,
				RecordedBy:       "Dr. Elena Rostova, DACVIM",
				ClinicalNote:     "Cryoprecipitate 3.5 units IV (vWF/Factor VIII/Fibrinogen support) + Tranexamic Acid (TXA) 10 mg/kg IV slow over 20 min q8h + FFP 12 mL/kg IV staged for traumatic coagulopathy and hyperfibrinolysis (TEG LY30 8.4%).",
				RecordedAt:       now.Add(-80 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-80 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// DIC-OP-103: Willow — Feline, Domestic Medium Hair, 4.4 kg, HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED
	prof103 := BuildCoagulationTEGProfilePlan(
		domain.DICPhenotypeHyperCoagulableEarly,
		215.0, 10.8, 64.0, 180.0, 210.0, 94.0,
		false, false,
	)
	comp103 := BuildDefaultFFPAntifibrinolyticPlan(4.4, domain.DICPhenotypeHyperCoagulableEarly, 210.0)
	dischargedAt := now.Add(-25 * time.Minute)
	op103 := &domain.DICCoagulopathyOperation{
		ID:                         "DIC-OP-103",
		CaseID:                     "CASE-DIC-103",
		PetName:                    "Willow",
		Species:                    "Feline",
		Breed:                      "Domestic Medium Hair",
		WeightKg:                   4.4,
		Status:                     domain.DICStatusHemostasisNormocoagulableClear,
		PhasePhenotype:             domain.DICPhenotypeHyperCoagulableEarly,
		PlateletCountKUl:           215.0,
		PTSeconds:                  10.8,
		APTTSeconds:                64.0,
		DDimerNgML:                 180.0,
		FibrinogenMgDL:             210.0,
		AntithrombinPercent:        94.0,
		SchistocytesPresent:        false,
		SpontaneousMucosalBleeding: false,
		ProfilePlan:                prof103,
		ComponentPlan:              comp103,
		CoagChecks: []domain.CoagulationPanelCheck{
			{
				ID:               "DIC-CHK-103-1",
				PlateletCountKUl: 108.0,
				PTSeconds:        15.6,
				APTTSeconds:      98.0,
				DDimerNgML:       760.0,
				FibrinogenMgDL:   135.0,
				RecordedBy:       "Dr. Callum Vance, DACVECC",
				ClinicalNote:     "Early pancreatitis-associated DIC managed with type-specific Feline FFP 12 mL/kg IV and low-dose Enoxaparin 0.8 mg/kg SC q8h.",
				RecordedAt:       now.Add(-16 * time.Hour),
			},
			{
				ID:               "DIC-CHK-103-2",
				PlateletCountKUl: 215.0,
				PTSeconds:        10.8,
				APTTSeconds:      64.0,
				DDimerNgML:       180.0,
				FibrinogenMgDL:   210.0,
				RecordedBy:       "Dr. Callum Vance, DACVECC",
				ClinicalNote:     "Full hemostatic normalization: Platelets 215 K/µL, PT 10.8s, aPTT 64.0s, D-Dimer 180 ng/mL, Fibrinogen 210 mg/dL, ATIII 94%.",
				RecordedAt:       now.Add(-45 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Callum Vance, DACVECC",
		DischargeSummary: "Hemostasis restored and normocoagulable profile verified (Platelets 215.0 K/µL, PT 10.8s, aPTT 64.0s, D-Dimer 180.0 ng/mL, Fibrinogen 210.0 mg/dL, Antithrombin III 94.0%) with complete resolution of underlying pancreatitis trigger.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-20 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of DIC operations, optionally filtered by statusFilter.
func (s *DICService) ListOperations(statusFilter string) []domain.DICCoagulopathyOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.DICCoagulopathyOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneDICOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a DIC operation by ID.
func (s *DICService) GetOperation(id string) (*domain.DICCoagulopathyOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("dic coagulopathy operation %q not found", id)
	}
	cp := cloneDICOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all DIC operations.
func (s *DICService) GetOverviewMetrics() domain.DICOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.DICOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.DICStatusAcuteOvertDICTriage:
			m.OvertDICTriageCount++
		case domain.DICStatusFFPCryoprecipitateTXAStaged:
			m.FFPTXAStagedCount++
		case domain.DICStatusHemostasisNormocoagulableClear:
			m.ClearedNormocoagulableCount++
		}
		if op.PhasePhenotype == domain.DICPhenotypeHypoCoagulableConsumptive {
			m.ConsumptiveHemorrhagicCount++
		}
		if op.PhasePhenotype == domain.DICPhenotypeHyperfibrinolysisGreyhound {
			m.HyperfibrinolysisGreyhoundCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Coagulopathy / DIC triage operation.
func (s *DICService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	phasePhenotype domain.DICPhasePhenotype,
	plateletCountKUl, ptSeconds, apttSeconds, dDimerNgML, fibrinogenMgDL, antithrombinPercent float64,
	schistocytesPresent, spontaneousMucosalBleeding bool,
) (*domain.DICCoagulopathyOperation, error) {
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
	if !phasePhenotype.IsValid() {
		return nil, fmt.Errorf("invalid phase_phenotype: %q", phasePhenotype)
	}
	if plateletCountKUl <= 0 || plateletCountKUl > 1200.0 {
		return nil, fmt.Errorf("platelet_count_k_ul must be in (0, 1200.0], got %v", plateletCountKUl)
	}
	if ptSeconds <= 0 || ptSeconds > 180.0 {
		return nil, fmt.Errorf("pt_seconds must be in (0, 180.0], got %v", ptSeconds)
	}
	if apttSeconds <= 0 || apttSeconds > 300.0 {
		return nil, fmt.Errorf("aptt_seconds must be in (0, 300.0], got %v", apttSeconds)
	}
	if dDimerNgML < 0 || dDimerNgML > 20000.0 {
		return nil, fmt.Errorf("d_dimer_ng_ml must be in [0, 20000.0], got %v", dDimerNgML)
	}
	if fibrinogenMgDL <= 0 || fibrinogenMgDL > 1200.0 {
		return nil, fmt.Errorf("fibrinogen_mg_dl must be in (0, 1200.0], got %v", fibrinogenMgDL)
	}
	if antithrombinPercent <= 0 || antithrombinPercent > 200.0 {
		return nil, fmt.Errorf("antithrombin_percent must be in (0, 200.0], got %v", antithrombinPercent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("DIC-OP-%d", s.seq)
	now := time.Now().UTC()

	profPlan := BuildCoagulationTEGProfilePlan(
		phasePhenotype,
		plateletCountKUl, ptSeconds, apttSeconds, dDimerNgML, fibrinogenMgDL, antithrombinPercent,
		schistocytesPresent, spontaneousMucosalBleeding,
	)
	compPlan := BuildDefaultFFPAntifibrinolyticPlan(weightKg, phasePhenotype, fibrinogenMgDL)

	op := &domain.DICCoagulopathyOperation{
		ID:                         opID,
		CaseID:                     trimmedCase,
		PetName:                    trimmedPet,
		Species:                    trimmedSpecies,
		Breed:                      trimmedBreed,
		WeightKg:                   round1(weightKg),
		Status:                     domain.DICStatusAcuteOvertDICTriage,
		PhasePhenotype:             phasePhenotype,
		PlateletCountKUl:           round1(plateletCountKUl),
		PTSeconds:                  round1(ptSeconds),
		APTTSeconds:                round1(apttSeconds),
		DDimerNgML:                 round1(dDimerNgML),
		FibrinogenMgDL:             round1(fibrinogenMgDL),
		AntithrombinPercent:        round1(antithrombinPercent),
		SchistocytesPresent:        schistocytesPresent,
		SpontaneousMucosalBleeding: spontaneousMucosalBleeding,
		ProfilePlan:                profPlan,
		ComponentPlan:              compPlan,
		CoagChecks:                 []domain.CoagulationPanelCheck{},
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneDICOperation(op)
	return &cp, nil
}

// StageComponentTherapy stages the Fresh Frozen Plasma (FFP 10–20 mL/kg IV), Cryoprecipitate
// (1 unit/10 kg), Tranexamic Acid (TXA 10 mg/kg IV) / EACA plan, Antithrombin III / Heparin phase plan,
// underlying trigger control, and serial coagulation panel schedule, transitioning status to FFP_CRYOPRECIPITATE_TXA_STAGED.
func (s *DICService) StageComponentTherapy(
	id, ffpProtocol, cryoprecipitateProtocol, txaPlan, heparinPhasePlan, triggerControlPlan, coagSchedule string,
) (*domain.DICCoagulopathyOperation, error) {
	trimmedFFP := strings.TrimSpace(ffpProtocol)
	if trimmedFFP == "" {
		return nil, fmt.Errorf("fresh_frozen_plasma_protocol is required")
	}
	trimmedCryo := strings.TrimSpace(cryoprecipitateProtocol)
	if trimmedCryo == "" {
		return nil, fmt.Errorf("cryoprecipitate_vwf_protocol is required")
	}
	trimmedTXA := strings.TrimSpace(txaPlan)
	if trimmedTXA == "" {
		return nil, fmt.Errorf("tranexamic_acid_aminocaproic_plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("dic coagulopathy operation %q not found", id)
	}

	defaults := BuildDefaultFFPAntifibrinolyticPlan(op.WeightKg, op.PhasePhenotype, op.FibrinogenMgDL)
	trimmedHeparin := strings.TrimSpace(heparinPhasePlan)
	if trimmedHeparin == "" {
		trimmedHeparin = defaults.AntithrombinHeparinPhasePlan
	}
	trimmedTrigger := strings.TrimSpace(triggerControlPlan)
	if trimmedTrigger == "" {
		trimmedTrigger = defaults.UnderlyingTriggerControlPlan
	}
	trimmedSchedule := strings.TrimSpace(coagSchedule)
	if trimmedSchedule == "" {
		trimmedSchedule = defaults.SerialCoagPanelSchedule
	}

	op.ComponentPlan = domain.FFPAntifibrinolyticPlan{
		FreshFrozenPlasmaProtocol:      trimmedFFP,
		CryoprecipitatevWFProtocol:     trimmedCryo,
		TranexamicAcidAminocaproicPlan: trimmedTXA,
		AntithrombinHeparinPhasePlan:   trimmedHeparin,
		UnderlyingTriggerControlPlan:   trimmedTrigger,
		SerialCoagPanelSchedule:        trimmedSchedule,
	}
	op.Status = domain.DICStatusFFPCryoprecipitateTXAStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneDICOperation(op)
	return &cp, nil
}

// RecordCoagCheck appends a CoagulationPanelCheck and updates op.PlateletCountKUl,
// op.PTSeconds, op.APTTSeconds, op.DDimerNgML, and op.FibrinogenMgDL.
func (s *DICService) RecordCoagCheck(
	id string,
	plateletCountKUl, ptSeconds, apttSeconds, dDimerNgML, fibrinogenMgDL float64,
	recordedBy, clinicalNote string,
) (*domain.DICCoagulopathyOperation, error) {
	if plateletCountKUl <= 0 || plateletCountKUl > 1200.0 {
		return nil, fmt.Errorf("platelet_count_k_ul must be in (0, 1200.0], got %v", plateletCountKUl)
	}
	if ptSeconds <= 0 || ptSeconds > 180.0 {
		return nil, fmt.Errorf("pt_seconds must be in (0, 180.0], got %v", ptSeconds)
	}
	if apttSeconds <= 0 || apttSeconds > 300.0 {
		return nil, fmt.Errorf("aptt_seconds must be in (0, 300.0], got %v", apttSeconds)
	}
	if dDimerNgML < 0 || dDimerNgML > 20000.0 {
		return nil, fmt.Errorf("d_dimer_ng_ml must be in [0, 20000.0], got %v", dDimerNgML)
	}
	if fibrinogenMgDL <= 0 || fibrinogenMgDL > 1200.0 {
		return nil, fmt.Errorf("fibrinogen_mg_dl must be in (0, 1200.0], got %v", fibrinogenMgDL)
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
		return nil, fmt.Errorf("dic coagulopathy operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.CoagulationPanelCheck{
		ID:               fmt.Sprintf("DIC-CHK-%s-%d", op.ID, len(op.CoagChecks)+1),
		PlateletCountKUl: round1(plateletCountKUl),
		PTSeconds:        round1(ptSeconds),
		APTTSeconds:      round1(apttSeconds),
		DDimerNgML:       round1(dDimerNgML),
		FibrinogenMgDL:   round1(fibrinogenMgDL),
		RecordedBy:       trimmedBy,
		ClinicalNote:     trimmedNote,
		RecordedAt:       now,
	}

	op.CoagChecks = append(op.CoagChecks, chk)
	op.PlateletCountKUl = chk.PlateletCountKUl
	op.PTSeconds = chk.PTSeconds
	op.APTTSeconds = chk.APTTSeconds
	op.DDimerNgML = chk.DDimerNgML
	op.FibrinogenMgDL = chk.FibrinogenMgDL

	op.ProfilePlan = BuildCoagulationTEGProfilePlan(
		op.PhasePhenotype,
		op.PlateletCountKUl, op.PTSeconds, op.APTTSeconds, op.DDimerNgML, op.FibrinogenMgDL, op.AntithrombinPercent,
		op.SchistocytesPresent, op.SpontaneousMucosalBleeding,
	)
	op.UpdatedAt = now

	cp := cloneDICOperation(op)
	return &cp, nil
}

// ClearDICDischarge transitions the operation to HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED
// once coagulation parameters normalize and spontaneous bleeding resolves.
func (s *DICService) ClearDICDischarge(id, dischargedBy, summary string) (*domain.DICCoagulopathyOperation, error) {
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
		return nil, fmt.Errorf("dic coagulopathy operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.DICStatusHemostasisNormocoagulableClear
	op.SpontaneousMucosalBleeding = false
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneDICOperation(op)
	return &cp, nil
}

func cloneDICOperation(op *domain.DICCoagulopathyOperation) domain.DICCoagulopathyOperation {
	cp := *op
	if op.CoagChecks != nil {
		cp.CoagChecks = make([]domain.CoagulationPanelCheck, len(op.CoagChecks))
		copy(cp.CoagChecks, op.CoagChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
