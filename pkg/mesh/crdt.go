package mesh

import (
	"fmt"
	"sort"
	"sync"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// CRDTEngine encapsulates monotonic Conflict-Free Replicated Data Type (CRDT) state
// for wilderness search and rescue operations across air-gapped partitions.
type CRDTEngine struct {
	mu            sync.RWMutex
	nodeID        string
	searchPartyID string
	lamportClock  uint64

	sectors     map[string]domain.MeshSectorDelta
	breadcrumbs map[string]domain.MeshBreadcrumbDelta // Key: "volunteerID:seq"
	sightings   map[string]domain.MeshSightingDelta   // Key: sightingID
	evacuations map[string]domain.MeshEvacuationDelta // Key: intakeID
	sosAlerts   map[string]domain.MeshSOSAlert        // Key: alertID
}

// NewCRDTEngine constructs an initialized CRDTEngine for the given peer nodeID.
func NewCRDTEngine(nodeID string) *CRDTEngine {
	return &CRDTEngine{
		nodeID:      nodeID,
		sectors:     make(map[string]domain.MeshSectorDelta),
		breadcrumbs: make(map[string]domain.MeshBreadcrumbDelta),
		sightings:   make(map[string]domain.MeshSightingDelta),
		evacuations: make(map[string]domain.MeshEvacuationDelta),
		sosAlerts:   make(map[string]domain.MeshSOSAlert),
	}
}

// NodeID returns the identifier of this local mesh engine.
func (e *CRDTEngine) NodeID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.nodeID
}

// SearchPartyID returns the search party ID bound to this engine.
func (e *CRDTEngine) SearchPartyID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.searchPartyID
}

// SetSearchPartyID updates the search party ID bound to this engine.
func (e *CRDTEngine) SetSearchPartyID(partyID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.searchPartyID = partyID
}

// LamportClock returns the current logical clock of this engine.
func (e *CRDTEngine) LamportClock() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lamportClock
}

// TickClock increments the local logical clock and returns the new value.
func (e *CRDTEngine) TickClock() uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lamportClock++
	return e.lamportClock
}

// MergeSector applies an incoming sector delta using strict monotonic rank progression
// (UNCLAIMED < CLAIMED < SEARCHING < CLEARED), tie-breaking equal ranks via Lamport clock
// and lexicographical NodeID.
func (e *CRDTEngine) MergeSector(delta domain.MeshSectorDelta) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mergeSectorLocked(delta)
}

func (e *CRDTEngine) mergeSectorLocked(delta domain.MeshSectorDelta) (bool, error) {
	if delta.SectorID == "" {
		return false, fmt.Errorf("sector ID cannot be empty")
	}

	// Normalize rank and state
	if delta.Rank == domain.SectorRankUnclaimed && delta.State != "" && delta.State != domain.SectorStateUnclaimed {
		delta.Rank = domain.RankFromState(delta.State)
	}
	if delta.State == "" {
		delta.State = delta.Rank.State()
	}

	// Advance local Lamport clock if incoming clock is ahead
	if delta.LamportClock > e.lamportClock {
		e.lamportClock = delta.LamportClock
	}

	existing, exists := e.sectors[delta.SectorID]
	if !exists {
		e.sectors[delta.SectorID] = delta
		return true, nil
	}

	// 1. Monotonic Rank Progression: higher rank always wins regardless of Lamport clock
	if delta.Rank > existing.Rank {
		e.sectors[delta.SectorID] = delta
		return true, nil
	}
	if delta.Rank < existing.Rank {
		return false, nil // Stale lower-rank update discarded
	}

	// 2. Same Rank: Lamport Clock comparison
	if delta.LamportClock > existing.LamportClock {
		e.sectors[delta.SectorID] = delta
		return true, nil
	}
	if delta.LamportClock < existing.LamportClock {
		return false, nil // Stale clock discarded
	}

	// 3. Same Rank, Same Clock: Lexicographical NodeID tie-breaker
	uNode := delta.NodeID
	if uNode == "" {
		uNode = delta.ClaimedByVolunteerID
	}
	sNode := existing.NodeID
	if sNode == "" {
		sNode = existing.ClaimedByVolunteerID
	}

	if uNode > sNode {
		e.sectors[delta.SectorID] = delta
		return true, nil
	}
	if uNode < sNode {
		return false, nil
	}

	// 4. Equal NodeID: Newer physical timestamp wins
	if delta.Timestamp.After(existing.Timestamp) {
		e.sectors[delta.SectorID] = delta
		return true, nil
	}

	return false, nil
}

// MergeBreadcrumb records an append-only GPS breadcrumb point, deduplicating by
// (volunteerId, seq) and retaining the most recent timestamp in case of duplicates.
func (e *CRDTEngine) MergeBreadcrumb(delta domain.MeshBreadcrumbDelta) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mergeBreadcrumbLocked(delta)
}

