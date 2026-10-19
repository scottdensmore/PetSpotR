package megaesophagusfb

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// MegaesophagusService manages Acute Esophageal Foreign Body, Megaesophagus,
// Myasthenia Gravis (AChR Antibody / Edrophonium-Neostigmine), Aspiration Pneumonia &
// Pyridostigmine/Bailey Chair triage operations in a thread-safe store.
type MegaesophagusService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.MegaesophagusOperation
	order []string
	seq   int
}

// NewMegaesophagusService initializes a MegaesophagusService with 3 seeded operations
// (MEGA-OP-101, MEGA-OP-102, MEGA-OP-103).
func NewMegaesophagusService() *MegaesophagusService {
	s := &MegaesophagusService{
		ops:   make(map[string]*domain.MegaesophagusOperation),
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

// BuildRegurgitationAspirationPlan computes the clinical interpretation for a Megaesophagus /
// Esophageal Foreign Body operation.
func BuildRegurgitationAspirationPlan(
	pathology domain.EsophagealPathologyType,
	spO2Percent, aaGradientMmHg float64,
	regurgEpisodes24h, esophagitisGrade int,
	achrTiterNmolL float64,
	pneumoniaPresent bool,
) domain.RegurgitationAspirationPlan {
	var regurgNote string
	switch pathology {
	case domain.PathologyAcuteBoneFishhookForeignBody:
		regurgNote = fmt.Sprintf("Acute Esophageal Bone / Fishhook Foreign Body Obstruction (%d regurgitation episodes/24h, Grade %d mucosal esophagitis) — Passive retrograde expulsion of undigested food/tubular mucus and hypersalivation/odynophagia WITHOUT prodromal nausea, retching, abdominal contractions, or bile (distinguishing regurgitation from active emesis).", regurgEpisodes24h, esophagitisGrade)
	case domain.PathologyAcquiredMyastheniaGravis:
		regurgNote = fmt.Sprintf("Acquired Myasthenia Gravis Focal/Generalized Megaesophagus (%d passive regurgitation episodes/24h, Grade %d esophagitis) — Immune-mediated nicotinic Acetylcholine Receptor (AChR) destruction in canine striated esophageal musculature causing aperistalsis and effortless post-prandial regurgitation.", regurgEpisodes24h, esophagitisGrade)
	case domain.PathologyCongenitalPRAAIdiopathic:
		regurgNote = fmt.Sprintf("Congenital Vascular Ring Anomaly (Persistent Right Aortic Arch / PRAA) or Idiopathic Megaesophagus (%d regurgitation episodes/24h, Grade %d esophagitis) — Generalized esophageal dilation cranial to the heart base or lower esophageal sphincter (LES) achalasia.", regurgEpisodes24h, esophagitisGrade)
	default:
		regurgNote = fmt.Sprintf("Esophageal Dysmotility & Obstruction (%d passive regurgitation episodes/24h, Grade %d esophagitis) — Differentiate passive gravity-dependent regurgitation from active bile-stained vomiting.", regurgEpisodes24h, esophagitisGrade)
	}

	pneumoniaSummary := "No active alveolar cranioventral aspiration consolidate"
	if pneumoniaPresent {
		pneumoniaSummary = "ACTIVE CRANIOVENTRAL / RIGHT MIDDLE LUNG LOBE ASPIRATION PNEUMONIA PRESENT (alveolar air bronchograms & B-lines on thoracic POCUS)"
	}
	radiographNote := fmt.Sprintf("3-View Thoracic Radiographs (Right/Left Lateral + VD/DV) & Thoracic POCUS: SpO2 %.1f%% | Alveolar-Arterial (A-a) O2 Gradient %.1f mmHg (> 15 mmHg indicates V/Q mismatch) — %s; check predilection sites (thoracic inlet, base of heart, cranial to diaphragm).", spO2Percent, aaGradientMmHg, pneumoniaSummary)

	perforationAlert := fmt.Sprintf("Esophageal Perforation & Mediastinitis Surveillance (Current Mucosal Esophagitis Grade %d/4): Monitor for cervical subcutaneous emphysema, pneumomediastinum, pneumothorax, or septic pleural effusion (pyothorax) secondary to pressure necrosis or spiculated bone/fishhook penetration.", esophagitisGrade)

	achrInterpretation := "Seronegative / Normal (< 0.6 nmol/L)"
	if achrTiterNmolL > 0.6 {
		achrInterpretation = "SEROPOSITIVE DIAGNOSTIC FOR ACQUIRED MYASTHENIA GRAVIS (> 0.6 nmol/L gold standard)"
	}
	achrNote := fmt.Sprintf("Acetylcholine Receptor (AChR) Antibody Titer: %.2f nmol/L (%s) — Screen concurrently for Secondary Megaesophagus triggers: Hypoadrenocorticism (Addison's baseline cortisol / ACTH-stimulation test), Hypothyroidism (free T4/TSH), Lupus myositis, Thymoma, and Lead/Thallium/Organophosphate neurotoxicity.", achrTiterNmolL, achrInterpretation)

	bariumGuard := "CONTRAINDICATION GUARD: (1) NEVER perform blind forceful orogastric tube advancement of sharp spiculated bone or fishhook foreign bodies (high risk of full-thickness thoracic esophageal laceration, aortic/azygos erosion, and fatal pyothorax); (2) AVOID liquid Barium Sulfate contrast swallows in patients with suspected esophageal perforation (causes granulomatous mediastinitis) or severe regurgitation/aspiration risk (use non-ionic iodinated Iohexol contrast if contrast study is mandatory)."

	crisisNote := "Myasthenic Crisis vs. Cholinergic Crisis (SLUDGE) Guard: Edrophonium Chloride (0.1–0.2 mg/kg IV) or Neostigmine challenge transiently restores neuromuscular transmission in Myasthenia Gravis; however, excess Pyridostigmine/Neostigmine triggers muscarinic/nicotinic Cholinergic Crisis (Salivation, Lacrimation, Urination, Defecation, GI cramping, Emesis, Bradycardia, fasciculations, flaccid paralysis) — keep Atropine Sulfate (0.02–0.04 mg/kg IV/IM) immediately at bedside."

	return domain.RegurgitationAspirationPlan{
		RegurgitationVsVomitingNote:       regurgNote,
		ThoracicRadiographPOCUSFinding:    radiographNote,
		PerforationPneumomediastinumAlert: perforationAlert,
		AChRAntibodyAddisonScreenNote:     achrNote,
		BariumSwallowContraindication:     bariumGuard,
		CholinergicVsMyasthenicCrisisNote: crisisNote,
	}
}

// BuildDefaultEndoscopyBaileyChairPlan returns an evidence-based EndoscopyBaileyChairPlan.
func BuildDefaultEndoscopyBaileyChairPlan(
	weightKg float64,
	pathology domain.EsophagealPathologyType,
	achrTiterNmolL float64,
	pneumoniaPresent bool,
) domain.EndoscopyBaileyChairPlan {
	sucralfateDoseG := 0.5
	if weightKg >= 15.0 {
		sucralfateDoseG = 1.0
	}
	omeprazoleDoseMg := round1(weightKg * 1.0)
	pyridoLowMg := round1(weightKg * 1.0)
	pyridoHighMg := round1(weightKg * 3.0)
	sildenafilDoseMg := round1(weightKg * 1.0)
	ampicillinSulbactamMg := round1(weightKg * 30.0)
	enrofloxacinMg := round1(weightKg * 10.0)

	retrievalPlan := "Flexible Esophagoscopy under General Anesthesia with Cuffed Endotracheal Tube (preventing periprocedural aspiration): Insufflate lumen gently to disengage mucosa, grasp foreign body with rat-tooth/alligator forceps or snare, or advance smooth distally-located bone gently into stomach for gastrotomy/digestion; NEVER blindly force sharp spiculated bones or fishhooks."
	if pathology != domain.PathologyAcuteBoneFishhookForeignBody {
		retrievalPlan = "Diagnostic Survey & Esophageal Lumen Clearance: Confirm absence of mechanical intraluminal foreign body or stricture; suction pooled esophageal fluid/saliva with cuffed endotracheal tube protection during any sedated imaging."
	}

	sucralfatePlan := fmt.Sprintf("Mucosal Cytoprotection & Reflux Esophagitis Control: Sucralfate oral suspension slurry %.1f g PO q8h (coat ulcerated mucosa 1h separated from other oral meds) + Omeprazole 1 mg/kg PO/IV q12h (%.1f mg q12h) + Maropitant 1 mg/kg IV q24h.", sucralfateDoseG, omeprazoleDoseMg)

	pyridoPlan := fmt.Sprintf("Pyridostigmine Bromide (Mestinon) Anticholinesterase Protocol: 1–3 mg/kg PO q8–12h (%.1f–%.1f mg PO q8–12h for %.1f kg patient) indicated when AChR antibody titer > 0.6 nmol/L (current %.2f nmol/L) or positive Edrophonium/Neostigmine test; titrate to minimize regurgitation while monitoring for SLUDGE cholinergic overdose (Atropine 0.02–0.04 mg/kg IV ready).", pyridoLowMg, pyridoHighMg, weightKg, achrTiterNmolL)

	abxPlan := fmt.Sprintf("Aspiration Pneumonia Antimicrobial & Airway Clearance Protocol: Ampicillin-Sulbactam 30 mg/kg IV q8h (%.0f mg IV) + Enrofloxacin 10 mg/kg IV q24h (%.1f mg IV) for severe gram-negative/anaerobic aspiration pneumonia, isotonic 0.9%% NaCl nebulization q6–8h + gentle coupage (withhold coupage immediately post-feeding), and supplemental oxygen to maintain SpO2 >= 96%%.", ampicillinSulbactamMg, enrofloxacinMg)
	if !pneumoniaPresent {
		abxPlan = "Aspiration Prophylaxis & Airway Surveillance: No empirical antibiotics required in uncomplicated megaesophagus without radiographic/clinical aspiration pneumonia; monitor respiratory rate, temperature, SpO2, and thoracic auscultation q4h."
	}

	baileyProtocol := "Upright Bailey Chair Vertical Feeding Protocol: Feed high-calorie meatballs or Knox gelatin slurry in 90-degree vertical posture and maintain strict upright elevation in Bailey Chair for 20–30 minutes post-prandially (and after water/medication administration) to utilize gravity for esophageal transit."

	sildenafilNote := fmt.Sprintf("Lower Esophageal Sphincter (LES) Relaxation & Achalasia Therapy: Sildenafil citrate 1 mg/kg PO q8–12h (%.1f mg PO q8–12h) to reduce lower esophageal sphincter tone via nitric oxide–cGMP smooth muscle relaxation and decrease regurgitation frequency.", sildenafilDoseMg)

	return domain.EndoscopyBaileyChairPlan{
		EndoscopicRetrievalOrGastricPush:  retrievalPlan,
		PostRemovalSucralfateOmeprazole:   sucralfatePlan,
		PyridostigmineAnticholinesterase:  pyridoPlan,
		AspirationPneumoniaAntibioticPlan: abxPlan,
		UprightBaileyChairFeedingProtocol: baileyProtocol,
		SildenafilLESRelaxationNote:       sildenafilNote,
	}
}

func (s *MegaesophagusService) seed() {
	now := time.Now().UTC()

	// MEGA-OP-101: Waffles — Canine, West Highland White Terrier, 8.6 kg, ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE
	asp101 := BuildRegurgitationAspirationPlan(
		domain.PathologyAcuteBoneFishhookForeignBody,
		91.0, 34.0, 6, 3, 0.12, true,
	)
	bailey101 := BuildDefaultEndoscopyBaileyChairPlan(
		8.6, domain.PathologyAcuteBoneFishhookForeignBody, 0.12, true,
	)
	op101 := &domain.MegaesophagusOperation{
		ID:                           "MEGA-OP-101",
		CaseID:                       "CASE-MEGA-101",
		PetName:                      "Waffles",
		Species:                      "Canine",
		Breed:                        "West Highland White Terrier",
		WeightKg:                     8.6,
		Status:                       domain.MegaStatusAcuteObstructionAspirationTriage,
		PathologyType:                domain.PathologyAcuteBoneFishhookForeignBody,
		SpO2Percent:                  91.0,
		AlveolarArterialGradientMmHg: 34.0,
		RegurgitationEpisodes24h:     6,
		UprightHoldMinutes:           0,
		MucosalEsophagitisGrade:      3,
		AChRAntibodyTiterNmolL:       0.12,
		AspirationPneumoniaPresent:   true,
		AspirationPlan:               asp101,
		BaileyPlan:                   bailey101,
		EsophagealChecks: []domain.EsophagealAspirationCheck{
			{
				ID:                           "MEGA-CHK-101-1",
				SpO2Percent:                  89.5,
				AlveolarArterialGradientMmHg: 38.0,
				RegurgitationEpisodes24h:     6,
				UprightHoldMinutes:           0,
				MucosalEsophagitisGrade:      3,
				RecordedBy:                   "Dr. Clara Sterling, DACVIM (SAIM)",
				ClinicalNote:                 "Presentation triage: acute onset ptyalism, retching-free passive regurgitation of undigested food/foam (6 episodes), wedged pork rib bone at heart base on 3-view thoracic radiographs with right middle lung lobe aspiration pneumonia (SpO2 89.5% on room air).",
				RecordedAt:                   now.Add(-55 * time.Minute),
			},
			{
				ID:                           "MEGA-CHK-101-2",
				SpO2Percent:                  91.0,
				AlveolarArterialGradientMmHg: 34.0,
				RegurgitationEpisodes24h:     6,
				UprightHoldMinutes:           0,
				MucosalEsophagitisGrade:      3,
				RecordedBy:                   "Dr. Clara Sterling, DACVIM (SAIM)",
				ClinicalNote:                 "Oxygen cage 40% FiO2 initiated (SpO2 91.0%, A-a gradient 34.0 mmHg); liquid barium strictly contraindicated; preparing cuffed-tube flexible esophagoscopy for foreign body retrieval and circumferential Grade 3 mucosal esophagitis inspection.",
				RecordedAt:                   now.Add(-20 * time.Minute),
			},
		},
		CreatedAt: now.Add(-65 * time.Minute),
		UpdatedAt: now.Add(-20 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// MEGA-OP-102: Freya — Canine, German Shepherd Dog, 29.4 kg, ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED
	asp102 := BuildRegurgitationAspirationPlan(
		domain.PathologyAcquiredMyastheniaGravis,
		95.0, 18.0, 2, 1, 2.85, true,
	)
	bailey102 := BuildDefaultEndoscopyBaileyChairPlan(
		29.4, domain.PathologyAcquiredMyastheniaGravis, 2.85, true,
	)
	op102 := &domain.MegaesophagusOperation{
		ID:                           "MEGA-OP-102",
		CaseID:                       "CASE-MEGA-102",
		PetName:                      "Freya",
		Species:                      "Canine",
		Breed:                        "German Shepherd Dog",
		WeightKg:                     29.4,
		Status:                       domain.MegaStatusEndoscopicPyridostigmineStaged,
		PathologyType:                domain.PathologyAcquiredMyastheniaGravis,
		SpO2Percent:                  95.0,
		AlveolarArterialGradientMmHg: 18.0,
		RegurgitationEpisodes24h:     2,
		UprightHoldMinutes:           25,
		MucosalEsophagitisGrade:      1,
		AChRAntibodyTiterNmolL:       2.85,
		AspirationPneumoniaPresent:   true,
		AspirationPlan:               asp102,
		BaileyPlan:                   bailey102,
		EsophagealChecks: []domain.EsophagealAspirationCheck{
			{
				ID:                           "MEGA-CHK-102-1",
				SpO2Percent:                  95.0,
				AlveolarArterialGradientMmHg: 18.0,
				RegurgitationEpisodes24h:     2,
				UprightHoldMinutes:           25,
				MucosalEsophagitisGrade:      1,
				RecordedBy:                   "Dr. Marcus Thorne, DACVECC",
				ClinicalNote:                 "Seropositive Acquired Myasthenia Gravis (AChR antibody titer 2.85 nmol/L > 0.6 nmol/L threshold): staged on Pyridostigmine Bromide 1.5 mg/kg PO q8h, Sildenafil 1 mg/kg PO q8h, Ampicillin-Sulbactam/Enrofloxacin, and 25-min Upright Bailey Chair meatball feeding.",
				RecordedAt:                   now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-3 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// MEGA-OP-103: Pippin — Canine, Miniature Schnauzer, 6.8 kg, UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED
	asp103 := BuildRegurgitationAspirationPlan(
		domain.PathologyCongenitalPRAAIdiopathic,
		99.0, 8.0, 0, 0, 0.08, false,
	)
	bailey103 := BuildDefaultEndoscopyBaileyChairPlan(
		6.8, domain.PathologyCongenitalPRAAIdiopathic, 0.08, false,
	)
	dischargedAt := now.Add(-30 * time.Minute)
	op103 := &domain.MegaesophagusOperation{
		ID:                           "MEGA-OP-103",
		CaseID:                       "CASE-MEGA-103",
		PetName:                      "Pippin",
		Species:                      "Canine",
		Breed:                        "Miniature Schnauzer",
		WeightKg:                     6.8,
		Status:                       domain.MegaStatusUprightBaileyChairCleared,
		PathologyType:                domain.PathologyCongenitalPRAAIdiopathic,
		SpO2Percent:                  99.0,
		AlveolarArterialGradientMmHg: 8.0,
		RegurgitationEpisodes24h:     0,
		UprightHoldMinutes:           30,
		MucosalEsophagitisGrade:      0,
		AChRAntibodyTiterNmolL:       0.08,
		AspirationPneumoniaPresent:   false,
		AspirationPlan:               asp103,
		BaileyPlan:                   bailey103,
		EsophagealChecks: []domain.EsophagealAspirationCheck{
			{
				ID:                           "MEGA-CHK-103-1",
				SpO2Percent:                  96.5,
				AlveolarArterialGradientMmHg: 14.0,
				RegurgitationEpisodes24h:     1,
				UprightHoldMinutes:           25,
				MucosalEsophagitisGrade:      1,
				RecordedBy:                   "Dr. Clara Sterling, DACVIM (SAIM)",
				ClinicalNote:                 "Idiopathic megaesophagus stabilized on Sildenafil 1 mg/kg PO q8h and Knox gelatin + meatball slurry in Upright Bailey Chair for 25 minutes post-feeding.",
				RecordedAt:                   now.Add(-18 * time.Hour),
			},
			{
				ID:                           "MEGA-CHK-103-2",
				SpO2Percent:                  99.0,
				AlveolarArterialGradientMmHg: 8.0,
				RegurgitationEpisodes24h:     0,
				UprightHoldMinutes:           30,
				MucosalEsophagitisGrade:      0,
				RecordedBy:                   "Dr. Clara Sterling, DACVIM (SAIM)",
				ClinicalNote:                 "Zero passive regurgitation episodes over 24h with 30-min Upright Bailey Chair hold, SpO2 99.0% on room air, A-a gradient 8.0 mmHg, clear thoracic radiographs.",
				RecordedAt:                   now.Add(-45 * time.Minute),
			},
		},
		DischargedBy:     "Dr. Clara Sterling, DACVIM (SAIM)",
		DischargeSummary: "Zero passive regurgitation episodes over 24h on 30-minute Upright Bailey Chair vertical feeding protocol and Sildenafil 1 mg/kg PO q8h; normothermic, SpO2 99.0%, A-a gradient 8.0 mmHg, cleared for home Bailey Chair management.",
		DischargedAt:     &dischargedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        dischargedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Megaesophagus operations, optionally filtered by statusFilter.
func (s *MegaesophagusService) ListOperations(statusFilter string) []domain.MegaesophagusOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.MegaesophagusOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneMegaesophagusOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Megaesophagus operation by ID.
func (s *MegaesophagusService) GetOperation(id string) (*domain.MegaesophagusOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("megaesophagus operation %q not found", id)
	}
	cp := cloneMegaesophagusOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes summary counts across all Megaesophagus & Foreign Body operations.
func (s *MegaesophagusService) GetOverviewMetrics() domain.MegaesophagusOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.MegaesophagusOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.MegaStatusAcuteObstructionAspirationTriage:
			m.AcuteObstructionTriageCount++
		case domain.MegaStatusEndoscopicPyridostigmineStaged:
			m.EndoscopyPyridostigmineStagedCount++
		case domain.MegaStatusUprightBaileyChairCleared:
			m.ClearedBaileyChairCount++
		}
		if op.PathologyType == domain.PathologyAcuteBoneFishhookForeignBody {
			m.ForeignBodyObstructionCount++
		}
		if op.AspirationPneumoniaPresent {
			m.AspirationPneumoniaActiveCount++
		}
	}
	return m
}

// CreateOperation validates and creates a new Acute Esophageal Foreign Body / Megaesophagus triage operation.
func (s *MegaesophagusService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	pathologyType domain.EsophagealPathologyType,
	spO2Percent, aaGradientMmHg float64,
	regurgEpisodes24h, uprightHoldMinutes, esophagitisGrade int,
	achrTiterNmolL float64,
	aspirationPneumoniaPresent bool,
) (*domain.MegaesophagusOperation, error) {
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
	if !pathologyType.IsValid() {
		return nil, fmt.Errorf("invalid pathology_type: %q", pathologyType)
	}
	if spO2Percent < 50.0 || spO2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", spO2Percent)
	}
	if aaGradientMmHg < 0.0 || aaGradientMmHg > 80.0 {
		return nil, fmt.Errorf("alveolar_arterial_gradient_mmhg must be in [0.0, 80.0], got %v", aaGradientMmHg)
	}
	if regurgEpisodes24h < 0 || regurgEpisodes24h > 50 {
		return nil, fmt.Errorf("regurgitation_episodes_24h must be in [0, 50], got %d", regurgEpisodes24h)
	}
	if uprightHoldMinutes < 0 || uprightHoldMinutes > 120 {
		return nil, fmt.Errorf("upright_hold_minutes must be in [0, 120], got %d", uprightHoldMinutes)
	}
	if esophagitisGrade < 0 || esophagitisGrade > 4 {
		return nil, fmt.Errorf("mucosal_esophagitis_grade must be in [0, 4], got %d", esophagitisGrade)
	}
	if achrTiterNmolL < 0.0 || achrTiterNmolL > 50.0 {
		return nil, fmt.Errorf("achr_antibody_titer_nmol_l must be in [0.0, 50.0], got %v", achrTiterNmolL)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("MEGA-OP-%d", s.seq)
	now := time.Now().UTC()

	aspPlan := BuildRegurgitationAspirationPlan(
		pathologyType,
		spO2Percent, aaGradientMmHg,
		regurgEpisodes24h, esophagitisGrade,
		achrTiterNmolL, aspirationPneumoniaPresent,
	)
	baileyPlan := BuildDefaultEndoscopyBaileyChairPlan(
		weightKg, pathologyType, achrTiterNmolL, aspirationPneumoniaPresent,
	)

	op := &domain.MegaesophagusOperation{
		ID:                           opID,
		CaseID:                       trimmedCase,
		PetName:                      trimmedPet,
		Species:                      trimmedSpecies,
		Breed:                        trimmedBreed,
		WeightKg:                     round1(weightKg),
		Status:                       domain.MegaStatusAcuteObstructionAspirationTriage,
		PathologyType:                pathologyType,
		SpO2Percent:                  round1(spO2Percent),
		AlveolarArterialGradientMmHg: round1(aaGradientMmHg),
		RegurgitationEpisodes24h:     regurgEpisodes24h,
		UprightHoldMinutes:           uprightHoldMinutes,
		MucosalEsophagitisGrade:      esophagitisGrade,
		AChRAntibodyTiterNmolL:       round2(achrTiterNmolL),
		AspirationPneumoniaPresent:   aspirationPneumoniaPresent,
		AspirationPlan:               aspPlan,
		BaileyPlan:                   baileyPlan,
		EsophagealChecks:             []domain.EsophagealAspirationCheck{},
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneMegaesophagusOperation(op)
	return &cp, nil
}

// StageEndoscopyBaileyPlan stages the endoscopic foreign body retrieval / gentle gastric push plan,
// post-removal Sucralfate slurry & Omeprazole cytoprotection, Pyridostigmine Bromide (1–3 mg/kg PO q8–12h)
// anticholinesterase plan, aspiration pneumonia antibiotic plan, Upright Bailey Chair feeding protocol,
// and Sildenafil LES relaxation note, transitioning status to ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED.
func (s *MegaesophagusService) StageEndoscopyBaileyPlan(
	id, retrievalPlan, sucralfateOmeprazole, pyridostigminePlan, pneumoniaAbxPlan, baileyChairProtocol, sildenafilNote string,
) (*domain.MegaesophagusOperation, error) {
	trimmedRetrieval := strings.TrimSpace(retrievalPlan)
	if trimmedRetrieval == "" {
		return nil, fmt.Errorf("endoscopic_retrieval_or_gastric_push is required")
	}
	trimmedSucralfate := strings.TrimSpace(sucralfateOmeprazole)
	if trimmedSucralfate == "" {
		return nil, fmt.Errorf("post_removal_sucralfate_omeprazole is required")
	}
	trimmedBailey := strings.TrimSpace(baileyChairProtocol)
	if trimmedBailey == "" {
		return nil, fmt.Errorf("upright_bailey_chair_feeding_protocol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("megaesophagus operation %q not found", id)
	}

	defaults := BuildDefaultEndoscopyBaileyChairPlan(
		op.WeightKg, op.PathologyType, op.AChRAntibodyTiterNmolL, op.AspirationPneumoniaPresent,
	)
	trimmedPyrido := strings.TrimSpace(pyridostigminePlan)
	if trimmedPyrido == "" {
		trimmedPyrido = defaults.PyridostigmineAnticholinesterase
	}
	trimmedAbx := strings.TrimSpace(pneumoniaAbxPlan)
	if trimmedAbx == "" {
		trimmedAbx = defaults.AspirationPneumoniaAntibioticPlan
	}
	trimmedSildenafil := strings.TrimSpace(sildenafilNote)
	if trimmedSildenafil == "" {
		trimmedSildenafil = defaults.SildenafilLESRelaxationNote
	}

	op.BaileyPlan = domain.EndoscopyBaileyChairPlan{
		EndoscopicRetrievalOrGastricPush:  trimmedRetrieval,
		PostRemovalSucralfateOmeprazole:   trimmedSucralfate,
		PyridostigmineAnticholinesterase:  trimmedPyrido,
		AspirationPneumoniaAntibioticPlan: trimmedAbx,
		UprightBaileyChairFeedingProtocol: trimmedBailey,
		SildenafilLESRelaxationNote:       trimmedSildenafil,
	}
	op.Status = domain.MegaStatusEndoscopicPyridostigmineStaged
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = time.Now().UTC()

	cp := cloneMegaesophagusOperation(op)
	return &cp, nil
}

// RecordEsophagealCheck appends an EsophagealAspirationCheck and updates op.SpO2Percent,
// op.AlveolarArterialGradientMmHg, op.RegurgitationEpisodes24h, op.UprightHoldMinutes,
// and op.MucosalEsophagitisGrade.
func (s *MegaesophagusService) RecordEsophagealCheck(
	id string,
	spO2Percent, aaGradientMmHg float64,
	regurgEpisodes24h, uprightHoldMinutes, esophagitisGrade int,
	recordedBy, clinicalNote string,
) (*domain.MegaesophagusOperation, error) {
	if spO2Percent < 50.0 || spO2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", spO2Percent)
	}
	if aaGradientMmHg < 0.0 || aaGradientMmHg > 80.0 {
		return nil, fmt.Errorf("alveolar_arterial_gradient_mmhg must be in [0.0, 80.0], got %v", aaGradientMmHg)
	}
	if regurgEpisodes24h < 0 || regurgEpisodes24h > 50 {
		return nil, fmt.Errorf("regurgitation_episodes_24h must be in [0, 50], got %d", regurgEpisodes24h)
	}
	if uprightHoldMinutes < 0 || uprightHoldMinutes > 120 {
		return nil, fmt.Errorf("upright_hold_minutes must be in [0, 120], got %d", uprightHoldMinutes)
	}
	if esophagitisGrade < 0 || esophagitisGrade > 4 {
		return nil, fmt.Errorf("mucosal_esophagitis_grade must be in [0, 4], got %d", esophagitisGrade)
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
		return nil, fmt.Errorf("megaesophagus operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.EsophagealAspirationCheck{
		ID:                           fmt.Sprintf("MEGA-CHK-%s-%d", op.ID, len(op.EsophagealChecks)+1),
		SpO2Percent:                  round1(spO2Percent),
		AlveolarArterialGradientMmHg: round1(aaGradientMmHg),
		RegurgitationEpisodes24h:     regurgEpisodes24h,
		UprightHoldMinutes:           uprightHoldMinutes,
		MucosalEsophagitisGrade:      esophagitisGrade,
		RecordedBy:                   trimmedBy,
		ClinicalNote:                 trimmedNote,
		RecordedAt:                   now,
	}

	op.EsophagealChecks = append(op.EsophagealChecks, chk)
	op.SpO2Percent = chk.SpO2Percent
	op.AlveolarArterialGradientMmHg = chk.AlveolarArterialGradientMmHg
	op.RegurgitationEpisodes24h = chk.RegurgitationEpisodes24h
	op.UprightHoldMinutes = chk.UprightHoldMinutes
	op.MucosalEsophagitisGrade = chk.MucosalEsophagitisGrade

	op.AspirationPlan = BuildRegurgitationAspirationPlan(
		op.PathologyType,
		op.SpO2Percent, op.AlveolarArterialGradientMmHg,
		op.RegurgitationEpisodes24h, op.MucosalEsophagitisGrade,
		op.AChRAntibodyTiterNmolL, op.AspirationPneumoniaPresent,
	)
	op.UpdatedAt = now

	cp := cloneMegaesophagusOperation(op)
	return &cp, nil
}

// ClearMegaesophagusDischarge transitions the operation to UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED
// once regurgitation is controlled, upright Bailey Chair feeding is established, and oxygenation normalizes.
func (s *MegaesophagusService) ClearMegaesophagusDischarge(
	id, dischargedBy, summary string,
) (*domain.MegaesophagusOperation, error) {
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
		return nil, fmt.Errorf("megaesophagus operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.MegaStatusUprightBaileyChairCleared
	op.AspirationPneumoniaPresent = false
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneMegaesophagusOperation(op)
	return &cp, nil
}

func cloneMegaesophagusOperation(op *domain.MegaesophagusOperation) domain.MegaesophagusOperation {
	cp := *op
	if op.EsophagealChecks != nil {
		cp.EsophagealChecks = make([]domain.EsophagealAspirationCheck, len(op.EsophagealChecks))
		copy(cp.EsophagealChecks, op.EsophagealChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
