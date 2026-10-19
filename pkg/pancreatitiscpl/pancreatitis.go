package pancreatitiscpl

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// PancreatitisService manages Acute Pancreatitis, Canine/Feline Spec cPL/fPL,
// POCUS Hypoechoic Pancreas/Hyperechoic Mesentery, Maropitant/Fuzapladib & Early Enteral Nutrition
// triage operations in a thread-safe in-memory store.
type PancreatitisService struct {
	mu       sync.RWMutex
	ops      map[string]*domain.PancreatitisOperation
	order    []string
	seq      int
	checkSeq int
}

// NewPancreatitisService initializes a PancreatitisService with 3 seeded operations (PANC-OP-101, PANC-OP-102, PANC-OP-103).
func NewPancreatitisService() *PancreatitisService {
	s := &PancreatitisService{
		ops:      make(map[string]*domain.PancreatitisOperation),
		order:    make([]string, 0, 8),
		seq:      103,
		checkSeq: 105,
	}
	s.seed()
	return s
}

func calculateRER(weightKg float64) int {
	if weightKg <= 0 {
		return 0
	}
	return int(math.Round(70.0 * math.Pow(weightKg, 0.75)))
}

// BuildDefaultDiagnosticPlan constructs a clinical DiagnosticPOCUSPlan from patient parameters.
func BuildDefaultDiagnosticPlan(species string, specPLUgL int, pocusHypoechoic, hyperechoicFat, prayerPosition, triaditisSuspected bool) domain.DiagnosticPOCUSPlan {
	isFeline := strings.Contains(strings.ToUpper(species), "FELINE") || strings.Contains(strings.ToUpper(species), "CAT")

	var assayResult string
	if isFeline {
		if specPLUgL >= 5 {
			assayResult = fmt.Sprintf("Spec fPL %d ug/L (> 5.4 ug/L consistent with acute feline pancreatitis; grey zone 3.6-5.3 ug/L)", specPLUgL)
		} else {
			assayResult = fmt.Sprintf("Spec fPL %d ug/L (<= 3.5 ug/L normal baseline range)", specPLUgL)
		}
	} else {
		if specPLUgL > 400 {
			assayResult = fmt.Sprintf("Spec cPL %d ug/L (> 400 ug/L diagnostic for acute canine pancreatitis; grey zone 200-400 ug/L)", specPLUgL)
		} else if specPLUgL >= 200 {
			assayResult = fmt.Sprintf("Spec cPL %d ug/L (200-400 ug/L equivocal grey zone; correlate with abdominal POCUS)", specPLUgL)
		} else {
			assayResult = fmt.Sprintf("Spec cPL %d ug/L (< 200 ug/L recovering / normal canine pancreatic lipase range)", specPLUgL)
		}
	}

	var pocusSign string
	switch {
	case pocusHypoechoic && hyperechoicFat:
		pocusSign = "Hypoechoic enlarged pancreas (edema/necrosis) surrounded by brightly hyperechoic peripancreatic mesentery (saponification of peripancreatic fat) + focal free peripancreatic fluid"
	case pocusHypoechoic:
		pocusSign = "Hypoechoic pancreatic parenchyma with mild peripancreatic reactivity"
	default:
		pocusSign = "Resolving pancreatic echogenicity; peripancreatic mesentery normoechoic without free fluid"
	}

	corrugation := "Mild reactive duodenitis; normal peristalsis"
	if pocusHypoechoic && hyperechoicFat {
		corrugation = "Spastic duodenal corrugation ('wavy' mucosal/submucosal wall thickening & focal paralytic ileus adjacent to right pancreatic limb)"
	}
	if prayerPosition {
		corrugation += " + cranial abdominal 'prayer position'splinting"
	}

	ehbdo := "Common bile duct (CBD) patent (< 3 mm canine / < 4 mm feline); monitor total bilirubin q24h for extrahepatic biliary duct obstruction (EHBDO)"
	if pocusHypoechoic && hyperechoicFat {
		ehbdo = "Swollen pancreatic body adjacent to major duodenal papilla; serial ultrasound & bilirubin monitoring active for reversible EHBDO"
	}

	triaditisNote := "Canine presentation: screen for hypertriglyceridemia (Miniature Schnauzer), dietary indiscretion, or endocrine comorbidity (Cushing's / Diabetes / Hypothyroidism)"
	if isFeline || triaditisSuspected {
		triaditisNote = "Feline Triaditis (concurrent acute pancreatitis + lymphocytic/neutrophilic cholangiohepatitis + inflammatory bowel disease): supplement Cyanocobalamin (Vitamin B12) 250 mcg SC weekly (distal ileal cobalamin malabsorption due to intrinsic factor deficiency) + S-adenosylmethionine/Ursodiol support"
	}

	npoWarning := "CONTRAINDICATED MYTH WARNING: Prolonged NPO ('resting the pancreas' / gut starvation) is STRICTLY CONTRAINDICATED — starvation > 24-48h causes rapid enterocyte villous atrophy, tight-junction breakdown, and fatal enteric bacterial translocation (E. coli / Clostridium) into necrotic pancreatic tissue (infected pancreatic necrosis / septic peritonitis)."

	return domain.DiagnosticPOCUSPlan{
		SpecPLAssayResult:         assayResult,
		POCUSSonographicSign:      pocusSign,
		DuodenalCorrugationStatus: corrugation,
		BiliaryObstructionEHBDO:   ehbdo,
		TriaditisCobalaminNote:    triaditisNote,
		OutdatedNPOMythWarning:    npoWarning,
	}
}

