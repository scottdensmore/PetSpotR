package webfrontend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// SearchPartyResponse represents the API response for search party operations,
// augmenting the core search party with lost pet metadata such as collar beacon configuration.
type SearchPartyResponse struct {
	searchparty.SearchParty
	CollarBeacon *domain.CollarBeaconConfig `json:"collarBeacon,omitempty"`
}

// handleApiLostPetSearchParty routes GET and POST requests for a lost pet's search party.
func (s *Server) handleApiLostPetSearchParty(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleApiCreateSearchParty(w, r)
	case http.MethodGet:
		s.handleApiGetSearchParty(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleApiCreateSearchParty initializes a new SearchParty if not existing.
func (s *Server) handleApiCreateSearchParty(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	// 1. Validate lost pet exists
	petBytes, err := s.stateStore.GetState(r.Context(), store.LostPetsCollection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve lost pet", http.StatusInternalServerError)
		return
	}

	var pet domain.LostPetRecord
	if err := json.Unmarshal(petBytes, &pet); err != nil {
		http.Error(w, "Failed to decode lost pet", http.StatusInternalServerError)
		return
	}
	pet = domain.NormalizeLostPetRecord(pet)

	// 2. Check if search party already exists for this pet
	existingParty, found, err := s.findSearchPartyByPetID(r.Context(), petID)
	if err != nil {
		http.Error(w, "Failed to check existing search party", http.StatusInternalServerError)
		return
	}
	if found {
		s.annotatePartySectorsWithUrgency(r.Context(), pet, &existingParty)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchPartyResponse{
			SearchParty:  existingParty,
			CollarBeacon: pet.CollarBeacon,
		})
		return
	}

	// 3. Load sightings to compute trajectory perimeter
	rawItems, err := s.stateStore.ListState(r.Context(), store.SightingsCollection)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Failed to list sightings", http.StatusInternalServerError)
		return
	}

	sightings := make([]domain.PetSightingRecord, 0)
	for _, b := range rawItems {
		var sRec domain.PetSightingRecord
		if err := json.Unmarshal(b, &sRec); err != nil {
			continue
		}
		if sRec.LostPetID == petID && sRec.Status == domain.SightingStatusActive {
			sightings = append(sightings, sRec)
		}
	}

	originCoords := pet.Coordinates
	if originCoords == nil && pet.Location != "" {
		if pt, ok := extractCoordinates(nil, pet.Location); ok {
			originCoords = &pt
		}
	}

	analysis := sighting.CalculateTrajectory(petID, originCoords, pet.ReportedAt, sightings)
	if analysis.EstimatedPerimeter == nil {
		http.Error(w, "Cannot create search party: pet has no location coordinates", http.StatusBadRequest)
		return
	}

	// 4. Parse optional request parameters
	type createRequest struct {
		SectorCount  int     `json:"sectorCount"`
		RadiusMeters float64 `json:"radiusMeters"`
	}
	var reqData createRequest
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&reqData)
	}

	sectorCount := 4
	if reqData.SectorCount > 0 {
		sectorCount = reqData.SectorCount
		if sectorCount > 32 {
			sectorCount = 32
		}
	}
	radiusMeters := analysis.EstimatedPerimeter.RadiusMeters
	if reqData.RadiusMeters > 0 {
		radiusMeters = reqData.RadiusMeters
	}
	if radiusMeters <= 0 {
		radiusMeters = 804.672 // ~0.5 miles default
	}
	center := analysis.EstimatedPerimeter.CenterCoordinates

	// 5. Decompose perimeter with directional bearing if available
	var sectors []searchparty.SearchSector
	if len(analysis.Legs) > 0 {
		lastBearing := analysis.Legs[len(analysis.Legs)-1].BearingDegrees
		sectors = searchparty.DecomposePerimeterWithBearing(&center, radiusMeters, sectorCount, lastBearing)
	} else {
		sectors = searchparty.DecomposePerimeter(&center, radiusMeters, sectorCount)
	}

	partyID := fmt.Sprintf("party-%s", petID)
	now := time.Now().UTC()
	party := searchparty.SearchParty{
		PartyID:               partyID,
		LostPetID:             petID,
		CenterCoordinates:     center,
		RadiusMeters:          radiusMeters,
		CreatedAt:             now,
		Sectors:               sectors,
		ActiveAssignments:     []searchparty.SectorAssignment{},
		CoveragePercentage:    0.0,
		ActiveVolunteersCount: 0,
	}

	s.annotatePartySectorsWithUrgency(r.Context(), pet, &party)

	partyBytes, err := json.Marshal(party)
	if err != nil {
		http.Error(w, "Failed to serialize search party", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.SearchPartiesCollection, partyID, partyBytes); err != nil {
		http.Error(w, "Failed to save search party", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(SearchPartyResponse{
		SearchParty:  party,
		CollarBeacon: pet.CollarBeacon,
	})
}

// handleApiGetSearchParty returns the search party for the given lost pet.
func (s *Server) handleApiGetSearchParty(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	// Validate lost pet exists
	petBytes, err := s.stateStore.GetState(r.Context(), store.LostPetsCollection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve lost pet", http.StatusInternalServerError)
		return
	}

	var pet domain.LostPetRecord
	if err := json.Unmarshal(petBytes, &pet); err != nil {
		http.Error(w, "Failed to decode lost pet", http.StatusInternalServerError)
		return
	}

	party, found, err := s.findSearchPartyByPetID(r.Context(), petID)
	if err != nil {
		http.Error(w, "Failed to load search party", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	party.CoveragePercentage = party.CalculateCoverage()
	party.ActiveVolunteersCount = countActiveVolunteers(party.ActiveAssignments)

	s.annotatePartySectorsWithUrgency(r.Context(), pet, &party)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SearchPartyResponse{
		SearchParty:  party,
		CollarBeacon: pet.CollarBeacon,
	})
}

// handleApiClaimSector allows a volunteer to claim a sector.
func (s *Server) handleApiClaimSector(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	partyID := strings.TrimSpace(r.PathValue("partyID"))
	sectorID := strings.TrimSpace(r.PathValue("sectorID"))
	if partyID == "" || sectorID == "" {
		http.NotFound(w, r)
		return
	}

	party, found, err := s.getSearchPartyByID(r.Context(), partyID)
	if err != nil {
		http.Error(w, "Failed to retrieve search party", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	sectorIdx := -1
	for i, sec := range party.Sectors {
		if sec.SectorID == sectorID {
			sectorIdx = i
			break
		}
	}
	if sectorIdx == -1 {
		http.NotFound(w, r)
		return
	}

	// Check if already claimed, cleared, or in sighting_reported
	currentStatus := party.Sectors[sectorIdx].Status
	if currentStatus != searchparty.SectorStatusUnassigned {
		http.Error(w, "Sector is already claimed or cleared", http.StatusConflict)
		return
	}

	type claimRequest struct {
		VolunteerAlias string `json:"volunteerAlias"`
	}
	var req claimRequest
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&req)
	}

	alias := sanitizeVolunteerAlias(req.VolunteerAlias, len(party.ActiveAssignments)+1)
	assignmentID := generateAssignmentID()
	now := time.Now().UTC()

	assignment := searchparty.SectorAssignment{
		AssignmentID:   assignmentID,
		SectorID:       sectorID,
		VolunteerAlias: alias,
		ClaimedAt:      now,
		UpdatedAt:      now,
		Status:         searchparty.SectorStatusActiveSearch,
	}

	// Save assignment to store
	asgnBytes, err := json.Marshal(assignment)
	if err != nil {
		http.Error(w, "Failed to serialize assignment", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.SectorAssignmentsCollection, assignmentID, asgnBytes); err != nil {
		http.Error(w, "Failed to save assignment", http.StatusInternalServerError)
		return
	}

	// Update party state
	party.Sectors[sectorIdx].Status = searchparty.SectorStatusActiveSearch
	party.ActiveAssignments = append(party.ActiveAssignments, assignment)
	party.CoveragePercentage = party.CalculateCoverage()
	party.ActiveVolunteersCount = countActiveVolunteers(party.ActiveAssignments)

	partyBytes, err := json.Marshal(party)
	if err != nil {
		http.Error(w, "Failed to serialize party", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.SearchPartiesCollection, party.PartyID, partyBytes); err != nil {
		http.Error(w, "Failed to update party", http.StatusInternalServerError)
		return
	}

	s.broadcastSearchPartyUpdate(r.Context(), party, sectorID, searchparty.SectorStatusActiveSearch)

	resp := struct {
		searchparty.SectorAssignment
		PartyID               string  `json:"partyId"`
		CoveragePercentage    float64 `json:"coveragePercentage"`
		ActiveVolunteersCount int     `json:"activeVolunteersCount"`
	}{
		SectorAssignment:      assignment,
		PartyID:               party.PartyID,
		CoveragePercentage:    party.CoveragePercentage,
		ActiveVolunteersCount: party.ActiveVolunteersCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleApiUpdateSectorStatus updates the status of a sector in a search party.
func (s *Server) handleApiUpdateSectorStatus(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	partyID := strings.TrimSpace(r.PathValue("partyID"))
	sectorID := strings.TrimSpace(r.PathValue("sectorID"))
	if partyID == "" || sectorID == "" {
		http.NotFound(w, r)
		return
	}

	type updateStatusRequest struct {
		Status         searchparty.SectorStatus `json:"status"`
		ClearanceNotes string                   `json:"clearanceNotes"`
	}
	var req updateStatusRequest
	if r.Body == nil {
		http.Error(w, "Missing request body", http.StatusBadRequest)
		return
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := req.Status.Validate(); err != nil {
		http.Error(w, "Invalid sector status: "+err.Error(), http.StatusBadRequest)
		return
	}

	party, found, err := s.getSearchPartyByID(r.Context(), partyID)
	if err != nil {
		http.Error(w, "Failed to retrieve search party", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	sectorIdx := -1
	for i, sec := range party.Sectors {
		if sec.SectorID == sectorID {
			sectorIdx = i
			break
		}
	}
	if sectorIdx == -1 {
		http.NotFound(w, r)
		return
	}

	// Update sector status
	party.Sectors[sectorIdx].Status = req.Status

	// Update assignment if present
	now := time.Now().UTC()
	for i := range party.ActiveAssignments {
		if party.ActiveAssignments[i].SectorID == sectorID {
			party.ActiveAssignments[i].Status = req.Status
			party.ActiveAssignments[i].UpdatedAt = now
			if req.ClearanceNotes != "" {
				party.ActiveAssignments[i].ClearanceNotes = req.ClearanceNotes
			}
			asgnBytes, err := json.Marshal(party.ActiveAssignments[i])
			if err == nil {
				_ = s.stateStore.SaveState(r.Context(), store.SectorAssignmentsCollection, party.ActiveAssignments[i].AssignmentID, asgnBytes)
			}
		}
	}

	party.CoveragePercentage = party.CalculateCoverage()
	party.ActiveVolunteersCount = countActiveVolunteers(party.ActiveAssignments)

	partyBytes, err := json.Marshal(party)
	if err != nil {
		http.Error(w, "Failed to serialize party", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.SearchPartiesCollection, party.PartyID, partyBytes); err != nil {
		http.Error(w, "Failed to update party", http.StatusInternalServerError)
		return
	}

	s.broadcastSearchPartyUpdate(r.Context(), party, sectorID, req.Status)

	resp := struct {
		PartyID               string                   `json:"partyId"`
		SectorID              string                   `json:"sectorId"`
		Status                searchparty.SectorStatus `json:"status"`
		ClearanceNotes        string                   `json:"clearanceNotes,omitempty"`
		CoveragePercentage    float64                  `json:"coveragePercentage"`
		ActiveVolunteersCount int                      `json:"activeVolunteersCount"`
	}{
		PartyID:               party.PartyID,
		SectorID:              sectorID,
		Status:                req.Status,
		ClearanceNotes:        req.ClearanceNotes,
		CoveragePercentage:    party.CoveragePercentage,
		ActiveVolunteersCount: party.ActiveVolunteersCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// findSearchPartyByPetID locates a search party associated with petID.
func (s *Server) findSearchPartyByPetID(ctx context.Context, petID string) (searchparty.SearchParty, bool, error) {
	if s.stateStore == nil {
		return searchparty.SearchParty{}, false, errors.New("state store uninitialized")
	}

	// Try direct lookup by standard key
	data, err := s.stateStore.GetState(ctx, store.SearchPartiesCollection, "party-"+petID)
	if err == nil {
		var p searchparty.SearchParty
		if err := json.Unmarshal(data, &p); err == nil && p.LostPetID == petID {
			return p, true, nil
		}
	}

	// Fallback: list all search parties
	parties, err := s.stateStore.ListState(ctx, store.SearchPartiesCollection)
	if err != nil {
		if errors.Is(err, store.ErrStoreNotFound) || errors.Is(err, store.ErrNotFound) {
			return searchparty.SearchParty{}, false, nil
		}
		return searchparty.SearchParty{}, false, err
	}

	for _, raw := range parties {
		var p searchparty.SearchParty
		if err := json.Unmarshal(raw, &p); err == nil && p.LostPetID == petID {
			return p, true, nil
		}
	}

	return searchparty.SearchParty{}, false, nil
}

// getSearchPartyByID retrieves a search party by its partyID.
func (s *Server) getSearchPartyByID(ctx context.Context, partyID string) (searchparty.SearchParty, bool, error) {
	if s.stateStore == nil {
		return searchparty.SearchParty{}, false, errors.New("state store uninitialized")
	}

	data, err := s.stateStore.GetState(ctx, store.SearchPartiesCollection, partyID)
	if err == nil {
		var p searchparty.SearchParty
		if err := json.Unmarshal(data, &p); err == nil {
			return p, true, nil
		}
	}
	if !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		return searchparty.SearchParty{}, false, err
	}

	parties, err := s.stateStore.ListState(ctx, store.SearchPartiesCollection)
	if err != nil {
		if errors.Is(err, store.ErrStoreNotFound) || errors.Is(err, store.ErrNotFound) {
			return searchparty.SearchParty{}, false, nil
		}
		return searchparty.SearchParty{}, false, err
	}

	for _, raw := range parties {
		var p searchparty.SearchParty
		if err := json.Unmarshal(raw, &p); err == nil && p.PartyID == partyID {
			return p, true, nil
		}
	}

	return searchparty.SearchParty{}, false, nil
}

// countActiveVolunteers counts distinct volunteer aliases with active search or sighting reported status.
func countActiveVolunteers(assignments []searchparty.SectorAssignment) int {
	active := make(map[string]struct{})
	for _, a := range assignments {
		if (a.Status == searchparty.SectorStatusActiveSearch || a.Status == searchparty.SectorStatusSightingReported) && a.VolunteerAlias != "" {
			active[a.VolunteerAlias] = struct{}{}
		}
	}
	return len(active)
}

func generateAssignmentID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("asgn-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("asgn-%s", hex.EncodeToString(random[:]))
}

func (s *Server) broadcastSearchPartyUpdate(ctx context.Context, party searchparty.SearchParty, sectorID string, status searchparty.SectorStatus) {
	if s.reunionHub == nil {
		return
	}

	payload := domain.SearchPartyEventPayload{
		Type:                  "search_party_updated",
		PartyID:               party.PartyID,
		SectorID:              sectorID,
		Status:                string(status),
		CoveragePercentage:    party.CoveragePercentage,
		ActiveVolunteersCount: party.ActiveVolunteersCount,
	}

	now := time.Now().UTC()
	event := domain.ReunionStreamEvent{
		EventID:   fmt.Sprintf("evt_search_party_%s_%d", party.PartyID, now.UnixNano()),
		Type:      domain.ReunionEventSearchPartyUpdated,
		MatchID:   party.LostPetID,
		Timestamp: now,
		Payload:   payload,
	}

	// Broadcast for lost pet subscribers
	s.reunionHub.Broadcast(event)

	// Broadcast for party subscribers (if party.PartyID differs from lostPetID)
	if party.PartyID != party.LostPetID {
		partyEvent := event
		partyEvent.MatchID = party.PartyID
		s.reunionHub.Broadcast(partyEvent)
	}

	// Broadcast to any active matches associated with this pet
	if party.LostPetID != "" && s.stateStore != nil {
		if matchBytes, err := s.stateStore.ListState(ctx, store.MatchesCollection); err == nil {
			for _, mb := range matchBytes {
				var m domain.MatchRecord
				if err := json.Unmarshal(mb, &m); err == nil {
					if m.LostPetID == party.LostPetID || m.MatchedPetID == party.LostPetID || m.LostPet.PetID == party.LostPetID {
						matchEvent := event
						matchEvent.MatchID = m.MatchID
						s.reunionHub.Broadcast(matchEvent)
					}
				}
			}
		}
	}
}

// sanitizeVolunteerAlias enforces Zero-PII by replacing contact info with pseudonyms.
func sanitizeVolunteerAlias(raw string, seq int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Sprintf("Volunteer #%d", seq)
	}
	if strings.Contains(trimmed, "@") {
		return fmt.Sprintf("Volunteer #%d", seq)
	}

	digitCount := 0
	for _, r := range trimmed {
		if unicode.IsDigit(r) {
			digitCount++
		}
	}
	if digitCount > 6 {
		return fmt.Sprintf("Volunteer #%d", seq)
	}

	if len(trimmed) > 40 {
		trimmed = trimmed[:40]
	}
	return trimmed
}

// annotatePartySectorsWithUrgency scores each sector against predictive trajectory isochrones
// and sets priorityScore, urgencyLevel, and hidingClusterIds.
func (s *Server) annotatePartySectorsWithUrgency(ctx context.Context, pet domain.LostPetRecord, party *searchparty.SearchParty) {
	if party == nil || len(party.Sectors) == 0 {
		return
	}

	originCoords := pet.Coordinates
	if originCoords == nil && pet.Location != "" {
		if pt, ok := extractCoordinates(nil, pet.Location); ok {
			originCoords = &pt
		}
	}

	sightings := s.getActiveSightingsForPet(ctx, pet.PetID)
	if originCoords == nil && len(sightings) == 0 {
		return
	}

	species := strings.ToLower(strings.TrimSpace(pet.Species))
	if species == "" {
		species = "dog"
	}

	elapsedHours := 1.0
	var lastTime time.Time
	if len(sightings) > 0 {
		for _, sRec := range sightings {
			if sRec.SightedAt.After(lastTime) {
				lastTime = sRec.SightedAt
			}
		}
	} else if !pet.ReportedAt.IsZero() {
		lastTime = pet.ReportedAt
	}
	if !lastTime.IsZero() {
		diff := time.Since(lastTime)
		if diff > 0 {
			elapsedHours = diff.Hours()
		}
	}
	if elapsedHours <= 0 {
		elapsedHours = 1.0
	}

	predResult := sighting.GeneratePredictiveTrajectory(pet.PetID, species, originCoords, sightings, elapsedHours)

	for i := range party.Sectors {
		dSec := domain.SearchSector{
			SectorID:        party.Sectors[i].SectorID,
			Name:            party.Sectors[i].Name,
			BoundingPolygon: party.Sectors[i].PolygonPoints,
			PolygonPoints:   party.Sectors[i].PolygonPoints,
			Status:          string(party.Sectors[i].Status),
			TotalAreaSqM:    party.Sectors[i].TotalAreaSqM,
		}
		score, urgency := sighting.ScoreSectorUrgency(dSec, predResult.Isochrones)
		party.Sectors[i].PriorityScore = score
		party.Sectors[i].UrgencyLevel = urgency

		var clusterIDs []string
		for _, hc := range predResult.HidingClusters {
			bcPt := searchparty.BreadcrumbPoint{
				Latitude:  hc.Centroid.Latitude,
				Longitude: hc.Centroid.Longitude,
			}
			if searchparty.PointInSectorPolygon(bcPt, party.Sectors[i].PolygonPoints) {
				clusterIDs = append(clusterIDs, hc.ID)
			}
		}
		party.Sectors[i].HidingClusterIDs = clusterIDs
	}
}
