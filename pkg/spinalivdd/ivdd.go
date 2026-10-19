package spinalivdd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// CreateOperationInput defines parameters for creating a new Spinal IVDD operation.
type CreateOperationInput struct {
	CaseID                   string
	PetName                  string
	Species                  string
	Breed                    string
	WeightKg                 float64
	FrankelGrade             domain.ModifiedFrankelGrade
	LocalizationZone         domain.NeurolocalizationZone
	OnsetHours               float64
	CutaneousTrunciVertebra  string
	DeepPainPresent          bool
	SchiffSherringtonPresent bool
	PriorSteroidOrNSAID      string
	SurgicalWindowHours      float64
	TargetMAPMmHg            int
}

// IVDDService manages Acute Spinal Cord Injury, Intervertebral Disc Extrusion (IVDD),
// Deep Pain Nociception, Schiff-Sherrington Posture & Hemilaminectomy triage operations.
type IVDDService struct {
	mu       sync.RWMutex
	ops      map[string]*domain.SpinalIVDDOperation
	order    []string
	seq      int
	checkSeq int
}

// NewIVDDService initializes an IVDDService with 3 seeded operations (IVDD-OP-101, IVDD-OP-102, IVDD-OP-103).
func NewIVDDService() *IVDDService {
	s := &IVDDService{
		ops:      make(map[string]*domain.SpinalIVDDOperation),
		order:    make([]string, 0, 8),
		seq:      103,
		checkSeq: 5,
	}
	s.seed()
	return s
}

// BuildNociceptionPlan computes the NociceptionAssessmentPlan for an IVDD operation.
func BuildNociceptionPlan(
	frankelGrade domain.ModifiedFrankelGrade,
	deepPainPresent bool,
	schiffSherrington bool,
	cutaneousTrunci string,
	priorMed string,
) domain.NociceptionAssessmentPlan {
	deepPainDesc := "INTACT — Conscious cerebral perception (behavioral turn of head, vocalization, or autonomic dilation) verified via periosteal digit钳 clamp."
	if !deepPainPresent || frankelGrade == domain.FrankelGrade5DeepPainAbsent {
		deepPainDesc = "ABSENT (Grade 5 Emergency) — Zero conscious behavioural response to hemostat periosteal bone clamping on medial/lateral digits & tail base; <12–24h surgical decompression window."
	}

	withdrawalNote := "CRITICAL DISTINCTION: Local spinal flexor withdrawal reflex (L4–S1 sciatic/femoral arc) pulls the limb away WITHOUT cerebral pain perception; never mistake an intact spinal flexor reflex for intact Deep Pain Perception!"

	schiffNote := "Absent — Normal thoracic limb extensor tone."
	if schiffSherrington {
		schiffNote = "PRESENT — Severe thoracic limb hypertonicity/extensor rigidity from acute T3–L3 border-cell (L1–L7) inhibitory interneuron disruption; indicates severe acute thoracolumbar lesion but is NOT a prognostic indicator of irreversible cord transection when deep pain is evaluated."
	}

	cutoffNote := fmt.Sprintf("Cutaneous trunci (panniculus) reflex cutoff at %s (lesion typically 1–2 vertebral segments cranial to cutoff); monitor q2h for cranial migration.", cutaneousTrunci)
	myelomalaciaWarning := "ASCENDING/DESCENDING HEMORRHAGIC MYELOMALACIA GUARD: Cranial migration of cutaneous trunci cutoff >1–2 vertebral bodies, loss of pelvic LMN reflexes (patellar/sciatic), hypothermia, or diaphragmatic paresis signals progressive hemorrhagic myelomalacia (~10–15% risk in Grade 5 extrusions)."

	washoutAlert := "STEROID + NSAID GI PERFORATION CONTRAINDICATION: NEVER co-administer corticosteroids (Dexamethasone / Prednisone / Methylprednisolone) with NSAIDs (Meloxicam / Carprofen / Robenacoxib) without a 5–7 day washout; high-dose steroids are contraindicated in acute IVDD due to fatal colonic perforation and gastrointestinal hemorrhage."
	if strings.TrimSpace(priorMed) != "" {
		washoutAlert = fmt.Sprintf("%s [Prior Exposure: %s]", washoutAlert, strings.TrimSpace(priorMed))
	}

	return domain.NociceptionAssessmentPlan{
		DeepPainPerception:       deepPainDesc,
		WithdrawalReflexNote:     withdrawalNote,
		SchiffSherringtonPosture: schiffNote,
		CutaneousTrunciCutoff:    cutoffNote,
		MyelomalaciaRiskWarning:  myelomalaciaWarning,
		SteroidNSAIDWashoutAlert: washoutAlert,
	}
}

