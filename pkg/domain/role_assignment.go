package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	RoleAssignmentVersion = 1

	maxRoleScopeIDRunes   = 128
	maxRoleOperationRunes = 128
)

var (
	ErrRoleAlreadyActive     = errors.New("domain: role assignment is already active")
	ErrRoleAssignmentMissing = errors.New("domain: role assignment does not exist")
	ErrRoleAlreadyRevoked    = errors.New("domain: role assignment is already revoked")
)

// Role identifies one durable application authorization. Authentication and
// contextual reporter/finder ownership remain separate concerns.
type Role string

const RoleOperator Role = "operator"

// RoleScopeKind distinguishes an application-wide role from one constrained
// to a shelter. Shelter authorization is not effective until a resource has a
// trusted shelter association.
type RoleScopeKind string

const (
	RoleScopeGlobal  RoleScopeKind = "global"
	RoleScopeShelter RoleScopeKind = "shelter"
)

// RoleScope is an explicit, bounded authorization scope.
type RoleScope struct {
	Kind      RoleScopeKind `json:"kind"`
	ShelterID string        `json:"shelterId,omitempty"`
}

// RoleAssignmentStatus preserves revocation tombstones rather than deleting
// authorization history.
type RoleAssignmentStatus string

const (
	RoleAssignmentStatusActive  RoleAssignmentStatus = "active"
	RoleAssignmentStatusRevoked RoleAssignmentStatus = "revoked"
)

// RoleAssignmentAction identifies one immutable authorization audit event.
type RoleAssignmentAction string

const (
	RoleAssignmentActionGrant  RoleAssignmentAction = "grant"
	RoleAssignmentActionRevoke RoleAssignmentAction = "revoke"
)

// RoleAssignmentChange is a trusted command input. Raw provider principals are
// converted to opaque digests before anything is persisted.
type RoleAssignmentChange struct {
	Target      PrincipalRef
	Role        Role
	Scope       RoleScope
	Actor       PrincipalRef
	OperationID string
	OccurredAt  time.Time
}

// RoleAssignment is the private, provider-neutral persisted authorization
// record. It intentionally contains no issuer, subject, email, or display name.
type RoleAssignment struct {
	Version      int                  `json:"version"`
	AssignmentID string               `json:"assignmentId"`
	PrincipalKey string               `json:"principalKey"`
	Role         Role                 `json:"role"`
	Scope        RoleScope            `json:"scope"`
	Status       RoleAssignmentStatus `json:"status"`
	Revision     int64                `json:"revision"`
	GrantedByKey string               `json:"grantedByKey"`
	GrantedAt    time.Time            `json:"grantedAt"`
	RevokedByKey string               `json:"revokedByKey,omitempty"`
	RevokedAt    *time.Time           `json:"revokedAt,omitempty"`
}

// RoleAssignmentAudit is an immutable, idempotency-keyed operation receipt.
// Result captures the original outcome so a retry remains stable even after a
// later grant or revocation changes the current assignment.
type RoleAssignmentAudit struct {
	Version      int                  `json:"version"`
	AuditID      string               `json:"auditId"`
	AssignmentID string               `json:"assignmentId"`
	OperationID  string               `json:"operationId"`
	Action       RoleAssignmentAction `json:"action"`
	ActorKey     string               `json:"actorKey"`
	OccurredAt   time.Time            `json:"occurredAt"`
	Result       RoleAssignment       `json:"result"`
}

// RolePrincipalKey returns a stable opaque key while preserving the provider's
// subject as exact opaque data. Issuers are canonicalized by trimming their
// surrounding whitespace, matching PrincipalRef persistence elsewhere.
func RolePrincipalKey(principal PrincipalRef) (string, error) {
	principal.Issuer = strings.TrimSpace(principal.Issuer)
	if err := principal.Validate(); err != nil {
		return "", fmt.Errorf("domain: invalid role principal: %w", err)
	}
	return stableRoleDigest("role-principal-v1", "role_principal_v1_", principal.Issuer, principal.Subject), nil
}

