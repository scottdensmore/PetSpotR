package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/scoring"
)

//go:embed static/* templates/*
var embeddedFiles embed.FS

// Server encapsulates HTTP routes and handlers for the PetSpotR Web Frontend.
type Server struct {
	mux *http.ServeMux
}

// NewServer initializes a new Server instance with static asset handlers and page routes.
func NewServer() *Server {
	s := &Server{
		mux: http.NewServeMux(),
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

func (s *Server) handleApiLostPets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

func (s *Server) handleApiMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	matches := []MatchRecord{
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

	var req MatchActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	status := "CONFIRMED"
	if strings.ToLower(req.Action) == "reject" {
		status = "REJECTED"
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

	var req ReunionResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.MatchID) == "" || strings.TrimSpace(req.PetID) == "" {
		http.Error(w, "matchId and petId are required", http.StatusBadRequest)
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

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// ServeHTTP satisfies the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