// BuildDefaultDecompressionPlan computes default surgical decompression, neurogenic bladder, and analgesia protocols.
func BuildDefaultDecompressionPlan(
	frankelGrade domain.ModifiedFrankelGrade,
	zone domain.NeurolocalizationZone,
	windowHours float64,
	targetMAP int,
) domain.DecompressionBladderPlan {
	windowTarget := "Elective / Urgent Decompression within 24–48h (Deep Pain Intact)"
	if frankelGrade == domain.FrankelGrade5DeepPainAbsent {
		windowTarget = fmt.Sprintf("EMERGENT Hemilaminectomy within %.0fh (<12–24h Deep Pain Absent Window); maintain MAP >= %d mmHg for spinal cord perfusion", windowHours, targetMAP)
	} else if windowHours > 0 {
		windowTarget = fmt.Sprintf("Staged Decompression within %.0fh; maintain MAP >= %d mmHg", windowHours, targetMAP)
	}

	imaging := "High-Field 1.5T/3.0T MRI (T2W sagittal/transverse) or Non-Contrast Helical CT / CT-Myelogram"
	hemiSite := "Thoracolumbar Hemilaminectomy (T12–T13 / T13–L1 / L1–L2)"
	bladderProto := "UMN Bladder (T3–L3 Hypertonic Sphincter): Prazosin 0.05–0.1 mg/kg PO q8–12h + Diazepam 0.2 mg/kg IV/PO 20 min prior to gentle manual expression q6–8h or intermittent sterile catheterization."

	switch zone {
	case domain.LocalizationL4S3LumbosacralLMN:
		hemiSite = "Lumbosacral Dorsal Laminectomy / Caudal Lumbar Hemilaminectomy (L4–S1)"
		bladderProto = "LMN Bladder (L4–S3 Atonic Detrusor + Sphincter Overflow): Manual expression q6h + Bethanechol 5–15 mg PO q8h once outflow resistance confirmed low; monitor for UTI."
	case domain.LocalizationC1C5CervicalTetraparesis:
		hemiSite = "Cervical Ventral Slot Decompression (C2–C5)"
		bladderProto = "UMN Tetraparetic Bladder: Intermittent urinary catheterization or indwelling closed collection system q6h; monitor respiratory excursion for C3–C5 phrenic nerve compromise."
	}

	return domain.DecompressionBladderPlan{
		SurgicalWindowTarget:      windowTarget,
		ImagingModality:           imaging,
		HemilaminectomySite:       hemiSite,
		BladderManagementProtocol: bladderProto,
		AnalgesiaCRIProtocol:      "Fentanyl CRI 2–5 mcg/kg/h IV + Ketamine sub-anesthetic CRI 2–5 mcg/kg/min + Gabapentin 10–20 mg/kg PO q8h (ZERO concurrent steroid + NSAID)",
		StrictCrateRestProtocol:   "Strict 4–6 week confined crate rest, non-slip padded bedding turned q4h, passive range-of-motion (PROM) physiotherapy, sling-supported elimination walks only.",
	}
}

