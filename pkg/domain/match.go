package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// MatchStatus is the review lifecycle of a persisted candidate match.
type MatchStatus string

const (
	MatchStatusPendingReview MatchStatus = "PENDING_REVIEW"
	MatchStatusConfirmed     MatchStatus = "CONFIRMED"
	MatchStatusRejected      MatchStatus = "REJECTED"
	MatchStatusReunited      MatchStatus = "REUNITED"
)

// MatchDecision is one participant's immutable review decision.
type MatchDecision string

const (
	MatchDecisionConfirm MatchDecision = "CONFIRM"
	MatchDecisionReject  MatchDecision = "REJECT"
)

// MatchParticipantRole identifies which report owner made a match decision.
type MatchParticipantRole string

const (
	MatchParticipantRoleReporter MatchParticipantRole = "reporter"
	MatchParticipantRoleFinder   MatchParticipantRole = "finder"
)

var (
	ErrNotMatchParticipant         = errors.New("domain: principal is not a match participant")
	ErrIncompleteMatchParticipants = errors.New("domain: bilateral match participants are required")
	ErrMatchDecisionConflict       = errors.New("domain: match decision is immutable")
)

// MatchScoreBreakdown preserves the components and threshold used to rank a
// candidate so the combined score remains explainable after model changes.
type MatchScoreBreakdown struct {
	Visual        float64 `json:"visual"`
	Color         float64 `json:"color"`
	Spatial       float64 `json:"spatial"`
	DistanceMiles float64 `json:"distanceMiles"`
	Threshold     float64 `json:"threshold,omitempty"`
}

// MatchPetDetail is the immutable report snapshot presented with a match.
type MatchPetDetail struct {
	PetID    string `json:"petId"`
	PetName  string `json:"petName,omitempty"`
	Breed    string `json:"breed"`
	ImageURL string `json:"imageUrl"`
	Location string `json:"location"`
}

// MatchRecord is the canonical durable candidate match. It is written before
// matchFound publication and contains no reporter or finder contact.
type MatchRecord struct {
	MatchID          string              `json:"matchId"`
	FoundPetID       string              `json:"foundPetId"`
	MatchedPetID     string              `json:"matchedPetId"`
	Score            float64             `json:"score"`
	Status           MatchStatus         `json:"status"`
	MatchedAt        time.Time           `json:"matchedAt"`
	Scores           MatchScoreBreakdown `json:"scores"`
	LostPet          MatchPetDetail      `json:"lostPet"`
	FoundPet         MatchPetDetail      `json:"foundPet"`
	SourceEventID    string              `json:"sourceEventId,omitempty"`
	Model            string              `json:"model,omitempty"`
	ThresholdVersion string              `json:"thresholdVersion,omitempty"`
	Explanation      string              `json:"explanation,omitempty"`
}

// MatchParticipantRecord is the private authorization link between a match
// and the trusted principals that own its lost and found reports. It is stored
// separately from MatchRecord so public match responses cannot expose identity
// provider subjects. Legacy or mixed-auth matches may have only one owner.
type MatchParticipantRecord struct {
	MatchID          string                 `json:"matchId"`
	LostPetID        string                 `json:"lostPetId"`
	FoundPetID       string                 `json:"foundPetId"`
	Reporter         *PrincipalRef          `json:"reporter,omitempty"`
	Finder           *PrincipalRef          `json:"finder,omitempty"`
	ReporterDecision MatchDecision          `json:"reporterDecision,omitempty"`
	FinderDecision   MatchDecision          `json:"finderDecision,omitempty"`
	DecisionAudit    []MatchDecisionAudit   `json:"decisionAudit,omitempty"`
	Messages         []MediatedMatchMessage `json:"messages,omitempty"`
}

// MatchDecisionAudit records the first accepted decision for one participant
// role. It remains in the private participant document.
type MatchDecisionAudit struct {
	Role      MatchParticipantRole `json:"role"`
	Decision  MatchDecision        `json:"decision"`
	DecidedAt time.Time            `json:"decidedAt"`
}

