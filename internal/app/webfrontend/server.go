// Package webfrontend implements the PetSpotR browser application server.
package webfrontend

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/ratelimit"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/telemetry"
)

//go:embed static/* templates/*
var embeddedFiles embed.FS

const contentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https://storage.petspotr.io https://*.tile.openstreetmap.org; connect-src 'self' https://storage.petspotr.io; worker-src 'self'"

// Server encapsulates HTTP routes and handlers for the PetSpotR Web Frontend.
type Server struct {
	mux                      *http.ServeMux
	metrics                  *telemetry.MetricsRegistry
	stateStore               store.StateStore
	foundPetReporter         FoundPetReporter
	foundPetLifecycle        FoundPetLifecycle
	lostPetReporter          LostPetReporter
	lostPetLifecycle         LostPetLifecycle
	allowPrivilegedMutations bool
	identitySessions         identity.SessionManager
	identityClientConfig     identity.WebClientConfig
	secureSessionCookie      bool
	rateLimiter              ratelimit.Limiter
	reunionHub               *ReunionHub
	reunionPingInterval      time.Duration
	handler                  http.Handler
}

// LostPetReporter is the canonical lost-pet command consumed by the browser
// adapter.
type LostPetReporter interface {
	ReportLostPet(context.Context, lostpet.ReportCommand, lostpet.ReportMetadata) (lostpet.ReportResult, error)
}

// LostPetLifecycle is the authenticated owner-only lost-pet lifecycle command.
type LostPetLifecycle interface {
	ReuniteLostPet(context.Context, lostpet.LifecycleCommand) (lostpet.LifecycleResult, error)
}

// FoundPetReporter is the canonical found-pet command consumed by the browser
// adapter.
type FoundPetReporter interface {
	ReportFoundPet(context.Context, foundpet.ReportCommand, foundpet.ReportMetadata) (foundpet.ReportResult, error)
}

// FoundPetLifecycle is the authenticated finder-owned found-pet lifecycle
// command.
type FoundPetLifecycle interface {
	ResolveFoundPet(context.Context, foundpet.LifecycleCommand) (foundpet.LifecycleResult, error)
}