func (s *IVDDService) seed() {
	t101 := time.Date(2026, 10, 19, 5, 0, 0, 0, time.UTC)
	op101 := &domain.SpinalIVDDOperation{
		ID:                       "IVDD-OP-101",
		CaseID:                   "CASE-IVDD-2231",
		PetName:                  "Barnaby",
		Species:                  "Canine",
		Breed:                    "French Bulldog",
		WeightKg:                 12.4,
		Status:                   domain.SpinalStatusAcuteParaplegiaTriage,
		FrankelGrade:             domain.FrankelGrade5DeepPainAbsent,
		LocalizationZone:         domain.LocalizationT3L3ThoracolumbarUMN,
		OnsetHours:               4.5,
		CutaneousTrunciVertebra:  "L1 (Caudal Border Absent)",
		DeepPainPresent:          false,
		SchiffSherringtonPresent: true,
		PriorSteroidOrNSAID:      "Meloxicam 0.1 mg/kg PO given 6h ago — Dexamethasone/Prednisone STRICTLY CONTRAINDICATED",
		SurgicalWindowHours:      12.0,
		TargetMAPMmHg:            85,
		NociceptionPlan: BuildNociceptionPlan(
			domain.FrankelGrade5DeepPainAbsent,
			false,
			true,
			"L1 (Caudal Border Absent)",
			"Meloxicam 0.1 mg/kg PO given 6h ago — Dexamethasone/Prednisone STRICTLY CONTRAINDICATED",
		),
		DecompressionPlan: BuildDefaultDecompressionPlan(
			domain.FrankelGrade5DeepPainAbsent,
			domain.LocalizationT3L3ThoracolumbarUMN,
			12.0,
			85,
		),
		NeuroChecks: []domain.NeuroNociceptionCheck{
			{
				ID:                   "NC-101",
				FrankelGrade:         domain.FrankelGrade5DeepPainAbsent,
				DeepPainStatus:       "Absent Bilaterally (Local Flexor Withdrawal Present Only)",
				CutaneousTrunciLevel: "L1 (Caudal Border Absent)",
				BladderToneStatus:    "UMN Hypertonic Turgid — Difficult Manual Expression (Prazosin Started)",
				RecordedBy:           "Dr. Marcus Vance, DACVECC",
				ClinicalNote:         "Acute thoracolumbar Hansen Type I extrusion; Schiff-Sherrington forelimb extensor rigidity present; local pedal withdrawal intact but zero conscious cerebral response to periosteal digit clamping.",
				RecordedAt:           t101.Add(10 * time.Minute),
			},
			{
				ID:                   "NC-102",
				FrankelGrade:         domain.FrankelGrade5DeepPainAbsent,
				DeepPainStatus:       "Absent Bilaterally — Stable Cutaneous Trunci at L1",
				CutaneousTrunciLevel: "L1 (Stable — No Cranial Migration)",
				BladderToneStatus:    "UMN Sphincter Relaxing on Prazosin + Diazepam; Aseptically Expressed 140 mL",
				RecordedBy:           "Dr. Elena Rostova, DACVIM (Neurology)",
				ClinicalNote:         "No cranial migration of panniculus cutoff (L1 stable); MAP maintained at 88 mmHg on crystalloid support; prepped for emergent 3D CT & left T12–L1 hemilaminectomy.",
				RecordedAt:           t101.Add(45 * time.Minute),
			},
		},
		CreatedAt: t101,
		UpdatedAt: t101.Add(45 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	t102 := time.Date(2026, 10, 19, 5, 30, 0, 0, time.UTC)
	op102 := &domain.SpinalIVDDOperation{
		ID:                       "IVDD-OP-102",
		CaseID:                   "CASE-IVDD-2232",
		PetName:                  "Strudel",
		Species:                  "Canine",
		Breed:                    "Miniature Dachshund",
		WeightKg:                 5.8,
		Status:                   domain.SpinalStatusHemilaminectomyStaged,
		FrankelGrade:             domain.FrankelGrade4DeepPainIntact,
		LocalizationZone:         domain.LocalizationT3L3ThoracolumbarUMN,
		OnsetHours:               8.0,
		CutaneousTrunciVertebra:  "T13",
		DeepPainPresent:          true,
		SchiffSherringtonPresent: false,
		PriorSteroidOrNSAID:      "None — Gabapentin + Methadone/Fentanyl CRI only",
		SurgicalWindowHours:      24.0,
		TargetMAPMmHg:            80,
		NociceptionPlan: BuildNociceptionPlan(
			domain.FrankelGrade4DeepPainIntact,
			true,
			false,
			"T13",
			"None — Gabapentin + Methadone/Fentanyl CRI only",
		),
		DecompressionPlan: domain.DecompressionBladderPlan{
			SurgicalWindowTarget:      "Staged Decompression within 24h (Grade 4 Paraplegia with Intact Deep Pain); Target MAP >= 80 mmHg",
			ImagingModality:           "Non-Contrast Helical CT + 1.5T Thoracolumbar MRI",
			HemilaminectomySite:       "Right T12–T13 Hemilaminectomy + Prophylactic T13–L1 Disc Fenestration",
			BladderManagementProtocol: "UMN Bladder Protocol: Prazosin 0.5 mg PO q8h + Diazepam 0.2 mg/kg IV pre-expression q6h",
			AnalgesiaCRIProtocol:      "Fentanyl CRI 3 mcg/kg/h + Gabapentin 10 mg/kg PO q8h",
			StrictCrateRestProtocol:   "Strict 4-week post-hemilaminectomy crate confinement, non-slip orthopedic mat, passive ROM q8h",
		},
		NeuroChecks: []domain.NeuroNociceptionCheck{
			{
				ID:                   "NC-103",
				FrankelGrade:         domain.FrankelGrade4DeepPainIntact,
				DeepPainStatus:       "Intact Bilaterally (Clear Head Turn & Vocalization on Medial/Lateral Digits)",
				CutaneousTrunciLevel: "T13",
				BladderToneStatus:    "UMN Moderate Sphincter Tone — Expressed 65 mL Post-Diazepam",
				RecordedBy:           "Dr. Elena Rostova, DACVIM (Neurology)",
				ClinicalNote:         "Paraplegic (no voluntary motor) but conscious deep pain intact bilaterally; CT confirmed mineralized extruded nucleus pulposus ventral-right at T12–T13.",
				RecordedAt:           t102.Add(20 * time.Minute),
			},
		},
		CreatedAt: t102,
		UpdatedAt: t102.Add(20 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	t103 := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	dischargedAt103 := t103.Add(72 * time.Hour)
	op103 := &domain.SpinalIVDDOperation{
		ID:                       "IVDD-OP-103",
		CaseID:                   "CASE-IVDD-2233",
		PetName:                  "Pickles",
		Species:                  "Canine",
		Breed:                    "Pembroke Welsh Corgi",
		WeightKg:                 11.2,
		Status:                   domain.SpinalStatusPostopBladderCleared,
		FrankelGrade:             domain.FrankelGrade2AmbulatoryParaparesis,
		LocalizationZone:         domain.LocalizationL4S3LumbosacralLMN,
		OnsetHours:               72.0,
		CutaneousTrunciVertebra:  "Intact to L5",
		DeepPainPresent:          true,
		SchiffSherringtonPresent: false,
		PriorSteroidOrNSAID:      "Carprofen Post-Op Day 3",
		SurgicalWindowHours:      0,
		TargetMAPMmHg:            80,
		NociceptionPlan: BuildNociceptionPlan(
			domain.FrankelGrade2AmbulatoryParaparesis,
			true,
			false,
			"Intact to L5",
			"Carprofen Post-Op Day 3",
		),
		DecompressionPlan: domain.DecompressionBladderPlan{
			SurgicalWindowTarget:      "Completed L4–L5 Left Hemilaminectomy 72h Ago",
			ImagingModality:           "1.5T Lumbosacral MRI Completed",
			HemilaminectomySite:       "Left L4–L5 Hemilaminectomy (Decompressed)",
			BladderManagementProtocol: "Spontaneous Voluntary Micturition Restored (<2 mL/kg Post-Void Residual)",
			AnalgesiaCRIProtocol:      "Oral Carprofen 2.2 mg/kg PO q12h + Gabapentin 10 mg/kg PO q8h (Zero Steroids)",
			StrictCrateRestProtocol:   "4-week strict crate rest at home, short 5-minute leash-only bathroom walks with rear harness support, zero stairs or jumping.",
		},
		NeuroChecks: []domain.NeuroNociceptionCheck{
			{
				ID:                   "NC-104",
				FrankelGrade:         domain.FrankelGrade3NonAmbulatoryParaparesis,
				DeepPainStatus:       "Intact Bilaterally — Strong Voluntary Tail Wag & Pelvic Limb Motor",
				CutaneousTrunciLevel: "Intact to L5",
				BladderToneStatus:    "Improving Detrusor Contraction — Low Residual",
				RecordedBy:           "Dr. Marcus Vance, DACVECC",
				ClinicalNote:         "24h post-op L4–L5 hemilaminectomy; strong non-ambulatory voluntary motor movement and intact deep pain.",
				RecordedAt:           t103.Add(24 * time.Hour),
			},
			{
				ID:                   "NC-105",
				FrankelGrade:         domain.FrankelGrade2AmbulatoryParaparesis,
				DeepPainStatus:       "Intact Bilaterally — Ambulatory Paraparesis with Mild Proprioceptive Ataxia",
				CutaneousTrunciLevel: "Intact to L5",
				BladderToneStatus:    "Voluntary Voiding Confirmed Twice Today",
				RecordedBy:           "Dr. Elena Rostova, DACVIM (Neurology)",
				ClinicalNote:         "72h post-op; ambulatory with mild ataxia, voluntary urination confirmed, incision clean/dry; cleared for strict crate-rest home recovery.",
				RecordedAt:           dischargedAt103,
			},
		},
		DischargedBy:     "Dr. Elena Rostova, DACVIM (Neurology)",
		DischargeSummary: "72h post-L4–L5 hemilaminectomy; recovered to Grade 2 ambulatory paraparesis with intact deep pain nociception and voluntary bladder voiding; discharged on Carprofen + Gabapentin with 4-week strict crate rest.",
		DischargedAt:     &dischargedAt103,
		CreatedAt:        t103,
		UpdatedAt:        dischargedAt103,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of SpinalIVDDOperation records, optionally filtered by statusFilter.
func (s *IVDDService) ListOperations(statusFilter string) []domain.SpinalIVDDOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.SpinalIVDDOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneSpinalIVDDOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a SpinalIVDDOperation by ID.
func (s *IVDDService) GetOperation(id string) (*domain.SpinalIVDDOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("spinal IVDD operation %q not found", id)
	}
	cp := cloneSpinalIVDDOperation(op)
	return &cp, nil
}

// GetOverviewMetrics computes aggregate KPI counts across all Spinal IVDD operations.
func (s *IVDDService) GetOverviewMetrics() domain.SpinalIVDDOverviewMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.SpinalIVDDOverviewMetrics
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.SpinalStatusAcuteParaplegiaTriage:
			m.AcuteParaplegiaTriageCount++
		case domain.SpinalStatusHemilaminectomyStaged:
			m.HemilaminectomyStagedCount++
		case domain.SpinalStatusPostopBladderCleared:
			m.ClearedPostopCount++
		}
		if !op.DeepPainPresent || op.FrankelGrade == domain.FrankelGrade5DeepPainAbsent {
			m.DeepPainAbsentEmergencyCount++
		}
		if op.SchiffSherringtonPresent {
			m.SchiffSherringtonCount++
		}
	}
	return m
}

// ComputeMetrics is an alias for GetOverviewMetrics for consistency across services.
func (s *IVDDService) ComputeMetrics() domain.SpinalIVDDOverviewMetrics {
	return s.GetOverviewMetrics()
}

// CreateOperation validates and creates a new Spinal IVDD triage operation.
func (s *IVDDService) CreateOperation(input CreateOperationInput) (*domain.SpinalIVDDOperation, error) {
	petName := strings.TrimSpace(input.PetName)
	if petName == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	species := strings.TrimSpace(input.Species)
	if species == "" {
		species = "Canine"
	}
	breed := strings.TrimSpace(input.Breed)
	if breed == "" {
		breed = "Mixed Chondrodystrophic"
	}
	if input.WeightKg <= 0 || input.WeightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 120.0], got %v", input.WeightKg)
	}
	frankelGrade := domain.ModifiedFrankelGrade(strings.ToUpper(strings.TrimSpace(string(input.FrankelGrade))))
	if frankelGrade == "" {
		frankelGrade = domain.FrankelGrade5DeepPainAbsent
	}
	if !frankelGrade.IsValid() {
		return nil, fmt.Errorf("invalid frankel_grade: %q", input.FrankelGrade)
	}
	zone := domain.NeurolocalizationZone(strings.ToUpper(strings.TrimSpace(string(input.LocalizationZone))))
	if zone == "" {
		zone = domain.LocalizationT3L3ThoracolumbarUMN
	}
	if !zone.IsValid() {
		return nil, fmt.Errorf("invalid localization_zone: %q", input.LocalizationZone)
	}
	if input.OnsetHours < 0 || input.OnsetHours > 720 {
		return nil, fmt.Errorf("onset_hours must be in [0, 720], got %v", input.OnsetHours)
	}
	cutaneousTrunci := strings.TrimSpace(input.CutaneousTrunciVertebra)
	if cutaneousTrunci == "" {
		cutaneousTrunci = "L1"
	}
	priorMed := strings.TrimSpace(input.PriorSteroidOrNSAID)
	if priorMed == "" {
		priorMed = "None reported — Maintain strict Steroid/NSAID separation"
	}
	windowHours := input.SurgicalWindowHours
	if windowHours <= 0 {
		if frankelGrade == domain.FrankelGrade5DeepPainAbsent {
			windowHours = 12.0
		} else {
			windowHours = 24.0
		}
	}
	targetMAP := input.TargetMAPMmHg
	if targetMAP == 0 {
		targetMAP = 85
	}
	if targetMAP < 50 || targetMAP > 180 {
		return nil, fmt.Errorf("target_map_mmhg must be in [50, 180], got %d", targetMAP)
	}

	deepPain := input.DeepPainPresent
	if frankelGrade == domain.FrankelGrade5DeepPainAbsent {
		deepPain = false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("IVDD-OP-%d", s.seq)
	caseID := strings.TrimSpace(input.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-IVDD-%d", 2230+s.seq-100)
	}
	now := time.Now().UTC()

	op := &domain.SpinalIVDDOperation{
		ID:                       opID,
		CaseID:                   caseID,
		PetName:                  petName,
		Species:                  species,
		Breed:                    breed,
		WeightKg:                 input.WeightKg,
		Status:                   domain.SpinalStatusAcuteParaplegiaTriage,
		FrankelGrade:             frankelGrade,
		LocalizationZone:         zone,
		OnsetHours:               input.OnsetHours,
		CutaneousTrunciVertebra:  cutaneousTrunci,
		DeepPainPresent:          deepPain,
		SchiffSherringtonPresent: input.SchiffSherringtonPresent,
		PriorSteroidOrNSAID:      priorMed,
		SurgicalWindowHours:      windowHours,
		TargetMAPMmHg:            targetMAP,
		NociceptionPlan:          BuildNociceptionPlan(frankelGrade, deepPain, input.SchiffSherringtonPresent, cutaneousTrunci, priorMed),
		DecompressionPlan:        BuildDefaultDecompressionPlan(frankelGrade, zone, windowHours, targetMAP),
		NeuroChecks:              []domain.NeuroNociceptionCheck{},
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneSpinalIVDDOperation(op)
	return &cp, nil
}

// StageDecompression stages CT/MRI imaging, hemilaminectomy surgical site, UMN/LMN bladder protocol,
// analgesia CRI, and strict crate rest, transitioning the operation to CT_MYELOGRAM_HEMILAMINECTOMY_STAGED.
func (s *IVDDService) StageDecompression(
	id, surgicalWindowTarget, imagingModality, hemilaminectomySite, bladderProtocol, analgesiaProtocol, crateRestProtocol string,
) (*domain.SpinalIVDDOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("spinal IVDD operation %q not found", id)
	}

	win := strings.TrimSpace(surgicalWindowTarget)
	if win == "" {
		win = op.DecompressionPlan.SurgicalWindowTarget
	}
	img := strings.TrimSpace(imagingModality)
	if img == "" {
		img = op.DecompressionPlan.ImagingModality
	}
	site := strings.TrimSpace(hemilaminectomySite)
	if site == "" {
		site = op.DecompressionPlan.HemilaminectomySite
	}
	bladder := strings.TrimSpace(bladderProtocol)
	if bladder == "" {
		bladder = op.DecompressionPlan.BladderManagementProtocol
	}
	analgesia := strings.TrimSpace(analgesiaProtocol)
	if analgesia == "" {
		analgesia = op.DecompressionPlan.AnalgesiaCRIProtocol
	}
	crateRest := strings.TrimSpace(crateRestProtocol)
	if crateRest == "" {
		crateRest = op.DecompressionPlan.StrictCrateRestProtocol
	}

	op.DecompressionPlan = domain.DecompressionBladderPlan{
		SurgicalWindowTarget:      win,
		ImagingModality:           img,
		HemilaminectomySite:       site,
		BladderManagementProtocol: bladder,
		AnalgesiaCRIProtocol:      analgesia,
		StrictCrateRestProtocol:   crateRest,
	}
	op.Status = domain.SpinalStatusHemilaminectomyStaged
	op.DischargedAt = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.UpdatedAt = time.Now().UTC()

	cp := cloneSpinalIVDDOperation(op)
	return &cp, nil
}

// RecordNeuroCheck records a serial neurological nociception, cutaneous trunci cutoff, and bladder tone evaluation.
func (s *IVDDService) RecordNeuroCheck(
	id string,
	frankelGrade domain.ModifiedFrankelGrade,
	deepPainStatus, cutaneousTrunciLevel, bladderToneStatus, recordedBy, clinicalNote string,
) (*domain.SpinalIVDDOperation, error) {
	fg := domain.ModifiedFrankelGrade(strings.ToUpper(strings.TrimSpace(string(frankelGrade))))
	if !fg.IsValid() {
		return nil, fmt.Errorf("invalid frankel_grade: %q", frankelGrade)
	}
	dpStatus := strings.TrimSpace(deepPainStatus)
	if dpStatus == "" {
		return nil, fmt.Errorf("deep_pain_status is required")
	}
	trunci := strings.TrimSpace(cutaneousTrunciLevel)
	if trunci == "" {
		return nil, fmt.Errorf("cutaneous_trunci_level is required")
	}
	bladder := strings.TrimSpace(bladderToneStatus)
	if bladder == "" {
		return nil, fmt.Errorf("bladder_tone_status is required")
	}
	recBy := strings.TrimSpace(recordedBy)
	if recBy == "" {
		return nil, fmt.Errorf("recorded_by is required")
	}
	note := strings.TrimSpace(clinicalNote)

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("spinal IVDD operation %q not found", id)
	}

	s.checkSeq++
	now := time.Now().UTC()
	chk := domain.NeuroNociceptionCheck{
		ID:                   fmt.Sprintf("NC-%d", 100+s.checkSeq),
		FrankelGrade:         fg,
		DeepPainStatus:       dpStatus,
		CutaneousTrunciLevel: trunci,
		BladderToneStatus:    bladder,
		RecordedBy:           recBy,
		ClinicalNote:         note,
		RecordedAt:           now,
	}

	op.NeuroChecks = append(op.NeuroChecks, chk)
	op.FrankelGrade = fg
	op.CutaneousTrunciVertebra = trunci
	op.DeepPainPresent = fg != domain.FrankelGrade5DeepPainAbsent && !strings.Contains(strings.ToUpper(dpStatus), "ABSENT")
	if op.DeepPainPresent && (fg == domain.FrankelGrade2AmbulatoryParaparesis || fg == domain.FrankelGrade3NonAmbulatoryParaparesis) {
		op.SchiffSherringtonPresent = false
	}
	op.NociceptionPlan = BuildNociceptionPlan(op.FrankelGrade, op.DeepPainPresent, op.SchiffSherringtonPresent, op.CutaneousTrunciVertebra, op.PriorSteroidOrNSAID)
	op.UpdatedAt = now

	cp := cloneSpinalIVDDOperation(op)
	return &cp, nil
}

