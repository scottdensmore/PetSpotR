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

	// Negative dog capacity
	invalidDogCap := map[string]interface{}{
		"name":          "Negative Dog Capacity Hub",
		"totalCapacity": 50,
		"dogCapacity":   -1,
	}
	body3, _ := json.Marshal(invalidDogCap)
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/hubs", bytes.NewReader(body3))
	w3 := httptest.NewRecorder()
	srv.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for negative dog capacity, got %d", w3.Code)
	}

	// Negative cat capacity
	invalidCatCap := map[string]interface{}{
		"name":          "Negative Cat Capacity Hub",
		"totalCapacity": 50,
		"catCapacity":   -5,
	}
	body4, _ := json.Marshal(invalidCatCap)
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/evacuations/hubs", bytes.NewReader(body4))
	w4 := httptest.NewRecorder()
	srv.ServeHTTP(w4, req4)
	if w4.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for negative cat capacity, got %d", w4.Code)
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

	// 2b. Attempt invalid transition: STAGED -> RECEIVED (should return 409 Conflict)
	invalidSkipUpdate := map[string]string{"status": string(domain.TransferStatusReceived)}
	invalidSkipData, _ := json.Marshal(invalidSkipUpdate)
	invalidReq := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(invalidSkipData))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidW := httptest.NewRecorder()
	srv.ServeHTTP(invalidW, invalidReq)
	if invalidW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict attempting STAGED -> RECEIVED, got %d: %s", invalidW.Code, invalidW.Body.String())
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

	// 3b. Test idempotency: calling IN_TRANSIT again returns 200 OK
	putReqRetry := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(statusData))
	putReqRetry.Header.Set("Content-Type", "application/json")
	putWRetry := httptest.NewRecorder()
	srv.ServeHTTP(putWRetry, putReqRetry)
	if putWRetry.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for idempotent IN_TRANSIT update, got %d: %s", putWRetry.Code, putWRetry.Body.String())
	}

	// 3c. Attempt invalid regression: IN_TRANSIT -> STAGED (should return 409 Conflict)
	invalidRegress := map[string]string{"status": string(domain.TransferStatusStaged)}
	invalidRegressData, _ := json.Marshal(invalidRegress)
	regressReq := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(invalidRegressData))
	regressReq.Header.Set("Content-Type", "application/json")
	regressW := httptest.NewRecorder()
	srv.ServeHTTP(regressW, regressReq)
	if regressW.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict attempting IN_TRANSIT -> STAGED, got %d: %s", regressW.Code, regressW.Body.String())
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

	// 4b. Test idempotency: calling RECEIVED again returns 200 OK and does not double-increment
	putReq2Retry := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(statusData2))
	putReq2Retry.Header.Set("Content-Type", "application/json")
	putW2Retry := httptest.NewRecorder()
	srv.ServeHTTP(putW2Retry, putReq2Retry)
	if putW2Retry.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for idempotent RECEIVED update, got %d: %s", putW2Retry.Code, putW2Retry.Body.String())
	}

	// 4c. Attempt invalid regression: RECEIVED -> IN_TRANSIT (should return 409 Conflict)
	invalidRegress2 := map[string]string{"status": string(domain.TransferStatusInTransit)}
	invalidRegressData2, _ := json.Marshal(invalidRegress2)
	regressReq2 := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+stagedManifest.TransferID+"/status", bytes.NewReader(invalidRegressData2))
	regressReq2.Header.Set("Content-Type", "application/json")
	regressW2 := httptest.NewRecorder()
	srv.ServeHTTP(regressW2, regressReq2)
	if regressW2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict attempting RECEIVED -> IN_TRANSIT, got %d: %s", regressW2.Code, regressW2.Body.String())
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

