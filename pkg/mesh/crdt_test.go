package mesh_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

func TestMonotonicSectorRankTransitions(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-test-1")
	baseTime := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)

	// 1. Initial UNCLAIMED sector delta
	applied, err := engine.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-123",
		SectorID:             "sec-1",
		State:                domain.SectorStateUnclaimed,
		Rank:                 domain.SectorRankUnclaimed,
		ClaimedByVolunteerID: "",
		NodeID:               "node-test-1",
		LamportClock:         1,
		Timestamp:            baseTime,
	})
	if err != nil {
		t.Fatalf("unexpected error merging UNCLAIMED: %v", err)
	}
	if !applied {
		t.Fatalf("expected initial UNCLAIMED sector to be applied")
	}

	sec, ok := engine.GetSector("sec-1")
	if !ok || sec.Rank != domain.SectorRankUnclaimed {
		t.Fatalf("expected sector sec-1 to have rank UNCLAIMED, got %v (ok=%v)", sec.Rank, ok)
	}

	// 2. UNCLAIMED -> CLAIMED (Rank 0 -> 1)
	applied, err = engine.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-123",
		SectorID:             "sec-1",
		State:                domain.SectorStateClaimed,
		Rank:                 domain.SectorRankClaimed,
		ClaimedByVolunteerID: "vol-1",
		NodeID:               "node-test-1",
		LamportClock:         2,
		Timestamp:            baseTime.Add(1 * time.Minute),
	})
	if err != nil || !applied {
		t.Fatalf("expected transition to CLAIMED to succeed, applied=%v, err=%v", applied, err)
	}
	sec, _ = engine.GetSector("sec-1")
	if sec.Rank != domain.SectorRankClaimed || sec.State != domain.SectorStateClaimed {
		t.Fatalf("expected sector state CLAIMED, got %s (rank %d)", sec.State, sec.Rank)
	}

	// 3. CLAIMED -> SEARCHING (Rank 1 -> 2)
	applied, err = engine.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-123",
		SectorID:             "sec-1",
		State:                domain.SectorStateSearching,
		Rank:                 domain.SectorRankSearching,
		ClaimedByVolunteerID: "vol-1",
		NodeID:               "node-test-1",
		LamportClock:         3,
		Timestamp:            baseTime.Add(2 * time.Minute),
	})
	if err != nil || !applied {
		t.Fatalf("expected transition to SEARCHING to succeed, applied=%v, err=%v", applied, err)
	}
	sec, _ = engine.GetSector("sec-1")
	if sec.Rank != domain.SectorRankSearching {
		t.Fatalf("expected sector rank SEARCHING (2), got %d", sec.Rank)
	}

	// 4. SEARCHING -> CLEARED (Rank 2 -> 3)
	applied, err = engine.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-123",
		SectorID:             "sec-1",
		State:                domain.SectorStateCleared,
		Rank:                 domain.SectorRankCleared,
		ClaimedByVolunteerID: "vol-1",
		NodeID:               "node-test-1",
		LamportClock:         4,
		Timestamp:            baseTime.Add(3 * time.Minute),
	})
	if err != nil || !applied {
		t.Fatalf("expected transition to CLEARED to succeed, applied=%v, err=%v", applied, err)
	}
	sec, _ = engine.GetSector("sec-1")
	if sec.Rank != domain.SectorRankCleared {
		t.Fatalf("expected sector rank CLEARED (3), got %d", sec.Rank)
	}

	// 5. Stale regression attempts on CLEARED sector:
	// Try reverting CLEARED -> CLAIMED with massive Lamport clock and futuristic timestamp
	applied, err = engine.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-123",
		SectorID:             "sec-1",
		State:                domain.SectorStateClaimed,
		Rank:                 domain.SectorRankClaimed,
		ClaimedByVolunteerID: "vol-2",
		NodeID:               "node-other",
		LamportClock:         9999,
		Timestamp:            baseTime.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("unexpected error on stale merge: %v", err)
	}
	if applied {
		t.Fatalf("expected CLAIMED (rank 1) regression against CLEARED (rank 3) to be REJECTED")
	}

	// Check that sector remains CLEARED
	sec, _ = engine.GetSector("sec-1")
	if sec.Rank != domain.SectorRankCleared || sec.State != domain.SectorStateCleared {
		t.Fatalf("sector was corrupted by lower rank update! got rank %d", sec.Rank)
	}

	// Try reverting CLEARED -> SEARCHING
	applied, _ = engine.MergeSector(domain.MeshSectorDelta{
		PetID:        "pet-123",
		SectorID:     "sec-1",
		State:        domain.SectorStateSearching,
		Rank:         domain.SectorRankSearching,
		NodeID:       "node-other",
		LamportClock: 500,
		Timestamp:    baseTime.Add(10 * time.Minute),
	})
	if applied {
		t.Fatalf("expected SEARCHING (rank 2) regression against CLEARED (rank 3) to be REJECTED")
	}

	// Try reverting CLEARED -> UNCLAIMED
	applied, _ = engine.MergeSector(domain.MeshSectorDelta{
		PetID:        "pet-123",
		SectorID:     "sec-1",
		State:        domain.SectorStateUnclaimed,
		Rank:         domain.SectorRankUnclaimed,
		NodeID:       "node-other",
		LamportClock: 500,
		Timestamp:    baseTime.Add(10 * time.Minute),
	})
	if applied {
		t.Fatalf("expected UNCLAIMED (rank 0) regression against CLEARED (rank 3) to be REJECTED")
	}

	// 6. Direct jump: CLAIMED sector can be directly overwritten by CLEARED even with lower Lamport clock
	applied, err = engine.MergeSector(domain.MeshSectorDelta{
		PetID:        "pet-123",
		SectorID:     "sec-2",
		State:        domain.SectorStateClaimed,
		Rank:         domain.SectorRankClaimed,
		NodeID:       "node-test-1",
		LamportClock: 100,
		Timestamp:    baseTime,
	})
	if !applied || err != nil {
		t.Fatalf("failed to claim sec-2")
	}

	// Incoming CLEARED with clock 5 (lower than 100) must WIN because rank 3 > rank 1
	applied, err = engine.MergeSector(domain.MeshSectorDelta{
		PetID:        "pet-123",
		SectorID:     "sec-2",
		State:        domain.SectorStateCleared,
		Rank:         domain.SectorRankCleared,
		NodeID:       "node-test-2",
		LamportClock: 5,
		Timestamp:    baseTime.Add(1 * time.Minute),
	})
	if !applied || err != nil {
		t.Fatalf("expected CLEARED (rank 3) to overwrite CLAIMED (rank 1) despite lower clock, applied=%v, err=%v", applied, err)
	}
	sec, _ = engine.GetSector("sec-2")
	if sec.Rank != domain.SectorRankCleared {
		t.Fatalf("expected sec-2 to be CLEARED, got rank %d", sec.Rank)
	}
}

