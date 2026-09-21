package webfrontend

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sheltersync"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func defaultEvacuationHubs(now time.Time) []domain.EvacuationHub {
	return []domain.EvacuationHub{
		{
			HubID:            "hub-seattle-center",
			Name:             "Seattle Center Exhibition Hall",
			Type:             domain.HubTypePopUpCrisisCenter,
			Status:           domain.HubStatusActive,
			Address:          "301 Mercer St, Seattle, WA 98109",
			Coordinates:      domain.LocationPoint{Latitude: 47.6242, Longitude: -122.3518},
			TotalCapacity:    150,
			CurrentOccupancy: 0,
			DogCapacity:      90,
			DogOccupancy:     0,
			CatCapacity:      60,
			CatOccupancy:     0,
			ContactName:      "Seattle Emergency Shelter Command",
			ContactPhone:     "206-555-0100",
			ContactEmail:     "evac-seattlecenter@seattle.gov",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			HubID:            "hub-magnuson-park",
			Name:             "Magnuson Park Community Staging",
			Type:             domain.HubTypeFairgroundStaging,
			Status:           domain.HubStatusActive,
			Address:          "7400 Sand Point Way NE, Seattle, WA 98115",
			Coordinates:      domain.LocationPoint{Latitude: 47.6833, Longitude: -122.2577},
			TotalCapacity:    100,
			CurrentOccupancy: 0,
			DogCapacity:      60,
			DogOccupancy:     0,
			CatCapacity:      40,
			CatOccupancy:     0,
			ContactName:      "Magnuson Crisis Logistics",
			ContactPhone:     "206-555-0102",
			ContactEmail:     "evac-magnuson@kingcounty.gov",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			HubID:            "hub-south-king-fairgrounds",
			Name:             "South King County Fairgrounds",
			Type:             domain.HubTypePermanentShelter,
			Status:           domain.HubStatusActive,
			Address:          "45224 284th Ave SE, Enumclaw, WA 98022",
			Coordinates:      domain.LocationPoint{Latitude: 47.2045, Longitude: -121.9912},
			TotalCapacity:    200,
			CurrentOccupancy: 0,
			DogCapacity:      120,
			DogOccupancy:     0,
			CatCapacity:      80,
			CatOccupancy:     0,
			ContactName:      "Regional Agricultural Evac Center",
			ContactPhone:     "360-555-0199",
			ContactEmail:     "evac-fairgrounds@kingcounty.gov",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
}

// handleApiEvacuationHubs handles GET and POST requests for disaster evacuation hubs.
func (s *Server) handleApiEvacuationHubs(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleListEvacuationHubs(w, r)
	case http.MethodPost:
		s.handleCreateEvacuationHub(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListEvacuationHubs(w http.ResponseWriter, r *http.Request) {
	rawHubs, err := s.stateStore.ListState(r.Context(), store.EvacuationHubsCollection)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "failed to list evacuation hubs", http.StatusInternalServerError)
		return
	}

	var hubs []domain.EvacuationHub
	now := time.Now().UTC()

	if len(rawHubs) == 0 {
		defaults := defaultEvacuationHubs(now)
		for _, dh := range defaults {
			data, err := json.Marshal(dh)
			if err == nil {
				_ = s.stateStore.SaveState(r.Context(), store.EvacuationHubsCollection, dh.HubID, data)
				hubs = append(hubs, dh)
			}
		}
	} else {
		for _, raw := range rawHubs {
			var hub domain.EvacuationHub
			if err := json.Unmarshal(raw, &hub); err == nil {
				hubs = append(hubs, hub)
			}
		}
	}

	sort.Slice(hubs, func(i, j int) bool {
		return hubs[i].Name < hubs[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(hubs)
}

func (s *Server) handleCreateEvacuationHub(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var hub domain.EvacuationHub
	if err := json.NewDecoder(r.Body).Decode(&hub); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	hub.Name = strings.TrimSpace(hub.Name)
	if hub.Name == "" {
		http.Error(w, "hub name is required", http.StatusBadRequest)
		return
	}
	if hub.TotalCapacity <= 0 {
		http.Error(w, "totalCapacity must be greater than zero", http.StatusBadRequest)
		return
	}

	hub.HubID = strings.TrimSpace(hub.HubID)
	if hub.HubID == "" {
		hub.HubID = fmt.Sprintf("hub-%s-%s", sanitizeID(hub.Name), randomHex(4))
	}
	if hub.Status == "" {
		hub.Status = domain.HubStatusActive
	}
	if hub.Type == "" {
		hub.Type = domain.HubTypePopUpCrisisCenter
	}
	now := time.Now().UTC()
	hub.CreatedAt = now
	hub.UpdatedAt = now

	data, err := json.Marshal(hub)
	if err != nil {
		http.Error(w, "failed to marshal evacuation hub", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.EvacuationHubsCollection, hub.HubID, data); err != nil {
		http.Error(w, "failed to save evacuation hub", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(hub)
}

// handleApiEvacuationIntakeBatch handles ingestion of animal rosters via CSV or JSON.
func (s *Server) handleApiEvacuationIntakeBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	contentType := r.Header.Get("Content-Type")
	var summary sheltersync.BulkIntakeBatchSummary
	var petRecords []domain.FoundPetRecord
	var hubID string
	var hubName string
	var parseErr error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, fmt.Sprintf("failed to parse multipart form: %v", err), http.StatusBadRequest)
			return
		}
		hubID = strings.TrimSpace(r.FormValue("hubId"))
		if hubID == "" {
			hubID = strings.TrimSpace(r.FormValue("hub_id"))
		}
		hubName = strings.TrimSpace(r.FormValue("hubName"))
		if hubName == "" {
			hubName = strings.TrimSpace(r.FormValue("hub_name"))
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file is required in multipart upload", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		fileBytes, err := io.ReadAll(http.MaxBytesReader(w, file, 10<<20))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read file: %v", err), http.StatusBadRequest)
			return
		}

		isJSON := strings.HasSuffix(strings.ToLower(header.Filename), ".json") ||
			bytes.HasPrefix(bytes.TrimSpace(fileBytes), []byte("{")) ||
			bytes.HasPrefix(bytes.TrimSpace(fileBytes), []byte("["))

		if isJSON {
			summary, petRecords, parseErr = sheltersync.ParseBulkIntakeJSON(fileBytes, hubID, hubName)
		} else {
			summary, petRecords, parseErr = sheltersync.ParseBulkIntakeCSV(bytes.NewReader(fileBytes), hubID, hubName)
		}
		if parseErr != nil {
			http.Error(w, fmt.Sprintf("failed to parse intake file: %v", parseErr), http.StatusBadRequest)
			return
		}
	} else {
		// application/json
		bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
			return
		}

		hubID = strings.TrimSpace(r.URL.Query().Get("hubId"))
		if hubID == "" {
			hubID = strings.TrimSpace(r.URL.Query().Get("hub_id"))
		}

		var payloadWrapper struct {
			HubID   string `json:"hubId"`
			HubName string `json:"hubName"`
		}
		_ = json.Unmarshal(bodyBytes, &payloadWrapper)
		if hubID == "" && payloadWrapper.HubID != "" {
			hubID = strings.TrimSpace(payloadWrapper.HubID)
		}
		if hubName == "" && payloadWrapper.HubName != "" {
			hubName = strings.TrimSpace(payloadWrapper.HubName)
		}

		summary, petRecords, parseErr = sheltersync.ParseBulkIntakeJSON(bodyBytes, hubID, hubName)
		if parseErr != nil {
			http.Error(w, fmt.Sprintf("failed to parse JSON intake: %v", parseErr), http.StatusBadRequest)
			return
		}
	}

	if hubID == "" {
		hubID = summary.HubID
	}
	if hubID == "" {
		http.Error(w, "hubId is required", http.StatusBadRequest)
		return
	}

	// Validate target hub exists in store
	targetHubBytes, err := s.stateStore.GetState(r.Context(), store.EvacuationHubsCollection, hubID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.Error(w, fmt.Sprintf("evacuation hub not found: %s", hubID), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to query evacuation hub", http.StatusInternalServerError)
		return
	}
	var targetHub domain.EvacuationHub
	if err := json.Unmarshal(targetHubBytes, &targetHub); err == nil {
		if summary.HubName == "" {
			summary.HubName = targetHub.Name
		}
	}

	// Persist valid FoundPetRecords
	for _, rec := range petRecords {
		recBytes, err := json.Marshal(rec)
		if err == nil {
			_ = s.stateStore.SaveState(r.Context(), store.FoundPetsCollection, rec.PetID, recBytes)
		}
	}

	// Check LostPetsCollection for instant microchip matches
	rawLostPets, err := s.stateStore.ListState(r.Context(), store.LostPetsCollection)
	if err == nil && len(rawLostPets) > 0 {
		lostChipMap := make(map[string]domain.LostPetRecord)
		for _, raw := range rawLostPets {
			var lp domain.LostPetRecord
			if err := json.Unmarshal(raw, &lp); err == nil {
				if lp.Status.IsActive() && strings.TrimSpace(lp.MicrochipID) != "" {
					lostChipMap[strings.ToUpper(strings.TrimSpace(lp.MicrochipID))] = lp
				}
			}
		}

		for i := range summary.Results {
			res := &summary.Results[i]
			if res.Status != "ERROR" && res.Microchip != "" && res.MicrochipValid {
				chipKey := strings.ToUpper(strings.TrimSpace(res.Microchip))
				if lostPet, exists := lostChipMap[chipKey]; exists {
					summary.InstantMatches++
					res.MatchedLostPetID = lostPet.PetID

					ownerName := "Registered Owner"
					if lostPet.OwnedBy != nil && lostPet.OwnedBy.Subject != "" {
						ownerName = lostPet.OwnedBy.Subject
					}
					ownerContact := ""
					if lostPet.OwnerIdentityRef != "" {
						if contactData, err := s.stateStore.GetState(r.Context(), store.ReportContactsCollection, lostPet.OwnerIdentityRef); err == nil {
							var c domain.ReportContact
							if err := json.Unmarshal(contactData, &c); err == nil {
								if c.Phone != "" {
									ownerContact = c.Phone
								} else if c.Email != "" {
									ownerContact = c.Email
								}
								if ownerName == "Registered Owner" && c.Email != "" {
									ownerName = c.Email
								}
							}
						}
					}

					petName := lostPet.PetName
					if petName == "" {
						petName = "Displaced Pet"
					}

					matchID := fmt.Sprintf("crisis-match-%s-%s", res.PetID, lostPet.PetID)
					item := domain.CrisisReunificationItem{
						MatchID:         matchID,
						FoundPetID:      res.PetID,
						LostPetID:       lostPet.PetID,
						PetName:         petName,
						Species:         lostPet.Species,
						Breed:           lostPet.Breed,
						CurrentHubID:    summary.HubID,
						CurrentHubName:  summary.HubName,
						OwnerName:       ownerName,
						OwnerContact:    ownerContact,
						MicrochipID:     res.Microchip,
						Priority:        domain.CrisisPriorityMicrochipMatch,
						SimilarityScore: 1.0,
						Status:          "PENDING",
						IdentifiedAt:    time.Now().UTC(),
					}
					itemBytes, _ := json.Marshal(item)
					_ = s.stateStore.SaveState(r.Context(), store.CrisisReunificationsCollection, matchID, itemBytes)
				}
			}
		}
	}

	// Increment target hub occupancy
	dogsCount := 0
	catsCount := 0
	for _, rec := range petRecords {
		if strings.EqualFold(rec.Species, "dog") {
			dogsCount++
		} else if strings.EqualFold(rec.Species, "cat") {
			catsCount++
		}
	}
	targetHub.CurrentOccupancy += summary.IngestedCount
	targetHub.DogOccupancy += dogsCount
	targetHub.CatOccupancy += catsCount
	if targetHub.CurrentOccupancy >= targetHub.TotalCapacity {
		targetHub.Status = domain.HubStatusFull
	}
	targetHub.UpdatedAt = time.Now().UTC()
	updatedHubBytes, _ := json.Marshal(targetHub)
	_ = s.stateStore.SaveState(r.Context(), store.EvacuationHubsCollection, targetHub.HubID, updatedHubBytes)

	// Persist batch summary in CrisisIntakesCollection
	sumBytes, _ := json.Marshal(summary)
	_ = s.stateStore.SaveState(r.Context(), store.CrisisIntakesCollection, summary.BatchID, sumBytes)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(summary)
}

// handleApiEvacuationTransfers handles GET and POST requests for transfer manifests.
func (s *Server) handleApiEvacuationTransfers(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleListEvacuationTransfers(w, r)
	case http.MethodPost:
		s.handleCreateEvacuationTransfer(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListEvacuationTransfers(w http.ResponseWriter, r *http.Request) {
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	rawManifests, err := s.stateStore.ListState(r.Context(), store.TransferManifestsCollection)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "failed to list transfer manifests", http.StatusInternalServerError)
		return
	}

	manifests := make([]domain.TransferManifest, 0)
	for _, raw := range rawManifests {
		var m domain.TransferManifest
		if err := json.Unmarshal(raw, &m); err == nil {
			if statusFilter == "" || strings.EqualFold(string(m.Status), statusFilter) {
				manifests = append(manifests, m)
			}
		}
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].CreatedAt.After(manifests[j].CreatedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(manifests)
}

func (s *Server) handleCreateEvacuationTransfer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var manifest domain.TransferManifest
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	manifest.OriginHubID = strings.TrimSpace(manifest.OriginHubID)
	manifest.DestHubID = strings.TrimSpace(manifest.DestHubID)

	if manifest.OriginHubID == "" || manifest.DestHubID == "" {
		http.Error(w, "originHubId and destHubId are required", http.StatusBadRequest)
		return
	}
	if manifest.OriginHubID == manifest.DestHubID {
		http.Error(w, "originHubId and destHubId cannot be identical", http.StatusBadRequest)
		return
	}
	if len(manifest.AnimalIDs) == 0 {
		http.Error(w, "at least one animalId is required", http.StatusBadRequest)
		return
	}

	// Validate origin hub
	origBytes, err := s.stateStore.GetState(r.Context(), store.EvacuationHubsCollection, manifest.OriginHubID)
	if err != nil {
		http.Error(w, fmt.Sprintf("origin hub not found: %s", manifest.OriginHubID), http.StatusBadRequest)
		return
	}
	var origHub domain.EvacuationHub
	if err := json.Unmarshal(origBytes, &origHub); err == nil {
		if manifest.OriginHubName == "" {
			manifest.OriginHubName = origHub.Name
		}
	}

	// Validate destination hub
	destBytes, err := s.stateStore.GetState(r.Context(), store.EvacuationHubsCollection, manifest.DestHubID)
	if err != nil {
		http.Error(w, fmt.Sprintf("destination hub not found: %s", manifest.DestHubID), http.StatusBadRequest)
		return
	}
	var destHub domain.EvacuationHub
	if err := json.Unmarshal(destBytes, &destHub); err == nil {
		if manifest.DestHubName == "" {
			manifest.DestHubName = destHub.Name
		}
	}

	manifest.TotalAnimals = len(manifest.AnimalIDs)
	manifest.TransferID = strings.TrimSpace(manifest.TransferID)
	if manifest.TransferID == "" {
		manifest.TransferID = fmt.Sprintf("xfer-%d-%s", time.Now().Unix(), randomHex(4))
	}
	manifest.Status = domain.TransferStatusStaged
	now := time.Now().UTC()
	manifest.CreatedAt = now
	manifest.UpdatedAt = now

	data, err := json.Marshal(manifest)
	if err != nil {
		http.Error(w, "failed to marshal transfer manifest", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.TransferManifestsCollection, manifest.TransferID, data); err != nil {
		http.Error(w, "failed to save transfer manifest", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(manifest)
}

// handleApiEvacuationTransferStatus handles PUT requests to update a transfer's lifecycle status.
func (s *Server) handleApiEvacuationTransferStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.NotFound(w, r)
		return
	}

	manifestBytes, err := s.stateStore.GetState(r.Context(), store.TransferManifestsCollection, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get transfer manifest", http.StatusInternalServerError)
		return
	}

	var manifest domain.TransferManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		http.Error(w, "failed to parse transfer manifest", http.StatusInternalServerError)
		return
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	targetStatus := domain.TransferStatus(strings.ToUpper(strings.TrimSpace(payload.Status)))
	now := time.Now().UTC()

	switch targetStatus {
	case domain.TransferStatusInTransit:
		manifest.DepartureTime = &now
		// Decrement origin hub occupancy
		if origBytes, err := s.stateStore.GetState(r.Context(), store.EvacuationHubsCollection, manifest.OriginHubID); err == nil {
			var origHub domain.EvacuationHub
			if err := json.Unmarshal(origBytes, &origHub); err == nil {
				dogCount := 0
				catCount := 0
				for _, petID := range manifest.AnimalIDs {
					if petBytes, err := s.stateStore.GetState(r.Context(), store.FoundPetsCollection, petID); err == nil {
						var pet domain.FoundPetRecord
						if err := json.Unmarshal(petBytes, &pet); err == nil {
							if strings.EqualFold(pet.Species, "dog") {
								dogCount++
							} else if strings.EqualFold(pet.Species, "cat") {
								catCount++
							}
						}
					}
				}
				origHub.CurrentOccupancy = max(0, origHub.CurrentOccupancy-manifest.TotalAnimals)
				origHub.DogOccupancy = max(0, origHub.DogOccupancy-dogCount)
				origHub.CatOccupancy = max(0, origHub.CatOccupancy-catCount)
				if origHub.CurrentOccupancy < origHub.TotalCapacity && origHub.Status == domain.HubStatusFull {
					origHub.Status = domain.HubStatusActive
				}
				origHub.UpdatedAt = now
				updatedOrigBytes, _ := json.Marshal(origHub)
				_ = s.stateStore.SaveState(r.Context(), store.EvacuationHubsCollection, origHub.HubID, updatedOrigBytes)
			}
		}

	case domain.TransferStatusReceived:
		manifest.ArrivalTime = &now
		// Increment destination hub occupancy and update pet records
		if destBytes, err := s.stateStore.GetState(r.Context(), store.EvacuationHubsCollection, manifest.DestHubID); err == nil {
			var destHub domain.EvacuationHub
			if err := json.Unmarshal(destBytes, &destHub); err == nil {
				dogCount := 0
				catCount := 0
				for _, petID := range manifest.AnimalIDs {
					if petBytes, err := s.stateStore.GetState(r.Context(), store.FoundPetsCollection, petID); err == nil {
						var pet domain.FoundPetRecord
						if err := json.Unmarshal(petBytes, &pet); err == nil {
							if strings.EqualFold(pet.Species, "dog") {
								dogCount++
							} else if strings.EqualFold(pet.Species, "cat") {
								catCount++
							}
							pet.Location = destHub.Address
							if pet.Location == "" {
								pet.Location = destHub.Name
							}
							pet.ShelterID = destHub.HubID
							pet.ShelterName = destHub.Name
							if destHub.Coordinates.Latitude != 0 || destHub.Coordinates.Longitude != 0 {
								pet.Coordinates = &destHub.Coordinates
								pet.GeocodingStatus = domain.GeocodingVerified
							}
							updatedPetBytes, _ := json.Marshal(pet)
							_ = s.stateStore.SaveState(r.Context(), store.FoundPetsCollection, pet.PetID, updatedPetBytes)
						}
					}
				}
				destHub.CurrentOccupancy += manifest.TotalAnimals
				destHub.DogOccupancy += dogCount
				destHub.CatOccupancy += catCount
				if destHub.CurrentOccupancy >= destHub.TotalCapacity {
					destHub.Status = domain.HubStatusFull
				}
				destHub.UpdatedAt = now
				updatedDestBytes, _ := json.Marshal(destHub)
				_ = s.stateStore.SaveState(r.Context(), store.EvacuationHubsCollection, destHub.HubID, updatedDestBytes)
			}
		}

	case domain.TransferStatusReconciled:
		// Reconciled - marks transfer finalized

	default:
		http.Error(w, fmt.Sprintf("invalid transfer status: %s", payload.Status), http.StatusBadRequest)
		return
	}

	manifest.Status = targetStatus
	manifest.UpdatedAt = now
	updatedData, err := json.Marshal(manifest)
	if err != nil {
		http.Error(w, "failed to marshal transfer manifest", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.TransferManifestsCollection, manifest.TransferID, updatedData); err != nil {
		http.Error(w, "failed to update transfer manifest", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(manifest)
}

// handleApiEvacuationReunificationQueue handles GET requests for the disaster reunification queue.
func (s *Server) handleApiEvacuationReunificationQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	rawItems, err := s.stateStore.ListState(r.Context(), store.CrisisReunificationsCollection)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "failed to list crisis reunifications", http.StatusInternalServerError)
		return
	}

	items := make([]domain.CrisisReunificationItem, 0)
	for _, raw := range rawItems {
		var item domain.CrisisReunificationItem
		if err := json.Unmarshal(raw, &item); err == nil {
			if statusFilter == "" || strings.EqualFold(item.Status, statusFilter) {
				items = append(items, item)
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			if items[i].Priority == domain.CrisisPriorityMicrochipMatch {
				return true
			}
			if items[j].Priority == domain.CrisisPriorityMicrochipMatch {
				return false
			}
		}
		return items[i].IdentifiedAt.After(items[j].IdentifiedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

// handleApiEvacuationReunificationContact handles POST requests to notify an owner and mark candidate contacted.
func (s *Server) handleApiEvacuationReunificationContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	matchID := strings.TrimSpace(r.PathValue("matchId"))
	if matchID == "" {
		http.NotFound(w, r)
		return
	}

	itemBytes, err := s.stateStore.GetState(r.Context(), store.CrisisReunificationsCollection, matchID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get crisis reunification item", http.StatusInternalServerError)
		return
	}

	var item domain.CrisisReunificationItem
	if err := json.Unmarshal(itemBytes, &item); err != nil {
		http.Error(w, "failed to parse crisis reunification item", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	item.Status = "CONTACTED"
	item.ContactedAt = &now

	updatedBytes, err := json.Marshal(item)
	if err != nil {
		http.Error(w, "failed to marshal crisis reunification item", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CrisisReunificationsCollection, item.MatchID, updatedBytes); err != nil {
		http.Error(w, "failed to save crisis reunification item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(clean, "-")
}

func randomHex(byteLen int) string {
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