func (e *CRDTEngine) mergeBreadcrumbLocked(delta domain.MeshBreadcrumbDelta) (bool, error) {
	if delta.VolunteerID == "" {
		return false, fmt.Errorf("volunteer ID cannot be empty")
	}

	key := fmt.Sprintf("%s:%d", delta.VolunteerID, delta.Seq)
	existing, exists := e.breadcrumbs[key]
	if !exists {
		e.breadcrumbs[key] = delta
		return true, nil
	}

	// If duplicate sequence received, retain the point with the more recent timestamp
	if delta.Timestamp.After(existing.Timestamp) {
		e.breadcrumbs[key] = delta
		return true, nil
	}

	return false, nil
}

// MergeSighting records a field sighting using Add-Wins semantics and Last-Write-Wins (LWW)
// Lamport clocks for updates to notes or attachments.
func (e *CRDTEngine) MergeSighting(delta domain.MeshSightingDelta) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mergeSightingLocked(delta)
}

func (e *CRDTEngine) mergeSightingLocked(delta domain.MeshSightingDelta) (bool, error) {
	if delta.SightingID == "" {
		return false, fmt.Errorf("sighting ID cannot be empty")
	}

	if delta.LamportClock > e.lamportClock {
		e.lamportClock = delta.LamportClock
	}

	existing, exists := e.sightings[delta.SightingID]
	if !exists {
		e.sightings[delta.SightingID] = delta
		return true, nil
	}

	// LWW by Lamport clock
	if delta.LamportClock > existing.LamportClock {
		e.sightings[delta.SightingID] = delta
		return true, nil
	}
	if delta.LamportClock < existing.LamportClock {
		return false, nil
	}

	// Equal clock: timestamp tie-breaker
	if delta.Timestamp.After(existing.Timestamp) {
		e.sightings[delta.SightingID] = delta
		return true, nil
	}

	return false, nil
}

// MergeEvacuation applies an evacuation manifest delta using LWW Lamport clocks.
func (e *CRDTEngine) MergeEvacuation(delta domain.MeshEvacuationDelta) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mergeEvacuationLocked(delta)
}

func (e *CRDTEngine) mergeEvacuationLocked(delta domain.MeshEvacuationDelta) (bool, error) {
	if delta.IntakeID == "" {
		return false, fmt.Errorf("intake ID cannot be empty")
	}

	if delta.LamportClock > e.lamportClock {
		e.lamportClock = delta.LamportClock
	}

	existing, exists := e.evacuations[delta.IntakeID]
	if !exists {
		e.evacuations[delta.IntakeID] = delta
		return true, nil
	}

	if delta.LamportClock > existing.LamportClock {
		e.evacuations[delta.IntakeID] = delta
		return true, nil
	}
	if delta.LamportClock < existing.LamportClock {
		return false, nil
	}

	if delta.Timestamp.After(existing.Timestamp) {
		e.evacuations[delta.IntakeID] = delta
		return true, nil
	}

	return false, nil
}

// MergeSOSAlert records an emergency distress beacon, deduplicating by AlertID.
func (e *CRDTEngine) MergeSOSAlert(alert domain.MeshSOSAlert) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mergeSOSAlertLocked(alert)
}

func (e *CRDTEngine) mergeSOSAlertLocked(alert domain.MeshSOSAlert) (bool, error) {
	if alert.AlertID == "" {
		return false, fmt.Errorf("alert ID cannot be empty")
	}

	existing, exists := e.sosAlerts[alert.AlertID]
	if !exists {
		e.sosAlerts[alert.AlertID] = alert
		return true, nil
	}

	if alert.Timestamp.After(existing.Timestamp) {
		e.sosAlerts[alert.AlertID] = alert
		return true, nil
	}

	return false, nil
}

// ApplyBatch applies an incoming synchronization batch of deltas and returns the count of accepted mutations.
func (e *CRDTEngine) ApplyBatch(batch domain.MeshBatchDelta) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	appliedCount := 0
	for _, s := range batch.Sectors {
		if applied, err := e.mergeSectorLocked(s); err == nil && applied {
			appliedCount++
		}
	}
	for _, b := range batch.Breadcrumbs {
		if applied, err := e.mergeBreadcrumbLocked(b); err == nil && applied {
			appliedCount++
		}
	}
	for _, s := range batch.Sightings {
		if applied, err := e.mergeSightingLocked(s); err == nil && applied {
			appliedCount++
		}
	}
	for _, ev := range batch.Evacuations {
		if applied, err := e.mergeEvacuationLocked(ev); err == nil && applied {
			appliedCount++
		}
	}
	for _, a := range batch.SOSAlerts {
		if applied, err := e.mergeSOSAlertLocked(a); err == nil && applied {
			appliedCount++
		}
	}

	return appliedCount
}

