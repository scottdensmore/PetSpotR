package domain

import (
	"fmt"
	"strings"
	"time"
)

// MegaesophagusTriageStatus represents the clinical workflow state of an Acute Esophageal
// Foreign Body, Megaesophagus, Myasthenia Gravis (AChR Antibody / Edrophonium-Neostigmine),
// Aspiration Pneumonia & Pyridostigmine/Bailey Chair triage operation.
type MegaesophagusTriageStatus string

const (
	MegaStatusAcuteObstructionAspirationTriage MegaesophagusTriageStatus = "ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE"
	MegaStatusEndoscopicPyridostigmineStaged   MegaesophagusTriageStatus = "ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED"
	MegaStatusUprightBaileyChairCleared        MegaesophagusTriageStatus = "UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED"
)

// IsValid returns true when MegaesophagusTriageStatus is a recognized lifecycle state.
func (s MegaesophagusTriageStatus) IsValid() bool {
	switch s {
	case MegaStatusAcuteObstructionAspirationTriage,
		MegaStatusEndoscopicPyridostigmineStaged,
		MegaStatusUprightBaileyChairCleared:
		return true
	default:
		return false
	}
}

// EsophagealPathologyType classifies the primary esophageal obstruction or dysmotility pathology.
type EsophagealPathologyType string

const (
	PathologyAcuteBoneFishhookForeignBody EsophagealPathologyType = "ACUTE_ESOPHAGEAL_BONE_FISHHOOK_FOREIGN_BODY"
	PathologyAcquiredMyastheniaGravis     EsophagealPathologyType = "ACQUIRED_MYASTHENIA_GRAVIS_MEGAESOPHAGUS"
	PathologyCongenitalPRAAIdiopathic     EsophagealPathologyType = "CONGENITAL_PRAA_IDIOPATHIC_MEGAESOPHAGUS"
)

// IsValid returns true when EsophagealPathologyType is a recognized esophageal pathology.
func (p EsophagealPathologyType) IsValid() bool {
	switch p {
	case PathologyAcuteBoneFishhookForeignBody,
		PathologyAcquiredMyastheniaGravis,
		PathologyCongenitalPRAAIdiopathic:
		return true
	default:
		return false
	}
}

// RegurgitationAspirationPlan captures passive regurgitation vs. active vomiting differentiation,
// 3-view thoracic radiography & POCUS findings, esophageal perforation / pneumomediastinum /
// pyothorax alert, Acetylcholine Receptor (AChR) antibody titer (> 0.6 nmol/L) & ACTH-stim /
// hypothyroidism screen note, liquid barium contrast contraindication, and Cholinergic (SLUDGE)
// vs. Myasthenic crisis differentiation with Atropine readiness.
type RegurgitationAspirationPlan struct {
	RegurgitationVsVomitingNote       string `json:"regurgitation_vs_vomiting_note"`
	ThoracicRadiographPOCUSFinding    string `json:"thoracic_radiograph_pocus_finding"`
	PerforationPneumomediastinumAlert string `json:"perforation_pneumomediastinum_alert"`
	AChRAntibodyAddisonScreenNote     string `json:"achr_antibody_addison_screen_note"`
	BariumSwallowContraindication     string `json:"barium_swallow_contraindication"`
	CholinergicVsMyasthenicCrisisNote string `json:"cholinergic_vs_myasthenic_crisis_note"`
}

// EndoscopyBaileyChairPlan captures flexible endoscopic foreign body retrieval or gentle gastric
// advancement (with strict contraindication of blind forceful pushing of spiculated bone/fishhooks),
// post-retrieval Sucralfate slurry & Omeprazole mucosal protection, Pyridostigmine Bromide
// anticholinesterase dosing (1–3 mg/kg PO q8–12h), aspiration pneumonia broad-spectrum antibiotic
// & coupage/nebulization plan, Upright Bailey Chair vertical feeding (20–30 min post-prandial
// gravity transit with Knox gelatin / meatball slurry consistency), and Sildenafil (1 mg/kg PO q8–12h)
// lower esophageal sphincter (LES) relaxation guidance.
type EndoscopyBaileyChairPlan struct {
	EndoscopicRetrievalOrGastricPush  string `json:"endoscopic_retrieval_or_gastric_push"`
	PostRemovalSucralfateOmeprazole   string `json:"post_removal_sucralfate_omeprazole"`
	PyridostigmineAnticholinesterase  string `json:"pyridostigmine_anticholinesterase"`
	AspirationPneumoniaAntibioticPlan string `json:"aspiration_pneumonia_antibiotic_plan"`
	UprightBaileyChairFeedingProtocol string `json:"upright_bailey_chair_feeding_protocol"`
	SildenafilLESRelaxationNote       string `json:"sildenafil_les_relaxation_note"`
}

