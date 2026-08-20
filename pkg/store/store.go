package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

type outboxIndexRecord struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Sentinel errors for StateStore operations.
var (
	ErrStoreNotFound = errors.New("store: state store not found")
	ErrNotFound      = errors.New("store: key not found")
	ErrConflict      = errors.New("store: conflicting aggregate state")
	ErrRoleDenied    = errors.New("store: active role assignment required")
)

// StateWrite describes one state document written as part of an atomic unit.
type StateWrite struct {
	StoreName string
	Key       string
	Data      []byte
}

// StateUpdater computes the next value from an isolated copy of current state.
// Managed stores may invoke it more than once when a transaction is retried.
type StateUpdater func(current []byte) (next []byte, err error)

// MatchStateUpdater computes one atomic replacement for the public match and
// its private participant record. Managed stores may invoke it more than once
// when a transaction is retried.
type MatchStateUpdater func(match, participants []byte) (nextMatch, nextParticipants []byte, err error)

// RoleAuthorizedMatchStateUpdater receives the exact active assignment read in
// the same transaction as the match update. Managed stores may invoke it more
// than once when a transaction is retried, so it must remain side-effect-free.
type RoleAuthorizedMatchStateUpdater func(
	assignment domain.RoleAssignment,
	match, participants []byte,
) (nextMatch, nextParticipants []byte, err error)

// MatchStateStore atomically updates the public and private documents that
// comprise one authorized match operation.
type MatchStateStore interface {
	UpdateMatchAndParticipants(ctx context.Context, matchID string, update MatchStateUpdater) error
}

// RoleAuthorizedMatchStateStore atomically requires one active role assignment
// and updates a match plus its private participant record. This prevents a
// grant revocation from racing a privileged mutation.
type RoleAuthorizedMatchStateStore interface {
	UpdateMatchAndParticipantsAsRole(
		ctx context.Context,
		principal domain.PrincipalRef,
		role domain.Role,
		scope domain.RoleScope,
		matchID string,
		update RoleAuthorizedMatchStateUpdater,
	) error
}

// MatchDecisionStore preserves the name introduced with bilateral decisions.
type MatchDecisionStore = MatchStateStore

// StateStore defines key-value state store persistence operations.
type StateStore interface {
	SaveState(ctx context.Context, storeName, key string, data []byte) error
	UpdateState(ctx context.Context, storeName, key string, update StateUpdater) error
	CreateStateAndOutbox(ctx context.Context, state, outbox StateWrite) (bool, error)
	CreateStatesAndOutbox(ctx context.Context, states []StateWrite, outbox StateWrite) (bool, error)
	GetState(ctx context.Context, storeName, key string) ([]byte, error)
	DeleteState(ctx context.Context, storeName, key string) error
	ListState(ctx context.Context, storeName string) (map[string][]byte, error)
	ListPendingOutbox(ctx context.Context, topic string, limit int) ([]string, error)
}

// DeliveryOperationStore provides transactional leases and fenced results for
// side effects performed by at-least-once event consumers.
type DeliveryOperationStore interface {
	ClaimDeliveryOperation(ctx context.Context, operation delivery.Operation, now time.Time, leaseDuration time.Duration) (delivery.Claim, error)
	CompleteDeliveryOperation(ctx context.Context, id string, attempt int, completedAt time.Time) error
	FailDeliveryOperation(ctx context.Context, id string, attempt int, failedAt time.Time, failure string) error
	GetDeliveryOperation(ctx context.Context, id string) (delivery.Operation, error)
}

// OutboxIndexBackfiller upgrades legacy Firestore outbox documents in bounded
// batches before indexed recovery. Memory stores do not need this migration.
type OutboxIndexBackfiller interface {
	BackfillOutboxIndexes(ctx context.Context, limit int) (migrated int, complete bool, err error)
}

// CreateStateAndOutbox atomically creates aggregate state and its outbox
// record. An exact retry is a successful no-op; a competing aggregate create
// returns ErrConflict.
func (m *MemoryStore) CreateStateAndOutbox(ctx context.Context, state, outbox StateWrite) (bool, error) {
	return m.CreateStatesAndOutbox(ctx, []StateWrite{state}, outbox)
}