// GenerateDigest creates a MeshBatchDelta containing the complete current CRDT state,
// sorted deterministically for anti-entropy exchange and verification.
func (e *CRDTEngine) GenerateDigest() domain.MeshBatchDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sectors := make([]domain.MeshSectorDelta, 0, len(e.sectors))
	for _, s := range e.sectors {
		sectors = append(sectors, s)
	}
	sort.Slice(sectors, func(i, j int) bool {
		return sectors[i].SectorID < sectors[j].SectorID
	})

	breadcrumbs := make([]domain.MeshBreadcrumbDelta, 0, len(e.breadcrumbs))
	for _, b := range e.breadcrumbs {
		breadcrumbs = append(breadcrumbs, b)
	}
	sort.Slice(breadcrumbs, func(i, j int) bool {
		if breadcrumbs[i].VolunteerID != breadcrumbs[j].VolunteerID {
			return breadcrumbs[i].VolunteerID < breadcrumbs[j].VolunteerID
		}
		return breadcrumbs[i].Seq < breadcrumbs[j].Seq
	})

	sightings := make([]domain.MeshSightingDelta, 0, len(e.sightings))
	for _, s := range e.sightings {
		sightings = append(sightings, s)
	}
	sort.Slice(sightings, func(i, j int) bool {
		return sightings[i].SightingID < sightings[j].SightingID
	})

	evacuations := make([]domain.MeshEvacuationDelta, 0, len(e.evacuations))
	for _, ev := range e.evacuations {
		evacuations = append(evacuations, ev)
	}
	sort.Slice(evacuations, func(i, j int) bool {
		return evacuations[i].IntakeID < evacuations[j].IntakeID
	})

	alerts := make([]domain.MeshSOSAlert, 0, len(e.sosAlerts))
	for _, a := range e.sosAlerts {
		alerts = append(alerts, a)
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].AlertID < alerts[j].AlertID
	})

	return domain.MeshBatchDelta{
		SearchPartyID: e.searchPartyID,
		SenderNodeID:  e.nodeID,
		Sectors:       sectors,
		Breadcrumbs:   breadcrumbs,
		Sightings:     sightings,
		Evacuations:   evacuations,
		SOSAlerts:     alerts,
	}
}

// GetSector looks up a single sector state by ID.
func (e *CRDTEngine) GetSector(sectorID string) (domain.MeshSectorDelta, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sec, ok := e.sectors[sectorID]
	return sec, ok
}

// GetSectors returns all sectors in deterministic order by SectorID.
func (e *CRDTEngine) GetSectors() []domain.MeshSectorDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sectors := make([]domain.MeshSectorDelta, 0, len(e.sectors))
	for _, s := range e.sectors {
		sectors = append(sectors, s)
	}
	sort.Slice(sectors, func(i, j int) bool {
		return sectors[i].SectorID < sectors[j].SectorID
	})
	return sectors
}

// GetBreadcrumbs returns all recorded breadcrumbs in deterministic order by VolunteerID and Seq.
func (e *CRDTEngine) GetBreadcrumbs() []domain.MeshBreadcrumbDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	crumbs := make([]domain.MeshBreadcrumbDelta, 0, len(e.breadcrumbs))
	for _, b := range e.breadcrumbs {
		crumbs = append(crumbs, b)
	}
	sort.Slice(crumbs, func(i, j int) bool {
		if crumbs[i].VolunteerID != crumbs[j].VolunteerID {
			return crumbs[i].VolunteerID < crumbs[j].VolunteerID
		}
		return crumbs[i].Seq < crumbs[j].Seq
	})
	return crumbs
}

// GetSightings returns all recorded sightings in deterministic order by SightingID.
func (e *CRDTEngine) GetSightings() []domain.MeshSightingDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sightings := make([]domain.MeshSightingDelta, 0, len(e.sightings))
	for _, s := range e.sightings {
		sightings = append(sightings, s)
	}
	sort.Slice(sightings, func(i, j int) bool {
		return sightings[i].SightingID < sightings[j].SightingID
	})
	return sightings
}

// GetEvacuations returns all evacuation records in deterministic order by IntakeID.
func (e *CRDTEngine) GetEvacuations() []domain.MeshEvacuationDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	evacs := make([]domain.MeshEvacuationDelta, 0, len(e.evacuations))
	for _, ev := range e.evacuations {
		evacs = append(evacs, ev)
	}
	sort.Slice(evacs, func(i, j int) bool {
		return evacs[i].IntakeID < evacs[j].IntakeID
	})
	return evacs
}

// GetSOSAlerts returns all SOS alerts in deterministic order by AlertID.
func (e *CRDTEngine) GetSOSAlerts() []domain.MeshSOSAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	alerts := make([]domain.MeshSOSAlert, 0, len(e.sosAlerts))
	for _, a := range e.sosAlerts {
		alerts = append(alerts, a)
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].AlertID < alerts[j].AlertID
	})
	return alerts
}
