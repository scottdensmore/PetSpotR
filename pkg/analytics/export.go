package analytics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/microchip"
)

// GeoJSONGeometry represents an RFC 7946 geometry object.
type GeoJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// GeoJSONProperties holds the intake and reconciliation properties for a GeoJSON feature.
type GeoJSONProperties struct {
	PetID             string `json:"petId"`
	IntakeID          string `json:"intakeId"`
	ShelterID         string `json:"shelterId"`
	ShelterName       string `json:"shelterName"`
	Species           string `json:"species"`
	Breed             string `json:"breed"`
	CustodyStatus     string `json:"custodyStatus"`
	MicrochipStatus   string `json:"microchipStatus"`
	MaskedMicrochip   string `json:"maskedMicrochip"`
	MicrochipRegistry string `json:"microchipRegistry"`
	MatchStatus       string `json:"matchStatus"`
	IntakeDate        string `json:"intakeDate"`
}

// GeoJSONFeature represents an RFC 7946 Feature.
type GeoJSONFeature struct {
	Type       string            `json:"type"`
	Geometry   GeoJSONGeometry   `json:"geometry"`
	Properties GeoJSONProperties `json:"properties"`
}

// GeoJSONFeatureCollection represents an RFC 7946 FeatureCollection.
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

func filterIntakes(foundPets []domain.FoundPetRecord, filter FilterOptions) []domain.FoundPetRecord {
	var filtered []domain.FoundPetRecord
	for _, pet := range foundPets {
		if !isShelterIntake(pet) {
			continue
		}
		if filter.ShelterID != "" && pet.ShelterID != filter.ShelterID {
			continue
		}
		if !filter.StartDate.IsZero() && pet.FoundAt.Before(filter.StartDate) {
			continue
		}
		if !filter.EndDate.IsZero() && pet.FoundAt.After(filter.EndDate) {
			continue
		}
		filtered = append(filtered, pet)
	}
	return filtered
}

func matchStatusPriority(status domain.MatchStatus) int {
	switch status {
	case domain.MatchStatusReunited:
		return 4
	case domain.MatchStatusConfirmed:
		return 3
	case domain.MatchStatusPendingReview:
		return 2
	case domain.MatchStatusRejected:
		return 1
	default:
		return 0
	}
}

func isBetterMatch(candidate, current domain.MatchRecord) bool {
	cp := matchStatusPriority(candidate.Status)
	currp := matchStatusPriority(current.Status)
	if cp > currp {
		return true
	}
	if cp < currp {
		return false
	}
	if candidate.Score > current.Score {
		return true
	}
	if candidate.Score < current.Score {
		return false
	}
	return candidate.MatchedAt.After(current.MatchedAt)
}

func mapBestMatches(matches []domain.MatchRecord) map[string]domain.MatchRecord {
	best := make(map[string]domain.MatchRecord)
	for _, m := range matches {
		if current, exists := best[m.FoundPetID]; exists {
			if isBetterMatch(m, current) {
				best[m.FoundPetID] = m
			}
		} else {
			best[m.FoundPetID] = m
		}
	}
	return best
}

func resolveMicrochipInfo(pet domain.FoundPetRecord) (status, masked, registry string) {
	chip := strings.TrimSpace(pet.MicrochipID)
	if chip == "" {
		return "Unverified", "", ""
	}
	val := microchip.ValidateAndNormalize(chip)
	if !val.Valid {
		return "Unverified", "", ""
	}

	masked = microchip.MaskMicrochip(chip)
	regName := strings.TrimSpace(pet.MicrochipRegistry)
	if regName == "" {
		regInfo := microchip.IdentifyIssuingRegistry(val.NormalizedID)
		regName = regInfo.RegistryName
	}
	return "Verified", masked, regName
}

