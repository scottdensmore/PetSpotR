package webfrontend

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// PublicPetDirectoryItem is the unified redacted listing item for the public pet directory.
type PublicPetDirectoryItem struct {
	PetID               string                 `json:"petId"`
	ReportType          string                 `json:"reportType"` // "lost" or "found"
	Status              string                 `json:"status"`     // "lost" or "found"
	PetName             string                 `json:"petName,omitempty"`
	Species             string                 `json:"species,omitempty"`
	Breed               string                 `json:"breed,omitempty"`
	PrimaryColor        string                 `json:"primaryColor,omitempty"`
	SecondaryColor      string                 `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string               `json:"distinctiveMarkings,omitempty"`
	Description         string                 `json:"description,omitempty"`
	ImageURL            string                 `json:"imageUrl,omitempty"`
	Location            string                 `json:"location"`
	ReportedAt          time.Time              `json:"reportedAt"`
	Coordinates         *domain.LocationPoint  `json:"coordinates,omitempty"`
	GeocodingStatus     domain.GeocodingStatus `json:"geocodingStatus,omitempty"`
	CustodyStatus       domain.CustodyStatus   `json:"custodyStatus,omitempty"`
}

// DirectoryQueryParams defines validated filters and pagination options.
type DirectoryQueryParams struct {
	Species     string
	Status      string
	Query       string
	Limit       int
	Offset      int
	Cursor      string
	HasGeo      bool
	GeoPoint    domain.LocationPoint
	RadiusMiles float64
}

// PetMapMarker represents a lightweight client marker payload for directory map pins.
type PetMapMarker struct {
	PetID      string  `json:"petId"`
	Status     string  `json:"status"`
	PetName    string  `json:"petName"`
	Species    string  `json:"species"`
	Breed      string  `json:"breed"`
	Location   string  `json:"location"`
	ImageURL   string  `json:"imageUrl"`
	ReportedAt string  `json:"reportedAt"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
}

// PetsPageData provides context data to the pets.html template.
type PetsPageData struct {
	Pets        []PublicPetDirectoryItem
	TotalCount  int
	CurrentPage int
	TotalPages  int
	Species     string
	Status      string
	Query       string
	Limit       int
	Offset      int
	HasPrev     bool
	HasNext     bool
	PrevPageURL string
	NextPageURL string
	NextCursor  string
	PrevCursor  string
	Lat         float64
	Lng         float64
	RadiusMiles float64
	HasGeo      bool
	MappedCount int
	PetsJSON    template.JS
}

func encodeCursor(t time.Time, id string) string {
	raw := fmt.Sprintf("%d:%s", t.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (time.Time, string, error) {
	bytes, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("malformed cursor encoding: %w", err)
	}
	parts := strings.SplitN(string(bytes), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("malformed cursor format")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("malformed cursor timestamp: %w", err)
	}
	id := parts[1]
	if id == "" {
		return time.Time{}, "", errors.New("malformed cursor pet id")
	}
	return time.Unix(0, nanos).UTC(), id, nil
}

func parseDirectoryQueryParams(r *http.Request) (DirectoryQueryParams, error) {
	q := r.URL.Query()

	limit := 20
	if lStr := q.Get("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 {
			return DirectoryQueryParams{}, errors.New("invalid limit: must be a positive integer")
		}
		if l > 100 {
			l = 100
		}
		limit = l
	}

	offset := 0
	if oStr := q.Get("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 {
			return DirectoryQueryParams{}, errors.New("invalid offset: must be a non-negative integer")
		}
		offset = o
	}

	cursor := strings.TrimSpace(q.Get("cursor"))
	if cursor != "" {
		if _, _, err := decodeCursor(cursor); err != nil {
			return DirectoryQueryParams{}, fmt.Errorf("invalid cursor: %w", err)
		}
	}

	species := strings.ToLower(strings.TrimSpace(q.Get("species")))

	status := strings.ToLower(strings.TrimSpace(q.Get("status")))
	if status != "" && status != "lost" && status != "found" && status != "all" {
		return DirectoryQueryParams{}, errors.New("invalid status filter: must be lost, found, or all")
	}

	query := strings.TrimSpace(q.Get("q"))
	if query == "" {
		query = strings.TrimSpace(q.Get("query"))
	}
	if query == "" {
		query = strings.TrimSpace(q.Get("breed"))
	}

	var hasGeo bool
	var geoPoint domain.LocationPoint
	radiusMiles := 10.0

	latStr := q.Get("lat")
	lngStr := q.Get("lng")
	if (latStr != "" && lngStr == "") || (latStr == "" && lngStr != "") {
		return DirectoryQueryParams{}, errors.New("both lat and lng parameters are required for proximity filtering")
	}
	if latStr != "" && lngStr != "" {
		lat, err1 := strconv.ParseFloat(latStr, 64)
		lng, err2 := strconv.ParseFloat(lngStr, 64)
		if err1 != nil || err2 != nil || math.IsNaN(lat) || math.IsNaN(lng) || math.IsInf(lat, 0) || math.IsInf(lng, 0) ||
			lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return DirectoryQueryParams{}, errors.New("invalid coordinates: lat must be [-90, 90] and lng [-180, 180]")
		}
		hasGeo = true
		geoPoint = domain.LocationPoint{Latitude: lat, Longitude: lng}
	}

	if rStr := q.Get("radiusMiles"); rStr != "" {
		rVal, err := strconv.ParseFloat(rStr, 64)
		if err != nil || math.IsNaN(rVal) || math.IsInf(rVal, 0) || rVal <= 0 {
			return DirectoryQueryParams{}, errors.New("invalid radiusMiles: must be a positive number")
		}
		radiusMiles = rVal
	}

	return DirectoryQueryParams{
		Species:     species,
		Status:      status,
		Query:       query,
		Limit:       limit,
		Offset:      offset,
		Cursor:      cursor,
		HasGeo:      hasGeo,
		GeoPoint:    geoPoint,
		RadiusMiles: radiusMiles,
	}, nil
}

