package sheltersync_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/microchip"
	"github.com/scottdensmore/petspotr/pkg/sheltersync"
)

func TestParseBulkIntakeCSV_Valid(t *testing.T) {
	t.Parallel()

	csvData := `species,breed,primary_color,gender,microchip_id,address,notes
dog,Golden Retriever,Golden,male,985141000123456,"2061 15th Ave W, Seattle, WA","Found near Interbay"
cat,Domestic Shorthair,Tabby,female,456789123,"Mercer St, Seattle, WA","Scanned Avid chip"
dog,Poodle,White,male,invalid-chip,"Rainier Ave, Seattle, WA","Triage check ok"
`
	summary, records, err := sheltersync.ParseBulkIntakeCSV(strings.NewReader(csvData), "hub-test-1", "Test Crisis Center")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if summary.TotalProcessed != 3 {
		t.Errorf("expected 3 processed, got %d", summary.TotalProcessed)
	}
	if summary.IngestedCount != 3 {
		t.Errorf("expected 3 ingested, got %d", summary.IngestedCount)
	}
	if summary.MicrochipCount != 2 {
		t.Errorf("expected 2 valid microchips, got %d", summary.MicrochipCount)
	}
	if summary.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", summary.ErrorCount)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 found pet records, got %d", len(records))
	}

	// Verify Record 0 (ISO-15 chip)
	r0 := records[0]
	if r0.Species != "Dog" {
		t.Errorf("expected species Dog, got %q", r0.Species)
	}
	if r0.Breed != "Golden Retriever" {
		t.Errorf("expected breed Golden Retriever, got %q", r0.Breed)
	}
	if r0.PrimaryColor != "Golden" {
		t.Errorf("expected color Golden, got %q", r0.PrimaryColor)
	}
	if r0.MicrochipID != "985141000123456" {
		t.Errorf("expected microchip 985141000123456, got %q", r0.MicrochipID)
	}
	if r0.ShelterID != "hub-test-1" {
		t.Errorf("expected shelter ID hub-test-1, got %q", r0.ShelterID)
	}
	if r0.ShelterName != "Test Crisis Center" {
		t.Errorf("expected shelter name Test Crisis Center, got %q", r0.ShelterName)
	}
	if r0.Status != domain.FoundPetStatusFound {
		t.Errorf("expected status %s, got %s", domain.FoundPetStatusFound, r0.Status)
	}
	if r0.CustodyStatus != domain.CustodyShelterCare {
		t.Errorf("expected custody status %s, got %s", domain.CustodyShelterCare, r0.CustodyStatus)
	}

	// Verify Record 1 (Avid-9 chip)
	r1 := records[1]
	if r1.Species != "Cat" {
		t.Errorf("expected species Cat, got %q", r1.Species)
	}
	if r1.MicrochipID != "456789123" {
		t.Errorf("expected microchip 456789123, got %q", r1.MicrochipID)
	}

	// Verify Record 2 (Invalid chip - still ingested into shelter care, but microchip unindexed)
	r2 := records[2]
	if r2.Species != "Dog" {
		t.Errorf("expected species Dog, got %q", r2.Species)
	}
	if r2.MicrochipID != "" {
		t.Errorf("expected empty microchip on record with invalid chip, got %q", r2.MicrochipID)
	}

	// Verify Summary Results
	if len(summary.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(summary.Results))
	}
	res0 := summary.Results[0]
	if !res0.MicrochipValid || res0.MicrochipStandard != microchip.StandardISO15 || res0.Status != "INGESTED" {
		t.Errorf("res0 mismatch: %+v", res0)
	}
	res1 := summary.Results[1]
	if !res1.MicrochipValid || res1.MicrochipStandard != microchip.StandardAvid9 || res1.Status != "INGESTED" {
		t.Errorf("res1 mismatch: %+v", res1)
	}
	res2 := summary.Results[2]
	if res2.MicrochipValid || res2.Status != "INVALID_MICROCHIP" || res2.ErrorMessage == "" {
		t.Errorf("res2 mismatch: %+v", res2)
	}
}

func TestParseBulkIntakeCSV_FlexibleHeaders(t *testing.T) {
	t.Parallel()

	csvData := `animal_type,animal_breed,color,secondary_colour,sex,chip,found_location,triage_notes
cat,Siamese,Seal Point,Cream,female,985141000999888,"Bellevue Way NE, Bellevue, WA","Mild dehydration"
`
	summary, records, err := sheltersync.ParseBulkIntakeCSV(strings.NewReader(csvData), "hub-bel-1", "Bellevue Evac Center")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if summary.IngestedCount != 1 || len(records) != 1 {
		t.Fatalf("expected 1 ingested record, got %d / len %d", summary.IngestedCount, len(records))
	}

	rec := records[0]
	if rec.Species != "Cat" {
		t.Errorf("expected species Cat, got %q", rec.Species)
	}
	if rec.Breed != "Siamese" {
		t.Errorf("expected breed Siamese, got %q", rec.Breed)
	}
	if rec.PrimaryColor != "Seal Point" {
		t.Errorf("expected primary color Seal Point, got %q", rec.PrimaryColor)
	}
	if rec.SecondaryColor != "Cream" {
		t.Errorf("expected secondary color Cream, got %q", rec.SecondaryColor)
	}
	if rec.MicrochipID != "985141000999888" {
		t.Errorf("expected microchip 985141000999888, got %q", rec.MicrochipID)
	}
	if rec.Location != "Bellevue Way NE, Bellevue, WA" {
		t.Errorf("expected location Bellevue Way NE, Bellevue, WA, got %q", rec.Location)
	}
	if len(rec.DistinctiveMarkings) == 0 || rec.DistinctiveMarkings[0] != "Mild dehydration" {
		t.Errorf("expected markings 'Mild dehydration', got %v", rec.DistinctiveMarkings)
	}
}

