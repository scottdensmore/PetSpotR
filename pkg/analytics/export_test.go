package analytics_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/analytics"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestExportReconciliationCSV(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	foundPets := []domain.FoundPetRecord{
		{
			PetID:             "found-1",
			ShelterID:         "shelter-sea-01",
			ShelterName:       "Seattle Animal Shelter",
			IntakeID:          "INT-8819",
			CustodyStatus:     domain.CustodyShelterCare,
			Status:            domain.FoundPetStatusFound,
			FoundAt:           now.Add(-24 * time.Hour),
			Species:           "Dog",
			Breed:             "Golden Retriever",
			PrimaryColor:      "Golden",
			MicrochipID:       "985141000123456",
			MicrochipRegistry: "HomeAgain",
		},
	}

	matches := []domain.MatchRecord{
		{
			MatchID:            "match-1",
			FoundPetID:         "found-1",
			MatchedPetID:       "lost-1",
			Score:              1.0,
			Status:             domain.MatchStatusConfirmed,
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedAt:          now.Add(-20 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := analytics.ExportReconciliationCSV(&buf, foundPets, matches, analytics.FilterOptions{})
	if err != nil {
		t.Fatalf("ExportReconciliationCSV failed: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 CSV rows (1 header + 1 record), got %d", len(records))
	}

	header := records[0]
	if header[0] != "IntakeID" || header[9] != "MaskedMicrochip" {
		t.Errorf("unexpected CSV headers: %v", header)
	}

	row := records[1]
	if row[0] != "INT-8819" {
		t.Errorf("expected IntakeID INT-8819, got %s", row[0])
	}
	// Zero-PII assertion
	if !strings.Contains(row[9], "••••3456") || strings.Contains(row[9], "985141000123456") {
		t.Errorf("expected masked microchip, got %s", row[9])
	}
	if row[11] != "CONFIRMED" {
		t.Errorf("expected MatchStatus CONFIRMED, got %s", row[11])
	}
}

func TestExportReconciliationCSV_FiltersAndMatchResolution(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	foundPets := []domain.FoundPetRecord{
		{
			PetID:         "found-sea-1",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-101",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-48 * time.Hour),
			Species:       "Dog",
			Breed:         "Labrador",
			PrimaryColor:  "Black",
			MicrochipID:   "985141000111111",
		},
		{
			PetID:         "found-sea-2",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-102",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-24 * time.Hour),
			Species:       "Cat",
			Breed:         "Domestic Shorthair",
			PrimaryColor:  "Calico",
		},
		{
			PetID:         "found-bel-1",
			ShelterID:     "shelter-bel-02",
			ShelterName:   "Bellevue Humane",
			IntakeID:      "BEL-201",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-12 * time.Hour),
			Species:       "Dog",
			Breed:         "Poodle",
			PrimaryColor:  "White",
		},
		{
			PetID:         "found-non-shelter",
			ShelterID:     "",
			CustodyStatus: domain.CustodyFinderHome,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-10 * time.Hour),
			Species:       "Dog",
		},
	}

	matches := []domain.MatchRecord{
		// Matches for found-sea-1 to test resolution priority
		{
			MatchID:      "match-low-pending",
			FoundPetID:   "found-sea-1",
			MatchedPetID: "lost-p1",
			Score:        0.95,
			Status:       domain.MatchStatusPendingReview,
			MatchType:    "multimodal_visual",
			MatchedAt:    now.Add(-40 * time.Hour),
		},
		{
			MatchID:      "match-best-reunited",
			FoundPetID:   "found-sea-1",
			MatchedPetID: "lost-r1",
			Score:        0.90,
			Status:       domain.MatchStatusReunited,
			MatchType:    "deterministic_microchip",
			MatchedAt:    now.Add(-30 * time.Hour),
		},
		{
			MatchID:      "match-confirmed-higher-score",
			FoundPetID:   "found-sea-1",
			MatchedPetID: "lost-c1",
			Score:        0.99,
			Status:       domain.MatchStatusConfirmed,
			MatchType:    "deterministic_microchip",
			MatchedAt:    now.Add(-35 * time.Hour),
		},
	}

	t.Run("Filters by ShelterID and handles unmatched pet", func(t *testing.T) {
		var buf bytes.Buffer
		err := analytics.ExportReconciliationCSV(&buf, foundPets, matches, analytics.FilterOptions{
			ShelterID: "shelter-sea-01",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader := csv.NewReader(&buf)
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV: %v", err)
		}

		// 1 header + 2 Seattle shelter records (non-shelter and Bellevue excluded)
		if len(records) != 3 {
			t.Fatalf("expected 3 CSV rows, got %d", len(records))
		}

		// Record 1: found-sea-1 (best match should be REUNITED over CONFIRMED/PENDING)
		row1 := records[1]
		if row1[0] != "INT-101" {
			t.Errorf("row1 expected IntakeID INT-101, got %s", row1[0])
		}
		if row1[8] != "Verified" {
			t.Errorf("row1 expected MicrochipStatus Verified, got %s", row1[8])
		}
		if row1[11] != "REUNITED" {
			t.Errorf("row1 expected MatchStatus REUNITED, got %s", row1[11])
		}
		if row1[12] != "deterministic_microchip" {
			t.Errorf("row1 expected MatchType deterministic_microchip, got %s", row1[12])
		}
		if row1[13] != "0.90" {
			t.Errorf("row1 expected MatchScore 0.90, got %s", row1[13])
		}
		if row1[14] == "" {
			t.Errorf("row1 expected non-empty ReunitedDate")
		}

		// Record 2: found-sea-2 (no microchip, no match)
		row2 := records[2]
		if row2[0] != "INT-102" {
			t.Errorf("row2 expected IntakeID INT-102, got %s", row2[0])
		}
		if row2[8] != "Unverified" {
			t.Errorf("row2 expected MicrochipStatus Unverified, got %s", row2[8])
		}
		if row2[9] != "" {
			t.Errorf("row2 expected blank MaskedMicrochip, got %s", row2[9])
		}
		if row2[10] != "" {
			t.Errorf("row2 expected blank MicrochipRegistry, got %s", row2[10])
		}
		if row2[11] != "" || row2[12] != "" || row2[13] != "" || row2[14] != "" {
			t.Errorf("row2 expected blank match fields, got %v", row2[11:])
		}
	})

	t.Run("Filters by Date range", func(t *testing.T) {
		var buf bytes.Buffer
		err := analytics.ExportReconciliationCSV(&buf, foundPets, matches, analytics.FilterOptions{
			StartDate: now.Add(-30 * time.Hour),
			EndDate:   now.Add(-15 * time.Hour),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reader := csv.NewReader(&buf)
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV: %v", err)
		}

		// Only found-sea-2 (-24h) falls within [-30h, -15h]
		if len(records) != 2 {
			t.Fatalf("expected 2 CSV rows (1 header + 1 record), got %d", len(records))
		}
		if records[1][0] != "INT-102" {
			t.Errorf("expected INT-102, got %s", records[1][0])
		}
	})
}

func TestExportReconciliationGeoJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	foundPets := []domain.FoundPetRecord{
		{
			PetID:         "found-geo-1",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-9901",
			CustodyStatus: domain.CustodyShelterCare,
			Coordinates: &domain.LocationPoint{
				Latitude:  47.648,
				Longitude: -122.378,
			},
			Species: "Cat",
			Breed:   "Tabby",
			FoundAt: now.Add(-24 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := analytics.ExportReconciliationGeoJSON(&buf, foundPets, nil, analytics.FilterOptions{})
	if err != nil {
		t.Fatalf("ExportReconciliationGeoJSON failed: %v", err)
	}

	var fc map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &fc); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if fc["type"] != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %v", fc["type"])
	}

	features := fc["features"].([]interface{})
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	feat := features[0].(map[string]interface{})
	if feat["type"] != "Feature" {
		t.Errorf("expected Feature, got %v", feat["type"])
	}

	geom := feat["geometry"].(map[string]interface{})
	if geom["type"] != "Point" {
		t.Errorf("expected Point geometry, got %v", geom["type"])
	}
	coords := geom["coordinates"].([]interface{})
	if len(coords) != 2 || coords[0].(float64) != -122.378 || coords[1].(float64) != 47.648 {
		t.Errorf("expected coordinates [-122.378, 47.648], got %v", coords)
	}

	props := feat["properties"].(map[string]interface{})
	if props["petId"] != "found-geo-1" || props["intakeId"] != "INT-9901" {
		t.Errorf("unexpected properties: %v", props)
	}
	if props["microchipStatus"] != "Unverified" {
		t.Errorf("expected microchipStatus Unverified, got %v", props["microchipStatus"])
	}
}

func TestExportReconciliationGeoJSON_FiltersAndMissingCoords(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	foundPets := []domain.FoundPetRecord{
		{
			PetID:         "geo-valid",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-VALID",
			CustodyStatus: domain.CustodyShelterCare,
			Coordinates: &domain.LocationPoint{
				Latitude:  47.6,
				Longitude: -122.3,
			},
			Species:           "Dog",
			Breed:             "Husky",
			MicrochipID:       "985141000999999",
			MicrochipRegistry: "HomeAgain",
			FoundAt:           now.Add(-10 * time.Hour),
		},
		{
			PetID:         "geo-missing-coords",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-NO-COORDS",
			CustodyStatus: domain.CustodyShelterCare,
			Coordinates:   nil, // Omitted
			Species:       "Dog",
			Breed:         "Pug",
			FoundAt:       now.Add(-5 * time.Hour),
		},
		{
			PetID:         "geo-bellevue",
			ShelterID:     "shelter-bel-02",
			ShelterName:   "Bellevue Humane",
			IntakeID:      "INT-BEL",
			CustodyStatus: domain.CustodyShelterCare,
			Coordinates: &domain.LocationPoint{
				Latitude:  47.61,
				Longitude: -122.2,
			},
			Species: "Cat",
			FoundAt: now.Add(-8 * time.Hour),
		},
	}

	matches := []domain.MatchRecord{
		{
			MatchID:      "match-geo-1",
			FoundPetID:   "geo-valid",
			MatchedPetID: "lost-geo-1",
			Score:        0.98,
			Status:       domain.MatchStatusConfirmed,
			MatchType:    "deterministic_microchip",
			MatchedAt:    now.Add(-2 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := analytics.ExportReconciliationGeoJSON(&buf, foundPets, matches, analytics.FilterOptions{
		ShelterID: "shelter-sea-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var fc map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &fc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	features := fc["features"].([]interface{})
	// Only geo-valid should be present: geo-missing-coords lacks coordinates, geo-bellevue filtered out
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	feat := features[0].(map[string]interface{})
	props := feat["properties"].(map[string]interface{})
	if props["petId"] != "geo-valid" {
		t.Errorf("expected petId geo-valid, got %v", props["petId"])
	}
	if props["microchipStatus"] != "Verified" {
		t.Errorf("expected microchipStatus Verified, got %v", props["microchipStatus"])
	}
	if props["maskedMicrochip"] != "HomeAgain ••••9999" {
		t.Errorf("expected maskedMicrochip HomeAgain ••••9999, got %v", props["maskedMicrochip"])
	}
	if props["matchStatus"] != "CONFIRMED" {
		t.Errorf("expected matchStatus CONFIRMED, got %v", props["matchStatus"])
	}
	if props["intakeDate"] != now.Add(-10*time.Hour).UTC().Format(time.RFC3339) {
		t.Errorf("unexpected intakeDate: %v", props["intakeDate"])
	}
}

type errWriter struct{}

func (e errWriter) Write(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

func TestExportReconciliation_WriterErrors(t *testing.T) {
	t.Parallel()

	foundPets := []domain.FoundPetRecord{
		{
			PetID:         "found-1",
			ShelterID:     "shelter-sea-01",
			IntakeID:      "INT-1",
			CustodyStatus: domain.CustodyShelterCare,
			Coordinates: &domain.LocationPoint{
				Latitude:  47.6,
				Longitude: -122.3,
			},
		},
	}

	errCSV := analytics.ExportReconciliationCSV(errWriter{}, foundPets, nil, analytics.FilterOptions{})
	if errCSV == nil {
		t.Error("expected CSV writer error, got nil")
	}

	errGeo := analytics.ExportReconciliationGeoJSON(errWriter{}, foundPets, nil, analytics.FilterOptions{})
	if errGeo == nil {
		t.Error("expected GeoJSON writer error, got nil")
	}
}

func TestExportReconciliationCSV_MatchTieBreakersAndLifecycleAudit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	auditTime := now.Add(-5 * time.Hour)

	foundPets := []domain.FoundPetRecord{
		{
			PetID:         "pet-score-tiebreak",
			ShelterID:     "shelter-sea-01",
			IntakeID:      "INT-SCORE",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-20 * time.Hour),
		},
		{
			PetID:         "pet-date-tiebreak",
			ShelterID:     "shelter-sea-01",
			IntakeID:      "INT-DATE",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       now.Add(-20 * time.Hour),
		},
		{
			PetID:         "pet-resolved-audit",
			ShelterID:     "shelter-sea-01",
			IntakeID:      "INT-RESOLVED",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusResolved,
			FoundAt:       now.Add(-20 * time.Hour),
			LifecycleAudit: &domain.FoundPetLifecycleAudit{
				ChangedAt: auditTime,
			},
		},
		{
			PetID:         "pet-invalid-chip",
			ShelterID:     "shelter-sea-01",
			IntakeID:      "INT-INVALID-CHIP",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			MicrochipID:   "invalid-not-a-chip",
			FoundAt:       now.Add(-20 * time.Hour),
		},
	}

	matches := []domain.MatchRecord{
		// Ties on status CONFIRMED: score 0.85 vs score 0.95 -> 0.95 wins
		{
			MatchID:      "match-score-low",
			FoundPetID:   "pet-score-tiebreak",
			MatchedPetID: "lost-1",
			Score:        0.85,
			Status:       domain.MatchStatusConfirmed,
			MatchType:    "visual_low",
			MatchedAt:    now.Add(-10 * time.Hour),
		},
		{
			MatchID:      "match-score-high",
			FoundPetID:   "pet-score-tiebreak",
			MatchedPetID: "lost-2",
			Score:        0.95,
			Status:       domain.MatchStatusConfirmed,
			MatchType:    "visual_high",
			MatchedAt:    now.Add(-12 * time.Hour),
		},
		// Ties on status CONFIRMED and score 0.90: earlier -15h vs later -8h -> later wins
		{
			MatchID:      "match-date-earlier",
			FoundPetID:   "pet-date-tiebreak",
			MatchedPetID: "lost-3",
			Score:        0.90,
			Status:       domain.MatchStatusConfirmed,
			MatchType:    "type_earlier",
			MatchedAt:    now.Add(-15 * time.Hour),
		},
		{
			MatchID:      "match-date-later",
			FoundPetID:   "pet-date-tiebreak",
			MatchedPetID: "lost-4",
			Score:        0.90,
			Status:       domain.MatchStatusConfirmed,
			MatchType:    "type_later",
			MatchedAt:    now.Add(-8 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := analytics.ExportReconciliationCSV(&buf, foundPets, matches, analytics.FilterOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	// 1 header + 4 rows
	if len(records) != 5 {
		t.Fatalf("expected 5 CSV rows, got %d", len(records))
	}

	// Row 1: pet-score-tiebreak should pick visual_high (0.95)
	row1 := records[1]
	if row1[0] != "INT-SCORE" || row1[12] != "visual_high" || row1[13] != "0.95" {
		t.Errorf("expected visual_high with score 0.95, got %v", row1)
	}

	// Row 2: pet-date-tiebreak should pick type_later
	row2 := records[2]
	if row2[0] != "INT-DATE" || row2[12] != "type_later" {
		t.Errorf("expected type_later, got %v", row2)
	}

	// Row 3: pet-resolved-audit should have ReunitedDate from auditTime
	row3 := records[3]
	if row3[0] != "INT-RESOLVED" || row3[14] != auditTime.UTC().Format(time.RFC3339) {
		t.Errorf("expected audit reunion time %s, got %s", auditTime.UTC().Format(time.RFC3339), row3[14])
	}

	// Row 4: pet-invalid-chip should have MicrochipStatus Unverified and empty MaskedMicrochip
	row4 := records[4]
	if row4[0] != "INT-INVALID-CHIP" || row4[8] != "Unverified" || row4[9] != "" {
		t.Errorf("expected Unverified and empty MaskedMicrochip, got %v", row4)
	}
}
