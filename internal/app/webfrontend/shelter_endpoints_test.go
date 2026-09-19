package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/sheltersync"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestShelterIntakeIngest(t *testing.T) {
	t.Parallel()
	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	payload := map[string]interface{}{
		"shelterId":      "shelter-sea-01",
		"shelterName":    "Seattle Animal Shelter",
		"shelterAddress": "2061 15th Ave W, Seattle, WA 98119",
		"shelterPhone":   "(206) 386-7387",
		"intakeId":       "INT-2026-8819",
		"animal": map[string]interface{}{
			"species":     "dog",
			"breed":       "Golden Retriever",
			"description": "Found near Interbay",
			"microchipId": "985141000123456",
			"images": []map[string]string{
				{"url": "https://storage.petspotr.io/shelters/intake-8819.jpg", "view": "primary"},
			},
		},
		"location": map[string]interface{}{
			"address":   "Interbay, Seattle, WA",
			"latitude":  47.648,
			"longitude": -122.378,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shelter-intakes/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 201 Created or 200 OK, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["intakeId"] != "INT-2026-8819" {
		t.Errorf("expected intakeId INT-2026-8819, got %v", resp["intakeId"])
	}
	if resp["shelterId"] != "shelter-sea-01" {
		t.Errorf("expected shelterId shelter-sea-01, got %v", resp["shelterId"])
	}
	if resp["custodyStatus"] != "Shelter Care" {
		t.Errorf("expected custodyStatus 'Shelter Care', got %v", resp["custodyStatus"])
	}
	if resp["microchipId"] != "985141000123456" {
		t.Errorf("expected microchipId 985141000123456, got %v", resp["microchipId"])
	}
	if resp["microchipRegistry"] != "HomeAgain" {
		t.Errorf("expected microchipRegistry HomeAgain, got %v", resp["microchipRegistry"])
	}

	petID, ok := resp["petId"].(string)
	if !ok || petID == "" {
		t.Fatalf("expected valid non-empty petId string in response, got %v", resp["petId"])
	}

	// Verify persistence in store.FoundPetsCollection
	recordData, err := st.GetState(context.Background(), store.FoundPetsCollection, petID)
	if err != nil {
		t.Fatalf("expected record in FoundPetsCollection, got error: %v", err)
	}
	var record domain.FoundPetRecord
	if err := json.Unmarshal(recordData, &record); err != nil {
		t.Fatalf("failed to unmarshal persisted FoundPetRecord: %v", err)
	}
	if record.PetID != petID {
		t.Errorf("expected record.PetID == %s, got %s", petID, record.PetID)
	}
	if record.ShelterID != "shelter-sea-01" {
		t.Errorf("expected record.ShelterID == 'shelter-sea-01', got %s", record.ShelterID)
	}
	if record.ShelterName != "Seattle Animal Shelter" {
		t.Errorf("expected record.ShelterName == 'Seattle Animal Shelter', got %s", record.ShelterName)
	}
	if record.IntakeID != "INT-2026-8819" {
		t.Errorf("expected record.IntakeID == 'INT-2026-8819', got %s", record.IntakeID)
	}
	if record.CustodyStatus != domain.CustodyShelterCare {
		t.Errorf("expected record.CustodyStatus == %q, got %q", domain.CustodyShelterCare, record.CustodyStatus)
	}
	if record.MicrochipID != "985141000123456" {
		t.Errorf("expected record.MicrochipID == '985141000123456', got %s", record.MicrochipID)
	}
	if record.MicrochipRegistry != "HomeAgain" {
		t.Errorf("expected record.MicrochipRegistry == 'HomeAgain', got %s", record.MicrochipRegistry)
	}

	// Verify outbox publication has PayloadVersion: 2 and shelter/microchip fields
	outboxRecords, err := st.ListState(context.Background(), store.OutboxCollection)
	if err != nil {
		t.Fatalf("failed to list outbox collection: %v", err)
	}
	if len(outboxRecords) == 0 {
		t.Fatalf("expected at least one outbox record, found none")
	}

	var foundEnvelope bool
	for _, rawRecord := range outboxRecords {
		var rec outbox.Record
		if err := json.Unmarshal(rawRecord, &rec); err != nil {
			continue
		}
		if rec.Topic != "foundPet" {
			continue
		}
		event, envelope, err := domain.DecodeFoundPetReported(rec.Payload)
		if err != nil {
			t.Fatalf("failed to decode foundPet event: %v", err)
		}
		if envelope.PayloadVersion != 2 {
			t.Errorf("expected envelope.PayloadVersion == 2, got %d", envelope.PayloadVersion)
		}
		if event.PetID == petID && event.IntakeID == "INT-2026-8819" {
			foundEnvelope = true
			if event.ShelterID != "shelter-sea-01" {
				t.Errorf("expected event.ShelterID == 'shelter-sea-01', got %s", event.ShelterID)
			}
			if event.ShelterName != "Seattle Animal Shelter" {
				t.Errorf("expected event.ShelterName == 'Seattle Animal Shelter', got %s", event.ShelterName)
			}
			if event.CustodyStatus != domain.CustodyShelterCare {
				t.Errorf("expected event.CustodyStatus == %q, got %q", domain.CustodyShelterCare, event.CustodyStatus)
			}
			if event.MicrochipID != "985141000123456" {
				t.Errorf("expected event.MicrochipID == '985141000123456', got %s", event.MicrochipID)
			}
			if event.MicrochipRegistry != "HomeAgain" {
				t.Errorf("expected event.MicrochipRegistry == 'HomeAgain', got %s", event.MicrochipRegistry)
			}
			break
		}
	}
	if !foundEnvelope {
		t.Errorf("did not find matching foundPet event in outbox for petID %s", petID)
	}
}

func TestShelterIntakeIngest_InvalidPayloads(t *testing.T) {
	t.Parallel()
	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	testCases := []struct {
		name         string
		method       string
		payload      map[string]interface{}
		rawBody      string
		expectedCode int
	}{
		{
			name:   "method not allowed",
			method: http.MethodGet,
			payload: map[string]interface{}{
				"shelterId": "shelter-01",
				"intakeId":  "INT-01",
			},
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "invalid json",
			method:       http.MethodPost,
			rawBody:      "{bad json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "missing shelterId",
			method: http.MethodPost,
			payload: map[string]interface{}{
				"intakeId": "INT-01",
				"animal":   map[string]interface{}{"species": "dog"},
				"location": map[string]interface{}{"address": "Seattle"},
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "missing intakeId",
			method: http.MethodPost,
			payload: map[string]interface{}{
				"shelterId": "shelter-01",
				"animal":    map[string]interface{}{"species": "dog"},
				"location":  map[string]interface{}{"address": "Seattle"},
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "missing species",
			method: http.MethodPost,
			payload: map[string]interface{}{
				"shelterId": "shelter-01",
				"intakeId":  "INT-01",
				"animal":    map[string]interface{}{"species": ""},
				"location":  map[string]interface{}{"address": "Seattle"},
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "missing location address",
			method: http.MethodPost,
			payload: map[string]interface{}{
				"shelterId": "shelter-01",
				"intakeId":  "INT-01",
				"animal":    map[string]interface{}{"species": "dog"},
				"location":  map[string]interface{}{"address": ""},
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "invalid microchip",
			method: http.MethodPost,
			payload: map[string]interface{}{
				"shelterId": "shelter-01",
				"intakeId":  "INT-01",
				"animal": map[string]interface{}{
					"species":     "dog",
					"microchipId": "invalid-chip-123",
				},
				"location": map[string]interface{}{"address": "Seattle"},
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var req *http.Request
			if tc.rawBody != "" {
				req = httptest.NewRequest(tc.method, "/api/v1/shelter-intakes/ingest", bytes.NewReader([]byte(tc.rawBody)))
			} else {
				data, _ := json.Marshal(tc.payload)
				req = httptest.NewRequest(tc.method, "/api/v1/shelter-intakes/ingest", bytes.NewReader(data))
			}
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != tc.expectedCode {
				t.Fatalf("expected status %d, got %d (body: %s)", tc.expectedCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestShelterSyncWorkerWithHttpIngest(t *testing.T) {
	t.Parallel()
	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()

	adapter := sheltersync.NewMockShelterFeedAdapter()
	ingester := sheltersync.NewHTTPIngester(ts.URL+"/api/v1/shelter-intakes/ingest", ts.Client())
	worker := sheltersync.NewSyncWorker(adapter, ingester)

	count, err := worker.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 intakes ingested, got %d", count)
	}

	// Verify all 3 records persisted in FoundPetsCollection
	expectedIDs := []string{
		"shelter-shelter-sea-01-INT-2026-8819",
		"shelter-shelter-bel-02-INT-2026-9901",
		"shelter-shelter-kcras-03-INT-2026-7732",
	}

	for _, petID := range expectedIDs {
		data, err := st.GetState(context.Background(), store.FoundPetsCollection, petID)
		if err != nil {
			t.Errorf("expected pet %s in store, got error: %v", petID, err)
			continue
		}
		var record domain.FoundPetRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Errorf("failed to unmarshal record %s: %v", petID, err)
		}
		if record.CustodyStatus != domain.CustodyShelterCare {
			t.Errorf("pet %s: expected custody status %q, got %q", petID, domain.CustodyShelterCare, record.CustodyStatus)
		}
		if record.MicrochipID == "" {
			t.Errorf("pet %s: expected non-empty microchip ID", petID)
		}
		if record.MicrochipRegistry == "" {
			t.Errorf("pet %s: expected non-empty microchip registry", petID)
		}
	}
}
