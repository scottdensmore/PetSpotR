package polytraumaortho

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// OrthoService manages Acute Bite Wound, Vehicular Polytrauma, Degloving/Shearing Injury,
// Open Fracture Gustilo-Anderson Classification, Wet-to-Dry/NPWT Vacuum-Assisted Closure &
// Damage-Control Orthopedics triage operations in a thread-safe store.
type OrthoService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.PolytraumaOrthoOperation
	order []string
	seq   int
}

// NewOrthoService initializes an OrthoService with 3 seeded operations (ORTHO-OP-101, ORTHO-OP-102, ORTHO-OP-103).
func NewOrthoService() *OrthoService {
	s := &OrthoService{
		ops:   make(map[string]*domain.PolytraumaOrthoOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// BuildGustiloWoundAssessmentPlan computes the clinical interpretation and contraindication guards
// for a Polytrauma & Orthopedic operation.
func BuildGustiloWoundAssessmentPlan(
	gustiloClass domain.GustiloInjuryClassification,
	attScore, lavageVolumeML, npwtVacuumMmHg int,
	bloodLactateMmolL, distalToeTempDeltaC float64,
	exposedCorticalBoneTendon, thoracicWallPenetrationSuspected bool,
) domain.GustiloWoundAssessmentPlan {
	var patho string
	switch gustiloClass {
	case domain.GustiloClassGradeIIIShearingDegloving:
		patho = "Gustilo-Anderson Grade III Open Fracture & Vehicular Tarsal/Carpal Shearing-Degloving Injury — High-energy road-rash friction and shear forces stripping skin, subcutis, collateral ligaments, and periosteum with exposed cortical bone/tendon and severe polymicrobial road-gravel contamination."
	case domain.GustiloClassBDLDThoracicBiteFlail:
		patho = "Big-Dog-Little-Dog (BDLD) Crushing Thoracic Bite Wound & Pseudo-Flail Chest — High-force jaw compression (> 200–400 PSI) combined with violent shaking, producing small cutaneous fang punctures over massive subcutaneous avulsion, intercostal muscle tearing, rib fractures, and potential pleural penetration."
	case domain.GustiloClassGradeIOpenLongBoneFracture:
		patho = "Gustilo-Anderson Grade I–II Open Diaphyseal Long-Bone Fracture — Inside-out (< 1 cm Grade I) or moderate (1–10 cm Grade II) soft-tissue laceration communicating with fracture hematoma without extensive periosteal stripping or major neurovascular avulsion."
	default:
		patho = "Acute Vehicular Polytrauma, Open Fracture & Contaminated Soft-Tissue Wound — Requires damage-control resuscitation, copious low-pressure lavage, and staged immobilization."
	}

	icebergNote := fmt.Sprintf("BDLD 'Iceberg Effect' & Dead-Space Alert (Thoracic Penetration Suspected: %t | Exposed Bone/Tendon: %t): ", thoracicWallPenetrationSuspected, exposedCorticalBoneTendon)
	if thoracicWallPenetrationSuspected || gustiloClass == domain.GustiloClassBDLDThoracicBiteFlail {
		icebergNote += "CRITICAL ICEBERG EFFECT — External skin punctures represent < 10% of subfascial crushing/shearing damage; mandatory surgical exploration, intercostal/body-wall reconstruction, thoracic POCUS/radiographs for pneumothorax, and active closed-suction (Jackson-Pratt) or NPWT dead-space management."
	} else {
		icebergNote += "Evaluate subfascial dead space, periosteal viability, and collateral ligament integrity under sterile exploration after hemodynamic stabilization."
	}

	lavageNote := fmt.Sprintf("Pulsatile Lavage Telemetry (%d mL Warm Isotonic LRS / 0.9%% NaCl delivered at 7–8 PSI): Deliver via 1 L fluid bag inside 300 mmHg pressure cuff with 18G needle (or 35 mL syringe + 18G needle). STRICTLY AVOID cytotoxic Povidone-Iodine, Chlorhexidine > 0.05%%, or Hydrogen Peroxide in deep open fractures/shearing beds (causes fibroblast necrosis and microvascular thrombosis).", lavageVolumeML)

	closureGuard := fmt.Sprintf("PRIMARY CLOSURE CONTRAINDICATION GUARD (NPWT %d mmHg): DO NOT perform primary tension closure over contaminated Gustilo Grade III shearing/degloving wounds or acute crushing bite wounds without thorough surgical debridement, deep post-lavage culture, and open wound management (NPWT Vacuum-Assisted Closure at -125 mmHg or sterile tie-over/wet-to-dry dressings until healthy granulation bed forms in 3–5 days).", npwtVacuumMmHg)

	splintRule := "IMMOBILIZATION BIOMECHANICS RULE: Temporary Robert Jones bandage or lateral/spica splint MUST immobilize the JOINT ABOVE and JOINT BELOW the fracture/shearing site (e.g., stifle + tarsus for tibia; elbow + carpus for radius/ulna) to prevent sharp cortical fragments from severing neurovascular bundles or converting closed fractures into open fractures."

	neuroVasc := fmt.Sprintf("ATLS / ATT Score %d/18 | Blood Lactate %.1f mmol/L | Distal Toe-Web ΔT %.1f°C: ", attScore, bloodLactateMmolL, distalToeTempDeltaC)
	if distalToeTempDeltaC >= 1.5 || bloodLactateMmolL >= 3.5 {
		neuroVasc += "CRITICAL PERFUSION / DISTAL LIMB ISCHEMIA ALERT (ΔT >= 1.5°C or Lactate >= 3.5 mmol/L): Prioritize shock resuscitation before prolonged orthopedic anesthesia; verify deep pain nociception, plantar/palmar arterial Doppler signal, and check bandage/splint constriction q4–6h."
	} else {
		neuroVasc += "Systemic perfusion and distal extremity vascularity stable (ΔT < 1.5°C, Lactate < 3.5 mmol/L); continue serial toe-web temperature and swelling checks."
	}

	return domain.GustiloWoundAssessmentPlan{
		GustiloGradePathophysiology:    patho,
		IcebergBiteDeadSpaceAlert:      icebergNote,
		HighPressureLavage7to8PSINote:  lavageNote,
		PrimaryClosureContraindication: closureGuard,
		JointAboveJointBelowSplintRule: splintRule,
		VascularNeuroDistalLimbNote:    neuroVasc,
	}
}

// BuildDefaultDebridementFixationPlan returns an evidence-based DebridementFixationPlan.
func BuildDefaultDebridementFixationPlan(
	weightKg float64,
	gustiloClass domain.GustiloInjuryClassification,
	exposedCorticalBoneTendon, thoracicWallPenetrationSuspected bool,
) domain.DebridementFixationPlan {
	cefazolinMg := round1(weightKg * 22.0)

	gelClipPlan := "Pack open wound bed generously with sterile water-soluble lubricant (K-Y Jelly) BEFORE wide circumferential #40 blade hair clipping and aseptic chlorhexidine skin preparation, then flush gel out to prevent clipped hair and dermal debris from embedding in exposed bone/tendon."

	lavagePlan := "Copious 7–8 PSI warm isotonic Lactated Ringer's Solution (LRS) or 0.9% NaCl lavage (minimum 1,500–3,000 mL via 1 L bag pressurized to 300 mmHg + 18G needle or 35 mL syringe + 18G needle) with sharp staged debridement of devitalized necrotic tissue while preserving viable periosteum and neurovascular bundles."

	npwtPlan := "Apply Negative Pressure Wound Therapy (NPWT / Vacuum-Assisted Closure) with open-pore polyurethane foam at -125 mmHg continuous vacuum (or sterile tie-over / wet-to-dry hypertonic-to-isotonic debridement dressing changed q24h) to evacuate exudate, collapse dead space, and stimulate robust angiogenesis/granulation."
	if thoracicWallPenetrationSuspected {
		npwtPlan = "Exploratory thoracotomy / body-wall reconstruction with pericostal circumcostal sutures, pleural lavage, thoracostomy tube evacuation of pneumothorax, and closed-suction Jackson-Pratt drain or -75 to -125 mmHg NPWT over reconstructed muscular fascia."
	}

	abxPlan := fmt.Sprintf("Collect deep tissue/periosteal aerobic & anaerobic culture + susceptibility AFTER debridement/lavage (not superficial swab), and initiate immediate IV bactericidal coverage: Cefazolin 22 mg/kg IV q8h (%.1f mg for %.1f kg patient) + Enrofloxacin or Amoxicillin-Clavulanate + Metronidazole for polymicrobial/bite flora.", cefazolinMg, weightKg)

	splintPlan := "Modified Robert Jones bandage or lateral fiberglass/thermoplastic splint immobilizing the joint above and joint below the fracture/shearing zone, leaving distal digits 3 & 4 visible to monitor toe-web temperature delta, swelling, and strikethrough."

	esfPlan := "Transarticular External Skeletal Fixator (ESF — Type Ib / Type II linear or circular hybrid frame) to maintain joint stability while permitting daily open wound access, followed by delayed primary closure, mesh skin graft, or axial pattern flap once granulation covers exposed cortical bone."
	if !exposedCorticalBoneTendon && gustiloClass == domain.GustiloClassGradeIOpenLongBoneFracture {
		esfPlan = "Rigid internal locking plate-rod or External Skeletal Fixator (ESF) stabilization after copious 7–8 PSI lavage and negative deep-tissue contamination check."
	}

	return domain.DebridementFixationPlan{
		SterileWaterSolubleGelClipPlan:    gelClipPlan,
		CopiousLavage35mLSyringe18GPlan:   lavagePlan,
		NPWTVacuumOrTieOverDressingPlan:   npwtPlan,
		EmpiricAbxCultureBeforeLavage:     abxPlan,
		RobertJonesOrSpicaSplintPlan:      splintPlan,
		DefinitiveExternalSkeletalFixator: esfPlan,
	}
}

func (s *OrthoService) seed() {
	now := time.Now().UTC()

	// ORTHO-OP-101: Ranger — Canine, Border Collie, 19.5 kg, ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE
	assess101 := BuildGustiloWoundAssessmentPlan(
		domain.GustiloClassGradeIIIShearingDegloving,
		9, 1500, 0, 4.6, 1.8,
		true, false,
	)
	fix101 := BuildDefaultDebridementFixationPlan(
		19.5, domain.GustiloClassGradeIIIShearingDegloving,
		true, false,
	)
	op101 := &domain.PolytraumaOrthoOperation{
		ID:                               "ORTHO-OP-101",
		CaseID:                           "CASE-ORTHO-101",
		PetName:                          "Ranger",
		Species:                          "Canine",
		Breed:                            "Border Collie",
		WeightKg:                         19.5,
		Status:                           domain.OrthoStatusAcuteOpenFractureTriage,
		GustiloClass:                     domain.GustiloClassGradeIIIShearingDegloving,
		AnimalTraumaTriageScore:          9,
		LavageVolumeML:                   1500,
		NPWTVacuumMmHg:                   0,
		BloodLactateMmolL:                4.6,
		DistalToeTempDeltaC:              1.8,
		ExposedCorticalBoneTendon:        true,
		ThoracicWallPenetrationSuspected: false,
		AssessmentPlan:                   assess101,
		FixationPlan:                     fix101,
		WoundChecks: []domain.OrthopedicWoundCheck{
			{
				ID:                      "ORTHO-CHK-101-1",
				AnimalTraumaTriageScore: 10,
				LavageVolumeML:          500,
				NPWTVacuumMmHg:          0,
				BloodLactateMmolL:       5.2,
				DistalToeTempDeltaC:     2.1,
				RecordedBy:              "Dr. Caleb Thorne, DACVS (Orthopedics)",
				ClinicalNote:            "Vehicular hit-by-car (HBC) polytrauma: Gustilo Grade IIIb medial tarsocrural shearing/degloving injury with loss of medial malleolus and medial collateral ligament, exposed talus/calcaneus cortical bone, road-gravel contamination, ATT score 10, Lactate 5.2 mmol/L.",
				RecordedAt:              now.Add(-55 * time.Minute),
			},
			{
				ID:                      "ORTHO-CHK-101-2",
				AnimalTraumaTriageScore: 9,
				LavageVolumeML:          1500,
				NPWTVacuumMmHg:          0,
				BloodLactateMmolL:       4.6,
				DistalToeTempDeltaC:     1.8,
				RecordedBy:              "Dr. Caleb Thorne, DACVS (Orthopedics)",
				ClinicalNote:            "Wound packed with sterile K-Y water-soluble gel, wide clip completed, initial 1,500 mL 7–8 PSI warm LRS lavage via 1 L cuff bag (300 mmHg) + 18G needle performed; primary closure strictly contraindicated — preparing -125 mmHg NPWT and transarticular ESF.",
				RecordedAt:              now.Add(-20 * time.Minute),
			},
		},
		CreatedAt: now.Add(-65 * time.Minute),
		UpdatedAt: now.Add(-20 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// ORTHO-OP-102: Peanut — Canine, Chihuahua Mix, 4.2 kg, LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED
	assess102 := BuildGustiloWoundAssessmentPlan(
		domain.GustiloClassBDLDThoracicBiteFlail,
		7, 2000, -125, 2.3, 0.4,
		false, true,
	)
	fix102 := BuildDefaultDebridementFixationPlan(
		4.2, domain.GustiloClassBDLDThoracicBiteFlail,
		false, true,
	)
	op102 := &domain.PolytraumaOrthoOperation{
		ID:                               "ORTHO-OP-102",
		CaseID:                           "CASE-ORTHO-102",
		PetName:                          "Peanut",
		Species:                          "Canine",
		Breed:                            "Chihuahua Mix",
		WeightKg:                         4.2,
		Status:                           domain.OrthoStatusNPWTSplintStaged,
		GustiloClass:                     domain.GustiloClassBDLDThoracicBiteFlail,
		AnimalTraumaTriageScore:          7,
		LavageVolumeML:                   2000,
		NPWTVacuumMmHg:                   -125,
		BloodLactateMmolL:                2.3,
		DistalToeTempDeltaC:              0.4,
		ExposedCorticalBoneTendon:        false,
		ThoracicWallPenetrationSuspected: true,
		AssessmentPlan:                   assess102,
		FixationPlan:                     fix102,
		WoundChecks: []domain.OrthopedicWoundCheck{
			{
				ID:                      "ORTHO-CHK-102-1",
				AnimalTraumaTriageScore: 7,
				LavageVolumeML:          2000,
				NPWTVacuumMmHg:          -125,
				BloodLactateMmolL:       2.3,
				DistalToeTempDeltaC:     0.4,
				RecordedBy:              "Dr. Caleb Thorne, DACVS (Orthopedics)",
				ClinicalNote:            "Big-Dog-Little-Dog (BDLD) crushing thoracic bite wound ('Iceberg Effect'): surgical exploration revealed extensive subfascial avulsion and 6th–8th intercostal disruption; 2,000 mL warm LRS 7–8 PSI lavage, pericostal body-wall repair, deep culture, and -125 mmHg NPWT staged.",
				RecordedAt:              now.Add(-80 * time.Minute),
			},
		},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-80 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// ORTHO-OP-103: Toby — Feline, Domestic Shorthair, 5.1 kg, GRANULATION_FIXATION_STABLE_CLEARED
	assess103 := BuildGustiloWoundAssessmentPlan(
		domain.GustiloClassGradeIOpenLongBoneFracture,
		2, 2500, 0, 1.1, 0.2,
		false, false,
	)
	fix103 := BuildDefaultDebridementFixationPlan(
		5.1, domain.GustiloClassGradeIOpenLongBoneFracture,
		false, false,
	)
	dischargedAt := now.Add(-30 * time.Minute)
	op103 := &domain.PolytraumaOrthoOperation{
		ID:                               "ORTHO-OP-103",
		CaseID:                           "CASE-ORTHO-103",
		PetName:                          "Toby",
		Species:                          "Feline",
		Breed:                            "Domestic Shorthair",
		WeightKg:                         5.1,
		Status:                           domain.OrthoStatusGranulationFixationCleared,
		GustiloClass:                     domain.GustiloClassGradeIOpenLongBoneFracture,
		AnimalTraumaTriageScore:          2,
		LavageVolumeML:                   2500,
		NPWTVacuumMmHg:                   0,
		BloodLactateMmolL:                1.1,
		DistalToeTempDeltaC:              0.2,
		ExposedCorticalBoneTendon:        false,
		ThoracicWallPenetrationSuspected: false,
		AssessmentPlan:                   assess103,
		FixationPlan:                     fix103,
		WoundChecks: []domain.OrthopedicWoundCheck{
			{
				ID:                      "ORTHO-CHK-103-1",
				AnimalTraumaTriageScore: 5,
				LavageVolumeML:          1500,
				NPWTVacuumMmHg:          -125,
				BloodLactateMmolL:       2.1,
				DistalToeTempDeltaC:     0.6,
				RecordedBy:              "Dr. Caleb Thorne, DACVS (Orthopedics)",
				ClinicalNote:            "Gustilo Grade II open tibial diaphyseal fracture: sterile gel clip, 1,500 mL 7–8 PSI warm LRS lavage, temporary Robert Jones splint immobilizing stifle and tarsus, followed by Type Ib External Skeletal Fixator.",
				RecordedAt:              now.Add(-16 * time.Hour),
			},
			{
				ID:                      "ORTHO-CHK-103-2",
				AnimalTraumaTriageScore: 2,
				LavageVolumeML:          2500,
				NPWTVacuumMmHg:          0,
				BloodLactateMmolL:       1.1,
				DistalToeTempDeltaC:     0.2,
				RecordedBy:              "Dr. Caleb Thorne, DACVS (Orthopedics)",
				ClinicalNote:            "Healthy vascularized granulation bed complete, ESF pins clean and rigid, ATT score 2, Lactate 1.1 mmol/L, distal toe-web ΔT 0.2°C with brisk pedal perfusion.",
				RecordedAt:              now.Add(-45 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Caleb Thorne, DACVS (Orthopedics)",
		DischargeSummary: "Healthy granulation bed established over tibial fracture site, Type Ib External Skeletal Fixator stable with warm distal toes (ΔT 0.2°C, ATT score 2, Lactate 1.1 mmol/L); cleared for orthopedic step-down.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-20 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Polytrauma & Orthopedic operations, optionally filtered by statusFilter.
func (s *OrthoService) ListOperations(statusFilter string) []domain.PolytraumaOrthoOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.PolytraumaOrthoOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneOrthoOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Polytrauma & Orthopedic operation by ID.
func (s *OrthoService) GetOperation(id string) (*domain.PolytraumaOrthoOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("polytrauma orthopedic operation %q not found", id)
	}
	cp := cloneOrthoOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all Polytrauma & Orthopedic operations.
func (s *OrthoService) GetOverviewMetrics() domain.PolytraumaOrthoOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.PolytraumaOrthoOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.OrthoStatusAcuteOpenFractureTriage:
			m.AcuteOpenFractureTriageCount++
		case domain.OrthoStatusNPWTSplintStaged:
			m.NPWTSplintStagedCount++
		case domain.OrthoStatusGranulationFixationCleared:
			m.ClearedFixationCount++
		}
		if op.GustiloClass == domain.GustiloClassGradeIIIShearingDegloving {
			m.GustiloGradeIIIShearingCount++
		}
		if op.ThoracicWallPenetrationSuspected || op.GustiloClass == domain.GustiloClassBDLDThoracicBiteFlail {
			m.ThoracicBitePenetrationCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Bite Wound, Polytrauma, Open Fracture & NPWT triage operation.
func (s *OrthoService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	gustiloClass domain.GustiloInjuryClassification,
	attScore, lavageVolumeML, npwtVacuumMmHg int,
	bloodLactateMmolL, distalToeTempDeltaC float64,
	exposedCorticalBoneTendon, thoracicWallPenetrationSuspected bool,
) (*domain.PolytraumaOrthoOperation, error) {
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
	if !gustiloClass.IsValid() {
		return nil, fmt.Errorf("invalid gustilo_class: %q", gustiloClass)
	}
	if attScore < 0 || attScore > 18 {
		return nil, fmt.Errorf("animal_trauma_triage_score must be in [0, 18], got %d", attScore)
	}
	if lavageVolumeML < 0 || lavageVolumeML > 20000 {
		return nil, fmt.Errorf("lavage_volume_ml must be in [0, 20000], got %d", lavageVolumeML)
	}
	if npwtVacuumMmHg < -250 || npwtVacuumMmHg > 0 {
		return nil, fmt.Errorf("npwt_vacuum_mmhg must be in [-250, 0], got %d", npwtVacuumMmHg)
	}
	if bloodLactateMmolL <= 0 || bloodLactateMmolL > 25.0 {
		return nil, fmt.Errorf("blood_lactate_mmol_l must be in (0, 25.0], got %v", bloodLactateMmolL)
	}
	if distalToeTempDeltaC < 0 || distalToeTempDeltaC > 15.0 {
		return nil, fmt.Errorf("distal_toe_temp_delta_c must be in [0, 15.0], got %v", distalToeTempDeltaC)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("ORTHO-OP-%d", s.seq)
	now := time.Now().UTC()

	assessPlan := BuildGustiloWoundAssessmentPlan(
		gustiloClass,
		attScore, lavageVolumeML, npwtVacuumMmHg,
		bloodLactateMmolL, distalToeTempDeltaC,
		exposedCorticalBoneTendon, thoracicWallPenetrationSuspected,
	)
	fixPlan := BuildDefaultDebridementFixationPlan(
		weightKg, gustiloClass, exposedCorticalBoneTendon, thoracicWallPenetrationSuspected,
	)

	op := &domain.PolytraumaOrthoOperation{
		ID:                               opID,
		CaseID:                           trimmedCase,
		PetName:                          trimmedPet,
		Species:                          trimmedSpecies,
		Breed:                            trimmedBreed,
		WeightKg:                         round1(weightKg),
		Status:                           domain.OrthoStatusAcuteOpenFractureTriage,
		GustiloClass:                     gustiloClass,
		AnimalTraumaTriageScore:          attScore,
		LavageVolumeML:                   lavageVolumeML,
		NPWTVacuumMmHg:                   npwtVacuumMmHg,
		BloodLactateMmolL:                round1(bloodLactateMmolL),
		DistalToeTempDeltaC:              round1(distalToeTempDeltaC),
		ExposedCorticalBoneTendon:        exposedCorticalBoneTendon,
		ThoracicWallPenetrationSuspected: thoracicWallPenetrationSuspected,
		AssessmentPlan:                   assessPlan,
		FixationPlan:                     fixPlan,
		WoundChecks:                      []domain.OrthopedicWoundCheck{},
		CreatedAt:                        now,
		UpdatedAt:                        now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneOrthoOperation(op)
	return &cp, nil
}

// StageDebridementFixation stages sterile water-soluble gel clip protection, copious 7–8 PSI warm isotonic lavage,
// NPWT (-125 mmHg) or sterile tie-over dressing, post-lavage culture & empiric antimicrobials,
// Robert Jones / Spica splint immobilization (joint above & joint below), and External Skeletal Fixator (ESF),
// transitioning status to LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED.
func (s *OrthoService) StageDebridementFixation(
	id, gelClipPlan, lavagePlan, npwtOrTieOverPlan, abxCulturePlan, splintPlan, esfFixationPlan string,
) (*domain.PolytraumaOrthoOperation, error) {
	trimmedGel := strings.TrimSpace(gelClipPlan)
	if trimmedGel == "" {
		return nil, fmt.Errorf("sterile_water_soluble_gel_clip_plan is required")
	}
	trimmedLavage := strings.TrimSpace(lavagePlan)
	if trimmedLavage == "" {
		return nil, fmt.Errorf("copious_lavage_35_ml_syringe_18g_plan is required")
	}
	trimmedNPWT := strings.TrimSpace(npwtOrTieOverPlan)
	if trimmedNPWT == "" {
		return nil, fmt.Errorf("npwt_vacuum_or_tie_over_dressing_plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("polytrauma orthopedic operation %q not found", id)
	}

	defaults := BuildDefaultDebridementFixationPlan(
		op.WeightKg, op.GustiloClass, op.ExposedCorticalBoneTendon, op.ThoracicWallPenetrationSuspected,
	)
	trimmedAbx := strings.TrimSpace(abxCulturePlan)
	if trimmedAbx == "" {
		trimmedAbx = defaults.EmpiricAbxCultureBeforeLavage
	}
	trimmedSplint := strings.TrimSpace(splintPlan)
	if trimmedSplint == "" {
		trimmedSplint = defaults.RobertJonesOrSpicaSplintPlan
	}
	trimmedESF := strings.TrimSpace(esfFixationPlan)
	if trimmedESF == "" {
		trimmedESF = defaults.DefinitiveExternalSkeletalFixator
	}

	op.FixationPlan = domain.DebridementFixationPlan{
		SterileWaterSolubleGelClipPlan:    trimmedGel,
		CopiousLavage35mLSyringe18GPlan:   trimmedLavage,
		NPWTVacuumOrTieOverDressingPlan:   trimmedNPWT,
		EmpiricAbxCultureBeforeLavage:     trimmedAbx,
		RobertJonesOrSpicaSplintPlan:      trimmedSplint,
		DefinitiveExternalSkeletalFixator: trimmedESF,
	}
	op.Status = domain.OrthoStatusNPWTSplintStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneOrthoOperation(op)
	return &cp, nil
}

// RecordWoundCheck appends an OrthopedicWoundCheck and updates op.AnimalTraumaTriageScore,
// op.LavageVolumeML, op.NPWTVacuumMmHg, op.BloodLactateMmolL, and op.DistalToeTempDeltaC.
func (s *OrthoService) RecordWoundCheck(
	id string,
	attScore, lavageVolumeML, npwtVacuumMmHg int,
	bloodLactateMmolL, distalToeTempDeltaC float64,
	recordedBy, clinicalNote string,
) (*domain.PolytraumaOrthoOperation, error) {
	if attScore < 0 || attScore > 18 {
		return nil, fmt.Errorf("animal_trauma_triage_score must be in [0, 18], got %d", attScore)
	}
	if lavageVolumeML < 0 || lavageVolumeML > 20000 {
		return nil, fmt.Errorf("lavage_volume_ml must be in [0, 20000], got %d", lavageVolumeML)
	}
	if npwtVacuumMmHg < -250 || npwtVacuumMmHg > 0 {
		return nil, fmt.Errorf("npwt_vacuum_mmhg must be in [-250, 0], got %d", npwtVacuumMmHg)
	}
	if bloodLactateMmolL <= 0 || bloodLactateMmolL > 25.0 {
		return nil, fmt.Errorf("blood_lactate_mmol_l must be in (0, 25.0], got %v", bloodLactateMmolL)
	}
	if distalToeTempDeltaC < 0 || distalToeTempDeltaC > 15.0 {
		return nil, fmt.Errorf("distal_toe_temp_delta_c must be in [0, 15.0], got %v", distalToeTempDeltaC)
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
		return nil, fmt.Errorf("polytrauma orthopedic operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.OrthopedicWoundCheck{
		ID:                      fmt.Sprintf("ORTHO-CHK-%s-%d", op.ID, len(op.WoundChecks)+1),
		AnimalTraumaTriageScore: attScore,
		LavageVolumeML:          lavageVolumeML,
		NPWTVacuumMmHg:          npwtVacuumMmHg,
		BloodLactateMmolL:       round1(bloodLactateMmolL),
		DistalToeTempDeltaC:     round1(distalToeTempDeltaC),
		RecordedBy:              trimmedBy,
		ClinicalNote:            trimmedNote,
		RecordedAt:              now,
	}

	op.WoundChecks = append(op.WoundChecks, chk)
	op.AnimalTraumaTriageScore = chk.AnimalTraumaTriageScore
	op.LavageVolumeML = chk.LavageVolumeML
	op.NPWTVacuumMmHg = chk.NPWTVacuumMmHg
	op.BloodLactateMmolL = chk.BloodLactateMmolL
	op.DistalToeTempDeltaC = chk.DistalToeTempDeltaC

	op.AssessmentPlan = BuildGustiloWoundAssessmentPlan(
		op.GustiloClass,
		op.AnimalTraumaTriageScore, op.LavageVolumeML, op.NPWTVacuumMmHg,
		op.BloodLactateMmolL, op.DistalToeTempDeltaC,
		op.ExposedCorticalBoneTendon, op.ThoracicWallPenetrationSuspected,
	)
	op.UpdatedAt = now

	cp := cloneOrthoOperation(op)
	return &cp, nil
}

// ClearOrthoDischarge transitions the operation to GRANULATION_FIXATION_STABLE_CLEARED
// once healthy granulation tissue, orthopedic fixation stability, and distal limb perfusion are verified.
func (s *OrthoService) ClearOrthoDischarge(id, dischargedBy, summary string) (*domain.PolytraumaOrthoOperation, error) {
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
		return nil, fmt.Errorf("polytrauma orthopedic operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.OrthoStatusGranulationFixationCleared
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneOrthoOperation(op)
	return &cp, nil
}

func cloneOrthoOperation(op *domain.PolytraumaOrthoOperation) domain.PolytraumaOrthoOperation {
	cp := *op
	if op.WoundChecks != nil {
		cp.WoundChecks = make([]domain.OrthopedicWoundCheck, len(op.WoundChecks))
		copy(cp.WoundChecks, op.WoundChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