// CreateStatesAndOutbox atomically creates one or more related state records
// and their outbox record. Exact retries compare every state record while
// preserving the first outbox value, which may already record publication.
func (m *MemoryStore) CreateStatesAndOutbox(ctx context.Context, states []StateWrite, outbox StateWrite) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validateAtomicWrites(states, outbox); err != nil {
		return false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, outboxExists := m.items[outbox.StoreName][outbox.Key]
	anyExists := outboxExists
	allExistAndMatch := outboxExists
	for _, state := range states {
		stateData, stateExists := m.items[state.StoreName][state.Key]
		anyExists = anyExists || stateExists
		allExistAndMatch = allExistAndMatch && stateExists && bytes.Equal(stateData, state.Data)
	}
	if anyExists {
		if allExistAndMatch {
			return false, nil
		}
		return false, fmt.Errorf("%w: %s/%s", ErrConflict, states[0].StoreName, states[0].Key)
	}

	for _, state := range states {
		if _, exists := m.items[state.StoreName]; !exists {
			m.items[state.StoreName] = make(map[string][]byte)
		}
		m.items[state.StoreName][state.Key] = bytes.Clone(state.Data)
	}
	if _, exists := m.items[outbox.StoreName]; !exists {
		m.items[outbox.StoreName] = make(map[string][]byte)
	}
	m.items[outbox.StoreName][outbox.Key] = bytes.Clone(outbox.Data)
	return true, nil
}

// MemoryStore implements StateStore in memory for testing and local dev.
type MemoryStore struct {
	mu              sync.RWMutex
	items           map[string]map[string][]byte
	roleAssignments map[string][]byte
	roleAudits      map[string]map[string][]byte
}

// NewMemoryStore constructs a thread-safe MemoryStore instance.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items:           make(map[string]map[string][]byte),
		roleAssignments: make(map[string][]byte),
		roleAudits:      make(map[string]map[string][]byte),
	}
}

// SaveState saves data under storeName and key.
func (m *MemoryStore) SaveState(ctx context.Context, storeName, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := rejectPrivateRoleCollection(storeName); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.items[storeName]; !exists {
		m.items[storeName] = make(map[string][]byte)
	}
	m.items[storeName][key] = bytes.Clone(data)
	return nil
}

// UpdateState atomically replaces one existing value using the caller's
// retry-safe update function.
func (m *MemoryStore) UpdateState(ctx context.Context, storeName, key string, update StateUpdater) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if update == nil {
		return errors.New("store: state updater is required")
	}
	if err := rejectPrivateRoleCollection(storeName); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	storeMap, exists := m.items[storeName]
	if !exists {
		return fmt.Errorf("%w: %s", ErrStoreNotFound, storeName)
	}
	current, exists := storeMap[key]
	if !exists {
		return fmt.Errorf("%w: %s in store %s", ErrNotFound, key, storeName)
	}
	next, err := update(bytes.Clone(current))
	if err != nil {
		return err
	}
	storeMap[key] = bytes.Clone(next)
	return nil
}

// UpdateMatchAndParticipants atomically replaces an existing public match and
// private participant record under one lock.
func (m *MemoryStore) UpdateMatchAndParticipants(
	ctx context.Context,
	matchID string,
	update MatchStateUpdater,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(matchID) == "" {
		return errors.New("store: match ID is required")
	}
	if update == nil {
		return errors.New("store: match state updater is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateMatchAndParticipantsLocked(matchID, update)
}

// UpdateMatchAndParticipantsAsRole authorizes and mutates under one lock.
func (m *MemoryStore) UpdateMatchAndParticipantsAsRole(
	ctx context.Context,
	principal domain.PrincipalRef,
	role domain.Role,
	scope domain.RoleScope,
	matchID string,
	update RoleAuthorizedMatchStateUpdater,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(matchID) == "" || update == nil {
		return errors.New("store: match ID and state updater are required")
	}
	assignmentID, err := domain.RoleAssignmentID(principal, role, scope)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	assignmentData, exists := m.roleAssignments[assignmentID]
	if !exists {
		return ErrRoleDenied
	}
	assignment, err := decodeRoleAssignment(assignmentData)
	if err != nil {
		return err
	}
	if assignment.AssignmentID != assignmentID || assignment.Status != domain.RoleAssignmentStatusActive {
		return ErrRoleDenied
	}
	return m.updateMatchAndParticipantsLocked(matchID, func(match, participants []byte) ([]byte, []byte, error) {
		return update(assignment, match, participants)
	})
}

func (m *MemoryStore) updateMatchAndParticipantsLocked(matchID string, update MatchStateUpdater) error {
	matches, ok := m.items[MatchesCollection]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStoreNotFound, MatchesCollection)
	}
	match, ok := matches[matchID]
	if !ok {
		return fmt.Errorf("%w: %s in store %s", ErrNotFound, matchID, MatchesCollection)
	}
	participantsByMatch, ok := m.items[MatchParticipantsCollection]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStoreNotFound, MatchParticipantsCollection)
	}
	participants, ok := participantsByMatch[matchID]
	if !ok {
		return fmt.Errorf("%w: %s in store %s", ErrNotFound, matchID, MatchParticipantsCollection)
	}
	nextMatch, nextParticipants, err := update(bytes.Clone(match), bytes.Clone(participants))
	if err != nil {
		return err
	}
	matches[matchID] = bytes.Clone(nextMatch)
	participantsByMatch[matchID] = bytes.Clone(nextParticipants)
	return nil
}

