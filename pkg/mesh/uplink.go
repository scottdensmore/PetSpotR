package mesh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Collection names for mesh-specific persistent metadata.
const (
	MeshSectorDeltasCollection = "mesh_sector_deltas"
	MeshBreadcrumbsCollection  = "mesh_breadcrumbs"
	MeshSightingsCollection    = "mesh_sightings"
	MeshEvacuationsCollection  = "mesh_evacuations"
	MeshSOSAlertsCollection    = "mesh_sos_alerts"
)

// ReunionBroadcaster defines an interface for emitting real-time SSE stream events.
type ReunionBroadcaster interface {
	Broadcast(event domain.ReunionStreamEvent)
}

type uplinkConfig struct {
	broadcaster ReunionBroadcaster
}

// UplinkOption configures optional behavior for ReconcileUplinkBatch.
type UplinkOption func(*uplinkConfig)

// WithBroadcaster injects a ReunionBroadcaster into the uplink reconciliation process.
func WithBroadcaster(b ReunionBroadcaster) UplinkOption {
	return func(c *uplinkConfig) {
		c.broadcaster = b
	}
}

// ReconcileUplinkBatch transactionally reconciles a field-synchronized MeshBatchDelta
// into the cloud StateStore, upholding monotonic state progression for sectors,
// persisting GPS breadcrumbs into search party trails, logging field sightings,
// updating disaster evacuation rosters, and returning any server-side latest deltas.
func ReconcileUplinkBatch(
	ctx context.Context,
	st store.StateStore,
	batch domain.MeshBatchDelta,
	opts ...UplinkOption,
) (*domain.MeshUplinkSyncResponse, error) {
	if st == nil {
		return nil, errors.New("uplink: state store is required")
	}

	var cfg uplinkConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	reconciledSectors := 0
	reconciledBreadcrumbs := 0
	reconciledSightings := 0
	reconciledEvacuations := 0

	serverDeltas := &domain.MeshBatchDelta{
		SearchPartyID: batch.SearchPartyID,
		SenderNodeID:  "server-uplink",
	}
	hasServerDeltas := false

	// 1. Resolve and Reconcile Search Party Sectors
	partyID := strings.TrimSpace(batch.SearchPartyID)
	if partyID == "" && len(batch.Sectors) > 0 && batch.Sectors[0].PetID != "" {
		partyID = "party-" + batch.Sectors[0].PetID
	}

	var party searchparty.SearchParty
	var partyFound bool

	if partyID != "" {
		partyBytes, err := st.GetState(ctx, store.SearchPartiesCollection, partyID)
		if err == nil {
			if json.Unmarshal(partyBytes, &party) == nil {
				partyFound = true
			}
		} else if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			// Fallback: search by lostPetID across parties
			if rawParties, listErr := st.ListState(ctx, store.SearchPartiesCollection); listErr == nil {
				for pKey, pBytes := range rawParties {
					var p searchparty.SearchParty
					if json.Unmarshal(pBytes, &p) == nil {
						if p.PartyID == partyID || (len(batch.Sectors) > 0 && p.LostPetID == batch.Sectors[0].PetID) {
							party = p
							partyID = pKey
							partyFound = true
							break
						}
					}
				}
			}
		}
	}

	if partyFound && len(batch.Sectors) > 0 {
		partyUpdated := false

		for _, incoming := range batch.Sectors {
			if incoming.SectorID == "" {
				continue
			}

			// Normalize incoming rank and state
			if incoming.Rank == domain.SectorRankUnclaimed && incoming.State != "" && incoming.State != domain.SectorStateUnclaimed {
				incoming.Rank = domain.RankFromState(incoming.State)
			}
			if incoming.State == "" {
				incoming.State = incoming.Rank.State()
			}

			// Find matching sector in party
			secIdx := -1
			for i, sec := range party.Sectors {
				if sec.SectorID == incoming.SectorID {
					secIdx = i
					break
				}
			}

			if secIdx == -1 {
				// Sector not yet registered in party; record delta directly
				if deltaBytes, mErr := json.Marshal(incoming); mErr == nil {
					_ = st.SaveState(ctx, MeshSectorDeltasCollection, incoming.SectorID, deltaBytes)
				}
				reconciledSectors++
				continue
			}

			currentSector := party.Sectors[secIdx]

			// Determine server rank and Lamport clock from stored delta or sector status
			storedDelta := domain.MeshSectorDelta{
				PetID:        party.LostPetID,
				SectorID:     currentSector.SectorID,
				LamportClock: 0,
				Timestamp:    party.CreatedAt,
			}

			if sBytes, sErr := st.GetState(ctx, MeshSectorDeltasCollection, currentSector.SectorID); sErr == nil {
				_ = json.Unmarshal(sBytes, &storedDelta)
			} else {
				switch currentSector.Status {
				case searchparty.SectorStatusCleared:
					storedDelta.Rank = domain.SectorRankCleared
					storedDelta.State = domain.SectorStateCleared
				case searchparty.SectorStatusActiveSearch, searchparty.SectorStatusSightingReported:
					storedDelta.Rank = domain.SectorRankSearching
					storedDelta.State = domain.SectorStateSearching
				default:
					storedDelta.Rank = domain.SectorRankUnclaimed
					storedDelta.State = domain.SectorStateUnclaimed
				}
			}

			// Apply Monotonic Conflict Resolution Rules
			incomingWins := false

			// Rule 1: Monotonic rank progression
			if incoming.Rank > storedDelta.Rank {
				incomingWins = true
			} else if incoming.Rank < storedDelta.Rank {
				// Server state is strictly ahead; discard incoming and return server state
				serverDeltas.Sectors = append(serverDeltas.Sectors, storedDelta)
				hasServerDeltas = true
			} else {
				// Rule 2: Same rank - Lamport Clock tie-breaker
				if incoming.LamportClock > storedDelta.LamportClock {
					incomingWins = true
				} else if incoming.LamportClock < storedDelta.LamportClock {
					serverDeltas.Sectors = append(serverDeltas.Sectors, storedDelta)
					hasServerDeltas = true
				} else {
					// Rule 3: Lexicographical NodeID tie-breaker
					uNode := incoming.NodeID
					if uNode == "" {
						uNode = incoming.ClaimedByVolunteerID
					}
					sNode := storedDelta.NodeID
					if sNode == "" {
						sNode = storedDelta.ClaimedByVolunteerID
					}

					if uNode > sNode {
						incomingWins = true
					} else if uNode < sNode {
						serverDeltas.Sectors = append(serverDeltas.Sectors, storedDelta)
						hasServerDeltas = true
					} else {
						// Rule 4: Timestamp tie-breaker
						if incoming.Timestamp.After(storedDelta.Timestamp) {
							incomingWins = true
						} else if incoming.Timestamp.Before(storedDelta.Timestamp) {
							serverDeltas.Sectors = append(serverDeltas.Sectors, storedDelta)
							hasServerDeltas = true
						}
						// If incoming.Timestamp.Equal(storedDelta.Timestamp), it is an idempotent replay;
						// do not trigger incomingWins or a server conflict.
					}
				}
			}

			if incomingWins {
				reconciledSectors++
				partyUpdated = true

				// Update sector status
				var newStatus searchparty.SectorStatus
				switch incoming.Rank {
				case domain.SectorRankCleared:
					newStatus = searchparty.SectorStatusCleared
				case domain.SectorRankSearching, domain.SectorRankClaimed:
					newStatus = searchparty.SectorStatusActiveSearch
				default:
					newStatus = searchparty.SectorStatusUnassigned
				}
				party.Sectors[secIdx].Status = newStatus

				// Update or add assignment
				asgnIdx := -1
				for aIdx, asgn := range party.ActiveAssignments {
					if asgn.SectorID == incoming.SectorID {
						asgnIdx = aIdx
						break
					}
				}

				alias := incoming.ClaimedByName
				if alias == "" {
					alias = incoming.ClaimedByVolunteerID
				}
				if alias == "" {
					alias = fmt.Sprintf("Volunteer %s", incoming.NodeID)
				}

				now := time.Now().UTC()
				if asgnIdx >= 0 {
					party.ActiveAssignments[asgnIdx].Status = newStatus
					party.ActiveAssignments[asgnIdx].UpdatedAt = now
					if alias != "" {
						party.ActiveAssignments[asgnIdx].VolunteerAlias = alias
					}
				} else if newStatus != searchparty.SectorStatusUnassigned {
					party.ActiveAssignments = append(party.ActiveAssignments, searchparty.SectorAssignment{
						AssignmentID:   fmt.Sprintf("asgn-mesh-%s-%d", incoming.SectorID, now.UnixNano()),
						SectorID:       incoming.SectorID,
						VolunteerAlias: alias,
						ClaimedAt:      incoming.Timestamp,
						UpdatedAt:      now,
						Status:         newStatus,
					})
				}

				// Persist winning delta
				if dBytes, err := json.Marshal(incoming); err == nil {
					_ = st.SaveState(ctx, MeshSectorDeltasCollection, incoming.SectorID, dBytes)
				}

				// Broadcast real-time SSE event if active
				if cfg.broadcaster != nil {
					cfg.broadcaster.Broadcast(domain.ReunionStreamEvent{
						EventID:   fmt.Sprintf("evt_mesh_sector_%s_%d", incoming.SectorID, now.UnixNano()),
						Type:      domain.ReunionEventSearchPartyUpdated,
						MatchID:   party.LostPetID,
						Timestamp: now,
						Payload: domain.SearchPartyEventPayload{
							Type:               "search_party_updated",
							PartyID:            party.PartyID,
							SectorID:           incoming.SectorID,
							Status:             string(newStatus),
							CoveragePercentage: party.CoveragePercentage,
						},
					})
				}
			}
		}

		if partyUpdated {
			party.CoveragePercentage = party.CalculateCoverage()
			party.ActiveVolunteersCount = countActiveVolunteers(party.ActiveAssignments)
			if pBytes, err := json.Marshal(party); err == nil {
				_ = st.SaveState(ctx, store.SearchPartiesCollection, party.PartyID, pBytes)
			}
		}
	}

	// 2. Reconcile GPS Breadcrumbs into Search Party Trails
	for _, b := range batch.Breadcrumbs {
		if b.VolunteerID == "" {
			continue
		}

		bcKey := fmt.Sprintf("%s:%d", b.VolunteerID, b.Seq)
		_, existsErr := st.GetState(ctx, MeshBreadcrumbsCollection, bcKey)
		if existsErr == nil {
			// Duplicate sequence already stored
			continue
		}

		// Persist breadcrumb point
		if bBytes, err := json.Marshal(b); err == nil {
			_ = st.SaveState(ctx, MeshBreadcrumbsCollection, bcKey, bBytes)
		}
		reconciledBreadcrumbs++

		pt := searchparty.BreadcrumbPoint{
			Latitude:       b.Latitude,
			Longitude:      b.Longitude,
			AccuracyMeters: b.Accuracy,
			Timestamp:      b.Timestamp,
		}

		// Locate or create volunteer trail in store.BreadcrumbsCollection
		trailKey := fmt.Sprintf("trail-%s-%s", partyID, b.VolunteerID)
		var trail searchparty.VolunteerBreadcrumbTrail

		tBytes, err := st.GetState(ctx, store.BreadcrumbsCollection, trailKey)
		if err == nil {
			_ = json.Unmarshal(tBytes, &trail)
		}

		now := time.Now().UTC()
		if trail.TrailID == "" {
			alias := b.VolunteerName
			if alias == "" {
				alias = fmt.Sprintf("Volunteer %s", b.VolunteerID)
			}
			trail = searchparty.VolunteerBreadcrumbTrail{
				TrailID:        trailKey,
				SearchPartyID:  partyID,
				SectorID:       "mesh-sector",
				VolunteerAlias: alias,
				Points:         []searchparty.BreadcrumbPoint{pt},
				CreatedAt:      now,
				UpdatedAt:      now,
			}
		} else {
			trail.AppendPoints(pt)
			trail.UpdatedAt = now
		}

		if updatedTrailBytes, err := json.Marshal(trail); err == nil {
			_ = st.SaveState(ctx, store.BreadcrumbsCollection, trail.TrailID, updatedTrailBytes)
		}

		if cfg.broadcaster != nil {
			matchID := b.PetID
			if matchID == "" {
				matchID = party.LostPetID
			}
			if matchID == "" {
				matchID = partyID
			}
			cfg.broadcaster.Broadcast(domain.ReunionStreamEvent{
				EventID:   fmt.Sprintf("evt_mesh_breadcrumb_%s_%d", trail.TrailID, now.UnixNano()),
				Type:      domain.ReunionEventBreadcrumbUpdated,
				MatchID:   matchID,
				Timestamp: now,
				Payload: map[string]any{
					"type":           "breadcrumb_updated",
					"partyId":        partyID,
					"volunteerAlias": trail.VolunteerAlias,
					"trail":          trail,
				},
			})
		}
	}

	// 3. Reconcile Field Sightings
	for _, s := range batch.Sightings {
		if s.SightingID == "" {
			continue
		}

		existingBytes, sErr := st.GetState(ctx, MeshSightingsCollection, s.SightingID)
		if sErr == nil {
			var existing domain.MeshSightingDelta
			if json.Unmarshal(existingBytes, &existing) == nil && s.LamportClock <= existing.LamportClock {
				continue // Stale or duplicate sighting
			}
		}

		// Store mesh sighting delta
		if sBytes, err := json.Marshal(s); err == nil {
			_ = st.SaveState(ctx, MeshSightingsCollection, s.SightingID, sBytes)
		}
		reconciledSightings++

		// Also persist as PetSightingRecord in store.SightingsCollection
		sRec := domain.PetSightingRecord{
			SightingID:          s.SightingID,
			LostPetID:           s.PetID,
			ReportedAt:          s.Timestamp,
			SightedAt:           s.Timestamp,
			LocationDescription: fmt.Sprintf("Field sighting by %s", s.VolunteerName),
			Coordinates: &domain.LocationPoint{
				Latitude:  s.Latitude,
				Longitude: s.Longitude,
			},
			Notes:    s.Notes,
			ImageURL: s.PhotoThumbBase64,
			Status:   domain.SightingStatusActive,
		}

		if sRecBytes, err := json.Marshal(sRec); err == nil {
			_ = st.SaveState(ctx, store.SightingsCollection, s.SightingID, sRecBytes)
		}

		if cfg.broadcaster != nil {
			now := time.Now().UTC()
			cfg.broadcaster.Broadcast(domain.ReunionStreamEvent{
				EventID:   fmt.Sprintf("evt_mesh_sighting_%s_%d", s.SightingID, now.UnixNano()),
				Type:      domain.ReunionEventSighting,
				MatchID:   s.PetID,
				Timestamp: now,
				Payload: domain.SightingEventPayload{
					Type:                "sighting",
					PetID:               s.PetID,
					SightingID:          s.SightingID,
					SightedAt:           s.Timestamp,
					LocationDescription: sRec.LocationDescription,
					Coordinates:         sRec.Coordinates,
				},
			})
		}
	}

	// 4. Reconcile Evacuation Manifest Deltas
	for _, ev := range batch.Evacuations {
		if ev.IntakeID == "" {
			continue
		}

		existingBytes, evErr := st.GetState(ctx, MeshEvacuationsCollection, ev.IntakeID)
		if evErr == nil {
			var existing domain.MeshEvacuationDelta
			if json.Unmarshal(existingBytes, &existing) == nil && ev.LamportClock <= existing.LamportClock {
				continue
			}
		}

		if evBytes, err := json.Marshal(ev); err == nil {
			_ = st.SaveState(ctx, MeshEvacuationsCollection, ev.IntakeID, evBytes)
			_ = st.SaveState(ctx, store.CrisisIntakesCollection, ev.IntakeID, evBytes)
		}
		reconciledEvacuations++
	}

	// 5. Reconcile SOS Distress Alerts
	for _, alert := range batch.SOSAlerts {
		if alert.AlertID == "" {
			continue
		}

		if aBytes, err := json.Marshal(alert); err == nil {
			_ = st.SaveState(ctx, MeshSOSAlertsCollection, alert.AlertID, aBytes)
		}

		if cfg.broadcaster != nil {
			now := time.Now().UTC()
			sosMatchID := party.LostPetID
			if sosMatchID == "" {
				sosMatchID = partyID
			}
			cfg.broadcaster.Broadcast(domain.ReunionStreamEvent{
				EventID:   fmt.Sprintf("evt_mesh_sos_%s_%d", alert.AlertID, now.UnixNano()),
				Type:      "mesh_sos_alert",
				MatchID:   sosMatchID,
				Timestamp: now,
				Payload: map[string]any{
					"type":      "sos_alert",
					"alertId":   alert.AlertID,
					"volunteer": alert.VolunteerName,
					"latitude":  alert.Latitude,
					"longitude": alert.Longitude,
					"message":   alert.Message,
				},
			})
		}
	}

	resp := &domain.MeshUplinkSyncResponse{
		Success:               true,
		ReconciledSectors:     reconciledSectors,
		ReconciledBreadcrumbs: reconciledBreadcrumbs,
		ReconciledSightings:   reconciledSightings,
		ReconciledEvacuations: reconciledEvacuations,
		SyncedAt:              time.Now().UTC(),
	}

	if hasServerDeltas {
		resp.ServerLatestDeltas = serverDeltas
	}

	return resp, nil
}

func countActiveVolunteers(assignments []searchparty.SectorAssignment) int {
	active := make(map[string]struct{})
	for _, a := range assignments {
		if (a.Status == searchparty.SectorStatusActiveSearch || a.Status == searchparty.SectorStatusSightingReported) && a.VolunteerAlias != "" {
			active[a.VolunteerAlias] = struct{}{}
		}
	}
	return len(active)
}