func TestEqualRankTieBreaking(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)

	t.Run("Lamport clock breaks ties on equal rank", func(t *testing.T) {
		engine := mesh.NewCRDTEngine("node-a")

		// Initial claim: rank CLAIMED, clock 10, node-a
		applied, err := engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-10",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-a",
			NodeID:               "node-a",
			LamportClock:         10,
			Timestamp:            baseTime,
		})
		if !applied || err != nil {
			t.Fatalf("initial claim failed: %v", err)
		}

		// Competitor claim: same rank CLAIMED, higher clock 15, node-b -> WINS
		applied, err = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-10",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-b",
			NodeID:               "node-b",
			LamportClock:         15,
			Timestamp:            baseTime,
		})
		if !applied || err != nil {
			t.Fatalf("expected higher Lamport clock to win on equal rank, applied=%v, err=%v", applied, err)
		}
		sec, _ := engine.GetSector("sec-10")
		if sec.ClaimedByVolunteerID != "vol-b" || sec.LamportClock != 15 {
			t.Fatalf("expected vol-b with clock 15 to hold sector, got %s (clock %d)", sec.ClaimedByVolunteerID, sec.LamportClock)
		}

		// Competitor claim: same rank CLAIMED, lower clock 12, node-c -> LOSES
		applied, err = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-10",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-c",
			NodeID:               "node-c",
			LamportClock:         12,
			Timestamp:            baseTime,
		})
		if err != nil {
			t.Fatalf("unexpected error on competitor claim: %v", err)
		}
		if applied {
			t.Fatalf("expected lower Lamport clock 12 to lose against clock 15")
		}
		sec, _ = engine.GetSector("sec-10")
		if sec.ClaimedByVolunteerID != "vol-b" {
			t.Fatalf("expected vol-b to retain sector")
		}
	})

	t.Run("Lexicographical NodeID breaks ties on equal rank and equal Lamport clock", func(t *testing.T) {
		engine := mesh.NewCRDTEngine("node-test")

		// Initial claim: rank CLAIMED, clock 20, NodeID "node-10"
		applied, err := engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-20",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-first",
			NodeID:               "node-10",
			LamportClock:         20,
			Timestamp:            baseTime,
		})
		if !applied || err != nil {
			t.Fatalf("initial claim failed: %v", err)
		}

		// Competitor claim: same rank CLAIMED, same clock 20, NodeID "node-20" ("node-20" > "node-10") -> WINS
		applied, err = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-20",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-winner",
			NodeID:               "node-20",
			LamportClock:         20,
			Timestamp:            baseTime,
		})
		if !applied || err != nil {
			t.Fatalf("expected higher NodeID 'node-20' to win lexicographically, applied=%v, err=%v", applied, err)
		}
		sec, _ := engine.GetSector("sec-20")
		if sec.ClaimedByVolunteerID != "vol-winner" || sec.NodeID != "node-20" {
			t.Fatalf("expected vol-winner with node-20 to win, got %s (%s)", sec.ClaimedByVolunteerID, sec.NodeID)
		}

		// Competitor claim: same rank CLAIMED, same clock 20, NodeID "node-05" ("node-05" < "node-20") -> LOSES
		applied, err = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-20",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-loser",
			NodeID:               "node-05",
			LamportClock:         20,
			Timestamp:            baseTime,
		})
		if err != nil {
			t.Fatalf("unexpected error on competitor claim: %v", err)
		}
		if applied {
			t.Fatalf("expected lower NodeID 'node-05' to lose lexicographically against 'node-20'")
		}
		sec, _ = engine.GetSector("sec-20")
		if sec.NodeID != "node-20" {
			t.Fatalf("expected node-20 to retain claim, got %s", sec.NodeID)
		}
	})

	t.Run("Fallback to ClaimedByVolunteerID if NodeID is empty", func(t *testing.T) {
		engine := mesh.NewCRDTEngine("node-test")

		_, _ = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-30",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-alpha",
			LamportClock:         5,
			Timestamp:            baseTime,
		})

		// "vol-zeta" > "vol-alpha"
		applied, _ := engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-30",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-zeta",
			LamportClock:         5,
			Timestamp:            baseTime,
		})
		if !applied {
			t.Fatalf("expected vol-zeta to win over vol-alpha on identical clock")
		}
		sec, _ := engine.GetSector("sec-30")
		if sec.ClaimedByVolunteerID != "vol-zeta" {
			t.Fatalf("expected vol-zeta to hold claim")
		}
	})

	t.Run("Identical rank, clock, and NodeID resolves via newer timestamp", func(t *testing.T) {
		engine := mesh.NewCRDTEngine("node-test")

		_, _ = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-40",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-1",
			NodeID:               "node-1",
			LamportClock:         5,
			Timestamp:            baseTime,
		})

		// Same node and clock, newer timestamp
		applied, err := engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-40",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-1-updated",
			NodeID:               "node-1",
			LamportClock:         5,
			Timestamp:            baseTime.Add(1 * time.Second),
		})
		if !applied || err != nil {
			t.Fatalf("expected newer timestamp to win, applied=%v, err=%v", applied, err)
		}
		sec, _ := engine.GetSector("sec-40")
		if sec.ClaimedByVolunteerID != "vol-1-updated" {
			t.Fatalf("expected vol-1-updated to win")
		}

		// Same node and clock, older timestamp -> rejected
		applied, _ = engine.MergeSector(domain.MeshSectorDelta{
			PetID:                "pet-1",
			SectorID:             "sec-40",
			State:                domain.SectorStateClaimed,
			Rank:                 domain.SectorRankClaimed,
			ClaimedByVolunteerID: "vol-1-stale",
			NodeID:               "node-1",
			LamportClock:         5,
			Timestamp:            baseTime.Add(-1 * time.Second),
		})
		if applied {
			t.Fatalf("expected older timestamp to lose")
		}
	})
}

