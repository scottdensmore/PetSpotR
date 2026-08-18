package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const idempotencyKeyHeader = "Idempotency-Key"

var (
	errMediatedThreadHidden = errors.New("webfrontend: mediated thread is not visible to principal")
	errMediatedThreadClosed = errors.New("webfrontend: mediated thread is read-only")
)

type mediatedThreadResponse struct {
	MatchID  string                        `json:"matchId"`
	Messages []domain.MediatedMatchMessage `json:"messages"`
}

type mediatedMessageResponse struct {
	Status  string                      `json:"status"`
	MatchID string                      `json:"matchId"`
	Message domain.MediatedMatchMessage `json:"message"`
}

func (s *Server) handleAuthenticatedMediatedContact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, ok := s.verifiedRequestPrincipal(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		s.handleMediatedThreadRead(w, r, principal)
		return
	}
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	s.handleMediatedMessageCreate(w, r, principal)
}

func (s *Server) handleMediatedThreadRead(w http.ResponseWriter, r *http.Request, principal identity.Principal) {
	matchID := strings.TrimSpace(r.URL.Query().Get("matchId"))
	if matchID == "" {
		http.Error(w, "matchId is required", http.StatusBadRequest)
		return
	}
	participants, err := s.loadAuthorizedMediatedThread(r.Context(), matchID, principal)
	if errors.Is(err, errMediatedThreadHidden) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load mediated thread", http.StatusInternalServerError)
		return
	}
	messages := make([]domain.MediatedMatchMessage, len(participants.Messages))
	copy(messages, participants.Messages)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mediatedThreadResponse{MatchID: matchID, Messages: messages})
}

func (s *Server) loadAuthorizedMediatedThread(
	ctx context.Context,
	matchID string,
	principal identity.Principal,
) (domain.MatchParticipantRecord, error) {
	matchData, err := s.stateStore.GetState(ctx, store.MatchesCollection, matchID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		return domain.MatchParticipantRecord{}, errMediatedThreadHidden
	}
	if err != nil {
		return domain.MatchParticipantRecord{}, err
	}
	participantData, err := s.stateStore.GetState(ctx, store.MatchParticipantsCollection, matchID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		return domain.MatchParticipantRecord{}, errMediatedThreadHidden
	}
	if err != nil {
		return domain.MatchParticipantRecord{}, err
	}
	var match domain.MatchRecord
	if err := json.Unmarshal(matchData, &match); err != nil {
		return domain.MatchParticipantRecord{}, err
	}
	var participants domain.MatchParticipantRecord
	if err := json.Unmarshal(participantData, &participants); err != nil {
		return domain.MatchParticipantRecord{}, errMediatedThreadHidden
	}
	if !validMediatedThreadBinding(matchID, match, participants) ||
		participants.Reporter == nil || participants.Finder == nil ||
		(!principalMatchesRef(principal, participants.Reporter) && !principalMatchesRef(principal, participants.Finder)) {
		return domain.MatchParticipantRecord{}, errMediatedThreadHidden
	}
	return participants, nil
}

func (s *Server) handleMediatedMessageCreate(w http.ResponseWriter, r *http.Request, principal identity.Principal) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req ReunionContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	req.MatchID = strings.TrimSpace(req.MatchID)
	if req.MatchID == "" || strings.TrimSpace(req.Message) == "" || strings.TrimSpace(r.Header.Get(idempotencyKeyHeader)) == "" {
		http.Error(w, "matchId, message, and Idempotency-Key are required", http.StatusBadRequest)
		return
	}

	matchStore, ok := s.stateStore.(store.MatchStateStore)
	if !ok {
		http.Error(w, "Mediated thread storage is unavailable", http.StatusInternalServerError)
		return
	}
	actor := domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject}
	sentAt := time.Now().UTC()
	var accepted domain.MediatedMatchMessage
	created := false
	err := matchStore.UpdateMatchAndParticipants(
		r.Context(), req.MatchID,
		func(matchData, participantData []byte) ([]byte, []byte, error) {
			var match domain.MatchRecord
			if err := json.Unmarshal(matchData, &match); err != nil {
				return nil, nil, fmt.Errorf("decode match: %w", err)
			}
			var participants domain.MatchParticipantRecord
			if err := json.Unmarshal(participantData, &participants); err != nil {
				return nil, nil, errMediatedThreadHidden
			}
			if !validMediatedThreadBinding(req.MatchID, match, participants) ||
				participants.Reporter == nil || participants.Finder == nil ||
				(!principalMatchesRef(principal, participants.Reporter) &&
					!principalMatchesRef(principal, participants.Finder)) {
				return nil, nil, errMediatedThreadHidden
			}
			next, message, wasCreated, err := participants.AppendMediatedMessage(
				actor, r.Header.Get(idempotencyKeyHeader), req.Message, sentAt,
			)
			if errors.Is(err, domain.ErrNotMatchParticipant) || errors.Is(err, domain.ErrIncompleteMatchParticipants) {
				return nil, nil, errMediatedThreadHidden
			}
			if err != nil {
				return nil, nil, err
			}
			if wasCreated && match.Status != domain.MatchStatusPendingReview && match.Status != domain.MatchStatusConfirmed {
				return nil, nil, errMediatedThreadClosed
			}
			nextParticipantData, err := json.Marshal(next)
			if err != nil {
				return nil, nil, err
			}
			accepted = message
			created = wasCreated
			return matchData, nextParticipantData, nil
		},
	)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) ||
		errors.Is(err, errMediatedThreadHidden) {
		http.NotFound(w, r)
		return
	}
	if errors.Is(err, errMediatedThreadClosed) || errors.Is(err, domain.ErrMatchMessageConflict) ||
		errors.Is(err, domain.ErrMatchThreadFull) || errors.Is(err, domain.ErrNoMediatedMessageRecipient) {
		http.Error(w, "Mediated message conflicts with the match thread", http.StatusConflict)
		return
	}
	if errors.Is(err, domain.ErrInvalidMediatedMessage) {
		http.Error(w, "Invalid mediated message", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Failed to accept mediated message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(mediatedMessageResponse{
		Status: "accepted", MatchID: req.MatchID, Message: accepted,
	})
}

func validMediatedThreadBinding(
	matchID string,
	match domain.MatchRecord,
	participants domain.MatchParticipantRecord,
) bool {
	return match.MatchID == matchID && participants.MatchID == matchID &&
		participants.LostPetID == match.MatchedPetID && participants.FoundPetID == match.FoundPetID &&
		participants.Validate() == nil
}