// ServerOptions controls injected commands and behavior that must remain
// limited to explicit demo runtimes until authorization is implemented.
type ServerOptions struct {
	AllowPrivilegedMutations bool
	FoundPetReporter         FoundPetReporter
	FoundPetLifecycle        FoundPetLifecycle
	LostPetReporter          LostPetReporter
	LostPetLifecycle         LostPetLifecycle
	IdentitySessions         identity.SessionManager
	IdentityClientConfig     identity.WebClientConfig
	SecureSessionCookie      bool
	RateLimiter              ratelimit.Limiter
	DisableRateLimiting      bool
	ReunionHub               *ReunionHub
	ReunionPingInterval      time.Duration
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
	if err := SeedDemoPets(context.Background(), memory); err != nil {
		panic(fmt.Sprintf("seed demo pets: %v", err))
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
	foundPetLifecycle := options.FoundPetLifecycle
	if foundPetLifecycle == nil {
		foundPetLifecycle, _ = foundPetReporter.(FoundPetLifecycle)
	}
	lostPetReporter := options.LostPetReporter
	if lostPetReporter == nil {
		lostPetReporter = lostpet.NewService(st, pubsub.NewMemoryPubSub())
	}
	lostPetLifecycle := options.LostPetLifecycle
	if lostPetLifecycle == nil {
		lostPetLifecycle, _ = lostPetReporter.(LostPetLifecycle)
	}
	identityClientConfig := options.IdentityClientConfig
	if options.IdentitySessions == nil || identityClientConfig.Validate() != nil {
		identityClientConfig = identity.WebClientConfig{}
	}
	allowPrivilegedMutations := options.AllowPrivilegedMutations && options.IdentitySessions == nil
	rateLimiter := options.RateLimiter
	if rateLimiter == nil {
		if options.DisableRateLimiting {
			rateLimiter = ratelimit.NewNoop()
		} else {
			var limiterOpts []ratelimit.Option
			if options.IdentitySessions != nil {
				cookieName := localSessionCookieName
				if options.SecureSessionCookie {
					cookieName = secureSessionCookieName
				}
				limiterOpts = append(limiterOpts, ratelimit.WithSubjectExtractor(func(r *http.Request) string {
					cookie, err := r.Cookie(cookieName)
					if err != nil || strings.TrimSpace(cookie.Value) == "" {
						return ""
					}
					principal, err := options.IdentitySessions.VerifySession(r.Context(), cookie.Value)
					if err != nil {
						return ""
					}
					return principal.Subject
				}))
			}
			rateLimiter = ratelimit.New(limiterOpts...)
		}
	}
	reunionHub := options.ReunionHub
	if reunionHub == nil {
		reunionHub = NewReunionHub()
	}
	s := &Server{
		mux:                      http.NewServeMux(),
		metrics:                  telemetry.NewMetricsRegistry("web-frontend"),
		stateStore:               st,
		foundPetReporter:         foundPetReporter,
		foundPetLifecycle:        foundPetLifecycle,
		lostPetReporter:          lostPetReporter,
		lostPetLifecycle:         lostPetLifecycle,
		allowPrivilegedMutations: allowPrivilegedMutations,
		identitySessions:         options.IdentitySessions,
		identityClientConfig:     identityClientConfig,
		secureSessionCookie:      options.SecureSessionCookie,
		rateLimiter:              rateLimiter,
		reunionHub:               reunionHub,
		reunionPingInterval:      options.ReunionPingInterval,
	}
	s.routes()
	s.handler = telemetry.TraceContextMiddleware(s.mux)
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
	s.mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		data, err := embeddedFiles.ReadFile("static/manifest.webmanifest")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
	})
	s.mux.HandleFunc("/offline.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		data, err := embeddedFiles.ReadFile("templates/offline.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
	})
	s.mux.HandleFunc("/p/{petID}", s.handleFinderLanding)
	s.mux.HandleFunc("/pets", s.handlePets)
	s.mux.HandleFunc("/pets/{petID}/poster", s.handlePetPoster)
	s.mux.HandleFunc("/report-lost", s.handleReportLost)
	s.mux.HandleFunc("/report-found", s.handleReportFound)
	s.mux.HandleFunc("/matches", s.handleMatches)
	s.mux.HandleFunc("/shelters/analytics", s.handleShelterAnalytics)
	if s.rateLimiter == nil {
		s.rateLimiter = ratelimit.NewNoop()
	}

	s.mux.HandleFunc("/api/v1/pets", s.rateLimiter.RequireRateLimitFunc(ratelimit.GenerousLimit, s.handleApiPets))
	s.mux.HandleFunc("/api/v1/pets/{petID}/qr.svg", s.handleApiPetQR)
	s.mux.HandleFunc("/api/v1/pets/{petID}/share-card.svg", s.handleApiPetShareCard)
	s.mux.HandleFunc("/api/v1/lost-pets", s.rateLimiter.RequireRateLimitByMethodFunc(
		map[string]ratelimit.Limit{
			http.MethodPost: ratelimit.ModerateLimit,
			http.MethodGet:  ratelimit.GenerousLimit,
		},
		nil,
		s.handleApiLostPets,
	))
	s.mux.HandleFunc("/api/v1/lost-pets/{petID}/contact", s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, s.handleApiLostPetContact))
	s.mux.HandleFunc("/api/v1/lost-pets/{petID}/status", s.handleApiLostPetStatus)
	s.mux.HandleFunc("/api/v1/found-pets/extract-features", s.rateLimiter.RequireRateLimitFunc(ratelimit.StrictLimit, s.handleApiExtractFeatures))
	s.mux.HandleFunc("/api/v1/found-pets", s.rateLimiter.RequireRateLimitByMethodFunc(
		map[string]ratelimit.Limit{
			http.MethodPost: ratelimit.ModerateLimit,
			http.MethodGet:  ratelimit.GenerousLimit,
		},
		nil,
		s.handleApiFoundPets,
	))
	s.mux.HandleFunc("/api/v1/found-pets/{petID}/contact", s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, s.handleApiFoundPetContact))
	s.mux.HandleFunc("/api/v1/found-pets/{petID}/status", s.handleApiFoundPetStatus)
	s.mux.HandleFunc("/api/v1/shelter-intakes/ingest", s.handleApiShelterIntakeIngest)
	s.mux.HandleFunc("/api/v1/shelters/analytics", s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, s.handleApiShelterAnalytics))
	s.mux.HandleFunc("/api/v1/shelters/analytics/export.csv", s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, s.handleApiShelterAnalyticsExportCSV))
	s.mux.HandleFunc("/api/v1/shelters/analytics/export.geojson", s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, s.handleApiShelterAnalyticsExportGeoJSON))
	s.mux.HandleFunc("/api/v1/matches", s.rateLimiter.RequireRateLimitFunc(ratelimit.GenerousLimit, s.handleApiMatches))
	s.mux.HandleFunc("/api/v1/matches/action", s.handleApiMatchAction)
	s.mux.HandleFunc("/api/v1/reunions/contact", s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, s.handleApiReunionContact))
	s.mux.HandleFunc("/api/v1/reunions/resolve", s.handleApiReunionResolve)
	s.mux.HandleFunc("/api/v1/reunions/events", s.handleApiReunionEvents)
	s.mux.HandleFunc("/api/v1/reunions/presence", s.handleApiReunionPresence)
	s.mux.HandleFunc("/api/v1/push/subscribe", s.handleApiPushSubscribe)
	s.mux.HandleFunc("/api/v1/push/test", s.handleApiPushTest)
	s.mux.HandleFunc("/api/v1/notifications", s.handleApiNotifications)
	s.mux.HandleFunc("/api/v1/notifications/mark-read", s.handleApiNotificationsMarkRead)
	s.mux.HandleFunc("/api/v1/notifications/preferences", s.handleApiNotificationsPreferences)
	s.mux.HandleFunc("/api/v1/uploads/presigned-url", s.handleApiPresignedURL)
	s.mux.HandleFunc("/api/v1/session/csrf", s.handleApiSessionCSRF)
	s.mux.HandleFunc("/api/v1/session/client-config", s.handleApiIdentityClientConfig)
	s.mux.HandleFunc("/api/v1/session", s.handleApiSession)
	s.mux.Handle("/metrics", s.metrics.MetricsHandler())
	telemetry.RegisterHealthRoutes(s.mux, map[string]telemetry.ReadinessChecker{
		"state": func(ctx context.Context) error {
			if s.stateStore == nil {
				return errors.New("state store uninitialized")
			}
			return nil
		},
	})
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
	PetID         string            `json:"petId"`
	PetName       string            `json:"petName"`
	Species       string            `json:"species"`
	Breed         string            `json:"breed"`
	PrimaryColor  string            `json:"primaryColor"`
	Description   string            `json:"description"`
	Location      string            `json:"location"`
	ReporterEmail string            `json:"reporterEmail"`
	Phone         string            `json:"phone"`
	MicrochipID   string            `json:"microchipId,omitempty"`
	ImageObject   string            `json:"imageObject,omitempty"`
	Images        []domain.PetImage `json:"images,omitempty"`
	ReportedAt    time.Time         `json:"reportedAt"`
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