// BuildDefaultNutritionPlan constructs a clinical AntiemeticNutritionPlan from patient weight and species.
func BuildDefaultNutritionPlan(species string, weightKg float64, triaditisSuspected bool) domain.AntiemeticNutritionPlan {
	isFeline := strings.Contains(strings.ToUpper(species), "FELINE") || strings.Contains(strings.ToUpper(species), "CAT")
	rerKcal := calculateRER(weightKg)

	fuzapladibDoseMg := math.Round(0.4*weightKg*100) / 100
	fuzapladibProtocol := fmt.Sprintf("Fuzapladib sodium (Panoquell-CA1) 0.4 mg/kg IV q24h x 3 days (%.2f mg IV slow bolus over 15s-1min; LFA-1 activation inhibitor blocking neutrophil extravasation into pancreatic parenchyma)", fuzapladibDoseMg)
	if isFeline || triaditisSuspected {
		fuzapladibProtocol = "Feline Triaditis Leukocyte & Hepatobiliary Protocol: Cyanocobalamin (Vitamin B12) 250 mcg SC weekly + SAMe/Silybin; note Fuzapladib sodium is FDA-approved for canine acute pancreatitis (0.4 mg/kg IV q24h x 3d)"
	}

	maropitantMg := math.Round(1.0*weightKg*10) / 10
	ondansetronMg := math.Round(0.5*weightKg*10) / 10
	antiemeticProtocol := fmt.Sprintf("Dual Antiemetic Blockade: Maropitant (Cerenia) 1.0 mg/kg IV q24h (%.1f mg IV; NK-1 receptor Substance P antagonist with visceral analgesic synergy) + Ondansetron 0.5 mg/kg IV q8h (%.1f mg IV; central/vagal 5-HT3 antagonist)", maropitantMg, ondansetronMg)

	analgesiaCRI := "Multimodal Visceral Analgesia: Fentanyl CRI 3-5 mcg/kg/hr IV (or Buprenorphine 0.02 mg/kg IV q6h in cats) + Ketamine sub-anesthetic CRI 2-5 mcg/kg/min (NMDA central sensitization blockade) + Lidocaine CRI 25-50 mcg/kg/min (canine prokinetic/anti-inflammatory). STRICT ZERO NSAIDs (meloxicam/carprofen cause acute renal papillary necrosis & GI perforation)."

	tubePlan := "Early Enteral Nutrition via 5-8 Fr Nasoesophageal (NE) or 14 Fr Esophagostomy (E-tube) feeding tube within 12-24h of antiemetic control (ultra-low-fat polymeric liquid diet < 20% fat DM for dogs; highly digestible recovery diet + Cobalamin B12 for feline triaditis to prevent hepatic lipidosis & bacterial translocation)"

	rerSchedule := fmt.Sprintf("RER = %d kcal/day (70 x %.1f kg^0.75): Day 1 trickle-feed 25%% RER (%d kcal/day = %.1f kcal/hr CRI), Day 2 ramp to 50%% RER (%d kcal/day), Day 3 ramp to 75%% RER (%d kcal/day), Day 4 target 100%% RER (%d kcal/day)",
		rerKcal, weightKg, rerKcal/4, float64(rerKcal/4)/24.0, rerKcal/2, (rerKcal*3)/4, rerKcal)

	fluidPlan := "Balanced Isotonic Crystalloid (Lactated Ringer's / Plasma-Lyte A) maintenance + dehydration deficit + ongoing emesis/third-space losses + KCl 20-30 mEq/L supplementation to restore pancreatic microcirculation and prevent ischemic acinar necrosis"

	return domain.AntiemeticNutritionPlan{
		FuzapladibProtocol:     fuzapladibProtocol,
		AntiemeticDualProtocol: antiemeticProtocol,
		VisceralAnalgesiaCRI:   analgesiaCRI,
		EarlyEnteralTubePlan:   tubePlan,
		RERCaloricRampSchedule: rerSchedule,
		FluidCrystalloidPlan:   fluidPlan,
	}
}