func TestAppendOnlyBreadcrumbs(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-geo")
	baseTime := time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC)

	// Add breadcrumbs: (vol-1, 1), (vol-1, 2), (vol-2, 1)
	p1 := domain.MeshBreadcrumbDelta{
		PetID:         "pet-1",
		VolunteerID:   "vol-1",
		VolunteerName: "Alice",
		Seq:           1,
		Latitude:      47.6150,
		Longitude:     -122.3200,
		Accuracy:      5.0,
		Timestamp:     baseTime,
	}
	applied, err := engine.MergeBreadcrumb(p1)
	if !applied || err != nil {
		t.Fatalf("failed to merge breadcrumb 1: %v", err)
	}

	p2 := domain.MeshBreadcrumbDelta{
		PetID:         "pet-1",
		VolunteerID:   "vol-1",
		VolunteerName: "Alice",
		Seq:           2,
		Latitude:      47.6155,
		Longitude:     -122.3205,
		Accuracy:      4.5,
		Timestamp:     baseTime.Add(10 * time.Second),
	}
	applied, err = engine.MergeBreadcrumb(p2)
	if !applied || err != nil {
		t.Fatalf("failed to merge breadcrumb 2: %v", err)
	}

	p3 := domain.MeshBreadcrumbDelta{
		PetID:         "pet-1",
		VolunteerID:   "vol-2",
		VolunteerName: "Bob",
		Seq:           1,
		Latitude:      47.6160,
		Longitude:     -122.3210,
		Accuracy:      6.0,
		Timestamp:     baseTime.Add(5 * time.Second),
	}
	applied, err = engine.MergeBreadcrumb(p3)
	if !applied || err != nil {
		t.Fatalf("failed to merge breadcrumb 3: %v", err)
	}

	// Duplicate (vol-1, seq 1) with older timestamp -> rejected
	dupStale := domain.MeshBreadcrumbDelta{
		PetID:         "pet-1",
		VolunteerID:   "vol-1",
		VolunteerName: "Alice",
		Seq:           1,
		Latitude:      47.6000,
		Longitude:     -122.3000,
		Accuracy:      10.0,
		Timestamp:     baseTime.Add(-10 * time.Second),
	}
	applied, err = engine.MergeBreadcrumb(dupStale)
	if err != nil {
		t.Fatalf("unexpected error merging stale duplicate: %v", err)
	}
	if applied {
		t.Fatalf("expected duplicate breadcrumb with older timestamp to be rejected")
	}

	// Duplicate (vol-1, seq 1) with newer timestamp -> accepted and updated
	dupNewer := domain.MeshBreadcrumbDelta{
		PetID:         "pet-1",
		VolunteerID:   "vol-1",
		VolunteerName: "Alice",
		Seq:           1,
		Latitude:      47.6151,
		Longitude:     -122.3201,
		Accuracy:      3.0,
		Timestamp:     baseTime.Add(1 * time.Second),
	}
	applied, err = engine.MergeBreadcrumb(dupNewer)
	if err != nil || !applied {
		t.Fatalf("expected duplicate breadcrumb with newer timestamp to be accepted, applied=%v, err=%v", applied, err)
	}

	// Duplicate (vol-1, seq 1) with identical timestamp -> rejected
	applied, err = engine.MergeBreadcrumb(dupNewer)
	if err != nil {
		t.Fatalf("unexpected error on duplicate breadcrumb: %v", err)
	}
	if applied {
		t.Fatalf("expected identical timestamp duplicate to be rejected")
	}

	// Verify total count is 3
	crumbs := engine.GetBreadcrumbs()
	if len(crumbs) != 3 {
		t.Fatalf("expected exactly 3 breadcrumbs, got %d", len(crumbs))
	}
}

