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
	s.handleApiReportContact(w, r, store.LostPetsCollection, lostPetContactOwner)
}

func (s *Server) handleApiFoundPetContact(w http.ResponseWriter, r *http.Request) {
	s.handleApiReportContact(w, r, store.FoundPetsCollection, foundPetContactOwner)
}

type reportContactOwner func([]byte) (*domain.PrincipalRef, string, error)

func (s *Server) handleApiReportContact(
	w http.ResponseWriter,
	r *http.Request,
	collection string,
	contactOwner reportContactOwner,
) {
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
	reportData, err := s.stateStore.GetState(r.Context(), collection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}
	owner, identityRef, err := contactOwner(reportData)
	if err != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}
	if !principalMatchesRef(principal, owner) || identityRef == "" {
		http.NotFound(w, r)
		return
	}

	contactData, err := s.stateStore.GetState(r.Context(), store.ReportContactsCollection, identityRef)
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
	if contact.IdentityRef != identityRef || contact.Validate() != nil {
		http.Error(w, "Failed to load private contact", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(privateReportContact{Email: contact.Email, Phone: contact.Phone})
}

func lostPetContactOwner(data []byte) (*domain.PrincipalRef, string, error) {
	var report domain.LostPetRecord
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, "", err
	}
	report = domain.NormalizeLostPetRecord(report)
	return report.OwnedBy, report.OwnerIdentityRef, nil
}

func foundPetContactOwner(data []byte) (*domain.PrincipalRef, string, error) {
	var report domain.FoundPetRecord
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, "", err
	}
	report = domain.NormalizeFoundPetRecord(report)
	return report.OwnedBy, report.FinderIdentityRef, nil
}

func principalMatchesRef(principal identity.Principal, ref *domain.PrincipalRef) bool {
	return ref != nil && ref.Issuer == principal.Issuer && ref.Subject == principal.Subject
}
