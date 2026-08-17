package webfrontend

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type privateReportContact struct {
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

func (s *Server) handleApiLostPetContact(w http.ResponseWriter, r *http.Request) {
	if s.identitySessions == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, ok := s.verifiedRequestPrincipal(w, r)
	if !ok {
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}
	reportData, err := s.stateStore.GetState(r.Context(), store.LostPetsCollection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}
	var report domain.LostPetRecord
	if err := json.Unmarshal(reportData, &report); err != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}
	report = domain.NormalizeLostPetRecord(report)
	if !principalOwnsReport(principal, report.OwnedBy) || report.OwnerIdentityRef == "" {
		http.NotFound(w, r)
		return
	}

	contactData, err := s.stateStore.GetState(r.Context(), store.ReportContactsCollection, report.OwnerIdentityRef)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}
	var contact domain.ReportContact
	if err := json.Unmarshal(contactData, &contact); err != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}
	contact = domain.NormalizeReportContact(contact)
	if contact.IdentityRef != report.OwnerIdentityRef || contact.Validate() != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(privateReportContact{Email: contact.Email, Phone: contact.Phone})
}

func principalOwnsReport(principal identity.Principal, owner *domain.PrincipalRef) bool {
	return owner != nil && owner.Issuer == principal.Issuer && owner.Subject == principal.Subject
}