func TestAddWinsSightings(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-sighting")
	baseTime := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	// 1. Initial sighting
	applied, err := engine.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-100",
		PetID:         "pet-1",
		VolunteerID:   "vol-1",
		VolunteerName: "Alice",
		Latitude:      47.6200,
		Longitude:     -122.3300,
		Notes:         "Initial paw prints spotted near ridge",
		LamportClock:  1,
		Timestamp:     baseTime,
	})
	if !applied || err != nil {
		t.Fatalf("initial sighting merge failed: %v", err)
	}

	// 2. Update with higher Lamport clock (LWW)
	applied, err = engine.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-100",
		PetID:         "pet-1",
		VolunteerID:   "vol-2",
		VolunteerName: "Bob",
		Latitude:      47.6205,
		Longitude:     -122.3305,
		Notes:         "Visual confirmation: golden retriever spotted eating berries",
		LamportClock:  2,
		Timestamp:     baseTime.Add(5 * time.Minute),
	})
	if !applied || err != nil {
		t.Fatalf("higher clock update failed: %v", err)
	}

	sightings := engine.GetSightings()
	if len(sightings) != 1 {
		t.Fatalf("expected 1 sighting, got %d", len(sightings))
	}
	if sightings[0].Notes != "Visual confirmation: golden retriever spotted eating berries" {
		t.Fatalf("expected updated notes, got %q", sightings[0].Notes)
	}

	// 3. Stale update with lower Lamport clock -> rejected
	applied, err = engine.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-100",
		PetID:         "pet-1",
		VolunteerID:   "vol-3",
		VolunteerName: "Charlie",
		Notes:         "Stale note that arrived late",
		LamportClock:  1,
		Timestamp:     baseTime.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error merging stale sighting: %v", err)
	}
	if applied {
		t.Fatalf("expected lower clock sighting update to be rejected")
	}

	sightings = engine.GetSightings()
	if sightings[0].Notes != "Visual confirmation: golden retriever spotted eating berries" {
		t.Fatalf("stale update corrupted notes! got %q", sightings[0].Notes)
	}

	// 4. Update with identical clock but newer timestamp
	applied, err = engine.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-100",
		PetID:         "pet-1",
		VolunteerID:   "vol-2",
		VolunteerName: "Bob",
		Notes:         "Visual confirmation: golden retriever drinking water",
		LamportClock:  2,
		Timestamp:     baseTime.Add(6 * time.Minute),
	})
	if !applied || err != nil {
		t.Fatalf("expected identical clock newer timestamp update to succeed, applied=%v", applied)
	}

	// Stale update with identical clock but older timestamp
	applied, _ = engine.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-100",
		PetID:         "pet-1",
		VolunteerID:   "vol-2",
		VolunteerName: "Bob",
		Notes:         "Old notes",
		LamportClock:  2,
		Timestamp:     baseTime.Add(4 * time.Minute),
	})
	if applied {
		t.Fatalf("expected older timestamp update to fail")
	}

	// 5. Add second sighting
	applied, err = engine.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-200",
		PetID:         "pet-1",
		VolunteerID:   "vol-1",
		VolunteerName: "Alice",
		Latitude:      47.6250,
		Longitude:     -122.3350,
		Notes:         "Barking heard in sector B",
		LamportClock:  3,
		Timestamp:     baseTime.Add(15 * time.Minute),
	})
	if !applied || err != nil {
		t.Fatalf("failed to add second sighting: %v", err)
	}
	if len(engine.GetSightings()) != 2 {
		t.Fatalf("expected 2 sightings, got %d", len(engine.GetSightings()))
	}
}

