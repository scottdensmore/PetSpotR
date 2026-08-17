package webfrontend

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

var errStateWrite = errors.New("state write failed")

type failingSaveStore struct {
	*store.MemoryStore
}

func (s *failingSaveStore) SaveState(context.Context, string, string, []byte) error {
	return errStateWrite
}

func (s *failingSaveStore) CreateStateAndOutbox(context.Context, store.StateWrite, store.StateWrite) (bool, error) {
	return false, errStateWrite
}

func (s *failingSaveStore) CreateStatesAndOutbox(context.Context, []store.StateWrite, store.StateWrite) (bool, error) {
	return false, errStateWrite
}

func TestNewServer_Routes(t *testing.T) {
	srv := NewDemoServer()

	t.Run("GET / returns 200 OK with HTML layout shell", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("expected body to contain DOCTYPE html")
		}
		if !strings.Contains(body, "PetSpotR") {
			t.Error("expected body to contain PetSpotR title")
		}
		if !strings.Contains(body, "theme-toggle") {
			t.Error("expected body to contain theme-toggle element")
		}
	})

	t.Run("GET /healthz returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "OK") {
			t.Errorf("expected body OK, got %s", rec.Body.String())
		}
	})

	t.Run("GET /static/css/styles.css returns CSS content", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/css/styles.css", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK for CSS, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/css") && !strings.Contains(rec.Body.String(), "--bg-primary") {
			t.Error("expected CSS content with design system tokens")
		}
	})

	t.Run("GET /report-lost returns 200 OK with wizard page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/report-lost", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "lost-wizard") {
			t.Error("expected body to contain lost-wizard container")
		}
		if !strings.Contains(body, "Report Lost Pet") {
			t.Error("expected body to contain Report Lost Pet heading")
		}
	})

	t.Run("POST /api/v1/lost-pets with valid payload returns 201 Created", func(t *testing.T) {
		payload := `{"petName":"Buddy","reporterEmail":"owner@example.com","location":"Seattle, WA"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "success") {
			t.Errorf("expected success response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/lost-pets with invalid payload returns 400 Bad Request", func(t *testing.T) {
		payload := `{"petName":"","reporterEmail":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("GET /report-found returns 200 OK with report found page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/report-found", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "found-report") {
			t.Error("expected body to contain found-report container")
		}
		if !strings.Contains(body, "Report Found Pet") {
			t.Error("expected body to contain Report Found Pet heading")
		}
	})

	t.Run("POST /api/v1/found-pets/extract-features returns 200 OK with AI traits", func(t *testing.T) {
		payload := `{"imageUrl":"https://storage.petspotr.io/found-sample.jpg"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "species") || !strings.Contains(rec.Body.String(), "primaryColor") {
			t.Errorf("expected AI traits in response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/found-pets with valid payload returns 201 Created", func(t *testing.T) {
		payload := `{"imageUrl":"https://storage.petspotr.io/found-1.jpg","location":"Capitol Hill, Seattle, WA"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "success") {
			t.Errorf("expected success response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/found-pets with invalid payload returns 400 Bad Request", func(t *testing.T) {
		payload := `{"imageUrl":"","location":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("GET /matches returns 200 OK with match dashboard page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/matches", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "matches-dashboard") {
			t.Error("expected body to contain matches-dashboard container")
		}
		if !strings.Contains(body, "Match Comparison Dashboard") {
			t.Error("expected body to contain Match Comparison Dashboard heading")
		}
	})

	t.Run("GET /api/v1/matches returns 200 OK with match records", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "foundPetId") || !strings.Contains(rec.Body.String(), "score") {
			t.Errorf("expected match records in response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/matches/action handles match confirmation", func(t *testing.T) {
		payload := `{"matchId":"match-101","action":"confirm"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/matches/action", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "CONFIRMED") {
			t.Errorf("expected status CONFIRMED in response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/reunions/contact dispatches secure owner message", func(t *testing.T) {
		payload := `{"matchId":"match-101","senderEmail":"owner@example.com","message":"Hello! I believe this is my dog Buddy."}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/contact", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "sent") {
			t.Errorf("expected message sent confirmation, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/reunions/resolve updates status to REUNITED with feedback", func(t *testing.T) {
		payload := `{"matchId":"match-101","petId":"lost-101","rating":5,"feedback":"Gemma 4 AI matched Buddy perfectly!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/resolve", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "REUNITED") {
			t.Errorf("expected status REUNITED in response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/reunions/resolve with invalid payload returns 400 Bad Request", func(t *testing.T) {
		payload := `{"matchId":"","petId":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/resolve", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("GET /sw.js serves Service Worker script", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "showNotification") {
			t.Errorf("expected service worker script content, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/push/subscribe registers web push subscription", func(t *testing.T) {
		payload := `{"endpoint":"https://fcm.googleapis.com/fcm/send/sample-token","keys":{"p256dh":"key1","auth":"auth1"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "subscribed") {
			t.Errorf("expected subscribed confirmation in response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/push/subscribe with missing endpoint returns 400 Bad Request", func(t *testing.T) {
		payload := `{"endpoint":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/push/test dispatches test push payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/push/test", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "title") || !strings.Contains(rec.Body.String(), "body") {
			t.Errorf("expected test push payload in response, got %s", rec.Body.String())
		}
	})

	t.Run("POST /api/v1/uploads/presigned-url generates direct GCS upload URL", func(t *testing.T) {
		payload := `{"fileName":"pet-photo.jpg","contentType":"image/jpeg"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/presigned-url", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "uploadUrl") || !strings.Contains(rec.Body.String(), "publicUrl") {
			t.Errorf("expected presigned upload URL response, got %s", rec.Body.String())
		}
	})

	t.Run("GET /api/v1/lost-pets returns persisted lost pets from StateStore", func(t *testing.T) {
		payload := `{"petName":"Rover","reporterEmail":"rover@example.com","location":"Portland, OR"}`
		reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		reqPost.Header.Set("Content-Type", "application/json")
		recPost := httptest.NewRecorder()
		srv.ServeHTTP(recPost, reqPost)
		if recPost.Code != http.StatusCreated {
			t.Fatalf("failed to create lost pet: %d", recPost.Code)
		}

		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets", nil)
		recGet := httptest.NewRecorder()
		srv.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", recGet.Code)
		}
		if strings.Contains(recGet.Body.String(), "rover@example.com") || strings.Contains(recGet.Body.String(), "reporterEmail") {
			t.Errorf("expected GET /api/v1/lost-pets to redact reporter contact, got %s", recGet.Body.String())
		}
		if !strings.Contains(recGet.Body.String(), "Portland, OR") {
			t.Errorf("expected GET /api/v1/lost-pets to contain the public report, got %s", recGet.Body.String())
		}
	})

	t.Run("GET /api/v1/found-pets returns persisted found pets from StateStore", func(t *testing.T) {
		payload := `{"imageUrl":"https://storage.petspotr.io/found-rover.jpg","location":"Portland, OR"}`
		reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
		reqPost.Header.Set("Content-Type", "application/json")
		recPost := httptest.NewRecorder()
		srv.ServeHTTP(recPost, reqPost)
		if recPost.Code != http.StatusCreated {
			t.Fatalf("failed to create found pet: %d", recPost.Code)
		}

		reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/found-pets", nil)
		recGet := httptest.NewRecorder()
		srv.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", recGet.Code)
		}
		if !strings.Contains(recGet.Body.String(), "found-rover.jpg") {
			t.Errorf("expected GET /api/v1/found-pets to contain persisted record, got %s", recGet.Body.String())
		}
	})

	t.Run("GET /api/v1/lost-pets supports pagination and spatial radius filtering", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		stSrv := NewServerWithStore(memStore)

		// Seed 3 lost pet reports with different locations
		pet1 := `{"petName":"CapitolPet","reporterEmail":"p1@example.com","location":"Capitol Hill, Seattle, WA"}`
		pet2 := `{"petName":"BallardPet","reporterEmail":"p2@example.com","location":"Ballard, Seattle, WA"}`
		pet3 := `{"petName":"GreenLakePet","reporterEmail":"p3@example.com","location":"Green Lake, Seattle, WA"}`

		for _, p := range []string{pet1, pet2, pet3} {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(p))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			stSrv.ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated {
				t.Fatalf("failed to post lost pet: %d", rec.Code)
			}
		}

		// 1. Pagination limit=1 & offset=0
		reqPag := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets?limit=1&offset=0", nil)
		recPag := httptest.NewRecorder()
		stSrv.ServeHTTP(recPag, reqPag)
		if recPag.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", recPag.Code)
		}
		if recPag.Header().Get("X-Total-Count") != "3" {
			t.Errorf("expected X-Total-Count header to be 3, got %s", recPag.Header().Get("X-Total-Count"))
		}

		// 2. Spatial radius filtering around Capitol Hill (47.6150, -122.3200) within 3 miles
		reqGeo := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets?lat=47.6150&lng=-122.3200&radiusMiles=3", nil)
		recGeo := httptest.NewRecorder()
		stSrv.ServeHTTP(recGeo, reqGeo)
		if recGeo.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", recGeo.Code)
		}
		if !strings.Contains(recGeo.Body.String(), "capitolpet") {
			t.Errorf("expected spatial radius filter result to contain capitolpet, got %s", recGeo.Body.String())
		}
	})

	t.Run("GET /api/v1/found-pets supports species filtering", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		stSrv := NewServerWithStore(memStore)

		dog := `{"imageUrl":"https://storage.petspotr.io/dog.jpg","location":"Seattle, WA","species":"Dog"}`
		cat := `{"imageUrl":"https://storage.petspotr.io/cat.jpg","location":"Seattle, WA","species":"Cat"}`

		for _, p := range []string{dog, cat} {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(p))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			stSrv.ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated {
				t.Fatalf("failed to post found pet: %d", rec.Code)
			}
		}

		reqCat := httptest.NewRequest(http.MethodGet, "/api/v1/found-pets?species=Cat", nil)
		recCat := httptest.NewRecorder()
		stSrv.ServeHTTP(recCat, reqCat)
		if recCat.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", recCat.Code)
		}
		if !strings.Contains(recCat.Body.String(), "cat.jpg") {
			t.Errorf("expected species filter result to contain cat.jpg, got %s", recCat.Body.String())
		}
	})
}

func TestNewServerStartsWithoutDemoMatches(t *testing.T) {
	srv := NewServer()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	recorder := httptest.NewRecorder()

	srv.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/matches status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Fatalf("default matches = %s, want []", body)
	}
}

func TestSeedDemoMatchesExplicitly(t *testing.T) {
	memory := store.NewMemoryStore()
	if err := SeedDemoMatches(context.Background(), memory); err != nil {
		t.Fatalf("SeedDemoMatches() error = %v", err)
	}
	matches, err := memory.ListState(context.Background(), store.MatchesCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("seeded matches = %d, want 2", len(matches))
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	srv := NewServer()
	wantCSP := "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https://storage.petspotr.io; connect-src 'self'; worker-src 'self'"
	tests := []struct {
		name string
		path string
	}{
		{name: "HTML page", path: "/matches"},
		{name: "static asset", path: "/static/js/match-dashboard.js"},
		{name: "API response", path: "/api/v1/matches"},
		{name: "not found response", path: "/missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
				t.Errorf("Content-Security-Policy = %q, want %q", got, wantCSP)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
				t.Errorf("Referrer-Policy = %q, want no-referrer", got)
			}
			if got := rec.Header().Get("Permissions-Policy"); got != "camera=(), geolocation=(), microphone=()" {
				t.Errorf("Permissions-Policy = %q, want camera=(), geolocation=(), microphone=()", got)
			}
			if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("X-Frame-Options = %q, want DENY", got)
			}
		})
	}
}

func TestDurableStateFailuresAreNotReportedAsSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		payload   string
		seedMatch bool
	}{
		{
			name:    "lost pet report",
			path:    "/api/v1/lost-pets",
			payload: `{"petName":"Rover","reporterEmail":"rover@example.com","location":"Portland, OR"}`,
		},
		{
			name:    "found pet report",
			path:    "/api/v1/found-pets",
			payload: `{"imageUrl":"https://storage.petspotr.io/found-rover.jpg","location":"Portland, OR"}`,
		},
		{
			name:    "push subscription",
			path:    "/api/v1/push/subscribe",
			payload: `{"endpoint":"https://push.example.test/send/subscription","keys":{"p256dh":"key","auth":"auth"}}`,
		},
		{
			name:      "match action",
			path:      "/api/v1/matches/action",
			payload:   `{"matchId":"match-write-failure","action":"confirm"}`,
			seedMatch: true,
		},
		{
			name:      "reunion resolution",
			path:      "/api/v1/reunions/resolve",
			payload:   `{"matchId":"match-write-failure","petId":"lost-1","rating":5}`,
			seedMatch: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			memory := store.NewMemoryStore()
			if tt.seedMatch {
				record := []byte(`{"matchId":"match-write-failure","status":"PENDING_REVIEW"}`)
				if err := memory.SaveState(context.Background(), store.MatchesCollection, "match-write-failure", record); err != nil {
					t.Fatalf("seed match: %v", err)
				}
			}
			srv := NewServerWithOptions(&failingSaveStore{MemoryStore: memory}, ServerOptions{
				AllowPrivilegedMutations: true,
			})
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
			}
		})
	}
}

