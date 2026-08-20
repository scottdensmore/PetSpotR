package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// RoleAssignmentStore is the private authorization persistence boundary.
// Implementations must update an assignment and its immutable operation audit
// atomically. Exact operation retries return the original result with changed
// false; changed reuse of an operation ID returns ErrConflict.
type RoleAssignmentStore interface {
	GetRoleAssignment(
		ctx context.Context,
		principal domain.PrincipalRef,
		role domain.Role,
		scope domain.RoleScope,
	) (domain.RoleAssignment, error)
	GrantRoleAssignment(
		ctx context.Context,
		change domain.RoleAssignmentChange,
	) (assignment domain.RoleAssignment, changed bool, err error)
	RevokeRoleAssignment(
		ctx context.Context,
		change domain.RoleAssignmentChange,
	) (assignment domain.RoleAssignment, changed bool, err error)
	ListRoleAssignmentAudit(ctx context.Context, assignmentID string) ([]domain.RoleAssignmentAudit, error)
}

func rejectPrivateRoleCollection(storeName string) error {
	if strings.TrimSpace(storeName) == roleAssignmentsCollection {
		return errors.New("store: operator role assignments require RoleAssignmentStore")
	}
	return nil
}

// GetRoleAssignment returns the current assignment, including a revoked
// tombstone. Authorization callers must require StatusActive.
func (m *MemoryStore) GetRoleAssignment(
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
	m.mu.RLock()
	data, exists := m.roleAssignments[assignmentID]
	m.mu.RUnlock()
	if !exists {
		return domain.RoleAssignment{}, fmt.Errorf("%w: role assignment %s", ErrNotFound, assignmentID)
	}
	assignment, err := decodeRoleAssignment(data)
	if err != nil {
		return domain.RoleAssignment{}, err
	}
	if assignment.AssignmentID != assignmentID {
		return domain.RoleAssignment{}, errors.New("store: role assignment key mismatch")
	}
	return assignment, nil
}

// GrantRoleAssignment atomically grants or regrants one role assignment and
// appends its immutable audit receipt.
func (m *MemoryStore) GrantRoleAssignment(
	ctx context.Context,
	change domain.RoleAssignmentChange,
) (domain.RoleAssignment, bool, error) {
	return m.changeRoleAssignment(ctx, domain.RoleAssignmentActionGrant, change)
}

// RevokeRoleAssignment atomically revokes one role assignment and appends its
// immutable audit receipt.
func (m *MemoryStore) RevokeRoleAssignment(
	ctx context.Context,
	change domain.RoleAssignmentChange,
) (domain.RoleAssignment, bool, error) {
	return m.changeRoleAssignment(ctx, domain.RoleAssignmentActionRevoke, change)
}

func (m *MemoryStore) changeRoleAssignment(
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

	m.mu.Lock()
	defer m.mu.Unlock()
	if audits := m.roleAudits[assignmentID]; audits != nil {
		if data, exists := audits[auditID]; exists {
			audit, decodeErr := decodeRoleAssignmentAudit(data)
			if decodeErr != nil {
				return domain.RoleAssignment{}, false, decodeErr
			}
			if !audit.MatchesChange(action, change) {
				return domain.RoleAssignment{}, false,
					fmt.Errorf("%w: role assignment operation %s", ErrConflict, change.OperationID)
			}
			return audit.Result, false, nil
		}
	}

	var current *domain.RoleAssignment
	if data, exists := m.roleAssignments[assignmentID]; exists {
		decoded, decodeErr := decodeRoleAssignment(data)
		if decodeErr != nil {
			return domain.RoleAssignment{}, false, decodeErr
		}
		if decoded.AssignmentID != assignmentID {
			return domain.RoleAssignment{}, false, errors.New("store: role assignment key mismatch")
		}
		current = &decoded
	}

	next, audit, err := applyRoleAssignmentChange(action, current, change)
	if err != nil {
		return domain.RoleAssignment{}, false, err
	}
	assignmentData, err := json.Marshal(next)
	if err != nil {
		return domain.RoleAssignment{}, false, fmt.Errorf("store: encode role assignment: %w", err)
	}
	auditData, err := json.Marshal(audit)
	if err != nil {
		return domain.RoleAssignment{}, false, fmt.Errorf("store: encode role assignment audit: %w", err)
	}
	m.roleAssignments[assignmentID] = bytes.Clone(assignmentData)
	if m.roleAudits[assignmentID] == nil {
		m.roleAudits[assignmentID] = make(map[string][]byte)
	}
	m.roleAudits[assignmentID][audit.AuditID] = bytes.Clone(auditData)
	return next, true, nil
}

