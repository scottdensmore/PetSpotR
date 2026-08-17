// Package webfrontend implements the PetSpotR browser application server.
package webfrontend

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/telemetry"
)

//go:embed static/* templates/*
var embeddedFiles embed.FS

const contentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https://storage.petspotr.io; connect-src 'self'; worker-src 'self'"

// Server encapsulates HTTP routes and handlers for the PetSpotR Web Frontend.
type Server struct {
	mux                      *http.ServeMux
	metrics                  *telemetry.MetricsRegistry
	stateStore               store.StateStore
	foundPetReporter         FoundPetReporter
	lostPetReporter          LostPetReporter
	allowPrivilegedMutations bool
	identitySessions         identity.SessionManager
	secureSessionCookie      bool
}

// LostPetReporter is the canonical lost-pet command consumed by the browser
// adapter.
type LostPetReporter interface {
	ReportLostPet(context.Context, lostpet.ReportCommand, lostpet.ReportMetadata) (lostpet.ReportResult, error)
}

// FoundPetReporter is the canonical found-pet command consumed by the browser
// adapter.
type FoundPetReporter interface {
	ReportFoundPet(context.Context, foundpet.ReportCommand, foundpet.ReportMetadata) (foundpet.ReportResult, error)
}

// ServerOptions controls injected commands and behavior that must remain
// limited to explicit demo runtimes until authorization is implemented.
type ServerOptions struct {
	AllowPrivilegedMutations bool
	FoundPetReporter         FoundPetReporter
	LostPetReporter          LostPetReporter
	IdentitySessions         identity.SessionManager
	SecureSessionCookie      bool
}

// NewServer initializes an empty in-memory Server for tests and local callers.
func NewServer() *Server {
	memory := store.NewMemoryStore()
	return NewServerWithOptions(memory, ServerOptions{AllowPrivilegedMutations: true})
}

// NewDemoServer initializes a demo/test Server with explicit seeded match data.
func NewDemoServer() *Server {
	memory := store.NewMemoryStore()
	if err := SeedDemoMatches(context.Background(), memory); err != nil {
		panic(fmt.Sprintf("seed demo matches: %v", err))
	}
	return NewServerWithOptions(memory, ServerOptions{AllowPrivilegedMutations: true})
}

// NewServerWithStore constructs a secure-by-default Server with a custom
// StateStore. Privileged mutations remain disabled.
func NewServerWithStore(st store.StateStore) *Server {
	return NewServerWithOptions(st, ServerOptions{})
}

