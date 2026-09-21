package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sheltersync"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func newTestEvacServer(t *testing.T) (*webfrontend.Server, store.StateStore) {
	t.Helper()
	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})
	return srv, st
}

func TestEvacuationHubs_ListingAndSeeding(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	// 1. Initial GET when collection is empty should seed default hubs and return 200 OK
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/hubs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var hubs []domain.EvacuationHub
	if err := json.NewDecoder(w.Body).Decode(&hubs); err != nil {
		t.Fatalf("failed to decode hubs response: %v", err)
	}

	if len(hubs) < 3 {
		t.Fatalf("expected at least 3 default seeded hubs, got %d", len(hubs))
	}

	// Verify one of the default hubs is Seattle Center
	foundSeattleCenter := false
	for _, h := range hubs {
		if strings.Contains(h.Name, "Seattle Center") {
			foundSeattleCenter = true
			if h.HubID == "" {
				t.Error("expected non-empty HubID for Seattle Center")
			}
			if h.TotalCapacity <= 0 {
				t.Errorf("expected positive TotalCapacity, got %d", h.TotalCapacity)
			}
			if h.Status != domain.HubStatusActive {
				t.Errorf("expected Status ACTIVE, got %s", h.Status)
			}
		}
	}
	if !foundSeattleCenter {
		t.Error("expected Seattle Center Exhibition Hall among default seeded hubs")
	}

	// Verify hubs persisted in store.EvacuationHubsCollection
	storedHubs, err := st.ListState(context.Background(), store.EvacuationHubsCollection)
	if err != nil {
		t.Fatalf("failed to list hubs in store: %v", err)
	}
	if len(storedHubs) != len(hubs) {
		t.Errorf("expected %d hubs in store, got %d", len(hubs), len(storedHubs))
	}

	// 2. Second GET should return same hubs from store without duplicate seeding
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/hubs", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on second request, got %d", w2.Code)
	}
	var hubs2 []domain.EvacuationHub
	if err := json.NewDecoder(w2.Body).Decode(&hubs2); err != nil {
		t.Fatalf("failed to decode hubs2: %v", err)
	}
	if len(hubs2) != len(hubs) {
		t.Errorf("expected %d hubs on second call, got %d", len(hubs), len(hubs2))
	}
}

func TestEvacuationHubs_Create(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	// Ensure defaults seeded first
	seedReq := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/hubs", nil)
	seedW := httptest.NewRecorder()
	srv.ServeHTTP(seedW, seedReq)

	newHub := domain.EvacuationHub{
		HubID:         "hub-bellevue-high",
		Name:          "Bellevue High Evacuation Shelter",
		Type:          domain.HubTypePopUpCrisisCenter,
		Address:       "10416 SE Wolverine Way, Bellevue, WA 98004",
		Coordinates:   domain.LocationPoint{Latitude: 47.6041, Longitude: -122.1932},
		TotalCapacity: 80,
		DogCapacity:   50,
		CatCapacity:   30,
		ContactName:   "Bellevue Logistics",
		ContactPhone:  "425-555-0155",
		ContactEmail:  "evac-bellevue@kingcounty.gov",
	}

	body, err := json.Marshal(newHub)
	if err != nil {
		t.Fatalf("failed to marshal hub: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/hubs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 201 Created or 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var created domain.EvacuationHub
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created hub: %v", err)
	}
	if created.HubID != "hub-bellevue-high" {
		t.Errorf("expected hubId hub-bellevue-high, got %s", created.HubID)
	}
	if created.Status != domain.HubStatusActive {
		t.Errorf("expected default ACTIVE status, got %s", created.Status)
	}

	// Verify persistence in store
	hubData, err := st.GetState(context.Background(), store.EvacuationHubsCollection, "hub-bellevue-high")
	if err != nil {
		t.Fatalf("failed to fetch created hub from store: %v", err)
	}
	var stored domain.EvacuationHub
	if err := json.Unmarshal(hubData, &stored); err != nil {
		t.Fatalf("failed to unmarshal stored hub: %v", err)
	}
	if stored.Name != newHub.Name {
		t.Errorf("expected stored name %q, got %q", newHub.Name, stored.Name)
	}
}

func TestEvacuationHubs_Create_ValidationErrors(t *testing.T) {
	t.Parallel()
	srv, _ := newTestEvacServer(t)

	// Missing Name
	invalidHub := map[string]interface{}{
		"totalCapacity": 50,
	}
	body, _ := json.Marshal(invalidHub)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/hubs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing name, got %d", w.Code)
	}

	// Zero / negative capacity
	invalidCapacity := map[string]interface{}{
		"name":          "Invalid Capacity Hub",
		"totalCapacity": 0,
	}
	body2, _ := json.Marshal(invalidCapacity)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/hubs", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for zero capacity, got %d", w2.Code)
	}
}