// ClearNeurologyDischarge certifies intact post-op nociception, bladder clearance, and strict crate-rest discharge.
func (s *IVDDService) ClearNeurologyDischarge(id, dischargedBy, summary string) (*domain.SpinalIVDDOperation, error) {
	by := strings.TrimSpace(dischargedBy)
	if by == "" {
		return nil, fmt.Errorf("discharged_by is required")
	}
	sum := strings.TrimSpace(summary)
	if sum == "" {
		return nil, fmt.Errorf("discharge_summary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("spinal IVDD operation %q not found", id)
	}

	now := time.Now().UTC()
	op.Status = domain.SpinalStatusPostopBladderCleared
	op.DeepPainPresent = true
	op.SchiffSherringtonPresent = false
	if op.FrankelGrade == domain.FrankelGrade5DeepPainAbsent || op.FrankelGrade == domain.FrankelGrade4DeepPainIntact {
		op.FrankelGrade = domain.FrankelGrade3NonAmbulatoryParaparesis
	}
	op.NociceptionPlan = BuildNociceptionPlan(op.FrankelGrade, op.DeepPainPresent, op.SchiffSherringtonPresent, op.CutaneousTrunciVertebra, op.PriorSteroidOrNSAID)
	op.DischargedBy = by
	op.DischargeSummary = sum
	op.DischargedAt = &now
	op.UpdatedAt = now

	cp := cloneSpinalIVDDOperation(op)
	return &cp, nil
}

func cloneSpinalIVDDOperation(op *domain.SpinalIVDDOperation) domain.SpinalIVDDOperation {
	cp := *op
	if op.NeuroChecks != nil {
		cp.NeuroChecks = make([]domain.NeuroNociceptionCheck, len(op.NeuroChecks))
		copy(cp.NeuroChecks, op.NeuroChecks)
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