// RoleAssignmentID returns the document key for one principal-role-scope
// tuple. One principal may therefore hold independent shelter assignments.
func RoleAssignmentID(principal PrincipalRef, role Role, scope RoleScope) (string, error) {
	principalKey, err := RolePrincipalKey(principal)
	if err != nil {
		return "", err
	}
	if err := validateRole(role); err != nil {
		return "", err
	}
	if err := scope.Validate(); err != nil {
		return "", err
	}
	return roleAssignmentIDFromKey(principalKey, role, scope), nil
}

// RoleAssignmentAuditID returns the immutable operation receipt key within one
// assignment's audit log.
func RoleAssignmentAuditID(assignmentID, operationID string) (string, error) {
	if !validRoleDigest(assignmentID, "role_assignment_v1_") {
		return "", errors.New("domain: invalid role assignment ID")
	}
	if err := validateCanonicalRoleText("operation ID", operationID, maxRoleOperationRunes); err != nil {
		return "", err
	}
	return roleAuditID(assignmentID, operationID), nil
}

// Validate requires one explicit supported scope.
func (s RoleScope) Validate() error {
	switch s.Kind {
	case RoleScopeGlobal:
		if s.ShelterID != "" {
			return errors.New("domain: global role scope must not have a shelter ID")
		}
	case RoleScopeShelter:
		if err := validateCanonicalRoleText("shelter ID", s.ShelterID, maxRoleScopeIDRunes); err != nil {
			return err
		}
	default:
		return fmt.Errorf("domain: unsupported role scope %q", s.Kind)
	}
	return nil
}

// Validate requires a complete internally consistent assignment record.
func (a RoleAssignment) Validate() error {
	if a.Version != RoleAssignmentVersion {
		return fmt.Errorf("domain: unsupported role assignment version %d", a.Version)
	}
	if !validRoleDigest(a.PrincipalKey, "role_principal_v1_") {
		return errors.New("domain: invalid role assignment principal key")
	}
	if err := validateRole(a.Role); err != nil {
		return err
	}
	if err := a.Scope.Validate(); err != nil {
		return err
	}
	wantID := roleAssignmentIDFromKey(a.PrincipalKey, a.Role, a.Scope)
	if a.AssignmentID != wantID {
		return errors.New("domain: role assignment ID does not match its principal, role, and scope")
	}
	if a.Revision < 1 || !validRoleDigest(a.GrantedByKey, "role_principal_v1_") || a.GrantedAt.IsZero() {
		return errors.New("domain: role assignment grant metadata is incomplete")
	}
	if a.GrantedAt.Location() != time.UTC {
		return errors.New("domain: role assignment grant time must be UTC")
	}
	switch a.Status {
	case RoleAssignmentStatusActive:
		if a.Revision%2 == 0 {
			return errors.New("domain: active role assignment revision must be odd")
		}
		if a.RevokedByKey != "" || a.RevokedAt != nil {
			return errors.New("domain: active role assignment has revocation metadata")
		}
	case RoleAssignmentStatusRevoked:
		if a.Revision%2 != 0 {
			return errors.New("domain: revoked role assignment revision must be even")
		}
		if !validRoleDigest(a.RevokedByKey, "role_principal_v1_") || a.RevokedAt == nil || a.RevokedAt.IsZero() {
			return errors.New("domain: revoked role assignment metadata is incomplete")
		}
		if a.RevokedAt.Location() != time.UTC || a.RevokedAt.Before(a.GrantedAt) {
			return errors.New("domain: invalid role assignment revocation time")
		}
	default:
		return fmt.Errorf("domain: unsupported role assignment status %q", a.Status)
	}
	return nil
}