func TestEvacuationIntakeBatch_JSON(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	// Seed hubs
	seedReq := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/hubs", nil)
	seedW := httptest.NewRecorder()
	srv.ServeHTTP(seedW, seedReq)

	jsonPayload := `{
		"hubId": "hub-seattle-center",
		"records": [
			{
				"species": "dog",
				"breed": "German Shepherd",
				"primaryColor": "Black/Tan",
				"gender": "male",
				"microchipId": "985141000555666",
				"address": "Queen Anne, Seattle, WA",
				"notes": "Healthy adult, triage clear"
			},
			{
				"species": "cat",
				"breed": "Siamese",
				"primaryColor": "Seal Point",
				"gender": "female",
				"address": "Belltown, Seattle, WA",
				"notes": "Mild dehydration"
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/intake-batch", strings.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("expected 200 OK or 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var summary sheltersync.BulkIntakeBatchSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode batch summary: %v", err)
	}

	if summary.TotalProcessed != 2 {
		t.Errorf("expected 2 total processed, got %d", summary.TotalProcessed)
	}
	if summary.IngestedCount != 2 {
		t.Errorf("expected 2 ingested, got %d", summary.IngestedCount)
	}
	if summary.MicrochipCount != 1 {
		t.Errorf("expected 1 microchip count, got %d", summary.MicrochipCount)
	}
	if summary.HubID != "hub-seattle-center" {
		t.Errorf("expected HubID 'hub-seattle-center', got %q", summary.HubID)
	}

	// Verify records persisted in store.FoundPetsCollection
	foundPets, err := st.ListState(context.Background(), store.FoundPetsCollection)
	if err != nil {
		t.Fatalf("failed to list found pets: %v", err)
	}
	if len(foundPets) != 2 {
		t.Errorf("expected 2 found pets in store, got %d", len(foundPets))
	}

	// Verify hub occupancy updated
	hubData, err := st.GetState(context.Background(), store.EvacuationHubsCollection, "hub-seattle-center")
	if err != nil {
		t.Fatalf("failed to get hub state: %v", err)
	}
	var hub domain.EvacuationHub
	_ = json.Unmarshal(hubData, &hub)
	if hub.CurrentOccupancy != 2 {
		t.Errorf("expected hub currentOccupancy=2, got %d", hub.CurrentOccupancy)
	}
	if hub.DogOccupancy != 1 {
		t.Errorf("expected hub dogOccupancy=1, got %d", hub.DogOccupancy)
	}
	if hub.CatOccupancy != 1 {
		t.Errorf("expected hub catOccupancy=1, got %d", hub.CatOccupancy)
	}

	// Verify batch saved in CrisisIntakesCollection
	batchData, err := st.GetState(context.Background(), store.CrisisIntakesCollection, summary.BatchID)
	if err != nil {
		t.Fatalf("expected batch saved in CrisisIntakesCollection: %v", err)
	}
	var storedSummary sheltersync.BulkIntakeBatchSummary
	_ = json.Unmarshal(batchData, &storedSummary)
	if storedSummary.BatchID != summary.BatchID {
		t.Errorf("expected batch ID %s, got %s", summary.BatchID, storedSummary.BatchID)
	}
}

func TestEvacuationIntakeBatch_MultipartCSV_WithMicrochipMatch(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	// Seed hubs
	seedReq := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/hubs", nil)
	seedW := httptest.NewRecorder()
	srv.ServeHTTP(seedW, seedReq)

	// Seed an active lost pet report with matching microchip
	matchingChip := "985141000123456"
	lostPet := domain.LostPetRecord{
		PetID:            "lost-dog-cooper",
		PetName:          "Cooper",
		Species:          "dog",
		Breed:            "Golden Retriever",
		PrimaryColor:     "Golden",
		Status:           domain.LostPetStatusLost,
		MicrochipID:      matchingChip,
		OwnerIdentityRef: "lost/lost-dog-cooper/owner",
		OwnedBy:          &domain.PrincipalRef{Issuer: "petspotr", Subject: "Sarah Connor"},
	}
	lostPetBytes, err := json.Marshal(lostPet)
	if err != nil {
		t.Fatalf("failed to marshal lost pet: %v", err)
	}
	if err := st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostPetBytes); err != nil {
		t.Fatalf("failed to save lost pet: %v", err)
	}

	contact := domain.ReportContact{
		IdentityRef: "lost/lost-dog-cooper/owner",
		Phone:       "206-555-0199",
		Email:       "sarah@example.com",
	}
	contactBytes, err := json.Marshal(contact)
	if err != nil {
		t.Fatalf("failed to marshal report contact: %v", err)
	}
	if err := st.SaveState(context.Background(), store.ReportContactsCollection, contact.IdentityRef, contactBytes); err != nil {
		t.Fatalf("failed to save report contact: %v", err)
	}

	// Prepare CSV upload with 3 animals:
	// 1. Cooper (matching microchip)
	// 2. Cat with avid chip
	// 3. Dog without chip
	csvContent := `species,breed,primary_color,gender,microchip_id,address,notes
dog,Golden Retriever,Golden,male,985141000123456,"Interbay, Seattle, WA","Found near athletic complex"
cat,Domestic Shorthair,Tabby,female,456789123,"Mercer St, Seattle, WA","Found near stadium"
dog,Terrier Mix,Brown,male,,"Pike St, Seattle, WA","Found sheltering under awning"
`

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("hubId", "hub-seattle-center")

	part, err := writer.CreateFormFile("file", "roster.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte(csvContent))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/intake-batch", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("expected 200 OK or 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var summary sheltersync.BulkIntakeBatchSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode summary: %v", err)
	}

	if summary.TotalProcessed != 3 {
		t.Errorf("expected 3 processed, got %d", summary.TotalProcessed)
	}
	if summary.IngestedCount != 3 {
		t.Errorf("expected 3 ingested, got %d", summary.IngestedCount)
	}
	if summary.MicrochipCount != 2 {
		t.Errorf("expected 2 microchip count, got %d", summary.MicrochipCount)
	}
	if summary.InstantMatches != 1 {
		t.Errorf("expected 1 instant match, got %d", summary.InstantMatches)
	}

	// Result 0 should reference Cooper
	if summary.Results[0].MatchedLostPetID != "lost-dog-cooper" {
		t.Errorf("expected MatchedLostPetID 'lost-dog-cooper', got %q", summary.Results[0].MatchedLostPetID)
	}

	// Verify crisis reunification candidate staged in store.CrisisReunificationsCollection
	reunifications, err := st.ListState(context.Background(), store.CrisisReunificationsCollection)
	if err != nil {
		t.Fatalf("failed to list crisis reunifications: %v", err)
	}
	if len(reunifications) != 1 {
		t.Fatalf("expected 1 crisis reunification in store, got %d", len(reunifications))
	}

	var matchItem domain.CrisisReunificationItem
	for _, raw := range reunifications {
		_ = json.Unmarshal(raw, &matchItem)
	}
	if matchItem.LostPetID != "lost-dog-cooper" {
		t.Errorf("expected LostPetID lost-dog-cooper, got %s", matchItem.LostPetID)
	}
	if matchItem.PetName != "Cooper" {
		t.Errorf("expected PetName Cooper, got %s", matchItem.PetName)
	}
	if matchItem.Priority != domain.CrisisPriorityMicrochipMatch {
		t.Errorf("expected Priority MICROCHIP_EXACT, got %s", matchItem.Priority)
	}
	if matchItem.Status != "PENDING" {
		t.Errorf("expected Status PENDING, got %s", matchItem.Status)
	}
	if matchItem.SimilarityScore != 1.0 {
		t.Errorf("expected SimilarityScore 1.0, got %f", matchItem.SimilarityScore)
	}
	if matchItem.OwnerName != "Sarah Connor" {
		t.Errorf("expected OwnerName Sarah Connor, got %s", matchItem.OwnerName)
	}
}

func TestEvacuationTransfers_LifecycleAndOccupancy(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	// Seed hubs
	seedReq := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/hubs", nil)
	seedW := httptest.NewRecorder()
	srv.ServeHTTP(seedW, seedReq)

	// Ingest 2 animals at hub-seattle-center
	intakeJSON := `{
		"hubId": "hub-seattle-center",
		"records": [
			{"species": "dog", "breed": "Husky", "intakeId": "DOG-1", "address": "Seattle"},
			{"species": "cat", "breed": "Tabby", "intakeId": "CAT-1", "address": "Seattle"}
		]
	}`
	intakeReq := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/intake-batch", strings.NewReader(intakeJSON))
	intakeReq.Header.Set("Content-Type", "application/json")
	intakeW := httptest.NewRecorder()
	srv.ServeHTTP(intakeW, intakeReq)
	if intakeW.Code != http.StatusOK && intakeW.Code != http.StatusCreated {
		t.Fatalf("failed to ingest test animals: %d: %s", intakeW.Code, intakeW.Body.String())
	}

	var batchSummary sheltersync.BulkIntakeBatchSummary
	_ = json.NewDecoder(intakeW.Body).Decode(&batchSummary)
	animalIDs := []string{batchSummary.Results[0].PetID, batchSummary.Results[1].PetID}

	// Verify origin hub occupancy is 2
	origHubData, _ := st.GetState(context.Background(), store.EvacuationHubsCollection, "hub-seattle-center")
	var origHub domain.EvacuationHub
	_ = json.Unmarshal(origHubData, &origHub)
	if origHub.CurrentOccupancy != 2 || origHub.DogOccupancy != 1 || origHub.CatOccupancy != 1 {
		t.Fatalf("unexpected origin hub occupancy: total=%d, dog=%d, cat=%d", origHub.CurrentOccupancy, origHub.DogOccupancy, origHub.CatOccupancy)
	}

	// 1. Stage Transfer: Seattle Center -> Magnuson Park
	transferReqBody := domain.TransferManifest{
		OriginHubID:      "hub-seattle-center",
		DestHubID:        "hub-magnuson-park",
		AnimalIDs:        animalIDs,
		TransporterName:  "Convoy Team Alpha",
		TransporterPhone: "206-555-9000",
		VehicleNotes:     "Sprinter Van #3",
	}
	xferData, _ := json.Marshal(transferReqBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/transfers", bytes.NewReader(xferData))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	srv.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated && createW.Code != http.StatusOK {
		t.Fatalf("expected 201 Created for transfer, got %d: %s", createW.Code, createW.Body.String())
	}

	var stagedManifest domain.TransferManifest
	if err := json.NewDecoder(createW.Body).Decode(&stagedManifest); err != nil {
		t.Fatalf("failed to decode staged manifest: %v", err)
	}
	if stagedManifest.TransferID == "" {
		t.Error("expected non-empty TransferID")
	}
	if stagedManifest.Status != domain.TransferStatusStaged {
		t.Errorf("expected Status STAGED, got %s", stagedManifest.Status)
	}
	if stagedManifest.TotalAnimals != 2 {
		t.Errorf("expected TotalAnimals=2, got %d", stagedManifest.TotalAnimals)
	}

	// 2. Query Transfers list with filter
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/transfers?status=STAGED", nil)
	listW := httptest.NewRecorder()
	srv.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing transfers, got %d", listW.Code)
	}
	var manifests []domain.TransferManifest
	_ = json.NewDecoder(listW.Body).Decode(&manifests)
	if len(manifests) != 1 {
		t.Errorf("expected 1 staged manifest, got %d", len(manifests))
	}

	// 3. Update status: STAGED -> IN_TRANSIT
	inTransitUpdate := map[string]string{"status": string(domain.TransferStatusInTransit)}
	statusData, _ := json.Marshal(inTransitUpdate)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(statusData))
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	srv.ServeHTTP(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating to IN_TRANSIT, got %d: %s", putW.Code, putW.Body.String())
	}

	var inTransitManifest domain.TransferManifest
	_ = json.NewDecoder(putW.Body).Decode(&inTransitManifest)
	if inTransitManifest.Status != domain.TransferStatusInTransit {
		t.Errorf("expected Status IN_TRANSIT, got %s", inTransitManifest.Status)
	}
	if inTransitManifest.DepartureTime == nil {
		t.Error("expected non-nil DepartureTime on IN_TRANSIT")
	}

	// Verify origin hub occupancy decremented!
	origHubDataAfter, _ := st.GetState(context.Background(), store.EvacuationHubsCollection, "hub-seattle-center")
	var origHubAfter domain.EvacuationHub
	_ = json.Unmarshal(origHubDataAfter, &origHubAfter)
	if origHubAfter.CurrentOccupancy != 0 {
		t.Errorf("expected origin hub occupancy to decrement to 0, got %d", origHubAfter.CurrentOccupancy)
	}
	if origHubAfter.DogOccupancy != 0 || origHubAfter.CatOccupancy != 0 {
		t.Errorf("expected origin dog/cat occupancy to decrement to 0, got dog=%d, cat=%d", origHubAfter.DogOccupancy, origHubAfter.CatOccupancy)
	}

	// 4. Update status: IN_TRANSIT -> RECEIVED
	receivedUpdate := map[string]string{"status": string(domain.TransferStatusReceived)}
	statusData2, _ := json.Marshal(receivedUpdate)
	putReq2 := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(statusData2))
	putReq2.Header.Set("Content-Type", "application/json")
	putW2 := httptest.NewRecorder()
	srv.ServeHTTP(putW2, putReq2)

	if putW2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating to RECEIVED, got %d: %s", putW2.Code, putW2.Body.String())
	}

	var receivedManifest domain.TransferManifest
	_ = json.NewDecoder(putW2.Body).Decode(&receivedManifest)
	if receivedManifest.Status != domain.TransferStatusReceived {
		t.Errorf("expected Status RECEIVED, got %s", receivedManifest.Status)
	}
	if receivedManifest.ArrivalTime == nil {
		t.Error("expected non-nil ArrivalTime on RECEIVED")
	}

	// Verify destination hub occupancy incremented!
	destHubData, _ := st.GetState(context.Background(), store.EvacuationHubsCollection, "hub-magnuson-park")
	var destHub domain.EvacuationHub
	_ = json.Unmarshal(destHubData, &destHub)
	if destHub.CurrentOccupancy != 2 {
		t.Errorf("expected dest hub occupancy=2, got %d", destHub.CurrentOccupancy)
	}
	if destHub.DogOccupancy != 1 || destHub.CatOccupancy != 1 {
		t.Errorf("expected dest hub dog=1, cat=1, got dog=%d, cat=%d", destHub.DogOccupancy, destHub.CatOccupancy)
	}

	// Verify FoundPetRecords updated location and shelter to destination hub
	for _, petID := range animalIDs {
		petBytes, err := st.GetState(context.Background(), store.FoundPetsCollection, petID)
		if err != nil {
			t.Fatalf("failed to fetch transferred pet %s: %v", petID, err)
		}
		var pet domain.FoundPetRecord
		_ = json.Unmarshal(petBytes, &pet)
		if pet.ShelterID != "hub-magnuson-park" {
			t.Errorf("expected pet.ShelterID 'hub-magnuson-park', got %q", pet.ShelterID)
		}
		if pet.ShelterName != destHub.Name {
			t.Errorf("expected pet.ShelterName %q, got %q", destHub.Name, pet.ShelterName)
		}
	}

	// 5. Update status: RECEIVED -> RECONCILED
	reconciledUpdate := map[string]string{"status": string(domain.TransferStatusReconciled)}
	statusData3, _ := json.Marshal(reconciledUpdate)
	putReq3 := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(statusData3))
	putReq3.Header.Set("Content-Type", "application/json")
	putW3 := httptest.NewRecorder()
	srv.ServeHTTP(putW3, putReq3)

	if putW3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating to RECONCILED, got %d: %s", putW3.Code, putW3.Body.String())
	}
	var reconManifest domain.TransferManifest
	_ = json.NewDecoder(putW3.Body).Decode(&reconManifest)
	if reconManifest.Status != domain.TransferStatusReconciled {
		t.Errorf("expected Status RECONCILED, got %s", reconManifest.Status)
	}
}

func TestEvacuationCrisisReunificationQueue_ContactDispatch(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	// Seed crisis reunification item directly
	now := time.Now().UTC()
	item := domain.CrisisReunificationItem{
		MatchID:         "crisis-match-101",
		FoundPetID:      "found-101",
		LostPetID:       "lost-202",
		PetName:         "Luna",
		Species:         "Cat",
		Breed:           "Russian Blue",
		CurrentHubID:    "hub-seattle-center",
		CurrentHubName:  "Seattle Center Exhibition Hall",
		OwnerName:       "Alex Mercer",
		OwnerContact:    "206-555-8888",
		MicrochipID:     "985141000777888",
		Priority:        domain.CrisisPriorityMicrochipMatch,
		SimilarityScore: 1.0,
		Status:          "PENDING",
		IdentifiedAt:    now,
	}
	data, _ := json.Marshal(item)
	_ = st.SaveState(context.Background(), store.CrisisReunificationsCollection, item.MatchID, data)

	// 1. GET queue
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/evacuations/reunification-queue", nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getW.Code, getW.Body.String())
	}
	var queue []domain.CrisisReunificationItem
	if err := json.NewDecoder(getW.Body).Decode(&queue); err != nil {
		t.Fatalf("failed to decode queue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("expected 1 item in queue, got %d", len(queue))
	}
	if queue[0].MatchID != "crisis-match-101" {
		t.Errorf("expected MatchID crisis-match-101, got %s", queue[0].MatchID)
	}

	// 2. POST contact dispatch
	contactReq := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/reunification-queue/crisis-match-101/contact", nil)
	contactW := httptest.NewRecorder()
	srv.ServeHTTP(contactW, contactReq)

	if contactW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK contacting owner, got %d: %s", contactW.Code, contactW.Body.String())
	}
	var updatedItem domain.CrisisReunificationItem
	if err := json.NewDecoder(contactW.Body).Decode(&updatedItem); err != nil {
		t.Fatalf("failed to decode updated item: %v", err)
	}
	if updatedItem.Status != "CONTACTED" {
		t.Errorf("expected Status CONTACTED, got %s", updatedItem.Status)
	}
	if updatedItem.ContactedAt == nil {
		t.Error("expected non-nil ContactedAt")
	}

	// Verify updated in store
	storedBytes, err := st.GetState(context.Background(), store.CrisisReunificationsCollection, "crisis-match-101")
	if err != nil {
		t.Fatalf("failed to get updated item from store: %v", err)
	}
	var storedItem domain.CrisisReunificationItem
	_ = json.Unmarshal(storedBytes, &storedItem)
	if storedItem.Status != "CONTACTED" {
		t.Errorf("expected stored status CONTACTED, got %s", storedItem.Status)
	}

	// 3. Contact non-existent match should return 404
	missingReq := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/reunification-queue/non-existent-match/contact", nil)
	missingW := httptest.NewRecorder()
	srv.ServeHTTP(missingW, missingReq)
	if missingW.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent match, got %d", missingW.Code)
	}
}
