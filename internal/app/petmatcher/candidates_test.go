package petmatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMatcherWorkerSelectsEligibleCandidateDeterministically(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	foundPoint := domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}
	nearPoint := domain.LocationPoint{Latitude: 47.6200, Longitude: -122.3200}
	farPoint := domain.LocationPoint{Latitude: 47.2529, Longitude: -122.4443}

	st := store.NewMemoryStore()
	seedCandidateRecord(t, st, domain.LostPetRecord{
		PetID: "lost-101", PetName: "Out of range", Species: "Dog", Breed: "Golden Retriever",
		PrimaryColor: "Golden", OwnerIdentityRef: "identity-outside", ReportedAt: now.Add(-time.Hour),
		Location: "Tacoma, WA", GeocodingStatus: domain.GeocodingVerified, Coordinates: &farPoint,
		Status: domain.LostPetStatusLost,
	})
	if err := st.SaveState(context.Background(), store.LostPetsCollection, "lost-corrupt", []byte(`{invalid`)); err != nil {
		t.Fatal(err)
	}
	for _, petID := range []string{"lost-beta", "lost-alpha"} {
		seedCandidateRecord(t, st, domain.LostPetRecord{
			PetID: petID, PetName: petID, Species: "Dog", Breed: "Golden Retriever",
			PrimaryColor: "Golden", OwnerIdentityRef: "identity-" + petID, ReportedAt: now.Add(-2 * time.Hour),
			Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingVerified, Coordinates: &nearPoint,
			Status: domain.LostPetStatusLost,
		})
	}
	seedCandidateRecord(t, st, domain.LostPetRecord{
		PetID: "lost-expired", Species: "Dog", Breed: "Golden Retriever", PrimaryColor: "Golden",
		OwnerIdentityRef: "identity-expired", ReportedAt: now.Add(-31 * 24 * time.Hour),
		Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingVerified, Coordinates: &nearPoint,
		Status: domain.LostPetStatusLost,
	})
	seedCandidateRecord(t, st, domain.LostPetRecord{
		PetID: "lost-pending", Species: "Dog", Breed: "Golden Retriever", PrimaryColor: "Golden",
		OwnerIdentityRef: "identity-pending", ReportedAt: now.Add(-time.Hour),
		Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Status: domain.LostPetStatusLost,
	})
	seedCandidateRecord(t, st, domain.LostPetRecord{
		PetID: "lost-closed", Species: "Dog", Breed: "Golden Retriever", PrimaryColor: "Golden",
		OwnerIdentityRef: "identity-closed", ReportedAt: now.Add(-time.Hour),
		Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingVerified, Coordinates: &nearPoint,
		Status: domain.LostPetStatus("reunited"),
	})
	seedCandidateRecord(t, st, domain.LostPetRecord{
		PetID: "lost-cat", Species: "Cat", Breed: "Golden Retriever", PrimaryColor: "Golden",
		OwnerIdentityRef: "identity-cat", ReportedAt: now.Add(-time.Hour),
		Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingVerified, Coordinates: &nearPoint,
		Status: domain.LostPetStatusLost,
	})

	ps := pubsub.NewMemoryPubSub()
	var published domain.MatchResult
	if err := ps.Subscribe("matchFound", func(_ context.Context, data []byte) error {
		_, err := domain.DecodeEventPayload(data, domain.EventTypeMatchFound, &published)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	worker := NewWorker(st, ps, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	worker.now = func() time.Time { return now }

	foundData := encodeFoundCandidateEvent(t, domain.FoundPetReportedV2{
		PetID: "found-candidate", ImageURL: "https://storage.petspotr.io/found-candidate.jpg",
		FoundAt: now, Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: &foundPoint, Species: "Dog", Breed: "Golden Retriever", PrimaryColor: "Golden",
		CustodyStatus: domain.CustodyFinderHome, Status: domain.FoundPetStatusFound,
	})
	if err := worker.ProcessFoundPet(context.Background(), foundData); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}
	if published.MatchedPetID != "lost-alpha" {
		t.Fatalf("matched pet ID = %q, want deterministic eligible candidate lost-alpha", published.MatchedPetID)
	}
	matches, err := st.ListState(context.Background(), store.MatchesCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("persisted matches = %d, want one deterministic winner", len(matches))
	}
	var persisted domain.MatchRecord
	for _, data := range matches {
		if err := json.Unmarshal(data, &persisted); err != nil {
			t.Fatal(err)
		}
	}
	if persisted.MatchedPetID != "lost-alpha" || persisted.LostPet.PetName != "lost-alpha" ||
		persisted.Scores.DistanceMiles != domain.HaversineDistanceMiles(foundPoint, nearPoint) {
		t.Fatalf("persisted deterministic winner = %#v", persisted)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls = %d, want 1", got)
	}
}

func TestMatcherWorkerDefersUnverifiedFoundLocation(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	point := domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}
	st := store.NewMemoryStore()
	seedCandidateRecord(t, st, domain.LostPetRecord{
		PetID: "lost-101", Species: "Dog", Breed: "Golden Retriever", PrimaryColor: "Golden",
		OwnerIdentityRef: "identity-lost-101", ReportedAt: now.Add(-time.Hour),
		Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingVerified, Coordinates: &point,
		Status: domain.LostPetStatusLost,
	})
	ps := pubsub.NewMemoryPubSub()
	var publications atomic.Int32
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error {
		publications.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	worker := NewWorker(st, ps, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	worker.now = func() time.Time { return now }

	foundData := encodeFoundCandidateEvent(t, domain.FoundPetReportedV2{
		PetID: "found-pending", ImageURL: "https://storage.petspotr.io/found-pending.jpg",
		FoundAt: now, Location: "Capitol Hill, Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Species: "Dog", Breed: "Golden Retriever", PrimaryColor: "Golden",
		CustodyStatus: domain.CustodyFinderHome, Status: domain.FoundPetStatusFound,
	})
	if err := worker.ProcessFoundPet(context.Background(), foundData); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 0 {
		t.Fatalf("Ollama calls = %d, want 0 when found coordinates are unverified", got)
	}
	if got := publications.Load(); got != 0 {
		t.Fatalf("matchFound publications = %d, want 0", got)
	}
	matches, err := st.ListState(context.Background(), store.MatchesCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("persisted matches = %d, want 0", len(matches))
	}
}

func TestMatcherCandidateSelectionOrdersByScoreDistanceAndID(t *testing.T) {
	tests := []struct {
		name       string
		current    rankedCandidate
		challenger rankedCandidate
		want       bool
	}{
		{
			name: "higher score wins despite later ID",
			current: rankedCandidate{
				candidate: lostPetCandidate{record: domain.LostPetRecord{PetID: "lost-alpha"}, distanceMiles: 1},
				result:    &domain.MatchResult{Score: 0.80},
			},
			challenger: rankedCandidate{
				candidate: lostPetCandidate{record: domain.LostPetRecord{PetID: "lost-zulu"}, distanceMiles: 5},
				result:    &domain.MatchResult{Score: 0.90},
			},
			want: true,
		},
		{
			name: "nearer candidate wins tied score despite later ID",
			current: rankedCandidate{
				candidate: lostPetCandidate{record: domain.LostPetRecord{PetID: "lost-alpha"}, distanceMiles: 5},
				result:    &domain.MatchResult{Score: 0.90},
			},
			challenger: rankedCandidate{
				candidate: lostPetCandidate{record: domain.LostPetRecord{PetID: "lost-zulu"}, distanceMiles: 1},
				result:    &domain.MatchResult{Score: 0.90},
			},
			want: true,
		},
		{
			name: "lower ID wins tied score and distance",
			current: rankedCandidate{
				candidate: lostPetCandidate{record: domain.LostPetRecord{PetID: "lost-zulu"}, distanceMiles: 1},
				result:    &domain.MatchResult{Score: 0.90},
			},
			challenger: rankedCandidate{
				candidate: lostPetCandidate{record: domain.LostPetRecord{PetID: "lost-alpha"}, distanceMiles: 1},
				result:    &domain.MatchResult{Score: 0.90},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outranks(tt.challenger, tt.current); got != tt.want {
				t.Fatalf("outranks() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestMatcherCandidateLogsDoNotLeakMalformedValues(t *testing.T) {
	const secret = "sentinel-private-value"
	malformed := []byte(`{"petId":"lost-secret","reportedAt":"` + secret + `"}`)
	st := &fixedCandidateQueryStore{
		MemoryStore: store.NewMemoryStore(),
		records:     map[string][]byte{"lost-secret": malformed},
	}

	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	})

	worker := NewWorker(st, pubsub.NewMemoryPubSub(), nil)
	_, err := worker.eligibleLostPetCandidates(context.Background(), domain.FoundPetReportedV2{
		PetID: "found-safe-log", FoundAt: time.Now().UTC(), GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), Status: domain.FoundPetStatusFound, CustodyStatus: domain.CustodyUnknown,
	})
	if err != nil {
		t.Fatalf("eligibleLostPetCandidates() error = %v", err)
	}
	if got := output.String(); bytes.Contains([]byte(got), []byte(secret)) {
		t.Fatalf("candidate log leaked malformed value: %q", got)
	} else if !bytes.Contains([]byte(got), []byte("Skipping malformed lost-pet candidate")) {
		t.Fatalf("candidate log omitted safe classification: %q", got)
	}
}

func TestBoundedCandidateQueriesWrapAntimeridian(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	queries := boundedCandidateQueries(domain.FoundPetReportedV2{
		FoundAt: now, Species: "Dog",
		Coordinates: &domain.LocationPoint{Latitude: 0, Longitude: 179.9},
	})
	if len(queries) != 2 {
		t.Fatalf("boundedCandidateQueries() count = %d, want 2 at antimeridian", len(queries))
	}
	if queries[0].MinLongitude < -180 || queries[0].MaxLongitude > 180 ||
		queries[1].MinLongitude < -180 || queries[1].MaxLongitude > 180 {
		t.Fatalf("wrapped longitude ranges are invalid: %#v", queries)
	}
	if queries[0].ReportedAfter != now.Add(-matcherCandidateWindow) ||
		queries[0].ReportedBefore != now.Add(matcherCandidateWindow) {
		t.Fatalf("candidate time window = %s..%s, want +/- %s", queries[0].ReportedAfter, queries[0].ReportedBefore, matcherCandidateWindow)
	}
	containsEast := func(query store.LostPetCandidateQuery) bool {
		return query.MinLongitude <= 179.95 && query.MaxLongitude >= 179.95
	}
	containsWest := func(query store.LostPetCandidateQuery) bool {
		return query.MinLongitude <= -179.95 && query.MaxLongitude >= -179.95
	}
	if (!containsEast(queries[0]) && !containsEast(queries[1])) ||
		(!containsWest(queries[0]) && !containsWest(queries[1])) {
		t.Fatalf("wrapped queries do not cover both sides of antimeridian: %#v", queries)
	}
}

type fixedCandidateQueryStore struct {
	*store.MemoryStore
	records map[string][]byte
}

func (s *fixedCandidateQueryStore) QueryLostPetCandidates(
	context.Context,
	store.LostPetCandidateQuery,
) (map[string][]byte, error) {
	return s.records, nil
}

func seedCandidateRecord(t *testing.T, st store.StateStore, record domain.LostPetRecord) {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveState(context.Background(), store.LostPetsCollection, record.PetID, data); err != nil {
		t.Fatal(err)
	}
}

func encodeFoundCandidateEvent(t *testing.T, event domain.FoundPetReportedV2) []byte {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeFoundPetReported, OccurredAt: event.FoundAt, AggregateID: event.PetID,
		AggregateVersion: 1, PayloadVersion: domain.FoundPetReportedPayloadVersion, Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
