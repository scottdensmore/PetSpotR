package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DetectFirestoreProjectID asks the Firestore client to obtain the project ID
// from Application Default Credentials or the metadata server.
const DetectFirestoreProjectID = firestore.DetectProjectID

// ErrInvalidPath indicates that a collection name is not a valid top-level
// Firestore path segment. Opaque state keys are hashed into document IDs.
var ErrInvalidPath = errors.New("store: invalid Firestore path")

type firestoreRecord struct {
	Key       string    `firestore:"key"`
	Data      []byte    `firestore:"data"`
	Topic     string    `firestore:"topic,omitempty"`
	Status    string    `firestore:"status,omitempty"`
	CreatedAt time.Time `firestore:"createdAt,omitempty"`
}

type outboxIndexMigration struct {
	Cursor    string    `firestore:"cursor"`
	Complete  bool      `firestore:"complete"`
	UpdatedAt time.Time `firestore:"updatedAt"`
}

// FirestoreStore persists StateStore payloads in Firestore collections.
type FirestoreStore struct {
	client   *firestore.Client
	closeErr error
	close    sync.Once
}

// NewFirestoreStore constructs a Firestore-backed StateStore using
// Application Default Credentials unless client options override them.
func NewFirestoreStore(ctx context.Context, projectID string, opts ...option.ClientOption) (*FirestoreStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("store: Firestore project ID is required")
	}

	client, err := firestore.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("store: create Firestore client: %w", err)
	}
	return &FirestoreStore{client: client}, nil
}

// NewFirestoreEmulatorStore constructs a Firestore-backed StateStore using an
// explicit emulator endpoint and no production credentials.
func NewFirestoreEmulatorStore(ctx context.Context, projectID, host string) (*FirestoreStore, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("store: Firestore emulator host is required")
	}
	configuredHost := strings.TrimSpace(os.Getenv("FIRESTORE_EMULATOR_HOST"))
	if configuredHost == "" {
		return nil, errors.New("store: FIRESTORE_EMULATOR_HOST is not set")
	}
	if configuredHost != host {
		return nil, fmt.Errorf(
			"store: configured Firestore emulator host %q does not match environment host %q",
			host,
			configuredHost,
		)
	}
	return NewFirestoreStore(ctx, projectID)
}

// SaveState saves data under a top-level collection and document key.
func (s *FirestoreStore) SaveState(ctx context.Context, storeName, key string, data []byte) error {
	doc, err := s.document(storeName, key)
	if err != nil {
		return err
	}
	record, err := newFirestoreRecord(storeName, key, data)
	if err != nil {
		return err
	}
	if _, err := doc.Set(ctx, record); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("store: save %s/%s: %w", storeName, key, err)
	}
	return nil
}

// CreateStateAndOutbox creates aggregate state and an outbox record in one
// Firestore transaction. Exact retries do not replace a completed outbox
// record, while competing aggregate creates fail with ErrConflict.
func (s *FirestoreStore) CreateStateAndOutbox(ctx context.Context, state, outbox StateWrite) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validateAtomicWrites(state, outbox); err != nil {
		return false, err
	}
	stateDoc, err := s.document(state.StoreName, state.Key)
	if err != nil {
		return false, err
	}
	outboxDoc, err := s.document(outbox.StoreName, outbox.Key)
	if err != nil {
		return false, err
	}

	created := false
	err = s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Firestore may retry this callback after contention. Reset the result
		// for every attempt so a superseded write cannot leak into the return.
		created = false
		stateSnapshot, stateErr := tx.Get(stateDoc)
		outboxSnapshot, outboxErr := tx.Get(outboxDoc)
		stateExists := stateErr == nil
		outboxExists := outboxErr == nil
		if stateErr != nil && status.Code(stateErr) != codes.NotFound {
			return stateErr
		}
		if outboxErr != nil && status.Code(outboxErr) != codes.NotFound {
			return outboxErr
		}
		if stateExists || outboxExists {
			if !stateExists || !outboxExists {
				return fmt.Errorf("%w: %s/%s", ErrConflict, state.StoreName, state.Key)
			}
			var storedState, storedOutbox firestoreRecord
			if err := stateSnapshot.DataTo(&storedState); err != nil {
				return err
			}
			if err := outboxSnapshot.DataTo(&storedOutbox); err != nil {
				return err
			}
			if storedState.Key == state.Key && storedOutbox.Key == outbox.Key && bytes.Equal(storedState.Data, state.Data) {
				return nil
			}
			return fmt.Errorf("%w: %s/%s", ErrConflict, state.StoreName, state.Key)
		}
		stateRecord, err := newFirestoreRecord(state.StoreName, state.Key, state.Data)
		if err != nil {
			return err
		}
		outboxRecord, err := newFirestoreRecord(outbox.StoreName, outbox.Key, outbox.Data)
		if err != nil {
			return err
		}
		if err := tx.Set(stateDoc, stateRecord); err != nil {
			return err
		}
		if err := tx.Set(outboxDoc, outboxRecord); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, ctxErr
		}
		return false, fmt.Errorf("store: create state and outbox: %w", err)
	}
	return created, nil
}