func TestEvacuationAndSOSAlert(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-rescue")
	baseTime := time.Date(2026, 9, 22, 10, 30, 0, 0, time.UTC)

	// Evacuation delta merge
	applied, err := engine.MergeEvacuation(domain.MeshEvacuationDelta{
		IntakeID:     "intake-1",
		PetID:        "pet-1",
		FacilityID:   "shelter-alpha",
		Status:       "DISPATCHED",
		LamportClock: 1,
		Timestamp:    baseTime,
	})
	if !applied || err != nil {
		t.Fatalf("failed to merge evacuation delta: %v", err)
	}

	// Update evacuation status with higher clock
	applied, err = engine.MergeEvacuation(domain.MeshEvacuationDelta{
		IntakeID:     "intake-1",
		PetID:        "pet-1",
		FacilityID:   "shelter-alpha",
		Status:       "SHELTERED",
		LamportClock: 2,
		Timestamp:    baseTime.Add(10 * time.Minute),
	})
	if !applied || err != nil {
		t.Fatalf("failed to update evacuation delta: %v", err)
	}
	evacs := engine.GetEvacuations()
	if len(evacs) != 1 || evacs[0].Status != "SHELTERED" {
		t.Fatalf("expected status SHELTERED, got %v", evacs)
	}

	// Stale clock evacuation update -> rejected
	applied, _ = engine.MergeEvacuation(domain.MeshEvacuationDelta{
		IntakeID:     "intake-1",
		PetID:        "pet-1",
		FacilityID:   "shelter-alpha",
		Status:       "DISPATCHED",
		LamportClock: 1,
		Timestamp:    baseTime.Add(15 * time.Minute),
	})
	if applied {
		t.Fatalf("expected lower clock evacuation update to be rejected")
	}

	// Same clock newer timestamp evacuation update -> accepted
	applied, _ = engine.MergeEvacuation(domain.MeshEvacuationDelta{
		IntakeID:     "intake-1",
		PetID:        "pet-1",
		FacilityID:   "shelter-alpha",
		Status:       "REUNITED",
		LamportClock: 2,
		Timestamp:    baseTime.Add(20 * time.Minute),
	})
	if !applied {
		t.Fatalf("expected same clock newer timestamp evacuation to be accepted")
	}
	if engine.GetEvacuations()[0].Status != "REUNITED" {
		t.Fatalf("expected status REUNITED")
	}

	// SOS Alert merge
	alert := domain.MeshSOSAlert{
		AlertID:       "sos-911",
		VolunteerID:   "vol-k9",
		VolunteerName: "Handler Dave & K9 Buster",
		Latitude:      47.6500,
		Longitude:     -122.3500,
		Message:       "Handler heat exhaustion; need water and evacuation at ridge checkpoint 4",
		Timestamp:     baseTime.Add(20 * time.Minute),
	}
	applied, err = engine.MergeSOSAlert(alert)
	if !applied || err != nil {
		t.Fatalf("failed to merge SOS alert: %v", err)
	}

	// Duplicate SOS alert with same timestamp -> rejected
	applied, err = engine.MergeSOSAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error on duplicate SOS alert: %v", err)
	}
	if applied {
		t.Fatalf("expected duplicate SOS alert to be rejected")
	}

	// SOS alert update with newer timestamp -> accepted
	alertUpdate := alert
	alertUpdate.Message = "Handler stabilized, K9 safe"
	alertUpdate.Timestamp = baseTime.Add(25 * time.Minute)
	applied, err = engine.MergeSOSAlert(alertUpdate)
	if !applied || err != nil {
		t.Fatalf("expected newer timestamp alert to update, applied=%v, err=%v", applied, err)
	}

	alerts := engine.GetSOSAlerts()
	if len(alerts) != 1 || alerts[0].Message != "Handler stabilized, K9 safe" {
		t.Fatalf("expected updated SOS alert message, got %v", alerts)
	}
}

func TestCRDTEngine_ValidationErrors(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-val")

	if _, err := engine.MergeSector(domain.MeshSectorDelta{SectorID: ""}); err == nil {
		t.Fatalf("expected error on empty SectorID")
	}
	if _, err := engine.MergeBreadcrumb(domain.MeshBreadcrumbDelta{VolunteerID: ""}); err == nil {
		t.Fatalf("expected error on empty VolunteerID")
	}
	if _, err := engine.MergeSighting(domain.MeshSightingDelta{SightingID: ""}); err == nil {
		t.Fatalf("expected error on empty SightingID")
	}
	if _, err := engine.MergeEvacuation(domain.MeshEvacuationDelta{IntakeID: ""}); err == nil {
		t.Fatalf("expected error on empty IntakeID")
	}
	if _, err := engine.MergeSOSAlert(domain.MeshSOSAlert{AlertID: ""}); err == nil {
		t.Fatalf("expected error on empty AlertID")
	}
}