func (s *Server) queryDirectoryPets(
	ctx context.Context,
	params DirectoryQueryParams,
) ([]PublicPetDirectoryItem, int, string, string, error) {
	var items []PublicPetDirectoryItem

	// 1. Fetch Lost Pets if status is not "found"
	if params.Status != "found" {
		rawLost, err := s.stateStore.ListState(ctx, store.LostPetsCollection)
		if err != nil {
			return nil, 0, "", "", fmt.Errorf("list lost pets: %w", err)
		}
		for _, b := range rawLost {
			var rec domain.LostPetRecord
			if err := json.Unmarshal(b, &rec); err != nil {
				continue
			}
			rec = domain.NormalizeLostPetRecord(rec)
			if !rec.Status.IsActive() {
				continue
			}
			if !matchesSpeciesFilter(rec.Species, params.Species) {
				continue
			}
			if params.Query != "" && !matchesLostPetQuery(rec, params.Query) {
				continue
			}
			if params.HasGeo {
				coords, ok := extractCoordinates(rec.Coordinates, rec.Location)
				if !ok {
					continue
				}
				dist := domain.HaversineDistanceMiles(params.GeoPoint, coords)
				if dist > params.RadiusMiles {
					continue
				}
			}

			pub := rec.Public()
			item := PublicPetDirectoryItem{
				PetID:           pub.PetID,
				ReportType:      "lost",
				Status:          "lost",
				PetName:         pub.PetName,
				Species:         pub.Species,
				Breed:           pub.Breed,
				PrimaryColor:    pub.PrimaryColor,
				Description:     pub.Description,
				Location:        pub.Location,
				ReportedAt:      pub.ReportedAt.UTC(),
				Coordinates:     pub.Coordinates,
				GeocodingStatus: pub.GeocodingStatus,
			}
			items = append(items, item)
		}
	}

	// 2. Fetch Found Pets if status is not "lost"
	if params.Status != "lost" {
		rawFound, err := s.stateStore.ListState(ctx, store.FoundPetsCollection)
		if err != nil {
			return nil, 0, "", "", fmt.Errorf("list found pets: %w", err)
		}
		for _, b := range rawFound {
			var rec domain.FoundPetRecord
			if err := json.Unmarshal(b, &rec); err != nil {
				continue
			}
			rec = domain.NormalizeFoundPetRecord(rec)
			if !rec.Status.IsActive() {
				continue
			}
			if !matchesSpeciesFilter(rec.Species, params.Species) {
				continue
			}
			if params.Query != "" && !matchesFoundPetQuery(rec, params.Query) {
				continue
			}
			if params.HasGeo {
				coords, ok := extractCoordinates(rec.Coordinates, rec.Location)
				if !ok {
					continue
				}
				dist := domain.HaversineDistanceMiles(params.GeoPoint, coords)
				if dist > params.RadiusMiles {
					continue
				}
			}

			pub := rec.Public()
			item := PublicPetDirectoryItem{
				PetID:               pub.PetID,
				ReportType:          "found",
				Status:              "found",
				ImageURL:            pub.ImageURL,
				Species:             pub.Species,
				Breed:               pub.Breed,
				PrimaryColor:        pub.PrimaryColor,
				SecondaryColor:      pub.SecondaryColor,
				DistinctiveMarkings: pub.DistinctiveMarkings,
				Location:            pub.Location,
				ReportedAt:          pub.FoundAt.UTC(),
				Coordinates:         pub.Coordinates,
				GeocodingStatus:     pub.GeocodingStatus,
				CustodyStatus:       pub.CustodyStatus,
			}
			items = append(items, item)
		}
	}

	// 3. Deterministic Sort: ReportedAt DESC, PetID DESC
	sort.Slice(items, func(i, j int) bool {
		if !items[i].ReportedAt.Equal(items[j].ReportedAt) {
			return items[i].ReportedAt.After(items[j].ReportedAt)
		}
		return items[i].PetID > items[j].PetID
	})

	totalCount := len(items)

	// 4. Deterministic Pagination
	start := 0
	if params.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(params.Cursor)
		if err != nil {
			return nil, 0, "", "", err
		}
		start = len(items)
		for idx, item := range items {
			if item.ReportedAt.Before(cursorTime) || (item.ReportedAt.Equal(cursorTime) && item.PetID < cursorID) {
				start = idx
				break
			}
		}
	} else if params.Offset > 0 {
		start = params.Offset
		if start > len(items) {
			start = len(items)
		}
	}

	end := start + params.Limit
	if end > len(items) {
		end = len(items)
	}

	pagedItems := items[start:end]

	var nextCursor, prevCursor string
	if end < len(items) && len(pagedItems) > 0 {
		last := pagedItems[len(pagedItems)-1]
		nextCursor = encodeCursor(last.ReportedAt, last.PetID)
	}
	if start > 0 && len(pagedItems) > 0 {
		first := pagedItems[0]
		prevCursor = encodeCursor(first.ReportedAt, first.PetID)
	}

	return pagedItems, totalCount, nextCursor, prevCursor, nil
}