// Validate requires an immutable audit receipt that agrees with its result.
func (a RoleAssignmentAudit) Validate() error {
	if a.Version != RoleAssignmentVersion {
		return fmt.Errorf("domain: unsupported role assignment audit version %d", a.Version)
	}
	if err := validateCanonicalRoleText("operation ID", a.OperationID, maxRoleOperationRunes); err != nil {
		return err
	}
	if !validRoleDigest(a.ActorKey, "role_principal_v1_") || a.OccurredAt.IsZero() || a.OccurredAt.Location() != time.UTC {
		return errors.New("domain: role assignment audit actor and time are required")
	}
	if err := a.Result.Validate(); err != nil {
		return fmt.Errorf("domain: invalid role assignment audit result: %w", err)
	}
	if a.AssignmentID != a.Result.AssignmentID {
		return errors.New("domain: role assignment audit references a different assignment")
	}
	wantAuditID := roleAuditID(a.AssignmentID, a.OperationID)
	if a.AuditID != wantAuditID {
		return errors.New("domain: role assignment audit ID does not match its operation")
	}
	switch a.Action {
	case RoleAssignmentActionGrant:
		if a.Result.Status != RoleAssignmentStatusActive || a.Result.GrantedByKey != a.ActorKey ||
			!a.Result.GrantedAt.Equal(a.OccurredAt) {
			return errors.New("domain: grant audit does not match its result")
		}
	case RoleAssignmentActionRevoke:
		if a.Result.Status != RoleAssignmentStatusRevoked || a.Result.RevokedAt == nil ||
			a.Result.RevokedByKey != a.ActorKey || !a.Result.RevokedAt.Equal(a.OccurredAt) {
			return errors.New("domain: revoke audit does not match its result")
		}
	default:
		return fmt.Errorf("domain: unsupported role assignment action %q", a.Action)
	}
	return nil
}

// ValidateRoleAssignmentAuditChain requires immutable receipts to describe the
// exact grant, revoke, regrant state machine rather than merely valid isolated
// snapshots with contiguous revision numbers.
func ValidateRoleAssignmentAuditChain(audits []RoleAssignmentAudit) error {
	if len(audits) == 0 {
		return errors.New("domain: role assignment audit chain is empty")
	}
	seen := make(map[string]struct{}, len(audits))
	for index := range audits {
		current := audits[index]
		if err := current.Validate(); err != nil {
			return fmt.Errorf("domain: invalid role assignment audit revision %d: %w", index+1, err)
		}
		wantRevision := int64(index + 1)
		if current.Result.Revision != wantRevision {
			return fmt.Errorf("domain: role assignment audit revision %d is missing or duplicated", wantRevision)
		}
		if _, exists := seen[current.AuditID]; exists {
			return errors.New("domain: duplicate role assignment audit operation")
		}
		seen[current.AuditID] = struct{}{}
		if index == 0 {
			if current.Action != RoleAssignmentActionGrant {
				return errors.New("domain: role assignment audit chain must begin with a grant")
			}
			continue
		}

		previous := audits[index-1]
		if !sameRoleAssignmentTuple(previous.Result, current.Result) {
			return errors.New("domain: role assignment audit chain changes its assignment tuple")
		}
		switch previous.Result.Status {
		case RoleAssignmentStatusActive:
			if current.Action != RoleAssignmentActionRevoke ||
				current.Result.GrantedByKey != previous.Result.GrantedByKey ||
				!current.Result.GrantedAt.Equal(previous.Result.GrantedAt) {
				return errors.New("domain: role revocation audit does not preserve its grant")
			}
		case RoleAssignmentStatusRevoked:
			if current.Action != RoleAssignmentActionGrant || previous.Result.RevokedAt == nil ||
				current.OccurredAt.Before(*previous.Result.RevokedAt) {
				return errors.New("domain: role regrant audit does not follow its revocation")
			}
		default:
			return errors.New("domain: role assignment audit chain has invalid prior status")
		}
	}
	return nil
}

