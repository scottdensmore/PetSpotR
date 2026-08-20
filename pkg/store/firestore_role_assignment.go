package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	roleAssignmentsCollection = "operatorRoleAssignments"
	roleAssignmentAudit       = "audit"
)

// GetRoleAssignment reads one exact private principal-role-scope assignment.
// It performs no collection query and returns revoked tombstones to callers.
func (s *FirestoreStore) GetRoleAssignment(
	ctx context.Context,
	principal domain.PrincipalRef,
	role domain.Role,
	scope domain.RoleScope,
) (domain.RoleAssignment, error) {
	if err := ctx.Err(); err != nil {
		return domain.RoleAssignment{}, err
	}
	assignmentID, err := domain.RoleAssignmentID(principal, role, scope)
	if err != nil {
		return domain.RoleAssignment{}, err
	}
	doc, err := s.document(roleAssignmentsCollection, assignmentID)
	if err != nil {
		return domain.RoleAssignment{}, err
	}
	snapshot, err := doc.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return domain.RoleAssignment{}, fmt.Errorf("%w: role assignment %s", ErrNotFound, assignmentID)
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.RoleAssignment{}, ctxErr
		}
		return domain.RoleAssignment{}, fmt.Errorf("store: get role assignment %s: %w", assignmentID, err)
	}
	assignment, err := roleAssignmentFromSnapshot(snapshot, assignmentID)
	if err != nil {
		return domain.RoleAssignment{}, fmt.Errorf("store: get role assignment %s: %w", assignmentID, err)
	}
	return assignment, nil
}

// UpdateStateAndCreateOutboxAsRole reads the private assignment and updates an
// aggregate plus its outbox record in one Firestore transaction. A concurrent
// revocation is serialized before or after the privileged write.
func (s *FirestoreStore) UpdateStateAndCreateOutboxAsRole(
	ctx context.Context,
	principal domain.PrincipalRef,
	role domain.Role,
	scope domain.RoleScope,
	storeName, key string,
	update RoleAuthorizedStateAndOutboxUpdater,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(storeName) == "" || strings.TrimSpace(key) == "" || update == nil {
		return errors.New("store: role-authorized state target and updater are required")
	}
	if err := rejectPrivateRoleCollection(storeName); err != nil {
		return err
	}
	assignmentID, err := domain.RoleAssignmentID(principal, role, scope)
	if err != nil {
		return err
	}
	assignmentDoc, err := s.document(roleAssignmentsCollection, assignmentID)
	if err != nil {
		return err
	}
	stateDoc, err := s.document(storeName, key)
	if err != nil {
		return err
	}

	err = s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		assignmentSnapshot, getErr := tx.Get(assignmentDoc)
		if status.Code(getErr) == codes.NotFound {
			return ErrRoleDenied
		}
		if getErr != nil {
			return getErr
		}
		assignment, decodeErr := roleAssignmentFromSnapshot(assignmentSnapshot, assignmentID)
		if decodeErr != nil {
			return decodeErr
		}
		if assignment.Status != domain.RoleAssignmentStatusActive {
			return ErrRoleDenied
		}

		current, getErr := transactionRecord(tx, stateDoc, storeName, key)
		if getErr != nil {
			return getErr
		}
		next, outboxWrite, updateErr := update(assignment, bytes.Clone(current.Data))
		if updateErr != nil {
			return updateErr
		}
		if next.StoreName != storeName || next.Key != key {
			return errors.New("store: updater changed its state target")
		}
		if validateErr := validateAtomicWrites([]StateWrite{next}, outboxWrite); validateErr != nil {
			return validateErr
		}
		outboxDoc, documentErr := s.document(outboxWrite.StoreName, outboxWrite.Key)
		if documentErr != nil {
			return documentErr
		}
		if _, getErr := tx.Get(outboxDoc); status.Code(getErr) != codes.NotFound {
			if getErr == nil {
				return fmt.Errorf("%w: %s/%s", ErrConflict, outboxWrite.StoreName, outboxWrite.Key)
			}
			return getErr
		}
		stateRecord, encodeErr := newFirestoreRecord(next.StoreName, next.Key, next.Data)
		if encodeErr != nil {
			return encodeErr
		}
		outboxRecord, encodeErr := newFirestoreRecord(outboxWrite.StoreName, outboxWrite.Key, outboxWrite.Data)
		if encodeErr != nil {
			return encodeErr
		}
		if setErr := tx.Set(stateDoc, stateRecord); setErr != nil {
			return setErr
		}
		return tx.Set(outboxDoc, outboxRecord)
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("store: role-authorized update %s/%s and create outbox: %w", storeName, key, err)
	}
	return nil
}

