package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvacuationDomainSerialization(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 21, 14, 0, 0, 0, time.UTC)
	later := now.Add(2 * time.Hour)

	hub := domain.EvacuationHub{
		HubID:            "hub-sea-1",
		Name:             "Seattle Center Exhibition Hall",
		Type:             domain.HubTypePopUpCrisisCenter,
		Status:           domain.HubStatusActive,
		Address:          "301 Mercer St, Seattle, WA 98109",
		Coordinates:      domain.LocationPoint{Latitude: 47.6242, Longitude: -122.3518},
		TotalCapacity:    250,
		CurrentOccupancy: 120,
		DogCapacity:      150,
		DogOccupancy:     80,
		CatCapacity:      100,
		CatOccupancy:     40,
		ContactName:      "Sarah Jenkins",
		ContactPhone:     "(206) 555-0199",
		ContactEmail:     "sjenkins@seattleemergency.gov",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(hub)
	if err != nil {
		t.Fatalf("failed to marshal EvacuationHub: %v", err)
	}

	var roundtripHub domain.EvacuationHub
	if err := json.Unmarshal(data, &roundtripHub); err != nil {
		t.Fatalf("failed to unmarshal EvacuationHub: %v", err)
	}

	if roundtripHub.HubID != hub.HubID || roundtripHub.Type != domain.HubTypePopUpCrisisCenter {
		t.Errorf("EvacuationHub roundtrip mismatch: got %+v, want %+v", roundtripHub, hub)
	}

	manifest := domain.TransferManifest{
		TransferID:       "xfer-001",
		OriginHubID:      hub.HubID,
		OriginHubName:    hub.Name,
		DestHubID:        "hub-bel-2",
		DestHubName:      "Bellevue Fairgrounds",
		Status:           domain.TransferStatusInTransit,
		AnimalIDs:        []string{"found-1", "found-2"},
		TotalAnimals:     2,
		TransporterName:  "Convoy Team Alpha",
		TransporterPhone: "(206) 555-9012",
		VehicleNotes:     "Van 4 with 10 climate-controlled crates",
		DepartureTime:    &now,
		ArrivalTime:      &later,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal TransferManifest: %v", err)
	}

	var roundtripManifest domain.TransferManifest
	if err := json.Unmarshal(manifestData, &roundtripManifest); err != nil {
		t.Fatalf("failed to unmarshal TransferManifest: %v", err)
	}

	if roundtripManifest.TransferID != "xfer-001" || roundtripManifest.Status != domain.TransferStatusInTransit {
		t.Errorf("TransferManifest roundtrip mismatch: got %+v, want %+v", roundtripManifest, manifest)
	}

	crisisItem := domain.CrisisReunificationItem{
		MatchID:         "match-disaster-1",
		FoundPetID:      "found-1",
		LostPetID:       "lost-99",
		PetName:         "Bella",
		Species:         "Dog",
		Breed:           "Golden Retriever",
		CurrentHubID:    hub.HubID,
		CurrentHubName:  hub.Name,
		OwnerName:       "Mark Taylor",
		OwnerContact:    "(206) 555-8833",
		MicrochipID:     "985141000123456",
		Priority:        domain.CrisisPriorityMicrochipMatch,
		SimilarityScore: 1.0,
		Status:          "PENDING",
		IdentifiedAt:    now,
	}

	itemData, err := json.Marshal(crisisItem)
	if err != nil {
		t.Fatalf("failed to marshal CrisisReunificationItem: %v", err)
	}

	var roundtripItem domain.CrisisReunificationItem
	if err := json.Unmarshal(itemData, &roundtripItem); err != nil {
		t.Fatalf("failed to unmarshal CrisisReunificationItem: %v", err)
	}

	if roundtripItem.MatchID != "match-disaster-1" || roundtripItem.Priority != domain.CrisisPriorityMicrochipMatch {
		t.Errorf("CrisisReunificationItem roundtrip mismatch: got %+v, want %+v", roundtripItem, crisisItem)
	}
}