func TestCRDTEngine_AccessorsAndClocks(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-accessors")
	if engine.NodeID() != "node-accessors" {
		t.Fatalf("expected NodeID 'node-accessors', got %s", engine.NodeID())
	}

	if engine.SearchPartyID() != "" {
		t.Fatalf("expected empty initial party ID, got %s", engine.SearchPartyID())
	}
	engine.SetSearchPartyID("party-99")
	if engine.SearchPartyID() != "party-99" {
		t.Fatalf("expected party-99, got %s", engine.SearchPartyID())
	}

	if engine.LamportClock() != 0 {
		t.Fatalf("expected initial clock 0, got %d", engine.LamportClock())
	}
	c1 := engine.TickClock()
	if c1 != 1 || engine.LamportClock() != 1 {
		t.Fatalf("expected clock 1 after tick, got %d", c1)
	}

	// Test GetSectors getter
	_, _ = engine.MergeSector(domain.MeshSectorDelta{
		SectorID: "sec-alpha",
		State:    domain.SectorStateClaimed,
		Rank:     domain.SectorRankClaimed,
	})
	sectors := engine.GetSectors()
	if len(sectors) != 1 || sectors[0].SectorID != "sec-alpha" {
		t.Fatalf("expected 1 sector from GetSectors(), got %v", sectors)
	}
}