func TestMatchMutationsReturnNotFoundForUnknownMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path    string
		payload string
	}{
		{
			path:    "/api/v1/matches/action",
			payload: `{"matchId":"missing","action":"confirm"}`,
		},
		{
			path:    "/api/v1/reunions/resolve",
			payload: `{"matchId":"missing","petId":"lost-1"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
				AllowPrivilegedMutations: true,
			})
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}
}

func TestManagedServerRejectsPrivilegedMutations(t *testing.T) {
	t.Parallel()

	memory := store.NewMemoryStore()
	record := []byte(`{"matchId":"managed-match","status":"PENDING_REVIEW"}`)
	if err := memory.SaveState(context.Background(), store.MatchesCollection, "managed-match", record); err != nil {
		t.Fatalf("seed match: %v", err)
	}
	srv := NewServerWithStore(memory)

	tests := []struct {
		path    string
		payload string
	}{
		{
			path:    "/api/v1/matches/action",
			payload: `{"matchId":"managed-match","action":"confirm"}`,
		},
		{
			path:    "/api/v1/reunions/resolve",
			payload: `{"matchId":"managed-match","petId":"lost-1"}`,
		},
		{
			path:    "/api/v1/reunions/contact",
			payload: `{"matchId":"managed-match","senderEmail":"sender@example.com","message":"hello"}`,
		},
		{
			path:    "/api/v1/push/subscribe",
			payload: `{"endpoint":"https://push.example.test/send/managed","keys":{"p256dh":"key","auth":"auth"}}`,
		},
		{
			path:    "/api/v1/uploads/presigned-url",
			payload: `{"fileName":"caller.jpg","contentType":"image/jpeg"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
		})
	}

	got, err := memory.GetState(context.Background(), store.MatchesCollection, "managed-match")
	if err != nil {
		t.Fatalf("load match: %v", err)
	}
	if string(got) != string(record) {
		t.Fatalf("managed match was mutated: got %s, want %s", got, record)
	}
}

func TestMatchListDoesNotSeedInjectedStore(t *testing.T) {
	t.Parallel()

	memory := store.NewMemoryStore()
	srv := NewServerWithStore(memory)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	items, err := memory.ListState(context.Background(), store.MatchesCollection)
	if err != nil {
		t.Fatalf("ListState() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("persisted matches = %d, want 0", len(items))
	}
}
