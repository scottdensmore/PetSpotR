package mesh_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type mockBroadcaster struct {
	mu     sync.Mutex
	events []domain.ReunionStreamEvent
}

func (m *mockBroadcaster) Broadcast(event domain.ReunionStreamEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockBroadcaster) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestReconcileUplinkBatch_NilStore(t *testing.T) {
	_, err := mesh.ReconcileUplinkBatch(context.Background(), nil, domain.MeshBatchDelta{})
	if err == nil {
		t.Fatalf("expected error when state store is nil")
	}
}

func TestReconcileUplinkBatch_MonotonicSectorProgression(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	partyID := "party-uplink-1"
	petID := "pet-uplink-1"
	sectorID := "sec-uplink-1"

	// Seed search party with CLAIMED sector (Rank 1)
	party := searchparty.SearchParty{
		PartyID:   partyID,
		LostPetID: petID,
		CenterCoordinates: domain.LocationPoint{
			Latitude:  47.6,
			Longitude: -122.3,
		},
		RadiusMeters: 500,
		CreatedAt:    now.Add(-1 * time.Hour),
		Sectors: []searchparty.SearchSector{
			{
				SectorID: sectorID,
				Name:     "Ridge Sector",
				PolygonPoints: []domain.LocationPoint{
					{Latitude: 47.6, Longitude: -122.3},
					{Latitude: 47.61, Longitude: -122.3},
					{Latitude: 47.61, Longitude: -122.29},
					{Latitude: 47.6, Longitude: -122.29},
				},
				Status:       searchparty.SectorStatusActiveSearch, // Rank 1/2
				TotalAreaSqM: 100000,
			},
		},
	}
	partyBytes, _ := json.Marshal(party)
	_ = st.SaveState(ctx, store.SearchPartiesCollection, partyID, partyBytes)

	// Save baseline sector delta on server: SEARCHING (Rank 2), Clock 10
	baselineDelta := domain.MeshSectorDelta{
		PetID:        petID,
		SectorID:     sectorID,
		State:        domain.SectorStateSearching,
		Rank:         domain.SectorRankSearching,
		NodeID:       "node-server",
		LamportClock: 10,
		Timestamp:    now.Add(-30 * time.Minute),
	}
	baseBytes, _ := json.Marshal(baselineDelta)
	_ = st.SaveState(ctx, mesh.MeshSectorDeltasCollection, sectorID, baseBytes)

	broadcaster := &mockBroadcaster{}

	// Case 1: Client sends CLEARED (Rank 3) -> Higher rank must win regardless of Lamport clock
	resp, err := mesh.ReconcileUplinkBatch(ctx, st, domain.MeshBatchDelta{
		SearchPartyID: partyID,
		Sectors: []domain.MeshSectorDelta{
			{
				PetID:                petID,
				SectorID:             sectorID,
				State:                domain.SectorStateCleared,
				Rank:                 domain.SectorRankCleared,
				ClaimedByVolunteerID: "vol-k9",
				ClaimedByName:        "K9 Handler",
				NodeID:               "node-field",
				LamportClock:         5, // lower than server clock, but rank is higher
				Timestamp:            now,
			},
		},
	}, mesh.WithBroadcaster(broadcaster))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ReconciledSectors != 1 {
		t.Errorf("expected 1 reconciled sector, got %d", resp.ReconciledSectors)
	}
	if resp.ServerLatestDeltas != nil {
		t.Errorf("expected nil ServerLatestDeltas when client wins, got %+v", resp.ServerLatestDeltas)
	}

	// Verify store was updated to CLEARED
	secBytes, _ := st.GetState(ctx, mesh.MeshSectorDeltasCollection, sectorID)
	var updatedSec domain.MeshSectorDelta
	_ = json.Unmarshal(secBytes, &updatedSec)
	if updatedSec.Rank != domain.SectorRankCleared {
		t.Errorf("stored sector rank = %d, want %d", updatedSec.Rank, domain.SectorRankCleared)
	}

	// Case 2: Client sends SEARCHING (Rank 2) -> Lower rank must be rejected
	resp2, err := mesh.ReconcileUplinkBatch(ctx, st, domain.MeshBatchDelta{
		SearchPartyID: partyID,
		Sectors: []domain.MeshSectorDelta{
			{
				PetID:        petID,
				SectorID:     sectorID,
				State:        domain.SectorStateSearching,
				Rank:         domain.SectorRankSearching,
				NodeID:       "node-field-stale",
				LamportClock: 20, // even higher clock cannot revert CLEARED to SEARCHING
				Timestamp:    now,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.ReconciledSectors != 0 {
		t.Errorf("expected 0 reconciled sectors (stale rejected), got %d", resp2.ReconciledSectors)
	}
	if resp2.ServerLatestDeltas == nil || len(resp2.ServerLatestDeltas.Sectors) == 0 {
		t.Fatalf("expected ServerLatestDeltas to contain server CLEARED state")
	}
	if resp2.ServerLatestDeltas.Sectors[0].Rank != domain.SectorRankCleared {
		t.Errorf("ServerLatestDeltas rank = %d, want CLEARED (3)", resp2.ServerLatestDeltas.Sectors[0].Rank)
	}
}

func TestReconcileUplinkBatch_BreadcrumbsAndSightings(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	partyID := "party-full-sync"
	petID := "pet-full-sync"

	broadcaster := &mockBroadcaster{}

	batch := domain.MeshBatchDelta{
		SearchPartyID: partyID,
		SenderNodeID:  "node-field",
		Breadcrumbs: []domain.MeshBreadcrumbDelta{
			{
				PetID:         petID,
				VolunteerID:   "vol-alice",
				VolunteerName: "Alice",
				Seq:           1,
				Latitude:      47.6201,
				Longitude:     -122.3491,
				Accuracy:      4.2,
				Timestamp:     now,
			},
			{
				PetID:         petID,
				VolunteerID:   "vol-alice",
				VolunteerName: "Alice",
				Seq:           2,
				Latitude:      47.6205,
				Longitude:     -122.3485,
				Accuracy:      3.8,
				Timestamp:     now.Add(1 * time.Minute),
			},
		},
		Sightings: []domain.MeshSightingDelta{
			{
				SightingID:       "sight-101",
				PetID:            petID,
				VolunteerID:      "vol-alice",
				VolunteerName:    "Alice",
				Latitude:         47.6210,
				Longitude:        -122.3480,
				Notes:            "Brown dog sighted drinking from stream",
				PhotoThumbBase64: "data:image/jpeg;base64,...",
				LamportClock:     1,
				Timestamp:        now.Add(2 * time.Minute),
			},
		},
		Evacuations: []domain.MeshEvacuationDelta{
			{
				IntakeID:     "intake-501",
				PetID:        petID,
				FacilityID:   "hub-seattle-center",
				Status:       "INTAKEN",
				LamportClock: 1,
				Timestamp:    now,
			},
		},
		SOSAlerts: []domain.MeshSOSAlert{
			{
				AlertID:       "sos-999",
				VolunteerID:   "vol-alice",
				VolunteerName: "Alice",
				Latitude:      47.6215,
				Longitude:     -122.3475,
				Message:       "Injured ankle on rocky incline",
				Timestamp:     now.Add(3 * time.Minute),
			},
		},
	}

	resp, err := mesh.ReconcileUplinkBatch(ctx, st, batch, mesh.WithBroadcaster(broadcaster))
	if err != nil {
		t.Fatalf("unexpected error during full reconcile: %v", err)
	}

	if resp.ReconciledBreadcrumbs != 2 {
		t.Errorf("ReconciledBreadcrumbs = %d, want 2", resp.ReconciledBreadcrumbs)
	}
	if resp.ReconciledSightings != 1 {
		t.Errorf("ReconciledSightings = %d, want 1", resp.ReconciledSightings)
	}
	if resp.ReconciledEvacuations != 1 {
		t.Errorf("ReconciledEvacuations = %d, want 1", resp.ReconciledEvacuations)
	}

	// Verify breadcrumbs trail in store
	trailBytes, err := st.GetState(ctx, store.BreadcrumbsCollection, "trail-"+partyID+"-vol-alice")
	if err != nil {
		t.Fatalf("failed to get trail: %v", err)
	}
	var trail searchparty.VolunteerBreadcrumbTrail
	_ = json.Unmarshal(trailBytes, &trail)
	if len(trail.Points) != 2 {
		t.Errorf("trail points count = %d, want 2", len(trail.Points))
	}

	// Verify sighting in store
	sightBytes, err := st.GetState(ctx, store.SightingsCollection, "sight-101")
	if err != nil {
		t.Fatalf("failed to get sighting: %v", err)
	}
	var sighting domain.PetSightingRecord
	_ = json.Unmarshal(sightBytes, &sighting)
	if !strings.Contains(sighting.Notes, "Brown dog") {
		t.Errorf("sighting notes = %q, want containing 'Brown dog'", sighting.Notes)
	}

	// Verify broadcaster received events
	if broadcaster.count() < 3 {
		t.Errorf("broadcaster received %d events, want at least 3", broadcaster.count())
	}
}