// ListPendingOutbox returns at most limit pending IDs for one topic using
// indexed Firestore fields rather than scanning historical payload blobs.
func (s *FirestoreStore) ListPendingOutbox(ctx context.Context, topic string, limit int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(topic) == "" || limit < 1 {
		return nil, errors.New("store: pending outbox topic and positive limit are required")
	}
	collection, err := s.collection(OutboxCollection)
	if err != nil {
		return nil, err
	}
	documents := collection.
		Where("topic", "==", topic).
		Where("status", "==", "pending").
		OrderBy("createdAt", firestore.Asc).
		OrderBy("key", firestore.Asc).
		Limit(limit).
		Documents(ctx)
	defer documents.Stop()
	ids := make([]string, 0, limit)
	for {
		snapshot, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			return ids, nil
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("store: list pending outbox for %s: %w", topic, err)
		}
		var record firestoreRecord
		if err := snapshot.DataTo(&record); err != nil {
			return nil, fmt.Errorf("store: decode pending outbox %s: %w", snapshot.Ref.ID, err)
		}
		if record.Key == "" {
			return nil, fmt.Errorf("store: pending outbox %s has no key", snapshot.Ref.ID)
		}
		ids = append(ids, record.Key)
	}
}

// BackfillOutboxIndexes upgrades legacy key/data-only outbox documents in a
// cursor-backed batch. The durable cursor prevents every recovery tick from
// rescanning the collection from the beginning.
func (s *FirestoreStore) BackfillOutboxIndexes(ctx context.Context, limit int) (int, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if limit < 1 || limit > 400 {
		return 0, false, errors.New("store: outbox index backfill limit must be between 1 and 400")
	}
	migrationDoc := s.client.Collection("runtimeMigrations").Doc(firestoreDocumentID("outbox-index-v1"))
	var migration outboxIndexMigration
	snapshot, err := migrationDoc.Get(ctx)
	if err == nil {
		if err := snapshot.DataTo(&migration); err != nil {
			return 0, false, fmt.Errorf("store: decode outbox index migration: %w", err)
		}
		if migration.Complete {
			// A prior revision can finish an in-flight request after this one
			// reaches the end. Start a new bounded sweep so late legacy records,
			// including keys behind the old cursor, remain recoverable.
			migration.Cursor = ""
			migration.Complete = false
		}
	} else if status.Code(err) != codes.NotFound {
		return 0, false, fmt.Errorf("store: read outbox index migration: %w", err)
	}

	collection, err := s.collection(OutboxCollection)
	if err != nil {
		return 0, false, err
	}
	query := collection.OrderBy("key", firestore.Asc)
	if migration.Cursor != "" {
		query = query.StartAfter(migration.Cursor)
	}
	documents := query.Limit(limit).Documents(ctx)
	defer documents.Stop()
	snapshots := make([]*firestore.DocumentSnapshot, 0, limit)
	for {
		snapshot, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return 0, false, ctxErr
			}
			return 0, false, fmt.Errorf("store: scan legacy outbox indexes: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	migrated := 0
	var recordErrors []error
	bulkWriter := s.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(snapshots))
	for _, snapshot := range snapshots {
		keyValue, err := snapshot.DataAt("key")
		key, keyOK := keyValue.(string)
		if err != nil || !keyOK || key == "" {
			recordErrors = append(recordErrors, fmt.Errorf("store: legacy outbox %s has no string key", snapshot.Ref.ID))
			continue
		}
		migration.Cursor = key
		var record firestoreRecord
		if err := snapshot.DataTo(&record); err != nil {
			recordErrors = append(recordErrors, fmt.Errorf("store: decode legacy outbox %s: %w", snapshot.Ref.ID, err))
			continue
		}
		if record.Topic != "" && record.Status != "" && !record.CreatedAt.IsZero() {
			continue
		}
		indexed, err := newFirestoreRecord(OutboxCollection, record.Key, record.Data)
		if err != nil {
			recordErrors = append(recordErrors, err)
			continue
		}
		job, err := s.queueOutboxIndexBackfill(bulkWriter, snapshot, indexed)
		if err != nil {
			bulkWriter.End()
			return 0, false, fmt.Errorf("store: queue outbox index backfill for %s: %w", record.Key, err)
		}
		jobs = append(jobs, job)
		migrated++
	}
	bulkWriter.End()
	var writeErrors []error
	for index, job := range jobs {
		if _, err := job.Results(); err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("store: write outbox index backfill %d: %w", index, err))
		}
	}
	if err := errors.Join(writeErrors...); err != nil {
		return 0, false, err
	}
	migration.Complete = len(snapshots) < limit
	migration.UpdatedAt = time.Now().UTC()
	if _, err := migrationDoc.Set(ctx, migration); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, false, ctxErr
		}
		return 0, false, fmt.Errorf("store: save outbox index backfill cursor: %w", err)
	}
	return migrated, migration.Complete, errors.Join(recordErrors...)
}

