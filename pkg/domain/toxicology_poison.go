package domain

import (
	"fmt"
	"strings"
)

// ToxicologyAgentType classifies the poison or toxicant agent in a veterinary toxicology & poison control triage operation.
type ToxicologyAgentType string

const (
	ToxicologyAgentSecondGenAnticoagulantBrodifacoum  ToxicologyAgentType = "SECOND_GEN_ANTICOAGULANT_BRODIFACOUM"
	ToxicologyAgentXylitolInsulinSurgeHepatotoxin     ToxicologyAgentType = "XYLITOL_INSULIN_SURGE_HEPATOTOXIN"
	ToxicologyAgentBromethalinNeurotoxicCerebralEdema ToxicologyAgentType = "BROMETHALIN_NEUROTOXIC_CEREBRAL_EDEMA"
	ToxicologyAgentLiliumSppFelineNephrotoxin         ToxicologyAgentType = "LILIUM_SPP_FELINE_NEPHROTOXIN"
	ToxicologyAgentIvermectinPermethrinLipophilic     ToxicologyAgentType = "IVERMECTIN_PERYTHRIN_LIPOPHILIC_TOXIN"
)

// IsValid returns true when ToxicologyAgentType is a supported agent enum value.
func (a ToxicologyAgentType) IsValid() bool {
	switch a {
	case ToxicologyAgentSecondGenAnticoagulantBrodifacoum,
		ToxicologyAgentXylitolInsulinSurgeHepatotoxin,
		ToxicologyAgentBromethalinNeurotoxicCerebralEdema,
		ToxicologyAgentLiliumSppFelineNephrotoxin,
		ToxicologyAgentIvermectinPermethrinLipophilic:
		return true
	default:
		return false
	}
}

// ToxicologyTriageStatus represents the lifecycle state of an Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control triage operation.
type ToxicologyTriageStatus string

const (
	ToxicologyStatusCriticalCoagulopathyHepaticNeuro ToxicologyTriageStatus = "COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL"
	ToxicologyStatusStagedVitaminK1ILECharcoalDecon  ToxicologyTriageStatus = "VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED"
	ToxicologyStatusClearedDecontaminatedPTGlucose   ToxicologyTriageStatus = "DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED"
)

// IsValid returns true when ToxicologyTriageStatus is a supported lifecycle state.
func (s ToxicologyTriageStatus) IsValid() bool {
	switch s {
	case ToxicologyStatusCriticalCoagulopathyHepaticNeuro,
		ToxicologyStatusStagedVitaminK1ILECharcoalDecon,
		ToxicologyStatusClearedDecontaminatedPTGlucose:
		return true
	default:
		return false
	}
}

// ToxicologyProtocolStep represents an actionable emesis/charcoal gate, Phytonadione Vitamin K1 / 20% ILE lipid sink, dextrose/NAC/SAMe/48h IV diuresis, or 48h PT/hepatic/renal panel step.
type ToxicologyProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// ToxicologyPoisonOperation represents an Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control triage operation.
type ToxicologyPoisonOperation struct {
	ID                           string                   `json:"id"`
	PetID                        string                   `json:"pet_id"`
	PetName                      string                   `json:"pet_name"`
	Species                      string                   `json:"species"`
	WeightKg                     float64                  `json:"weight_kg"`
	AgentType                    ToxicologyAgentType      `json:"agent_type"`
	Status                       ToxicologyTriageStatus   `json:"status"`
	EstimatedDoseMgKg            float64                  `json:"estimated_dose_mg_kg"`
	ProthrombinTimeRatio         float64                  `json:"prothrombin_time_ratio"`
	BloodGlucoseMgDl             float64                  `json:"blood_glucose_mg_dl"`
	TimeSinceIngestionMins       int                      `json:"time_since_ingestion_mins"`
	CharcoalAvoidedForXylitol    bool                     `json:"charcoal_avoided_for_xylitol"`
	VitaminK3MenadioneAvoided    bool                     `json:"vitamin_k3_menadione_avoided"`
	RequiredAntidoteDeconRegimen string                   `json:"required_antidote_decon_regimen"`
	ToxicologyPitfallWarningNote string                   `json:"toxicology_pitfall_warning_note"`
	ProtocolSteps                []ToxicologyProtocolStep `json:"protocol_steps"`
	ClearanceToken               string                   `json:"clearance_token,omitempty"`
	CreatedAt                    string                   `json:"created_at"`
	UpdatedAt                    string                   `json:"updated_at"`
}

// ToxicologyEvaluationRequest represents input parameters for evaluating a toxicology & poison control triage operation.
type ToxicologyEvaluationRequest struct {
	ID                        string              `json:"id,omitempty"`
	PetID                     string              `json:"pet_id"`
	PetName                   string              `json:"pet_name"`
	Species                   string              `json:"species"`
	WeightKg                  float64             `json:"weight_kg"`
	AgentType                 ToxicologyAgentType `json:"agent_type"`
	EstimatedDoseMgKg         float64             `json:"estimated_dose_mg_kg"`
	ProthrombinTimeRatio      float64             `json:"prothrombin_time_ratio"`
	BloodGlucoseMgDl          float64             `json:"blood_glucose_mg_dl"`
	TimeSinceIngestionMins    int                 `json:"time_since_ingestion_mins"`
	CharcoalAvoidedForXylitol bool                `json:"charcoal_avoided_for_xylitol"`
	VitaminK3MenadioneAvoided bool                `json:"vitamin_k3_menadione_avoided"`
}

// ToxicologyStepUpdateRequest updates a protocol step on an existing toxicology poison triage operation.
type ToxicologyStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on ToxicologyPoisonOperation.
func (op ToxicologyPoisonOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.PetID) == "" {
		return fmt.Errorf("pet_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg < 0.8 || op.WeightKg > 100.0 {
		return fmt.Errorf("weight_kg must be in [0.8, 100.0], got %v", op.WeightKg)
	}
	if !op.AgentType.IsValid() {
		return fmt.Errorf("invalid agent_type: %q", op.AgentType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.EstimatedDoseMgKg <= 0 || op.EstimatedDoseMgKg > 5000.0 {
		return fmt.Errorf("estimated_dose_mg_kg must be in (0, 5000.0], got %v", op.EstimatedDoseMgKg)
	}
	if op.ProthrombinTimeRatio < 0.8 || op.ProthrombinTimeRatio > 10.0 {
		return fmt.Errorf("prothrombin_time_ratio must be in [0.8, 10.0], got %v", op.ProthrombinTimeRatio)
	}
	if op.BloodGlucoseMgDl < 15.0 || op.BloodGlucoseMgDl > 400.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 400.0], got %v", op.BloodGlucoseMgDl)
	}
	if op.TimeSinceIngestionMins < 1 || op.TimeSinceIngestionMins > 10080 {
		return fmt.Errorf("time_since_ingestion_mins must be in [1, 10080], got %d", op.TimeSinceIngestionMins)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
