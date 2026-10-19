package fipgs441524

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// FIPService manages Acute Feline Infectious Peritonitis (FIP — Effusive Wet & Non-Effusive Dry/Ocular/Neurological),
// Rivalta Test, Albumin:Globulin Ratio < 0.4, GS-441524 / Remdesivir Nucleoside Analog & Blood-Brain/Ocular Barrier
// Dosing triage operations in a thread-safe store.
type FIPService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.FIPGS441524Operation
	order []string
	seq   int
}

// NewFIPService initializes a FIPService with 3 seeded operations (FIP-OP-101, FIP-OP-102, FIP-OP-103).
func NewFIPService() *FIPService {
	s := &FIPService{
		ops:   make(map[string]*domain.FIPGS441524Operation),
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

// BuildRivaltaBiomarkerAssessmentPlan computes the clinical interpretation and contraindication guards
// for an Acute Feline Infectious Peritonitis (FIP) operation.
func BuildRivaltaBiomarkerAssessmentPlan(
	phenotype domain.FIPClinicalPhenotype,
	agRatio, totalBilirubinMgDL, hematocritPercent, dailyGS441524MgKg float64,
	rivaltaPositive, pleuralDyspneaPresent bool,
) domain.RivaltaBiomarkerAssessmentPlan {
	var patho string
	switch phenotype {
	case domain.FIPPhenotypeEffusiveCavitary:
		patho = "Effusive (Wet) Cavitary Feline Infectious Peritonitis (FIP) — Mutated Feline Coronavirus (FCoV Spike-gene internal mutation enabling monocyte/macrophage tropism) inducing immune-complex pyogranulomatous phlebitis/perivasculitis, vascular endothelial VEGF hyperpermeability, and straw-colored high-protein exudative ascites/pleural effusion."
	case domain.FIPPhenotypeNeurologicalCNS:
		patho = "Non-Effusive Neurological (CNS) Feline Infectious Peritonitis (FIP) — Pyogranulomatous meningoencephalomyelitis, ependymitis, and choroid plexitis causing cerebellar/vestibular ataxia, nystagmus, seizures, and obstructive hydrocephalus across the Blood-Brain Barrier."
	case domain.FIPPhenotypeOcularUveitis:
		patho = "Non-Effusive Ocular Feline Infectious Peritonitis (FIP) — Breakdown of the Blood-Aqueous/Ocular Barrier with pyogranulomatous anterior uveitis, mutton-fat keratic precipitates, aqueous flare, fibrinous hypopyon/hyphema, and chorioretinitis."
	default:
		patho = "Feline Infectious Peritonitis (FIP) — Systemic macrophage-tropic FCoV pyogranulomatous vasculitis and hyperglobulinemia."
	}

	rivaltaStr := "NEGATIVE / NOT PERFORMED"
	if rivaltaPositive {
		rivaltaStr = "POSITIVE (Acetic Acid Drop Retains Jelly-Like Globular Pellicle)"
	}
	rivaltaNote := fmt.Sprintf("Rivalta Test: %s | Albumin:Globulin (A:G) Ratio %.2f | Total Bilirubin %.1f mg/dL | HCT %.1f%% — ", rivaltaStr, agRatio, totalBilirubinMgDL, hematocritPercent)
	if agRatio < 0.40 {
		rivaltaNote += "HIGHLY DIAGNOSTIC FOR FIP (A:G Ratio < 0.40 with polyclonal hypergammaglobulinemia, hypoalbuminemia, microcytic non-regenerative anemia of inflammatory disease, and hyperbilirubinemia)."
	} else if agRatio >= 0.60 {
		rivaltaNote += "A:G Ratio >= 0.60 (Normalized / Resolving Hyperglobulinemia — favorable antiviral response toward 84-day cure threshold)."
	} else {
		rivaltaNote += "Borderline A:G Ratio (0.40–0.59) — intermediate hyperglobulinemia improving on nucleoside analog therapy."
	}

	agpPCR := "Serum Alpha-1-Acid Glycoprotein (AGP > 1,500 µg/mL acute-phase protein) + Effusion/CSF/Aqueous Humor FCoV RT-qPCR (Spike S-gene M1058L / S1060A mutation) & FCoV Antigen Immunohistochemistry/Immunocytochemistry."

	drainageGuard := "STRICT CONTRAINDICATION OF COMPLETE ABDOMINAL ASCITES DRAINAGE: DO NOT perform full-volume abdominocentesis evacuation of FIP ascitic fluid — ascitic exudate is rich in albumin and immunoglobulins; complete drainage precipitates acute third-space fluid redistribution, severe hypoproteinemia, and refractory hypovolemic shock."
	if pleuralDyspneaPresent {
		drainageGuard = "IMMEDIATE THERAPEUTIC THORACOCENTESIS INDICATED FOR PLEURAL EFFUSION DYSPNEA + STRICT ASCITES DRAINAGE CONTRAINDICATION: Perform immediate ultrasound-guided thoracocentesis to relieve life-threatening pleural restriction, while strictly avoiding complete abdominal ascites drainage (< 20–30% volume only if severe abdominal compartment dyspnea)."
	}

	barrierEscalation := fmt.Sprintf("Phenotype-Stratified GS-441524 / Remdesivir Dosing (Current %.1f mg/kg/day): Effusive/Dry Cavitary 6–10 mg/kg/day | Ocular 15 mg/kg/day (Blood-Ocular Barrier) | Neurological 20 mg/kg/day (10 mg/kg PO/SC q12h split dosing required to achieve therapeutic CSF concentration across the Blood-Brain Barrier).", dailyGS441524MgKg)

	toxicityWarn := "CONTRAINDICATED ADJUNCTS GUARD: (1) STRICTLY AVOID L-Lysine supplementation (antagonizes feline essential L-Arginine, precipitating hyperammonemia, and impairs GS-441524/host immune response); (2) STRICTLY AVOID Enrofloxacin (Baytril) > 5 mg/kg/day in cats (causes acute irreversible diffuse feline retinal degeneration and blindness; use Pradofloxacin or Doxycycline if secondary bacterial coverage is needed)."

	return domain.RivaltaBiomarkerAssessmentPlan{
		PhenotypePathophysiology:                patho,
		RivaltaAGRatioInterpretation:            rivaltaNote,
		Alpha1AcidGlycoproteinFCoVPCR:           agpPCR,
		CompleteAscitesDrainageContraindication: drainageGuard,
		BarrierPenetrationDoseEscalation:        barrierEscalation,
		FluoroquinoloneRetinalToxicityWarning:   toxicityWarn,
	}
}

// BuildDefaultNucleosideAntiviralProtocolPlan returns an evidence-based 84-day (12-week)
// GS-441524 / Remdesivir NucleosideAntiviralProtocolPlan.
func BuildDefaultNucleosideAntiviralProtocolPlan(
	weightKg float64,
	phenotype domain.FIPClinicalPhenotype,
	dailyGS441524MgKg float64,
	pleuralDyspneaPresent bool,
) domain.NucleosideAntiviralProtocolPlan {
	if dailyGS441524MgKg <= 0 {
		switch phenotype {
		case domain.FIPPhenotypeNeurologicalCNS:
			dailyGS441524MgKg = 20.0
		case domain.FIPPhenotypeOcularUveitis:
			dailyGS441524MgKg = 15.0
		default:
			dailyGS441524MgKg = 10.0
		}
	}
	totalDailyMg := round1(weightKg * dailyGS441524MgKg)
	splitDoseMg := round1(totalDailyMg / 2.0)

	dosingPlan := fmt.Sprintf("GS-441524 Nucleoside Analog %.1f mg/kg/day (%.1f mg/day total for %.1f kg cat; or %.1f mg PO q12h split dosing for optimal bioavailability & Blood-Brain/Ocular Barrier penetration) for a mandatory 84-Day (12-Week) uninterrupted course (or initial Remdesivir 10–20 mg/kg IV slow infusion over 30–60 min for NPO/critical ICU patients transitioning to oral GS-441524).", dailyGS441524MgKg, totalDailyMg, weightKg, splitDoseMg)

	weightRecalc := fmt.Sprintf("Weekly Weight-Adjusted Dose Up-Titration (Current %.1f kg): Weigh patient weekly on a pediatric scale and IMMEDIATELY RECALCULATE GS-441524 dose upward as kitten/cat gains weight and resolves cachexia — NEVER reduce the daily mg dose even if transient weight fluctuation occurs.", weightKg)

	centesisLimit := "Pleural vs. Ascites Centesis Protocol: Perform immediate therapeutic thoracocentesis (7th–8th ICS) if pleural effusion causes tachypnea/orthopnea; NEVER drain > 20–30% of abdominal ascites to prevent acute third-space hypoproteinemic shock (effusion resolves spontaneously within 7–14 days of GS-441524 initiation)."
	if pleuralDyspneaPresent {
		centesisLimit = "ACTIVE PLEURAL EFFUSION DYSPNEA: Immediate bilateral therapeutic thoracocentesis with butterfly/over-the-needle catheter + oxygen cage stabilization; strictly limit any abdominal ascites relief to < 20–30% volume."
	}

	ocularNote := "Ocular Anterior Uveitis Support: Topical Prednisolone Acetate 1% ophthalmic suspension q6–12h (anti-inflammatory across corneal epithelium) + Topical Atropine 1% ophthalmic to maintain mydriasis and prevent posterior synechiae/secondary glaucoma (avoid systemic immunosuppressant steroids once antiviral therapy is active unless severe CNS cerebral edema)."
	if phenotype == domain.FIPPhenotypeNeurologicalCNS {
		ocularNote = "Neurological CNS & Ocular Barrier Support: Levetiracetam 20–30 mg/kg IV/PO q8h for FIP-induced seizures + short-course anti-inflammatory Prednisolone (0.5–1.0 mg/kg/day tapered over 7–10 days) ONLY if severe raised ICP / obstructive hydrocephalus."
	}

	hepatoSupport := "Hepatoprotectant & Hematopoietic Support: Denamarin (S-Adenosylmethionine SAMe 90 mg + Silybin) PO q24h on empty stomach for hyperbilirubinemia/hepatic granulomas + Cyanocobalamin (Vitamin B12) 250 µg SC weekly for non-regenerative anemia + Maropitant/Mirtazapine nutritional support; STRICTLY AVOID L-Lysine & Enrofloxacin."

	monitoringPlan := "Mandatory 84-Day (12-Week) Checkpoint Schedule: Serial CBC (HCT %, Reticulocytes, Lymphocytes), Chemistry (Total Protein, Albumin, Globulin, A:G Ratio target > 0.60, Total Bilirubin, ALT), Serum AGP (< 300 µg/mL), and Pediatric Weight at Day 0, Day 28 (Week 4), Day 56 (Week 8), and Day 84 (Week 12 Cure Verification prior to stopping GS-441524) + 12-Week Post-Treatment Relapse Surveillance."

	return domain.NucleosideAntiviralProtocolPlan{
		GS441524OrRemdesivirDosing:      dosingPlan,
		WeightAdjustedDoseRecalcNote:    weightRecalc,
		TherapeuticThoracocentesisLimit: centesisLimit,
		OcularTopicalAtropinePredNote:   ocularNote,
		HepatoprotectantB12SupportPlan:  hepatoSupport,
		Day28Day56Day84MonitoringPlan:   monitoringPlan,
	}
}

func (s *FIPService) seed() {
	now := time.Now().UTC()

	// FIP-OP-101: Bramble — Feline, British Shorthair Kitten, 2.4 kg, ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE
	bio101 := BuildRivaltaBiomarkerAssessmentPlan(
		domain.FIPPhenotypeEffusiveCavitary,
		0.28, 2.6, 21.0, 10.0,
		true, true,
	)
	anti101 := BuildDefaultNucleosideAntiviralProtocolPlan(
		2.4, domain.FIPPhenotypeEffusiveCavitary,
		10.0, true,
	)
	op101 := &domain.FIPGS441524Operation{
		ID:                            "FIP-OP-101",
		CaseID:                        "CASE-FIP-101",
		PetName:                       "Bramble",
		Species:                       "Feline",
		Breed:                         "British Shorthair Kitten",
		WeightKg:                      2.4,
		Status:                        domain.FIPStatusAcuteTriage,
		Phenotype:                     domain.FIPPhenotypeEffusiveCavitary,
		TreatmentDay:                  1,
		AlbuminGlobulinRatio:          0.28,
		TotalBilirubinMgDL:            2.6,
		HematocritPercent:             21.0,
		DailyGS441524MgKg:             10.0,
		RivaltaTestPositive:           true,
		PleuralEffusionDyspneaPresent: true,
		BiomarkerPlan:                 bio101,
		AntiviralPlan:                 anti101,
		ProgressChecks: []domain.FIPAntiviralProgressCheck{
			{
				ID:                   "FIP-CHK-101-1",
				TreatmentDay:         1,
				WeightKg:             2.4,
				AlbuminGlobulinRatio: 0.28,
				TotalBilirubinMgDL:   2.6,
				HematocritPercent:    21.0,
				RecordedBy:           "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
				ClinicalNote:         "Day 1 ICU Admission: 6-month British Shorthair kitten with bicavitary pleural effusion & abdominal ascites; Rivalta Test POSITIVE, straw-colored viscous exudate (TP 6.8 g/dL), A:G Ratio 0.28 (< 0.4), Total Bilirubin 2.6 mg/dL, HCT 21.0%.",
				RecordedAt:           now.Add(-50 * time.Minute),
			},
			{
				ID:                   "FIP-CHK-101-2",
				TreatmentDay:         1,
				WeightKg:             2.4,
				AlbuminGlobulinRatio: 0.28,
				TotalBilirubinMgDL:   2.6,
				HematocritPercent:    21.0,
				RecordedBy:           "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
				ClinicalNote:         "Therapeutic thoracocentesis relieved 65 mL pleural effusion for dyspnea; abdominal ascites left undrained to prevent acute third-space hypoproteinemic shock; initiating Remdesivir/GS-441524 10 mg/kg/day.",
				RecordedAt:           now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// FIP-OP-102: Cosmo — Feline, Ragdoll, 3.6 kg, GS441524_REMDESIVIR_84DAY_STAGED
	bio102 := BuildRivaltaBiomarkerAssessmentPlan(
		domain.FIPPhenotypeNeurologicalCNS,
		0.48, 0.6, 29.0, 20.0,
		true, false,
	)
	anti102 := BuildDefaultNucleosideAntiviralProtocolPlan(
		3.6, domain.FIPPhenotypeNeurologicalCNS,
		20.0, false,
	)
	op102 := &domain.FIPGS441524Operation{
		ID:                            "FIP-OP-102",
		CaseID:                        "CASE-FIP-102",
		PetName:                       "Cosmo",
		Species:                       "Feline",
		Breed:                         "Ragdoll",
		WeightKg:                      3.6,
		Status:                        domain.FIPStatusGS441524Staged,
		Phenotype:                     domain.FIPPhenotypeNeurologicalCNS,
		TreatmentDay:                  28,
		AlbuminGlobulinRatio:          0.48,
		TotalBilirubinMgDL:            0.6,
		HematocritPercent:             29.0,
		DailyGS441524MgKg:             20.0,
		RivaltaTestPositive:           true,
		PleuralEffusionDyspneaPresent: false,
		BiomarkerPlan:                 bio102,
		AntiviralPlan:                 anti102,
		ProgressChecks: []domain.FIPAntiviralProgressCheck{
			{
				ID:                   "FIP-CHK-102-1",
				TreatmentDay:         28,
				WeightKg:             3.6,
				AlbuminGlobulinRatio: 0.48,
				TotalBilirubinMgDL:   0.6,
				HematocritPercent:    29.0,
				RecordedBy:           "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
				ClinicalNote:         "Day 28 (Week 4) Neurological FIP Checkpoint: Pelvic limb ataxia and vertical nystagmus resolved on escalated GS-441524 20 mg/kg/day (10 mg/kg PO q12h split dosing for Blood-Brain Barrier penetration); weight increased from 3.1 to 3.6 kg (dose up-titrated to 72 mg/day), A:G Ratio improved to 0.48.",
				RecordedAt:           now.Add(-75 * time.Minute),
			},
		},
		CreatedAt: now.Add(-28 * 24 * time.Hour),
		UpdatedAt: now.Add(-75 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// FIP-OP-103: Saffron — Feline, Abyssinian, 4.1 kg, AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED
	bio103 := BuildRivaltaBiomarkerAssessmentPlan(
		domain.FIPPhenotypeOcularUveitis,
		0.78, 0.2, 38.0, 15.0,
		false, false,
	)
	anti103 := BuildDefaultNucleosideAntiviralProtocolPlan(
		4.1, domain.FIPPhenotypeOcularUveitis,
		15.0, false,
	)
	dischargedAt := now.Add(-25 * time.Minute)
	op103 := &domain.FIPGS441524Operation{
		ID:                            "FIP-OP-103",
		CaseID:                        "CASE-FIP-103",
		PetName:                       "Saffron",
		Species:                       "Feline",
		Breed:                         "Abyssinian",
		WeightKg:                      4.1,
		Status:                        domain.FIPStatusCleared84DayCured,
		Phenotype:                     domain.FIPPhenotypeOcularUveitis,
		TreatmentDay:                  84,
		AlbuminGlobulinRatio:          0.78,
		TotalBilirubinMgDL:            0.2,
		HematocritPercent:             38.0,
		DailyGS441524MgKg:             15.0,
		RivaltaTestPositive:           false,
		PleuralEffusionDyspneaPresent: false,
		BiomarkerPlan:                 bio103,
		AntiviralPlan:                 anti103,
		ProgressChecks: []domain.FIPAntiviralProgressCheck{
			{
				ID:                   "FIP-CHK-103-1",
				TreatmentDay:         56,
				WeightKg:             3.8,
				AlbuminGlobulinRatio: 0.64,
				TotalBilirubinMgDL:   0.3,
				HematocritPercent:    35.0,
				RecordedBy:           "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
				ClinicalNote:         "Day 56 (Week 8) Ocular FIP Checkpoint: Mutton-fat keratic precipitates and aqueous flare resolved on GS-441524 15 mg/kg/day; A:G Ratio 0.64.",
				RecordedAt:           now.Add(-28 * 24 * time.Hour),
			},
			{
				ID:                   "FIP-CHK-103-2",
				TreatmentDay:         84,
				WeightKg:             4.1,
				AlbuminGlobulinRatio: 0.78,
				TotalBilirubinMgDL:   0.2,
				HematocritPercent:    38.0,
				RecordedBy:           "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
				ClinicalNote:         "Day 84 (Week 12) Final Antiviral Cure Verification: A:G Ratio normalized to 0.78 (> 0.60), Total Bilirubin 0.2 mg/dL, HCT 38.0%, AGP < 250 µg/mL, weight 4.1 kg (+1.1 kg gain); cleared for 84-day FIP clinical cure.",
				RecordedAt:           now.Add(-40 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
		DischargeSummary: "Completed full 84-day (12-week) GS-441524 15 mg/kg/day ocular barrier protocol; anterior uveitis resolved, Albumin:Globulin (A:G) Ratio normalized to 0.78 (> 0.60), Total Bilirubin 0.2 mg/dL, HCT 38.0%; cleared as 84-day FIP clinical cure.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-84 * 24 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of FIP & GS-441524 operations, optionally filtered by statusFilter.
func (s *FIPService) ListOperations(statusFilter string) []domain.FIPGS441524Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.FIPGS441524Operation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneFIPOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a FIP & GS-441524 operation by ID.
func (s *FIPService) GetOperation(id string) (*domain.FIPGS441524Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("fip gs441524 operation %q not found", id)
	}
	cp := cloneFIPOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all FIP & GS-441524 operations.
func (s *FIPService) GetOverviewMetrics() domain.FIPOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.FIPOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.FIPStatusAcuteTriage:
			m.AcuteFIPTriageCount++
		case domain.FIPStatusGS441524Staged:
			m.GS441524StagedCount++
		case domain.FIPStatusCleared84DayCured:
			m.Cleared84DayCuredCount++
		}
		if op.Phenotype == domain.FIPPhenotypeNeurologicalCNS ||
			op.Phenotype == domain.FIPPhenotypeOcularUveitis ||
			op.DailyGS441524MgKg >= 15.0 {
			m.NeuroOcularHighDoseCount++
		}
		if op.AlbuminGlobulinRatio < 0.40 {
			m.AGRatioUnder04Count++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Feline Infectious Peritonitis (FIP) triage operation.
func (s *FIPService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	phenotype domain.FIPClinicalPhenotype,
	treatmentDay int,
	agRatio, totalBilirubinMgDL, hematocritPercent, dailyGS441524MgKg float64,
	rivaltaPositive, pleuralDyspneaPresent bool,
) (*domain.FIPGS441524Operation, error) {
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
	if weightKg <= 0 || weightKg > 25.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 25.0], got %v", weightKg)
	}
	if !phenotype.IsValid() {
		return nil, fmt.Errorf("invalid phenotype: %q", phenotype)
	}
	if treatmentDay < 1 || treatmentDay > 120 {
		return nil, fmt.Errorf("treatment_day must be in [1, 120], got %d", treatmentDay)
	}
	if agRatio <= 0 || agRatio > 3.0 {
		return nil, fmt.Errorf("albumin_globulin_ratio must be in (0, 3.0], got %v", agRatio)
	}
	if totalBilirubinMgDL < 0 || totalBilirubinMgDL > 30.0 {
		return nil, fmt.Errorf("total_bilirubin_mg_dl must be in [0, 30.0], got %v", totalBilirubinMgDL)
	}
	if hematocritPercent <= 0 || hematocritPercent > 70.0 {
		return nil, fmt.Errorf("hematocrit_percent must be in (0, 70.0], got %v", hematocritPercent)
	}
	if dailyGS441524MgKg <= 0 || dailyGS441524MgKg > 40.0 {
		return nil, fmt.Errorf("daily_gs441524_mg_kg must be in (0, 40.0], got %v", dailyGS441524MgKg)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("FIP-OP-%d", s.seq)
	now := time.Now().UTC()

	bioPlan := BuildRivaltaBiomarkerAssessmentPlan(
		phenotype,
		agRatio, totalBilirubinMgDL, hematocritPercent, dailyGS441524MgKg,
		rivaltaPositive, pleuralDyspneaPresent,
	)
	antiPlan := BuildDefaultNucleosideAntiviralProtocolPlan(
		weightKg, phenotype, dailyGS441524MgKg, pleuralDyspneaPresent,
	)

	op := &domain.FIPGS441524Operation{
		ID:                            opID,
		CaseID:                        trimmedCase,
		PetName:                       trimmedPet,
		Species:                       trimmedSpecies,
		Breed:                         trimmedBreed,
		WeightKg:                      round1(weightKg),
		Status:                        domain.FIPStatusAcuteTriage,
		Phenotype:                     phenotype,
		TreatmentDay:                  treatmentDay,
		AlbuminGlobulinRatio:          round2(agRatio),
		TotalBilirubinMgDL:            round1(totalBilirubinMgDL),
		HematocritPercent:             round1(hematocritPercent),
		DailyGS441524MgKg:             round1(dailyGS441524MgKg),
		RivaltaTestPositive:           rivaltaPositive,
		PleuralEffusionDyspneaPresent: pleuralDyspneaPresent,
		BiomarkerPlan:                 bioPlan,
		AntiviralPlan:                 antiPlan,
		ProgressChecks:                []domain.FIPAntiviralProgressCheck{},
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneFIPOperation(op)
	return &cp, nil
}

// StageAntiviralProtocol stages the 84-day (12-week) GS-441524 / Remdesivir nucleoside analog dosing,
// weekly weight-adjusted dose up-titration, therapeutic thoracocentesis/ascites limit, ocular/neurological
// barrier support, hepatoprotectant/Vitamin B12 support, and Day 28/56/84 monitoring schedule,
// transitioning status to GS441524_REMDESIVIR_84DAY_STAGED.
func (s *FIPService) StageAntiviralProtocol(
	id, gs441524Dosing, weightRecalcNote, centesisLimit, ocularSupportNote, hepatoprotectantPlan, monitoringPlan string,
) (*domain.FIPGS441524Operation, error) {
	trimmedDosing := strings.TrimSpace(gs441524Dosing)
	if trimmedDosing == "" {
		return nil, fmt.Errorf("gs441524_or_remdesivir_dosing is required")
	}
	trimmedRecalc := strings.TrimSpace(weightRecalcNote)
	if trimmedRecalc == "" {
		return nil, fmt.Errorf("weight_adjusted_dose_recalc_note is required")
	}
	trimmedCentesis := strings.TrimSpace(centesisLimit)
	if trimmedCentesis == "" {
		return nil, fmt.Errorf("therapeutic_thoracocentesis_limit is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("fip gs441524 operation %q not found", id)
	}

	defaults := BuildDefaultNucleosideAntiviralProtocolPlan(
		op.WeightKg, op.Phenotype, op.DailyGS441524MgKg, op.PleuralEffusionDyspneaPresent,
	)
	trimmedOcular := strings.TrimSpace(ocularSupportNote)
	if trimmedOcular == "" {
		trimmedOcular = defaults.OcularTopicalAtropinePredNote
	}
	trimmedHepato := strings.TrimSpace(hepatoprotectantPlan)
	if trimmedHepato == "" {
		trimmedHepato = defaults.HepatoprotectantB12SupportPlan
	}
	trimmedMonitoring := strings.TrimSpace(monitoringPlan)
	if trimmedMonitoring == "" {
		trimmedMonitoring = defaults.Day28Day56Day84MonitoringPlan
	}

	op.AntiviralPlan = domain.NucleosideAntiviralProtocolPlan{
		GS441524OrRemdesivirDosing:      trimmedDosing,
		WeightAdjustedDoseRecalcNote:    trimmedRecalc,
		TherapeuticThoracocentesisLimit: trimmedCentesis,
		OcularTopicalAtropinePredNote:   trimmedOcular,
		HepatoprotectantB12SupportPlan:  trimmedHepato,
		Day28Day56Day84MonitoringPlan:   trimmedMonitoring,
	}
	op.Status = domain.FIPStatusGS441524Staged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneFIPOperation(op)
	return &cp, nil
}

// RecordProgressCheck appends a FIPAntiviralProgressCheck and updates op.TreatmentDay,
// op.WeightKg, op.AlbuminGlobulinRatio, op.TotalBilirubinMgDL, and op.HematocritPercent.
func (s *FIPService) RecordProgressCheck(
	id string,
	treatmentDay int,
	weightKg, agRatio, totalBilirubinMgDL, hematocritPercent float64,
	recordedBy, clinicalNote string,
) (*domain.FIPGS441524Operation, error) {
	if treatmentDay < 1 || treatmentDay > 120 {
		return nil, fmt.Errorf("treatment_day must be in [1, 120], got %d", treatmentDay)
	}
	if weightKg <= 0 || weightKg > 25.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 25.0], got %v", weightKg)
	}
	if agRatio <= 0 || agRatio > 3.0 {
		return nil, fmt.Errorf("albumin_globulin_ratio must be in (0, 3.0], got %v", agRatio)
	}
	if totalBilirubinMgDL < 0 || totalBilirubinMgDL > 30.0 {
		return nil, fmt.Errorf("total_bilirubin_mg_dl must be in [0, 30.0], got %v", totalBilirubinMgDL)
	}
	if hematocritPercent <= 0 || hematocritPercent > 70.0 {
		return nil, fmt.Errorf("hematocrit_percent must be in (0, 70.0], got %v", hematocritPercent)
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
		return nil, fmt.Errorf("fip gs441524 operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.FIPAntiviralProgressCheck{
		ID:                   fmt.Sprintf("FIP-CHK-%s-%d", op.ID, len(op.ProgressChecks)+1),
		TreatmentDay:         treatmentDay,
		WeightKg:             round1(weightKg),
		AlbuminGlobulinRatio: round2(agRatio),
		TotalBilirubinMgDL:   round1(totalBilirubinMgDL),
		HematocritPercent:    round1(hematocritPercent),
		RecordedBy:           trimmedBy,
		ClinicalNote:         trimmedNote,
		RecordedAt:           now,
	}

	op.ProgressChecks = append(op.ProgressChecks, chk)
	op.TreatmentDay = chk.TreatmentDay
	op.WeightKg = chk.WeightKg
	op.AlbuminGlobulinRatio = chk.AlbuminGlobulinRatio
	op.TotalBilirubinMgDL = chk.TotalBilirubinMgDL
	op.HematocritPercent = chk.HematocritPercent

	op.BiomarkerPlan = BuildRivaltaBiomarkerAssessmentPlan(
		op.Phenotype,
		op.AlbuminGlobulinRatio, op.TotalBilirubinMgDL, op.HematocritPercent, op.DailyGS441524MgKg,
		op.RivaltaTestPositive, op.PleuralEffusionDyspneaPresent,
	)
	op.UpdatedAt = now

	cp := cloneFIPOperation(op)
	return &cp, nil
}

// ClearFIPDischarge transitions the operation to AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED
// once the 84-day GS-441524 / Remdesivir course is completed or A:G ratio and clinical biomarkers normalize.
func (s *FIPService) ClearFIPDischarge(id, dischargedBy, summary string) (*domain.FIPGS441524Operation, error) {
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
		return nil, fmt.Errorf("fip gs441524 operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.FIPStatusCleared84DayCured
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneFIPOperation(op)
	return &cp, nil
}

func cloneFIPOperation(op *domain.FIPGS441524Operation) domain.FIPGS441524Operation {
	cp := *op
	if op.ProgressChecks != nil {
		cp.ProgressChecks = make([]domain.FIPAntiviralProgressCheck, len(op.ProgressChecks))
		copy(cp.ProgressChecks, op.ProgressChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
