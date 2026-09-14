package store

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewFirestoreStoreRejectsMissingProject(t *testing.T) {
	t.Parallel()

	if _, err := NewFirestoreStore(context.Background(), ""); err == nil {
		t.Fatal("NewFirestoreStore() error = nil, want non-nil")
	}
}

func TestFirestoreDocumentIDSupportsOpaqueKeys(t *testing.T) {
	t.Parallel()

	firstKey := "https://push.example.test/send/a/b?token=one"
	secondKey := "https://push.example.test/send/a/b?token=two"
	firstID := firestoreDocumentID(firstKey)
	secondID := firestoreDocumentID(secondKey)

	if firstID == secondID {
		t.Fatal("different opaque keys produced the same document ID")
	}
	if len(firstID) != 64 {
		t.Fatalf("document ID length = %d, want 64", len(firstID))
	}
	if strings.Contains(firstID, "/") {
		t.Fatalf("document ID %q contains a path separator", firstID)
	}
}

func TestNewFirestoreEmulatorStoreRejectsMismatchedEnvironment(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8085")

	if _, err := NewFirestoreEmulatorStore(context.Background(), "petspotr-test", "127.0.0.1:8086"); err == nil {
		t.Fatal("NewFirestoreEmulatorStore() error = nil, want non-nil")
	}
}

func TestNewFirestoreRecord_LostPetCandidateEmbedding(t *testing.T) {
	t.Parallel()

	rawEmbedding := make([]float32, 768)
	for i := range rawEmbedding {
		rawEmbedding[i] = float32(i+1) * 0.001
	}

	payloadWithEmbedding, err := json.Marshal(map[string]any{
		"petId":           "lost-with-emb",
		"species":         "Dog",
		"reportedAt":      time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
		"geocodingStatus": "verified",
		"status":          "lost",
		"coordinates": map[string]float64{
			"latitude":  47.6062,
			"longitude": -122.3321,
		},
		"embedding": rawEmbedding,
	})
	if err != nil {
		t.Fatalf("marshal payload with embedding: %v", err)
	}

	recWithEmb, err := newFirestoreRecord(LostPetsCollection, "lost-with-emb", payloadWithEmbedding)
	if err != nil {
		t.Fatalf("newFirestoreRecord() error = %v", err)
	}
	if len(recWithEmb.LostEmbedding) != 768 {
		t.Fatalf("LostEmbedding length = %d, want 768", len(recWithEmb.LostEmbedding))
	}
	if !reflect.DeepEqual(recWithEmb.LostEmbedding, rawEmbedding) {
		t.Errorf("LostEmbedding mismatch: got %v, want %v", recWithEmb.LostEmbedding[:5], rawEmbedding[:5])
	}

	payloadWithoutEmbedding, err := json.Marshal(map[string]any{
		"petId":           "lost-no-emb",
		"species":         "Cat",
		"reportedAt":      time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
		"geocodingStatus": "verified",
		"status":          "lost",
		"coordinates": map[string]float64{
			"latitude":  47.6062,
			"longitude": -122.3321,
		},
	})
	if err != nil {
		t.Fatalf("marshal payload without embedding: %v", err)
	}

	recNoEmb, err := newFirestoreRecord(LostPetsCollection, "lost-no-emb", payloadWithoutEmbedding)
	if err != nil {
		t.Fatalf("newFirestoreRecord() error = %v", err)
	}
	if len(recNoEmb.LostEmbedding) != 0 {
		t.Fatalf("LostEmbedding length = %d, want 0", len(recNoEmb.LostEmbedding))
	}
}

func TestLostPetCandidateIndexUpdates_Embedding(t *testing.T) {
	t.Parallel()

	rawEmbedding := make([]float32, 768)
	for i := range rawEmbedding {
		rawEmbedding[i] = float32(i+1) * 0.001
	}

	species := "dog"
	lat := 47.6062
	lng := -122.3321
	now := time.Now().UTC()

	t.Run("includes lostEmbedding when present", func(t *testing.T) {
		rec := firestoreRecord{
			Key:                 "lost-rec-1",
			LostStatus:          "lost",
			LostGeocodingStatus: "verified",
			LostSpecies:         &species,
			LostReportedAt:      now,
			LostLatitude:        &lat,
			LostLongitude:       &lng,
			LostEmbedding:       rawEmbedding,
		}

		updates := lostPetCandidateIndexUpdates(rec)
		var found bool
		for _, u := range updates {
			if u.Path == "lostEmbedding" {
				found = true
				if !reflect.DeepEqual(u.Value, rawEmbedding) {
					t.Errorf("lostEmbedding update value = %v, want %v", u.Value, rawEmbedding)
				}
			}
		}
		if !found {
			t.Error("lostPetCandidateIndexUpdates omitted lostEmbedding update when LostEmbedding is present")
		}
	})

	t.Run("omits lostEmbedding when empty", func(t *testing.T) {
		rec := firestoreRecord{
			Key:                 "lost-rec-2",
			LostStatus:          "lost",
			LostGeocodingStatus: "verified",
			LostSpecies:         &species,
			LostReportedAt:      now,
			LostLatitude:        &lat,
			LostLongitude:       &lng,
			LostEmbedding:       nil,
		}

		updates := lostPetCandidateIndexUpdates(rec)
		for _, u := range updates {
			if u.Path == "lostEmbedding" {
				t.Error("lostPetCandidateIndexUpdates included lostEmbedding update when LostEmbedding is nil/empty")
			}
		}
	})
}