// MatchesChange reports whether an existing operation receipt is an exact
// retry of a trusted change command.
func (a RoleAssignmentAudit) MatchesChange(action RoleAssignmentAction, change RoleAssignmentChange) bool {
	normalized, principalKey, actorKey, err := normalizeRoleAssignmentChange(change)
	if err != nil {
		return false
	}
	assignmentID := roleAssignmentIDFromKey(principalKey, normalized.Role, normalized.Scope)
	return a.Action == action && a.AssignmentID == assignmentID && a.OperationID == normalized.OperationID &&
		a.ActorKey == actorKey && a.OccurredAt.Equal(normalized.OccurredAt)
}

// GrantRoleAssignment applies one trusted grant or regrant transition.
func GrantRoleAssignment(current *RoleAssignment, change RoleAssignmentChange) (RoleAssignment, RoleAssignmentAudit, error) {
	normalized, principalKey, actorKey, err := normalizeRoleAssignmentChange(change)
	if err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, err
	}
	assignmentID := roleAssignmentIDFromKey(principalKey, normalized.Role, normalized.Scope)
	revision := int64(1)
	if current != nil {
		if err := current.Validate(); err != nil {
			return RoleAssignment{}, RoleAssignmentAudit{}, fmt.Errorf("domain: invalid current role assignment: %w", err)
		}
		if current.AssignmentID != assignmentID {
			return RoleAssignment{}, RoleAssignmentAudit{}, errors.New("domain: current role assignment does not match grant target")
		}
		if current.Status == RoleAssignmentStatusActive {
			return RoleAssignment{}, RoleAssignmentAudit{}, ErrRoleAlreadyActive
		}
		if current.Revision == math.MaxInt64 {
			return RoleAssignment{}, RoleAssignmentAudit{}, errors.New("domain: role assignment revision is exhausted")
		}
		if current.RevokedAt != nil && normalized.OccurredAt.Before(*current.RevokedAt) {
			return RoleAssignment{}, RoleAssignmentAudit{}, errors.New("domain: role regrant precedes its revocation")
		}
		revision = current.Revision + 1
	}
	next := RoleAssignment{
		Version:      RoleAssignmentVersion,
		AssignmentID: assignmentID,
		PrincipalKey: principalKey,
		Role:         normalized.Role,
		Scope:        normalized.Scope,
		Status:       RoleAssignmentStatusActive,
		Revision:     revision,
		GrantedByKey: actorKey,
		GrantedAt:    normalized.OccurredAt,
	}
	audit := newRoleAssignmentAudit(RoleAssignmentActionGrant, normalized, actorKey, next)
	if err := next.Validate(); err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, err
	}
	if err := audit.Validate(); err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, err
	}
	return next, audit, nil
}

// RevokeRoleAssignment applies one trusted revocation transition.
func RevokeRoleAssignment(current *RoleAssignment, change RoleAssignmentChange) (RoleAssignment, RoleAssignmentAudit, error) {
	normalized, principalKey, actorKey, err := normalizeRoleAssignmentChange(change)
	if err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, err
	}
	if current == nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, ErrRoleAssignmentMissing
	}
	if err := current.Validate(); err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, fmt.Errorf("domain: invalid current role assignment: %w", err)
	}
	assignmentID := roleAssignmentIDFromKey(principalKey, normalized.Role, normalized.Scope)
	if current.AssignmentID != assignmentID {
		return RoleAssignment{}, RoleAssignmentAudit{}, errors.New("domain: current role assignment does not match revoke target")
	}
	if current.Status == RoleAssignmentStatusRevoked {
		return RoleAssignment{}, RoleAssignmentAudit{}, ErrRoleAlreadyRevoked
	}
	if current.Revision == math.MaxInt64 {
		return RoleAssignment{}, RoleAssignmentAudit{}, errors.New("domain: role assignment revision is exhausted")
	}
	if normalized.OccurredAt.Before(current.GrantedAt) {
		return RoleAssignment{}, RoleAssignmentAudit{}, errors.New("domain: role revocation precedes its grant")
	}
	next := *current
	next.Status = RoleAssignmentStatusRevoked
	next.Revision++
	next.RevokedByKey = actorKey
	revokedAt := normalized.OccurredAt
	next.RevokedAt = &revokedAt
	audit := newRoleAssignmentAudit(RoleAssignmentActionRevoke, normalized, actorKey, next)
	if err := next.Validate(); err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, err
	}
	if err := audit.Validate(); err != nil {
		return RoleAssignment{}, RoleAssignmentAudit{}, err
	}
	return next, audit, nil
}