// UpdateMatchAndParticipantsAsRole reads the private assignment and updates
// both match records in one Firestore transaction. A concurrent revocation is
// serialized before or after the privileged write, never between its check and
// commit.
func (s *FirestoreStore) UpdateMatchAndParticipantsAsRole(
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
	assignmentDoc, err := s.document(roleAssignmentsCollection, assignmentID)
	if err != nil {
		return err
	}
	matchDoc, err := s.document(MatchesCollection, matchID)
	if err != nil {
		return err
	}
	participantsDoc, err := s.document(MatchParticipantsCollection, matchID)
	if err != nil {
		return err
	}

	err = s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		assignmentSnapshot, getErr := tx.Get(assignmentDoc)
		if status.Code(getErr) == codes.NotFound {
			return ErrRoleDenied
		}
		if getErr != nil {
			return getErr
		}
		assignment, decodeErr := roleAssignmentFromSnapshot(assignmentSnapshot, assignmentID)
		if decodeErr != nil {
			return decodeErr
		}
		if assignment.Status != domain.RoleAssignmentStatusActive {
			return ErrRoleDenied
		}

		match, getErr := transactionRecord(tx, matchDoc, MatchesCollection, matchID)
		if getErr != nil {
			return getErr
		}
		participants, getErr := transactionRecord(tx, participantsDoc, MatchParticipantsCollection, matchID)
		if getErr != nil {
			return getErr
		}
		nextMatch, nextParticipants, updateErr := update(
			assignment, bytes.Clone(match.Data), bytes.Clone(participants.Data),
		)
		if updateErr != nil {
			return updateErr
		}
		matchRecord, encodeErr := newFirestoreRecord(MatchesCollection, matchID, nextMatch)
		if encodeErr != nil {
			return encodeErr
		}
		participantsRecord, encodeErr := newFirestoreRecord(MatchParticipantsCollection, matchID, nextParticipants)
		if encodeErr != nil {
			return encodeErr
		}
		if setErr := tx.Set(matchDoc, matchRecord); setErr != nil {
			return setErr
		}
		return tx.Set(participantsDoc, participantsRecord)
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("store: role-authorized match update %s: %w", matchID, err)
	}
	return nil
}

// GrantRoleAssignment atomically grants or regrants one role assignment and
// creates its immutable operation receipt.
func (s *FirestoreStore) GrantRoleAssignment(
	ctx context.Context,
	change domain.RoleAssignmentChange,
) (domain.RoleAssignment, bool, error) {
	return s.changeRoleAssignment(ctx, domain.RoleAssignmentActionGrant, change)
}

// RevokeRoleAssignment atomically revokes one role assignment and creates its
// immutable operation receipt.
func (s *FirestoreStore) RevokeRoleAssignment(
	ctx context.Context,
	change domain.RoleAssignmentChange,
) (domain.RoleAssignment, bool, error) {
	return s.changeRoleAssignment(ctx, domain.RoleAssignmentActionRevoke, change)
}

func (s *FirestoreStore) changeRoleAssignment(
	ctx context.Context,
	action domain.RoleAssignmentAction,
	change domain.RoleAssignmentChange,
) (domain.RoleAssignment, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.RoleAssignment{}, false, err
	}
	assignmentID, auditID, err := roleChangeIDs(change)
	if err != nil {
		return domain.RoleAssignment{}, false, err
	}
	assignmentDoc, err := s.document(roleAssignmentsCollection, assignmentID)
	if err != nil {
		return domain.RoleAssignment{}, false, err
	}
	auditDoc := assignmentDoc.Collection(roleAssignmentAudit).Doc(firestoreDocumentID(auditID))

	var result domain.RoleAssignment
	var changed bool
	err = s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		result = domain.RoleAssignment{}
		changed = false

		auditSnapshot, auditErr := tx.Get(auditDoc)
		if auditErr == nil {
			audit, decodeErr := roleAssignmentAuditFromSnapshot(auditSnapshot, auditID, assignmentID)
			if decodeErr != nil {
				return decodeErr
			}
			if !audit.MatchesChange(action, change) {
				return fmt.Errorf("%w: role assignment operation %s", ErrConflict, change.OperationID)
			}
			result = audit.Result
			return nil
		}
		if status.Code(auditErr) != codes.NotFound {
			return auditErr
		}

		var current *domain.RoleAssignment
		assignmentSnapshot, assignmentErr := tx.Get(assignmentDoc)
		switch status.Code(assignmentErr) {
		case codes.OK:
			decoded, decodeErr := roleAssignmentFromSnapshot(assignmentSnapshot, assignmentID)
			if decodeErr != nil {
				return decodeErr
			}
			current = &decoded
		case codes.NotFound:
			// Grant creates a first revision; revoke fails closed in domain.
		default:
			return assignmentErr
		}

		next, audit, transitionErr := applyRoleAssignmentChange(action, current, change)
		if transitionErr != nil {
			return transitionErr
		}
		assignmentData, encodeErr := json.Marshal(next)
		if encodeErr != nil {
			return fmt.Errorf("store: encode role assignment: %w", encodeErr)
		}
		auditData, encodeErr := json.Marshal(audit)
		if encodeErr != nil {
			return fmt.Errorf("store: encode role assignment audit: %w", encodeErr)
		}
		assignmentRecord, encodeErr := newFirestoreRecord(roleAssignmentsCollection, assignmentID, assignmentData)
		if encodeErr != nil {
			return encodeErr
		}
		auditRecord, encodeErr := newFirestoreRecord(roleAssignmentAudit, auditID, auditData)
		if encodeErr != nil {
			return encodeErr
		}
		if setErr := tx.Set(assignmentDoc, assignmentRecord); setErr != nil {
			return setErr
		}
		if setErr := tx.Set(auditDoc, auditRecord); setErr != nil {
			return setErr
		}
		result = next
		changed = true
		return nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.RoleAssignment{}, false, ctxErr
		}
		return domain.RoleAssignment{}, false,
			fmt.Errorf("store: %s role assignment %s: %w", action, assignmentID, err)
	}
	return result, changed, nil
}

