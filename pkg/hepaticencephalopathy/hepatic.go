package hepaticencephalopathy

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// HepaticService manages Acute Hepatic Failure, Hepatic Encephalopathy (HE),
// Sago Palm (Cycad) / Amanita / Blue-Green Algae (Microcystin) Hepatotoxicity,
// N-Acetylcysteine (NAC) & Lactulose/Levetiracetam triage operations in a thread-safe store.
type HepaticService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.HepaticOperation
	order []string
	seq   int
}

// NewHepaticService initializes a HepaticService with 3 seeded operations (HEP-OP-101, HEP-OP-102, HEP-OP-103).
func NewHepaticService() *HepaticService {
	s := &HepaticService{
		ops:   make(map[string]*domain.HepaticOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildEncephalopathyPlan computes the etiology pathophysiology, hyperammonemia neurological status,
// synthetic failure hypoglycemia/coagulopathy note, lactulose NH3 -> NH4+ ion trapping plan,
// benzodiazepine/methionine contraindication guard, and refeeding electrolyte alert.
func BuildEncephalopathyPlan(
	etiology domain.HepatotoxinEtiologyType,
	altIUL int,
	bilirubin, ammonia, glucose, ptSeconds float64,
	heGrade int,
	headPressingSeizures bool,
) domain.HepatotoxinEncephalopathyPlan {
	var etiologySummary string
	switch etiology {
	case domain.HepatotoxinEtiologySagoPalmCycasin:
		etiologySummary = fmt.Sprintf(
			"Sago Palm (Cycas revoluta) Cycasin / Methylazoxymethanol (MAM) & BMAA Hepatotoxicity — Acute Centrilobular Hepatic Necrosis (ALT %d IU/L, Total Bilirubin %.1f mg/dL): Cycasin hydrolyzed by GI beta-glucosidase into hepatotoxic/neurotoxic MAM causing fulminant glutathione depletion, lipid peroxidation, coagulopathy, and hyperammonemic encephalopathy.",
			altIUL, bilirubin,
		)
	case domain.HepatotoxinEtiologyBlueGreenAlgaeAmanita:
		etiologySummary = fmt.Sprintf(
			"Blue-Green Algae (Cyanobacteria Microcystin-LR) / Amanita phalloides (Alpha-Amanitin) Fulminant Hepatotoxicity (ALT %d IU/L, Total Bilirubin %.1f mg/dL): Microcystin inhibits hepatocyte protein phosphatases PP1/PP2A causing cytoskeletal collapse and intrahepatic hemorrhage; Alpha-Amanitin inhibits RNA Polymerase II (stage high-dose Silybin/Penicillin G + NAC).",
			altIUL, bilirubin,
		)
	case domain.HepatotoxinEtiologyFelineLipidosisPSS:
		etiologySummary = fmt.Sprintf(
			"Feline Hepatic Lipidosis / Portosystemic Shunt (PSS) & Arginine-Deficiency Hyperammonemia (ALT %d IU/L, Total Bilirubin %.1f mg/dL): Severe hepatocellular triglyceride vacuolation with Marked Hyperbilirubinemia, Urea Cycle Arginine Depletion, and Portal-Systemic Shunting of Colonic Ammonia.",
			altIUL, bilirubin,
		)
	default:
		etiologySummary = fmt.Sprintf(
			"Acute Fulminant Hepatotoxicity & Synthetic Hepatic Failure (ALT %d IU/L, Total Bilirubin %.1f mg/dL).",
			altIUL, bilirubin,
		)
	}

	seizureFlag := "No active ictal seizures"
	if headPressingSeizures {
		seizureFlag = "ACTIVE HEAD PRESSING / PTYALISM / HEPATIC ENCEPHALOPATHY SEIZURES PRESENT"
	}
	ammoniaDiagnostic := "Normal Ammonia (< 70 umol/L)"
	if ammonia >= 70.0 {
		ammoniaDiagnostic = "CRITICAL HYPERAMMONEMIA & ASTROCYTE GLUTAMINE CEREBRAL EDEMA RISK"
	}
	neuroSummary := fmt.Sprintf(
		"Blood Ammonia %.1f umol/L (%s) · Modified Hepatic Encephalopathy Grade %d/4 — %s.",
		ammonia, ammoniaDiagnostic, heGrade, seizureFlag,
	)

	hypoglycemiaNote := fmt.Sprintf(
		"Blood Glucose %.1f mg/dL & Prothrombin Time (PT) %.1f sec: > 70%% functional hepatic mass loss impairs gluconeogenesis/glycogenolysis (hypoglycemia < 60 mg/dL mimics/exacerbates HE) and depletes Vitamin K-dependent clotting factors II, VII, IX, X (give Vitamin K1 1–2.5 mg/kg SC q12h + Fresh Frozen Plasma 10–15 mL/kg IV BEFORE liver aspirate/biopsy).",
		glucose, ptSeconds,
	)

	lactulosePlan := "Colonic Acidification & NH3 -> NH4+ Ion Trapping: Administer warm water + Lactulose retention enema (1 part Lactulose : 3 parts warm water at 10–20 mL/kg via Foley catheter for 20–30 min) to acidify colonic lumen, converting diffusible ammonia (NH3) into non-absorbable ammonium ions (NH4+), plus oral Lactulose 0.5 mL/kg PO q8h + Neomycin 20 mg/kg PO q8h or low-dose Metronidazole 7.5 mg/kg PO/IV q12h."

	benzoWarning := "CONTRAINDICATION GUARD: NEVER administer Benzodiazepines (Diazepam / Midazolam) for Hepatic Encephalopathy seizures — endogenous benzodiazepine-like ligands & GABA-A receptor upregulation in HE cause paradoxical coma/respiratory arrest (use Levetiracetam 60 mg/kg IV loading dose instead). NEVER administer Methionine or high-protein diets during acute hyperammonemic HE (mercaptan conversion worsens coma)."

	refeedingAlert := "Electrolyte, Hypokalemia & Refeeding Guard: Hypokalemia and metabolic alkalosis increase renal tubular NH3 reabsorption and shifts NH4+ -> NH3 across the blood-brain barrier. In Feline Hepatic Lipidosis, supplement KCl, KPO4 (monitor for refeeding hemolytic hypophosphatemia < 1.5 mg/dL), Thiamine (Vitamin B1 50–100 mg SC q12h), Cobalamin (B12), and Taurine/L-Carnitine."

	return domain.HepatotoxinEncephalopathyPlan{
		EtiologyPathophysiology:         etiologySummary,
		HyperammonemiaNeuroStatus:       neuroSummary,
		HypoglycemiaCoagulopathyNote:    hypoglycemiaNote,
		LactuloseAmmoniumTrappingPlan:   lactulosePlan,
		BenzodiazepineMethionineWarning: benzoWarning,
		RefeedingElectrolyteAlert:       refeedingAlert,
	}
}

// BuildDefaultSupportPlan computes weight-based N-Acetylcysteine (NAC) loading/maintenance doses,
// SAMe/Silybin hepatoprotectant doses, Lactulose enema/oral regimen, Levetiracetam 60 mg/kg IV protocol,
// Dextrose + Vitamin K1 + FFP synthetic support, and electrolyte/thiamine protocol.
func BuildDefaultSupportPlan(weightKg float64, glucose, ptSeconds float64) domain.NACLactuloseSupportPlan {
	nacLoadMg := round1(weightKg * 140.0)
	nacMaintMg := round1(weightKg * 70.0)
	levetiracetamLoadMg := round1(weightKg * 60.0)
	levetiracetamMaintMg := round1(weightKg * 20.0)
	lactuloseEnemaMl := round1(weightKg * 15.0)
	vitK1Mg := round1(weightKg * 1.5)
	ffpMl := round1(weightKg * 12.0)

	dextroseNote := "Maintain normoglycemia (90–140 mg/dL) with 2.5%–5% Dextrose CRI in buffered isotonic crystalloid (Plasmalyte-A / Normosol-R — avoid high-lactate LRS in fulminant liver failure)."
	if glucose < 65.0 {
		dextroseBolusMl := round1(weightKg * 1.0)
		dextroseNote = fmt.Sprintf(
			"ACUTE HYPOGLYCEMIA RESCUE (BG %.1f mg/dL): Administer 0.5 g/kg 50%% Dextrose diluted 1:4 slow IV bolus (%.1f mL of 50%% Dextrose diluted 1:4) followed immediately by 2.5%%–5%% Dextrose CRI in Plasmalyte-A.",
			glucose, dextroseBolusMl,
		)
	}

	return domain.NACLactuloseSupportPlan{
		NACGlutathioneLoadingProtocol: fmt.Sprintf(
			"N-Acetylcysteine (NAC) Glutathione Replenishment: 140 mg/kg IV loading dose (%.1f mg for %.1f kg patient) diluted 1:4 in D5W or 0.9%% NaCl infused over 20–30 min through a 0.2-micron in-line filter, followed by 70 mg/kg IV q6h (%.1f mg IV q6h) for 7 maintenance doses.",
			nacLoadMg, weightKg, nacMaintMg,
		),
		SAMeSilybinHepatoprotectant: "Hepatoprotectant & Antioxidant Regimen: S-Adenosylmethionine (SAMe) 20 mg/kg PO q24h on empty stomach + Silybin-Phosphatidylcholine (Milk Thistle / Marin) 50–250 mg/kg IV/PO (blocks Amanita/Microcystin OATP1B3 hepatocyte uptake and scavenges free radicals) + Ursodiol 15 mg/kg PO q24h once extrahepatic biliary obstruction ruled out on ultrasound.",
		LactuloseEnemaOralProtocol: fmt.Sprintf(
			"Lactulose NH3 -> NH4+ Ion Trapping & GI Decontamination: Cleansing warm isotonic enema followed by Lactulose retention enema (1:3 in warm water, %.1f mL total volume retained 20–30 min q6–8h) + Oral Lactulose 0.5 mL/kg PO q8h titrated to 2–3 soft stools/day + Metronidazole 7.5 mg/kg IV/PO q12h (reduced hepatic clearance dose).",
			lactuloseEnemaMl,
		),
		LevetiracetamNeuroProtocol: fmt.Sprintf(
			"Anticonvulsant Priority (Benzodiazepine-Free): Levetiracetam (Keppra) 60 mg/kg IV loading dose (%.1f mg IV) followed by 20 mg/kg IV q8h (%.1f mg IV q8h) + 30° head elevation + Mannitol 0.5 g/kg IV over 20 min for astrocyte swelling / cerebral edema (STRICTLY AVOID Diazepam/Midazolam).",
			levetiracetamLoadMg, levetiracetamMaintMg,
		),
		DextroseVitaminK1FFPPlan: fmt.Sprintf(
			"%s Coagulopathy (PT %.1f sec): Vitamin K1 (Phytonadione) 1.5 mg/kg SC q12h (%.1f mg SC) + Fresh Frozen Plasma (FFP) 10–15 mL/kg IV (%.1f mL) for active mucosal hemorrhage or pre-procedure hemostasis.",
			dextroseNote, ptSeconds, vitK1Mg, ffpMl,
		),
		PotassiumPhosphateThiaminePlan: "Electrolyte & Micronutrient Stabilization: Supplement KCl 20–40 mEq/L in IV fluids to prevent hypokalemia-induced renal NH3 genesis; Potassium Phosphate 0.01–0.03 mmol/kg/hr IV CRI if PO4 < 2.0 mg/dL; Thiamine (Vitamin B1) 50–100 mg SC q12h + Cobalamin (Vitamin B12) 250–1000 mcg SC.",
	}
}

func (s *HepaticService) seed() {
	now := time.Now().UTC()

	// HEP-OP-101: Kona — Acute Fulminant Hepatotoxicity & Hepatic Encephalopathy Triage (Sago Palm Cycasin)
	plan101 := BuildEncephalopathyPlan(
		domain.HepatotoxinEtiologySagoPalmCycasin,
		2840, 6.8, 245.0, 48.0, 26.4,
		3, true,
	)
	support101 := BuildDefaultSupportPlan(26.4, 48.0, 26.4)
	op101 := &domain.HepaticOperation{
		ID:                          "HEP-OP-101",
		CaseID:                      "CASE-HEP-101",
		PetName:                     "Kona",
		Species:                     "Canine",
		Breed:                       "Labrador Retriever",
		WeightKg:                    26.4,
		Status:                      domain.HepaticStatusFulminantHETriage,
		EtiologyType:                domain.HepatotoxinEtiologySagoPalmCycasin,
		ALTIUL:                      2840,
		TotalBilirubinMgDL:          6.8,
		BloodAmmoniaUmolL:           245.0,
		BloodGlucoseMgDL:            48.0,
		PTSeconds:                   26.4,
		EncephalopathyGrade:         3,
		HeadPressingSeizuresPresent: true,
		EncephalopathyPlan:          plan101,
		SupportPlan:                 support101,
		MetabolicChecks: []domain.HepaticMetabolicCheck{
			{
				ID:                 "HEP-CHK-101-1",
				ALTIUL:             2840,
				TotalBilirubinMgDL: 6.8,
				BloodAmmoniaUmolL:  245.0,
				BloodGlucoseMgDL:   48.0,
				PTSeconds:          26.4,
				RecordedBy:         "Dr. Priya Nair, DACVECC",
				ClinicalNote:       "Ingested Sago Palm (Cycas revoluta) seeds 18h prior; presented with icterus, melena, head pressing, Grade 3 hepatic encephalopathy, hypoglycemia (48.0 mg/dL), and prolonged PT (26.4 sec). Dextrose bolus + Levetiracetam 60 mg/kg IV administered (Diazepam strictly avoided).",
				RecordedAt:         now.Add(-45 * time.Minute),
			},
			{
				ID:                 "HEP-CHK-101-2",
				ALTIUL:             2840,
				TotalBilirubinMgDL: 6.8,
				BloodAmmoniaUmolL:  245.0,
				BloodGlucoseMgDL:   48.0,
				PTSeconds:          26.4,
				RecordedBy:         "Dr. Priya Nair, DACVECC",
				ClinicalNote:       "Preparing N-Acetylcysteine (NAC) 140 mg/kg IV loading dose via 0.2-micron filter, warm Lactulose 1:3 retention enema for colonic NH3 -> NH4+ ion trapping, Vitamin K1 1.5 mg/kg SC, and Fresh Frozen Plasma (FFP) 12 mL/kg IV.",
				RecordedAt:         now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-50 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// HEP-OP-102: Marmalade — NAC, Lactulose & Vitamin K1 Staged (Feline Hepatic Lipidosis / PSS HE)
	plan102 := BuildEncephalopathyPlan(
		domain.HepatotoxinEtiologyFelineLipidosisPSS,
		620, 8.4, 138.0, 82.0, 19.2,
		2, false,
	)
	support102 := BuildDefaultSupportPlan(5.1, 82.0, 19.2)
	op102 := &domain.HepaticOperation{
		ID:                          "HEP-OP-102",
		CaseID:                      "CASE-HEP-102",
		PetName:                     "Marmalade",
		Species:                     "Feline",
		Breed:                       "Domestic Shorthair",
		WeightKg:                    5.1,
		Status:                      domain.HepaticStatusNACLactuloseStaged,
		EtiologyType:                domain.HepatotoxinEtiologyFelineLipidosisPSS,
		ALTIUL:                      620,
		TotalBilirubinMgDL:          8.4,
		BloodAmmoniaUmolL:           138.0,
		BloodGlucoseMgDL:            82.0,
		PTSeconds:                   19.2,
		EncephalopathyGrade:         2,
		HeadPressingSeizuresPresent: false,
		EncephalopathyPlan:          plan102,
		SupportPlan:                 support102,
		MetabolicChecks: []domain.HepaticMetabolicCheck{
			{
				ID:                 "HEP-CHK-102-1",
				ALTIUL:             620,
				TotalBilirubinMgDL: 8.4,
				BloodAmmoniaUmolL:  138.0,
				BloodGlucoseMgDL:   82.0,
				PTSeconds:          19.2,
				RecordedBy:         "Dr. Marcus Thorne, DACVIM",
				ClinicalNote:       "Feline hepatic lipidosis with Grade 2 ptyalism/ataxia HE (Ammonia 138.0 umol/L, Bilirubin 8.4 mg/dL); NAC 70 mg/kg IV q6h, Lactulose retention enema, Vitamin K1 1.5 mg/kg SC q12h, Thiamine, Cobalamin, and KPO4 CRI staged prior to esophagostomy tube refeeding.",
				RecordedAt:         now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// HEP-OP-103: Scout — Ammonia Normalized & Synthetic Function Stable Cleared (Blue-Green Algae Microcystin)
	plan103 := BuildEncephalopathyPlan(
		domain.HepatotoxinEtiologyBlueGreenAlgaeAmanita,
		210, 0.9, 32.0, 98.0, 11.5,
		0, false,
	)
	support103 := BuildDefaultSupportPlan(18.2, 98.0, 11.5)
	dischargedAt := now.Add(-30 * time.Minute)
	op103 := &domain.HepaticOperation{
		ID:                          "HEP-OP-103",
		CaseID:                      "CASE-HEP-103",
		PetName:                     "Scout",
		Species:                     "Canine",
		Breed:                       "Australian Shepherd",
		WeightKg:                    18.2,
		Status:                      domain.HepaticStatusAmmoniaNormalizedCleared,
		EtiologyType:                domain.HepatotoxinEtiologyBlueGreenAlgaeAmanita,
		ALTIUL:                      210,
		TotalBilirubinMgDL:          0.9,
		BloodAmmoniaUmolL:           32.0,
		BloodGlucoseMgDL:            98.0,
		PTSeconds:                   11.5,
		EncephalopathyGrade:         0,
		HeadPressingSeizuresPresent: false,
		EncephalopathyPlan:          plan103,
		SupportPlan:                 support103,
		MetabolicChecks: []domain.HepaticMetabolicCheck{
			{
				ID:                 "HEP-CHK-103-1",
				ALTIUL:             1890,
				TotalBilirubinMgDL: 4.2,
				BloodAmmoniaUmolL:  164.0,
				BloodGlucoseMgDL:   61.0,
				PTSeconds:          21.0,
				RecordedBy:         "Dr. Priya Nair, DACVECC",
				ClinicalNote:       "Pond cyanobacteria (Microcystin-LR) exposure; completed 7-dose IV N-Acetylcysteine protocol, Silybin, Lactulose enemas, and FFP + Vitamin K1.",
				RecordedAt:         now.Add(-24 * time.Hour),
			},
			{
				ID:                 "HEP-CHK-103-2",
				ALTIUL:             210,
				TotalBilirubinMgDL: 0.9,
				BloodAmmoniaUmolL:  32.0,
				BloodGlucoseMgDL:   98.0,
				PTSeconds:          11.5,
				RecordedBy:         "Dr. Priya Nair, DACVECC",
				ClinicalNote:       "Blood ammonia normalized to 32.0 umol/L, HE Grade 0, ALT down to 210 IU/L, Bilirubin 0.9 mg/dL, euglycemic (98.0 mg/dL), and PT normalized to 11.5 sec.",
				RecordedAt:         now.Add(-1 * time.Hour),
			},
		},
		DischargedBy:     "Dr. Priya Nair, DACVECC",
		DischargeSummary: "Microcystin hepatotoxicity resolved following 7-dose NAC IV protocol, Silybin, Lactulose, and Vitamin K1 support. Blood ammonia normalized (32.0 umol/L), HE Grade 0, PT 11.5 sec, euglycemic (98.0 mg/dL), discharged on oral SAMe/Silybin + Lactulose.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-36 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Hepatic operations, optionally filtered by statusFilter.
func (s *HepaticService) ListOperations(statusFilter string) []domain.HepaticOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.HepaticOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneHepaticOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Hepatic operation by ID.
func (s *HepaticService) GetOperation(id string) (*domain.HepaticOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("hepatic encephalopathy operation %q not found", id)
	}
	cp := cloneHepaticOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all Hepatic operations.
func (s *HepaticService) GetOverviewMetrics() domain.HepaticOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.HepaticOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.HepaticStatusFulminantHETriage:
			m.FulminantHETriageCount++
		case domain.HepaticStatusNACLactuloseStaged:
			m.NACLactuloseStagedCount++
		case domain.HepaticStatusAmmoniaNormalizedCleared:
			m.ClearedHepaticCount++
		}
		if op.EtiologyType == domain.HepatotoxinEtiologySagoPalmCycasin ||
			op.EtiologyType == domain.HepatotoxinEtiologyBlueGreenAlgaeAmanita {
			m.SagoPalmMicrocystinCount++
		}
		if op.BloodAmmoniaUmolL >= 70.0 || op.EncephalopathyGrade >= 2 || op.HeadPressingSeizuresPresent {
			m.HyperammonemicEncephalopathyCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Hepatic Failure / Hepatic Encephalopathy triage operation.
func (s *HepaticService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	etiologyType domain.HepatotoxinEtiologyType,
	altIUL int,
	totalBilirubinMgDL, bloodAmmoniaUmolL, bloodGlucoseMgDL, ptSeconds float64,
	encephalopathyGrade int,
	headPressingSeizuresPresent bool,
) (*domain.HepaticOperation, error) {
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
	if !etiologyType.IsValid() {
		return nil, fmt.Errorf("invalid etiology_type: %q", etiologyType)
	}
	if altIUL <= 0 {
		return nil, fmt.Errorf("alt_iu_l must be positive, got %d", altIUL)
	}
	if totalBilirubinMgDL < 0 {
		return nil, fmt.Errorf("total_bilirubin_mg_dl cannot be negative, got %v", totalBilirubinMgDL)
	}
	if bloodAmmoniaUmolL <= 0 {
		return nil, fmt.Errorf("blood_ammonia_umol_l must be positive, got %v", bloodAmmoniaUmolL)
	}
	if bloodGlucoseMgDL < 15.0 || bloodGlucoseMgDL > 600.0 {
		return nil, fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 600.0], got %v", bloodGlucoseMgDL)
	}
	if ptSeconds < 5.0 || ptSeconds > 120.0 {
		return nil, fmt.Errorf("pt_seconds must be in [5.0, 120.0], got %v", ptSeconds)
	}
	if encephalopathyGrade < 0 || encephalopathyGrade > 4 {
		return nil, fmt.Errorf("encephalopathy_grade must be in [0, 4], got %d", encephalopathyGrade)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("HEP-OP-%d", s.seq)
	now := time.Now().UTC()

	hePlan := BuildEncephalopathyPlan(
		etiologyType,
		altIUL,
		round1(totalBilirubinMgDL),
		round1(bloodAmmoniaUmolL),
		round1(bloodGlucoseMgDL),
		round1(ptSeconds),
		encephalopathyGrade,
		headPressingSeizuresPresent,
	)
	supportPlan := BuildDefaultSupportPlan(weightKg, round1(bloodGlucoseMgDL), round1(ptSeconds))

	op := &domain.HepaticOperation{
		ID:                          opID,
		CaseID:                      trimmedCase,
		PetName:                     trimmedPet,
		Species:                     trimmedSpecies,
		Breed:                       trimmedBreed,
		WeightKg:                    round1(weightKg),
		Status:                      domain.HepaticStatusFulminantHETriage,
		EtiologyType:                etiologyType,
		ALTIUL:                      altIUL,
		TotalBilirubinMgDL:          round1(totalBilirubinMgDL),
		BloodAmmoniaUmolL:           round1(bloodAmmoniaUmolL),
		BloodGlucoseMgDL:            round1(bloodGlucoseMgDL),
		PTSeconds:                   round1(ptSeconds),
		EncephalopathyGrade:         encephalopathyGrade,
		HeadPressingSeizuresPresent: headPressingSeizuresPresent,
		EncephalopathyPlan:          hePlan,
		SupportPlan:                 supportPlan,
		MetabolicChecks:             []domain.HepaticMetabolicCheck{},
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneHepaticOperation(op)
	return &cp, nil
}

// StageNACLactuloseSupport stages the N-Acetylcysteine (NAC) loading/maintenance protocol,
// SAMe/Silybin hepatoprotectant plan, Lactulose retention enema & oral protocol, Levetiracetam
// anticonvulsant protocol, Dextrose + Vitamin K1 + FFP synthetic support, and potassium/phosphate/thiamine
// plan, transitioning status to NAC_LACTULOSE_VITAMIN_K1_STAGED.
func (s *HepaticService) StageNACLactuloseSupport(
	id, nacProtocol, sameSilybinPlan, lactuloseProtocol, levetiracetamProtocol, dextroseVitK1Plan, electrolyteThiaminePlan string,
) (*domain.HepaticOperation, error) {
	trimmedNAC := strings.TrimSpace(nacProtocol)
	if trimmedNAC == "" {
		return nil, fmt.Errorf("nac_glutathione_loading_protocol is required")
	}
	trimmedLactulose := strings.TrimSpace(lactuloseProtocol)
	if trimmedLactulose == "" {
		return nil, fmt.Errorf("lactulose_enema_oral_protocol is required")
	}
	trimmedLevetiracetam := strings.TrimSpace(levetiracetamProtocol)
	if trimmedLevetiracetam == "" {
		return nil, fmt.Errorf("levetiracetam_neuro_protocol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("hepatic encephalopathy operation %q not found", id)
	}

	defaults := BuildDefaultSupportPlan(op.WeightKg, op.BloodGlucoseMgDL, op.PTSeconds)
	trimmedSAMe := strings.TrimSpace(sameSilybinPlan)
	if trimmedSAMe == "" {
		trimmedSAMe = defaults.SAMeSilybinHepatoprotectant
	}
	trimmedDexVitK := strings.TrimSpace(dextroseVitK1Plan)
	if trimmedDexVitK == "" {
		trimmedDexVitK = defaults.DextroseVitaminK1FFPPlan
	}
	trimmedElec := strings.TrimSpace(electrolyteThiaminePlan)
	if trimmedElec == "" {
		trimmedElec = defaults.PotassiumPhosphateThiaminePlan
	}

	op.SupportPlan = domain.NACLactuloseSupportPlan{
		NACGlutathioneLoadingProtocol:  trimmedNAC,
		SAMeSilybinHepatoprotectant:    trimmedSAMe,
		LactuloseEnemaOralProtocol:     trimmedLactulose,
		LevetiracetamNeuroProtocol:     trimmedLevetiracetam,
		DextroseVitaminK1FFPPlan:       trimmedDexVitK,
		PotassiumPhosphateThiaminePlan: trimmedElec,
	}
	op.Status = domain.HepaticStatusNACLactuloseStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneHepaticOperation(op)
	return &cp, nil
}

// RecordMetabolicCheck appends a HepaticMetabolicCheck and updates op.ALTIUL,
// op.TotalBilirubinMgDL, op.BloodAmmoniaUmolL, op.BloodGlucoseMgDL, and op.PTSeconds.
func (s *HepaticService) RecordMetabolicCheck(
	id string,
	altIUL int,
	totalBilirubinMgDL, bloodAmmoniaUmolL, bloodGlucoseMgDL, ptSeconds float64,
	recordedBy, clinicalNote string,
) (*domain.HepaticOperation, error) {
	if altIUL <= 0 {
		return nil, fmt.Errorf("alt_iu_l must be positive, got %d", altIUL)
	}
	if totalBilirubinMgDL < 0 {
		return nil, fmt.Errorf("total_bilirubin_mg_dl cannot be negative, got %v", totalBilirubinMgDL)
	}
	if bloodAmmoniaUmolL <= 0 {
		return nil, fmt.Errorf("blood_ammonia_umol_l must be positive, got %v", bloodAmmoniaUmolL)
	}
	if bloodGlucoseMgDL < 15.0 || bloodGlucoseMgDL > 600.0 {
		return nil, fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 600.0], got %v", bloodGlucoseMgDL)
	}
	if ptSeconds < 5.0 || ptSeconds > 120.0 {
		return nil, fmt.Errorf("pt_seconds must be in [5.0, 120.0], got %v", ptSeconds)
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
		return nil, fmt.Errorf("hepatic encephalopathy operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.HepaticMetabolicCheck{
		ID:                 fmt.Sprintf("HEP-CHK-%s-%d", op.ID, len(op.MetabolicChecks)+1),
		ALTIUL:             altIUL,
		TotalBilirubinMgDL: round1(totalBilirubinMgDL),
		BloodAmmoniaUmolL:  round1(bloodAmmoniaUmolL),
		BloodGlucoseMgDL:   round1(bloodGlucoseMgDL),
		PTSeconds:          round1(ptSeconds),
		RecordedBy:         trimmedBy,
		ClinicalNote:       trimmedNote,
		RecordedAt:         now,
	}

	op.MetabolicChecks = append(op.MetabolicChecks, chk)
	op.ALTIUL = chk.ALTIUL
	op.TotalBilirubinMgDL = chk.TotalBilirubinMgDL
	op.BloodAmmoniaUmolL = chk.BloodAmmoniaUmolL
	op.BloodGlucoseMgDL = chk.BloodGlucoseMgDL
	op.PTSeconds = chk.PTSeconds

	if op.BloodAmmoniaUmolL < 70.0 {
		op.HeadPressingSeizuresPresent = false
		if op.EncephalopathyGrade > 0 {
			op.EncephalopathyGrade = 0
		}
	}

	op.EncephalopathyPlan = BuildEncephalopathyPlan(
		op.EtiologyType,
		op.ALTIUL,
		op.TotalBilirubinMgDL,
		op.BloodAmmoniaUmolL,
		op.BloodGlucoseMgDL,
		op.PTSeconds,
		op.EncephalopathyGrade,
		op.HeadPressingSeizuresPresent,
	)
	op.UpdatedAt = now

	cp := cloneHepaticOperation(op)
	return &cp, nil
}

// ClearHepaticDischarge transitions the operation to AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED
// once blood ammonia normalization, resolution of hepatic encephalopathy, euglycemia, and synthetic
// coagulation stability are verified.
func (s *HepaticService) ClearHepaticDischarge(id, dischargedBy, summary string) (*domain.HepaticOperation, error) {
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
		return nil, fmt.Errorf("hepatic encephalopathy operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.HepaticStatusAmmoniaNormalizedCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneHepaticOperation(op)
	return &cp, nil
}

func cloneHepaticOperation(op *domain.HepaticOperation) domain.HepaticOperation {
	cp := *op
	if op.MetabolicChecks != nil {
		cp.MetabolicChecks = make([]domain.HepaticMetabolicCheck, len(op.MetabolicChecks))
		copy(cp.MetabolicChecks, op.MetabolicChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