func (s *FirestoreStore) queueOutboxIndexBackfill(
	bulkWriter *firestore.BulkWriter,
	snapshot *firestore.DocumentSnapshot,
	indexed firestoreRecord,
) (*firestore.BulkWriterJob, error) {
	return bulkWriter.Update(snapshot.Ref, []firestore.Update{
		{Path: "topic", Value: indexed.Topic},
		{Path: "status", Value: indexed.Status},
		{Path: "createdAt", Value: indexed.CreatedAt},
	}, firestore.LastUpdateTime(snapshot.UpdateTime))
}

// GetState retrieves data from a top-level collection and document key.
func (s *FirestoreStore) GetState(ctx context.Context, storeName, key string) ([]byte, error) {
	doc, err := s.document(storeName, key)
	if err != nil {
		return nil, err
	}

	snapshot, err := doc.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, fmt.Errorf("%w: %s in store %s", ErrNotFound, key, storeName)
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("store: get %s/%s: %w", storeName, key, err)
	}

	var record firestoreRecord
	if err := snapshot.DataTo(&record); err != nil {
		return nil, fmt.Errorf("store: decode %s/%s: %w", storeName, key, err)
	}
	return bytes.Clone(record.Data), nil
}

// DeleteState removes a document. Deleting an absent document succeeds.
func (s *FirestoreStore) DeleteState(ctx context.Context, storeName, key string) error {
	doc, err := s.document(storeName, key)
	if err != nil {
		return err
	}
	if _, err := doc.Delete(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("store: delete %s/%s: %w", storeName, key, err)
	}
	return nil
}

// ListState returns every document payload in a top-level collection.
func (s *FirestoreStore) ListState(ctx context.Context, storeName string) (map[string][]byte, error) {
	collection, err := s.collection(storeName)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	documents := collection.Documents(ctx)
	defer documents.Stop()
	for {
		snapshot, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			return result, nil
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("store: list %s: %w", storeName, err)
		}

		var record firestoreRecord
		if err := snapshot.DataTo(&record); err != nil {
			return nil, fmt.Errorf("store: decode %s/%s: %w", storeName, snapshot.Ref.ID, err)
		}
		result[record.Key] = bytes.Clone(record.Data)
	}
}

// Close flushes and closes the underlying Firestore client.
func (s *FirestoreStore) Close() error {
	s.close.Do(func() {
		s.closeErr = s.client.Close()
	})
	return s.closeErr
}

func (s *FirestoreStore) collection(storeName string) (*firestore.CollectionRef, error) {
	storeName = strings.TrimSpace(storeName)
	if storeName == "" || strings.Contains(storeName, "/") {
		return nil, fmt.Errorf("%w: collection %q", ErrInvalidPath, storeName)
	}
	return s.client.Collection(storeName), nil
}

func (s *FirestoreStore) document(storeName, key string) (*firestore.DocumentRef, error) {
	collection, err := s.collection(storeName)
	if err != nil {
		return nil, err
	}
	return collection.Doc(firestoreDocumentID(key)), nil
}

func firestoreDocumentID(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

func newFirestoreRecord(storeName, key string, data []byte) (firestoreRecord, error) {
	record := firestoreRecord{Key: key, Data: bytes.Clone(data)}
	if storeName != OutboxCollection {
		return record, nil
	}
	var index outboxIndexRecord
	if err := json.Unmarshal(data, &index); err != nil {
		return firestoreRecord{}, fmt.Errorf("store: decode outbox index %s: %w", key, err)
	}
	if index.ID != key || index.Topic == "" || index.Status == "" || index.CreatedAt.IsZero() {
		return firestoreRecord{}, fmt.Errorf("store: invalid outbox index metadata for %s", key)
	}
	record.Topic = index.Topic
	record.Status = index.Status
	record.CreatedAt = index.CreatedAt
	return record, nil
}
