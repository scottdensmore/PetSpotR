package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestMeshDomainModels(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	t.Run("SectorRank string and state mappings", func(t *testing.T) {
		ranks := []domain.SectorRank{
			domain.SectorRankUnclaimed,
			domain.SectorRankClaimed,
			domain.SectorRankSearching,
			domain.SectorRankCleared,
			domain.SectorRank(99),
		}

		expectedStates := []string{
			domain.SectorStateUnclaimed,
			domain.SectorStateClaimed,
			domain.SectorStateSearching,
			domain.SectorStateCleared,
			domain.SectorStateUnclaimed,
		}

		for i, r := range ranks {
			if r.State() != expectedStates[i] {
				t.Errorf("rank %d: expected state %q, got %q", r, expectedStates[i], r.State())
			}
		}

		stateToRank := map[string]domain.SectorRank{
			"unclaimed": domain.SectorRankUnclaimed,
			"CLAIMED":   domain.SectorRankClaimed,
			"Searching": domain.SectorRankSearching,
			"cleared":   domain.SectorRankCleared,
			"unknown":   domain.SectorRankUnclaimed,
		}

		for s, expected := range stateToRank {
			got := domain.RankFromState(s)
			if got != expected {
				t.Errorf("RankFromState(%q): expected %d, got %d", s, expected, got)
			}
		}
	})

	t.Run("MeshNode JSON serialization", func(t *testing.T) {
		node := domain.MeshNode{
			NodeID:         "node-alpha-1",
			VolunteerID:    "vol-10",
			VolunteerName:  "Sarah Jenkins",
			Role:           domain.MeshRoleK9Handler,
			SearchPartyID:  "party-99",
			BatteryLevel:   88,
			ConnectedPeers: []string{"node-bravo-2"},
			LastSeenAt:     now,
		}

		bytes, err := json.Marshal(node)
		if err != nil {
			t.Fatalf("marshal MeshNode failed: %v", err)
		}

		var decoded domain.MeshNode
		if err := json.Unmarshal(bytes, &decoded); err != nil {
			t.Fatalf("unmarshal MeshNode failed: %v", err)
		}

		if decoded.NodeID != node.NodeID || decoded.Role != domain.MeshRoleK9Handler || decoded.BatteryLevel != 88 {
			t.Fatalf("mismatch in decoded MeshNode: %+v", decoded)
		}
	})

	t.Run("MeshBatchDelta and UplinkSyncResponse JSON serialization", func(t *testing.T) {
		batch := domain.MeshBatchDelta{
			SearchPartyID: "party-99",
			SenderNodeID:  "node-alpha-1",
			Sectors: []domain.MeshSectorDelta{
				{
					PetID:                "pet-1",
					SectorID:             "sec-1",
					State:                domain.SectorStateCleared,
					Rank:                 domain.SectorRankCleared,
					ClaimedByVolunteerID: "vol-10",
					ClaimedByName:        "Sarah",
					LamportClock:         5,
					Timestamp:            now,
				},
			},
			Breadcrumbs: []domain.MeshBreadcrumbDelta{
				{
					PetID:         "pet-1",
					VolunteerID:   "vol-10",
					VolunteerName: "Sarah",
					Seq:           1,
					Latitude:      47.6,
					Longitude:     -122.3,
					Accuracy:      4.0,
					Timestamp:     now,
				},
			},
			Sightings: []domain.MeshSightingDelta{
				{
					SightingID:    "sight-1",
					PetID:         "pet-1",
					VolunteerID:   "vol-10",
					VolunteerName: "Sarah",
					Latitude:      47.61,
					Longitude:     -122.31,
					Notes:         "Saw collar",
					LamportClock:  2,
					Timestamp:     now,
				},
			},
			Evacuations: []domain.MeshEvacuationDelta{
				{
					IntakeID:     "intake-1",
					PetID:        "pet-1",
					FacilityID:   "shelter-1",
					Status:       "INTAKE_COMPLETE",
					LamportClock: 1,
					Timestamp:    now,
				},
			},
			SOSAlerts: []domain.MeshSOSAlert{
				{
					AlertID:       "sos-1",
					VolunteerID:   "vol-10",
					VolunteerName: "Sarah",
					Latitude:      47.6,
					Longitude:     -122.3,
					Message:       "Assistance required",
					Timestamp:     now,
				},
			},
		}

		resp := domain.MeshUplinkSyncResponse{
			Success:               true,
			ReconciledSectors:     1,
			ReconciledBreadcrumbs: 1,
			ReconciledSightings:   1,
			ReconciledEvacuations: 1,
			ServerLatestDeltas:    &batch,
			SyncedAt:              now,
		}

		bytes, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal MeshUplinkSyncResponse failed: %v", err)
		}

		var decoded domain.MeshUplinkSyncResponse
		if err := json.Unmarshal(bytes, &decoded); err != nil {
			t.Fatalf("unmarshal MeshUplinkSyncResponse failed: %v", err)
		}

		if !decoded.Success || decoded.ReconciledSectors != 1 || decoded.ServerLatestDeltas == nil {
			t.Fatalf("mismatch in decoded response: %+v", decoded)
		}
	})
}
