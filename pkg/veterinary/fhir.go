package veterinary

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const fhirPatientAnimalExtensionURL = "http://hl7.org/fhir/StructureDefinition/patient-animal"

// ComputeFHIRBundleDigest returns a deterministic SHA-256 hex digest over FHIR bundle entries,
// normalized via JSON round-trip so in-memory and unmarshaled bundles produce identical digests.
func ComputeFHIRBundleDigest(entries []domain.FHIRResourceEntry) (string, error) {
	raw, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal fhir entries: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return "", fmt.Errorf("unmarshal fhir entries for normalization: %w", err)
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal normalized fhir entries: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// BuildFHIRVeterinaryBundle constructs an HL7 FHIR R4 document bundle from a field triage assessment,
// optional veterinary passport, and target receiving veterinary clinic.
func BuildFHIRVeterinaryBundle(
	assessment domain.TriageAssessment,
	passport *domain.VeterinaryPassport,
	clinic domain.VeterinaryClinic,
) (domain.FHIRVeterinaryBundle, error) {
	if strings.TrimSpace(assessment.PetID) == "" {
		return domain.FHIRVeterinaryBundle{}, errors.New("assessment petId is required")
	}

	petName := assessment.PetID
	if passport != nil && strings.TrimSpace(passport.PetName) != "" {
		petName = passport.PetName
	}

	species := strings.TrimSpace(assessment.Species)
	if species == "" && passport != nil && strings.TrimSpace(passport.Species) != "" {
		species = passport.Species
	}
	if species == "" {
		species = "Unknown"
	}

	breed := "Unknown"
	if passport != nil && strings.TrimSpace(passport.Breed) != "" {
		breed = passport.Breed
	}

	identifiers := []map[string]any{
		{
			"system": "urn:petspotr:pet-id",
			"value":  assessment.PetID,
		},
	}
	if passport != nil && strings.TrimSpace(passport.MicrochipID) != "" {
		identifiers = append(identifiers, map[string]any{
			"system": "urn:iso:std:iso:11784:11785",
			"value":  passport.MicrochipID,
		})
	}
	if passport != nil && strings.TrimSpace(passport.RabiesTagID) != "" {
		identifiers = append(identifiers, map[string]any{
			"system": "urn:petspotr:rabies-tag",
			"value":  passport.RabiesTagID,
		})
	}

	patientRef := "urn:uuid:patient-" + assessment.PetID
	entries := make([]domain.FHIRResourceEntry, 0, 12)

	// 1. Patient resource with HL7 patient-animal extension
	entries = append(entries, domain.FHIRResourceEntry{
		FullURL: patientRef,
		Resource: map[string]any{
			"resourceType": "Patient",
			"id":           assessment.PetID,
			"name": []map[string]any{
				{"text": petName},
			},
			"identifier": identifiers,
			"extension": []map[string]any{
				{
					"url": fhirPatientAnimalExtensionURL,
					"extension": []map[string]any{
						{
							"url": "species",
							"valueCodeableConcept": map[string]any{
								"text": species,
							},
						},
						{
							"url": "breed",
							"valueCodeableConcept": map[string]any{
								"text": breed,
							},
						},
					},
				},
			},
		},
	})

	// 2. Encounter resource
	entries = append(entries, domain.FHIRResourceEntry{
		FullURL: "urn:uuid:encounter-" + assessment.AssessmentID,
		Resource: map[string]any{
			"resourceType": "Encounter",
			"id":           assessment.AssessmentID,
			"status":       "in-progress",
			"class": map[string]any{
				"system":  "http://terminology.hl7.org/CodeSystem/v3-ActCode",
				"code":    "EMER",
				"display": "emergency",
			},
			"priority": map[string]any{
				"text": string(assessment.Category),
			},
			"subject": map[string]any{
				"reference": patientRef,
			},
			"serviceProvider": map[string]any{
				"display": clinic.Name,
				"identifier": map[string]any{
					"value": clinic.ClinicID,
				},
			},
			"note": []map[string]any{
				{"text": assessment.TraumaNotes},
			},
		},
	})

	// 3. Observation resources for physiological vitals
	entries = append(entries, buildLOINCObservation(
		"urn:uuid:obs-weight-"+assessment.AssessmentID,
		patientRef,
		"29463-7",
		"Body weight",
		assessment.WeightKg,
		"kg",
	))

	if assessment.Vitals.HeartRateBPM > 0 || assessment.Vitals.HeartRateAssessed {
		entries = append(entries, buildLOINCObservation(
			"urn:uuid:obs-hr-"+assessment.AssessmentID,
			patientRef,
			"8867-4",
			"Heart rate",
			assessment.Vitals.HeartRateBPM,
			"/min",
		))
	}

	if assessment.Vitals.RespiratoryRateBPM > 0 || assessment.Vitals.RespRateAssessed {
		entries = append(entries, buildLOINCObservation(
			"urn:uuid:obs-rr-"+assessment.AssessmentID,
			patientRef,
			"9279-1",
			"Respiratory rate",
			assessment.Vitals.RespiratoryRateBPM,
			"/min",
		))
	}

	if assessment.Vitals.TemperatureF > 0 {
		entries = append(entries, buildLOINCObservation(
			"urn:uuid:obs-temp-"+assessment.AssessmentID,
			patientRef,
			"8310-5",
			"Body temperature",
			assessment.Vitals.TemperatureF,
			"[degF]",
		))
	}

	entries = append(entries, domain.FHIRResourceEntry{
		FullURL: "urn:uuid:obs-panel-" + assessment.AssessmentID,
		Resource: map[string]any{
			"resourceType": "Observation",
			"status":       "final",
			"code": map[string]any{
				"text": "Perfusion & Neurologic Triage Panel",
			},
			"subject": map[string]any{
				"reference": patientRef,
			},
			"valueString": fmt.Sprintf(
				"CRT: %.1fs | MM: %s | GCS: %d",
				assessment.Vitals.CapillaryRefillSec,
				assessment.Vitals.MucousMembrane,
				assessment.Vitals.GlasgowComaScale,
			),
		},
	})

	// 4. AllergyIntolerance & 5. Immunization resources (if passport is present)
	if passport != nil {
		for i, allergy := range passport.Allergies {
			criticality := "low"
			if allergy.Severity == domain.AllergySeverityAnaphylactic {
				criticality = "high"
			}
			entries = append(entries, domain.FHIRResourceEntry{
				FullURL: fmt.Sprintf("urn:uuid:allergy-%s-%d", assessment.PetID, i+1),
				Resource: map[string]any{
					"resourceType": "AllergyIntolerance",
					"clinicalStatus": map[string]any{
						"text": "active",
					},
					"criticality": criticality,
					"code": map[string]any{
						"text": allergy.Allergen,
					},
					"patient": map[string]any{
						"reference": patientRef,
					},
					"note": []map[string]any{
						{"text": allergy.ReactionDescription},
					},
				},
			})
		}

		for i, vax := range passport.Vaccinations {
			entries = append(entries, domain.FHIRResourceEntry{
				FullURL: fmt.Sprintf("urn:uuid:immunization-%s-%d", assessment.PetID, i+1),
				Resource: map[string]any{
					"resourceType": "Immunization",
					"status":       "completed",
					"vaccineCode": map[string]any{
						"text": vax.VaccineName,
					},
					"patient": map[string]any{
						"reference": patientRef,
					},
					"occurrenceDateTime": vax.AdministeredDate.UTC().Format(time.RFC3339),
				},
			})
		}
	}

	// 6. MedicationAdministration resources
	for i, tx := range assessment.AdministeredTreatments {
		txID := tx.TreatmentID
		if strings.TrimSpace(txID) == "" {
			txID = fmt.Sprintf("tx-%d", i+1)
		}
		entries = append(entries, domain.FHIRResourceEntry{
			FullURL: "urn:uuid:medadmin-" + txID,
			Resource: map[string]any{
				"resourceType": "MedicationAdministration",
				"id":           tx.TreatmentID,
				"status":       "completed",
				"medicationCodeableConcept": map[string]any{
					"text": tx.MedicationName,
				},
				"subject": map[string]any{
					"reference": patientRef,
				},
				"dosage": map[string]any{
					"text": fmt.Sprintf("%s (%s)", tx.Dosage, tx.Route),
				},
			},
		})
	}

	digestHex, err := ComputeFHIRBundleDigest(entries)
	if err != nil {
		return domain.FHIRVeterinaryBundle{}, err
	}

	return domain.FHIRVeterinaryBundle{
		ResourceType:    "Bundle",
		ID:              "fhir-bundle-" + assessment.AssessmentID,
		Type:            "document",
		Timestamp:       assessment.AssessedAt.UTC(),
		BundleDigestHex: digestHex,
		Entry:           entries,
	}, nil
}

func buildLOINCObservation(fullURL, patientRef, loincCode, display string, value any, unit string) domain.FHIRResourceEntry {
	return domain.FHIRResourceEntry{
		FullURL: fullURL,
		Resource: map[string]any{
			"resourceType": "Observation",
			"status":       "final",
			"code": map[string]any{
				"coding": []map[string]any{
					{
						"system":  "http://loinc.org",
						"code":    loincCode,
						"display": display,
					},
				},
				"text": display,
			},
			"subject": map[string]any{
				"reference": patientRef,
			},
			"valueQuantity": map[string]any{
				"value":  value,
				"unit":   unit,
				"system": "http://unitsofmeasure.org",
			},
		},
	}
}

// ValidateFHIRVeterinaryBundle validates the structure, veterinary Patient extension,
// and SHA-256 cryptographic digest of an HL7 FHIR R4 veterinary bundle.
func ValidateFHIRVeterinaryBundle(bundle domain.FHIRVeterinaryBundle) error {
	if bundle.ResourceType != "Bundle" {
		return fmt.Errorf("invalid FHIR bundle resourceType %q: expected Bundle", bundle.ResourceType)
	}
	if bundle.Type != "document" {
		return fmt.Errorf("invalid FHIR bundle type %q: expected document", bundle.Type)
	}
	if len(bundle.Entry) < 2 {
		return errors.New("FHIR bundle must contain at least 2 entries (Patient and Encounter)")
	}

	firstResource := bundle.Entry[0].Resource
	if firstResource == nil {
		return errors.New("first FHIR bundle entry resource is nil")
	}
	firstType, _ := firstResource["resourceType"].(string)
	if firstType != "Patient" {
		return fmt.Errorf("first FHIR bundle entry must be Patient, got %q", firstType)
	}
	if !hasPatientAnimalExtension(firstResource["extension"]) {
		return fmt.Errorf("Patient resource missing required %s extension", fhirPatientAnimalExtensionURL)
	}

	if strings.TrimSpace(bundle.BundleDigestHex) == "" {
		return errors.New("FHIR bundle digest is required")
	}
	computedDigest, err := ComputeFHIRBundleDigest(bundle.Entry)
	if err != nil {
		return fmt.Errorf("compute FHIR bundle digest: %w", err)
	}
	if computedDigest != bundle.BundleDigestHex {
		return fmt.Errorf("FHIR bundle digest mismatch: computed %s != expected %s", computedDigest, bundle.BundleDigestHex)
	}

	return nil
}

func hasPatientAnimalExtension(rawExt any) bool {
	switch exts := rawExt.(type) {
	case []map[string]any:
		for _, ext := range exts {
			if url, _ := ext["url"].(string); url == fhirPatientAnimalExtensionURL {
				return true
			}
		}
	case []any:
		for _, item := range exts {
			if ext, ok := item.(map[string]any); ok {
				if url, _ := ext["url"].(string); url == fhirPatientAnimalExtensionURL {
					return true
				}
			}
		}
	}
	return false
}

// BuildPIMSIntakeRecord flattens clinical assessment, passport, and clinic details into
// a PIMS-compatible intake DTO for Cornerstone, ezyVet, AVImark, and Shelterluv.
func BuildPIMSIntakeRecord(
	handoffID string,
	assessment domain.TriageAssessment,
	passport *domain.VeterinaryPassport,
	clinic domain.VeterinaryClinic,
) domain.PIMSIntakeRecord {
	petName := assessment.PetID
	if passport != nil && strings.TrimSpace(passport.PetName) != "" {
		petName = passport.PetName
	}

	species := strings.TrimSpace(assessment.Species)
	if species == "" && passport != nil && strings.TrimSpace(passport.Species) != "" {
		species = passport.Species
	}
	if species == "" {
		species = "Unknown"
	}

	breed := "Unknown"
	if passport != nil && strings.TrimSpace(passport.Breed) != "" {
		breed = passport.Breed
	}

	weightKg := assessment.WeightKg
	if weightKg <= 0 && passport != nil && passport.WeightKg > 0 {
		weightKg = passport.WeightKg
	}

	microchipID := ""
	if passport != nil {
		microchipID = passport.MicrochipID
	}

	criticalAllergies := make([]string, 0)
	if passport != nil {
		for _, allergy := range passport.Allergies {
			criticalAllergies = append(criticalAllergies, fmt.Sprintf("%s (%s)", allergy.Allergen, allergy.Severity))
		}
	}

	treatmentsGiven := make([]string, 0, len(assessment.AdministeredTreatments))
	for _, tx := range assessment.AdministeredTreatments {
		treatmentsGiven = append(treatmentsGiven, fmt.Sprintf("%s %s %s", tx.MedicationName, tx.Dosage, tx.Route))
	}

	return domain.PIMSIntakeRecord{
		HandoffID:          handoffID,
		ClinicID:           clinic.ClinicID,
		ClinicName:         clinic.Name,
		PIMS:               clinic.PIMS,
		PetID:              assessment.PetID,
		PetName:            petName,
		Species:            species,
		Breed:              breed,
		WeightKg:           weightKg,
		MicrochipID:        microchipID,
		TriageCategory:     assessment.Category,
		HeartRateBPM:       assessment.Vitals.HeartRateBPM,
		RespiratoryRateBPM: assessment.Vitals.RespiratoryRateBPM,
		TemperatureF:       assessment.Vitals.TemperatureF,
		CriticalAllergies:  criticalAllergies,
		TreatmentsGiven:    treatmentsGiven,
	}
}

// FormatPIMSRecordCSV serializes a slice of PIMS intake records as an RFC 4180 CSV document.
func FormatPIMSRecordCSV(records []domain.PIMSIntakeRecord) (string, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)

	header := []string{
		"handoff_id",
		"clinic_id",
		"clinic_name",
		"pims_provider",
		"pet_id",
		"pet_name",
		"species",
		"breed",
		"weight_kg",
		"microchip_id",
		"triage_category",
		"heart_rate_bpm",
		"respiratory_rate_bpm",
		"temperature_f",
		"critical_allergies",
		"treatments_given",
		"attending_vet",
		"discharge_summary",
	}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, r := range records {
		row := []string{
			r.HandoffID,
			r.ClinicID,
			r.ClinicName,
			string(r.PIMS),
			r.PetID,
			r.PetName,
			r.Species,
			r.Breed,
			fmt.Sprintf("%.2f", r.WeightKg),
			r.MicrochipID,
			string(r.TriageCategory),
			fmt.Sprintf("%d", r.HeartRateBPM),
			fmt.Sprintf("%d", r.RespiratoryRateBPM),
			fmt.Sprintf("%.1f", r.TemperatureF),
			strings.Join(r.CriticalAllergies, "; "),
			strings.Join(r.TreatmentsGiven, "; "),
			r.AttendingVet,
			r.DischargeSummary,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
