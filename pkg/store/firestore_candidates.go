package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const lostPetCandidateIndexMigrationID = "lost-pet-candidate-index-v1"

type lostPetCandidateIndexMigration struct {
	Cursor    string    `firestore:"cursor"`
	Complete  bool      `firestore:"complete"`
	UpdatedAt time.Time `firestore:"updatedAt"`
}

// QueryLostPetCandidates uses Firestore index fields to bound status,
// geocoding, species, time, latitude, and longitude before returning payloads.
func (s *FirestoreStore) QueryLostPetCandidates(
	ctx context.Context,
	query LostPetCandidateQuery,
) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := query.validate(); err != nil {
		return nil, err
	}
	query.Status = strings.TrimSpace(query.Status)
	query.GeocodingStatus = strings.TrimSpace(query.GeocodingStatus)
	query.Species = strings.ToLower(strings.TrimSpace(query.Species))
	collection, err := s.collection(LostPetsCollection)
	if err != nil {
		return nil, err
	}

	firestoreQuery := collection.
		Where("lostStatus", "==", query.Status).
		Where("lostGeocodingStatus", "==", query.GeocodingStatus)
	if query.Species != "" {
		firestoreQuery = firestoreQuery.Where("lostSpecies", "in", []string{"", query.Species})
	}
	firestoreQuery = firestoreQuery.
		Where("lostReportedAt", ">=", query.ReportedAfter).
		Where("lostReportedAt", "<=", query.ReportedBefore).
		Where("lostLatitude", ">=", query.MinLatitude).
		Where("lostLatitude", "<=", query.MaxLatitude).
		Where("lostLongitude", ">=", query.MinLongitude).
		Where("lostLongitude", "<=", query.MaxLongitude).
		OrderBy("lostReportedAt", firestore.Asc).
		OrderBy("lostLatitude", firestore.Asc).
		OrderBy("lostLongitude", firestore.Asc).
		OrderBy("key", firestore.Asc)

	documents := firestoreQuery.Documents(ctx)
	defer documents.Stop()
	result := make(map[string][]byte)
	for {
		snapshot, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			return result, nil
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("store: query lost-pet candidates: %w", err)
		}
		var record firestoreRecord
		if err := snapshot.DataTo(&record); err != nil {
			return nil, fmt.Errorf("store: decode indexed lost-pet candidate %s: %w", snapshot.Ref.ID, err)
		}
		if record.Key == "" {
			return nil, fmt.Errorf("store: indexed lost-pet candidate %s has no key", snapshot.Ref.ID)
		}
		result[record.Key] = bytes.Clone(record.Data)
	}
}

// BackfillLostPetCandidateIndexes upgrades legacy opaque records in bounded,
// cursor-backed batches. The durable completion marker prevents every matcher
// instance startup from rescanning the collection.
func (s *FirestoreStore) BackfillLostPetCandidateIndexes(ctx context.Context, limit int) (int, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if limit < 1 || limit > MaxLostPetCandidateBackfillBatch {
		return 0, false, errors.New("store: lost-pet candidate index backfill limit must be between 1 and 400")
	}
	migrationDoc := s.client.Collection("runtimeMigrations").Doc(firestoreDocumentID(lostPetCandidateIndexMigrationID))
	var migration lostPetCandidateIndexMigration
	snapshot, err := migrationDoc.Get(ctx)
	if err == nil {
		if err := snapshot.DataTo(&migration); err != nil {
			return 0, false, fmt.Errorf("store: decode lost-pet candidate index migration: %w", err)
		}
		if migration.Complete {
			return 0, true, nil
		}
	} else if status.Code(err) != codes.NotFound {
		return 0, false, fmt.Errorf("store: read lost-pet candidate index migration: %w", err)
	}

	collection, err := s.collection(LostPetsCollection)
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
			return 0, false, fmt.Errorf("store: scan legacy lost-pet candidate indexes: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	migrated := 0
	bulkWriter := s.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var record firestoreRecord
		if err := snapshot.DataTo(&record); err != nil {
			bulkWriter.End()
			return 0, false, fmt.Errorf("store: decode legacy lost-pet candidate %s: %w", snapshot.Ref.ID, err)
		}
		if record.Key == "" {
			bulkWriter.End()
			return 0, false, fmt.Errorf("store: legacy lost-pet candidate %s has no key", snapshot.Ref.ID)
		}
		migration.Cursor = record.Key
		if record.LostLatitude != nil && record.LostLongitude != nil && !record.LostReportedAt.IsZero() {
			continue
		}
		indexed, ok := indexedLostPetCandidate(record.Key, record.Data)
		if !ok {
			continue
		}
		job, err := bulkWriter.Update(snapshot.Ref, lostPetCandidateIndexUpdates(indexed), firestore.LastUpdateTime(snapshot.UpdateTime))
		if err != nil {
			bulkWriter.End()
			return 0, false, fmt.Errorf("store: queue lost-pet candidate index backfill for %s: %w", record.Key, err)
		}
		jobs = append(jobs, job)
		migrated++
	}
	bulkWriter.End()
	for index, job := range jobs {
		if _, err := job.Results(); err != nil {
			return 0, false, fmt.Errorf("store: write lost-pet candidate index backfill %d: %w", index, err)
		}
	}

	migration.Complete = len(snapshots) < limit
	migration.UpdatedAt = time.Now().UTC()
	if _, err := migrationDoc.Set(ctx, migration); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, false, ctxErr
		}
		return 0, false, fmt.Errorf("store: save lost-pet candidate index backfill cursor: %w", err)
	}
	return migrated, migration.Complete, nil
}

func indexedLostPetCandidate(key string, data []byte) (firestoreRecord, bool) {
	record, err := newFirestoreRecord(LostPetsCollection, key, data)
	if err != nil || record.LostSpecies == nil || record.LostLatitude == nil || record.LostLongitude == nil ||
		record.LostReportedAt.IsZero() {
		return firestoreRecord{}, false
	}
	return record, true
}

func lostPetCandidateIndexUpdates(record firestoreRecord) []firestore.Update {
	return []firestore.Update{
		{Path: "lostStatus", Value: record.LostStatus},
		{Path: "lostGeocodingStatus", Value: record.LostGeocodingStatus},
		{Path: "lostSpecies", Value: *record.LostSpecies},
		{Path: "lostReportedAt", Value: record.LostReportedAt},
		{Path: "lostLatitude", Value: *record.LostLatitude},
		{Path: "lostLongitude", Value: *record.LostLongitude},
	}
}
