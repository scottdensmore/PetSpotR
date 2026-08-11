package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/telemetry"
)

//go:embed static/* templates/*
var embeddedFiles embed.FS

// Server encapsulates HTTP routes and handlers for the PetSpotR Web Frontend.
type Server struct {
	mux                      *http.ServeMux
	metrics                  *telemetry.MetricsRegistry
	stateStore               store.StateStore
	allowPrivilegedMutations bool
}

// ServerOptions controls behavior that must remain limited to explicit demo
// runtimes until authentication and ownership checks are implemented.
type ServerOptions struct {
	AllowPrivilegedMutations bool
}

// NewServer initializes a demo/test Server with seeded in-memory match data.
func NewServer() *Server {
	memory := store.NewMemoryStore()
	if err := seedDemoMatches(context.Background(), memory); err != nil {
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
	s := &Server{
		mux:                      http.NewServeMux(),
		metrics:                  telemetry.NewMetricsRegistry("web-frontend"),
		stateStore:               st,
		allowPrivilegedMutations: options.AllowPrivilegedMutations,
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
	s.mux.HandleFunc("/api/v1/found-pets/extract-features", s.handleApiExtractFeatures)
	s.mux.HandleFunc("/api/v1/found-pets", s.handleApiFoundPets)
	s.mux.HandleFunc("/api/v1/matches", s.handleApiMatches)
	s.mux.HandleFunc("/api/v1/matches/action", s.handleApiMatchAction)
	s.mux.HandleFunc("/api/v1/reunions/contact", s.handleApiReunionContact)
	s.mux.HandleFunc("/api/v1/reunions/resolve", s.handleApiReunionResolve)
	s.mux.HandleFunc("/api/v1/push/subscribe", s.handleApiPushSubscribe)
	s.mux.HandleFunc("/api/v1/push/test", s.handleApiPushTest)
	s.mux.HandleFunc("/api/v1/uploads/presigned-url", s.handleApiPresignedURL)
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
	PetName       string `json:"petName"`
	Species       string `json:"species"`
	Breed         string `json:"breed"`
	PrimaryColor  string `json:"primaryColor"`
	Description   string `json:"description"`
	Location      string `json:"location"`
	ReporterEmail string `json:"reporterEmail"`
	Phone         string `json:"phone"`
}

type PublicLostPet struct {
	PetID      string    `json:"petId"`
	ReportedAt time.Time `json:"reportedAt"`
	Location   string    `json:"location"`
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

		pets := make([]domain.LostPetEvent, 0, len(rawItems))
		for _, b := range rawItems {
			var pet domain.LostPetEvent
			if err := json.Unmarshal(b, &pet); err == nil {
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
			pets = []domain.LostPetEvent{}
		} else {
			end := params.Offset + params.Limit
			if end > len(pets) {
				end = len(pets)
			}
			pets = pets[params.Offset:end]
		}

		publicPets := make([]PublicLostPet, 0, len(pets))
		for _, pet := range pets {
			publicPets = append(publicPets, PublicLostPet{
				PetID:      pet.PetID,
				ReportedAt: pet.ReportedAt,
				Location:   pet.Location,
			})
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

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req LostPetFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.ReporterEmail) == "" {
		http.Error(w, "reporterEmail is required", http.StatusBadRequest)
		return
	}

	petID := fmt.Sprintf("lost-%d", time.Now().UnixNano())
	if strings.TrimSpace(req.PetName) != "" {
		petID = fmt.Sprintf("lost-%s-%d", strings.ToLower(req.PetName), time.Now().Unix())
	}

	evt := domain.LostPetEvent{
		PetID:         petID,
		ReporterEmail: req.ReporterEmail,
		ReportedAt:    time.Now().UTC(),
		Location:      req.Location,
	}

	if err := evt.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(evt)
	if err != nil {
		http.Error(w, "Failed to encode lost pet report", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.LostPetsCollection, evt.PetID, data); err != nil {
		http.Error(w, "Failed to save lost pet report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"petId":  evt.PetID,
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
	ImageURL      string `json:"imageUrl"`
	Location      string `json:"location"`
	FinderEmail   string `json:"finderEmail"`
	Species       string `json:"species"`
	Breed         string `json:"breed"`
	PrimaryColor  string `json:"primaryColor"`
	CustodyStatus string `json:"custodyStatus"`
}

func (s *Server) handleApiFoundPets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rawItems, err := s.stateStore.ListState(r.Context(), store.FoundPetsCollection)
		if err != nil {
			http.Error(w, "Failed to query found pets", http.StatusInternalServerError)
			return
		}

		params := parseQueryParams(r)

		pets := make([]domain.FoundPetEvent, 0, len(rawItems))
		for _, b := range rawItems {
			var pet domain.FoundPetEvent
			if err := json.Unmarshal(b, &pet); err == nil {
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
			pets = []domain.FoundPetEvent{}
		} else {
			end := params.Offset + params.Limit
			if end > len(pets) {
				end = len(pets)
			}
			pets = pets[params.Offset:end]
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(pets)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req FoundPetFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.ImageURL) == "" || strings.TrimSpace(req.Location) == "" {
		http.Error(w, "imageUrl and location are required", http.StatusBadRequest)
		return
	}

	petID := fmt.Sprintf("found-%d", time.Now().UnixNano())

	evt := domain.FoundPetEvent{
		PetID:    petID,
		ImageURL: req.ImageURL,
		FoundAt:  time.Now().UTC(),
		Location: req.Location,
	}

	if err := evt.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(evt)
	if err != nil {
		http.Error(w, "Failed to encode found pet report", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.FoundPetsCollection, evt.PetID, data); err != nil {
		http.Error(w, "Failed to save found pet report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"petId":  evt.PetID,
	})
}

type MatchScoreBreakdown struct {
	Visual        float64 `json:"visual"`
	Color         float64 `json:"color"`
	Spatial       float64 `json:"spatial"`
	DistanceMiles float64 `json:"distanceMiles"`
}

type PetDetail struct {
	PetID    string `json:"petId"`
	PetName  string `json:"petName,omitempty"`
	Breed    string `json:"breed"`
	ImageURL string `json:"imageUrl"`
	Location string `json:"location"`
}

type MatchRecord struct {
	MatchID      string              `json:"matchId"`
	FoundPetID   string              `json:"foundPetId"`
	MatchedPetID string              `json:"matchedPetId"`
	Score        float64             `json:"score"`
	Status       string              `json:"status"`
	MatchedAt    time.Time           `json:"matchedAt"`
	Scores       MatchScoreBreakdown `json:"scores"`
	LostPet      PetDetail           `json:"lostPet"`
	FoundPet     PetDetail           `json:"foundPet"`
}

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

func seedDemoMatches(ctx context.Context, stateStore store.StateStore) error {
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
	record.Status = status
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
	s.mux.ServeHTTP(w, r)
}