func normalizeRoleAssignmentChange(change RoleAssignmentChange) (RoleAssignmentChange, string, string, error) {
	change.Target.Issuer = strings.TrimSpace(change.Target.Issuer)
	change.Actor.Issuer = strings.TrimSpace(change.Actor.Issuer)
	if err := change.Target.Validate(); err != nil {
		return RoleAssignmentChange{}, "", "", fmt.Errorf("domain: invalid role target: %w", err)
	}
	if err := change.Actor.Validate(); err != nil {
		return RoleAssignmentChange{}, "", "", fmt.Errorf("domain: invalid role actor: %w", err)
	}
	if err := validateRole(change.Role); err != nil {
		return RoleAssignmentChange{}, "", "", err
	}
	if err := change.Scope.Validate(); err != nil {
		return RoleAssignmentChange{}, "", "", err
	}
	if err := validateCanonicalRoleText("operation ID", change.OperationID, maxRoleOperationRunes); err != nil {
		return RoleAssignmentChange{}, "", "", err
	}
	if change.OccurredAt.IsZero() {
		return RoleAssignmentChange{}, "", "", errors.New("domain: role assignment change time is required")
	}
	change.OccurredAt = change.OccurredAt.UTC()
	principalKey, err := RolePrincipalKey(change.Target)
	if err != nil {
		return RoleAssignmentChange{}, "", "", err
	}
	actorKey, err := RolePrincipalKey(change.Actor)
	if err != nil {
		return RoleAssignmentChange{}, "", "", err
	}
	return change, principalKey, actorKey, nil
}

func validateRole(role Role) error {
	if role != RoleOperator {
		return fmt.Errorf("domain: unsupported role %q", role)
	}
	return nil
}

func sameRoleAssignmentTuple(left, right RoleAssignment) bool {
	return left.Version == right.Version && left.AssignmentID == right.AssignmentID &&
		left.PrincipalKey == right.PrincipalKey && left.Role == right.Role && left.Scope == right.Scope
}

func validateCanonicalRoleText(name, value string, limit int) error {
	if !utf8.ValidString(value) || value == "" || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("domain: %s must be canonical bounded valid UTF-8", name)
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return fmt.Errorf("domain: %s must not contain control characters", name)
		}
	}
	return nil
}

func roleAssignmentIDFromKey(principalKey string, role Role, scope RoleScope) string {
	return stableRoleDigest(
		"role-assignment-v1",
		"role_assignment_v1_",
		principalKey,
		string(role),
		string(scope.Kind),
		scope.ShelterID,
	)
}

func roleAuditID(assignmentID, operationID string) string {
	return stableRoleDigest("role-assignment-audit-v1", "role_audit_v1_", assignmentID, operationID)
}

func newRoleAssignmentAudit(
	action RoleAssignmentAction,
	change RoleAssignmentChange,
	actorKey string,
	result RoleAssignment,
) RoleAssignmentAudit {
	return RoleAssignmentAudit{
		Version:      RoleAssignmentVersion,
		AuditID:      roleAuditID(result.AssignmentID, change.OperationID),
		AssignmentID: result.AssignmentID,
		OperationID:  change.OperationID,
		Action:       action,
		ActorKey:     actorKey,
		OccurredAt:   change.OccurredAt,
		Result:       result,
	}
}

func stableRoleDigest(domainTag, prefix string, fields ...string) string {
	digest := sha256.New()
	writeLengthPrefixed(digest, domainTag)
	for _, field := range fields {
		writeLengthPrefixed(digest, field)
	}
	return prefix + hex.EncodeToString(digest.Sum(nil))
}

func writeLengthPrefixed(digest hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}

func validRoleDigest(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil
}
