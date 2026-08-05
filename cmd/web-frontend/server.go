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
	s.mux.HandleFunc("/api/v1/lost-pets", s.handleApiLostPets)
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

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// ServeHTTP satisfies the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