func parseQueryParams(r *http.Request) (QueryParams, error) {
	q := r.URL.Query()

	limit := 20
	if lStr := q.Get("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 {
			return QueryParams{}, errors.New("invalid limit: must be a positive integer")
		}
		if l > 100 {
			l = 100
		}
		limit = l
	}

	offset := 0
	if oStr := q.Get("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 {
			return QueryParams{}, errors.New("invalid offset: must be a non-negative integer")
		}
		offset = o
	}

	species := strings.TrimSpace(q.Get("species"))
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !strings.EqualFold(status, "lost") && !strings.EqualFold(status, "found") && !strings.EqualFold(status, "all") {
		return QueryParams{}, errors.New("invalid status filter: must be lost, found, or all")
	}

	var hasGeo bool
	var geoPoint domain.LocationPoint
	radiusMiles := 10.0

	latStr := q.Get("lat")
	lngStr := q.Get("lng")
	if (latStr != "" && lngStr == "") || (latStr == "" && lngStr != "") {
		return QueryParams{}, errors.New("both lat and lng parameters are required for proximity filtering")
	}
	if latStr != "" && lngStr != "" {
		lat, err1 := strconv.ParseFloat(latStr, 64)
		lng, err2 := strconv.ParseFloat(lngStr, 64)
		if err1 != nil || err2 != nil || math.IsNaN(lat) || math.IsNaN(lng) || math.IsInf(lat, 0) || math.IsInf(lng, 0) ||
			lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return QueryParams{}, errors.New("invalid coordinates: lat must be [-90, 90] and lng [-180, 180]")
		}
		hasGeo = true
		geoPoint = domain.LocationPoint{Latitude: lat, Longitude: lng}
	}

	if rStr := q.Get("radiusMiles"); rStr != "" {
		rVal, err := strconv.ParseFloat(rStr, 64)
		if err != nil || math.IsNaN(rVal) || math.IsInf(rVal, 0) || rVal <= 0 {
			return QueryParams{}, errors.New("invalid radiusMiles: must be a positive number")
		}
		radiusMiles = rVal
	}

	return QueryParams{
		Limit:       limit,
		Offset:      offset,
		Species:     species,
		Status:      status,
		HasGeo:      hasGeo,
		GeoPoint:    geoPoint,
		RadiusMiles: radiusMiles,
	}, nil
}

