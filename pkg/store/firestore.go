package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

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
	Key  string `firestore:"key"`
	Data []byte `firestore:"data"`
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
	if _, err := doc.Set(ctx, firestoreRecord{Key: key, Data: bytes.Clone(data)}); err != nil {
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
		if err := tx.Set(stateDoc, firestoreRecord{Key: state.Key, Data: bytes.Clone(state.Data)}); err != nil {
			return err
		}
		if err := tx.Set(outboxDoc, firestoreRecord{Key: outbox.Key, Data: bytes.Clone(outbox.Data)}); err != nil {
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