func TestParseBulkIntakeCSV_EdgeCases(t *testing.T) {
	t.Parallel()

	// Empty lines, missing species, quoted fields with commas
	csvData := `species,breed,primary_color,gender,microchip_id,address,notes

dog,Beagle,Tri-color,male,985141000111222,"123 Main St, Suite 400, Seattle, WA","Active, friendly"

,Unknown,Black,female,,Seattle,"Missing species row"

`
	summary, records, err := sheltersync.ParseBulkIntakeCSV(strings.NewReader(csvData), "hub-1", "Main Hub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalProcessed != 2 {
		t.Errorf("expected 2 processed rows (skipping empty lines), got %d", summary.TotalProcessed)
	}
	if summary.IngestedCount != 1 {
		t.Errorf("expected 1 ingested, got %d", summary.IngestedCount)
	}
	if summary.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", summary.ErrorCount)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	// Verify error record in summary
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(summary.Results))
	}
	errResult := summary.Results[1]
	if errResult.Status != "ERROR" || errResult.ErrorMessage == "" {
		t.Errorf("expected ERROR status with message, got %+v", errResult)
	}
}

func TestParseBulkIntakeCSV_MissingSpeciesHeader(t *testing.T) {
	t.Parallel()

	csvData := `breed,primary_color,notes
Golden Retriever,Gold,"No species header"
`
	_, _, err := sheltersync.ParseBulkIntakeCSV(strings.NewReader(csvData), "hub-1", "Main Hub")
	if err == nil {
		t.Fatal("expected error when species header is missing, got nil")
	}
}

func TestParseBulkIntakeCSV_Empty(t *testing.T) {
	t.Parallel()

	summary, records, err := sheltersync.ParseBulkIntakeCSV(strings.NewReader(""), "hub-1", "Main Hub")
	if err != nil {
		t.Fatalf("unexpected error on empty CSV: %v", err)
	}
	if summary.TotalProcessed != 0 || len(records) != 0 {
		t.Errorf("expected 0 processed, got %d / len %d", summary.TotalProcessed, len(records))
	}
}

func TestParseBulkIntakeJSON_Array(t *testing.T) {
	t.Parallel()

	jsonData := `[
		{
			"species": "cat",
			"breed": "Tabby",
			"primaryColor": "Brown",
			"microchipId": "985141000333444",
			"address": "Capitol Hill, Seattle, WA",
			"notes": "Scanned chip at triage"
		},
		{
			"animalType": "dog",
			"breed": "Boxer",
			"color": "Brindle",
			"chip": "invalid-chip",
			"foundLocation": "Green Lake, Seattle, WA"
		},
		{
			"breed": "Chihuahua",
			"notes": "No species specified"
		}
	]`

	summary, records, err := sheltersync.ParseBulkIntakeJSON([]byte(jsonData), "hub-json-1", "JSON Test Hub")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if summary.TotalProcessed != 3 {
		t.Errorf("expected 3 processed, got %d", summary.TotalProcessed)
	}
	if summary.IngestedCount != 2 {
		t.Errorf("expected 2 ingested, got %d", summary.IngestedCount)
	}
	if summary.MicrochipCount != 1 {
		t.Errorf("expected 1 microchip, got %d", summary.MicrochipCount)
	}
	if summary.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", summary.ErrorCount)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestParseBulkIntakeJSON_NestedObject(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"records": [
			{
				"species": "dog",
				"breed": "Husky",
				"primary_color": "Grey",
				"microchip_id": "456789123",
				"found_location": "Fremont, Seattle, WA"
			}
		]
	}`

	summary, records, err := sheltersync.ParseBulkIntakeJSON([]byte(jsonData), "hub-json-2", "Nested JSON Hub")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if summary.TotalProcessed != 1 || summary.IngestedCount != 1 || len(records) != 1 {
		t.Fatalf("expected 1 processed/ingested, got processed=%d, ingested=%d, len=%d",
			summary.TotalProcessed, summary.IngestedCount, len(records))
	}
	if records[0].Species != "Dog" || records[0].MicrochipID != "456789123" {
		t.Errorf("unexpected record data: %+v", records[0])
	}
}

func TestParseBulkIntakeJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, _, err := sheltersync.ParseBulkIntakeJSON([]byte("not valid json"), "hub-1", "Hub")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
