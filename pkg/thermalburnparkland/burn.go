package thermalburnparkland

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// BurnService manages Acute Smoke Inhalation, Thermal Burn Classification
// (Superficial/Partial/Full-Thickness TBSA%), Parkland Crystalloid Resuscitation Formula,
// Silver Sulfadiazine & Escharotomy triage operations in a thread-safe store.
type BurnService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.ThermalBurnOperation
	order []string
	seq   int
}

// NewBurnService initializes a BurnService with 3 seeded operations (BURN-OP-101, BURN-OP-102, BURN-OP-103).
func NewBurnService() *BurnService {
	s := &BurnService{
		ops:   make(map[string]*domain.ThermalBurnOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// ComputeParkland24hResuscitation calculates the 24-hour Parkland crystalloid volume (mL),
// first 8h rate (mL/hr), and next 16h rate (mL/hr) based on species, weight (kg), and %TBSA.
// Canine: 4.0 mL/kg/%TBSA; Feline: 2.5 mL/kg/%TBSA (2–3 mL/kg/%TBSA to avoid feline fluid overload).
func ComputeParkland24hResuscitation(species string, weightKg, tbsaPercent float64) (total24hML, first8hRateMLHr, next16hRateMLHr float64) {
	factor := 4.0
	if strings.EqualFold(strings.TrimSpace(species), "Feline") {
		factor = 2.5
	}
	total24hML = round1(factor * weightKg * tbsaPercent)
	halfML := total24hML * 0.5
	first8hRateMLHr = round1(halfML / 8.0)
	next16hRateMLHr = round1(halfML / 16.0)
	return total24hML, first8hRateMLHr, next16hRateMLHr
}

// BuildTBSAParklandAssessmentPlan computes the clinical interpretation for a Thermal Burn & Parkland operation.
func BuildTBSAParklandAssessmentPlan(
	species string,
	weightKg float64,
	depthClass domain.BurnDepthClassification,
	tbsaPercent, total24hML, first8hRateMLHr, next16hRateMLHr, currentFluidRateMLHr, urineOutputMLKgHr, serumAlbuminGDL, pcvPercent float64,
	circumferentialThoracicEschar, singedVibrissaeSootAirway bool,
) domain.TBSAParklandAssessmentPlan {
	var depthPatho string
	switch depthClass {
	case domain.BurnDepthFullThicknessCircumferentialEschar:
		depthPatho = "Full-Thickness (3rd/4th Degree) Circumferential Leathery Eschar — Complete destruction of epidermis, dermis, adnexal structures, and subcutaneous neurovascular plexus producing insensate, avascular, non-compliant leathery eschar with Jackson's zones of coagulation, stasis, and hyperemia."
	case domain.BurnDepthDeepPartialThicknessDermal:
		depthPatho = "Deep Partial-Thickness (Deep 2nd Degree) Dermal Burn — Thermal coagulation extending into reticular dermis with severe microvascular endothelial gap formation, massive third-space protein/oncotic extravasation, hypoproteinemia, and delayed re-epithelialization from surviving hair follicles."
	case domain.BurnDepthThermalAirwayCornealFlash:
		depthPatho = "Supraglottic Thermal Airway, Singed Vibrissae & Corneal Flash Burn — Direct thermal injury to upper airway mucosa and corneal epithelium with soot/carbonaceous cast sloughing, laryngeal edema risk, and superficial-to-partial dermal flash erythema."
	default:
		depthPatho = "Mixed Superficial / Partial / Full-Thickness Thermal Burn Injury — Microvascular capillary leak and systemic inflammatory response proportional to %TBSA."
	}

	speciesFactor := "4 mL/kg/%TBSA (Canine)"
	if strings.EqualFold(strings.TrimSpace(species), "Feline") {
		speciesFactor = "2.5 mL/kg/%TBSA (Feline 2–3 mL/kg/%TBSA)"
	}

	parklandSummary := fmt.Sprintf("Parkland Crystalloid Formula (%s for %.1f kg × %.1f%% TBSA): Total 24h Volume = %.1f mL Isotonic Buffered Crystalloid (LRS / Normosol-R) | Current Rate %.1f mL/hr | Urine Output %.1f mL/kg/hr (target 1.0–2.0 mL/kg/hr) | Serum Albumin %.1f g/dL | PCV %.1f%%.",
		speciesFactor, weightKg, tbsaPercent, total24hML, currentFluidRateMLHr, urineOutputMLKgHr, serumAlbuminGDL, pcvPercent)

	splitNote := fmt.Sprintf("Parkland Biphasic 8h/16h Split: Deliver 50%% (%.1f mL) over First 8 Hours FROM TIME OF THERMAL INJURY (not time of hospital presentation) at %.1f mL/hr, followed by remaining 50%% (%.1f mL) over Next 16 Hours at %.1f mL/hr — titrate hourly to maintain Urine Output 1.0–2.0 mL/kg/hr and normalize hemoconcentrated PCV (%.1f%%).",
		round1(total24hML*0.5), first8hRateMLHr, round1(total24hML*0.5), next16hRateMLHr, pcvPercent)

	escharNote := "No circumferential constricting thoracic/limb eschar present; monitor chest wall excursion and distal limb Doppler pulses q2h."
	if circumferentialThoracicEschar {
		escharNote = "EMERGENT ESCHAROTOMY INDICATION: Circumferential full-thickness leathery thoracic/limb eschar present — unyielding eschar over edematous underlying tissue restricts chest wall excursion (ventilatory asphyxiation) and causes limb compartment syndrome; perform immediate longitudinal mid-lateral thoracic / medial-lateral limb escharotomy incisions through eschar into subcutaneous fat."
	}
	if singedVibrissaeSootAirway {
		escharNote += " [AIRWAY ALERT: Singed vibrissae & carbonaceous oral/nasal soot — monitor for progressive supraglottic laryngeal thermal edema and pseudomembranous cast obstruction]."
	}

	colloidGuard := "STRICT 24-HOUR COLLOID CONTRAINDICATION: Withhold synthetic colloids (VetStarch/HES) and natural colloids (human/canine albumin, FFP) during the first 12–24 hours post-burn while microvascular endothelial gap junctions are maximally permeable — early colloids extravasate into the interstitium, worsening pulmonary and tissue third-space edema."

	steroidAbxGuard := "CONTRAINDICATION GUARD: STRICTLY AVOID systemic corticosteroids (impair wound healing, suppress immunity, and markedly increase fatal burn sepsis/Pseudomonas mortality) AND avoid routine prophylactic systemic antibiotics (selects for multi-drug-resistant Pseudomonas aeruginosa and MRSA; use topical 1% Silver Sulfadiazine or medical-grade Manuka honey instead)."

	return domain.TBSAParklandAssessmentPlan{
		BurnDepthPathophysiology:              depthPatho,
		Parkland24hCalculationSummary:         parklandSummary,
		First8hVsNext16hSplitNote:             splitNote,
		CircumferentialEscharotomyAlert:       escharNote,
		Colloid24hCapillaryLeakWarning:        colloidGuard,
		SystemicSteroidProphylacticAbxWarning: steroidAbxGuard,
	}
}

// BuildDefaultResuscitationWoundCarePlan returns an evidence-based ResuscitationWoundCarePlan.
func BuildDefaultResuscitationWoundCarePlan(
	species string,
	weightKg, tbsaPercent, total24hML, first8hRateMLHr, next16hRateMLHr, serumAlbuminGDL float64,
	circumferentialThoracicEschar bool,
) domain.ResuscitationWoundCarePlan {
	escharPlan := "Aseptic Hydrotherapy & Staged Debridement: Cool recent thermal burns (< 20 min) with running room-temperature water (15–20°C) for 20 min (NEVER use ice water — induces vasoconstrictive dermal necrosis and hypothermia); perform conservative staged sharp/autolytic debridement preserving Jackson's zone of stasis."
	if circumferentialThoracicEschar {
		escharPlan = "Emergent Thoracic/Limb Escharotomy & Staged Debridement: Immediate longitudinal escharotomy incisions along lateral thoracic wall and/or medial-lateral limb axes through full-thickness leathery eschar to restore chest compliance and distal perfusion, followed by staged surgical debridement and wet-to-dry / hydrocolloid tie-over dressings."
	}

	return domain.ResuscitationWoundCarePlan{
		ParklandCrystalloidSchedule: fmt.Sprintf("Isotonic Buffered Crystalloid (Lactated Ringer's / Normosol-R): Total 24h Parkland = %.1f mL (%.1f kg, %.1f%% TBSA) — First 8h from time of burn at %.1f mL/hr, Next 16h at %.1f mL/hr, titrated to Urine Output 1.0–2.0 mL/kg/hr; avoid synthetic/natural colloids in first 12–24h.",
			total24hML, weightKg, tbsaPercent, first8hRateMLHr, next16hRateMLHr),
		SilverSulfadiazineHoneyWoundPlan: "Topical Antimicrobial Barrier: Apply 1% Silver Sulfadiazine (SSD) water-miscible cream (broad Gram-negative Pseudomonas & Gram-positive coverage; penetrates eschar) OR sterile medical-grade Leptospermum Manuka Honey (UMF >= 15+ osmotic autolytic debridement) under non-adherent contact dressings q12–24h.",
		EscharotomyWetDebridementPlan:    escharPlan,
		MultimodalBurnAnalgesiaCRI:       "Multimodal Neuropathic & Nociceptive Burn Analgesia: Full mu-opioid agonist CRI (Fentanyl 3–5 mcg/kg/hr or Methadone 0.1–0.2 mg/kg IV q4h) + Ketamine sub-anesthetic CRI (2–10 mcg/kg/min NMDA wind-up blockade) + Lidocaine CRI (canine 25–50 mcg/kg/min; avoid systemic lidocaine toxicity in cats); strictly avoid NSAIDs during hypovolemic burn shock.",
		AlbuminOncotic24hTransition: fmt.Sprintf("Post-24h Capillary Integrity & Oncotic Transition: Once endothelial capillary leak seals (> 18–24h post-burn) and Serum Albumin < 2.0 g/dL (current %.1f g/dL), transition to Canine/Feline Plasma or 5%% Canine Albumin infusion + high-protein enteral nutrition (1.5–2.0x RER).",
			serumAlbuminGDL),
		CornealThermalAirwayProtection: "Corneal Fluorescein & Smoke Inhalation Protection: Perform bilateral fluorescein stain immediately for corneal flash ulcers (topical preservative-free artificial tears + broad-spectrum ophthalmic ointment q4–6h); provide humidified 40–60% O2 and nebulized 0.9% NaCl + N-acetylcysteine for carbonaceous airway casts.",
	}
}

func (s *BurnService) seed() {
	now := time.Now().UTC()

	// BURN-OP-101: Ember — Canine, Australian Cattle Dog, 20.0 kg, ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE
	park101 := BuildTBSAParklandAssessmentPlan(
		"Canine", 20.0,
		domain.BurnDepthFullThicknessCircumferentialEschar,
		30.0, 2400.0, 150.0, 75.0, 150.0, 0.8, 1.7, 58.0,
		true, true,
	)
	wound101 := BuildDefaultResuscitationWoundCarePlan("Canine", 20.0, 30.0, 2400.0, 150.0, 75.0, 1.7, true)
	op101 := &domain.ThermalBurnOperation{
		ID:                            "BURN-OP-101",
		CaseID:                        "CASE-BURN-101",
		PetName:                       "Ember",
		Species:                       "Canine",
		Breed:                         "Australian Cattle Dog",
		WeightKg:                      20.0,
		Status:                        domain.ThermalBurnStatusAcuteTriage,
		DepthClass:                    domain.BurnDepthFullThicknessCircumferentialEschar,
		TBSAPercent:                   30.0,
		Total24hParklandML:            2400.0,
		First8hRateMLHr:               150.0,
		Next16hRateMLHr:               75.0,
		CurrentFluidRateMLHr:          150.0,
		UrineOutputMLKgHr:             0.8,
		SerumAlbuminGDL:               1.7,
		PCVPercent:                    58.0,
		CircumferentialThoracicEschar: true,
		SingedVibrissaeSootAirway:     true,
		ParklandPlan:                  park101,
		WoundPlan:                     wound101,
		BurnChecks: []domain.BurnFluidWoundCheck{
			{
				ID:                   "BURN-CHK-101-1",
				TBSAPercent:          30.0,
				CurrentFluidRateMLHr: 150.0,
				UrineOutputMLKgHr:    0.6,
				SerumAlbuminGDL:      1.8,
				PCVPercent:           61.0,
				RecordedBy:           "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
				ClinicalNote:         "Extracted from barn structure fire 1h ago: 30% TBSA full-thickness circumferential thoracic eschar restricting chest excursion, singed vibrissae, PCV 61% hemoconcentration; initiated Parkland first-8h LRS at 150.0 mL/hr.",
				RecordedAt:           now.Add(-55 * time.Minute),
			},
			{
				ID:                   "BURN-CHK-101-2",
				TBSAPercent:          30.0,
				CurrentFluidRateMLHr: 150.0,
				UrineOutputMLKgHr:    0.8,
				SerumAlbuminGDL:      1.7,
				PCVPercent:           58.0,
				RecordedBy:           "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
				ClinicalNote:         "Urine output rising from 0.6 to 0.8 mL/kg/hr, albumin 1.7 g/dL (colloids strictly withheld during 24h capillary leak window); prepping lateral thoracic escharotomy and 1% Silver Sulfadiazine barrier.",
				RecordedAt:           now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-65 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// BURN-OP-102: Cinder — Feline, Domestic Shorthair, 4.0 kg, PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED
	park102 := BuildTBSAParklandAssessmentPlan(
		"Feline", 4.0,
		domain.BurnDepthDeepPartialThicknessDermal,
		22.0, 220.0, 13.8, 6.9, 13.8, 1.4, 2.1, 44.0,
		false, true,
	)
	wound102 := BuildDefaultResuscitationWoundCarePlan("Feline", 4.0, 22.0, 220.0, 13.8, 6.9, 2.1, false)
	op102 := &domain.ThermalBurnOperation{
		ID:                            "BURN-OP-102",
		CaseID:                        "CASE-BURN-102",
		PetName:                       "Cinder",
		Species:                       "Feline",
		Breed:                         "Domestic Shorthair",
		WeightKg:                      4.0,
		Status:                        domain.ThermalBurnStatusParklandStaged,
		DepthClass:                    domain.BurnDepthDeepPartialThicknessDermal,
		TBSAPercent:                   22.0,
		Total24hParklandML:            220.0,
		First8hRateMLHr:               13.8,
		Next16hRateMLHr:               6.9,
		CurrentFluidRateMLHr:          13.8,
		UrineOutputMLKgHr:             1.4,
		SerumAlbuminGDL:               2.1,
		PCVPercent:                    44.0,
		CircumferentialThoracicEschar: false,
		SingedVibrissaeSootAirway:     true,
		ParklandPlan:                  park102,
		WoundPlan:                     wound102,
		BurnChecks: []domain.BurnFluidWoundCheck{
			{
				ID:                   "BURN-CHK-102-1",
				TBSAPercent:          22.0,
				CurrentFluidRateMLHr: 13.8,
				UrineOutputMLKgHr:    1.4,
				SerumAlbuminGDL:      2.1,
				PCVPercent:           44.0,
				RecordedBy:           "Dr. Hannah Lin, DACVECC",
				ClinicalNote:         "Feline Parkland protocol (2.5 mL/kg/%TBSA = 220.0 mL/24h; First 8h at 13.8 mL/hr) staged with Fentanyl CRI, sterile Manuka Honey + 1% SSD paw/dorsal dressings, and humidified O2 cage.",
				RecordedAt:           now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// BURN-OP-103: Blaze — Canine, Dalmatian, 24.0 kg, GRANULATION_BED_NORMOALBUMINEMIC_CLEARED
	park103 := BuildTBSAParklandAssessmentPlan(
		"Canine", 24.0,
		domain.BurnDepthThermalAirwayCornealFlash,
		8.0, 768.0, 48.0, 24.0, 35.0, 1.9, 2.8, 41.0,
		false, false,
	)
	wound103 := BuildDefaultResuscitationWoundCarePlan("Canine", 24.0, 8.0, 768.0, 48.0, 24.0, 2.8, false)
	dischargedAt := now.Add(-30 * time.Minute)
	op103 := &domain.ThermalBurnOperation{
		ID:                            "BURN-OP-103",
		CaseID:                        "CASE-BURN-103",
		PetName:                       "Blaze",
		Species:                       "Canine",
		Breed:                         "Dalmatian",
		WeightKg:                      24.0,
		Status:                        domain.ThermalBurnStatusGranulationCleared,
		DepthClass:                    domain.BurnDepthThermalAirwayCornealFlash,
		TBSAPercent:                   8.0,
		Total24hParklandML:            768.0,
		First8hRateMLHr:               48.0,
		Next16hRateMLHr:               24.0,
		CurrentFluidRateMLHr:          35.0,
		UrineOutputMLKgHr:             1.9,
		SerumAlbuminGDL:               2.8,
		PCVPercent:                    41.0,
		CircumferentialThoracicEschar: false,
		SingedVibrissaeSootAirway:     false,
		ParklandPlan:                  park103,
		WoundPlan:                     wound103,
		BurnChecks: []domain.BurnFluidWoundCheck{
			{
				ID:                   "BURN-CHK-103-1",
				TBSAPercent:          12.0,
				CurrentFluidRateMLHr: 48.0,
				UrineOutputMLKgHr:    1.6,
				SerumAlbuminGDL:      2.4,
				PCVPercent:           46.0,
				RecordedBy:           "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
				ClinicalNote:         "Initial 24h Parkland resuscitation completed alongside 1% Silver Sulfadiazine dressing changes and corneal flash burn lubrication.",
				RecordedAt:           now.Add(-20 * time.Hour),
			},
			{
				ID:                   "BURN-CHK-103-2",
				TBSAPercent:          8.0,
				CurrentFluidRateMLHr: 35.0,
				UrineOutputMLKgHr:    1.9,
				SerumAlbuminGDL:      2.8,
				PCVPercent:           41.0,
				RecordedBy:           "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
				ClinicalNote:         "Healthy vascularized granulation bed established across residual 8.0% TBSA, Urine Output 1.9 mL/kg/hr, Serum Albumin 2.8 g/dL, PCV 41.0%, corneal fluorescein negative.",
				RecordedAt:           now.Add(-45 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
		DischargeSummary: "Healthy vascularized granulation bed established (residual 8.0% TBSA), normoalbuminemic (Serum Albumin 2.8 g/dL), Urine Output 1.9 mL/kg/hr, PCV 41.0%, cleared for step-down reconstructive wound care.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Thermal Burn operations, optionally filtered by statusFilter.
func (s *BurnService) ListOperations(statusFilter string) []domain.ThermalBurnOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.ThermalBurnOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneBurnOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Thermal Burn operation by ID.
func (s *BurnService) GetOperation(id string) (*domain.ThermalBurnOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("thermal burn operation %q not found", id)
	}
	cp := cloneBurnOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all Thermal Burn operations.
func (s *BurnService) GetOverviewMetrics() domain.ThermalBurnOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.ThermalBurnOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.ThermalBurnStatusAcuteTriage:
			m.AcuteBurnTriageCount++
		case domain.ThermalBurnStatusParklandStaged:
			m.ParklandStagedCount++
		case domain.ThermalBurnStatusGranulationCleared:
			m.ClearedGranulationCount++
		}
		if op.TBSAPercent >= 20.0 {
			m.SevereTBSAOver20PctCount++
		}
		if op.CircumferentialThoracicEschar {
			m.CircumferentialEscharCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Thermal Burn & Parkland triage operation.
func (s *BurnService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	depthClass domain.BurnDepthClassification,
	tbsaPercent, currentFluidRateMLHr, urineOutputMLKgHr, serumAlbuminGDL, pcvPercent float64,
	circumferentialThoracicEschar, singedVibrissaeSootAirway bool,
) (*domain.ThermalBurnOperation, error) {
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
	if !depthClass.IsValid() {
		return nil, fmt.Errorf("invalid depth_class: %q", depthClass)
	}
	if tbsaPercent <= 0 || tbsaPercent > 100.0 {
		return nil, fmt.Errorf("tbsa_percent must be in (0, 100.0], got %v", tbsaPercent)
	}
	if currentFluidRateMLHr < 0 || currentFluidRateMLHr > 2500.0 {
		return nil, fmt.Errorf("current_fluid_rate_ml_hr must be in [0, 2500.0], got %v", currentFluidRateMLHr)
	}
	if urineOutputMLKgHr < 0 || urineOutputMLKgHr > 25.0 {
		return nil, fmt.Errorf("urine_output_ml_kg_hr must be in [0, 25.0], got %v", urineOutputMLKgHr)
	}
	if serumAlbuminGDL <= 0 || serumAlbuminGDL > 6.5 {
		return nil, fmt.Errorf("serum_albumin_g_dl must be in (0, 6.5], got %v", serumAlbuminGDL)
	}
	if pcvPercent <= 0 || pcvPercent > 85.0 {
		return nil, fmt.Errorf("pcv_percent must be in (0, 85.0], got %v", pcvPercent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("BURN-OP-%d", s.seq)
	now := time.Now().UTC()

	total24h, first8h, next16h := ComputeParkland24hResuscitation(trimmedSpecies, weightKg, tbsaPercent)
	if currentFluidRateMLHr == 0 {
		currentFluidRateMLHr = first8h
	}

	parklandPlan := BuildTBSAParklandAssessmentPlan(
		trimmedSpecies,
		weightKg,
		depthClass,
		tbsaPercent, total24h, first8h, next16h, currentFluidRateMLHr, urineOutputMLKgHr, serumAlbuminGDL, pcvPercent,
		circumferentialThoracicEschar, singedVibrissaeSootAirway,
	)
	woundPlan := BuildDefaultResuscitationWoundCarePlan(
		trimmedSpecies,
		weightKg, tbsaPercent, total24h, first8h, next16h, serumAlbuminGDL,
		circumferentialThoracicEschar,
	)

	op := &domain.ThermalBurnOperation{
		ID:                            opID,
		CaseID:                        trimmedCase,
		PetName:                       trimmedPet,
		Species:                       trimmedSpecies,
		Breed:                         trimmedBreed,
		WeightKg:                      round1(weightKg),
		Status:                        domain.ThermalBurnStatusAcuteTriage,
		DepthClass:                    depthClass,
		TBSAPercent:                   round1(tbsaPercent),
		Total24hParklandML:            total24h,
		First8hRateMLHr:               first8h,
		Next16hRateMLHr:               next16h,
		CurrentFluidRateMLHr:          round1(currentFluidRateMLHr),
		UrineOutputMLKgHr:             round1(urineOutputMLKgHr),
		SerumAlbuminGDL:               round1(serumAlbuminGDL),
		PCVPercent:                    round1(pcvPercent),
		CircumferentialThoracicEschar: circumferentialThoracicEschar,
		SingedVibrissaeSootAirway:     singedVibrissaeSootAirway,
		ParklandPlan:                  parklandPlan,
		WoundPlan:                     woundPlan,
		BurnChecks:                    []domain.BurnFluidWoundCheck{},
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneBurnOperation(op)
	return &cp, nil
}

// StageParklandWoundPlan stages the Parkland crystalloid schedule, 1% Silver Sulfadiazine / Manuka Honey
// wound barrier plan, escharotomy & staged debridement plan, multimodal burn analgesia CRI, post-24h
// albumin/oncotic transition, and corneal/airway protection, transitioning status to PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED.
func (s *BurnService) StageParklandWoundPlan(
	id, parklandSchedule, ssdHoneyPlan, escharotomyPlan, analgesiaCRI, albuminTransition, airwayCornealPlan string,
) (*domain.ThermalBurnOperation, error) {
	trimmedParkland := strings.TrimSpace(parklandSchedule)
	if trimmedParkland == "" {
		return nil, fmt.Errorf("parkland_crystalloid_schedule is required")
	}
	trimmedSSD := strings.TrimSpace(ssdHoneyPlan)
	if trimmedSSD == "" {
		return nil, fmt.Errorf("silver_sulfadiazine_honey_wound_plan is required")
	}
	trimmedEschar := strings.TrimSpace(escharotomyPlan)
	if trimmedEschar == "" {
		return nil, fmt.Errorf("escharotomy_wet_debridement_plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("thermal burn operation %q not found", id)
	}

	defaults := BuildDefaultResuscitationWoundCarePlan(
		op.Species,
		op.WeightKg, op.TBSAPercent, op.Total24hParklandML, op.First8hRateMLHr, op.Next16hRateMLHr, op.SerumAlbuminGDL,
		op.CircumferentialThoracicEschar,
	)

	trimmedAnalgesia := strings.TrimSpace(analgesiaCRI)
	if trimmedAnalgesia == "" {
		trimmedAnalgesia = defaults.MultimodalBurnAnalgesiaCRI
	}
	trimmedAlbumin := strings.TrimSpace(albuminTransition)
	if trimmedAlbumin == "" {
		trimmedAlbumin = defaults.AlbuminOncotic24hTransition
	}
	trimmedAirway := strings.TrimSpace(airwayCornealPlan)
	if trimmedAirway == "" {
		trimmedAirway = defaults.CornealThermalAirwayProtection
	}

	op.WoundPlan = domain.ResuscitationWoundCarePlan{
		ParklandCrystalloidSchedule:      trimmedParkland,
		SilverSulfadiazineHoneyWoundPlan: trimmedSSD,
		EscharotomyWetDebridementPlan:    trimmedEschar,
		MultimodalBurnAnalgesiaCRI:       trimmedAnalgesia,
		AlbuminOncotic24hTransition:      trimmedAlbumin,
		CornealThermalAirwayProtection:   trimmedAirway,
	}
	op.Status = domain.ThermalBurnStatusParklandStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneBurnOperation(op)
	return &cp, nil
}

// RecordBurnCheck appends a BurnFluidWoundCheck and updates op.TBSAPercent,
// op.CurrentFluidRateMLHr, op.UrineOutputMLKgHr, op.SerumAlbuminGDL, and op.PCVPercent.
func (s *BurnService) RecordBurnCheck(
	id string,
	tbsaPercent, currentFluidRateMLHr, urineOutputMLKgHr, serumAlbuminGDL, pcvPercent float64,
	recordedBy, clinicalNote string,
) (*domain.ThermalBurnOperation, error) {
	if tbsaPercent <= 0 || tbsaPercent > 100.0 {
		return nil, fmt.Errorf("tbsa_percent must be in (0, 100.0], got %v", tbsaPercent)
	}
	if currentFluidRateMLHr < 0 || currentFluidRateMLHr > 2500.0 {
		return nil, fmt.Errorf("current_fluid_rate_ml_hr must be in [0, 2500.0], got %v", currentFluidRateMLHr)
	}
	if urineOutputMLKgHr < 0 || urineOutputMLKgHr > 25.0 {
		return nil, fmt.Errorf("urine_output_ml_kg_hr must be in [0, 25.0], got %v", urineOutputMLKgHr)
	}
	if serumAlbuminGDL <= 0 || serumAlbuminGDL > 6.5 {
		return nil, fmt.Errorf("serum_albumin_g_dl must be in (0, 6.5], got %v", serumAlbuminGDL)
	}
	if pcvPercent <= 0 || pcvPercent > 85.0 {
		return nil, fmt.Errorf("pcv_percent must be in (0, 85.0], got %v", pcvPercent)
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
		return nil, fmt.Errorf("thermal burn operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.BurnFluidWoundCheck{
		ID:                   fmt.Sprintf("BURN-CHK-%s-%d", op.ID, len(op.BurnChecks)+1),
		TBSAPercent:          round1(tbsaPercent),
		CurrentFluidRateMLHr: round1(currentFluidRateMLHr),
		UrineOutputMLKgHr:    round1(urineOutputMLKgHr),
		SerumAlbuminGDL:      round1(serumAlbuminGDL),
		PCVPercent:           round1(pcvPercent),
		RecordedBy:           trimmedBy,
		ClinicalNote:         trimmedNote,
		RecordedAt:           now,
	}

	op.BurnChecks = append(op.BurnChecks, chk)
	op.TBSAPercent = chk.TBSAPercent
	op.CurrentFluidRateMLHr = chk.CurrentFluidRateMLHr
	op.UrineOutputMLKgHr = chk.UrineOutputMLKgHr
	op.SerumAlbuminGDL = chk.SerumAlbuminGDL
	op.PCVPercent = chk.PCVPercent

	total24h, first8h, next16h := ComputeParkland24hResuscitation(op.Species, op.WeightKg, op.TBSAPercent)
	op.Total24hParklandML = total24h
	op.First8hRateMLHr = first8h
	op.Next16hRateMLHr = next16h

	op.ParklandPlan = BuildTBSAParklandAssessmentPlan(
		op.Species,
		op.WeightKg,
		op.DepthClass,
		op.TBSAPercent, op.Total24hParklandML, op.First8hRateMLHr, op.Next16hRateMLHr,
		op.CurrentFluidRateMLHr, op.UrineOutputMLKgHr, op.SerumAlbuminGDL, op.PCVPercent,
		op.CircumferentialThoracicEschar, op.SingedVibrissaeSootAirway,
	)
	op.UpdatedAt = now

	cp := cloneBurnOperation(op)
	return &cp, nil
}

// ClearBurnDischarge transitions the operation to GRANULATION_BED_NORMOALBUMINEMIC_CLEARED
// once a healthy vascularized granulation bed is established and oncotic/renal perfusion is restored.
func (s *BurnService) ClearBurnDischarge(id, dischargedBy, summary string) (*domain.ThermalBurnOperation, error) {
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
		return nil, fmt.Errorf("thermal burn operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.ThermalBurnStatusGranulationCleared
	op.CircumferentialThoracicEschar = false
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneBurnOperation(op)
	return &cp, nil
}

func cloneBurnOperation(op *domain.ThermalBurnOperation) domain.ThermalBurnOperation {
	cp := *op
	if op.BurnChecks != nil {
		cp.BurnChecks = make([]domain.BurnFluidWoundCheck, len(op.BurnChecks))
		copy(cp.BurnChecks, op.BurnChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