func TestThreeNodePartitionConvergence(t *testing.T) {
	t.Parallel()

	// Simulate 3 wilderness search nodes:
	// Node A: Alpine Ridge searcher
	// Node B: Valley Floor K9 handler
	// Node C: Base Camp Coordinator
	nodeA := mesh.NewCRDTEngine("node-A")
	nodeB := mesh.NewCRDTEngine("node-B")
	nodeC := mesh.NewCRDTEngine("node-C")

	nodeA.SetSearchPartyID("party-wilderness-1")
	nodeB.SetSearchPartyID("party-wilderness-1")
	nodeC.SetSearchPartyID("party-wilderness-1")

	baseTime := time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC)

	// --- PARTITION BEGINS: All 3 nodes operate completely offline from each other ---

	// Node A mutations:
	// - Claims Sector 1
	// - Records 2 breadcrumbs
	// - Records sighting-1
	_, err := nodeA.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-scout",
		SectorID:             "sec-1",
		State:                domain.SectorStateClaimed,
		Rank:                 domain.SectorRankClaimed,
		ClaimedByVolunteerID: "vol-A",
		ClaimedByName:        "Alice",
		NodeID:               "node-A",
		LamportClock:         1,
		Timestamp:            baseTime,
	})
	if err != nil {
		t.Fatalf("nodeA sector error: %v", err)
	}

	_, _ = nodeA.MergeBreadcrumb(domain.MeshBreadcrumbDelta{
		PetID:         "pet-scout",
		VolunteerID:   "vol-A",
		VolunteerName: "Alice",
		Seq:           1,
		Latitude:      47.7001,
		Longitude:     -122.4001,
		Timestamp:     baseTime.Add(1 * time.Minute),
	})
	_, _ = nodeA.MergeBreadcrumb(domain.MeshBreadcrumbDelta{
		PetID:         "pet-scout",
		VolunteerID:   "vol-A",
		VolunteerName: "Alice",
		Seq:           2,
		Latitude:      47.7005,
		Longitude:     -122.4005,
		Timestamp:     baseTime.Add(2 * time.Minute),
	})
	_, _ = nodeA.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-1",
		PetID:         "pet-scout",
		VolunteerID:   "vol-A",
		VolunteerName: "Alice",
		Notes:         "Alice spotted broken leash",
		LamportClock:  1,
		Timestamp:     baseTime.Add(3 * time.Minute),
	})

	// Node B mutations:
	// - Updates Sector 1 to SEARCHING with Lamport clock 5
	// - Claims Sector 2
	// - Records 1 breadcrumb
	// - Updates sighting-1 with higher Lamport clock 6
	_, err = nodeB.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-scout",
		SectorID:             "sec-1",
		State:                domain.SectorStateSearching,
		Rank:                 domain.SectorRankSearching,
		ClaimedByVolunteerID: "vol-B",
		ClaimedByName:        "Bob",
		NodeID:               "node-B",
		LamportClock:         5,
		Timestamp:            baseTime.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("nodeB sector error: %v", err)
	}

	_, _ = nodeB.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-scout",
		SectorID:             "sec-2",
		State:                domain.SectorStateClaimed,
		Rank:                 domain.SectorRankClaimed,
		ClaimedByVolunteerID: "vol-B",
		ClaimedByName:        "Bob",
		NodeID:               "node-B",
		LamportClock:         1,
		Timestamp:            baseTime.Add(6 * time.Minute),
	})
	_, _ = nodeB.MergeBreadcrumb(domain.MeshBreadcrumbDelta{
		PetID:         "pet-scout",
		VolunteerID:   "vol-B",
		VolunteerName: "Bob",
		Seq:           1,
		Latitude:      47.7100,
		Longitude:     -122.4100,
		Timestamp:     baseTime.Add(7 * time.Minute),
	})
	_, _ = nodeB.MergeSighting(domain.MeshSightingDelta{
		SightingID:    "sighting-1",
		PetID:         "pet-scout",
		VolunteerID:   "vol-B",
		VolunteerName: "Bob",
		Notes:         "Bob confirms pet tracks heading towards lake",
		LamportClock:  6,
		Timestamp:     baseTime.Add(8 * time.Minute),
	})

	// Node C mutations:
	// - Sets Sector 1 to CLEARED (Rank 3) with lower clock 2 (Crucial test: Rank monotonicity beats higher clock!)
	// - Claims Sector 3
	// - Emits SOS alert
	// - Records evacuation delta
	_, err = nodeC.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-scout",
		SectorID:             "sec-1",
		State:                domain.SectorStateCleared,
		Rank:                 domain.SectorRankCleared,
		ClaimedByVolunteerID: "vol-C",
		ClaimedByName:        "Charlie",
		NodeID:               "node-C",
		LamportClock:         2,
		Timestamp:            baseTime.Add(9 * time.Minute),
	})
	if err != nil {
		t.Fatalf("nodeC sector error: %v", err)
	}

	_, _ = nodeC.MergeSector(domain.MeshSectorDelta{
		PetID:                "pet-scout",
		SectorID:             "sec-3",
		State:                domain.SectorStateClaimed,
		Rank:                 domain.SectorRankClaimed,
		ClaimedByVolunteerID: "vol-C",
		ClaimedByName:        "Charlie",
		NodeID:               "node-C",
		LamportClock:         1,
		Timestamp:            baseTime.Add(10 * time.Minute),
	})
	_, _ = nodeC.MergeSOSAlert(domain.MeshSOSAlert{
		AlertID:       "sos-1",
		VolunteerID:   "vol-C",
		VolunteerName: "Charlie",
		Latitude:      47.7200,
		Longitude:     -122.4200,
		Message:       "First aid kit needed",
		Timestamp:     baseTime.Add(11 * time.Minute),
	})
	_, _ = nodeC.MergeEvacuation(domain.MeshEvacuationDelta{
		IntakeID:     "intake-1",
		PetID:        "pet-scout",
		FacilityID:   "shelter-wilderness",
		Status:       "IN_TRANSIT",
		LamportClock: 3,
		Timestamp:    baseTime.Add(12 * time.Minute),
	})

	// --- PARTITION HEALS: Full gossip / anti-entropy sync round ---
	digestA := nodeA.GenerateDigest()
	digestB := nodeB.GenerateDigest()
	digestC := nodeC.GenerateDigest()

	// Sync Node A with B and C
	nodeA.ApplyBatch(digestB)
	nodeA.ApplyBatch(digestC)

	// Sync Node B with A and C
	nodeB.ApplyBatch(digestA)
	nodeB.ApplyBatch(digestC)

	// Sync Node C with A and B
	nodeC.ApplyBatch(digestA)
	nodeC.ApplyBatch(digestB)

	// --- ASSERT MATHEMATICAL CONVERGENCE ---
	nodes := []*mesh.CRDTEngine{nodeA, nodeB, nodeC}

	for i, node := range nodes {
		t.Run(fmt.Sprintf("node_%d_state_check", i), func(t *testing.T) {
			// Sector 1 must be CLEARED on all nodes (monotonic rank 3 beats rank 2 and rank 1)
			sec1, ok := node.GetSector("sec-1")
			if !ok {
				t.Fatalf("sector 1 missing on node %d", i)
			}
			if sec1.Rank != domain.SectorRankCleared || sec1.State != domain.SectorStateCleared {
				t.Fatalf("expected sector 1 to be CLEARED on node %d, got %s (rank %d)", i, sec1.State, sec1.Rank)
			}

			// Sector 2 must be CLAIMED
			sec2, ok := node.GetSector("sec-2")
			if !ok || sec2.Rank != domain.SectorRankClaimed {
				t.Fatalf("expected sector 2 CLAIMED on node %d", i)
			}

			// Sector 3 must be CLAIMED
			sec3, ok := node.GetSector("sec-3")
			if !ok || sec3.Rank != domain.SectorRankClaimed {
				t.Fatalf("expected sector 3 CLAIMED on node %d", i)
			}

			// Breadcrumbs: exactly 3 breadcrumbs exist (vol-A #1, vol-A #2, vol-B #1)
			crumbs := node.GetBreadcrumbs()
			if len(crumbs) != 3 {
				t.Fatalf("expected 3 breadcrumbs on node %d, got %d", i, len(crumbs))
			}

			// Sightings: exactly 1 sighting exists, notes from Bob (clock 6 won over clock 1)
			sightings := node.GetSightings()
			if len(sightings) != 1 {
				t.Fatalf("expected 1 sighting on node %d, got %d", i, len(sightings))
			}
			if sightings[0].Notes != "Bob confirms pet tracks heading towards lake" {
				t.Fatalf("expected clock 6 notes on node %d, got %q", i, sightings[0].Notes)
			}

			// SOS Alert: exactly 1 alert exists
			alerts := node.GetSOSAlerts()
			if len(alerts) != 1 || alerts[0].AlertID != "sos-1" {
				t.Fatalf("expected 1 SOS alert on node %d, got %v", i, alerts)
			}

			// Evacuation: exactly 1 evacuation exists with status IN_TRANSIT
			evacs := node.GetEvacuations()
			if len(evacs) != 1 || evacs[0].Status != "IN_TRANSIT" {
				t.Fatalf("expected 1 evacuation on node %d, got %v", i, evacs)
			}
		})
	}

	// Assert byte-for-byte serialization convergence across all 3 nodes
	// Note: Digest contains SenderNodeID which differs per node, but contents must match
	dFinalA := nodeA.GenerateDigest()
	dFinalB := nodeB.GenerateDigest()
	dFinalC := nodeC.GenerateDigest()

	// Normalize SenderNodeID for deterministic comparison of data contents
	dFinalB.SenderNodeID = dFinalA.SenderNodeID
	dFinalC.SenderNodeID = dFinalA.SenderNodeID

	jsonA, err := json.Marshal(dFinalA)
	if err != nil {
		t.Fatalf("marshal A failed: %v", err)
	}
	jsonB, err := json.Marshal(dFinalB)
	if err != nil {
		t.Fatalf("marshal B failed: %v", err)
	}
	jsonC, err := json.Marshal(dFinalC)
	if err != nil {
		t.Fatalf("marshal C failed: %v", err)
	}

	if string(jsonA) != string(jsonB) {
		t.Fatalf("Node A and Node B did not converge to identical state!\nA: %s\nB: %s", jsonA, jsonB)
	}
	if string(jsonA) != string(jsonC) {
		t.Fatalf("Node A and Node C did not converge to identical state!\nA: %s\nC: %s", jsonA, jsonC)
	}
}