func TestEvacuationTransfers_IdempotencyAndStateTransitions(t *testing.T) {
	t.Parallel()
	srv, st := newTestEvacServer(t)

	now := time.Now().UTC()
	origHub := domain.EvacuationHub{
		HubID:            "hub-origin-alpha",
		Name:             "Origin Alpha",
		Type:             domain.HubTypePopUpCrisisCenter,
		Status:           domain.HubStatusActive,
		TotalCapacity:    100,
		CurrentOccupancy: 10,
		DogCapacity:      60,
		DogOccupancy:     6,
		CatCapacity:      40,
		CatOccupancy:     4,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	destHub := domain.EvacuationHub{
		HubID:            "hub-dest-beta",
		Name:             "Destination Beta",
		Type:             domain.HubTypePermanentShelter,
		Status:           domain.HubStatusActive,
		TotalCapacity:    100,
		CurrentOccupancy: 0,
		DogCapacity:      60,
		DogOccupancy:     0,
		CatCapacity:      40,
		CatOccupancy:     0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	origBytes, _ := json.Marshal(origHub)
	destBytes, _ := json.Marshal(destHub)
	_ = st.SaveState(context.Background(), store.EvacuationHubsCollection, origHub.HubID, origBytes)
	_ = st.SaveState(context.Background(), store.EvacuationHubsCollection, destHub.HubID, destBytes)

	// Seed 2 found pets
	pet1 := domain.FoundPetRecord{
		PetID:     "pet-dog-100",
		Species:   "Dog",
		ShelterID: origHub.HubID,
	}
	pet2 := domain.FoundPetRecord{
		PetID:     "pet-cat-200",
		Species:   "Cat",
		ShelterID: origHub.HubID,
	}
	p1Bytes, _ := json.Marshal(pet1)
	p2Bytes, _ := json.Marshal(pet2)
	_ = st.SaveState(context.Background(), store.FoundPetsCollection, pet1.PetID, p1Bytes)
	_ = st.SaveState(context.Background(), store.FoundPetsCollection, pet2.PetID, p2Bytes)

	// Create Transfer (starts in STAGED)
	manifest := domain.TransferManifest{
		TransferID:   "xfer-state-test",
		OriginHubID:  origHub.HubID,
		DestHubID:    destHub.HubID,
		AnimalIDs:    []string{"pet-dog-100", "pet-cat-200"},
		TotalAnimals: 2,
		Status:       domain.TransferStatusStaged,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	mBytes, _ := json.Marshal(manifest)
	_ = st.SaveState(context.Background(), store.TransferManifestsCollection, manifest.TransferID, mBytes)

	sendUpdate := func(status string) (int, map[string]interface{}) {
		body, _ := json.Marshal(map[string]string{"status": status})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/evacuations/transfers/"+manifest.TransferID+"/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		return w.Code, resp
	}

	getHub := func(id string) domain.EvacuationHub {
		data, err := st.GetState(context.Background(), store.EvacuationHubsCollection, id)
		if err != nil {
			t.Fatalf("failed to get hub %s: %v", id, err)
		}
		var h domain.EvacuationHub
		_ = json.Unmarshal(data, &h)
		return h
	}

	// 1. Invalid transition from STAGED -> RECEIVED should return 409 Conflict
	code, resp := sendUpdate("RECEIVED")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for STAGED -> RECEIVED, got %d", code)
	}
	if resp["error"] == nil {
		t.Error("expected error message in JSON response")
	}

	// 2. Invalid transition from STAGED -> RECONCILED should return 409 Conflict
	code, _ = sendUpdate("RECONCILED")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for STAGED -> RECONCILED, got %d", code)
	}

	// 3. Bogus status should return 400 Bad Request
	code, _ = sendUpdate("NOT_A_VALID_STATUS")
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for bogus status, got %d", code)
	}

	// Verify occupancies remained unchanged
	if h := getHub(origHub.HubID); h.CurrentOccupancy != 10 {
		t.Errorf("expected origin occupancy 10, got %d", h.CurrentOccupancy)
	}
	if h := getHub(destHub.HubID); h.CurrentOccupancy != 0 {
		t.Errorf("expected dest occupancy 0, got %d", h.CurrentOccupancy)
	}

	// 4. Valid transition: STAGED -> IN_TRANSIT
	code, _ = sendUpdate("IN_TRANSIT")
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK for STAGED -> IN_TRANSIT, got %d", code)
	}
	if h := getHub(origHub.HubID); h.CurrentOccupancy != 8 || h.DogOccupancy != 5 || h.CatOccupancy != 3 {
		t.Errorf("expected origin occupancy (8, d:5, c:3), got (%d, d:%d, c:%d)", h.CurrentOccupancy, h.DogOccupancy, h.CatOccupancy)
	}

	// 5. Idempotent status update: IN_TRANSIT again should return 200 OK without re-decrementing
	code, _ = sendUpdate("IN_TRANSIT")
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK for idempotent IN_TRANSIT, got %d", code)
	}
	if h := getHub(origHub.HubID); h.CurrentOccupancy != 8 || h.DogOccupancy != 5 || h.CatOccupancy != 3 {
		t.Errorf("idempotency failed: origin occupancy changed on retry to (%d, d:%d, c:%d)", h.CurrentOccupancy, h.DogOccupancy, h.CatOccupancy)
	}

	// 6. Invalid regression: IN_TRANSIT -> STAGED should return 409 Conflict
	code, _ = sendUpdate("STAGED")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for IN_TRANSIT -> STAGED, got %d", code)
	}

	// 7. Invalid transition: IN_TRANSIT -> RECONCILED should return 409 Conflict
	code, _ = sendUpdate("RECONCILED")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for IN_TRANSIT -> RECONCILED, got %d", code)
	}

	// 8. Valid transition: IN_TRANSIT -> RECEIVED
	code, _ = sendUpdate("RECEIVED")
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK for IN_TRANSIT -> RECEIVED, got %d", code)
	}
	if h := getHub(destHub.HubID); h.CurrentOccupancy != 2 || h.DogOccupancy != 1 || h.CatOccupancy != 1 {
		t.Errorf("expected dest occupancy (2, d:1, c:1), got (%d, d:%d, c:%d)", h.CurrentOccupancy, h.DogOccupancy, h.CatOccupancy)
	}

	// 9. Idempotent status update: RECEIVED again should return 200 OK without double-incrementing
	code, _ = sendUpdate("RECEIVED")
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK for idempotent RECEIVED, got %d", code)
	}
	if h := getHub(destHub.HubID); h.CurrentOccupancy != 2 || h.DogOccupancy != 1 || h.CatOccupancy != 1 {
		t.Errorf("idempotency failed: dest occupancy changed on retry to (%d, d:%d, c:%d)", h.CurrentOccupancy, h.DogOccupancy, h.CatOccupancy)
	}

	// 10. Invalid regression: RECEIVED -> IN_TRANSIT should return 409 Conflict
	code, _ = sendUpdate("IN_TRANSIT")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for RECEIVED -> IN_TRANSIT, got %d", code)
	}

	// 11. Invalid regression: RECEIVED -> STAGED should return 409 Conflict
	code, _ = sendUpdate("STAGED")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for RECEIVED -> STAGED, got %d", code)
	}

	// 12. Valid transition: RECEIVED -> RECONCILED
	code, _ = sendUpdate("RECONCILED")
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK for RECEIVED -> RECONCILED, got %d", code)
	}

	// 13. Idempotent status update: RECONCILED again should return 200 OK
	code, _ = sendUpdate("RECONCILED")
	if code != http.StatusOK {
		t.Fatalf("expected 200 OK for idempotent RECONCILED, got %d", code)
	}

	// 14. Invalid regression: RECONCILED -> RECEIVED should return 409 Conflict
	code, _ = sendUpdate("RECEIVED")
	if code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for RECONCILED -> RECEIVED, got %d", code)
	}
}

