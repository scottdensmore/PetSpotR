package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMemoryStore_ScanActiveReportsMissingEmbeddings(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()

	// Add a lost pet missing embedding
	lost1 := domain.LostPetRecord{
		PetID:      "lost1",
		Status:     domain.LostPetStatusLost,
		ReportedAt: time.Now(),
		Embedding:  nil,
	}
	lost1Data, _ := json.Marshal(lost1)
	if err := s.SaveState(ctx, store.LostPetsCollection, "lost1", lost1Data); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Add a lost pet with embedding
	lost2 := domain.LostPetRecord{
		PetID:      "lost2",
		Status:     domain.LostPetStatusLost,
		ReportedAt: time.Now(),
		Embedding:  []float32{1.0, 2.0},
	}
	lost2Data, _ := json.Marshal(lost2)
	if err := s.SaveState(ctx, store.LostPetsCollection, "lost2", lost2Data); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	dataMap, nextCursor, err := s.ScanActiveReportsMissingEmbeddings(ctx, store.LostPetsCollection, 10, "")
	if err != nil {
		t.Fatalf("ScanActiveReportsMissingEmbeddings() error = %v", err)
	}
	if len(dataMap) != 1 {
		t.Errorf("expected 1 record, got %d", len(dataMap))
	}
	if _, ok := dataMap["lost1"]; !ok {
		t.Errorf("expected lost1 to be returned")
	}
	if nextCursor != "lost1" {
		t.Errorf("expected cursor lost1, got %s", nextCursor)
	}
}
