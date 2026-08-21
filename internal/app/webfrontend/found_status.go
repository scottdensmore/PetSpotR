package webfrontend

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func (s *Server) handleApiFoundPetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.identitySessions == nil || s.foundPetLifecycle == nil {
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
		!utf8.ValidString(operationID) || utf8.RuneCountInString(operationID) > domain.MaxFoundPetLifecycleOperationRunes ||
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
		Status domain.FoundPetStatus `json:"status"`
	}
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if request.Status != domain.FoundPetStatusResolved {
		http.Error(w, "Status must be resolved", http.StatusBadRequest)
		return
	}
	result, err := s.foundPetLifecycle.ResolveFoundPet(r.Context(), foundpet.LifecycleCommand{
		PetID: petID, Status: request.Status, OperationID: operationID,
		Actor: domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject},
	})
	switch {
	case errors.Is(err, foundpet.ErrInvalidLifecycleCommand):
		http.Error(w, "Invalid lifecycle request", http.StatusBadRequest)
		return
	case errors.Is(err, foundpet.ErrLifecycleHidden), errors.Is(err, domain.ErrFoundPetNotOwned),
		errors.Is(err, domain.ErrInvalidFoundPetLifecycle):
		http.NotFound(w, r)
		return
	case errors.Is(err, domain.ErrFoundPetLifecycleConflict):
		http.Error(w, "Found-pet status already changed", http.StatusConflict)
		return
	case errors.Is(err, foundpet.ErrLifecycleUnavailable):
		http.Error(w, "Found-pet lifecycle is unavailable", http.StatusServiceUnavailable)
		return
	case err != nil:
		http.Error(w, "Failed to update found-pet status", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"petId": result.PetID, "status": string(result.Status), "eventId": result.EventID,
	})
}