// ListRoleAssignmentAudit returns the private immutable operation receipts in
// assignment revision order. Ordering happens in memory and needs no index.
func (s *FirestoreStore) ListRoleAssignmentAudit(
	ctx context.Context,
	assignmentID string,
) ([]domain.RoleAssignmentAudit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := domain.RoleAssignmentAuditID(assignmentID, "validate"); err != nil {
		return nil, err
	}
	assignmentDoc, err := s.document(roleAssignmentsCollection, assignmentID)
	if err != nil {
		return nil, err
	}
	iter := assignmentDoc.Collection(roleAssignmentAudit).Documents(ctx)
	defer iter.Stop()
	audits := make([]domain.RoleAssignmentAudit, 0)
	for {
		snapshot, nextErr := iter.Next()
		if errors.Is(nextErr, iterator.Done) {
			break
		}
		if nextErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("store: list role assignment audit %s: %w", assignmentID, nextErr)
		}
		var record firestoreRecord
		if decodeErr := snapshot.DataTo(&record); decodeErr != nil {
			return nil, fmt.Errorf("store: decode role assignment audit record: %w", decodeErr)
		}
		if snapshot.Ref.ID != firestoreDocumentID(record.Key) {
			return nil, errors.New("store: role assignment audit document key mismatch")
		}
		audit, decodeErr := decodeRoleAssignmentAudit(record.Data)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if audit.AuditID != record.Key || audit.AssignmentID != assignmentID {
			return nil, errors.New("store: role assignment audit key mismatch")
		}
		audits = append(audits, audit)
	}
	if len(audits) == 0 {
		return nil, fmt.Errorf("%w: role assignment audit %s", ErrNotFound, assignmentID)
	}
	if err := sortAndValidateRoleAssignmentAudits(audits); err != nil {
		return nil, err
	}
	return audits, nil
}

func roleAssignmentFromSnapshot(
	snapshot *firestore.DocumentSnapshot,
	assignmentID string,
) (domain.RoleAssignment, error) {
	var record firestoreRecord
	if err := snapshot.DataTo(&record); err != nil {
		return domain.RoleAssignment{}, err
	}
	if record.Key != assignmentID || snapshot.Ref.ID != firestoreDocumentID(assignmentID) {
		return domain.RoleAssignment{}, errors.New("store: role assignment key mismatch")
	}
	assignment, err := decodeRoleAssignment(bytes.Clone(record.Data))
	if err != nil {
		return domain.RoleAssignment{}, err
	}
	if assignment.AssignmentID != assignmentID {
		return domain.RoleAssignment{}, errors.New("store: role assignment payload key mismatch")
	}
	return assignment, nil
}

func roleAssignmentAuditFromSnapshot(
	snapshot *firestore.DocumentSnapshot,
	auditID string,
	assignmentID string,
) (domain.RoleAssignmentAudit, error) {
	var record firestoreRecord
	if err := snapshot.DataTo(&record); err != nil {
		return domain.RoleAssignmentAudit{}, err
	}
	if record.Key != auditID || snapshot.Ref.ID != firestoreDocumentID(auditID) {
		return domain.RoleAssignmentAudit{}, errors.New("store: role assignment audit key mismatch")
	}
	audit, err := decodeRoleAssignmentAudit(bytes.Clone(record.Data))
	if err != nil {
		return domain.RoleAssignmentAudit{}, err
	}
	if audit.AuditID != auditID || audit.AssignmentID != assignmentID {
		return domain.RoleAssignmentAudit{}, errors.New("store: role assignment audit payload key mismatch")
	}
	return audit, nil
}
