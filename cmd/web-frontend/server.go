package main

import (
	"embed"
	"io/fs"
	"net/http"
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

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// ServeHTTP satisfies the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