func matchesSpeciesFilter(petSpecies, filterSpecies string) bool {
	if filterSpecies == "" || filterSpecies == "all" {
		return true
	}
	petSpecies = strings.TrimSpace(petSpecies)
	if petSpecies == "" {
		return false
	}
	if filterSpecies == "other" {
		return !strings.EqualFold(petSpecies, "dog") && !strings.EqualFold(petSpecies, "cat")
	}
	return strings.EqualFold(petSpecies, filterSpecies)
}

func matchesLostPetQuery(pet domain.LostPetRecord, q string) bool {
	q = strings.ToLower(q)
	if strings.Contains(strings.ToLower(pet.PetName), q) ||
		strings.Contains(strings.ToLower(pet.Breed), q) ||
		strings.Contains(strings.ToLower(pet.Species), q) ||
		strings.Contains(strings.ToLower(pet.PrimaryColor), q) ||
		strings.Contains(strings.ToLower(pet.Description), q) ||
		strings.Contains(strings.ToLower(pet.Location), q) {
		return true
	}
	return false
}

func matchesFoundPetQuery(pet domain.FoundPetRecord, q string) bool {
	q = strings.ToLower(q)
	if strings.Contains(strings.ToLower(pet.Breed), q) ||
		strings.Contains(strings.ToLower(pet.Species), q) ||
		strings.Contains(strings.ToLower(pet.PrimaryColor), q) ||
		strings.Contains(strings.ToLower(pet.SecondaryColor), q) ||
		strings.Contains(strings.ToLower(pet.Location), q) ||
		strings.Contains(strings.ToLower(string(pet.CustodyStatus)), q) {
		return true
	}
	for _, m := range pet.DistinctiveMarkings {
		if strings.Contains(strings.ToLower(m), q) {
			return true
		}
	}
	return false
}

func extractCoordinates(coords *domain.LocationPoint, locationStr string) (domain.LocationPoint, bool) {
	if coords != nil && coords.Validate() == nil {
		return *coords, true
	}
	if locationStr != "" {
		pt := domain.ParseLocationCoordinates(locationStr)
		if pt.Validate() == nil {
			return pt, true
		}
	}
	return domain.LocationPoint{}, false
}