// NewServerWithOptions constructs a Server with explicit demo-only behavior.
func NewServerWithOptions(st store.StateStore, options ServerOptions) *Server {
	foundPetReporter := options.FoundPetReporter
	if foundPetReporter == nil {
		foundPetReporter = foundpet.NewReportService(st, pubsub.NewMemoryPubSub())
	}
	lostPetReporter := options.LostPetReporter
	if lostPetReporter == nil {
		lostPetReporter = lostpet.NewService(st, pubsub.NewMemoryPubSub())
	}
	s := &Server{
		mux:                      http.NewServeMux(),
		metrics:                  telemetry.NewMetricsRegistry("web-frontend"),
		stateStore:               st,
		foundPetReporter:         foundPetReporter,
		lostPetReporter:          lostPetReporter,
		allowPrivilegedMutations: options.AllowPrivilegedMutations,
		identitySessions:         options.IdentitySessions,
		secureSessionCookie:      options.SecureSessionCookie,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Sub-tree file server for static CSS/JS/images
	staticFS, err := fs.Sub(embeddedFiles, "static")
	if err == nil {
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	// Page & Health routes
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/sw.js", s.handleServiceWorker)
	s.mux.HandleFunc("/report-lost", s.handleReportLost)
	s.mux.HandleFunc("/report-found", s.handleReportFound)
	s.mux.HandleFunc("/matches", s.handleMatches)
	s.mux.HandleFunc("/api/v1/lost-pets", s.handleApiLostPets)
	s.mux.HandleFunc("/api/v1/lost-pets/{petID}/contact", s.handleApiLostPetContact)
	s.mux.HandleFunc("/api/v1/found-pets/extract-features", s.handleApiExtractFeatures)
	s.mux.HandleFunc("/api/v1/found-pets", s.handleApiFoundPets)
	s.mux.HandleFunc("/api/v1/matches", s.handleApiMatches)
	s.mux.HandleFunc("/api/v1/matches/action", s.handleApiMatchAction)
	s.mux.HandleFunc("/api/v1/reunions/contact", s.handleApiReunionContact)
	s.mux.HandleFunc("/api/v1/reunions/resolve", s.handleApiReunionResolve)
	s.mux.HandleFunc("/api/v1/push/subscribe", s.handleApiPushSubscribe)
	s.mux.HandleFunc("/api/v1/push/test", s.handleApiPushTest)
	s.mux.HandleFunc("/api/v1/uploads/presigned-url", s.handleApiPresignedURL)
	s.mux.HandleFunc("/api/v1/session/csrf", s.handleApiSessionCSRF)
	s.mux.HandleFunc("/api/v1/session", s.handleApiSession)
	s.mux.Handle("/metrics", s.metrics.MetricsHandler())
	s.mux.HandleFunc("/healthz", s.handleHealthz)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	content, err := embeddedFiles.ReadFile("templates/layout.html")
	if err != nil {
		http.Error(w, "Failed to load layout template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handleReportLost(w http.ResponseWriter, r *http.Request) {
	content, err := embeddedFiles.ReadFile("templates/report-lost.html")
	if err != nil {
		http.Error(w, "Failed to load report-lost template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handleReportFound(w http.ResponseWriter, r *http.Request) {
	content, err := embeddedFiles.ReadFile("templates/report-found.html")
	if err != nil {
		http.Error(w, "Failed to load report-found template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handleMatches(w http.ResponseWriter, r *http.Request) {
	content, err := embeddedFiles.ReadFile("templates/matches.html")
	if err != nil {
		http.Error(w, "Failed to load matches template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

type LostPetFormRequest struct {
	PetID         string    `json:"petId"`
	PetName       string    `json:"petName"`
	Species       string    `json:"species"`
	Breed         string    `json:"breed"`
	PrimaryColor  string    `json:"primaryColor"`
	Description   string    `json:"description"`
	Location      string    `json:"location"`
	ReporterEmail string    `json:"reporterEmail"`
	Phone         string    `json:"phone"`
	ReportedAt    time.Time `json:"reportedAt"`
}

func newLostPetID(petName string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate lost-pet ID: %w", err)
	}
	suffix := hex.EncodeToString(random[:])
	if name := strings.ToLower(strings.TrimSpace(petName)); name != "" {
		return fmt.Sprintf("lost-%s-%s", name, suffix), nil
	}
	return "lost-" + suffix, nil
}

type QueryParams struct {
	Limit       int
	Offset      int
	Species     string
	Status      string
	HasGeo      bool
	GeoPoint    domain.LocationPoint
	RadiusMiles float64
}

func parseQueryParams(r *http.Request) QueryParams {
	q := r.URL.Query()

	limit := 20
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l >= 1 {
			if l > 100 {
				l = 100
			}
			limit = l
		}
	}

	offset := 0
	if oStr := q.Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	species := strings.TrimSpace(q.Get("species"))
	status := strings.TrimSpace(q.Get("status"))

	var hasGeo bool
	var geoPoint domain.LocationPoint
	radiusMiles := 10.0

	latStr := q.Get("lat")
	lngStr := q.Get("lng")
	if latStr != "" && lngStr != "" {
		lat, err1 := strconv.ParseFloat(latStr, 64)
		lng, err2 := strconv.ParseFloat(lngStr, 64)
		if err1 == nil && err2 == nil && !math.IsNaN(lat) && !math.IsNaN(lng) && !math.IsInf(lat, 0) && !math.IsInf(lng, 0) {
			pt := domain.LocationPoint{Latitude: lat, Longitude: lng}
			if pt.Validate() == nil {
				hasGeo = true
				geoPoint = pt
			}
		}
	}

	if rStr := q.Get("radiusMiles"); rStr != "" {
		if rVal, err := strconv.ParseFloat(rStr, 64); err == nil && rVal > 0 {
			radiusMiles = rVal
		}
	}

	return QueryParams{
		Limit:       limit,
		Offset:      offset,
		Species:     species,
		Status:      status,
		HasGeo:      hasGeo,
		GeoPoint:    geoPoint,
		RadiusMiles: radiusMiles,
	}
}

func (s *Server) handleApiLostPets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rawItems, err := s.stateStore.ListState(r.Context(), store.LostPetsCollection)
		if err != nil {
			http.Error(w, "Failed to query lost pets", http.StatusInternalServerError)
			return
		}

		params := parseQueryParams(r)

		pets := make([]domain.LostPetRecord, 0, len(rawItems))
		for _, b := range rawItems {
			var pet domain.LostPetRecord
			if err := json.Unmarshal(b, &pet); err == nil {
				pet = domain.NormalizeLostPetRecord(pet)
				// Species filter check
				if params.Species != "" {
					var rawMap map[string]any
					_ = json.Unmarshal(b, &rawMap)
					if sp, ok := rawMap["species"].(string); ok && sp != "" {
						if !strings.EqualFold(sp, params.Species) {
							continue
						}
					}
				}
				// Geo radius filter
				if params.HasGeo {
					locPt := domain.ParseLocationCoordinates(pet.Location)
					dist := domain.HaversineDistanceMiles(params.GeoPoint, locPt)
					if dist > params.RadiusMiles {
						continue
					}
				}
				pets = append(pets, pet)
			}
		}

		totalCount := len(pets)
		w.Header().Set("X-Total-Count", strconv.Itoa(totalCount))

		// Apply pagination limit & offset
		if params.Offset > len(pets) {
			pets = []domain.LostPetRecord{}
		} else {
			end := params.Offset + params.Limit
			if end > len(pets) {
				end = len(pets)
			}
			pets = pets[params.Offset:end]
		}

		publicPets := make([]domain.PublicLostPetReport, 0, len(pets))
		for _, pet := range pets {
			publicPets = append(publicPets, pet.Public())
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(publicPets)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var principal *identity.Principal
	if s.identitySessions != nil {
		verified, ok := s.verifiedRequestPrincipal(w, r)
		if !ok {
			return
		}
		if !s.validCSRF(r) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
		principal = &verified
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req LostPetFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		var err error
		petID, err = newLostPetID(req.PetName)
		if err != nil {
			http.Error(w, "Failed to create lost pet report", http.StatusInternalServerError)
			return
		}
	}
	reportedAt := req.ReportedAt
	if reportedAt.IsZero() {
		reportedAt = time.Now().UTC()
	}
	reporterEmail := req.ReporterEmail
	var ownedBy *domain.PrincipalRef
	if principal != nil {
		reporterEmail = principal.Email
		ownedBy = &domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject}
	}

	command := lostpet.ReportCommand{
		PetID:         petID,
		PetName:       req.PetName,
		Species:       req.Species,
		Breed:         req.Breed,
		PrimaryColor:  req.PrimaryColor,
		Description:   req.Description,
		ReporterEmail: reporterEmail,
		Phone:         req.Phone,
		ReportedAt:    reportedAt,
		Location:      req.Location,
		OwnedBy:       ownedBy,
	}

	result, err := s.lostPetReporter.ReportLostPet(r.Context(), command, lostpet.ReportMetadata{
		CorrelationID: r.Header.Get("X-Correlation-ID"),
		TraceID:       r.Header.Get("X-Trace-ID"),
	})
	switch {
	case errors.Is(err, lostpet.ErrInvalidReport):
		message := err.Error()
		if cause := lostpet.InvalidReportCause(err); cause != nil {
			message = cause.Error()
		}
		if strings.TrimSpace(command.ReporterEmail) == "" {
			message = "reporterEmail is required"
		}
		http.Error(w, message, http.StatusBadRequest)
		return
	case errors.Is(err, store.ErrConflict):
		http.Error(w, "A different report already exists for this pet ID", http.StatusConflict)
		return
	case err != nil:
		http.Error(w, "Failed to save lost pet report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"petId":  result.PetID,
	})
}

type FeatureExtractRequest struct {
	ImageURL string `json:"imageUrl"`
}

func (s *Server) handleApiExtractFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req FeatureExtractRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Simulate/Run Gemma 4 Vision AI feature extraction
	mockGemmaJSON := `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":["White chest patch"]}`
	traits, _ := scoring.ParseGemmaResponse(mockGemmaJSON)

	resp := map[string]any{
		"species":             "Dog",
		"breed":               traits.Breed,
		"primaryColor":        traits.PrimaryColor,
		"secondaryColor":      traits.SecondaryColor,
		"distinctiveMarkings": traits.DistinctiveMarkings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

type FoundPetFormRequest struct {
	PetID               string               `json:"petId"`
	ImageURL            string               `json:"imageUrl"`
	Location            string               `json:"location"`
	FinderEmail         string               `json:"finderEmail"`
	Species             string               `json:"species"`
	Breed               string               `json:"breed"`
	PrimaryColor        string               `json:"primaryColor"`
	SecondaryColor      string               `json:"secondaryColor"`
	DistinctiveMarkings []string             `json:"distinctiveMarkings"`
	CustodyStatus       domain.CustodyStatus `json:"custodyStatus"`
	FoundAt             time.Time            `json:"foundAt"`
}

func newFoundPetID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate found-pet ID: %w", err)
	}
	return "found-" + hex.EncodeToString(random[:]), nil
}

func (s *Server) handleApiFoundPets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rawItems, err := s.stateStore.ListState(r.Context(), store.FoundPetsCollection)
		if err != nil {
			http.Error(w, "Failed to query found pets", http.StatusInternalServerError)
			return
		}

		params := parseQueryParams(r)

		pets := make([]domain.FoundPetRecord, 0, len(rawItems))
		for _, b := range rawItems {
			var pet domain.FoundPetRecord
			if err := json.Unmarshal(b, &pet); err == nil {
				pet = domain.NormalizeFoundPetRecord(pet)
				// Species filter check
				if params.Species != "" {
					if pet.Species != "" && !strings.EqualFold(pet.Species, params.Species) {
						continue
					}
				}
				// Geo filter check
				if params.HasGeo {
					locPt := domain.ParseLocationCoordinates(pet.Location)
					dist := domain.HaversineDistanceMiles(params.GeoPoint, locPt)
					if dist > params.RadiusMiles {
						continue
					}
				}
				pets = append(pets, pet)
			}
		}

		totalCount := len(pets)
		w.Header().Set("X-Total-Count", strconv.Itoa(totalCount))

		// Apply pagination limit & offset
		if params.Offset > len(pets) {
			pets = []domain.FoundPetRecord{}
		} else {
			end := params.Offset + params.Limit
			if end > len(pets) {
				end = len(pets)
			}
			pets = pets[params.Offset:end]
		}

		publicPets := make([]domain.PublicFoundPetReport, 0, len(pets))
		for _, pet := range pets {
			publicPets = append(publicPets, pet.Public())
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(publicPets)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var principal *identity.Principal
	if s.identitySessions != nil {
		verified, ok := s.verifiedRequestPrincipal(w, r)
		if !ok {
			return
		}
		if !s.validCSRF(r) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
		principal = &verified
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req FoundPetFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		var err error
		petID, err = newFoundPetID()
		if err != nil {
			http.Error(w, "Failed to create found pet report", http.StatusInternalServerError)
			return
		}
	}
	foundAt := req.FoundAt
	if foundAt.IsZero() {
		foundAt = time.Now().UTC()
	}
	finderEmail := req.FinderEmail
	var ownedBy *domain.PrincipalRef
	if principal != nil {
		finderEmail = principal.Email
		ownedBy = &domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject}
	}

	command := foundpet.ReportCommand{
		PetID:               petID,
		ImageURL:            req.ImageURL,
		FoundAt:             foundAt,
		Location:            req.Location,
		FinderEmail:         finderEmail,
		Species:             req.Species,
		Breed:               req.Breed,
		PrimaryColor:        req.PrimaryColor,
		SecondaryColor:      req.SecondaryColor,
		DistinctiveMarkings: req.DistinctiveMarkings,
		CustodyStatus:       req.CustodyStatus,
		OwnedBy:             ownedBy,
	}

	result, err := s.foundPetReporter.ReportFoundPet(r.Context(), command, foundpet.ReportMetadata{
		CorrelationID: r.Header.Get("X-Correlation-ID"),
		TraceID:       r.Header.Get("X-Trace-ID"),
	})
	switch {
	case errors.Is(err, foundpet.ErrInvalidReport):
		message := err.Error()
		if cause := foundpet.InvalidReportCause(err); cause != nil {
			message = cause.Error()
		}
		if strings.TrimSpace(command.ImageURL) == "" && strings.TrimSpace(command.ImageObject) == "" ||
			strings.TrimSpace(command.Location) == "" {
			message = "imageUrl and location are required"
		}
		http.Error(w, message, http.StatusBadRequest)
		return
	case errors.Is(err, store.ErrConflict):
		http.Error(w, "A different report already exists for this pet ID", http.StatusConflict)
		return
	case err != nil:
		http.Error(w, "Failed to save found pet report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"petId":  result.PetID,
	})
}

type MatchScoreBreakdown = domain.MatchScoreBreakdown

type PetDetail = domain.MatchPetDetail

type MatchRecord = domain.MatchRecord

func demoMatchRecords() []MatchRecord {
	return []MatchRecord{
		{
			MatchID:      "match-101",
			FoundPetID:   "found-202",
			MatchedPetID: "lost-101",
			Score:        0.92,
			Status:       "PENDING_REVIEW",
			MatchedAt:    time.Now().UTC().Add(-15 * time.Minute),
			Scores: MatchScoreBreakdown{
				Visual:        0.95,
				Color:         0.90,
				Spatial:       0.88,
				DistanceMiles: 2.4,
			},
			LostPet: PetDetail{
				PetID:    "lost-101",
				PetName:  "Buddy",
				Breed:    "Golden Retriever",
				ImageURL: "https://storage.petspotr.io/lost-101.jpg",
				Location: "Capitol Hill, Seattle, WA",
			},
			FoundPet: PetDetail{
				PetID:    "found-202",
				Breed:    "Golden Retriever",
				ImageURL: "https://storage.petspotr.io/found-202.jpg",
				Location: "Green Lake Park, Seattle, WA",
			},
		},
		{
			MatchID:      "match-102",
			FoundPetID:   "found-203",
			MatchedPetID: "lost-105",
			Score:        0.87,
			Status:       "PENDING_REVIEW",
			MatchedAt:    time.Now().UTC().Add(-2 * time.Hour),
			Scores: MatchScoreBreakdown{
				Visual:        0.88,
				Color:         0.85,
				Spatial:       0.86,
				DistanceMiles: 4.1,
			},
			LostPet: PetDetail{
				PetID:    "lost-105",
				PetName:  "Luna",
				Breed:    "Siamese Cat",
				ImageURL: "https://storage.petspotr.io/lost-105.jpg",
				Location: "Ballard, Seattle, WA",
			},
			FoundPet: PetDetail{
				PetID:    "found-203",
				Breed:    "Siamese Cat",
				ImageURL: "https://storage.petspotr.io/found-203.jpg",
				Location: "Fremont, Seattle, WA",
			},
		},
	}
}

// SeedDemoMatches replaces the fixed-ID development match fixtures.
func SeedDemoMatches(ctx context.Context, stateStore store.StateStore) error {
	for _, match := range demoMatchRecords() {
		data, err := json.Marshal(match)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", match.MatchID, err)
		}
		if err := stateStore.SaveState(ctx, store.MatchesCollection, match.MatchID, data); err != nil {
			return fmt.Errorf("save %s: %w", match.MatchID, err)
		}
	}
	return nil
}

func (s *Server) handleApiMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawMatches, err := s.stateStore.ListState(r.Context(), store.MatchesCollection)
	if err != nil {
		http.Error(w, "Failed to query matches from state store", http.StatusInternalServerError)
		return
	}

	matches := make([]MatchRecord, 0, len(rawMatches))
	for _, b := range rawMatches {
		var m MatchRecord
		if err := json.Unmarshal(b, &m); err == nil {
			matches = append(matches, m)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(matches)
}

type MatchActionRequest struct {
	MatchID string `json:"matchId"`
	Action  string `json:"action"`
}

func (s *Server) handleApiMatchAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowPrivilegedMutations {
		http.Error(w, "Authentication is required for match actions", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req MatchActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	actionLower := strings.ToLower(strings.TrimSpace(req.Action))
	if actionLower != "confirm" && actionLower != "reject" {
		http.Error(w, "invalid action: must be confirm or reject", http.StatusBadRequest)
		return
	}

	status := "CONFIRMED"
	if actionLower == "reject" {
		status = "REJECTED"
	}

	data, err := s.stateStore.GetState(r.Context(), store.MatchesCollection, req.MatchID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load match", http.StatusInternalServerError)
		return
	}
	var record MatchRecord
	if err := json.Unmarshal(data, &record); err != nil {
		http.Error(w, "Failed to decode match", http.StatusInternalServerError)
		return
	}
	record.Status = domain.MatchStatus(status)
	updated, err := json.Marshal(record)
	if err != nil {
		http.Error(w, "Failed to encode match", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.MatchesCollection, req.MatchID, updated); err != nil {
		http.Error(w, "Failed to save match", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"matchId": req.MatchID,
		"status":  status,
		"message": fmt.Sprintf("Match status updated to %s", status),
	})
}

type ReunionContactRequest struct {
	MatchID     string `json:"matchId"`
	SenderEmail string `json:"senderEmail"`
	Message     string `json:"message"`
}

func (s *Server) handleApiReunionContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowPrivilegedMutations {
		http.Error(w, "Authentication is required for contact messages", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req ReunionContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.SenderEmail) == "" || strings.TrimSpace(req.Message) == "" {
		http.Error(w, "senderEmail and message are required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "sent",
		"matchId": req.MatchID,
		"message": "Secure message dispatched successfully",
	})
}

type ReunionResolveRequest struct {
	MatchID  string `json:"matchId"`
	PetID    string `json:"petId"`
	Rating   int    `json:"rating"`
	Feedback string `json:"feedback"`
}

func (s *Server) handleApiReunionResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowPrivilegedMutations {
		http.Error(w, "Authentication is required for reunion resolution", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req ReunionResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.MatchID) == "" || strings.TrimSpace(req.PetID) == "" {
		http.Error(w, "matchId and petId are required", http.StatusBadRequest)
		return
	}

	data, err := s.stateStore.GetState(r.Context(), store.MatchesCollection, req.MatchID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load match", http.StatusInternalServerError)
		return
	}
	var record MatchRecord
	if err := json.Unmarshal(data, &record); err != nil {
		http.Error(w, "Failed to decode match", http.StatusInternalServerError)
		return
	}
	record.Status = "REUNITED"
	updated, err := json.Marshal(record)
	if err != nil {
		http.Error(w, "Failed to encode match", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.MatchesCollection, req.MatchID, updated); err != nil {
		http.Error(w, "Failed to save match", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"matchId":  req.MatchID,
		"petId":    req.PetID,
		"status":   "REUNITED",
		"rating":   req.Rating,
		"feedback": req.Feedback,
		"message":  "Pet status successfully updated to REUNITED",
	})
}

func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	content, err := embeddedFiles.ReadFile("static/sw.js")
	if err != nil {
		http.Error(w, "Failed to load service worker script", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

type PushKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

type PushSubscriptionRequest struct {
	Endpoint string   `json:"endpoint"`
	Keys     PushKeys `json:"keys"`
}

func (s *Server) handleApiPushSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowPrivilegedMutations {
		http.Error(w, "Authentication is required for push subscriptions", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req PushSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" || len(endpoint) > 2048 || !strings.HasPrefix(endpoint, "https://") {
		http.Error(w, "valid https endpoint URL is required", http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Failed to encode push subscription", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.PushSubscriptionsCollection, endpoint, data); err != nil {
		http.Error(w, "Failed to save push subscription", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "subscribed",
		"endpoint": endpoint,
		"message":  "Web Push subscription registered successfully",
	})
}

func (s *Server) handleApiPushTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"title": "PetSpotR High-Confidence Match! 🐾",
		"body":  "A 95% visual match was found for your pet Buddy in Capitol Hill.",
		"url":   "/matches",
	})
}

type PresignedURLRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
}

func (s *Server) handleApiPresignedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowPrivilegedMutations {
		http.Error(w, "Use the authenticated found-pet upload service", http.StatusForbidden)
		return
	}

	var req PresignedURLRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	blobStore := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")
	res, err := blobStore.GeneratePresignedUploadURL(r.Context(), req.FileName, req.ContentType, 15*time.Minute)
	if err != nil {
		http.Error(w, "Failed to generate presigned upload URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// ServeHTTP satisfies the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
	w.Header().Set("X-Frame-Options", "DENY")
	s.mux.ServeHTTP(w, r)
}
