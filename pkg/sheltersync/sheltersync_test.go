package sheltersync_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/microchip"
	"github.com/scottdensmore/petspotr/pkg/sheltersync"
)

func TestMockShelterFeedAdapter_SeededIntakes(t *testing.T) {
	t.Parallel()
	adapter := sheltersync.NewMockShelterFeedAdapter()

	intakes, err := adapter.FetchIntakes(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(intakes) != 3 {
		t.Fatalf("expected 3 seeded intakes, got %d", len(intakes))
	}

	expectedShelters := map[string]string{
		"INT-2026-8819": "shelter-sea-01",
		"INT-2026-9901": "shelter-bel-02",
		"INT-2026-7732": "shelter-kcras-03",
	}

	for _, intake := range intakes {
		shelterID, ok := expectedShelters[intake.IntakeID]
		if !ok {
			t.Errorf("unexpected intake ID %s", intake.IntakeID)
			continue
		}
		if intake.ShelterID != shelterID {
			t.Errorf("intake %s: expected shelter ID %s, got %s", intake.IntakeID, shelterID, intake.ShelterID)
		}
		if intake.Location.Address == "" {
			t.Errorf("intake %s: expected non-empty address", intake.IntakeID)
		}
		if intake.Location.Latitude == 0 || intake.Location.Longitude == 0 {
			t.Errorf("intake %s: expected non-zero coordinates", intake.IntakeID)
		}
		if len(intake.Animal.Images) == 0 {
			t.Errorf("intake %s: expected at least one image", intake.IntakeID)
		}

		// Verify microchips are valid standard transponders
		chipRes := microchip.ValidateAndNormalize(intake.Animal.MicrochipID)
		if !chipRes.Valid {
			t.Errorf("intake %s: expected valid microchip %s, got error: %s", intake.IntakeID, intake.Animal.MicrochipID, chipRes.ErrorMessage)
		}
	}
}

func TestMockShelterFeedAdapter_CustomIntakesAndError(t *testing.T) {
	t.Parallel()

	custom := sheltersync.ShelterIntakeRequest{
		ShelterID: "custom-shelter",
		IntakeID:  "custom-intake",
	}
	adapter := sheltersync.NewMockShelterFeedAdapter(custom)
	intakes, err := adapter.FetchIntakes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(intakes) != 1 || intakes[0].IntakeID != "custom-intake" {
		t.Fatalf("expected custom intake, got %v", intakes)
	}

	expectedErr := errors.New("simulated upstream failure")
	adapter.Err = expectedErr
	_, err = adapter.FetchIntakes(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestSyncWorker_SyncOnce(t *testing.T) {
	t.Parallel()

	adapter := sheltersync.NewMockShelterFeedAdapter()
	var ingested []sheltersync.ShelterIntakeRequest
	var mu sync.Mutex

	ingester := sheltersync.IngestFunc(func(ctx context.Context, req sheltersync.ShelterIntakeRequest) error {
		mu.Lock()
		defer mu.Unlock()
		ingested = append(ingested, req)
		return nil
	})

	worker := sheltersync.NewSyncWorker(adapter, ingester)
	count, err := worker.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 ingested records, got %d", count)
	}
	if len(ingested) != 3 {
		t.Errorf("expected 3 ingested records stored, got %d", len(ingested))
	}
}

func TestSyncWorker_SyncOnce_IngestError(t *testing.T) {
	t.Parallel()

	adapter := sheltersync.NewMockShelterFeedAdapter()
	failErr := errors.New("ingestion storage rejected")

	callCount := 0
	ingester := sheltersync.IngestFunc(func(ctx context.Context, req sheltersync.ShelterIntakeRequest) error {
		callCount++
		if callCount == 2 {
			return failErr
		}
		return nil
	})

	worker := sheltersync.NewSyncWorker(adapter, ingester)
	count, err := worker.SyncOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, failErr) {
		t.Errorf("expected error wrapping failErr, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record ingested before error, got %d", count)
	}
}

func TestSyncWorker_Start_ContextCancel(t *testing.T) {
	t.Parallel()

	adapter := sheltersync.NewMockShelterFeedAdapter()
	var runCount atomic.Int64
	ingester := sheltersync.IngestFunc(func(ctx context.Context, req sheltersync.ShelterIntakeRequest) error {
		runCount.Add(1)
		return nil
	})

	worker := sheltersync.NewSyncWorker(adapter, ingester)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx, 20*time.Millisecond)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not shut down in time")
	}

	if runCount.Load() == 0 {
		t.Errorf("expected at least one intake processed, got 0")
	}
}

func TestHTTPIngester(t *testing.T) {
	t.Parallel()

	var receivedHeaders http.Header
	var receivedBody sheltersync.ShelterIntakeRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	ingester := sheltersync.NewHTTPIngester(ts.URL, ts.Client())
	ingester.APIKey = "secret-shelter-token"

	intake := sheltersync.ShelterIntakeRequest{
		ShelterID: "test-shelter",
		IntakeID:  "test-intake-42",
		Animal: sheltersync.ShelterAnimalData{
			Species: "cat",
		},
		Location: sheltersync.ShelterLocationData{
			Address: "Seattle, WA",
		},
	}

	err := ingester.IngestIntake(context.Background(), intake)
	if err != nil {
		t.Fatalf("IngestIntake failed: %v", err)
	}

	if receivedHeaders.Get("X-Shelter-API-Key") != "secret-shelter-token" {
		t.Errorf("expected X-Shelter-API-Key 'secret-shelter-token', got %s", receivedHeaders.Get("X-Shelter-API-Key"))
	}
	if receivedBody.IntakeID != "test-intake-42" {
		t.Errorf("expected intake ID 'test-intake-42', got %s", receivedBody.IntakeID)
	}
}