func (s *Server) handlePets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	params, err := parseDirectoryQueryParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") || r.URL.Query().Get("format") == "json" {
		s.serveDirectoryJSON(w, r, params)
		return
	}

	pagedItems, totalCount, nextCursor, prevCursor, err := s.queryDirectoryPets(r.Context(), params)
	if err != nil {
		http.Error(w, "Failed to load pets directory", http.StatusInternalServerError)
		return
	}

	currentPage := 1
	if params.Limit > 0 {
		currentPage = (params.Offset / params.Limit) + 1
	}
	totalPages := 1
	if params.Limit > 0 && totalCount > 0 {
		totalPages = int(math.Ceil(float64(totalCount) / float64(params.Limit)))
	}

	var prevPageURL, nextPageURL string
	baseURL := "/pets"
	if prevCursor != "" || params.Offset > 0 {
		prevOffset := params.Offset - params.Limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		prevPageURL = buildDirectoryPageURL(baseURL, r.URL.Query(), prevOffset, prevCursor, params.Cursor != "")
	}
	if nextCursor != "" || params.Offset+params.Limit < totalCount {
		nextOffset := params.Offset + params.Limit
		nextPageURL = buildDirectoryPageURL(baseURL, r.URL.Query(), nextOffset, nextCursor, params.Cursor != "")
	}

	markers := make([]PetMapMarker, 0, len(pagedItems))
	for _, item := range pagedItems {
		if item.Coordinates != nil && (item.Coordinates.Latitude != 0 || item.Coordinates.Longitude != 0) {
			markers = append(markers, PetMapMarker{
				PetID:      item.PetID,
				Status:     item.Status,
				PetName:    item.PetName,
				Species:    item.Species,
				Breed:      item.Breed,
				Location:   item.Location,
				ImageURL:   item.ImageURL,
				ReportedAt: item.ReportedAt.Format("Jan 02, 2006"),
				Latitude:   item.Coordinates.Latitude,
				Longitude:  item.Coordinates.Longitude,
			})
		}
	}

	var petsJSON template.JS = "[]"
	if jsonBytes, err := json.Marshal(markers); err == nil {
		petsJSON = template.JS(jsonBytes)
	}

	data := PetsPageData{
		Pets:        pagedItems,
		TotalCount:  totalCount,
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		Species:     params.Species,
		Status:      params.Status,
		Query:       params.Query,
		Limit:       params.Limit,
		Offset:      params.Offset,
		HasPrev:     prevPageURL != "",
		HasNext:     nextPageURL != "",
		PrevPageURL: prevPageURL,
		NextPageURL: nextPageURL,
		NextCursor:  nextCursor,
		PrevCursor:  prevCursor,
		Lat:         params.GeoPoint.Latitude,
		Lng:         params.GeoPoint.Longitude,
		RadiusMiles: params.RadiusMiles,
		HasGeo:      params.HasGeo,
		MappedCount: len(markers),
		PetsJSON:    petsJSON,
	}

	tmpl, err := template.ParseFS(embeddedFiles, "templates/pets.html")
	if err != nil {
		http.Error(w, "Failed to load pets template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = tmpl.Execute(w, data)
}

func (s *Server) handleApiPets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	params, err := parseDirectoryQueryParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.serveDirectoryJSON(w, r, params)
}