func TestEvacuationDashboard_RendersOK(t *testing.T) {
	t.Parallel()

	srv, _ := newTestEvacServer(t)

	// 1. Valid GET /evacuation
	req := httptest.NewRequest(http.MethodGet, "/evacuation", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /evacuation, got %d: %s", rec.Code, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", contentType)
	}

	body := rec.Body.String()

	// 2. Method Not Allowed check
	postReq := httptest.NewRequest(http.MethodPost, "/evacuation", nil)
	postRec := httptest.NewRecorder()
	srv.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed for POST /evacuation, got %d", postRec.Code)
	}

	// 3. Expected elements & IDs in dashboard
	expectedSnippets := []string{
		// Metrics HUD
		`id="metric-evacuated-total"`,
		`id="metric-active-hubs"`,
		`id="metric-open-capacity"`,
		`id="metric-reunifications-pending"`,
		// Tab navigation
		`role="tablist"`,
		`aria-label="Disaster Operations Navigation"`,
		`id="tab-hubs"`,
		`id="tab-intake"`,
		`id="tab-transfers"`,
		`id="tab-reunifications"`,
		// Panel 1: Facilities
		`id="tab-panel-hubs"`,
		`id="hubs-card-grid"`,
		`id="btn-new-hub"`,
		`id="modal-new-hub"`,
		// Panel 2: Bulk Intake
		`id="tab-panel-intake"`,
		`id="intake-dropzone"`,
		`id="intake-file-input"`,
		`accept=".csv,.json"`,
		`id="intake-file-details"`,
		`id="intake-hub-select"`,
		`id="btn-process-batch"`,
		`id="intake-feedback"`,
		`id="intake-batch-summary"`,
		`id="intake-results-table"`,
		// Panel 3: Transfers
		`id="tab-panel-transfers"`,
		`id="transfers-ledger-table"`,
		`id="transfers-status-filter"`,
		`id="btn-stage-transfer"`,
		`id="modal-stage-transfer"`,
		// Panel 4: Crisis Reunifications
		`id="tab-panel-reunifications"`,
		`id="reunifications-grid"`,
		`id="btn-contact-owner"`,
		`🔥 Exact Microchip Match`,
		// Client script
		`<script src="/static/js/evacuation.js"></script>`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(body, snippet) {
			t.Errorf("GET /evacuation response missing required snippet: %s", snippet)
		}
	}
}

