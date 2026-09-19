package webfrontend

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/microchip"
	"github.com/scottdensmore/petspotr/pkg/sheltersync"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// ShelterIntakeIngestResponse represents the JSON response returned by the intake ingestion endpoint.
type ShelterIntakeIngestResponse struct {
	Status            string `json:"status"`
	PetID             string `json:"petId"`
	IntakeID          string `json:"intakeId"`
	ShelterID         string `json:"shelterId"`
	CustodyStatus     string `json:"custodyStatus"`
	MicrochipID       string `json:"microchipId,omitempty"`
	MicrochipRegistry string `json:"microchipRegistry,omitempty"`
}

// handleApiShelterIntakeIngest handles POST /api/v1/shelter-intakes/ingest requests.
func (s *Server) handleApiShelterIntakeIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req sheltersync.ShelterIntakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	req.ShelterID = strings.TrimSpace(req.ShelterID)
	req.IntakeID = strings.TrimSpace(req.IntakeID)
	req.Animal.Species = strings.TrimSpace(req.Animal.Species)
	req.Location.Address = strings.TrimSpace(req.Location.Address)

	if req.ShelterID == "" || req.IntakeID == "" || req.Animal.Species == "" || req.Location.Address == "" {
		http.Error(w, "missing required fields: shelterId, intakeId, species, and location address are required", http.StatusBadRequest)
		return
	}

	var normalizedChip string
	var registryTitle string
	if trimmedChip := strings.TrimSpace(req.Animal.MicrochipID); trimmedChip != "" {
		val := microchip.ValidateAndNormalize(trimmedChip)
		if !val.Valid {
			http.Error(w, fmt.Sprintf("invalid microchip: %s", val.ErrorMessage), http.StatusBadRequest)
			return
		}
		normalizedChip = val.NormalizedID
		reg := microchip.IdentifyIssuingRegistry(normalizedChip)
		registryTitle = reg.RegistryName
	}

	foundAt := req.IntakeDate
	if foundAt.IsZero() {
		foundAt = time.Now().UTC()
	}

	var coords *domain.LocationPoint
	geocodingStatus := domain.GeocodingPending
	if req.Location.Latitude != 0 || req.Location.Longitude != 0 {
		coords = &domain.LocationPoint{
			Latitude:  req.Location.Latitude,
			Longitude: req.Location.Longitude,
		}
		geocodingStatus = domain.GeocodingVerified
	}

	shelterName := strings.TrimSpace(req.ShelterName)
	if shelterName == "" {
		shelterName = req.ShelterID
	}

	var images []domain.PetImage
	var imageURL string
	for i, img := range req.Animal.Images {
		tag := domain.PetImageTagPrimary
		if img.View != "" {
			tag = domain.PetImageTag(img.View)
		} else if i > 0 {
			tag = domain.PetImageTagFace
		}
		if tag == domain.PetImageTagPrimary && imageURL == "" {
			imageURL = img.URL
		}
		images = append(images, domain.PetImage{
			URL: img.URL,
			Tag: tag,
		})
	}
	if imageURL == "" && len(images) > 0 {
		imageURL = images[0].URL
	}
	if imageURL == "" {
		imageURL = fmt.Sprintf("https://storage.petspotr.io/shelters/%s-%s.jpg", req.ShelterID, req.IntakeID)
		images = append(images, domain.PetImage{
			URL: imageURL,
			Tag: domain.PetImageTagPrimary,
		})
	}

	petID := fmt.Sprintf("shelter-%s-%s", req.ShelterID, req.IntakeID)

	command := foundpet.ReportCommand{
		PetID:               petID,
		ImageURL:            imageURL,
		Images:              images,
		FoundAt:             foundAt,
		Location:            req.Location.Address,
		GeocodingStatus:     geocodingStatus,
		Coordinates:         coords,
		Species:             req.Animal.Species,
		Breed:               req.Animal.Breed,
		PrimaryColor:        req.Animal.PrimaryColor,
		SecondaryColor:      req.Animal.SecondaryColor,
		CustodyStatus:       domain.CustodyShelterCare,
		ShelterID:           req.ShelterID,
		ShelterName:         shelterName,
		IntakeID:            req.IntakeID,
		MicrochipID:         normalizedChip,
		MicrochipRegistry:   registryTitle,
	}

	if s.foundPetReporter == nil {
		http.Error(w, "found pet reporter not configured", http.StatusInternalServerError)
		return
	}

	result, err := s.foundPetReporter.ReportFoundPet(r.Context(), command, foundpet.ReportMetadata{
		CorrelationID: r.Header.Get("X-Correlation-ID"),
		TraceID:       r.Header.Get("X-Trace-ID"),
	})
	if err != nil {
		if errors.Is(err, foundpet.ErrInvalidReport) {
			cause := foundpet.InvalidReportCause(err)
			msg := err.Error()
			if cause != nil {
				msg = cause.Error()
			}
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if errors.Is(err, store.ErrConflict) {
			http.Error(w, "conflict: report already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := ShelterIntakeIngestResponse{
		Status:            "created",
		PetID:             result.PetID,
		IntakeID:          req.IntakeID,
		ShelterID:         req.ShelterID,
		CustodyStatus:     string(domain.CustodyShelterCare),
		MicrochipID:       normalizedChip,
		MicrochipRegistry: registryTitle,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
