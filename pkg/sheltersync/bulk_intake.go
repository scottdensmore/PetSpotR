package sheltersync

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/microchip"
)

// BulkIntakeRecordResult represents the parsing and validation outcome for a single intake record.
type BulkIntakeRecordResult struct {
	RowIndex          int                `json:"rowIndex"`
	PetID             string             `json:"petId,omitempty"`
	Microchip         string             `json:"microchip,omitempty"`
	MicrochipValid    bool               `json:"microchipValid"`
	MicrochipStandard microchip.Standard `json:"microchipStandard,omitempty"`
	Status            string             `json:"status"` // INGESTED, INVALID_MICROCHIP, ERROR
	ErrorMessage      string             `json:"errorMessage,omitempty"`
	MatchedLostPetID  string             `json:"matchedLostPetId,omitempty"`
}

// BulkIntakeBatchSummary captures operational totals and individual results of a bulk intake batch.
type BulkIntakeBatchSummary struct {
	BatchID        string                   `json:"batchId"`
	HubID          string                   `json:"hubId"`
	HubName        string                   `json:"hubName"`
	TotalProcessed int                      `json:"totalProcessed"`
	IngestedCount  int                      `json:"ingestedCount"`
	ErrorCount     int                      `json:"errorCount"`
	MicrochipCount int                      `json:"microchipCount"`
	InstantMatches int                      `json:"instantMatches"`
	Results        []BulkIntakeRecordResult `json:"results"`
	ProcessedAt    time.Time                `json:"processedAt"`
}

type rawIntakeRow struct {
	Species        string
	Breed          string
	PrimaryColor   string
	SecondaryColor string
	Gender         string
	MicrochipID    string
	Location       string
	Notes          string
	IntakeID       string
	PetID          string
	Latitude       float64
	Longitude      float64
}

// ParseBulkIntakeCSV parses animal intake records from a CSV reader with flexible header normalization.
func ParseBulkIntakeCSV(r io.Reader, hubID string, hubName string) (BulkIntakeBatchSummary, []domain.FoundPetRecord, error) {
	now := time.Now().UTC()
	summary := BulkIntakeBatchSummary{
		BatchID:     newBatchID(hubID),
		HubID:       strings.TrimSpace(hubID),
		HubName:     strings.TrimSpace(hubName),
		Results:     make([]BulkIntakeRecordResult, 0),
		ProcessedAt: now,
	}
	records := make([]domain.FoundPetRecord, 0)

	csvReader := csv.NewReader(r)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	// Read headers
	headers, err := csvReader.Read()
	if errors.Is(err, io.EOF) {
		return summary, records, nil
	}
	if err != nil {
		return summary, nil, fmt.Errorf("bulk intake: read csv headers: %w", err)
	}

	colMap := make(map[int]string)
	hasSpecies := false

	for i, h := range headers {
		h = strings.TrimPrefix(h, "\ufeff") // Strip UTF-8 BOM
		norm := normalizeHeaderName(h)
		colType := mapHeaderToField(norm)
		if colType != "" {
			colMap[i] = colType
			if colType == "species" {
				hasSpecies = true
			}
		}
	}

	if !hasSpecies {
		return summary, nil, errors.New("bulk intake csv missing species column")
	}

	rowIdx := 0
	for {
		rowRecord, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return summary, records, fmt.Errorf("bulk intake: read csv line: %w", err)
		}

		// Skip rows where all fields are empty
		isEmpty := true
		for _, f := range rowRecord {
			if strings.TrimSpace(f) != "" {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			continue
		}

		var row rawIntakeRow
		for colIdx, val := range rowRecord {
			colType, ok := colMap[colIdx]
			if !ok {
				continue
			}
			val = strings.TrimSpace(val)
			populateRawRow(&row, colType, val)
		}

		result, petRecord := processIntakeRow(rowIdx, row, hubID, hubName, now)
		summary.TotalProcessed++
		summary.Results = append(summary.Results, result)

		if result.Status == "ERROR" {
			summary.ErrorCount++
		} else {
			summary.IngestedCount++
			if result.MicrochipValid {
				summary.MicrochipCount++
			}
			if petRecord != nil {
				records = append(records, *petRecord)
			}
		}

		rowIdx++
	}

	return summary, records, nil
}