func TestEvacuationNavigationAndStyles(t *testing.T) {
	t.Parallel()

	// 1. Verify navigation link in layout.html
	layoutData, err := webfrontend.EmbeddedFiles.ReadFile("templates/layout.html")
	if err != nil {
		t.Fatalf("failed to read templates/layout.html: %v", err)
	}
	layoutContent := string(layoutData)
	if !strings.Contains(layoutContent, `href="/evacuation"`) {
		t.Error("layout.html missing href=\"/evacuation\"")
	}
	if !strings.Contains(layoutContent, `nav-link-emergency`) {
		t.Error("layout.html missing nav-link-emergency class")
	}
	if !strings.Contains(layoutContent, `🚨 Disaster Hub`) {
		t.Error("layout.html missing '🚨 Disaster Hub' label")
	}

	// 2. Verify emergency styles in styles.css
	cssData, err := webfrontend.EmbeddedFiles.ReadFile("static/css/styles.css")
	if err != nil {
		t.Fatalf("failed to read static/css/styles.css: %v", err)
	}
	cssContent := string(cssData)

	expectedCSSSelectors := []string{
		".nav-link-emergency",
		".evacuation-dashboard",
		".metrics-hud",
		".metric-card",
		".metric-value",
		".metric-label",
		".evacuation-tabs",
		".tab-btn",
		".tab-btn.active",
		".hub-card",
		".occupancy-bar-track",
		".occupancy-bar-fill",
		".intake-dropzone",
		".intake-dropzone.dragover",
		".transfer-status-staged",
		".transfer-status-intransit",
		".transfer-status-received",
		".reunification-card",
		".badge-microchip-exact",
	}

	for _, selector := range expectedCSSSelectors {
		if !strings.Contains(cssContent, selector) {
			t.Errorf("static/css/styles.css missing required selector: %s", selector)
		}
	}
}