// GetState retrieves data for storeName and key.
func (m *MemoryStore) GetState(ctx context.Context, storeName, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := rejectPrivateRoleCollection(storeName); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	storeMap, exists := m.items[storeName]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrStoreNotFound, storeName)
	}

	val, exists := storeMap[key]
	if !exists {
		return nil, fmt.Errorf("%w: %s in store %s", ErrNotFound, key, storeName)
	}

	return bytes.Clone(val), nil
}

// DeleteState removes a key from storeName.
func (m *MemoryStore) DeleteState(ctx context.Context, storeName, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := rejectPrivateRoleCollection(storeName); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if storeMap, exists := m.items[storeName]; exists {
		delete(storeMap, key)
	}
	return nil
}

// ListState returns all keys and byte clones for a storeName.
func (m *MemoryStore) ListState(ctx context.Context, storeName string) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := rejectPrivateRoleCollection(storeName); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make(map[string][]byte)
	storeMap, exists := m.items[storeName]
	if !exists {
		return res, nil
	}

	for k, v := range storeMap {
		res[k] = bytes.Clone(v)
	}
	return res, nil
}

// ListPendingOutbox returns a bounded, deterministic set of pending event IDs
// for one topic without including published history.
func (m *MemoryStore) ListPendingOutbox(ctx context.Context, topic string, limit int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topic == "" || limit < 1 {
		return nil, errors.New("store: pending outbox topic and positive limit are required")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	records := make([]outboxIndexRecord, 0)
	var decodeErrors []error
	for key, data := range m.items[OutboxCollection] {
		var record outboxIndexRecord
		if err := json.Unmarshal(data, &record); err != nil {
			decodeErrors = append(decodeErrors, fmt.Errorf("store: decode outbox %s: %w", key, err))
			continue
		}
		if record.ID != key {
			decodeErrors = append(decodeErrors, fmt.Errorf("store: outbox ID %q does not match key %q", record.ID, key))
			continue
		}
		if record.Topic == topic && record.Status == "pending" {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID < records[j].ID
		}
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	if len(records) > limit {
		records = records[:limit]
	}
	ids := make([]string, len(records))
	for index := range records {
		ids[index] = records[index].ID
	}
	return ids, errors.Join(decodeErrors...)
}

func validateAtomicWrites(states []StateWrite, outbox StateWrite) error {
	if len(states) == 0 {
		return errors.New("store: at least one state write is required")
	}
	writes := append(append(make([]StateWrite, 0, len(states)+1), states...), outbox)
	targets := make(map[string]struct{}, len(writes))
	for _, write := range writes {
		if write.StoreName == "" || write.Key == "" {
			return errors.New("store: atomic write store name and key are required")
		}
		if err := rejectPrivateRoleCollection(write.StoreName); err != nil {
			return err
		}
		target := write.StoreName + "\x00" + write.Key
		if _, exists := targets[target]; exists {
			return fmt.Errorf("store: duplicate atomic write target %s/%s", write.StoreName, write.Key)
		}
		targets[target] = struct{}{}
	}
	if outbox.StoreName != OutboxCollection {
		return fmt.Errorf("store: outbox write must target %s", OutboxCollection)
	}
	return nil
}
