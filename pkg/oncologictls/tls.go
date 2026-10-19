package oncologictls

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TLSService manages Acute Tumor Lysis Syndrome (TLS) & Oncologic Hypercalcemia of Malignancy,
// Hyperkalemia/Hyperphosphatemia/Hypocalcemia, Rasburicase/Allopurinol & Zoledronate/Saline Diuresis
// triage operations in a thread-safe store.
type TLSService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.OncologicTLSOperation
	order []string
	seq   int
}

// NewTLSService initializes a TLSService with 3 seeded operations (TLS-OP-101, TLS-OP-102, TLS-OP-103).
func NewTLSService() *TLSService {
	s := &TLSService{
		ops:   make(map[string]*domain.OncologicTLSOperation),
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

// BuildMetabolicNephroprotectionPlan computes the clinical interpretation and contraindication guards
// for an Oncologic TLS / Hypercalcemia operation.
func BuildMetabolicNephroprotectionPlan(
	emergencyType domain.OncologicEmergencyType,
	potassiumMmolL, phosphorusMgDL, ionizedCalciumMmolL, uricAcidMgDL, creatinineMgDL, caxpProduct float64,
	definitiveCytologyConfirmed bool,
) domain.MetabolicNephroprotectionPlan {
	var patho string
	switch emergencyType {
	case domain.OncologicEmergencyTumorLysisLymphoma:
		patho = "Acute Tumor Lysis Syndrome (TLS — High-Grade Lymphoma / Acute Leukemia) — Rapid malignant lymphoblast cytolysis releasing massive intracellular Potassium (K+ > 6.5 mmol/L), Phosphorus (> 8.0 mg/dL), and Purine Nucleic Acids (Uric Acid / Xanthine), causing secondary Calcium Phosphate precipitation (Hypocalcemia iCa < 0.90 mmol/L) and acute oliguric urate/calcium-phosphate tubular nephropathy."
	case domain.OncologicEmergencyHypercalcemiaPTHrP:
		patho = "Paraneoplastic Hypercalcemia of Malignancy (PTHrP — T-Cell Mediastinal Lymphoma / Apocrine Gland Anal Sac Adenocarcinoma / Multiple Myeloma) — Parathyroid Hormone-related Protein (PTHrP) driving severe osteoclastic bone resorption and renal tubular calcium reabsorption (iCa > 1.50 mmol/L), causing ADH antagonism, nephrogenic diabetes insipidus, renal afferent vasoconstriction, and metastatic mineralization."
	case domain.OncologicEmergencyFebrileNeutropenia:
		patho = "Chemotherapy-Induced Febrile Neutropenia & Oncologic Sepsis — Post-cytotoxic nadir (< 1,000 neutrophils/µL) with mucosal barrier translocation and metabolic electrolyte instability."
	default:
		patho = "Acute Oncologic Metabolic & Renal Emergency — Combined electrolyte dysregulation and tubular nephrotoxicity risk."
	}

	caxpNote := fmt.Sprintf("Ca x P Product %.1f mg²/dL² (Phosphorus %.1f mg/dL | Creatinine %.1f mg/dL) — ", caxpProduct, phosphorusMgDL, creatinineMgDL)
	if caxpProduct >= 70.0 {
		caxpNote += "CRITICAL SOFT-TISSUE & RENAL TUBULAR MINERALIZATION THRESHOLD EXCEEDED (Ca x P >= 70): High risk of irreversible nephrocalcinosis and myocardial calcification; aggressive 0.9% NaCl diuresis and phosphate reduction mandatory."
	} else {
		caxpNote += "Below critical soft-tissue mineralization threshold (< 70 mg²/dL²); maintain renal perfusion and serial electrolyte surveillance."
	}

	alertNote := fmt.Sprintf("Serum K+ %.1f mmol/L | Ionized Calcium (iCa) %.2f mmol/L | Uric Acid %.1f mg/dL — ", potassiumMmolL, ionizedCalciumMmolL, uricAcidMgDL)
	if potassiumMmolL >= 6.5 {
		alertNote += "SEVERE HYPERKALEMIC CARDIOTOXICITY RISK (K+ >= 6.5 mmol/L) & SECONDARY HYPOCALCEMIC TETANY: Continuous Lead II ECG for peaked T-waves, prolonged PR, widened QRS, or bradyarrhythmia."
	} else if ionizedCalciumMmolL >= 1.50 {
		alertNote += "SEVERE IONIZED HYPERCALCEMIA CRISIS (iCa >= 1.50 mmol/L): High risk of progressive azotemia, QT shortening, bradyarrhythmia, and CNS obtundation."
	} else {
		alertNote += "Electrolytes stabilizing within cardioprotective range; continue ECG and ionized calcium monitoring."
	}

	cytologyStatus := "Definitive cytology/histopathology CONFIRMED"
	if !definitiveCytologyConfirmed {
		cytologyStatus = "Definitive cytology/biopsy PENDING — WITHHOLD ALL GLUCOCORTICOIDS"
	}
	steroidGuard := fmt.Sprintf("CONTRAINDICATION GUARD (%s): STRICTLY AVOID Prednisone, Prednisolone, or Dexamethasone prior to definitive lymph node FNA, flow cytometry, or bone marrow aspirate in undiagnosed hypercalcemia — premature corticosteroids induce rapid lymphocytolysis that masks lymphoma diagnosis and upregulates MDR1 (P-glycoprotein) multi-drug chemotherapy resistance.", cytologyStatus)

	bicarbNote := "URINE ALKALINIZATION GUARD: DO NOT administer Sodium Bicarbonate (NaHCO3) for urinary alkalinization in veterinary Tumor Lysis Syndrome — alkaline urine (pH > 7.0) drastically reduces Calcium Phosphate solubility, accelerating intratubular Calcium Phosphate crystal precipitation when hyperphosphatemia and Ca x P > 60–70 are present."

	urateRisk := fmt.Sprintf("Renal Tubular Nephropathy Profile (Uric Acid %.1f mg/dL, Creatinine %.1f mg/dL): Prevent intratubular uric acid, xanthine, and calcium phosphate obstruction via high-volume 0.9%% NaCl diuresis (target urine output > 2–4 mL/kg/hr) and recombinant urate oxidase (Rasburicase).", uricAcidMgDL, creatinineMgDL)

	return domain.MetabolicNephroprotectionPlan{
		EmergencyPathophysiology:            patho,
		CalciumPhosphorusProductNote:        caxpNote,
		IonizedCalciumOrPotassiumAlert:      alertNote,
		SteroidBeforeBiopsyContraindication: steroidGuard,
		SodiumBicarbonateUrineNote:          bicarbNote,
		UrateOxalateNephropathyRisk:         urateRisk,
	}
}

// BuildDefaultDiuresisBisphosphonatePlan returns an evidence-based DiuresisBisphosphonatePlan.
func BuildDefaultDiuresisBisphosphonatePlan(
	weightKg float64,
	emergencyType domain.OncologicEmergencyType,
	potassiumMmolL, ionizedCalciumMmolL float64,
	definitiveCytologyConfirmed bool,
) domain.DiuresisBisphosphonatePlan {
	salineRateMlHr := round1(weightKg * 5.0)
	zoledronateMg := round2(weightKg * 0.25)
	caGlucMl := round1(weightKg * 0.75)

	salinePlan := fmt.Sprintf("0.9%% NaCl (Calcium-Free Physiological Saline) IV Diuresis at 2–3× maintenance (%.1f mL/hr for %.1f kg patient) to restore intravascular volume, increase GFR, and promote calciuresis/kaliuresis/phosphaturia (avoid LRS/Normosol-R calcium-containing fluids in hypercalcemia).", salineRateMlHr, weightKg)

	rasburicasePlan := "Rasburicase (Recombinant Urate Oxidase) 0.15–0.2 mg/kg IV over 30 min once daily (converts uric acid to water-soluble allantoin) + Allopurinol 10 mg/kg PO q8–12h (xanthine oxidase inhibition) for acute Tumor Lysis Syndrome."
	if emergencyType == domain.OncologicEmergencyHypercalcemiaPTHrP {
		rasburicasePlan = "Monitor uric acid and xanthine upon initiation of definitive cytoreductive chemotherapy (CHOP protocol) to preempt secondary Tumor Lysis Syndrome."
	}

	steroidClause := "STRICTLY WITHHOLD Prednisone/Dexamethasone until lymph node/bone marrow cytology & PARR/flow cytometry are completed"
	if definitiveCytologyConfirmed {
		steroidClause = "Definitive cytology confirmed; Prednisone 1–2 mg/kg PO/SC q24h permissible alongside definitive oncology protocol"
	}
	zoledronatePlan := fmt.Sprintf("Zoledronate (Bisphosphonate) 0.25 mg/kg IV (%.2f mg diluted in 50–100 mL 0.9%% NaCl infused over 15 min) + Calcitonin-Salmon 4–6 IU/kg SC q8–12h for rapid iCa reduction; %s.", zoledronateMg, steroidClause)

	cardioPlan := fmt.Sprintf("Hyperkalemia Cardioprotection (K+ %.1f mmol/L): 10%% Calcium Gluconate 0.5–1.0 mL/kg IV slow over 15 min (%.1f mL — only if K+ > 6.5 mmol/L with ECG changes; caution if Ca x P > 70) + Regular Insulin 0.25–0.5 U/kg IV with Dextrose 2 g/U Insulin + Terbutaline/Albuterol.", potassiumMmolL, caGlucMl)
	if ionizedCalciumMmolL > 1.50 && potassiumMmolL < 5.5 {
		cardioPlan = fmt.Sprintf("Hypercalcemic Arrhythmia & Renal Guard (iCa %.2f mmol/L): Continuous telemetry for bradyarrhythmia/QT shortening; strictly avoid calcium-containing IV fluids.", ionizedCalciumMmolL)
	}

	binderDiuretic := "Furosemide (Loop Diuretic) 2 mg/kg IV q8–12h (or 0.66 mg/kg/hr CRI) ONLY AFTER full intravascular euvolemia is restored via 0.9% NaCl (calciuretic/kaliuretic) + Aluminum Hydroxide 30–90 mg/kg/day PO divided with meals for hyperphosphatemia."

	schedule := "Serial Ionized Calcium (iCa), Serum Potassium (K+), Phosphorus, Creatinine, BUN, Ca x P Product, Urine Specific Gravity/Output (> 2 mL/kg/hr), and Lead II ECG q4–6h."

	return domain.DiuresisBisphosphonatePlan{
		ZeroCalciumSalineDiuresisPlan:    salinePlan,
		RasburicaseAllopurinolProtocol:   rasburicasePlan,
		ZoledronateCalcitoninProtocol:    zoledronatePlan,
		HyperkalemiaCardioprotectionPlan: cardioPlan,
		PhosphateBinderOrLoopDiuretic:    binderDiuretic,
		SerialElectrolyteECGSchedule:     schedule,
	}
}

func (s *TLSService) seed() {
	now := time.Now().UTC()

	// TLS-OP-101: Bauer — Canine, Boxer, 32.0 kg, ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE
	met101 := BuildMetabolicNephroprotectionPlan(
		domain.OncologicEmergencyTumorLysisLymphoma,
		7.2, 11.8, 0.78, 9.4, 3.1, 98.4,
		true,
	)
	diur101 := BuildDefaultDiuresisBisphosphonatePlan(
		32.0, domain.OncologicEmergencyTumorLysisLymphoma,
		7.2, 0.78, true,
	)
	op101 := &domain.OncologicTLSOperation{
		ID:                          "TLS-OP-101",
		CaseID:                      "CASE-TLS-101",
		PetName:                     "Bauer",
		Species:                     "Canine",
		Breed:                       "Boxer",
		WeightKg:                    32.0,
		Status:                      domain.TLSStatusAcuteMetabolicTriage,
		EmergencyType:               domain.OncologicEmergencyTumorLysisLymphoma,
		SerumPotassiumMmolL:         7.2,
		SerumPhosphorusMgDL:         11.8,
		IonizedCalciumMmolL:         0.78,
		UricAcidMgDL:                9.4,
		SerumCreatinineMgDL:         3.1,
		CaXPProduct:                 98.4,
		DefinitiveCytologyConfirmed: true,
		MetabolicPlan:               met101,
		DiuresisPlan:                diur101,
		ElectrolyteChecks: []domain.OncologicElectrolyteCheck{
			{
				ID:                  "TLS-CHK-101-1",
				SerumPotassiumMmolL: 6.8,
				SerumPhosphorusMgDL: 10.6,
				IonizedCalciumMmolL: 0.84,
				UricAcidMgDL:        8.6,
				SerumCreatinineMgDL: 2.7,
				RecordedBy:          "Dr. Soren Lindqvist, DACVIM (Oncology)",
				ClinicalNote:        "12h post-L-asparaginase/vincristine induction for Stage Vb multicentric lymphoma: rapid lymphoblast cytolysis with rising K+ 6.8 mmol/L, Phosphorus 10.6 mg/dL, and secondary hypocalcemia iCa 0.84 mmol/L.",
				RecordedAt:          now.Add(-50 * time.Minute),
			},
			{
				ID:                  "TLS-CHK-101-2",
				SerumPotassiumMmolL: 7.2,
				SerumPhosphorusMgDL: 11.8,
				IonizedCalciumMmolL: 0.78,
				UricAcidMgDL:        9.4,
				SerumCreatinineMgDL: 3.1,
				RecordedBy:          "Dr. Soren Lindqvist, DACVIM (Oncology)",
				ClinicalNote:        "Overt Acute Tumor Lysis Syndrome: K+ 7.2 mmol/L with peaked T-waves, Phosphorus 11.8 mg/dL, iCa 0.78 mmol/L, Uric Acid 9.4 mg/dL, Ca x P 98.4; initiating 0.9% NaCl diuresis, Rasburicase 0.2 mg/kg IV, Regular Insulin/Dextrose, and avoiding NaHCO3 alkalinization.",
				RecordedAt:          now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// TLS-OP-102: Penny — Canine, English Cocker Spaniel, 13.5 kg, SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED
	met102 := BuildMetabolicNephroprotectionPlan(
		domain.OncologicEmergencyHypercalcemiaPTHrP,
		4.1, 4.2, 1.92, 1.4, 2.2, 66.8,
		false,
	)
	diur102 := BuildDefaultDiuresisBisphosphonatePlan(
		13.5, domain.OncologicEmergencyHypercalcemiaPTHrP,
		4.1, 1.92, false,
	)
	op102 := &domain.OncologicTLSOperation{
		ID:                          "TLS-OP-102",
		CaseID:                      "CASE-TLS-102",
		PetName:                     "Penny",
		Species:                     "Canine",
		Breed:                       "English Cocker Spaniel",
		WeightKg:                    13.5,
		Status:                      domain.TLSStatusDiuresisStaged,
		EmergencyType:               domain.OncologicEmergencyHypercalcemiaPTHrP,
		SerumPotassiumMmolL:         4.1,
		SerumPhosphorusMgDL:         4.2,
		IonizedCalciumMmolL:         1.92,
		UricAcidMgDL:                1.4,
		SerumCreatinineMgDL:         2.2,
		CaXPProduct:                 66.8,
		DefinitiveCytologyConfirmed: false,
		MetabolicPlan:               met102,
		DiuresisPlan:                diur102,
		ElectrolyteChecks: []domain.OncologicElectrolyteCheck{
			{
				ID:                  "TLS-CHK-102-1",
				SerumPotassiumMmolL: 4.1,
				SerumPhosphorusMgDL: 4.2,
				IonizedCalciumMmolL: 1.92,
				UricAcidMgDL:        1.4,
				SerumCreatinineMgDL: 2.2,
				RecordedBy:          "Dr. Soren Lindqvist, DACVIM (Oncology)",
				ClinicalNote:        "Severe PTHrP hypercalcemia of malignancy (iCa 1.92 mmol/L, Creatinine 2.2 mg/dL): 0.9% NaCl calcium-free diuresis + Zoledronate 0.25 mg/kg IV over 15m + Furosemide post-euvolemia staged; prednisone strictly withheld pending cranial mediastinal/anal sac cytology.",
				RecordedAt:          now.Add(-75 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-75 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// TLS-OP-103: Otis — Feline, Domestic Shorthair, 4.8 kg, ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED
	met103 := BuildMetabolicNephroprotectionPlan(
		domain.OncologicEmergencyFebrileNeutropenia,
		4.3, 4.5, 1.28, 1.1, 1.2, 44.1,
		true,
	)
	diur103 := BuildDefaultDiuresisBisphosphonatePlan(
		4.8, domain.OncologicEmergencyFebrileNeutropenia,
		4.3, 1.28, true,
	)
	dischargedAt := now.Add(-25 * time.Minute)
	op103 := &domain.OncologicTLSOperation{
		ID:                          "TLS-OP-103",
		CaseID:                      "CASE-TLS-103",
		PetName:                     "Otis",
		Species:                     "Feline",
		Breed:                       "Domestic Shorthair",
		WeightKg:                    4.8,
		Status:                      domain.TLSStatusNormoelectrolyticCleared,
		EmergencyType:               domain.OncologicEmergencyFebrileNeutropenia,
		SerumPotassiumMmolL:         4.3,
		SerumPhosphorusMgDL:         4.5,
		IonizedCalciumMmolL:         1.28,
		UricAcidMgDL:                1.1,
		SerumCreatinineMgDL:         1.2,
		CaXPProduct:                 44.1,
		DefinitiveCytologyConfirmed: true,
		MetabolicPlan:               met103,
		DiuresisPlan:                diur103,
		ElectrolyteChecks: []domain.OncologicElectrolyteCheck{
			{
				ID:                  "TLS-CHK-103-1",
				SerumPotassiumMmolL: 5.4,
				SerumPhosphorusMgDL: 6.2,
				IonizedCalciumMmolL: 1.12,
				UricAcidMgDL:        3.2,
				SerumCreatinineMgDL: 1.8,
				RecordedBy:          "Dr. Soren Lindqvist, DACVIM (Oncology)",
				ClinicalNote:        "Post-doxorubicin mild tumor lysis and febrile neutropenia stabilized with 0.9% NaCl diuresis and broad-spectrum IV antimicrobials.",
				RecordedAt:          now.Add(-14 * time.Hour),
			},
			{
				ID:                  "TLS-CHK-103-2",
				SerumPotassiumMmolL: 4.3,
				SerumPhosphorusMgDL: 4.5,
				IonizedCalciumMmolL: 1.28,
				UricAcidMgDL:        1.1,
				SerumCreatinineMgDL: 1.2,
				RecordedBy:          "Dr. Soren Lindqvist, DACVIM (Oncology)",
				ClinicalNote:        "Full normoelectrolytic and renal resolution: K+ 4.3 mmol/L, Phosphorus 4.5 mg/dL, iCa 1.28 mmol/L, Uric Acid 1.1 mg/dL, Creatinine 1.2 mg/dL, Ca x P 44.1.",
				RecordedAt:          now.Add(-40 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Soren Lindqvist, DACVIM (Oncology)",
		DischargeSummary: "Electrolytes normalized and renal tubular function protected (K+ 4.3 mmol/L, Phosphorus 4.5 mg/dL, iCa 1.28 mmol/L, Uric Acid 1.1 mg/dL, Creatinine 1.2 mg/dL, Ca x P 44.1); cleared for oncology step-down.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-18 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Oncologic TLS operations, optionally filtered by statusFilter.
func (s *TLSService) ListOperations(statusFilter string) []domain.OncologicTLSOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.OncologicTLSOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneTLSOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of an Oncologic TLS operation by ID.
func (s *TLSService) GetOperation(id string) (*domain.OncologicTLSOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("oncologic tls operation %q not found", id)
	}
	cp := cloneTLSOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all Oncologic TLS & Hypercalcemia operations.
func (s *TLSService) GetOverviewMetrics() domain.OncologicTLSOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.OncologicTLSOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.TLSStatusAcuteMetabolicTriage:
			m.AcuteMetabolicTriageCount++
		case domain.TLSStatusDiuresisStaged:
			m.DiuresisStagedCount++
		case domain.TLSStatusNormoelectrolyticCleared:
			m.ClearedNormoelectrolyticCount++
		}
		if op.EmergencyType == domain.OncologicEmergencyTumorLysisLymphoma {
			m.TumorLysisSyndromeCount++
		}
		if op.EmergencyType == domain.OncologicEmergencyHypercalcemiaPTHrP {
			m.HypercalcemiaMalignancyCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Tumor Lysis Syndrome (TLS) / Hypercalcemia triage operation.
func (s *TLSService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	emergencyType domain.OncologicEmergencyType,
	potassiumMmolL, phosphorusMgDL, ionizedCalciumMmolL, uricAcidMgDL, creatinineMgDL, caxpProduct float64,
	definitiveCytologyConfirmed bool,
) (*domain.OncologicTLSOperation, error) {
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
	if !emergencyType.IsValid() {
		return nil, fmt.Errorf("invalid emergency_type: %q", emergencyType)
	}
	if potassiumMmolL <= 0 || potassiumMmolL > 15.0 {
		return nil, fmt.Errorf("serum_potassium_mmol_l must be in (0, 15.0], got %v", potassiumMmolL)
	}
	if phosphorusMgDL <= 0 || phosphorusMgDL > 35.0 {
		return nil, fmt.Errorf("serum_phosphorus_mg_dl must be in (0, 35.0], got %v", phosphorusMgDL)
	}
	if ionizedCalciumMmolL <= 0 || ionizedCalciumMmolL > 4.0 {
		return nil, fmt.Errorf("ionized_calcium_mmol_l must be in (0, 4.0], got %v", ionizedCalciumMmolL)
	}
	if uricAcidMgDL < 0 || uricAcidMgDL > 40.0 {
		return nil, fmt.Errorf("uric_acid_mg_dl must be in [0, 40.0], got %v", uricAcidMgDL)
	}
	if creatinineMgDL <= 0 || creatinineMgDL > 25.0 {
		return nil, fmt.Errorf("serum_creatinine_mg_dl must be in (0, 25.0], got %v", creatinineMgDL)
	}
	if caxpProduct <= 0 || caxpProduct > 300.0 {
		return nil, fmt.Errorf("ca_x_p_product must be in (0, 300.0], got %v", caxpProduct)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("TLS-OP-%d", s.seq)
	now := time.Now().UTC()

	metPlan := BuildMetabolicNephroprotectionPlan(
		emergencyType,
		potassiumMmolL, phosphorusMgDL, ionizedCalciumMmolL, uricAcidMgDL, creatinineMgDL, caxpProduct,
		definitiveCytologyConfirmed,
	)
	diurPlan := BuildDefaultDiuresisBisphosphonatePlan(
		weightKg, emergencyType, potassiumMmolL, ionizedCalciumMmolL, definitiveCytologyConfirmed,
	)

	op := &domain.OncologicTLSOperation{
		ID:                          opID,
		CaseID:                      trimmedCase,
		PetName:                     trimmedPet,
		Species:                     trimmedSpecies,
		Breed:                       trimmedBreed,
		WeightKg:                    round1(weightKg),
		Status:                      domain.TLSStatusAcuteMetabolicTriage,
		EmergencyType:               emergencyType,
		SerumPotassiumMmolL:         round1(potassiumMmolL),
		SerumPhosphorusMgDL:         round1(phosphorusMgDL),
		IonizedCalciumMmolL:         round2(ionizedCalciumMmolL),
		UricAcidMgDL:                round1(uricAcidMgDL),
		SerumCreatinineMgDL:         round1(creatinineMgDL),
		CaXPProduct:                 round1(caxpProduct),
		DefinitiveCytologyConfirmed: definitiveCytologyConfirmed,
		MetabolicPlan:               metPlan,
		DiuresisPlan:                diurPlan,
		ElectrolyteChecks:           []domain.OncologicElectrolyteCheck{},
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneTLSOperation(op)
	return &cp, nil
}

// StageDiuresisTherapy stages 0.9% NaCl calcium-free diuresis, Rasburicase/Allopurinol urate reduction,
// Zoledronate/Calcitonin protocol, hyperkalemia cardioprotection, phosphate binder/loop diuretic,
// and serial electrolyte/ECG schedule, transitioning status to SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED.
func (s *TLSService) StageDiuresisTherapy(
	id, salineDiuresisPlan, rasburicaseProtocol, zoledronateProtocol, cardioprotectionPlan, binderOrDiuretic, electrolyteSchedule string,
) (*domain.OncologicTLSOperation, error) {
	trimmedSaline := strings.TrimSpace(salineDiuresisPlan)
	if trimmedSaline == "" {
		return nil, fmt.Errorf("zero_calcium_saline_diuresis_plan is required")
	}
	trimmedRasburicase := strings.TrimSpace(rasburicaseProtocol)
	if trimmedRasburicase == "" {
		return nil, fmt.Errorf("rasburicase_allopurinol_protocol is required")
	}
	trimmedZoledronate := strings.TrimSpace(zoledronateProtocol)
	if trimmedZoledronate == "" {
		return nil, fmt.Errorf("zoledronate_calcitonin_protocol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("oncologic tls operation %q not found", id)
	}

	defaults := BuildDefaultDiuresisBisphosphonatePlan(
		op.WeightKg, op.EmergencyType, op.SerumPotassiumMmolL, op.IonizedCalciumMmolL, op.DefinitiveCytologyConfirmed,
	)
	trimmedCardio := strings.TrimSpace(cardioprotectionPlan)
	if trimmedCardio == "" {
		trimmedCardio = defaults.HyperkalemiaCardioprotectionPlan
	}
	trimmedBinder := strings.TrimSpace(binderOrDiuretic)
	if trimmedBinder == "" {
		trimmedBinder = defaults.PhosphateBinderOrLoopDiuretic
	}
	trimmedSchedule := strings.TrimSpace(electrolyteSchedule)
	if trimmedSchedule == "" {
		trimmedSchedule = defaults.SerialElectrolyteECGSchedule
	}

	op.DiuresisPlan = domain.DiuresisBisphosphonatePlan{
		ZeroCalciumSalineDiuresisPlan:    trimmedSaline,
		RasburicaseAllopurinolProtocol:   trimmedRasburicase,
		ZoledronateCalcitoninProtocol:    trimmedZoledronate,
		HyperkalemiaCardioprotectionPlan: trimmedCardio,
		PhosphateBinderOrLoopDiuretic:    trimmedBinder,
		SerialElectrolyteECGSchedule:     trimmedSchedule,
	}
	op.Status = domain.TLSStatusDiuresisStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneTLSOperation(op)
	return &cp, nil
}

// RecordElectrolyteCheck appends an OncologicElectrolyteCheck and updates op.SerumPotassiumMmolL,
// op.SerumPhosphorusMgDL, op.IonizedCalciumMmolL, op.UricAcidMgDL, and op.SerumCreatinineMgDL.
func (s *TLSService) RecordElectrolyteCheck(
	id string,
	potassiumMmolL, phosphorusMgDL, ionizedCalciumMmolL, uricAcidMgDL, creatinineMgDL float64,
	recordedBy, clinicalNote string,
) (*domain.OncologicTLSOperation, error) {
	if potassiumMmolL <= 0 || potassiumMmolL > 15.0 {
		return nil, fmt.Errorf("serum_potassium_mmol_l must be in (0, 15.0], got %v", potassiumMmolL)
	}
	if phosphorusMgDL <= 0 || phosphorusMgDL > 35.0 {
		return nil, fmt.Errorf("serum_phosphorus_mg_dl must be in (0, 35.0], got %v", phosphorusMgDL)
	}
	if ionizedCalciumMmolL <= 0 || ionizedCalciumMmolL > 4.0 {
		return nil, fmt.Errorf("ionized_calcium_mmol_l must be in (0, 4.0], got %v", ionizedCalciumMmolL)
	}
	if uricAcidMgDL < 0 || uricAcidMgDL > 40.0 {
		return nil, fmt.Errorf("uric_acid_mg_dl must be in [0, 40.0], got %v", uricAcidMgDL)
	}
	if creatinineMgDL <= 0 || creatinineMgDL > 25.0 {
		return nil, fmt.Errorf("serum_creatinine_mg_dl must be in (0, 25.0], got %v", creatinineMgDL)
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
		return nil, fmt.Errorf("oncologic tls operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.OncologicElectrolyteCheck{
		ID:                  fmt.Sprintf("TLS-CHK-%s-%d", op.ID, len(op.ElectrolyteChecks)+1),
		SerumPotassiumMmolL: round1(potassiumMmolL),
		SerumPhosphorusMgDL: round1(phosphorusMgDL),
		IonizedCalciumMmolL: round2(ionizedCalciumMmolL),
		UricAcidMgDL:        round1(uricAcidMgDL),
		SerumCreatinineMgDL: round1(creatinineMgDL),
		RecordedBy:          trimmedBy,
		ClinicalNote:        trimmedNote,
		RecordedAt:          now,
	}

	op.ElectrolyteChecks = append(op.ElectrolyteChecks, chk)
	op.SerumPotassiumMmolL = chk.SerumPotassiumMmolL
	op.SerumPhosphorusMgDL = chk.SerumPhosphorusMgDL
	op.IonizedCalciumMmolL = chk.IonizedCalciumMmolL
	op.UricAcidMgDL = chk.UricAcidMgDL
	op.SerumCreatinineMgDL = chk.SerumCreatinineMgDL

	op.MetabolicPlan = BuildMetabolicNephroprotectionPlan(
		op.EmergencyType,
		op.SerumPotassiumMmolL, op.SerumPhosphorusMgDL, op.IonizedCalciumMmolL, op.UricAcidMgDL, op.SerumCreatinineMgDL, op.CaXPProduct,
		op.DefinitiveCytologyConfirmed,
	)
	op.UpdatedAt = now

	cp := cloneTLSOperation(op)
	return &cp, nil
}

// ClearTLSDischarge transitions the operation to ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED
// once hyperkalemia, hyperphosphatemia, ionized calcium, and renal perfusion are stabilized.
func (s *TLSService) ClearTLSDischarge(id, dischargedBy, summary string) (*domain.OncologicTLSOperation, error) {
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
		return nil, fmt.Errorf("oncologic tls operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.TLSStatusNormoelectrolyticCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneTLSOperation(op)
	return &cp, nil
}

func cloneTLSOperation(op *domain.OncologicTLSOperation) domain.OncologicTLSOperation {
	cp := *op
	if op.ElectrolyteChecks != nil {
		cp.ElectrolyteChecks = make([]domain.OncologicElectrolyteCheck, len(op.ElectrolyteChecks))
		copy(cp.ElectrolyteChecks, op.ElectrolyteChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