// ParseBulkIntakeJSON parses an intake roster from JSON (array of items or object with an items list).
func ParseBulkIntakeJSON(data []byte, hubID string, hubName string) (BulkIntakeBatchSummary, []domain.FoundPetRecord, error) {
	now := time.Now().UTC()
	summary := BulkIntakeBatchSummary{
		BatchID:     newBatchID(hubID),
		HubID:       strings.TrimSpace(hubID),
		HubName:     strings.TrimSpace(hubName),
		Results:     make([]BulkIntakeRecordResult, 0),
		ProcessedAt: now,
	}
	records := make([]domain.FoundPetRecord, 0)

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return summary, records, nil
	}

	var rawList []map[string]interface{}
	if err := json.Unmarshal(trimmed, &rawList); err != nil {
		var rawObj map[string]interface{}
		if err2 := json.Unmarshal(trimmed, &rawObj); err2 != nil {
			return summary, nil, fmt.Errorf("bulk intake json: unmarshal error: %w", err)
		}
		found := false
		for _, k := range []string{"records", "animals", "intakes", "items", "data", "pets"} {
			if arr, ok := rawObj[k].([]interface{}); ok {
				rawList = make([]map[string]interface{}, 0, len(arr))
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						rawList = append(rawList, m)
					}
				}
				found = true
				break
			}
		}
		if !found {
			return summary, nil, errors.New("bulk intake json: payload must be an array of records or contain a records list")
		}
	}

	for idx, itemMap := range rawList {
		row := mapToRawIntakeRow(itemMap)
		result, petRecord := processIntakeRow(idx, row, hubID, hubName, now)
		summary.TotalProcessed++
		summary.Results = append(summary.Results, result)

		if result.Status == "ERROR" {
			summary.ErrorCount++
		} else {
			summary.IngestedCount++
			if result.MicrochipValid {
				summary.MicrochipCount++
			}
			if petRecord != nil {
				records = append(records, *petRecord)
			}
		}
	}

	return summary, records, nil
}

func processIntakeRow(
	idx int,
	row rawIntakeRow,
	hubID string,
	hubName string,
	now time.Time,
) (BulkIntakeRecordResult, *domain.FoundPetRecord) {
	species := strings.TrimSpace(row.Species)
	if species == "" {
		return BulkIntakeRecordResult{
			RowIndex:     idx,
			Status:       "ERROR",
			ErrorMessage: "species is required",
		}, nil
	}

	var petID string
	if row.PetID != "" {
		petID = row.PetID
	} else if row.IntakeID != "" {
		petID = fmt.Sprintf("found-%s-%s", sanitizeIdentifier(hubID), sanitizeIdentifier(row.IntakeID))
	} else {
		petID = newIntakePetID(hubID)
	}

	result := BulkIntakeRecordResult{
		RowIndex: idx,
		PetID:    petID,
	}

	rawChip := strings.TrimSpace(row.MicrochipID)
	result.Microchip = rawChip

	var normalizedChip string
	var registryName string

	if rawChip == "" {
		result.MicrochipValid = false
		result.MicrochipStandard = microchip.StandardUnknown
		result.Status = "INGESTED"
	} else {
		val := microchip.ValidateAndNormalize(rawChip)
		result.MicrochipValid = val.Valid
		result.MicrochipStandard = val.Standard
		if val.Valid {
			result.Status = "INGESTED"
			result.Microchip = val.NormalizedID
			normalizedChip = val.NormalizedID
			reg := microchip.IdentifyIssuingRegistry(val.NormalizedID)
			registryName = reg.RegistryName
		} else {
			result.Status = "INVALID_MICROCHIP"
			result.ErrorMessage = val.ErrorMessage
		}
	}

	location := strings.TrimSpace(row.Location)
	if location == "" {
		if hubName != "" {
			location = hubName
		} else {
			location = hubID
		}
	}

	var coords *domain.LocationPoint
	geocodingStatus := domain.GeocodingPending
	if row.Latitude != 0 || row.Longitude != 0 {
		coords = &domain.LocationPoint{
			Latitude:  row.Latitude,
			Longitude: row.Longitude,
		}
		geocodingStatus = domain.GeocodingVerified
	}

	var markings []string
	if notes := strings.TrimSpace(row.Notes); notes != "" {
		markings = append(markings, notes)
	}

	rec := domain.FoundPetRecord{
		PetID:               petID,
		FoundAt:             now,
		Location:            location,
		GeocodingStatus:     geocodingStatus,
		Coordinates:         coords,
		Species:             species,
		Breed:               strings.TrimSpace(row.Breed),
		PrimaryColor:        strings.TrimSpace(row.PrimaryColor),
		SecondaryColor:      strings.TrimSpace(row.SecondaryColor),
		DistinctiveMarkings: markings,
		CustodyStatus:       domain.CustodyShelterCare,
		Status:              domain.FoundPetStatusFound,
		MicrochipID:         normalizedChip,
		MicrochipRegistry:   registryName,
		ShelterID:           hubID,
		ShelterName:         hubName,
		IntakeID:            strings.TrimSpace(row.IntakeID),
	}

	rec = domain.NormalizeFoundPetRecord(rec)
	return result, &rec
}