// Validate requires a match identity and at least one valid participant.
func (r MatchParticipantRecord) Validate() error {
	if strings.TrimSpace(r.MatchID) == "" || strings.TrimSpace(r.LostPetID) == "" ||
		strings.TrimSpace(r.FoundPetID) == "" {
		return errors.New("domain: match participant IDs are required")
	}
	if r.Reporter == nil && r.Finder == nil {
		return errors.New("domain: at least one match participant is required")
	}
	if r.Reporter != nil {
		if err := r.Reporter.Validate(); err != nil {
			return fmt.Errorf("domain: invalid match reporter: %w", err)
		}
	}
	if r.Finder != nil {
		if err := r.Finder.Validate(); err != nil {
			return fmt.Errorf("domain: invalid match finder: %w", err)
		}
	}
	if err := validateStoredMatchDecision(r.Reporter, r.ReporterDecision, MatchParticipantRoleReporter); err != nil {
		return err
	}
	if err := validateStoredMatchDecision(r.Finder, r.FinderDecision, MatchParticipantRoleFinder); err != nil {
		return err
	}
	seenAudit := map[MatchParticipantRole]bool{}
	for _, audit := range r.DecisionAudit {
		if audit.Role != MatchParticipantRoleReporter && audit.Role != MatchParticipantRoleFinder {
			return fmt.Errorf("domain: unsupported match participant role %q", audit.Role)
		}
		if seenAudit[audit.Role] || !validMatchDecision(audit.Decision) || audit.DecidedAt.IsZero() {
			return errors.New("domain: invalid match decision audit")
		}
		seenAudit[audit.Role] = true
		if audit.Role == MatchParticipantRoleReporter && audit.Decision != r.ReporterDecision {
			return errors.New("domain: reporter decision audit does not match decision")
		}
		if audit.Role == MatchParticipantRoleFinder && audit.Decision != r.FinderDecision {
			return errors.New("domain: finder decision audit does not match decision")
		}
	}
	if (r.ReporterDecision != "") != seenAudit[MatchParticipantRoleReporter] ||
		(r.FinderDecision != "") != seenAudit[MatchParticipantRoleFinder] {
		return errors.New("domain: every match decision requires one audit record")
	}
	if err := validateMediatedMatchMessages(r.Reporter, r.Finder, r.Messages); err != nil {
		return err
	}
	return nil
}

// ApplyDecision returns a copied participant record with the actor's first
// decision applied. Exact retries are no-ops and changed decisions conflict.
func (r MatchParticipantRecord) ApplyDecision(
	actor PrincipalRef,
	decision MatchDecision,
	decidedAt time.Time,
) (MatchParticipantRecord, MatchStatus, bool, error) {
	if err := r.Validate(); err != nil {
		return MatchParticipantRecord{}, "", false, err
	}
	if r.Reporter == nil || r.Finder == nil {
		return MatchParticipantRecord{}, "", false, ErrIncompleteMatchParticipants
	}
	if err := actor.Validate(); err != nil {
		return MatchParticipantRecord{}, "", false, fmt.Errorf("domain: invalid match decision actor: %w", err)
	}
	if !validMatchDecision(decision) || decidedAt.IsZero() {
		return MatchParticipantRecord{}, "", false, errors.New("domain: match decision and decidedAt are required")
	}
	reporter := principalRefsEqual(r.Reporter, actor)
	finder := principalRefsEqual(r.Finder, actor)
	if !reporter && !finder {
		return MatchParticipantRecord{}, "", false, ErrNotMatchParticipant
	}
	if reporter && r.ReporterDecision != "" && r.ReporterDecision != decision {
		return MatchParticipantRecord{}, "", false, ErrMatchDecisionConflict
	}
	if finder && r.FinderDecision != "" && r.FinderDecision != decision {
		return MatchParticipantRecord{}, "", false, ErrMatchDecisionConflict
	}

	next := r
	next.DecisionAudit = append([]MatchDecisionAudit(nil), r.DecisionAudit...)
	changed := false
	if reporter && next.ReporterDecision == "" {
		next.ReporterDecision = decision
		next.DecisionAudit = append(next.DecisionAudit, MatchDecisionAudit{
			Role: MatchParticipantRoleReporter, Decision: decision, DecidedAt: decidedAt,
		})
		changed = true
	}
	if finder && next.FinderDecision == "" {
		next.FinderDecision = decision
		next.DecisionAudit = append(next.DecisionAudit, MatchDecisionAudit{
			Role: MatchParticipantRoleFinder, Decision: decision, DecidedAt: decidedAt,
		})
		changed = true
	}
	return next, next.decisionStatus(), changed, nil
}