func (s *Server) serveDirectoryJSON(w http.ResponseWriter, r *http.Request, params DirectoryQueryParams) {
	pagedItems, totalCount, nextCursor, prevCursor, err := s.queryDirectoryPets(r.Context(), params)
	if err != nil {
		http.Error(w, "Failed to query pets directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(totalCount))
	w.Header().Set("X-Limit", strconv.Itoa(params.Limit))
	w.Header().Set("X-Offset", strconv.Itoa(params.Offset))
	baseURL := "/pets"
	if nextCursor != "" {
		w.Header().Set("X-Next-Cursor", nextCursor)
		nextURL := buildDirectoryPageURL(baseURL, r.URL.Query(), params.Offset+params.Limit, nextCursor, true)
		w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
	}
	if prevCursor != "" {
		w.Header().Set("X-Prev-Cursor", prevCursor)
		prevOffset := params.Offset - params.Limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		prevURL := buildDirectoryPageURL(baseURL, r.URL.Query(), prevOffset, prevCursor, true)
		w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="prev"`, prevURL))
	}

	if pagedItems == nil {
		pagedItems = []PublicPetDirectoryItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(pagedItems)
}

func buildDirectoryPageURL(baseURL string, q url.Values, offset int, cursor string, useCursor bool) string {
	copyQ := make(url.Values)
	for k, v := range q {
		copyQ[k] = v
	}
	if useCursor && cursor != "" {
		copyQ.Set("cursor", cursor)
		copyQ.Del("offset")
	} else {
		copyQ.Set("offset", strconv.Itoa(offset))
		copyQ.Del("cursor")
	}
	encoded := copyQ.Encode()
	if encoded == "" {
		return baseURL
	}
	return baseURL + "?" + encoded
}

// SeedDemoPets populates demo lost and found pet reports for local/demo runs.
func SeedDemoPets(ctx context.Context, stateStore store.StateStore) error {
	now := time.Now().UTC()

	lostPets := []domain.LostPetRecord{
		{
			PetID:        "demo-lost-1",
			PetName:      "Rusty",
			Species:      "Dog",
			Breed:        "Golden Retriever",
			PrimaryColor: "Golden",
			Description:  "Friendly golden retriever wearing blue collar.",
			Location:     "Capitol Hill, Seattle, WA",
			ReportedAt:   now.Add(-1 * time.Hour),
			Status:       domain.LostPetStatusLost,
		},
		{
			PetID:        "lost-101",
			PetName:      "Buddy",
			Species:      "Dog",
			Breed:        "Golden Retriever",
			PrimaryColor: "Golden",
			Description:  "Friendly golden retriever wearing blue collar.",
			Location:     "Capitol Hill, Seattle, WA",
			ReportedAt:   now.Add(-2 * time.Hour),
			Status:       domain.LostPetStatusLost,
		},
		{
			PetID:        "lost-105",
			PetName:      "Luna",
			Species:      "Cat",
			Breed:        "Siamese Cat",
			PrimaryColor: "Cream",
			Description:  "Vocal Siamese cat with striking blue eyes.",
			Location:     "Ballard, Seattle, WA",
			ReportedAt:   now.Add(-5 * time.Hour),
			Status:       domain.LostPetStatusLost,
		},
		{
			PetID:        "lost-999-reunited",
			PetName:      "Max",
			Species:      "Dog",
			Breed:        "Beagle",
			PrimaryColor: "Tricolor",
			Description:  "Reunited with loving owner.",
			Location:     "Downtown, Seattle, WA",
			ReportedAt:   now.Add(-24 * time.Hour),
			Status:       domain.LostPetStatusReunited,
		},
	}

	foundPets := []domain.FoundPetRecord{
		{
			PetID:               "found-202",
			ImageURL:            "https://storage.petspotr.io/found-202.jpg",
			Location:            "Green Lake Park, Seattle, WA",
			Species:             "Dog",
			Breed:               "Golden Retriever",
			PrimaryColor:        "Golden",
			DistinctiveMarkings: []string{"White chest patch"},
			CustodyStatus:       domain.CustodyFinderHome,
			Status:              domain.FoundPetStatusFound,
			FoundAt:             now.Add(-15 * time.Minute),
		},
		{
			PetID:               "found-203",
			ImageURL:            "https://storage.petspotr.io/found-203.jpg",
			Location:            "Fremont, Seattle, WA",
			Species:             "Cat",
			Breed:               "Siamese Cat",
			PrimaryColor:        "Cream",
			DistinctiveMarkings: []string{"Dark ears", "Dark tail"},
			CustodyStatus:       domain.CustodyLocalShelter,
			Status:              domain.FoundPetStatusFound,
			FoundAt:             now.Add(-2 * time.Hour),
		},
		{
			PetID:         "found-999-resolved",
			ImageURL:      "https://storage.petspotr.io/found-999.jpg",
			Location:      "West Seattle, WA",
			Species:       "Cat",
			Breed:         "Tabby",
			CustodyStatus: domain.CustodyLocalShelter,
			Status:        domain.FoundPetStatusResolved,
			FoundAt:       now.Add(-48 * time.Hour),
		},
	}

	for _, p := range lostPets {
		p = domain.NormalizeLostPetRecord(p)
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal lost pet %s: %w", p.PetID, err)
		}
		if err := stateStore.SaveState(ctx, store.LostPetsCollection, p.PetID, data); err != nil {
			return fmt.Errorf("save lost pet %s: %w", p.PetID, err)
		}
	}

	for _, p := range foundPets {
		p = domain.NormalizeFoundPetRecord(p)
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal found pet %s: %w", p.PetID, err)
		}
		if err := stateStore.SaveState(ctx, store.FoundPetsCollection, p.PetID, data); err != nil {
			return fmt.Errorf("save found pet %s: %w", p.PetID, err)
		}
	}

	return nil
}