func populateRawRow(row *rawIntakeRow, colType string, val string) {
	switch colType {
	case "species":
		row.Species = val
	case "breed":
		row.Breed = val
	case "primary_color":
		row.PrimaryColor = val
	case "secondary_color":
		row.SecondaryColor = val
	case "gender":
		row.Gender = val
	case "microchip_id":
		row.MicrochipID = val
	case "address":
		row.Location = val
	case "notes":
		row.Notes = val
	case "intake_id":
		row.IntakeID = val
	case "pet_id":
		row.PetID = val
	case "latitude":
		if lat, err := strconv.ParseFloat(val, 64); err == nil {
			row.Latitude = lat
		}
	case "longitude":
		if lon, err := strconv.ParseFloat(val, 64); err == nil {
			row.Longitude = lon
		}
	}
}

func mapHeaderToField(norm string) string {
	switch norm {
	case "species", "animaltype", "animal", "pettype", "type":
		return "species"
	case "breed", "animalbreed", "petbreed":
		return "breed"
	case "primarycolor", "primarycolour", "color", "colour", "maincolor":
		return "primary_color"
	case "secondarycolor", "secondarycolour":
		return "secondary_color"
	case "gender", "sex":
		return "gender"
	case "microchipid", "microchip", "chipid", "chip", "rfid", "transponder", "microchipnumber":
		return "microchip_id"
	case "address", "foundlocation", "location", "intakelocation", "foundaddress", "street":
		return "address"
	case "notes", "triagenotes", "description", "intakenotes", "comments", "intakecomment", "medicalnotes":
		return "notes"
	case "intakeid", "animalid", "petid", "id":
		return "intake_id"
	case "latitude", "lat":
		return "latitude"
	case "longitude", "lon", "lng":
		return "longitude"
	default:
		return ""
	}
}

func mapToRawIntakeRow(m map[string]interface{}) rawIntakeRow {
	var row rawIntakeRow
	for k, v := range m {
		normKey := normalizeHeaderName(k)
		strVal := fmt.Sprintf("%v", v)
		if v == nil || strVal == "<nil>" {
			strVal = ""
		}
		strVal = strings.TrimSpace(strVal)
		colType := mapHeaderToField(normKey)
		if colType != "" {
			populateRawRow(&row, colType, strVal)
		}
	}
	return row
}

func normalizeHeaderName(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	h = strings.ReplaceAll(h, "-", "")
	h = strings.ReplaceAll(h, "_", "")
	h = strings.ReplaceAll(h, " ", "")
	return h
}

func sanitizeIdentifier(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(clean, "-")
}

func newBatchID(hubID string) string {
	var random [8]byte
	_, _ = rand.Read(random[:])
	suffix := hex.EncodeToString(random[:])
	cleanHub := sanitizeIdentifier(hubID)
	if cleanHub != "" {
		return fmt.Sprintf("batch-%s-%s", cleanHub, suffix)
	}
	return fmt.Sprintf("batch-%s", suffix)
}

func newIntakePetID(hubID string) string {
	var random [12]byte
	_, _ = rand.Read(random[:])
	suffix := hex.EncodeToString(random[:])
	cleanHub := sanitizeIdentifier(hubID)
	if cleanHub != "" {
		return fmt.Sprintf("found-%s-%s", cleanHub, suffix)
	}
	return fmt.Sprintf("found-%s", suffix)
}