func (s *Server) handleApiLostPets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		params, err := parseQueryParams(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if params.Status != "" && !strings.EqualFold(params.Status, "all") && !strings.EqualFold(params.Status, "lost") {
			w.Header().Set("X-Total-Count", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]domain.PublicLostPetReport{})
			return
		}

		rawItems, err := s.stateStore.ListState(r.Context(), store.LostPetsCollection)
		if err != nil {
			http.Error(w, "Failed to query lost pets", http.StatusInternalServerError)
			return
		}

		pets := make([]domain.LostPetRecord, 0, len(rawItems))
		for _, b := range rawItems {
			var pet domain.LostPetRecord
			if err := json.Unmarshal(b, &pet); err == nil {
				pet = domain.NormalizeLostPetRecord(pet)
				if !pet.Status.IsActive() {
					continue
				}
				// Species filter check
				if params.Species != "" && !strings.EqualFold(params.Species, "all") {
					if pet.Species == "" {
						continue
					}
					if strings.EqualFold(params.Species, "other") {
						if strings.EqualFold(pet.Species, "dog") || strings.EqualFold(pet.Species, "cat") {
							continue
						}
					} else if !strings.EqualFold(pet.Species, params.Species) {
						continue
					}
				}
				// Geo radius filter
				if params.HasGeo {
					locPt, ok := extractCoordinates(pet.Coordinates, pet.Location)
					if !ok {
						continue
					}
					dist := domain.HaversineDistanceMiles(params.GeoPoint, locPt)
					if dist > params.RadiusMiles {
						continue
					}
				}
				pets = append(pets, pet)
			}
		}

		// Sort deterministically: ReportedAt DESC, PetID DESC
		sort.Slice(pets, func(i, j int) bool {
			if !pets[i].ReportedAt.Equal(pets[j].ReportedAt) {
				return pets[i].ReportedAt.After(pets[j].ReportedAt)
			}
			return pets[i].PetID > pets[j].PetID
		})

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

	if err := domain.ValidatePetImages(req.Images); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	imageObject := req.ImageObject
	if primary, ok := domain.PrimaryPetImage(req.Images); ok && primary.Object != "" {
		imageObject = primary.Object
	} else if len(req.Images) > 0 && req.Images[0].Object != "" {
		imageObject = req.Images[0].Object
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
		MicrochipID:   req.MicrochipID,
		ImageObject:   imageObject,
		Images:        req.Images,
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
	PetID               string                `json:"petId"`
	ImageURL            string                `json:"imageUrl"`
	ImageObject         string                `json:"imageObject,omitempty"`
	Images              []domain.PetImage     `json:"images,omitempty"`
	Location            string                `json:"location"`
	Coordinates         *domain.LocationPoint `json:"coordinates,omitempty"`
	Description         string                `json:"description,omitempty"`
	FinderEmail         string                `json:"finderEmail"`
	Species             string                `json:"species"`
	Breed               string                `json:"breed"`
	PrimaryColor        string                `json:"primaryColor"`
	SecondaryColor      string                `json:"secondaryColor"`
	DistinctiveMarkings []string              `json:"distinctiveMarkings"`
	CustodyStatus       domain.CustodyStatus  `json:"custodyStatus"`
	MicrochipID         string                `json:"microchipId,omitempty"`
	FoundAt             time.Time             `json:"foundAt"`
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
		params, err := parseQueryParams(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if params.Status != "" && !strings.EqualFold(params.Status, "all") && !strings.EqualFold(params.Status, "found") {
			w.Header().Set("X-Total-Count", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]domain.PublicFoundPetReport{})
			return
		}

		rawItems, err := s.stateStore.ListState(r.Context(), store.FoundPetsCollection)
		if err != nil {
			http.Error(w, "Failed to query found pets", http.StatusInternalServerError)
			return
		}

		pets := make([]domain.FoundPetRecord, 0, len(rawItems))
		for _, b := range rawItems {
			var pet domain.FoundPetRecord
			if err := json.Unmarshal(b, &pet); err == nil {
				pet = domain.NormalizeFoundPetRecord(pet)
				if !pet.Status.IsActive() {
					continue
				}
				// Species filter check
				if params.Species != "" && !strings.EqualFold(params.Species, "all") {
					if pet.Species == "" {
						continue
					}
					if strings.EqualFold(params.Species, "other") {
						if strings.EqualFold(pet.Species, "dog") || strings.EqualFold(pet.Species, "cat") {
							continue
						}
					} else if !strings.EqualFold(pet.Species, params.Species) {
						continue
					}
				}
				// Geo filter check
				if params.HasGeo {
					locPt, ok := extractCoordinates(pet.Coordinates, pet.Location)
					if !ok {
						continue
					}
					dist := domain.HaversineDistanceMiles(params.GeoPoint, locPt)
					if dist > params.RadiusMiles {
						continue
					}
				}
				pets = append(pets, pet)
			}
		}

		// Sort deterministically: FoundAt DESC, PetID DESC
		sort.Slice(pets, func(i, j int) bool {
			if !pets[i].FoundAt.Equal(pets[j].FoundAt) {
				return pets[i].FoundAt.After(pets[j].FoundAt)
			}
			return pets[i].PetID > pets[j].PetID
		})

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

	if err := domain.ValidatePetImages(req.Images); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var imageObject string
	if primary, ok := domain.PrimaryPetImage(req.Images); ok && primary.Object != "" {
		imageObject = primary.Object
	} else if len(req.Images) > 0 && req.Images[0].Object != "" {
		imageObject = req.Images[0].Object
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
		ImageObject:         imageObject,
		Images:              req.Images,
		FoundAt:             foundAt,
		Location:            req.Location,
		FinderEmail:         finderEmail,
		Species:             req.Species,
		Breed:               req.Breed,
		PrimaryColor:        req.PrimaryColor,
		SecondaryColor:      req.SecondaryColor,
		DistinctiveMarkings: req.DistinctiveMarkings,
		CustodyStatus:       req.CustodyStatus,
		MicrochipID:         req.MicrochipID,
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

	var authorized map[string]domain.MatchParticipantRecord
	if s.identitySessions != nil {
		w.Header().Set("Cache-Control", "no-store")
		principal, ok := s.verifiedRequestPrincipal(w, r)
		if !ok {
			return
		}
		var err error
		authorized, err = s.authorizedMatches(r.Context(), principal)
		if err != nil {
			http.Error(w, "Failed to authorize matches", http.StatusInternalServerError)
			return
		}
	}

	rawMatches, err := s.stateStore.ListState(r.Context(), store.MatchesCollection)
	if err != nil {
		http.Error(w, "Failed to query matches from state store", http.StatusInternalServerError)
		return
	}

	matches := make([]MatchRecord, 0, len(rawMatches))
	for key, b := range rawMatches {
		var m MatchRecord
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if authorized != nil {
			participants, ok := authorized[key]
			if !ok || m.MatchID != key || participants.MatchID != key ||
				participants.LostPetID != m.MatchedPetID || participants.FoundPetID != m.FoundPetID {
				continue
			}
		}
		matches = append(matches, m)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(matches)
}

func (s *Server) authorizedMatches(
	ctx context.Context,
	principal identity.Principal,
) (map[string]domain.MatchParticipantRecord, error) {
	rawParticipants, err := s.stateStore.ListState(ctx, store.MatchParticipantsCollection)
	if err != nil {
		return nil, err
	}
	authorized := make(map[string]domain.MatchParticipantRecord)
	for key, data := range rawParticipants {
		var participants domain.MatchParticipantRecord
		if err := json.Unmarshal(data, &participants); err != nil || participants.MatchID != key ||
			participants.Validate() != nil {
			continue
		}
		if principalMatchesRef(principal, participants.Reporter) || principalMatchesRef(principal, participants.Finder) {
			authorized[key] = participants
		}
	}
	return authorized, nil
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
	if s.identitySessions != nil {
		s.handleAuthenticatedMatchAction(w, r)
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

var errMatchActionHidden = errors.New("webfrontend: match is not actionable by principal")

func (s *Server) handleAuthenticatedMatchAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := s.verifiedRequestPrincipal(w, r)
	if !ok {
		return
	}
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req MatchActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	req.MatchID = strings.TrimSpace(req.MatchID)
	action := domain.MatchDecision(strings.ToUpper(strings.TrimSpace(req.Action)))
	if req.MatchID == "" || (action != domain.MatchDecisionConfirm && action != domain.MatchDecisionReject) {
		http.Error(w, "matchId and action (confirm or reject) are required", http.StatusBadRequest)
		return
	}

	decisionStore, ok := s.stateStore.(store.MatchStateStore)
	if !ok {
		http.Error(w, "Match decision storage is unavailable", http.StatusInternalServerError)
		return
	}
	actor := domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject}
	decidedAt := time.Now().UTC()
	status := domain.MatchStatusPendingReview
	err := decisionStore.UpdateMatchAndParticipants(
		r.Context(), req.MatchID,
		func(matchData, participantData []byte) ([]byte, []byte, error) {
			var match domain.MatchRecord
			if err := json.Unmarshal(matchData, &match); err != nil {
				return nil, nil, fmt.Errorf("decode match: %w", err)
			}
			var participants domain.MatchParticipantRecord
			if err := json.Unmarshal(participantData, &participants); err != nil {
				return nil, nil, errMatchActionHidden
			}
			if match.MatchID != req.MatchID || participants.MatchID != req.MatchID ||
				participants.LostPetID != match.MatchedPetID || participants.FoundPetID != match.FoundPetID ||
				participants.Validate() != nil {
				return nil, nil, errMatchActionHidden
			}

			nextParticipants, nextStatus, changed, err := participants.ApplyDecision(actor, action, decidedAt)
			if errors.Is(err, domain.ErrNotMatchParticipant) || errors.Is(err, domain.ErrIncompleteMatchParticipants) {
				return nil, nil, errMatchActionHidden
			}
			if err != nil {
				return nil, nil, err
			}
			if match.Status != domain.MatchStatusPendingReview {
				if changed || match.Status != nextStatus {
					return nil, nil, domain.ErrMatchDecisionConflict
				}
			}
			match.Status = nextStatus
			nextMatchData, err := json.Marshal(match)
			if err != nil {
				return nil, nil, err
			}
			nextParticipantData, err := json.Marshal(nextParticipants)
			if err != nil {
				return nil, nil, err
			}
			status = nextStatus
			return nextMatchData, nextParticipantData, nil
		},
	)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) || errors.Is(err, errMatchActionHidden) {
		http.NotFound(w, r)
		return
	}
	if errors.Is(err, domain.ErrMatchDecisionConflict) {
		http.Error(w, "Match decision conflicts with the accepted decision", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "Failed to update match decision", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"matchId": req.MatchID,
		"status":  string(status),
		"message": fmt.Sprintf("Match status updated to %s", status),
	})
}

type ReunionContactRequest struct {
	MatchID     string   `json:"matchId"`
	SenderEmail string   `json:"senderEmail"`
	Message     string   `json:"message"`
	Images      []string `json:"images,omitempty"`
}

func (s *Server) handleApiReunionContact(w http.ResponseWriter, r *http.Request) {
	if s.identitySessions != nil {
		s.handleAuthenticatedMediatedContact(w, r)
		return
	}
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
	if s.identitySessions != nil {
		s.handleGlobalOperatorReunionResolve(w, r)
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

	if s.reunionHub != nil {
		s.reunionHub.BroadcastLocal(domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_resolved_%s", req.MatchID),
			Type:      domain.ReunionEventResolved,
			MatchID:   req.MatchID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]any{
				"matchId": req.MatchID,
				"petId":   req.PetID,
				"status":  string(domain.MatchStatusReunited),
			},
		})
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

var errOperatorReunionHidden = errors.New("webfrontend: operator reunion target is unavailable")

func (s *Server) handleGlobalOperatorReunionResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, ok := s.verifiedRequestPrincipal(w, r)
	if !ok {
		return
	}
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	actor := domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject}
	globalScope := domain.RoleScope{Kind: domain.RoleScopeGlobal}
	assignments, ok := s.stateStore.(store.RoleAssignmentStore)
	if !ok {
		http.Error(w, "Operator authorization is unavailable", http.StatusServiceUnavailable)
		return
	}
	assignment, err := assignments.GetRoleAssignment(r.Context(), actor, domain.RoleOperator, globalScope)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) ||
		(err == nil && assignment.Status != domain.RoleAssignmentStatusActive) {
		http.Error(w, "Operator authorization required", http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, "Operator authorization is unavailable", http.StatusServiceUnavailable)
		return
	}
	operationID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if operationID == "" {
		http.Error(w, "Idempotency-Key is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req ReunionResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	req.MatchID = strings.TrimSpace(req.MatchID)
	req.PetID = strings.TrimSpace(req.PetID)
	if req.MatchID == "" || req.PetID == "" {
		http.Error(w, "matchId and petId are required", http.StatusBadRequest)
		return
	}

	roleStore, ok := s.stateStore.(store.RoleAuthorizedMatchStateStore)
	if !ok {
		http.Error(w, "Operator authorization is unavailable", http.StatusServiceUnavailable)
		return
	}
	resolvedAt := time.Now().UTC()
	err = roleStore.UpdateMatchAndParticipantsAsRole(
		r.Context(),
		actor,
		domain.RoleOperator,
		globalScope,
		req.MatchID,
		func(authorization domain.RoleAssignment, matchData, participantData []byte) ([]byte, []byte, error) {
			var match domain.MatchRecord
			if err := json.Unmarshal(matchData, &match); err != nil {
				return nil, nil, errOperatorReunionHidden
			}
			var participants domain.MatchParticipantRecord
			if err := json.Unmarshal(participantData, &participants); err != nil {
				return nil, nil, errOperatorReunionHidden
			}
			if match.MatchID != req.MatchID || match.MatchedPetID != req.PetID {
				return nil, nil, errOperatorReunionHidden
			}
			nextMatch, nextParticipants, _, err := domain.ApplyGlobalOperatorReunion(
				match, participants, actor, authorization, operationID, req.Rating, req.Feedback, resolvedAt,
			)
			if err != nil {
				return nil, nil, err
			}
			nextMatchData, err := json.Marshal(nextMatch)
			if err != nil {
				return nil, nil, err
			}
			nextParticipantData, err := json.Marshal(nextParticipants)
			if err != nil {
				return nil, nil, err
			}
			return nextMatchData, nextParticipantData, nil
		},
	)
	switch {
	case errors.Is(err, store.ErrRoleDenied):
		http.Error(w, "Operator authorization required", http.StatusForbidden)
		return
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrStoreNotFound), errors.Is(err, errOperatorReunionHidden):
		http.NotFound(w, r)
		return
	case errors.Is(err, domain.ErrMatchReunionConflict):
		http.Error(w, "Reunion resolution conflicts with persisted state", http.StatusConflict)
		return
	case errors.Is(err, domain.ErrInvalidMatchReunion):
		http.Error(w, "Invalid reunion resolution", http.StatusBadRequest)
		return
	case err != nil:
		http.Error(w, "Failed to resolve reunion", http.StatusInternalServerError)
		return
	}

	if s.reunionHub != nil {
		s.reunionHub.BroadcastLocal(domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_resolved_%s", req.MatchID),
			Type:      domain.ReunionEventResolved,
			MatchID:   req.MatchID,
			Timestamp: resolvedAt,
			Payload: map[string]any{
				"matchId": req.MatchID,
				"petId":   req.PetID,
				"status":  string(domain.MatchStatusReunited),
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"matchId":  req.MatchID,
		"petId":    req.PetID,
		"status":   string(domain.MatchStatusReunited),
		"rating":   req.Rating,
		"feedback": strings.TrimSpace(req.Feedback),
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

// ServeHTTP satisfies the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", s.securityPolicy())
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
	w.Header().Set("X-Frame-Options", "DENY")
	s.handler.ServeHTTP(w, r)
}

func (s *Server) securityPolicy() string {
	if !s.identityClientConfig.Enabled {
		return contentSecurityPolicy
	}
	connectSources := "'self' https://storage.petspotr.io https://identitytoolkit.googleapis.com https://securetoken.googleapis.com"
	frameSources := "https://" + s.identityClientConfig.AuthDomain
	if emulatorURL := strings.TrimSpace(s.identityClientConfig.AuthEmulatorURL); emulatorURL != "" {
		connectSources += " " + emulatorURL
		frameSources += " " + emulatorURL
	}
	return "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; " +
		"form-action 'self'; script-src 'self' https://www.gstatic.com; " +
		"style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; " +
		"img-src 'self' data: blob: https://storage.petspotr.io; connect-src " + connectSources +
		"; frame-src " + frameSources + "; worker-src 'self'"
}

// RateLimiter returns the configured rate limiter.
func (s *Server) RateLimiter() ratelimit.Limiter {
	return s.rateLimiter
}

// Close releases server resources including background rate limiting workers.
func (s *Server) Close() {
	if s.rateLimiter != nil {
		s.rateLimiter.Close()
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	http.Error(w, message, code)
}

type PosterViewModel struct {
	PetID               string
	PetName             string
	Species             string
	SpeciesUpper        string
	Breed               string
	Gender              string
	PrimaryColor        string
	DistinctiveMarkings []string
	LastSeenDate        string
	Location            string
	Description         string
	PhotoURL            string
	HasPhoto            bool
	RewardText          string
	HasReward           bool
	EmergencyText       string
	HasEmergency        bool
	ShortURL            string
	Tabs                []PosterTabViewModel
}

type PosterTabViewModel struct {
	PetName     string
	PetID       string
	ShortURL    string
	ContactInfo string
}

func (s *Server) handlePetPoster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		path := strings.TrimPrefix(r.URL.Path, "/pets/")
		petID = strings.TrimSuffix(path, "/poster")
	}
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	pet, err := s.getLostPetRecord(r.Context(), petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load pet record", http.StatusInternalServerError)
		return
	}

	petName := strings.TrimSpace(pet.PetName)
	if petName == "" {
		petName = "Missing Pet"
	}

	species := strings.TrimSpace(pet.Species)
	if species == "" {
		species = "Pet"
	}

	q := r.URL.Query()
	rewardParam := strings.TrimSpace(q.Get("reward"))
	hasReward := rewardParam != ""
	rewardText := rewardParam
	if hasReward && !strings.Contains(strings.ToUpper(rewardParam), "REWARD") {
		rewardText = rewardParam + " REWARD"
	}

	emergencyParam := strings.TrimSpace(q.Get("emergency"))
	hasEmergency := emergencyParam != ""

	phoneParam := strings.TrimSpace(q.Get("phone"))
	tabContact := "Scan QR to Help"
	if phoneParam != "" {
		tabContact = phoneParam
	}

	photoURL := strings.TrimSpace(pet.ImageURL)
	if photoURL == "" {
		if strings.TrimSpace(pet.ImageObject) != "" {
			photoURL = pet.ImageObject
		} else if len(pet.Images) > 0 && strings.TrimSpace(pet.Images[0].Object) != "" {
			photoURL = pet.Images[0].Object
		}
	}

	lastSeenDate := ""
	if !pet.ReportedAt.IsZero() {
		lastSeenDate = pet.ReportedAt.Format("Jan 02, 2006")
	}

	shortURL := fmt.Sprintf("%s/p/%s", determineRequestBaseURL(r), pet.PetID)

	tabs := make([]PosterTabViewModel, 10)
	for i := 0; i < 10; i++ {
		tabs[i] = PosterTabViewModel{
			PetName:     petName,
			PetID:       pet.PetID,
			ShortURL:    "/p/" + pet.PetID,
			ContactInfo: tabContact,
		}
	}

	viewModel := PosterViewModel{
		PetID:               pet.PetID,
		PetName:             petName,
		Species:             species,
		SpeciesUpper:        strings.ToUpper(species),
		Breed:               strings.TrimSpace(pet.Breed),
		Gender:              strings.TrimSpace(pet.Gender),
		PrimaryColor:        strings.TrimSpace(pet.PrimaryColor),
		DistinctiveMarkings: pet.DistinctiveMarkings,
		LastSeenDate:        lastSeenDate,
		Location:            extractLocationString(pet),
		Description:         strings.TrimSpace(pet.Description),
		PhotoURL:            photoURL,
		HasPhoto:            photoURL != "",
		RewardText:          rewardText,
		HasReward:           hasReward,
		EmergencyText:       emergencyParam,
		HasEmergency:        hasEmergency,
		ShortURL:            shortURL,
		Tabs:                tabs,
	}

	tmpl, err := template.ParseFS(embeddedFiles, "templates/poster.html")
	if err != nil {
		http.Error(w, "Failed to load poster template", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, viewModel); err != nil {
		http.Error(w, "Failed to render poster template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

type FinderLandingViewModel struct {
	PetID               string
	PetName             string
	Species             string
	SpeciesUpper        string
	Breed               string
	Gender              string
	PrimaryColor        string
	DistinctiveMarkings []string
	LastSeenDate        string
	Location            string
	Description         string
	PhotoURL            string
	HasPhoto            bool
	RewardText          string
	HasReward           bool
	EmergencyText       string
	HasEmergency        bool
	ShortURL            string
	BaseURL             string
	ShareCardURL        string
	SocialDescription   string
	ReportFoundURL      string
	ShortURLEncoded     string
	ShareTextEncoded    string
}

func (s *Server) handleFinderLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		path := strings.TrimPrefix(r.URL.Path, "/p/")
		petID = strings.TrimSpace(path)
	}
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	pet, err := s.getLostPetRecord(r.Context(), petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load pet record", http.StatusInternalServerError)
		return
	}

	petName := strings.TrimSpace(pet.PetName)
	if petName == "" {
		petName = "Missing Pet"
	}

	species := strings.TrimSpace(pet.Species)
	if species == "" {
		species = "Pet"
	}

	q := r.URL.Query()
	rewardParam := strings.TrimSpace(q.Get("reward"))
	hasReward := rewardParam != ""
	rewardText := rewardParam
	if hasReward && !strings.Contains(strings.ToUpper(rewardParam), "REWARD") {
		rewardText = rewardParam + " REWARD"
	}

	emergencyParam := strings.TrimSpace(q.Get("emergency"))
	hasEmergency := emergencyParam != ""

	photoURL := strings.TrimSpace(pet.ImageURL)
	if photoURL == "" {
		if strings.TrimSpace(pet.ImageObject) != "" {
			photoURL = pet.ImageObject
		} else if len(pet.Images) > 0 && strings.TrimSpace(pet.Images[0].Object) != "" {
			photoURL = pet.Images[0].Object
		}
	}

	lastSeenDate := ""
	if !pet.ReportedAt.IsZero() {
		lastSeenDate = pet.ReportedAt.Format("Jan 02, 2006")
	}

	baseURL := determineRequestBaseURL(r)
	shareCardURL := fmt.Sprintf("%s/api/v1/pets/%s/share-card.svg", baseURL, pet.PetID)
	shortURL := fmt.Sprintf("%s/p/%s", baseURL, pet.PetID)
	location := extractLocationString(pet)

	socialDesc := fmt.Sprintf("Help find %s, a lost %s last seen in %s. Tap to report sightings or contact the owner securely.", petName, pet.Breed, location)
	if strings.TrimSpace(pet.Description) != "" {
		socialDesc = fmt.Sprintf("Help find %s: %s", petName, strings.TrimSpace(pet.Description))
	}

	viewModel := FinderLandingViewModel{
		PetID:               pet.PetID,
		PetName:             petName,
		Species:             species,
		SpeciesUpper:        strings.ToUpper(species),
		Breed:               strings.TrimSpace(pet.Breed),
		Gender:              strings.TrimSpace(pet.Gender),
		PrimaryColor:        strings.TrimSpace(pet.PrimaryColor),
		DistinctiveMarkings: pet.DistinctiveMarkings,
		LastSeenDate:        lastSeenDate,
		Location:            location,
		Description:         strings.TrimSpace(pet.Description),
		PhotoURL:            photoURL,
		HasPhoto:            photoURL != "",
		RewardText:          rewardText,
		HasReward:           hasReward,
		EmergencyText:       emergencyParam,
		HasEmergency:        hasEmergency,
		ShortURL:            shortURL,
		BaseURL:             baseURL,
		ShareCardURL:        shareCardURL,
		SocialDescription:   socialDesc,
		ReportFoundURL:      fmt.Sprintf("/report-found?matchedPetId=%s", pet.PetID),
		ShortURLEncoded:     url.QueryEscape(shortURL),
		ShareTextEncoded:    url.QueryEscape(fmt.Sprintf("Help find %s! %s", petName, shortURL)),
	}

	tmpl, err := template.ParseFS(embeddedFiles, "templates/finder_landing.html")
	if err != nil {
		http.Error(w, "Failed to load finder landing template", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, viewModel); err != nil {
		http.Error(w, "Failed to render finder landing template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(self), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(buf.Bytes())
	}
}