func (r MatchParticipantRecord) decisionStatus() MatchStatus {
	if r.ReporterDecision == MatchDecisionReject || r.FinderDecision == MatchDecisionReject {
		return MatchStatusRejected
	}
	if r.ReporterDecision == MatchDecisionConfirm && r.FinderDecision == MatchDecisionConfirm {
		return MatchStatusConfirmed
	}
	return MatchStatusPendingReview
}

func validateStoredMatchDecision(owner *PrincipalRef, decision MatchDecision, role MatchParticipantRole) error {
	if decision == "" {
		return nil
	}
	if owner == nil || !validMatchDecision(decision) {
		return fmt.Errorf("domain: invalid %s match decision", role)
	}
	return nil
}

func validMatchDecision(decision MatchDecision) bool {
	return decision == MatchDecisionConfirm || decision == MatchDecisionReject
}

func principalRefsEqual(expected *PrincipalRef, actual PrincipalRef) bool {
	return expected != nil && expected.Issuer == actual.Issuer && expected.Subject == actual.Subject
}

// StableMatchID derives one idempotency key for a source report and candidate.
func StableMatchID(sourceEventID, foundPetID, lostPetID string) (string, error) {
	sourceEventID = strings.TrimSpace(sourceEventID)
	foundPetID = strings.TrimSpace(foundPetID)
	lostPetID = strings.TrimSpace(lostPetID)
	if sourceEventID == "" || foundPetID == "" || lostPetID == "" {
		return "", errors.New("domain: match source and pet IDs are required")
	}
	digest := sha256.Sum256([]byte(sourceEventID + "\x00" + foundPetID + "\x00" + lostPetID))
	return "match_" + hex.EncodeToString(digest[:]), nil
}

// Validate checks the persisted match and its scoring provenance.
func (m MatchRecord) Validate() error {
	if strings.TrimSpace(m.MatchID) == "" || strings.TrimSpace(m.SourceEventID) == "" {
		return errors.New("domain: matchId and sourceEventId are required")
	}
	if strings.TrimSpace(m.FoundPetID) == "" || strings.TrimSpace(m.MatchedPetID) == "" {
		return errors.New("domain: both foundPetId and matchedPetId are required")
	}
	if m.FoundPet.PetID != m.FoundPetID || m.LostPet.PetID != m.MatchedPetID {
		return errors.New("domain: match pet snapshots do not match record IDs")
	}
	wantID, err := StableMatchID(m.SourceEventID, m.FoundPetID, m.MatchedPetID)
	if err != nil || m.MatchID != wantID {
		return errors.New("domain: matchId does not match source and pet IDs")
	}
	if math.IsNaN(m.Score) || math.IsInf(m.Score, 0) || m.Score < 0 || m.Score > 1 {
		return fmt.Errorf("domain: match score must be between 0 and 1, got %f", m.Score)
	}
	if m.MatchedAt.IsZero() {
		return errors.New("domain: matchedAt is required")
	}
	switch m.Status {
	case MatchStatusPendingReview, MatchStatusConfirmed, MatchStatusRejected, MatchStatusReunited:
	default:
		return fmt.Errorf("domain: unsupported match status %q", m.Status)
	}
	components := []struct {
		name  string
		value float64
	}{
		{name: "visual", value: m.Scores.Visual},
		{name: "color", value: m.Scores.Color},
		{name: "spatial", value: m.Scores.Spatial},
		{name: "threshold", value: m.Scores.Threshold},
	}
	for _, component := range components {
		if math.IsNaN(component.value) || math.IsInf(component.value, 0) || component.value < 0 || component.value > 1 {
			return fmt.Errorf("domain: %s score must be between 0 and 1, got %f", component.name, component.value)
		}
	}
	if m.Scores.Threshold == 0 {
		return errors.New("domain: match threshold is required")
	}
	if math.IsNaN(m.Scores.DistanceMiles) || math.IsInf(m.Scores.DistanceMiles, 0) || m.Scores.DistanceMiles < 0 {
		return errors.New("domain: distanceMiles cannot be negative")
	}
	if strings.TrimSpace(m.Model) == "" || strings.TrimSpace(m.ThresholdVersion) == "" ||
		strings.TrimSpace(m.Explanation) == "" {
		return errors.New("domain: model, thresholdVersion, and explanation are required")
	}
	return nil
}