// EsophagealAspirationCheck records a serial pulse oximetry SpO2, alveolar-arterial (A-a) oxygen
// gradient, 24-hour passive regurgitation episode count, upright Bailey Chair post-prandial hold
// duration, and endoscopic mucosal esophagitis grade check.
type EsophagealAspirationCheck struct {
	ID                           string    `json:"id"`
	SpO2Percent                  float64   `json:"spo2_percent"`
	AlveolarArterialGradientMmHg float64   `json:"alveolar_arterial_gradient_mmhg"`
	RegurgitationEpisodes24h     int       `json:"regurgitation_episodes_24h"`
	UprightHoldMinutes           int       `json:"upright_hold_minutes"`
	MucosalEsophagitisGrade      int       `json:"mucosal_esophagitis_grade"`
	RecordedBy                   string    `json:"recorded_by"`
	ClinicalNote                 string    `json:"clinical_note"`
	RecordedAt                   time.Time `json:"recorded_at"`
}

// MegaesophagusOperation represents an Acute Esophageal Foreign Body, Megaesophagus,
// Myasthenia Gravis & Aspiration Pneumonia triage operation in the PetSpotR Veterinary ICU cockpit.
type MegaesophagusOperation struct {
	ID                           string                      `json:"id"`
	CaseID                       string                      `json:"case_id"`
	PetName                      string                      `json:"pet_name"`
	Species                      string                      `json:"species"`
	Breed                        string                      `json:"breed"`
	WeightKg                     float64                     `json:"weight_kg"`
	Status                       MegaesophagusTriageStatus   `json:"status"`
	PathologyType                EsophagealPathologyType     `json:"pathology_type"`
	SpO2Percent                  float64                     `json:"spo2_percent"`
	AlveolarArterialGradientMmHg float64                     `json:"alveolar_arterial_gradient_mmhg"`
	RegurgitationEpisodes24h     int                         `json:"regurgitation_episodes_24h"`
	UprightHoldMinutes           int                         `json:"upright_hold_minutes"`
	MucosalEsophagitisGrade      int                         `json:"mucosal_esophagitis_grade"`
	AChRAntibodyTiterNmolL       float64                     `json:"achr_antibody_titer_nmol_l"`
	AspirationPneumoniaPresent   bool                        `json:"aspiration_pneumonia_present"`
	AspirationPlan               RegurgitationAspirationPlan `json:"aspiration_plan"`
	BaileyPlan                   EndoscopyBaileyChairPlan    `json:"bailey_plan"`
	EsophagealChecks             []EsophagealAspirationCheck `json:"esophageal_checks"`
	DischargedBy                 string                      `json:"discharged_by,omitempty"`
	DischargeSummary             string                      `json:"discharge_summary,omitempty"`
	DischargedAt                 *time.Time                  `json:"discharged_at,omitempty"`
	CreatedAt                    time.Time                   `json:"created_at"`
	UpdatedAt                    time.Time                   `json:"updated_at"`
}

// Validate checks domain invariants on MegaesophagusOperation.
func (op MegaesophagusOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.CaseID) == "" {
		return fmt.Errorf("case_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if op.WeightKg <= 0 {
		return fmt.Errorf("weight_kg must be positive, got %v", op.WeightKg)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if !op.PathologyType.IsValid() {
		return fmt.Errorf("invalid pathology_type: %q", op.PathologyType)
	}
	return nil
}

// MegaesophagusOverviewMetrics summarizes aggregate counts across Megaesophagus & Foreign Body operations.
type MegaesophagusOverviewMetrics struct {
	TotalOperations                    int `json:"total_operations"`
	AcuteObstructionTriageCount        int `json:"acute_obstruction_triage_count"`
	EndoscopyPyridostigmineStagedCount int `json:"endoscopy_pyridostigmine_staged_count"`
	ClearedBaileyChairCount            int `json:"cleared_bailey_chair_count"`
	ForeignBodyObstructionCount        int `json:"foreign_body_obstruction_count"`
	AspirationPneumoniaActiveCount     int `json:"aspiration_pneumonia_active_count"`
}
