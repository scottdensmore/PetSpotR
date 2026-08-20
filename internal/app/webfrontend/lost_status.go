package webfrontend

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func (s *Server) handleApiLostPetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.identitySessions == nil || s.lostPetLifecycle == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", http.MethodPatch)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, ok := s.verifiedRequestPrincipal(w, r)
	if !ok {
		return
	}
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	operationID := r.Header.Get(idempotencyKeyHeader)
	if strings.TrimSpace(operationID) == "" || strings.TrimSpace(operationID) != operationID ||
		!utf8.ValidString(operationID) || utf8.RuneCountInString(operationID) > domain.MaxLostPetLifecycleOperationRunes ||
		strings.ContainsFunc(operationID, unicode.IsControl) {
		http.Error(w, "Valid Idempotency-Key is required", http.StatusBadRequest)
		return
	}
	petID := r.PathValue("petID")
	if strings.TrimSpace(petID) == "" || strings.TrimSpace(petID) != petID {
		http.Error(w, "Valid pet ID is required", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		Status domain.LostPetStatus `json:"status"`
	}
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if request.Status != domain.LostPetStatusReunited {
		http.Error(w, "Status must be reunited", http.StatusBadRequest)
		return
	}
	result, err := s.lostPetLifecycle.ReuniteLostPet(r.Context(), lostpet.LifecycleCommand{
		PetID: petID, Status: request.Status, OperationID: operationID,
		Actor: domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject},
	})
	switch {
	case errors.Is(err, lostpet.ErrInvalidLifecycleCommand):
		http.Error(w, "Invalid lifecycle request", http.StatusBadRequest)
		return
	case errors.Is(err, lostpet.ErrLifecycleHidden), errors.Is(err, domain.ErrLostPetNotOwned),
		errors.Is(err, domain.ErrInvalidLostPetLifecycle):
		http.NotFound(w, r)
		return
	case errors.Is(err, domain.ErrLostPetLifecycleConflict):
		http.Error(w, "Lost-pet status already changed", http.StatusConflict)
		return
	case errors.Is(err, lostpet.ErrLifecycleUnavailable):
		http.Error(w, "Lost-pet lifecycle is unavailable", http.StatusServiceUnavailable)
		return
	case err != nil:
		http.Error(w, "Failed to update lost-pet status", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"petId": result.PetID, "status": string(result.Status), "eventId": result.EventID,
	})
}
