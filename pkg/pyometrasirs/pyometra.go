package pyometrasirs

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// PyometraService manages Acute Pyometra (Open vs. Closed Cervix), POCUS Fluid-Filled
// Uterine Horns, E. coli Endotoxemia/SIRS, Stabilization & Ovariohysterectomy (OHE)
// triage operations in a thread-safe store.
type PyometraService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.PyometraOperation
	order []string
	seq   int
}

// NewPyometraService initializes a PyometraService with 3 seeded operations (PYO-OP-101, PYO-OP-102, PYO-OP-103).
func NewPyometraService() *PyometraService {
	s := &PyometraService{
		ops:   make(map[string]*domain.PyometraOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildEndotoxemiaSIRSPlan computes the cervical patency & rupture risk summary,
// POCUS/AFAST fluid-filled uterine horn finding, E. coli lipopolysaccharide (LPS)
// renal medullary ADH resistance polyuria/polydipsia note, strict contraindication
// of blind percutaneous cystocentesis & forceful abdominal palpation, prostaglandin
// luteolysis warning, and SIRS/lactate shock summary.
func BuildEndotoxemiaSIRSPlan(
	cervixType domain.CervixPatencyType,
	hornDiameterCm, bloodLactateMmolL, wbcCountKUl, bandNeutrophilPercent float64,
	mapMmHg int,
	pocusFreePeritonealFluid, polyuriaPolydipsiaPresent bool,
) domain.EndotoxemiaSIRSPlan {
	var cervixSummary string
	switch cervixType {
	case domain.CervixPatencyClosedRuptureRisk:
		cervixSummary = "CLOSED-CERVIX PYOMETRA (HIGH UTERINE RUPTURE & SEPTIC PERITONITIS RISK): Functional cervical closure prevents drainage of purulent/sanguinopurulent exudate; rapid intraluminal distension over thin necrotic myometrium creates imminent life-threatening rupture and E. coli endotoxic shock."
	case domain.CervixPatencyOpenPurulentDischarge:
		cervixSummary = "OPEN-CERVIX PYOMETRA (PURULENT/MUCOPURULENT VULVAR DISCHARGE): Patent cervix allows partial drainage of purulent exudate, reducing immediate wall tension, but friable cystic endometrial hyperplasia (CEH) myometrium remains susceptible to bacterial translocation and E. coli bacteremia."
	case domain.CervixPatencyStumpOvarianRemnant:
		cervixSummary = "UTERINE STUMP PYOMETRA & OVARIAN REMNANT SYNDROME (ORS): Progesterone-driven cystic endometrial hyperplasia and purulent distension of residual uterine body/cervical stump cranial to cervix following incomplete prior ovariohysterectomy."
	default:
		cervixSummary = "Acute Pyometra / Cystic Endometrial Hyperplasia-Pyometra Complex — Evaluate cervical patency and myometrial wall integrity."
	}

	effusionLabel := "No free peritoneal fluid on AFAST (intact serosal margin)"
	if pocusFreePeritonealFluid {
		effusionLabel = "FREE ECHOGENIC PERITONEAL FLUID PRESENT ON AFAST — SUSPECT UTERINE RUPTURE / SEPTIC PERITONITIS"
	}
	pocusFinding := fmt.Sprintf("POCUS / AFAST Uterine Horn Diameter %.1f cm: Tubular, convoluted, anechoic-to-hypoechoic fluid-filled structures dorsal to the urinary bladder and ventral to the descending colon (%s).", hornDiameterCm, effusionLabel)

	pupdStatus := "Normouric / Early Endotoxemia"
	if polyuriaPolydipsiaPresent {
		pupdStatus = "MARKED POLYURIA / POLYDIPSIA (PU/PD) & ISOSTHENURIA PRESENT"
	}
	adhNote := fmt.Sprintf("E. coli Lipopolysaccharide (LPS) Endotoxin Renal Medullary ADH Antagonism (%s): E. coli endotoxin downregulates renal collecting duct aquaporin-2 water channels and blocks antidiuretic hormone (ADH / vasopressin) responsiveness, causing obligatory secondary nephrogenic diabetes insipidus, severe dehydration, and pre-renal/renal azotemia.", pupdStatus)

	cystoContraindication := fmt.Sprintf("STRICT CONTRAINDICATION GUARD: NEVER perform blind percutaneous cystocentesis or forceful transabdominal uterine palpation (Horn Diameter %.1f cm) — needle puncture or manual compression of a friable, pus-filled uterus causes catastrophic iatrogenic uterine rupture and fulminant E. coli septic peritonitis.", hornDiameterCm)

	prostaglandinWarning := "LUTEOLYSIS / MEDICAL MANAGEMENT WARNING: Prostaglandin F2-alpha (Dinoprost / Cloprostenol) causes acute smooth muscle myometrial contraction prior to cervical relaxation and is STRICTLY CONTRAINDICATED in closed-cervix pyometra or unstable SIRS/endotoxic patients due to high risk of acute uterine rupture; emergent Ovariohysterectomy (OHE) after rapid IV resuscitation is the gold standard."

	sirsState := "Compensated / Post-Op Resolving"
	if bloodLactateMmolL >= 3.0 || bandNeutrophilPercent >= 10.0 || mapMmHg < 65 {
		sirsState = "DECOMPENSATED ENDOTOXIC SIRS / SEPTIC SHOCK"
	}
	sirsSummary := fmt.Sprintf("Lactate %.1f mmol/L · WBC %.1f K/µL · Band Neutrophils %.1f%% (Degenerative Left Shift) · MAP %d mmHg → %s.", bloodLactateMmolL, wbcCountKUl, bandNeutrophilPercent, mapMmHg, sirsState)

	return domain.EndotoxemiaSIRSPlan{
		CervixPatencySummary:           cervixSummary,
		POCUSUterineHornFinding:        pocusFinding,
		EColiADHResistancePolyuriaNote: adhNote,
		CystocentesisContraindication:  cystoContraindication,
		ProstaglandinLuteolysisWarning: prostaglandinWarning,
		SIRSLactateShockSummary:        sirsSummary,
	}
}

// BuildDefaultOHEStabilizationPlan returns an evidence-based StabilizationOHEPlan.
func BuildDefaultOHEStabilizationPlan(weightKg float64, pocusFreePeritonealFluid bool) domain.StabilizationOHEPlan {
	crystalloidBolusMl := round1(weightKg * 20.0)
	enrofloxacinMg := round1(weightKg * 10.0)
	ampSulbactamMg := round1(weightKg * 30.0)
	methadoneMg := round1(weightKg * 0.2)

	lavagePlan := "Intact Uterine Serosa Protocol: Pack off exteriorized uterine horns with moistened sterile laparotomy sponges prior to ovarian/cervical pedicle transection; change gloves & instruments after stump closure."
	if pocusFreePeritonealFluid {
		lavagePlan = "Uterine Rupture / Peritonitis Protocol: Collect aerobic/anaerobic peritoneal fluid culture, perform copious warm sterile 0.9% NaCl abdominal lavage (200–300 mL/kg) until effluent is clear, and place closed-suction Jackson-Pratt (JP) abdominal drain."
	}

	return domain.StabilizationOHEPlan{
		CrystalloidResuscitationPlan: fmt.Sprintf("Buffered Isotonic Crystalloid Resuscitation (Normosol-R / Plasmalyte / LRS): 15–20 mL/kg IV shock bolus (%.0f mL over 15–20 min for %.1f kg patient), reassessing lactate, HR, CRT, and MAP ≥ 65 mmHg; add Norepinephrine 0.1–0.5 mcg/kg/min CRI if fluid-refractory vasodilatory endotoxic shock persists.", crystalloidBolusMl, weightKg),
		EmpiricBactericidalAbxPlan:   fmt.Sprintf("Immediate Empiric Broad-Spectrum Bactericidal IV Antibiotics (within 1 hour of triage, prior to OHE): Enrofloxacin 10 mg/kg slow IV diluted q24h (%.1f mg; 5 mg/kg in cats) + Ampicillin-Sulbactam 30 mg/kg IV q8h (%.1f mg) targeting E. coli and anaerobes.", enrofloxacinMg, ampSulbactamMg),
		SurgicalOHEHandlingProtocol:  "Emergent Ovariohysterectomy (OHE) Handling: Do NOT delay surgery > 1–2 hours once initial shock resuscitation is underway; elevate friable distended uterine horns gently without digital pinching or traction, isolate with moistened laparotomy sponges, and double-ligate ovarian pedicles and uterine body just cranial to cervix with monofilament absorbable suture.",
		AbdominalLavagePlan:          lavagePlan,
		PerioperativeAnalgesiaPlan:   fmt.Sprintf("Multimodal Perioperative Analgesia: Methadone 0.2 mg/kg IV q4h (%.1f mg) or Fentanyl CRI 3–5 mcg/kg/hr + Maropitant 1 mg/kg IV; STRICTLY AVOID NSAIDs (Meloxicam/Carprofen) due to E. coli endotoxin-induced renal medullary ischemia and azotemia.", methadoneMg),
		PostopSepsisMonitoring:       "Post-Op ICU Sepsis & DIC Surveillance: Monitor serial blood lactate, MAP, urine output (target > 1–2 mL/kg/hr as ADH responsiveness recovers over 48–72h), CBC band neutrophil resolution, albumin, and coagulation panel (PT/aPTT/D-dimers/platelets).",
	}
}

func (s *PyometraService) seed() {
	now := time.Now().UTC()

	// PYO-OP-101: Bella — Canine Golden Retriever, Closed-Cervix Endotoxic SIRS Triage
	sirs101 := BuildEndotoxemiaSIRSPlan(
		domain.CervixPatencyClosedRuptureRisk,
		5.4, 4.8, 34.2, 18.0, 58,
		false, true,
	)
	ohe101 := BuildDefaultOHEStabilizationPlan(28.5, false)
	op101 := &domain.PyometraOperation{
		ID:                        "PYO-OP-101",
		CaseID:                    "CASE-PYO-101",
		PetName:                   "Bella",
		Species:                   "Canine",
		Breed:                     "Golden Retriever",
		WeightKg:                  28.5,
		Status:                    domain.PyometraStatusClosedCervixSIRSTriage,
		CervixType:                domain.CervixPatencyClosedRuptureRisk,
		UterineHornDiameterCm:     5.4,
		BloodLactateMmolL:         4.8,
		WBCCountKUl:               34.2,
		BandNeutrophilPercent:     18.0,
		MAPMmHg:                   58,
		POCUSFreePeritonealFluid:  false,
		PolyuriaPolydipsiaPresent: true,
		SIRSPlan:                  sirs101,
		OHEPlan:                   ohe101,
		HemodynamicChecks: []domain.PeriopHemodynamicCheck{
			{
				ID:                    "PYO-CHK-101-1",
				UterineHornDiameterCm: 5.4,
				BloodLactateMmolL:     5.2,
				WBCCountKUl:           34.2,
				BandNeutrophilPercent: 18.0,
				MAPMmHg:               54,
				RecordedBy:            "Dr. Hannah Lin, DACVS",
				ClinicalNote:          "Presentation triage: 7yo intact female Golden Retriever, 5 weeks post-estrus, severe lethargy, PU/PD with isosthenuria (USG 1.008), hyperemic/injected brick-red mucous membranes, closed cervix, POCUS bilateral convoluted fluid-filled uterine horns 5.4 cm; blind cystocentesis strictly prohibited.",
				RecordedAt:            now.Add(-45 * time.Minute),
			},
			{
				ID:                    "PYO-CHK-101-2",
				UterineHornDiameterCm: 5.4,
				BloodLactateMmolL:     4.8,
				WBCCountKUl:           34.2,
				BandNeutrophilPercent: 18.0,
				MAPMmHg:               58,
				RecordedBy:            "Dr. Hannah Lin, DACVS",
				ClinicalNote:          "First 20 mL/kg Normosol-R IV shock bolus + Enrofloxacin 10 mg/kg IV + Ampicillin-Sulbactam 30 mg/kg IV initiated; preparing OR for emergent Ovariohysterectomy (OHE).",
				RecordedAt:            now.Add(-15 * time.Minute),
			},
		},
		CreatedAt: now.Add(-50 * time.Minute),
		UpdatedAt: now.Add(-15 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// PYO-OP-102: Duchess — Feline Persian, Resuscitated Empiric Abx & OHE Staged
	sirs102 := BuildEndotoxemiaSIRSPlan(
		domain.CervixPatencyOpenPurulentDischarge,
		2.8, 2.1, 22.5, 8.0, 76,
		false, true,
	)
	ohe102 := BuildDefaultOHEStabilizationPlan(3.9, false)
	op102 := &domain.PyometraOperation{
		ID:                        "PYO-OP-102",
		CaseID:                    "CASE-PYO-102",
		PetName:                   "Duchess",
		Species:                   "Feline",
		Breed:                     "Persian",
		WeightKg:                  3.9,
		Status:                    domain.PyometraStatusResuscitatedOHEStaged,
		CervixType:                domain.CervixPatencyOpenPurulentDischarge,
		UterineHornDiameterCm:     2.8,
		BloodLactateMmolL:         2.1,
		WBCCountKUl:               22.5,
		BandNeutrophilPercent:     8.0,
		MAPMmHg:                   76,
		POCUSFreePeritonealFluid:  false,
		PolyuriaPolydipsiaPresent: true,
		SIRSPlan:                  sirs102,
		OHEPlan:                   ohe102,
		HemodynamicChecks: []domain.PeriopHemodynamicCheck{
			{
				ID:                    "PYO-CHK-102-1",
				UterineHornDiameterCm: 2.8,
				BloodLactateMmolL:     2.1,
				WBCCountKUl:           22.5,
				BandNeutrophilPercent: 8.0,
				MAPMmHg:               76,
				RecordedBy:            "Dr. Marcus Vance, DACVECC",
				ClinicalNote:          "Open-cervix feline pyometra with purulent vulvar discharge; Normosol-R resuscitation complete (MAP 76 mmHg, lactate 2.1 mmol/L), Ampicillin-Sulbactam + Pradofloxacin/Enrofloxacin IV administered, staged for immediate OHE.",
				RecordedAt:            now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// PYO-OP-103: Daisy — Canine English Springer Spaniel, Post-Op OHE Normotensive Afebrile Cleared
	sirs103 := BuildEndotoxemiaSIRSPlan(
		domain.CervixPatencyStumpOvarianRemnant,
		0.0, 1.2, 11.4, 1.0, 88,
		false, false,
	)
	ohe103 := BuildDefaultOHEStabilizationPlan(19.4, false)
	dischargedAt := now.Add(-30 * time.Minute)
	op103 := &domain.PyometraOperation{
		ID:                        "PYO-OP-103",
		CaseID:                    "CASE-PYO-103",
		PetName:                   "Daisy",
		Species:                   "Canine",
		Breed:                     "English Springer Spaniel",
		WeightKg:                  19.4,
		Status:                    domain.PyometraStatusPostopOHECleared,
		CervixType:                domain.CervixPatencyStumpOvarianRemnant,
		UterineHornDiameterCm:     0.0,
		BloodLactateMmolL:         1.2,
		WBCCountKUl:               11.4,
		BandNeutrophilPercent:     1.0,
		MAPMmHg:                   88,
		POCUSFreePeritonealFluid:  false,
		PolyuriaPolydipsiaPresent: false,
		SIRSPlan:                  sirs103,
		OHEPlan:                   ohe103,
		HemodynamicChecks: []domain.PeriopHemodynamicCheck{
			{
				ID:                    "PYO-CHK-103-1",
				UterineHornDiameterCm: 3.6,
				BloodLactateMmolL:     3.4,
				WBCCountKUl:           26.8,
				BandNeutrophilPercent: 12.0,
				MAPMmHg:               68,
				RecordedBy:            "Dr. Hannah Lin, DACVS",
				ClinicalNote:          "Pre-excision stump pyometra (3.6 cm distended cervical stump + right ovarian pedicle luteal remnant); stabilized on IV crystalloids and Ampicillin-Sulbactam + Enrofloxacin.",
				RecordedAt:            now.Add(-20 * time.Hour),
			},
			{
				ID:                    "PYO-CHK-103-2",
				UterineHornDiameterCm: 0.0,
				BloodLactateMmolL:     1.2,
				WBCCountKUl:           11.4,
				BandNeutrophilPercent: 1.0,
				MAPMmHg:               88,
				RecordedBy:            "Dr. Hannah Lin, DACVS",
				ClinicalNote:          "24h post-op complete uterine stump resection & right ovarian remnant excision: afebrile (38.4°C), normotensive (MAP 88 mmHg), lactate normalized (1.2 mmol/L), degenerative left shift resolved (bands 1.0%), AFAST negative for free fluid.",
				RecordedAt:            now.Add(-1 * time.Hour),
			},
		},
		DischargedBy:     "Dr. Hannah Lin, DACVS",
		DischargeSummary: "Complete surgical excision of uterine stump pyometra and right ovarian remnant (ORS) verified. Patient is normotensive (MAP 88 mmHg), afebrile, eating voluntarily with resolved E. coli endotoxic PU/PD, normalized lactate (1.2 mmol/L), and resolved band neutrophilia (1.0%). Discharged on oral Amoxicillin-Clavulanate + Enrofloxacin pending culture sensitivity.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Pyometra operations, optionally filtered by statusFilter.
func (s *PyometraService) ListOperations(statusFilter string) []domain.PyometraOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.PyometraOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, clonePyometraOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Pyometra operation by ID.
func (s *PyometraService) GetOperation(id string) (*domain.PyometraOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("pyometra operation %q not found", id)
	}
	cp := clonePyometraOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all Pyometra operations.
func (s *PyometraService) GetOverviewMetrics() domain.PyometraOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.PyometraOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.PyometraStatusClosedCervixSIRSTriage:
			m.ClosedCervixSIRSTriageCount++
		case domain.PyometraStatusResuscitatedOHEStaged:
			m.OHEStagedCount++
		case domain.PyometraStatusPostopOHECleared:
			m.ClearedPostopOHECount++
		}
		if op.CervixType == domain.CervixPatencyClosedRuptureRisk {
			m.ClosedCervixRuptureRiskCount++
		}
		if op.POCUSFreePeritonealFluid {
			m.PeritonealEffusionAlertCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Pyometra / Endotoxic SIRS triage operation.
func (s *PyometraService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	cervixType domain.CervixPatencyType,
	uterineHornDiameterCm, bloodLactateMmolL, wbcCountKUl, bandNeutrophilPercent float64,
	mapMmHg int,
	pocusFreePeritonealFluid, polyuriaPolydipsiaPresent bool,
) (*domain.PyometraOperation, error) {
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
	if !cervixType.IsValid() {
		return nil, fmt.Errorf("invalid cervix_type: %q", cervixType)
	}
	if uterineHornDiameterCm < 0 || uterineHornDiameterCm > 25.0 {
		return nil, fmt.Errorf("uterine_horn_diameter_cm must be in [0, 25.0], got %v", uterineHornDiameterCm)
	}
	if bloodLactateMmolL < 0 || bloodLactateMmolL > 30.0 {
		return nil, fmt.Errorf("blood_lactate_mmol_l must be in [0, 30.0], got %v", bloodLactateMmolL)
	}
	if wbcCountKUl <= 0 || wbcCountKUl > 150.0 {
		return nil, fmt.Errorf("wbc_count_k_ul must be in (0, 150.0], got %v", wbcCountKUl)
	}
	if bandNeutrophilPercent < 0 || bandNeutrophilPercent > 100.0 {
		return nil, fmt.Errorf("band_neutrophil_percent must be in [0, 100.0], got %v", bandNeutrophilPercent)
	}
	if mapMmHg < 25 || mapMmHg > 220 {
		return nil, fmt.Errorf("map_mm_hg must be in [25, 220], got %d", mapMmHg)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("PYO-OP-%d", s.seq)
	now := time.Now().UTC()

	diam := round1(uterineHornDiameterCm)
	lact := round1(bloodLactateMmolL)
	wbc := round1(wbcCountKUl)
	bands := round1(bandNeutrophilPercent)

	sirsPlan := BuildEndotoxemiaSIRSPlan(
		cervixType,
		diam, lact, wbc, bands, mapMmHg,
		pocusFreePeritonealFluid, polyuriaPolydipsiaPresent,
	)
	ohePlan := BuildDefaultOHEStabilizationPlan(weightKg, pocusFreePeritonealFluid)

	op := &domain.PyometraOperation{
		ID:                        opID,
		CaseID:                    trimmedCase,
		PetName:                   trimmedPet,
		Species:                   trimmedSpecies,
		Breed:                     trimmedBreed,
		WeightKg:                  weightKg,
		Status:                    domain.PyometraStatusClosedCervixSIRSTriage,
		CervixType:                cervixType,
		UterineHornDiameterCm:     diam,
		BloodLactateMmolL:         lact,
		WBCCountKUl:               wbc,
		BandNeutrophilPercent:     bands,
		MAPMmHg:                   mapMmHg,
		POCUSFreePeritonealFluid:  pocusFreePeritonealFluid,
		PolyuriaPolydipsiaPresent: polyuriaPolydipsiaPresent,
		SIRSPlan:                  sirsPlan,
		OHEPlan:                   ohePlan,
		HemodynamicChecks:         []domain.PeriopHemodynamicCheck{},
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := clonePyometraOperation(op)
	return &cp, nil
}

// StageOHEStabilization stages the IV crystalloid resuscitation plan, empiric bactericidal
// antibiotic protocol (Enrofloxacin + Ampicillin-Sulbactam), surgical OHE handling protocol,
// abdominal lavage plan, perioperative analgesia plan, and post-op sepsis monitoring,
// transitioning status to RESUSCITATED_EMPIRIC_ABX_OHE_STAGED.
func (s *PyometraService) StageOHEStabilization(
	id, crystalloidPlan, empiricAbxPlan, oheHandlingProtocol, lavagePlan, analgesiaPlan, sepsisMonitoring string,
) (*domain.PyometraOperation, error) {
	trimmedCrystalloid := strings.TrimSpace(crystalloidPlan)
	if trimmedCrystalloid == "" {
		return nil, fmt.Errorf("crystalloid_resuscitation_plan is required")
	}
	trimmedAbx := strings.TrimSpace(empiricAbxPlan)
	if trimmedAbx == "" {
		return nil, fmt.Errorf("empiric_bactericidal_abx_plan is required")
	}
	trimmedOHE := strings.TrimSpace(oheHandlingProtocol)
	if trimmedOHE == "" {
		return nil, fmt.Errorf("surgical_ohe_handling_protocol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("pyometra operation %q not found", id)
	}

	defaults := BuildDefaultOHEStabilizationPlan(op.WeightKg, op.POCUSFreePeritonealFluid)
	trimmedLavage := strings.TrimSpace(lavagePlan)
	if trimmedLavage == "" {
		trimmedLavage = defaults.AbdominalLavagePlan
	}
	trimmedAnalgesia := strings.TrimSpace(analgesiaPlan)
	if trimmedAnalgesia == "" {
		trimmedAnalgesia = defaults.PerioperativeAnalgesiaPlan
	}
	trimmedSepsis := strings.TrimSpace(sepsisMonitoring)
	if trimmedSepsis == "" {
		trimmedSepsis = defaults.PostopSepsisMonitoring
	}

	op.OHEPlan = domain.StabilizationOHEPlan{
		CrystalloidResuscitationPlan: trimmedCrystalloid,
		EmpiricBactericidalAbxPlan:   trimmedAbx,
		SurgicalOHEHandlingProtocol:  trimmedOHE,
		AbdominalLavagePlan:          trimmedLavage,
		PerioperativeAnalgesiaPlan:   trimmedAnalgesia,
		PostopSepsisMonitoring:       trimmedSepsis,
	}
	op.Status = domain.PyometraStatusResuscitatedOHEStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := clonePyometraOperation(op)
	return &cp, nil
}

// RecordHemodynamicCheck appends a PeriopHemodynamicCheck and updates op.UterineHornDiameterCm,
// op.BloodLactateMmolL, op.WBCCountKUl, op.BandNeutrophilPercent, and op.MAPMmHg.
func (s *PyometraService) RecordHemodynamicCheck(
	id string,
	uterineHornDiameterCm, bloodLactateMmolL, wbcCountKUl, bandNeutrophilPercent float64,
	mapMmHg int,
	recordedBy, clinicalNote string,
) (*domain.PyometraOperation, error) {
	if uterineHornDiameterCm < 0 || uterineHornDiameterCm > 25.0 {
		return nil, fmt.Errorf("uterine_horn_diameter_cm must be in [0, 25.0], got %v", uterineHornDiameterCm)
	}
	if bloodLactateMmolL < 0 || bloodLactateMmolL > 30.0 {
		return nil, fmt.Errorf("blood_lactate_mmol_l must be in [0, 30.0], got %v", bloodLactateMmolL)
	}
	if wbcCountKUl <= 0 || wbcCountKUl > 150.0 {
		return nil, fmt.Errorf("wbc_count_k_ul must be in (0, 150.0], got %v", wbcCountKUl)
	}
	if bandNeutrophilPercent < 0 || bandNeutrophilPercent > 100.0 {
		return nil, fmt.Errorf("band_neutrophil_percent must be in [0, 100.0], got %v", bandNeutrophilPercent)
	}
	if mapMmHg < 25 || mapMmHg > 220 {
		return nil, fmt.Errorf("map_mm_hg must be in [25, 220], got %d", mapMmHg)
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
		return nil, fmt.Errorf("pyometra operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.PeriopHemodynamicCheck{
		ID:                    fmt.Sprintf("PYO-CHK-%s-%d", op.ID, len(op.HemodynamicChecks)+1),
		UterineHornDiameterCm: round1(uterineHornDiameterCm),
		BloodLactateMmolL:     round1(bloodLactateMmolL),
		WBCCountKUl:           round1(wbcCountKUl),
		BandNeutrophilPercent: round1(bandNeutrophilPercent),
		MAPMmHg:               mapMmHg,
		RecordedBy:            trimmedBy,
		ClinicalNote:          trimmedNote,
		RecordedAt:            now,
	}

	op.HemodynamicChecks = append(op.HemodynamicChecks, chk)
	op.UterineHornDiameterCm = chk.UterineHornDiameterCm
	op.BloodLactateMmolL = chk.BloodLactateMmolL
	op.WBCCountKUl = chk.WBCCountKUl
	op.BandNeutrophilPercent = chk.BandNeutrophilPercent
	op.MAPMmHg = chk.MAPMmHg

	op.SIRSPlan = BuildEndotoxemiaSIRSPlan(
		op.CervixType,
		op.UterineHornDiameterCm, op.BloodLactateMmolL, op.WBCCountKUl, op.BandNeutrophilPercent, op.MAPMmHg,
		op.POCUSFreePeritonealFluid, op.PolyuriaPolydipsiaPresent,
	)
	op.UpdatedAt = now

	cp := clonePyometraOperation(op)
	return &cp, nil
}

// ClearPyometraDischarge transitions the operation to POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED
// once post-operative OHE hemostasis, normotension, lactate clearance, and resolution of SIRS are confirmed.
func (s *PyometraService) ClearPyometraDischarge(id, dischargedBy, summary string) (*domain.PyometraOperation, error) {
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
		return nil, fmt.Errorf("pyometra operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.PyometraStatusPostopOHECleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := clonePyometraOperation(op)
	return &cp, nil
}

func clonePyometraOperation(op *domain.PyometraOperation) domain.PyometraOperation {
	cp := *op
	if op.HemodynamicChecks != nil {
		cp.HemodynamicChecks = make([]domain.PeriopHemodynamicCheck, len(op.HemodynamicChecks))
		copy(cp.HemodynamicChecks, op.HemodynamicChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
