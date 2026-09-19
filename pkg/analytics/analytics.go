package analytics

import (
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// FilterOptions defines criteria for filtering analytics.
type FilterOptions struct {
	ShelterID string    `json:"shelterId,omitempty"`
	StartDate time.Time `json:"startDate,omitempty"`
	EndDate   time.Time `json:"endDate,omitempty"`
}

// ShelterKPISummary captures top-level recovery and intake metrics.
type ShelterKPISummary struct {
	TotalIntakes               int     `json:"totalIntakes"`
	ActiveInShelterCare        int     `json:"activeInShelterCare"`
	ReunitedCount              int     `json:"reunitedCount"`
	ReturnToOwnerRate          float64 `json:"returnToOwnerRate"` // ReunitedCount / TotalIntakes (0.0 to 1.0)
	MicrochippedCount          int     `json:"microchippedCount"`
	MicrochipScanRate          float64 `json:"microchipScanRate"` // MicrochippedCount / TotalIntakes (0.0 to 1.0)
	DeterministicMatchCount    int     `json:"deterministicMatchCount"`
	MultimodalMatchCount       int     `json:"multimodalMatchCount"`
	DeterministicMatchRatio    float64 `json:"deterministicMatchRatio"`    // Deterministic / (Deterministic + Multimodal)
	MedianIntakeToMatchHours   float64 `json:"medianIntakeToMatchHours"`   // Median hours from intake to match
	MedianIntakeToReunionHours float64 `json:"medianIntakeToReunionHours"` // Median hours from intake to reunion
}

// MunicipalShelterStats aggregates metrics for a single shelter.
type MunicipalShelterStats struct {
	ShelterID         string  `json:"shelterId"`
	ShelterName       string  `json:"shelterName"`
	IntakeCount       int     `json:"intakeCount"`
	ActiveCareCount   int     `json:"activeCareCount"`
	ReunitedCount     int     `json:"reunitedCount"`
	ReturnToOwnerRate float64 `json:"returnToOwnerRate"`
	MicrochipScanRate float64 `json:"microchipScanRate"`
}

// ShelterInfo provides identity metadata for shelter filter pickers.
type ShelterInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AnalyticsReport is the complete aggregated analytics payload.
type AnalyticsReport struct {
	GeneratedAt       time.Time               `json:"generatedAt"`
	Filter            FilterOptions           `json:"filter"`
	OverallKPIs       ShelterKPISummary       `json:"overallKpis"`
	ShelterBreakdown  []MunicipalShelterStats `json:"shelterBreakdown"`
	AvailableShelters []ShelterInfo           `json:"availableShelters"`
}

func isShelterIntake(pet domain.FoundPetRecord) bool {
	return pet.CustodyStatus == domain.CustodyShelterCare || strings.TrimSpace(pet.ShelterID) != ""
}

func calculateMedian(durations []float64) float64 {
	if len(durations) == 0 {
		return 0.0
	}
	sorted := make([]float64, len(durations))
	copy(sorted, durations)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2.0
}

type intakeData struct {
	record    domain.FoundPetRecord
	matches   []domain.MatchRecord
	reunited  bool
	reunionAt time.Time
}

type shelterAgg struct {
	id         string
	name       string
	total      int
	active     int
	reunited   int
	microchips int
}

// ComputeAnalytics aggregates shelter operations KPIs, turnaround velocities,
// per-shelter breakdowns, and available shelter picker metadata.
func ComputeAnalytics(foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) AnalyticsReport {
	// Extract available shelters across all shelter intakes in the dataset
	shelterMap := make(map[string]string)
	for _, pet := range foundPets {
		if !isShelterIntake(pet) {
			continue
		}
		shelterID := strings.TrimSpace(pet.ShelterID)
		if shelterID == "" {
			continue
		}
		name := strings.TrimSpace(pet.ShelterName)
		if name == "" {
			name = shelterID
		}
		if existing, ok := shelterMap[shelterID]; !ok || (existing == shelterID && name != shelterID) {
			shelterMap[shelterID] = name
		}
	}

	availableShelters := make([]ShelterInfo, 0, len(shelterMap))
	for id, name := range shelterMap {
		availableShelters = append(availableShelters, ShelterInfo{
			ID:   id,
			Name: name,
		})
	}
	sort.Slice(availableShelters, func(i, j int) bool {
		if availableShelters[i].Name == availableShelters[j].Name {
			return availableShelters[i].ID < availableShelters[j].ID
		}
		return availableShelters[i].Name < availableShelters[j].Name
	})

	// Filter intakes based on shelter scope, shelter filter, and date window
	var filteredIntakes []domain.FoundPetRecord
	intakeDataMap := make(map[string]*intakeData)

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

		filteredIntakes = append(filteredIntakes, pet)
		data := &intakeData{
			record: pet,
		}
		if pet.Status == domain.FoundPetStatusResolved {
			data.reunited = true
			if pet.LifecycleAudit != nil && !pet.LifecycleAudit.ChangedAt.IsZero() {
				data.reunionAt = pet.LifecycleAudit.ChangedAt
			}
		}
		intakeDataMap[pet.PetID] = data
	}

	// Link and evaluate matches belonging to filtered intakes
	var (
		deterministicMatchCount int
		multimodalMatchCount    int
		matchDurations          []float64
	)

	for _, m := range matches {
		data, ok := intakeDataMap[m.FoundPetID]
		if !ok {
			continue
		}
		data.matches = append(data.matches, m)

		if m.DeterministicMatch {
			deterministicMatchCount++
		} else {
			multimodalMatchCount++
		}

		if !m.MatchedAt.IsZero() && !data.record.FoundAt.IsZero() {
			dur := m.MatchedAt.Sub(data.record.FoundAt).Hours()
			if dur < 0 {
				dur = 0
			}
			matchDurations = append(matchDurations, dur)
		}

		if m.Status == domain.MatchStatusReunited || m.Status == domain.MatchStatusConfirmed {
			data.reunited = true
			if data.reunionAt.IsZero() || (!m.MatchedAt.IsZero() && m.MatchedAt.Before(data.reunionAt)) {
				data.reunionAt = m.MatchedAt
			}
		}
	}

	// Calculate overall KPIs
	totalIntakes := len(filteredIntakes)
	var (
		activeInShelterCare int
		reunitedCount       int
		microchippedCount   int
		reunionDurations    []float64
	)

	for _, pet := range filteredIntakes {
		data := intakeDataMap[pet.PetID]
		if data.reunited {
			reunitedCount++
			if !data.reunionAt.IsZero() && !pet.FoundAt.IsZero() {
				dur := data.reunionAt.Sub(pet.FoundAt).Hours()
				if dur < 0 {
					dur = 0
				}
				reunionDurations = append(reunionDurations, dur)
			}
		} else if pet.Status.IsActive() {
			activeInShelterCare++
		}

		if strings.TrimSpace(pet.MicrochipID) != "" {
			microchippedCount++
		}
	}

	var rtoRate float64
	var scanRate float64
	if totalIntakes > 0 {
		rtoRate = float64(reunitedCount) / float64(totalIntakes)
		scanRate = float64(microchippedCount) / float64(totalIntakes)
	}

	var deterministicRatio float64
	totalMatches := deterministicMatchCount + multimodalMatchCount
	if totalMatches > 0 {
		deterministicRatio = float64(deterministicMatchCount) / float64(totalMatches)
	}

	// Calculate per-shelter breakdown
	shelterAggMap := make(map[string]*shelterAgg)
	for _, pet := range filteredIntakes {
		sID := strings.TrimSpace(pet.ShelterID)
		if sID == "" {
			continue
		}
		agg, ok := shelterAggMap[sID]
		if !ok {
			sName := strings.TrimSpace(pet.ShelterName)
			if sName == "" {
				sName = sID
			}
			agg = &shelterAgg{
				id:   sID,
				name: sName,
			}
			shelterAggMap[sID] = agg
		} else if agg.name == agg.id && strings.TrimSpace(pet.ShelterName) != "" {
			agg.name = strings.TrimSpace(pet.ShelterName)
		}

		agg.total++
		data := intakeDataMap[pet.PetID]
		if data.reunited {
			agg.reunited++
		} else if pet.Status.IsActive() {
			agg.active++
		}
		if strings.TrimSpace(pet.MicrochipID) != "" {
			agg.microchips++
		}
	}

	breakdown := make([]MunicipalShelterStats, 0, len(shelterAggMap))
	for _, agg := range shelterAggMap {
		var sRtoRate float64
		var sScanRate float64
		if agg.total > 0 {
			sRtoRate = float64(agg.reunited) / float64(agg.total)
			sScanRate = float64(agg.microchips) / float64(agg.total)
		}
		breakdown = append(breakdown, MunicipalShelterStats{
			ShelterID:         agg.id,
			ShelterName:       agg.name,
			IntakeCount:       agg.total,
			ActiveCareCount:   agg.active,
			ReunitedCount:     agg.reunited,
			ReturnToOwnerRate: sRtoRate,
			MicrochipScanRate: sScanRate,
		})
	}
	sort.Slice(breakdown, func(i, j int) bool {
		if breakdown[i].ShelterName == breakdown[j].ShelterName {
			return breakdown[i].ShelterID < breakdown[j].ShelterID
		}
		return breakdown[i].ShelterName < breakdown[j].ShelterName
	})

	return AnalyticsReport{
		GeneratedAt: time.Now().UTC(),
		Filter:      filter,
		OverallKPIs: ShelterKPISummary{
			TotalIntakes:               totalIntakes,
			ActiveInShelterCare:        activeInShelterCare,
			ReunitedCount:              reunitedCount,
			ReturnToOwnerRate:          rtoRate,
			MicrochippedCount:          microchippedCount,
			MicrochipScanRate:          scanRate,
			DeterministicMatchCount:    deterministicMatchCount,
			MultimodalMatchCount:       multimodalMatchCount,
			DeterministicMatchRatio:    deterministicRatio,
			MedianIntakeToMatchHours:   calculateMedian(matchDurations),
			MedianIntakeToReunionHours: calculateMedian(reunionDurations),
		},
		ShelterBreakdown:  breakdown,
		AvailableShelters: availableShelters,
	}
}