func TestCRDTEngine_Concurrency(t *testing.T) {
	t.Parallel()

	engine := mesh.NewCRDTEngine("node-concurrent")
	var wg sync.WaitGroup

	numGoroutines := 16
	numOps := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				// Concurrent Sector merges
				_, _ = engine.MergeSector(domain.MeshSectorDelta{
					PetID:                "pet-c",
					SectorID:             fmt.Sprintf("sec-%d", i%5),
					State:                domain.SectorStateSearching,
					Rank:                 domain.SectorRankSearching,
					ClaimedByVolunteerID: fmt.Sprintf("vol-%d", gid),
					NodeID:               fmt.Sprintf("node-%d", gid),
					LamportClock:         uint64(i + 1),
					Timestamp:            time.Now(),
				})

				// Concurrent Breadcrumb merges
				_, _ = engine.MergeBreadcrumb(domain.MeshBreadcrumbDelta{
					PetID:         "pet-c",
					VolunteerID:   fmt.Sprintf("vol-%d", gid),
					VolunteerName: fmt.Sprintf("Volunteer %d", gid),
					Seq:           uint64(i),
					Latitude:      47.0 + float64(i)*0.001,
					Longitude:     -122.0 - float64(gid)*0.001,
					Accuracy:      5.0,
					Timestamp:     time.Now(),
				})

				// Concurrent Sighting merges
				_, _ = engine.MergeSighting(domain.MeshSightingDelta{
					SightingID:    fmt.Sprintf("sight-%d", i%3),
					PetID:         "pet-c",
					VolunteerID:   fmt.Sprintf("vol-%d", gid),
					VolunteerName: fmt.Sprintf("Volunteer %d", gid),
					Notes:         fmt.Sprintf("Sight note %d from g %d", i, gid),
					LamportClock:  uint64(i + 1),
					Timestamp:     time.Now(),
				})

				// Concurrent Digest generation
				if i%10 == 0 {
					_ = engine.GenerateDigest()
				}
			}
		}(g)
	}

	wg.Wait()

	// Final digest should succeed and be non-empty
	finalDigest := engine.GenerateDigest()
	if len(finalDigest.Sectors) == 0 || len(finalDigest.Breadcrumbs) == 0 {
		t.Fatalf("expected non-empty sectors and breadcrumbs after concurrent load")
	}
}