func (s *PancreatitisService) seed() {
	baseTime := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

	// 1. PANC-OP-101: "Schnitzel" (Canine — Miniature Schnauzer, 8.2 kg)
	op101 := &domain.PancreatitisOperation{
		ID:                           "PANC-OP-101",
		CaseID:                       "CASE-PANC-101",
		PetName:                      "Schnitzel",
		Species:                      "Canine",
		Breed:                        "Miniature Schnauzer",
		WeightKg:                     8.2,
		Status:                       domain.PancreatitisStatusAcuteNecrotizingTriage,
		SeverityClass:                domain.PancreatitisSeveritySevereNecrotizingSIRSMODS,
		SpecPLUgL:                    940,
		ModifiedGlasgowPainScore:     14,
		EnteralRERPercent:            0,
		POCUSHypoechoicPancreas:      true,
		HyperechoicPeripancreaticFat: true,
		PrayerPositionPresent:        true,
		FelineTriaditisSuspected:     false,
		DiagnosticPlan:               BuildDefaultDiagnosticPlan("Canine", 940, true, true, true, false),
		NutritionPlan:                BuildDefaultNutritionPlan("Canine", 8.2, false),
		EnteralChecks: []domain.EnteralPainCheck{
			{
				ID:                       "CHK-PANC-101",
				SpecPLUgL:                940,
				ModifiedGlasgowPainScore: 14,
				EnteralRERPercent:        0,
				VomitingRegurgStatus:     "ACTIVE_BILIOUS_EMESIS_Q2H",
				RecordedBy:               "Dr. Elena Rostova, DACVECC",
				ClinicalNote:             "Miniature Schnauzer with severe hypertriglyceridemia & acute necrotizing pancreatitis; Spec cPL 940 ug/L (> 400 ug/L diagnostic); POCUS shows hypoechoic swollen pancreas, brightly hyperechoic peripancreatic fat saponification, and duodenal corrugation; prayer position present (CMPS-SF 14/24). Prolonged NPO contraindicated; staging Fuzapladib 0.4 mg/kg IV + Maropitant/Ondansetron + NE tube.",
				RecordedAt:               baseTime.Add(-2 * time.Hour),
			},
			{
				ID:                       "CHK-PANC-102",
				SpecPLUgL:                910,
				ModifiedGlasgowPainScore: 11,
				EnteralRERPercent:        15,
				VomitingRegurgStatus:     "EMESIS_CONTROLLED_POST_MAROPITANT",
				RecordedBy:               "Dr. Elena Rostova, DACVECC",
				ClinicalNote:             "Fentanyl + Ketamine + Lidocaine CRI titrated; Maropitant 1 mg/kg IV + Ondansetron 0.5 mg/kg IV administered; 8 Fr nasoesophageal tube verified at 9th rib on lateral thoracic radiograph.",
				RecordedAt:               baseTime.Add(-1 * time.Hour),
			},
		},
		CreatedAt: baseTime.Add(-3 * time.Hour),
		UpdatedAt: baseTime.Add(-1 * time.Hour),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// 2. PANC-OP-102: "Miso" (Feline — Domestic Shorthair, 4.3 kg)
	op102 := &domain.PancreatitisOperation{
		ID:                           "PANC-OP-102",
		CaseID:                       "CASE-PANC-102",
		PetName:                      "Miso",
		Species:                      "Feline",
		Breed:                        "Domestic Shorthair",
		WeightKg:                     4.3,
		Status:                       domain.PancreatitisStatusFuzapladibEnteralStaged,
		SeverityClass:                domain.PancreatitisSeverityFelineTriaditis,
		SpecPLUgL:                    28,
		ModifiedGlasgowPainScore:     6,
		EnteralRERPercent:            50,
		POCUSHypoechoicPancreas:      true,
		HyperechoicPeripancreaticFat: true,
		PrayerPositionPresent:        false,
		FelineTriaditisSuspected:     true,
		DiagnosticPlan:               BuildDefaultDiagnosticPlan("Feline", 28, true, true, false, true),
		NutritionPlan:                BuildDefaultNutritionPlan("Feline", 4.3, true),
		EnteralChecks: []domain.EnteralPainCheck{
			{
				ID:                       "CHK-PANC-103",
				SpecPLUgL:                28,
				ModifiedGlasgowPainScore: 6,
				EnteralRERPercent:        50,
				VomitingRegurgStatus:     "NO_EMESIS_TOLERATING_ETUBE_50PCT_RER",
				RecordedBy:               "Dr. Priya Nair, DACVIM",
				ClinicalNote:             "Feline Triaditis (acute pancreatitis Spec fPL 28 ug/L > 5.4 ug/L + cholangiohepatitis + IBD); Cyanocobalamin (Vitamin B12) 250 mcg SC administered; tolerating 50% RER via 14 Fr esophagostomy tube on Buprenorphine + Maropitant/Ondansetron.",
				RecordedAt:               baseTime.Add(-90 * time.Minute),
			},
		},
		CreatedAt: baseTime.Add(-6 * time.Hour),
		UpdatedAt: baseTime.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// 3. PANC-OP-103: "Biscuit" (Canine — Yorkshire Terrier, 3.6 kg)
	dischargedTime := baseTime.Add(-30 * time.Minute)
	op103 := &domain.PancreatitisOperation{
		ID:                           "PANC-OP-103",
		CaseID:                       "CASE-PANC-103",
		PetName:                      "Biscuit",
		Species:                      "Canine",
		Breed:                        "Yorkshire Terrier",
		WeightKg:                     3.6,
		Status:                       domain.PancreatitisStatusToleratingEnteralCleared,
		SeverityClass:                domain.PancreatitisSeverityModerateAcute,
		SpecPLUgL:                    190,
		ModifiedGlasgowPainScore:     2,
		EnteralRERPercent:            100,
		POCUSHypoechoicPancreas:      false,
		HyperechoicPeripancreaticFat: false,
		PrayerPositionPresent:        false,
		FelineTriaditisSuspected:     false,
		DiagnosticPlan:               BuildDefaultDiagnosticPlan("Canine", 190, false, false, false, false),
		NutritionPlan:                BuildDefaultNutritionPlan("Canine", 3.6, false),
		EnteralChecks: []domain.EnteralPainCheck{
			{
				ID:                       "CHK-PANC-104",
				SpecPLUgL:                340,
				ModifiedGlasgowPainScore: 5,
				EnteralRERPercent:        75,
				VomitingRegurgStatus:     "ZERO_EMESIS_24H",
				RecordedBy:               "Dr. Marcus Vance, DACVECC",
				ClinicalNote:             "Completed 3-day Fuzapladib sodium (0.4 mg/kg IV q24h) course; tolerated 75% RER low-fat enteral feeding without nausea or cranial abdominal pain.",
				RecordedAt:               baseTime.Add(-12 * time.Hour),
			},
			{
				ID:                       "CHK-PANC-105",
				SpecPLUgL:                190,
				ModifiedGlasgowPainScore: 2,
				EnteralRERPercent:        100,
				VomitingRegurgStatus:     "VOLUNTARY_LOW_FAT_INTAKE_100PCT_RER",
				RecordedBy:               "Dr. Marcus Vance, DACVECC",
				ClinicalNote:             "Spec cPL decreased to 190 ug/L (< 200 ug/L normal); CMPS-SF pain score 2/24; consuming 100% RER ultra-low-fat prescription diet voluntarily; cleared for discharge.",
				RecordedAt:               dischargedTime,
			},
		},
		DischargedBy:     "Dr. Marcus Vance, DACVECC",
		DischargeSummary: "Completed 3-day Fuzapladib sodium (Panoquell-CA1) & Maropitant/Ondansetron protocol; Spec cPL normalized to 190 ug/L; pain-free (CMPS-SF 2/24) and voluntarily consuming 100% RER ultra-low-fat (< 20% fat DM) prescription diet.",
		DischargedAt:     &dischargedTime,
		CreatedAt:        baseTime.Add(-48 * time.Hour),
		UpdatedAt:        dischargedTime,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of pancreatitis operations, optionally filtered by statusFilter.
func (s *PancreatitisService) ListOperations(statusFilter string) []domain.PancreatitisOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.PancreatitisOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, clonePancreatitisOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a pancreatitis operation by ID.
func (s *PancreatitisService) GetOperation(id string) (*domain.PancreatitisOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("pancreatitis operation %q not found", id)
	}
	cp := clonePancreatitisOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes aggregate KPI counts across all pancreatitis operations.
func (s *PancreatitisService) GetOverviewMetrics() domain.PancreatitisOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.PancreatitisOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.PancreatitisStatusAcuteNecrotizingTriage:
			m.AcutePancreatitisTriageCount++
		case domain.PancreatitisStatusFuzapladibEnteralStaged:
			m.EnteralNutritionStagedCount++
		case domain.PancreatitisStatusToleratingEnteralCleared:
			m.ClearedEnteralCount++
		}
		isFeline := strings.Contains(strings.ToUpper(op.Species), "FELINE") || strings.Contains(strings.ToUpper(op.Species), "CAT")
		if (isFeline && op.SpecPLUgL >= 5) || (!isFeline && op.SpecPLUgL > 400) {
			m.SevereSpecPLElevatedCount++
		}
		if op.FelineTriaditisSuspected || op.SeverityClass == domain.PancreatitisSeverityFelineTriaditis {
			m.FelineTriaditisCount++
		}
	}
	return m
}

// ComputeMetrics is an alias for GetOverviewMetrics for consistency across services.
func (s *PancreatitisService) ComputeMetrics() domain.PancreatitisOverviewMetrics {
	return s.GetOverviewMetrics()
}

// CreateOperation validates and creates a new Acute Pancreatitis & Early Enteral Nutrition triage operation.
func (s *PancreatitisService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	severityClass domain.PancreatitisSeverityClass,
	specPLUgL, painScore, enteralRERPercent int,
	pocusHypoechoic, hyperechoicFat, prayerPosition, triaditisSuspected bool,
) (*domain.PancreatitisOperation, error) {
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	trimmedSpecies := strings.TrimSpace(species)
	if trimmedSpecies == "" {
		trimmedSpecies = "Canine"
	}
	if weightKg <= 0 || weightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 120.0], got %v", weightKg)
	}
	if severityClass == "" {
		if triaditisSuspected || strings.Contains(strings.ToUpper(trimmedSpecies), "FELINE") {
			severityClass = domain.PancreatitisSeverityFelineTriaditis
		} else if specPLUgL > 400 || (pocusHypoechoic && hyperechoicFat) {
			severityClass = domain.PancreatitisSeveritySevereNecrotizingSIRSMODS
		} else {
			severityClass = domain.PancreatitisSeverityModerateAcute
		}
	}
	if !severityClass.IsValid() {
		return nil, fmt.Errorf("invalid severity_class: %q", severityClass)
	}
	if specPLUgL < 0 || specPLUgL > 5000 {
		return nil, fmt.Errorf("spec_pl_ug_l must be in [0, 5000], got %d", specPLUgL)
	}
	if painScore < 0 || painScore > 24 {
		return nil, fmt.Errorf("modified_glasgow_pain_score must be in [0, 24], got %d", painScore)
	}
	if enteralRERPercent < 0 || enteralRERPercent > 150 {
		return nil, fmt.Errorf("enteral_rer_percent must be in [0, 150], got %d", enteralRERPercent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("PANC-OP-%d", s.seq)
	trimmedCase := strings.TrimSpace(caseID)
	if trimmedCase == "" {
		trimmedCase = fmt.Sprintf("CASE-PANC-%d", s.seq)
	}
	trimmedBreed := strings.TrimSpace(breed)
	if trimmedBreed == "" {
		trimmedBreed = "Mixed Breed"
	}

	now := time.Now().UTC()
	op := &domain.PancreatitisOperation{
		ID:                           opID,
		CaseID:                       trimmedCase,
		PetName:                      trimmedPet,
		Species:                      trimmedSpecies,
		Breed:                        trimmedBreed,
		WeightKg:                     weightKg,
		Status:                       domain.PancreatitisStatusAcuteNecrotizingTriage,
		SeverityClass:                severityClass,
		SpecPLUgL:                    specPLUgL,
		ModifiedGlasgowPainScore:     painScore,
		EnteralRERPercent:            enteralRERPercent,
		POCUSHypoechoicPancreas:      pocusHypoechoic,
		HyperechoicPeripancreaticFat: hyperechoicFat,
		PrayerPositionPresent:        prayerPosition,
		FelineTriaditisSuspected:     triaditisSuspected,
		DiagnosticPlan:               BuildDefaultDiagnosticPlan(trimmedSpecies, specPLUgL, pocusHypoechoic, hyperechoicFat, prayerPosition, triaditisSuspected),
		NutritionPlan:                BuildDefaultNutritionPlan(trimmedSpecies, weightKg, triaditisSuspected),
		EnteralChecks:                []domain.EnteralPainCheck{},
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := clonePancreatitisOperation(op)
	return &cp, nil
}

// StageEnteralNutrition stages Fuzapladib sodium, dual antiemetic (Maropitant + Ondansetron),
// visceral analgesia CRI, and early enteral feeding (NE / E-tube) protocol, transitioning the
// operation to FUZAPLADIB_MAROPITANT_ENTERAL_STAGED.
func (s *PancreatitisService) StageEnteralNutrition(
	id, fuzapladibProtocol, antiemeticProtocol, analgesiaCRI, enteralTubePlan, rerSchedule, fluidPlan string,
) (*domain.PancreatitisOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmedID := strings.TrimSpace(id)
	op, ok := s.ops[trimmedID]
	if !ok {
		return nil, fmt.Errorf("pancreatitis operation %q not found", id)
	}

	defaults := BuildDefaultNutritionPlan(op.Species, op.WeightKg, op.FelineTriaditisSuspected)

	if strings.TrimSpace(fuzapladibProtocol) == "" {
		fuzapladibProtocol = defaults.FuzapladibProtocol
	}
	if strings.TrimSpace(antiemeticProtocol) == "" {
		antiemeticProtocol = defaults.AntiemeticDualProtocol
	}
	if strings.TrimSpace(analgesiaCRI) == "" {
		analgesiaCRI = defaults.VisceralAnalgesiaCRI
	}
	if strings.TrimSpace(enteralTubePlan) == "" {
		enteralTubePlan = defaults.EarlyEnteralTubePlan
	}
	if strings.TrimSpace(rerSchedule) == "" {
		rerSchedule = defaults.RERCaloricRampSchedule
	}
	if strings.TrimSpace(fluidPlan) == "" {
		fluidPlan = defaults.FluidCrystalloidPlan
	}

	op.NutritionPlan = domain.AntiemeticNutritionPlan{
		FuzapladibProtocol:     strings.TrimSpace(fuzapladibProtocol),
		AntiemeticDualProtocol: strings.TrimSpace(antiemeticProtocol),
		VisceralAnalgesiaCRI:   strings.TrimSpace(analgesiaCRI),
		EarlyEnteralTubePlan:   strings.TrimSpace(enteralTubePlan),
		RERCaloricRampSchedule: strings.TrimSpace(rerSchedule),
		FluidCrystalloidPlan:   strings.TrimSpace(fluidPlan),
	}
	if op.EnteralRERPercent == 0 {
		op.EnteralRERPercent = 25
	}
	op.Status = domain.PancreatitisStatusFuzapladibEnteralStaged
	op.DischargedAt = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.UpdatedAt = time.Now().UTC()

	cp := clonePancreatitisOperation(op)
	return &cp, nil
}

// RecordEnteralCheck appends a serial Spec cPL/fPL, Modified Glasgow pain score, and enteral RER % check,
// updating op.SpecPLUgL, op.ModifiedGlasgowPainScore, and op.EnteralRERPercent.
func (s *PancreatitisService) RecordEnteralCheck(
	id string,
	specPLUgL, painScore, enteralRERPercent int,
	vomitingStatus, recordedBy, clinicalNote string,
) (*domain.PancreatitisOperation, error) {
	if specPLUgL < 0 || specPLUgL > 5000 {
		return nil, fmt.Errorf("spec_pl_ug_l must be in [0, 5000], got %d", specPLUgL)
	}
	if painScore < 0 || painScore > 24 {
		return nil, fmt.Errorf("modified_glasgow_pain_score must be in [0, 24], got %d", painScore)
	}
	if enteralRERPercent < 0 || enteralRERPercent > 150 {
		return nil, fmt.Errorf("enteral_rer_percent must be in [0, 150], got %d", enteralRERPercent)
	}
	trimmedRecordedBy := strings.TrimSpace(recordedBy)
	if trimmedRecordedBy == "" {
		return nil, fmt.Errorf("recorded_by is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	trimmedID := strings.TrimSpace(id)
	op, ok := s.ops[trimmedID]
	if !ok {
		return nil, fmt.Errorf("pancreatitis operation %q not found", id)
	}

	s.checkSeq++
	now := time.Now().UTC()

	trimmedVomiting := strings.TrimSpace(vomitingStatus)
	if trimmedVomiting == "" {
		trimmedVomiting = "ZERO_EMESIS_TOLERATING_ENTERAL"
	}
	trimmedNote := strings.TrimSpace(clinicalNote)
	if trimmedNote == "" {
		trimmedNote = fmt.Sprintf("Serial check: Spec PL %d ug/L, CMPS-SF pain score %d/24, tolerating %d%% RER (%s).", specPLUgL, painScore, enteralRERPercent, trimmedVomiting)
	}

	chk := domain.EnteralPainCheck{
		ID:                       fmt.Sprintf("CHK-PANC-%d", s.checkSeq),
		SpecPLUgL:                specPLUgL,
		ModifiedGlasgowPainScore: painScore,
		EnteralRERPercent:        enteralRERPercent,
		VomitingRegurgStatus:     trimmedVomiting,
		RecordedBy:               trimmedRecordedBy,
		ClinicalNote:             trimmedNote,
		RecordedAt:               now,
	}

	op.EnteralChecks = append(op.EnteralChecks, chk)
	op.SpecPLUgL = specPLUgL
	op.ModifiedGlasgowPainScore = painScore
	op.EnteralRERPercent = enteralRERPercent
	if painScore <= 4 {
		op.PrayerPositionPresent = false
	}
	op.DiagnosticPlan = BuildDefaultDiagnosticPlan(
		op.Species,
		op.SpecPLUgL,
		op.POCUSHypoechoicPancreas,
		op.HyperechoicPeripancreaticFat,
		op.PrayerPositionPresent,
		op.FelineTriaditisSuspected,
	)
	op.UpdatedAt = now

	cp := clonePancreatitisOperation(op)
	return &cp, nil
}

// ClearPancreatitisDischarge certifies enteral nutrition tolerance, visceral pain resolution,
// and transitions the operation to TOLERATING_ENTERAL_PAIN_FREE_CLEARED.
func (s *PancreatitisService) ClearPancreatitisDischarge(
	id, dischargedBy, summary string,
) (*domain.PancreatitisOperation, error) {
	trimmedBy := strings.TrimSpace(dischargedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("discharged_by is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	trimmedID := strings.TrimSpace(id)
	op, ok := s.ops[trimmedID]
	if !ok {
		return nil, fmt.Errorf("pancreatitis operation %q not found", id)
	}

	now := time.Now().UTC()
	trimmedSummary := strings.TrimSpace(summary)
	if trimmedSummary == "" {
		trimmedSummary = fmt.Sprintf("Tolerating %d%% RER enteral nutrition without vomiting/regurgitation; visceral pain resolved (CMPS-SF %d/24); Spec PL %d ug/L; cleared for discharge on ultra-low-fat diet.", op.EnteralRERPercent, op.ModifiedGlasgowPainScore, op.SpecPLUgL)
	}

	op.Status = domain.PancreatitisStatusToleratingEnteralCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.PrayerPositionPresent = false
	if op.EnteralRERPercent < 75 {
		op.EnteralRERPercent = 100
	}
	op.UpdatedAt = now

	cp := clonePancreatitisOperation(op)
	return &cp, nil
}

func clonePancreatitisOperation(op *domain.PancreatitisOperation) domain.PancreatitisOperation {
	cp := *op
	if op.EnteralChecks != nil {
		cp.EnteralChecks = make([]domain.EnteralPainCheck, len(op.EnteralChecks))
		copy(cp.EnteralChecks, op.EnteralChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
