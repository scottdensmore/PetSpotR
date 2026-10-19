package heartwormcaval

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// HeartwormService manages Acute Heartworm Caval Syndrome (Dirofilaria immitis),
// Vena Cava Worm Burden Echocardiography, Intravascular Hemolysis (Hemoglobinuria),
// Jugular Venotomy Extraction & Melarsomine Split-Dose / Doxycycline / Macrocyclic Lactone
// Thromboembolism Prevention triage operations in a thread-safe store.
type HeartwormService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.HeartwormCavalOperation
	order []string
	seq   int
}

// NewHeartwormService initializes a HeartwormService with 3 seeded operations (HWC-OP-101, HWC-OP-102, HWC-OP-103).
func NewHeartwormService() *HeartwormService {
	s := &HeartwormService{
		ops:   make(map[string]*domain.HeartwormCavalOperation),
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

// CalculateEstimatedRVSPMmHg calculates estimated Right Ventricular Systolic Pressure (RVSP, mmHg)
// from Tricuspid Regurgitation (TR) Doppler jet velocity (m/s) via the modified Bernoulli equation (ΔP = 4v²).
// A TR jet velocity > 3.4 m/s corresponds to ΔP > 46.2 mmHg (severe pulmonary hypertension).
func CalculateEstimatedRVSPMmHg(trJetVelocityMS float64) float64 {
	if trJetVelocityMS <= 0 {
		return 0
	}
	return round1(4.0 * trJetVelocityMS * trJetVelocityMS)
}

// BuildClinicalNarratives constructs the pathophysiology, echocardiography vena cava worm burden,
// and intravascular mechanical hemolysis (hemoglobinuria) clinical notes.
func BuildClinicalNarratives(
	severityClass domain.HeartwormClass,
	trJetVelocityMS float64,
	wormsExtractedCount int,
	pcvPercent float64,
	freeHemoglobinuriaPresent, parallelHyperechoicCavalLines bool,
) (patho, echoNote, hemolysisNote string) {
	switch severityClass {
	case domain.HeartwormClassIVCavalSyndrome:
		patho = "Class IV Heartworm Caval Syndrome (Dirofilaria immitis) — Massive adult worm burden retrograde-migrating from the pulmonary arteries into the right ventricle, right atrium, and cranial/caudal venae cavae, mechanically obstructing tricuspid valve inflow, dropping cardiac output, and shearing erythrocytes."
	case domain.HeartwormClassIIISeverePTERCHF:
		patho = "Class III Severe Dirofilariasis with Pulmonary Thromboembolism (PTE) & Right-Sided Congestive Heart Failure (RCHF) — Obliterative proliferative villous endarteritis of lobar pulmonary arteries, severe pre-capillary pulmonary hypertension, right ventricular concentric/eccentric hypertrophy, and cavitary ascites."
	case domain.HeartwormClassIIModerateAntigenPos:
		patho = "Class II Moderate Canine Heartworm Disease — Dirofilaria immitis antigen and microfilaria positive with moderate pulmonary arterial dilation and perivascular eosinophilic/wolbachial parenchymal inflammation."
	case domain.HeartwormClassFelineHARDDirofilariasis:
		patho = "Feline Heartworm-Associated Respiratory Disease (HARD) — Acute pulmonary intravascular macrophage activation from immature L5 larvae or dying adult Dirofilaria immitis; Melarsomine is strictly contraindicated in cats."
	default:
		patho = "Acute Dirofilaria immitis Cardiopulmonary & Vena Cava Obstruction Triage."
	}

	rvsp := CalculateEstimatedRVSPMmHg(trJetVelocityMS)
	cavalLinesStr := "ABSENT (Caval & Right Atrial Lumen Patent)"
	if parallelHyperechoicCavalLines {
		cavalLinesStr = "PRESENT (Dense Entangled Parallel Hyperechoic 'Equal-Sign' Cuticular Lines Straddling Tricuspid Orifice & Caudal Vena Cava)"
	}
	echoNote = fmt.Sprintf("Right Parasternal / Subcostal Echocardiography: Parallel Hyperechoic Caval Lines %s | TR Doppler Jet %.2f m/s (Estimated Bernoulli ΔP %.1f mmHg) | Worms Extracted: %d — ", cavalLinesStr, trJetVelocityMS, rvsp, wormsExtractedCount)
	if trJetVelocityMS > 3.4 {
		echoNote += "SEVERE PULMONARY HYPERTENSION ALERT (TR Jet > 3.4 m/s, ΔP > 46 mmHg): Initiate Sildenafil (PDE-5 inhibitor) 1–3 mg/kg PO q8–12h + Pimobendan 0.25 mg/kg PO q12h to unload the failing right ventricle."
	} else if trJetVelocityMS > 2.8 {
		echoNote += "Mild-to-moderate pulmonary hypertension (TR Jet 2.81–3.40 m/s); monitor right ventricular outflow and tricuspid regurgitation serial gradients."
	} else {
		echoNote += "Normal-to-baseline tricuspid regurgitation velocity (<= 2.8 m/s); right-heart pressure gradient resolved."
	}

	hbStr := "RESOLVED / ABSENT (Clear Straw-Yellow Urine)"
	if freeHemoglobinuriaPresent {
		hbStr = "ACTIVE PORT-WINE / COFFEE-COLORED HEMOGLOBINURIA (Intravascular Mechanical Microangiopathic Hemolysis)"
	}
	hemolysisNote = fmt.Sprintf("Intravascular Hemolysis & Anemia Telemetry: PCV %.1f%% | Free Hemoglobinuria: %s — ", pcvPercent, hbStr)
	if freeHemoglobinuriaPresent || pcvPercent < 25.0 {
		hemolysisNote += "High-velocity tricuspid regurgitant jet blasting across entangled adult Dirofilaria cuticles causes mechanical RBC fragmentation (schistocytes, acanthocytes,ghost cells), free hemoglobinemia, and pigment nephropathy; urgent transvenous worm extraction required."
	} else {
		hemolysisNote += "Mechanical intravascular shearing hemolysis resolved; PCV stable without ongoing hemoglobinuria."
	}

	return patho, echoNote, hemolysisNote
}

// BuildDefaultInterventionStaging constructs an evidence-based HeartwormInterventionStaging plan.
func BuildDefaultInterventionStaging(
	weightKg float64,
	severityClass domain.HeartwormClass,
	protocol domain.HeartwormProtocol,
	trJetVelocityMS float64,
	parallelHyperechoicCavalLines, freeHemoglobinuriaPresent bool,
	stagedBy string,
	now time.Time,
) domain.HeartwormInterventionStaging {
	doxyMgKgDay := 20.0 // 10 mg/kg PO q12h = 20 mg/kg/day (or 10 mg/kg q12h standard AHS dosing)
	doxySingleDoseMg := round1(weightKg * 10.0)
	predMgKgDay := 1.0 // 0.5 mg/kg PO q12h Week 1 = 1.0 mg/kg/day
	predSingleDoseMg := round1(weightKg * 0.5)
	sildenafilSingleDoseMg := round1(weightKg * 2.0)
	melarsomineDoseMg := round1(weightKg * 2.5)

	immediateContraindicated := severityClass == domain.HeartwormClassIVCavalSyndrome ||
		severityClass == domain.HeartwormClassFelineHARDDirofilariasis ||
		parallelHyperechoicCavalLines ||
		freeHemoglobinuriaPresent

	guardNote := "ABSOLUTE ADULTICIDE CONTRAINDICATION IN CAVAL SYNDROME: NEVER administer immediate Melarsomine (2.5 mg/kg IM) during active Class IV Caval Syndrome (parallel hyperechoic caval lines / port-wine hemoglobinuria) or Feline HARD — synchronous die-off of a massive caval/right-atrial worm burden triggers catastrophic pulmonary arterial occlusion, disseminated intravascular coagulation (DIC), and rapid death. Transvenous right external jugular venotomy extraction MUST precede any adulticide therapy."
	if !immediateContraindicated {
		guardNote = "Caval lumen clear of obstructive worm masses; strictly follow AHS 60-day Doxycycline + Macrocyclic Lactone pretreatment before initiating 3-dose split Melarsomine (Day 60, Day 90, Day 91) to minimize pulmonary thromboembolism."
	}

	jugularPlan := "Right External Jugular Venotomy under local/light balanced anesthesia & fluoroscopic/transesophageal echocardiographic guidance: introduce flexible Ishihara alligator forceps or nitinol endoscopic retrieval basket into the right atrium and cranial/caudal vena cava, extracting worms gently until parallel hyperechoic caval lines and tricuspid inflow obstruction are completely abolished."
	if severityClass == domain.HeartwormClassFelineHARDDirofilariasis {
		jugularPlan = "Feline HARD Management: Prednisolone 1–2 mg/kg PO q24h tapered + bronchodilator/oxygen support; surgical caval extraction reserved only if echocardiographic right atrial/caval worm obstruction is present (avoid worm laceration which triggers fatal feline anaphylaxis)."
	}

	doxyPlan := fmt.Sprintf("Doxycycline 10 mg/kg PO q12h (%.1f mg PO q12h for %.1f kg patient) for 28 consecutive days (Days 1–28) to eradicate endosymbiotic Wolbachia pipientis bacteria, deplete Wolbachia surface protein (WSP) pro-inflammatory endotoxins, sterilize female worms, and markedly attenuate post-adulticide pulmonary parenchymal pathology.", doxySingleDoseMg, weightKg)

	lactonePlan := "Administer Macrocyclic Lactone preventive (Ivermectin, Milbemycin oxime, or Moxidectin) on Day 1, Day 30, and Day 60 to clear circulating L1 microfilariae and arrest L3/L4 tissue larvae; pre-medicate microfilaremic dogs with Diphenhydramine 2 mg/kg IM + Dexamethasone 0.1–0.2 mg/kg IV or Prednisone 0.5 mg/kg PO to prevent acute Mazzotti-like microfilaricidal shock."

	predPlan := fmt.Sprintf("Prednisone Taper Regimen (Starting %.1f mg PO q12h for %.1f kg patient): 0.5 mg/kg PO q12h Week 1, 0.5 mg/kg PO q24h Week 2, 0.5 mg/kg PO q48h Weeks 3–4 at Day 1 macrocyclic lactone initiation and repeated after each Melarsomine injection (Days 60, 90, 91) to control pulmonary endarteritis and thromboembolism.", predSingleDoseMg, weightKg)

	sildenafilPlan := fmt.Sprintf("Sildenafil (PDE-5 Inhibitor) 1–3 mg/kg PO q8–12h (%.1f mg PO q8–12h at 2 mg/kg for %.1f kg patient) + Pimobendan 0.25 mg/kg PO q12h: INDICATED for severe pulmonary hypertension (TR Jet %.2f m/s > 3.4 m/s, Bernoulli ΔP %.1f mmHg > 46 mmHg) to reduce pulmonary vascular resistance and right ventricular afterload.", sildenafilSingleDoseMg, weightKg, trJetVelocityMS, CalculateEstimatedRVSPMmHg(trJetVelocityMS))
	if trJetVelocityMS <= 3.4 {
		sildenafilPlan = fmt.Sprintf("Pulmonary Hypertension Surveillance (Current TR Jet %.2f m/s, ΔP %.1f mmHg <= 46 mmHg): Hold Sildenafil unless TR Jet exceeds 3.4 m/s or right-sided heart failure develops.", trJetVelocityMS, CalculateEstimatedRVSPMmHg(trJetVelocityMS))
	}

	melarsomineSchedule := fmt.Sprintf("AHS 3-Dose Split Melarsomine Dihydrochloride Protocol (2.5 mg/kg = %.1f mg deep epaxial lumbar IM between L3–L5): ONLY AFTER caval extraction stabilization and 60-day Doxycycline/Lactone conditioning — 1st injection on Day 60 (kills ~50%% of worms), followed 30 days later by 2nd & 3rd injections 24h apart on Day 90 and Day 91 (alternating left/right epaxial bellies, killing ~98%% of adult worms safely).", melarsomineDoseMg)
	if severityClass == domain.HeartwormClassFelineHARDDirofilariasis {
		melarsomineSchedule = "FELINE CONTRAINDICATION: Melarsomine dihydrochloride is STRICTLY CONTRAINDICATED in cats due to narrow toxic margin and fatal pulmonary arterial anaphylaxis/thromboembolism from dying worms."
	}

	if strings.TrimSpace(stagedBy) == "" {
		stagedBy = "Dr. Thibodaux Broussard, DACVIM (Cardiology)"
	}

	return domain.HeartwormInterventionStaging{
		Protocol:                            protocol,
		RightJugularVenotomyExtractionPlan:  jugularPlan,
		ImmediateMelarsomineContraindicated: immediateContraindicated,
		AdulticideContraindicationGuardNote: guardNote,
		WolbachiaDoxycyclineMgKgDay:         doxyMgKgDay,
		WolbachiaDoxycycline28DayPlan:       doxyPlan,
		MacrocyclicLactonePretreatPlan:      lactonePlan,
		PrednisoneTaperMgKgDay:              predMgKgDay,
		PrednisoneAntiInflammatoryTaperPlan: predPlan,
		SildenafilPulmonaryHypertensionPlan: sildenafilPlan,
		SplitDoseMelarsomineScheduleNote:    melarsomineSchedule,
		StrictCageRestWeeks:                 8,
		StagedBy:                            stagedBy,
		StagedAt:                            now,
	}
}

func (s *HeartwormService) seed() {
	now := time.Now().UTC()

	// HWC-OP-101: Boudreaux — Canine, Labrador Retriever, 28.5 kg, JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED
	patho101, echo101, hemolysis101 := BuildClinicalNarratives(
		domain.HeartwormClassIVCavalSyndrome,
		3.9, 0, 21.0,
		true, true,
	)
	stage101 := BuildDefaultInterventionStaging(
		28.5,
		domain.HeartwormClassIVCavalSyndrome,
		domain.HeartwormProtocolRightJugularIshiharaExtraction,
		3.9,
		true, true,
		"Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		now.Add(-45*time.Minute),
	)
	op101 := &domain.HeartwormCavalOperation{
		ID:                                  "HWC-OP-101",
		CaseID:                              "CASE-HWC-101",
		PetName:                             "Boudreaux",
		Species:                             "Canine",
		Breed:                               "Labrador Retriever",
		WeightKg:                            28.5,
		Status:                              domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged,
		SeverityClass:                       domain.HeartwormClassIVCavalSyndrome,
		Protocol:                            domain.HeartwormProtocolRightJugularIshiharaExtraction,
		TRJetVelocityMS:                     3.9,
		EstimatedRVSPMmHg:                   CalculateEstimatedRVSPMmHg(3.9),
		WormsExtractedCount:                 0,
		PCVPercent:                          21.0,
		FreeHemoglobinuriaPresent:           true,
		ParallelHyperechoicCavalLines:       true,
		ImmediateMelarsomineContraindicated: true,
		WolbachiaDoxycyclineMgKgDay:         20.0,
		PrednisoneTaperMgKgDay:              1.0,
		StrictCageRestWeeks:                 8,
		SildenafilRequiredAlert:             true,
		PathophysiologySummary:              patho101,
		EchocardiographyCavalNote:           echo101,
		HemolysisHemoglobinuriaNote:         hemolysis101,
		InterventionStaging:                 stage101,
		HemodynamicChecks: []domain.HeartwormHemodynamicCheck{
			{
				ID:                            "HWC-CHK-101-1",
				TRJetVelocityMS:               3.9,
				EstimatedRVSPMmHg:             CalculateEstimatedRVSPMmHg(3.9),
				WormsExtractedCount:           0,
				PCVPercent:                    21.0,
				FreeHemoglobinuriaPresent:     true,
				ParallelHyperechoicCavalLines: true,
				SildenafilMgKgDose:            2.0,
				RecordedBy:                    "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
				ClinicalNote:                  "Class IV Heartworm Caval Syndrome admission: acute right-heart collapse, loud grade V/VI right apical systolic tricuspid murmur, port-wine hemoglobinuria, PCV 21.0% with schistocytes, parallel hyperechoic 'equal-sign' worm lines filling right atrium/caudal vena cava, TR jet 3.9 m/s (ΔP 60.8 mmHg); immediate Melarsomine strictly contraindicated — prepped for right external jugular Ishihara forceps extraction.",
				RecordedAt:                    now.Add(-45 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-45 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// HWC-OP-102: Magnolia — Canine, Australian Shepherd, 21.0 kg, PULMONARY_HYPERTENSION_PTE_MONITORED
	patho102, echo102, hemolysis102 := BuildClinicalNarratives(
		domain.HeartwormClassIIISeverePTERCHF,
		3.6, 26, 33.5,
		false, false,
	)
	stage102 := BuildDefaultInterventionStaging(
		21.0,
		domain.HeartwormClassIIISeverePTERCHF,
		domain.HeartwormProtocolSplitDoseMelarsomineDay609091,
		3.6,
		false, false,
		"Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		now.Add(-90*time.Minute),
	)
	op102 := &domain.HeartwormCavalOperation{
		ID:                                  "HWC-OP-102",
		CaseID:                              "CASE-HWC-102",
		PetName:                             "Magnolia",
		Species:                             "Canine",
		Breed:                               "Australian Shepherd",
		WeightKg:                            21.0,
		Status:                              domain.HeartwormStatusPulmonaryHypertensionPTEMonitored,
		SeverityClass:                       domain.HeartwormClassIIISeverePTERCHF,
		Protocol:                            domain.HeartwormProtocolSplitDoseMelarsomineDay609091,
		TRJetVelocityMS:                     3.6,
		EstimatedRVSPMmHg:                   CalculateEstimatedRVSPMmHg(3.6),
		WormsExtractedCount:                 26,
		PCVPercent:                          33.5,
		FreeHemoglobinuriaPresent:           false,
		ParallelHyperechoicCavalLines:       false,
		ImmediateMelarsomineContraindicated: false,
		WolbachiaDoxycyclineMgKgDay:         20.0,
		PrednisoneTaperMgKgDay:              1.0,
		StrictCageRestWeeks:                 8,
		SildenafilRequiredAlert:             true,
		PathophysiologySummary:              patho102,
		EchocardiographyCavalNote:           echo102,
		HemolysisHemoglobinuriaNote:         hemolysis102,
		InterventionStaging:                 stage102,
		HemodynamicChecks: []domain.HeartwormHemodynamicCheck{
			{
				ID:                            "HWC-CHK-102-1",
				TRJetVelocityMS:               3.6,
				EstimatedRVSPMmHg:             CalculateEstimatedRVSPMmHg(3.6),
				WormsExtractedCount:           26,
				PCVPercent:                    33.5,
				FreeHemoglobinuriaPresent:     false,
				ParallelHyperechoicCavalLines: false,
				SildenafilMgKgDose:            2.0,
				RecordedBy:                    "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
				ClinicalNote:                  "26 adult Dirofilaria immitis extracted via right external jugular venotomy; caval lumen now clear of parallel hyperechoic lines and hemoglobinuria resolved (PCV 33.5%); monitoring residual pulmonary hypertension (TR jet 3.6 m/s, ΔP 51.8 mmHg) on Sildenafil 2 mg/kg PO q8h, Doxycycline 10 mg/kg PO q12h (28d), Prednisone taper, and strict 8-week cage rest prior to Day 60/90/91 split-dose Melarsomine.",
				RecordedAt:                    now.Add(-80 * time.Minute),
			},
		},
		CreatedAt: now.Add(-4 * time.Hour),
		UpdatedAt: now.Add(-80 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// HWC-OP-103: Cypress — Canine, Golden Retriever, 31.0 kg, CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED
	patho103, echo103, hemolysis103 := BuildClinicalNarratives(
		domain.HeartwormClassIIModerateAntigenPos,
		2.3, 19, 43.0,
		false, false,
	)
	stage103 := BuildDefaultInterventionStaging(
		31.0,
		domain.HeartwormClassIIModerateAntigenPos,
		domain.HeartwormProtocolSplitDoseMelarsomineDay609091,
		2.3,
		false, false,
		"Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		now.Add(-12*time.Hour),
	)
	clearedAt := now.Add(-30 * time.Minute)
	clearance103 := &domain.HeartwormClearance{
		ParallelHyperechoicCavalLines: false,
		FreeHemoglobinuriaPresent:     false,
		TRJetVelocityMS:               2.3,
		StrictCageRestVerified:        true,
		AntigenNegativeOrStagedCure:   true,
		ClearedBy:                     "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		ClearanceSummary:              "Caval & right atrial lumen 100% patent (no parallel hyperechoic lines), hemoglobinuria resolved, PCV 43.0%, TR jet normalized to 2.3 m/s (ΔP 21.2 mmHg <= 2.8 m/s threshold), 28-day Doxycycline + 3-dose split Melarsomine (Days 60, 90, 91) and 8-week strict cage rest verified; cleared.",
		ClearedAt:                     clearedAt,
	}
	op103 := &domain.HeartwormCavalOperation{
		ID:                                  "HWC-OP-103",
		CaseID:                              "CASE-HWC-103",
		PetName:                             "Cypress",
		Species:                             "Canine",
		Breed:                               "Golden Retriever",
		WeightKg:                            31.0,
		Status:                              domain.HeartwormStatusCavalPatentAntigenNegativeCleared,
		SeverityClass:                       domain.HeartwormClassIIModerateAntigenPos,
		Protocol:                            domain.HeartwormProtocolSplitDoseMelarsomineDay609091,
		TRJetVelocityMS:                     2.3,
		EstimatedRVSPMmHg:                   CalculateEstimatedRVSPMmHg(2.3),
		WormsExtractedCount:                 19,
		PCVPercent:                          43.0,
		FreeHemoglobinuriaPresent:           false,
		ParallelHyperechoicCavalLines:       false,
		ImmediateMelarsomineContraindicated: false,
		WolbachiaDoxycyclineMgKgDay:         20.0,
		PrednisoneTaperMgKgDay:              1.0,
		StrictCageRestWeeks:                 8,
		SildenafilRequiredAlert:             false,
		PathophysiologySummary:              patho103,
		EchocardiographyCavalNote:           echo103,
		HemolysisHemoglobinuriaNote:         hemolysis103,
		InterventionStaging:                 stage103,
		HemodynamicChecks: []domain.HeartwormHemodynamicCheck{
			{
				ID:                            "HWC-CHK-103-1",
				TRJetVelocityMS:               2.3,
				EstimatedRVSPMmHg:             CalculateEstimatedRVSPMmHg(2.3),
				WormsExtractedCount:           19,
				PCVPercent:                    43.0,
				FreeHemoglobinuriaPresent:     false,
				ParallelHyperechoicCavalLines: false,
				SildenafilMgKgDose:            0.0,
				RecordedBy:                    "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
				ClinicalNote:                  "Post-extraction and completed Day 60/90/91 split-dose Melarsomine + 28-day Wolbachia Doxycycline + 8-week strict cage rest: caval lumen patent, TR jet 2.3 m/s (<= 2.8 m/s), PCV 43.0%, no hemoglobinuria, Knott's & Dirofilaria antigen negative.",
				RecordedAt:                    now.Add(-40 * time.Minute),
			},
		},
		Clearance:        clearance103,
		DischargedBy:     clearance103.ClearedBy,
		DischargeSummary: clearance103.ClearanceSummary,
		DischargedAt:     &clearedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        clearedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Heartworm Caval operations, optionally filtered by statusFilter.
func (s *HeartwormService) ListOperations(statusFilter string) []domain.HeartwormCavalOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.HeartwormCavalOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneHeartwormOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Heartworm Caval operation by ID.
func (s *HeartwormService) GetOperation(id string) (*domain.HeartwormCavalOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("heartworm caval operation %q not found", id)
	}
	cp := cloneHeartwormOperation(op)
	return &cp, nil
}

// CalculateOverview computes summary telemetry metrics across all Heartworm Caval operations.
func (s *HeartwormService) CalculateOverview() domain.HeartwormOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.HeartwormOverview
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged:
			m.JugularVenotomyWolbachiaStagedCount++
		case domain.HeartwormStatusPulmonaryHypertensionPTEMonitored:
			m.PulmonaryHypertensionPTEMonitoredCount++
		case domain.HeartwormStatusCavalPatentAntigenNegativeCleared:
			m.CavalPatentClearedCount++
		}
		if op.SeverityClass == domain.HeartwormClassIVCavalSyndrome {
			m.ClassIVCavalSyndromeCount++
		}
		if op.TRJetVelocityMS > 3.4 {
			m.SeverePulmonaryHypertensionCount++
		}
		if op.FreeHemoglobinuriaPresent {
			m.ActiveHemoglobinuriaCount++
		}
	}
	return m
}

// GetOverviewMetrics is an alias for CalculateOverview for handler consistency.
func (s *HeartwormService) GetOverviewMetrics() domain.HeartwormOverview {
	return s.CalculateOverview()
}

// CreateOperation validates and creates a new Acute Heartworm Caval Syndrome triage operation.
func (s *HeartwormService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	severityClass domain.HeartwormClass,
	protocol domain.HeartwormProtocol,
	trJetVelocityMS float64,
	wormsExtractedCount int,
	pcvPercent float64,
	freeHemoglobinuriaPresent, parallelHyperechoicCavalLines bool,
) (*domain.HeartwormCavalOperation, error) {
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
	if !severityClass.IsValid() {
		return nil, fmt.Errorf("invalid severity_class: %q", severityClass)
	}
	if protocol == "" {
		protocol = domain.HeartwormProtocolRightJugularIshiharaExtraction
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if trJetVelocityMS <= 0 || trJetVelocityMS > 7.0 {
		return nil, fmt.Errorf("tr_jet_velocity_ms must be in (0, 7.0], got %v", trJetVelocityMS)
	}
	if wormsExtractedCount < 0 || wormsExtractedCount > 300 {
		return nil, fmt.Errorf("worms_extracted_count must be in [0, 300], got %d", wormsExtractedCount)
	}
	if pcvPercent <= 0 || pcvPercent > 75.0 {
		return nil, fmt.Errorf("pcv_percent must be in (0, 75.0], got %v", pcvPercent)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("HWC-OP-%d", s.seq)
	now := time.Now().UTC()

	patho, echoNote, hemolysisNote := BuildClinicalNarratives(
		severityClass,
		trJetVelocityMS,
		wormsExtractedCount,
		pcvPercent,
		freeHemoglobinuriaPresent,
		parallelHyperechoicCavalLines,
	)
	staging := BuildDefaultInterventionStaging(
		weightKg,
		severityClass,
		protocol,
		trJetVelocityMS,
		parallelHyperechoicCavalLines,
		freeHemoglobinuriaPresent,
		"Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		now,
	)

	op := &domain.HeartwormCavalOperation{
		ID:                                  opID,
		CaseID:                              trimmedCase,
		PetName:                             trimmedPet,
		Species:                             trimmedSpecies,
		Breed:                               trimmedBreed,
		WeightKg:                            round1(weightKg),
		Status:                              domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged,
		SeverityClass:                       severityClass,
		Protocol:                            protocol,
		TRJetVelocityMS:                     round2(trJetVelocityMS),
		EstimatedRVSPMmHg:                   CalculateEstimatedRVSPMmHg(trJetVelocityMS),
		WormsExtractedCount:                 wormsExtractedCount,
		PCVPercent:                          round1(pcvPercent),
		FreeHemoglobinuriaPresent:           freeHemoglobinuriaPresent,
		ParallelHyperechoicCavalLines:       parallelHyperechoicCavalLines,
		ImmediateMelarsomineContraindicated: staging.ImmediateMelarsomineContraindicated,
		WolbachiaDoxycyclineMgKgDay:         staging.WolbachiaDoxycyclineMgKgDay,
		PrednisoneTaperMgKgDay:              staging.PrednisoneTaperMgKgDay,
		StrictCageRestWeeks:                 staging.StrictCageRestWeeks,
		SildenafilRequiredAlert:             trJetVelocityMS > 3.4,
		PathophysiologySummary:              patho,
		EchocardiographyCavalNote:           echoNote,
		HemolysisHemoglobinuriaNote:         hemolysisNote,
		InterventionStaging:                 staging,
		HemodynamicChecks:                   []domain.HeartwormHemodynamicCheck{},
		CreatedAt:                           now,
		UpdatedAt:                           now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneHeartwormOperation(op)
	return &cp, nil
}

// StageIntervention stages transvenous right external jugular venotomy extraction,
// Wolbachia 28-day Doxycycline + Macrocyclic Lactone pretreatment, Prednisone anti-inflammatory taper,
// Sildenafil pulmonary hypertension support, and AHS split-dose Melarsomine schedule,
// enforcing the absolute contraindication against immediate Melarsomine during active Class IV Caval Syndrome.
func (s *HeartwormService) StageIntervention(
	id string,
	protocol domain.HeartwormProtocol,
	jugularExtractionPlan string,
	immediateMelarsomineContraindicated bool,
	doxyMgKgDay float64,
	doxy28DayPlan, lactonePretreatPlan string,
	predTaperMgKgDay float64,
	predTaperPlan, sildenafilPlan, splitMelarsomineNote string,
	strictCageRestWeeks int,
	stagedBy string,
) (*domain.HeartwormCavalOperation, error) {
	if protocol == "" {
		protocol = domain.HeartwormProtocolRightJugularIshiharaExtraction
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	trimmedJugular := strings.TrimSpace(jugularExtractionPlan)
	if trimmedJugular == "" {
		return nil, fmt.Errorf("right_jugular_venotomy_extraction_plan is required")
	}
	trimmedDoxyPlan := strings.TrimSpace(doxy28DayPlan)
	if trimmedDoxyPlan == "" {
		return nil, fmt.Errorf("wolbachia_doxycycline_28day_plan is required")
	}
	trimmedLactone := strings.TrimSpace(lactonePretreatPlan)
	if trimmedLactone == "" {
		return nil, fmt.Errorf("macrocyclic_lactone_pretreat_plan is required")
	}
	if doxyMgKgDay == 0 {
		doxyMgKgDay = 20.0
	}
	if doxyMgKgDay < 10.0 || doxyMgKgDay > 30.0 {
		return nil, fmt.Errorf("wolbachia_doxycycline_mg_kg_day must be in [10.0, 30.0] (10 mg/kg PO q12h), got %v", doxyMgKgDay)
	}
	if predTaperMgKgDay == 0 {
		predTaperMgKgDay = 1.0
	}
	if predTaperMgKgDay <= 0 || predTaperMgKgDay > 4.0 {
		return nil, fmt.Errorf("prednisone_taper_mg_kg_day must be in (0, 4.0], got %v", predTaperMgKgDay)
	}
	if strictCageRestWeeks == 0 {
		strictCageRestWeeks = 8
	}
	if strictCageRestWeeks < 6 || strictCageRestWeeks > 16 {
		return nil, fmt.Errorf("strict_cage_rest_weeks must be in [6, 16] weeks to prevent fatal pulmonary thromboembolism, got %d", strictCageRestWeeks)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("heartworm caval operation %q not found", id)
	}

	// Safety Guard: Absolute Contraindication of Immediate Melarsomine in Active Class IV Caval Syndrome
	// or Feline HARD when caval worm burden (parallel hyperechoic lines) or hemoglobinuria is present.
	activeCavalBurden := op.SeverityClass == domain.HeartwormClassIVCavalSyndrome &&
		(op.ParallelHyperechoicCavalLines || op.FreeHemoglobinuriaPresent)
	if (activeCavalBurden || op.SeverityClass == domain.HeartwormClassFelineHARDDirofilariasis) &&
		protocol == domain.HeartwormProtocolSplitDoseMelarsomineDay609091 &&
		!immediateMelarsomineContraindicated {
		return nil, fmt.Errorf("immediate melarsomine adulticide (2.5 mg/kg) is strictly contraindicated during active Class IV Caval Syndrome or Feline HARD with parallel hyperechoic caval lines or hemoglobinuria; transvenous right external jugular extraction must precede adulticide therapy")
	}

	now := time.Now().UTC()
	defaults := BuildDefaultInterventionStaging(
		op.WeightKg,
		op.SeverityClass,
		protocol,
		op.TRJetVelocityMS,
		op.ParallelHyperechoicCavalLines,
		op.FreeHemoglobinuriaPresent,
		stagedBy,
		now,
	)

	trimmedPredPlan := strings.TrimSpace(predTaperPlan)
	if trimmedPredPlan == "" {
		trimmedPredPlan = defaults.PrednisoneAntiInflammatoryTaperPlan
	}
	trimmedSildenafil := strings.TrimSpace(sildenafilPlan)
	if trimmedSildenafil == "" {
		trimmedSildenafil = defaults.SildenafilPulmonaryHypertensionPlan
	}
	trimmedMelarsomine := strings.TrimSpace(splitMelarsomineNote)
	if trimmedMelarsomine == "" {
		trimmedMelarsomine = defaults.SplitDoseMelarsomineScheduleNote
	}
	trimmedBy := strings.TrimSpace(stagedBy)
	if trimmedBy == "" {
		trimmedBy = defaults.StagedBy
	}

	if activeCavalBurden || op.SeverityClass == domain.HeartwormClassFelineHARDDirofilariasis {
		immediateMelarsomineContraindicated = true
	}

	op.Protocol = protocol
	op.ImmediateMelarsomineContraindicated = immediateMelarsomineContraindicated
	op.WolbachiaDoxycyclineMgKgDay = round1(doxyMgKgDay)
	op.PrednisoneTaperMgKgDay = round2(predTaperMgKgDay)
	op.StrictCageRestWeeks = strictCageRestWeeks
	op.InterventionStaging = domain.HeartwormInterventionStaging{
		Protocol:                            protocol,
		RightJugularVenotomyExtractionPlan:  trimmedJugular,
		ImmediateMelarsomineContraindicated: immediateMelarsomineContraindicated,
		AdulticideContraindicationGuardNote: defaults.AdulticideContraindicationGuardNote,
		WolbachiaDoxycyclineMgKgDay:         op.WolbachiaDoxycyclineMgKgDay,
		WolbachiaDoxycycline28DayPlan:       trimmedDoxyPlan,
		MacrocyclicLactonePretreatPlan:      trimmedLactone,
		PrednisoneTaperMgKgDay:              op.PrednisoneTaperMgKgDay,
		PrednisoneAntiInflammatoryTaperPlan: trimmedPredPlan,
		SildenafilPulmonaryHypertensionPlan: trimmedSildenafil,
		SplitDoseMelarsomineScheduleNote:    trimmedMelarsomine,
		StrictCageRestWeeks:                 strictCageRestWeeks,
		StagedBy:                            trimmedBy,
		StagedAt:                            now,
	}
	op.Status = domain.HeartwormStatusPulmonaryHypertensionPTEMonitored
	op.Clearance = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = now

	cp := cloneHeartwormOperation(op)
	return &cp, nil
}

// RecordHemodynamicCheck appends a serial HeartwormHemodynamicCheck and updates echocardiographic
// and intravascular hemolysis telemetry on the operation.
func (s *HeartwormService) RecordHemodynamicCheck(
	id string,
	trJetVelocityMS float64,
	wormsExtractedCount int,
	pcvPercent float64,
	freeHemoglobinuriaPresent, parallelHyperechoicCavalLines bool,
	sildenafilMgKgDose float64,
	recordedBy, clinicalNote string,
) (*domain.HeartwormCavalOperation, error) {
	if trJetVelocityMS <= 0 || trJetVelocityMS > 7.0 {
		return nil, fmt.Errorf("tr_jet_velocity_ms must be in (0, 7.0], got %v", trJetVelocityMS)
	}
	if wormsExtractedCount < 0 || wormsExtractedCount > 300 {
		return nil, fmt.Errorf("worms_extracted_count must be in [0, 300], got %d", wormsExtractedCount)
	}
	if pcvPercent <= 0 || pcvPercent > 75.0 {
		return nil, fmt.Errorf("pcv_percent must be in (0, 75.0], got %v", pcvPercent)
	}
	if sildenafilMgKgDose < 0 || sildenafilMgKgDose > 10.0 {
		return nil, fmt.Errorf("sildenafil_mg_kg_dose must be in [0, 10.0], got %v", sildenafilMgKgDose)
	}
	// Safety Guard: Severe Pulmonary Hypertension (TR Jet > 3.4 m/s -> Bernoulli ΔP > 46 mmHg) requires Sildenafil >= 1.0 mg/kg
	if trJetVelocityMS > 3.4 && sildenafilMgKgDose <= 0 {
		sildenafilMgKgDose = 2.0
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
		return nil, fmt.Errorf("heartworm caval operation %q not found", id)
	}

	now := time.Now().UTC()
	rvsp := CalculateEstimatedRVSPMmHg(trJetVelocityMS)
	chk := domain.HeartwormHemodynamicCheck{
		ID:                            fmt.Sprintf("HWC-CHK-%s-%d", op.ID, len(op.HemodynamicChecks)+1),
		TRJetVelocityMS:               round2(trJetVelocityMS),
		EstimatedRVSPMmHg:             rvsp,
		WormsExtractedCount:           wormsExtractedCount,
		PCVPercent:                    round1(pcvPercent),
		FreeHemoglobinuriaPresent:     freeHemoglobinuriaPresent,
		ParallelHyperechoicCavalLines: parallelHyperechoicCavalLines,
		SildenafilMgKgDose:            round1(sildenafilMgKgDose),
		RecordedBy:                    trimmedBy,
		ClinicalNote:                  trimmedNote,
		RecordedAt:                    now,
	}

	op.HemodynamicChecks = append(op.HemodynamicChecks, chk)
	op.TRJetVelocityMS = chk.TRJetVelocityMS
	op.EstimatedRVSPMmHg = chk.EstimatedRVSPMmHg
	op.WormsExtractedCount = chk.WormsExtractedCount
	op.PCVPercent = chk.PCVPercent
	op.FreeHemoglobinuriaPresent = chk.FreeHemoglobinuriaPresent
	op.ParallelHyperechoicCavalLines = chk.ParallelHyperechoicCavalLines
	op.SildenafilRequiredAlert = chk.TRJetVelocityMS > 3.4
	op.ImmediateMelarsomineContraindicated = chk.ParallelHyperechoicCavalLines || chk.FreeHemoglobinuriaPresent || op.SeverityClass == domain.HeartwormClassFelineHARDDirofilariasis

	patho, echoNote, hemolysisNote := BuildClinicalNarratives(
		op.SeverityClass,
		op.TRJetVelocityMS,
		op.WormsExtractedCount,
		op.PCVPercent,
		op.FreeHemoglobinuriaPresent,
		op.ParallelHyperechoicCavalLines,
	)
	op.PathophysiologySummary = patho
	op.EchocardiographyCavalNote = echoNote
	op.HemolysisHemoglobinuriaNote = hemolysisNote
	if op.Status == domain.HeartwormStatusJugularVenotomyWolbachiaDoxyStaged {
		op.Status = domain.HeartwormStatusPulmonaryHypertensionPTEMonitored
	}
	op.UpdatedAt = now

	cp := cloneHeartwormOperation(op)
	return &cp, nil
}

// EvaluateClearance verifies that caval & right atrial worm obstruction is abolished
// (parallel_hyperechoic_caval_lines == false), intravascular hemolysis has resolved
// (free_hemoglobinuria_present == false), tricuspid regurgitation velocity is <= 2.8 m/s,
// and strict cage rest is verified (strict_cage_rest_verified == true), transitioning status
// to CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED.
func (s *HeartwormService) EvaluateClearance(
	id string,
	parallelHyperechoicCavalLines, freeHemoglobinuriaPresent bool,
	trJetVelocityMS float64,
	strictCageRestVerified bool,
	clearedBy, clearanceSummary string,
) (*domain.HeartwormCavalOperation, error) {
	if parallelHyperechoicCavalLines {
		return nil, fmt.Errorf("cannot clear heartworm caval operation: parallel_hyperechoic_caval_lines must be false (persistent caval/right-atrial worm burden)")
	}
	if freeHemoglobinuriaPresent {
		return nil, fmt.Errorf("cannot clear heartworm caval operation: free_hemoglobinuria_present must be false (active intravascular hemolysis)")
	}
	if trJetVelocityMS <= 0 || trJetVelocityMS > 2.8 {
		return nil, fmt.Errorf("cannot clear heartworm caval operation: tr_jet_velocity_ms must be in (0, 2.8] m/s, got %v", trJetVelocityMS)
	}
	if !strictCageRestVerified {
		return nil, fmt.Errorf("cannot clear heartworm caval operation: strict_cage_rest_verified must be true (6-8 weeks strict exercise restriction required)")
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
		return nil, fmt.Errorf("heartworm caval operation %q not found", id)
	}

	now := time.Now().UTC()
	op.ParallelHyperechoicCavalLines = false
	op.FreeHemoglobinuriaPresent = false
	op.TRJetVelocityMS = round2(trJetVelocityMS)
	op.EstimatedRVSPMmHg = CalculateEstimatedRVSPMmHg(op.TRJetVelocityMS)
	op.SildenafilRequiredAlert = false
	op.ImmediateMelarsomineContraindicated = op.SeverityClass == domain.HeartwormClassFelineHARDDirofilariasis

	patho, echoNote, hemolysisNote := BuildClinicalNarratives(
		op.SeverityClass,
		op.TRJetVelocityMS,
		op.WormsExtractedCount,
		op.PCVPercent,
		op.FreeHemoglobinuriaPresent,
		op.ParallelHyperechoicCavalLines,
	)
	op.PathophysiologySummary = patho
	op.EchocardiographyCavalNote = echoNote
	op.HemolysisHemoglobinuriaNote = hemolysisNote

	clearance := &domain.HeartwormClearance{
		ParallelHyperechoicCavalLines: false,
		FreeHemoglobinuriaPresent:     false,
		TRJetVelocityMS:               op.TRJetVelocityMS,
		StrictCageRestVerified:        true,
		AntigenNegativeOrStagedCure:   true,
		ClearedBy:                     trimmedBy,
		ClearanceSummary:              trimmedSummary,
		ClearedAt:                     now,
	}
	op.Clearance = clearance
	op.Status = domain.HeartwormStatusCavalPatentAntigenNegativeCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneHeartwormOperation(op)
	return &cp, nil
}

func cloneHeartwormOperation(op *domain.HeartwormCavalOperation) domain.HeartwormCavalOperation {
	cp := *op
	if op.HemodynamicChecks != nil {
		cp.HemodynamicChecks = make([]domain.HeartwormHemodynamicCheck, len(op.HemodynamicChecks))
		copy(cp.HemodynamicChecks, op.HemodynamicChecks)
	}
	if op.Clearance != nil {
		cl := *op.Clearance
		cp.Clearance = &cl
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
