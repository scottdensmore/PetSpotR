package domain

import "time"

// TriageCategory defines the emergency medical acuity tag.
type TriageCategory string

const (
	TriageCategoryRed    TriageCategory = "TRIAGE_RED"    // Immediate / Life-Threatening
	TriageCategoryYellow TriageCategory = "TRIAGE_YELLOW" // Delayed / Serious
	TriageCategoryGreen  TriageCategory = "TRIAGE_GREEN"  // Minor / Walking Wounded
	TriageCategoryBlack  TriageCategory = "TRIAGE_BLACK"  // Deceased / Expectant
)

// MucousMembraneColor represents the physical perfusion and oxygenation indicator.
type MucousMembraneColor string

const (
	MMColorPink     MucousMembraneColor = "MM_PINK"      // Normal perfusion
	MMColorPale     MucousMembraneColor = "MM_PALE"      // Shock, vasoconstriction, anemia
	MMColorCyanotic MucousMembraneColor = "MM_CYANOTIC"  // Severe hypoxia, respiratory distress
	MMColorIcteric  MucousMembraneColor = "MM_ICTERIC"   // Jaundice, hepatic dysfunction
	MMColorBrickRed MucousMembraneColor = "MM_BRICK_RED" // Sepsis, hyperthermia, vasodilation
)

// AllergySeverity defines the clinical consequence of allergen exposure.
type AllergySeverity string

const (
	AllergySeverityMild         AllergySeverity = "MILD"
	AllergySeverityModerate     AllergySeverity = "MODERATE"
	AllergySeverityAnaphylactic AllergySeverity = "ANAPHYLACTIC"
)

// ClinicalAllergy represents a diagnosed medication or environmental allergen.
type ClinicalAllergy struct {
	Allergen            string          `json:"allergen"`
	Severity            AllergySeverity `json:"severity"`
	ReactionDescription string          `json:"reactionDescription"`
}

// VaccinationRecord represents an immunization history entry.
type VaccinationRecord struct {
	VaccineName      string    `json:"vaccineName"`
	AdministeredDate time.Time `json:"administeredDate"`
	ExpirationDate   time.Time `json:"expirationDate"`
	BatchNumber      string    `json:"batchNumber,omitempty"`
	ClinicName       string    `json:"clinicName"`
	Verified         bool      `json:"verified"`
}

// ChronicCondition documents ongoing illnesses and active treatments.
type ChronicCondition struct {
	ConditionName string    `json:"conditionName"`
	DiagnosedDate time.Time `json:"diagnosedDate"`
	Medications   []string  `json:"medications"`
	CriticalFlag  bool      `json:"criticalFlag"`
}

// VitalSigns represents quantifiable physiological measurements.
type VitalSigns struct {
	HeartRateBPM       int                 `json:"heartRateBpm"`
	RespiratoryRateBPM int                 `json:"respiratoryRateBpm"`
	TemperatureF       float64             `json:"temperatureF"`
	CapillaryRefillSec float64             `json:"capillaryRefillSec"`
	MucousMembrane     MucousMembraneColor `json:"mucousMembrane"`
	GlasgowComaScale   int                 `json:"glasgowComaScale"` // 3 to 18
}

// TreatmentRoute defines the pharmacological administration pathway.
type TreatmentRoute string

const (
	RouteIV      TreatmentRoute = "IV"
	RouteIM      TreatmentRoute = "IM"
	RouteSC      TreatmentRoute = "SC"
	RoutePO      TreatmentRoute = "PO"
	RouteTopical TreatmentRoute = "TOPICAL"
)

// ClinicalTreatment records an emergency procedure or medication administered.
type ClinicalTreatment struct {
	TreatmentID    string         `json:"treatmentId"`
	MedicationName string         `json:"medicationName"`
	Dosage         string         `json:"dosage"`
	Route          TreatmentRoute `json:"route"`
	AdministeredBy string         `json:"administeredBy"`
	AdministeredAt time.Time      `json:"administeredAt"`
	Notes          string         `json:"notes,omitempty"`
}

// TriageAssessment records an emergency clinical evaluation in the field.
type TriageAssessment struct {
	AssessmentID           string              `json:"assessmentId"`
	PetID                  string              `json:"petId"`
	HubID                  string              `json:"hubId,omitempty"`
	MedicID                string              `json:"medicId"`
	MedicName              string              `json:"medicName"`
	Species                string              `json:"species"`
	WeightKg               float64             `json:"weightKg"`
	Category               TriageCategory      `json:"category"`
	Vitals                 VitalSigns          `json:"vitals"`
	TraumaNotes            string              `json:"traumaNotes"`
	AdministeredTreatments []ClinicalTreatment `json:"administeredTreatments"`
	AssessedAt             time.Time           `json:"assessedAt"`
	ReassessedAt           *time.Time          `json:"reassessedAt,omitempty"`
}

// VeterinaryPassport is the verifiable digital health record for an animal.
type VeterinaryPassport struct {
	PassportID        string              `json:"passportId"`
	PetID             string              `json:"petId"`
	PetName           string              `json:"petName"`
	Species           string              `json:"species"`
	Breed             string              `json:"breed"`
	MicrochipID       string              `json:"microchipId,omitempty"`
	RabiesTagID       string              `json:"rabiesTagId,omitempty"`
	BloodType         string              `json:"bloodType,omitempty"`
	WeightKg          float64             `json:"weightKg"`
	Vaccinations      []VaccinationRecord `json:"vaccinations"`
	Allergies         []ClinicalAllergy   `json:"allergies"`
	ChronicConditions []ChronicCondition  `json:"chronicConditions"`
	EmergencyContact  string              `json:"emergencyContact"`
	PrimaryClinic     string              `json:"primaryClinic"`
	PublicKeyHex      string              `json:"publicKeyHex"`
	SignatureHex      string              `json:"signatureHex"`
	IssuedAt          time.Time           `json:"issuedAt"`
	ExpiresAt         time.Time           `json:"expiresAt"`
}