func resolveReunionDate(pet domain.FoundPetRecord, match *domain.MatchRecord) string {
	var reunionAt time.Time
	if pet.Status == domain.FoundPetStatusResolved && pet.LifecycleAudit != nil && !pet.LifecycleAudit.ChangedAt.IsZero() {
		reunionAt = pet.LifecycleAudit.ChangedAt
	}
	if match != nil && (match.Status == domain.MatchStatusReunited || match.Status == domain.MatchStatusConfirmed) {
		if reunionAt.IsZero() || (!match.MatchedAt.IsZero() && match.MatchedAt.Before(reunionAt)) {
			reunionAt = match.MatchedAt
		}
	}
	if reunionAt.IsZero() {
		return ""
	}
	return reunionAt.UTC().Format(time.RFC3339)
}

// ExportReconciliationCSV streams RFC 4180 CSV reconciliation rows for filtered shelter intakes.
func ExportReconciliationCSV(w io.Writer, foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) error {
	filtered := filterIntakes(foundPets, filter)
	bestMatches := mapBestMatches(matches)

	csvWriter := csv.NewWriter(w)

	header := []string{
		"IntakeID",
		"ShelterID",
		"ShelterName",
		"ReportedDate",
		"Species",
		"Breed",
		"PrimaryColor",
		"CustodyStatus",
		"MicrochipStatus",
		"MaskedMicrochip",
		"MicrochipRegistry",
		"MatchStatus",
		"MatchType",
		"MatchScore",
		"ReunitedDate",
	}
	if err := csvWriter.Write(header); err != nil {
		return err
	}

	for _, pet := range filtered {
		var reportedDate string
		if !pet.FoundAt.IsZero() {
			reportedDate = pet.FoundAt.UTC().Format(time.RFC3339)
		}

		mcStatus, mcMasked, mcReg := resolveMicrochipInfo(pet)

		var matchStatus, matchType, matchScore string
		var matchPtr *domain.MatchRecord
		if m, hasMatch := bestMatches[pet.PetID]; hasMatch {
			matchPtr = &m
			matchStatus = string(m.Status)
			matchType = m.MatchType
			matchScore = fmt.Sprintf("%.2f", m.Score)
		}

		reunitedDate := resolveReunionDate(pet, matchPtr)

		row := []string{
			pet.IntakeID,
			pet.ShelterID,
			pet.ShelterName,
			reportedDate,
			pet.Species,
			pet.Breed,
			pet.PrimaryColor,
			string(pet.CustodyStatus),
			mcStatus,
			mcMasked,
			mcReg,
			matchStatus,
			matchType,
			matchScore,
			reunitedDate,
		}

		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

// ExportReconciliationGeoJSON streams an RFC 7946 FeatureCollection of Point geometries for filtered shelter intakes.
func ExportReconciliationGeoJSON(w io.Writer, foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) error {
	filtered := filterIntakes(foundPets, filter)
	bestMatches := mapBestMatches(matches)

	features := make([]GeoJSONFeature, 0)
	for _, pet := range filtered {
		if pet.Coordinates == nil || pet.Coordinates.Validate() != nil {
			continue
		}

		var intakeDate string
		if !pet.FoundAt.IsZero() {
			intakeDate = pet.FoundAt.UTC().Format(time.RFC3339)
		}

		mcStatus, mcMasked, mcReg := resolveMicrochipInfo(pet)

		var matchStatus string
		if m, hasMatch := bestMatches[pet.PetID]; hasMatch {
			matchStatus = string(m.Status)
		}

		feat := GeoJSONFeature{
			Type: "Feature",
			Geometry: GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{pet.Coordinates.Longitude, pet.Coordinates.Latitude},
			},
			Properties: GeoJSONProperties{
				PetID:             pet.PetID,
				IntakeID:          pet.IntakeID,
				ShelterID:         pet.ShelterID,
				ShelterName:       pet.ShelterName,
				Species:           pet.Species,
				Breed:             pet.Breed,
				CustodyStatus:     string(pet.CustodyStatus),
				MicrochipStatus:   mcStatus,
				MaskedMicrochip:   mcMasked,
				MicrochipRegistry: mcReg,
				MatchStatus:       matchStatus,
				IntakeDate:        intakeDate,
			},
		}
		features = append(features, feat)
	}

	fc := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}

	encoder := json.NewEncoder(w)
	return encoder.Encode(fc)
}