// ListRoleAssignmentAudit returns immutable receipts ordered by assignment
// revision. It is an internal administrative boundary, not a public listing.
func (m *MemoryStore) ListRoleAssignmentAudit(
	ctx context.Context,
	assignmentID string,
) ([]domain.RoleAssignmentAudit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	auditsByID, exists := m.roleAudits[assignmentID]
	data := make([][]byte, 0, len(auditsByID))
	for _, audit := range auditsByID {
		data = append(data, bytes.Clone(audit))
	}
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: role assignment audit %s", ErrNotFound, assignmentID)
	}
	audits := make([]domain.RoleAssignmentAudit, 0, len(data))
	for _, item := range data {
		audit, err := decodeRoleAssignmentAudit(item)
		if err != nil {
			return nil, err
		}
		if audit.AssignmentID != assignmentID {
			return nil, errors.New("store: role assignment audit key mismatch")
		}
		audits = append(audits, audit)
	}
	if err := sortAndValidateRoleAssignmentAudits(audits); err != nil {
		return nil, err
	}
	return audits, nil
}

func sortAndValidateRoleAssignmentAudits(audits []domain.RoleAssignmentAudit) error {
	sort.Slice(audits, func(i, j int) bool {
		return audits[i].Result.Revision < audits[j].Result.Revision
	})
	if err := domain.ValidateRoleAssignmentAuditChain(audits); err != nil {
		return fmt.Errorf("store: invalid role assignment audit chain: %w", err)
	}
	return nil
}

func roleChangeIDs(change domain.RoleAssignmentChange) (string, string, error) {
	assignmentID, err := domain.RoleAssignmentID(change.Target, change.Role, change.Scope)
	if err != nil {
		return "", "", err
	}
	auditID, err := domain.RoleAssignmentAuditID(assignmentID, change.OperationID)
	if err != nil {
		return "", "", err
	}
	return assignmentID, auditID, nil
}

func applyRoleAssignmentChange(
	action domain.RoleAssignmentAction,
	current *domain.RoleAssignment,
	change domain.RoleAssignmentChange,
) (domain.RoleAssignment, domain.RoleAssignmentAudit, error) {
	switch action {
	case domain.RoleAssignmentActionGrant:
		return domain.GrantRoleAssignment(current, change)
	case domain.RoleAssignmentActionRevoke:
		return domain.RevokeRoleAssignment(current, change)
	default:
		return domain.RoleAssignment{}, domain.RoleAssignmentAudit{},
			fmt.Errorf("store: unsupported role assignment action %q", action)
	}
}

func decodeRoleAssignment(data []byte) (domain.RoleAssignment, error) {
	var assignment domain.RoleAssignment
	if err := decodeStrictRoleJSON(data, &assignment); err != nil {
		return domain.RoleAssignment{}, fmt.Errorf("store: decode role assignment: %w", err)
	}
	if err := assignment.Validate(); err != nil {
		return domain.RoleAssignment{}, fmt.Errorf("store: validate role assignment: %w", err)
	}
	return assignment, nil
}

func decodeRoleAssignmentAudit(data []byte) (domain.RoleAssignmentAudit, error) {
	var audit domain.RoleAssignmentAudit
	if err := decodeStrictRoleJSON(data, &audit); err != nil {
		return domain.RoleAssignmentAudit{}, fmt.Errorf("store: decode role assignment audit: %w", err)
	}
	if err := audit.Validate(); err != nil {
		return domain.RoleAssignmentAudit{}, fmt.Errorf("store: validate role assignment audit: %w", err)
	}
	return audit, nil
}

func decodeStrictRoleJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
