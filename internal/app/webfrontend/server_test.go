package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
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

	t.Run("GET /static/favicon.svg returns the declared favicon", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/favicon.svg", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK for favicon, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "image/svg+xml") {
			t.Errorf("expected SVG favicon content type, got %q", got)
		}
	})

	t.Run("GET /healthz returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
			t.Errorf("expected body status ok, got %s", rec.Body.String())
		}
	})

	t.Run("GET /readyz returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"status":"ready"`) {
			t.Errorf("expected body status ready, got %s", rec.Body.String())
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

func TestRenderedPagesDeclareFavicon(t *testing.T) {
	srv := NewDemoServer()
	want := `<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">`

	for _, path := range []string{"/", "/report-lost", "/report-found", "/matches", "/pets"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200 OK, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), want) {
				t.Error("expected rendered page to declare the PetSpotR favicon")
			}
		})
	}
}

func TestRenderedPagesDeclareOfflineUI(t *testing.T) {
	srv := NewDemoServer()
	expectedSnippets := []string{
		`<link rel="manifest" href="/manifest.webmanifest">`,
		`<meta name="theme-color" content="#4f46e5">`,
		`id="offline-indicator"`,
		`class="offline-pill"`,
		`id="offline-status-text"`,
		`id="outbox-count-badge"`,
		`id="toast-container"`,
		`<script src="/static/js/outbox-sync.js" defer></script>`,
	}

	for _, path := range []string{"/", "/report-lost", "/report-found", "/matches", "/pets"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200 OK, got %d", rec.Code)
			}
			body := rec.Body.String()
			for _, snippet := range expectedSnippets {
				if !strings.Contains(body, snippet) {
					t.Errorf("path %s expected body to contain %q", path, snippet)
				}
			}
		})
	}

	t.Run("/pets directory notice banner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pets", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()
		wantBanner := `id="offline-directory-notice"`
		if !strings.Contains(body, wantBanner) {
			t.Errorf("/pets expected body to contain %q", wantBanner)
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
	wantCSP := "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https://storage.petspotr.io https://*.tile.openstreetmap.org; connect-src 'self' https://storage.petspotr.io; worker-src 'self'"
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

func TestIdentityModeClosesPrivilegedRoutesToUnauthorizedCallers(t *testing.T) {
	t.Parallel()

	const (
		matchID      = "identity-match"
		pushEndpoint = "https://push.example.test/send/identity-user"
		csrfToken    = "0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	originalMatch := []byte(`{"matchId":"identity-match","status":"PENDING_REVIEW"}`)

	tests := []struct {
		name    string
		path    string
		payload string
		verify  func(*testing.T, *store.MemoryStore)
	}{
		{
			name:    "reunion resolution",
			path:    "/api/v1/reunions/resolve",
			payload: `{"matchId":"identity-match","petId":"lost-1","rating":5}`,
			verify: func(t *testing.T, memory *store.MemoryStore) {
				t.Helper()
				got, err := memory.GetState(context.Background(), store.MatchesCollection, matchID)
				if err != nil {
					t.Fatalf("load match: %v", err)
				}
				if string(got) != string(originalMatch) {
					t.Fatalf("identity-mode match was mutated: got %s, want %s", got, originalMatch)
				}
			},
		},
		{
			name:    "push subscription",
			path:    "/api/v1/push/subscribe",
			payload: `{"endpoint":"https://push.example.test/send/identity-user","keys":{"p256dh":"key","auth":"auth"}}`,
			verify: func(t *testing.T, memory *store.MemoryStore) {
				t.Helper()
				_, err := memory.GetState(context.Background(), store.PushSubscriptionsCollection, pushEndpoint)
				if !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
					t.Fatalf("push subscription lookup error = %v, want not found", err)
				}
			},
		},
		{
			name:    "legacy upload grant",
			path:    "/api/v1/uploads/presigned-url",
			payload: `{"fileName":"caller.jpg","contentType":"image/jpeg"}`,
		},
	}

	callers := []struct {
		name          string
		authenticated bool
	}{
		{name: "anonymous"},
		{name: "ordinary user", authenticated: true},
	}
	for _, tt := range tests {
		tt := tt
		for _, caller := range callers {
			caller := caller
			t.Run(tt.name+"/"+caller.name, func(t *testing.T) {
				t.Parallel()

				memory := store.NewMemoryStore()
				if err := memory.SaveState(context.Background(), store.MatchesCollection, matchID, originalMatch); err != nil {
					t.Fatalf("seed match: %v", err)
				}
				srv := NewServerWithOptions(memory, ServerOptions{
					AllowPrivilegedMutations: true,
					IdentitySessions: &stubSessionManager{verified: identity.Principal{
						Issuer:  "https://securetoken.google.com/petspotr-test",
						Subject: "ordinary-user",
						Email:   "ordinary@example.com", EmailVerified: true,
					}},
				})
				req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.payload))
				req.Header.Set("Content-Type", "application/json")
				if caller.authenticated {
					req.Header.Set(csrfHeaderName, csrfToken)
					req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "ordinary-session"})
					req.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
				}
				rec := httptest.NewRecorder()

				srv.ServeHTTP(rec, req)

				wantStatus := http.StatusForbidden
				if tt.name == "reunion resolution" && !caller.authenticated {
					wantStatus = http.StatusUnauthorized
				}
				if rec.Code != wantStatus {
					t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
				}
				if tt.verify != nil {
					tt.verify(t, memory)
				}
			})
		}
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

func TestAccessibilityFeatures(t *testing.T) {
	t.Parallel()
	srv := NewDemoServer()

	t.Run("Report Lost template contains accessible dropzone and live regions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/report-lost", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		expectedSnippets := []string{
			`id="dropzone"`,
			`aria-label="Upload pet photo"`,
			`tabindex="0"`,
			`aria-describedby="dropzone-help"`,
			`id="dropzone-help"`,
			`Supports JPG, PNG, WEBP up to 10MB`,
			`id="lost-photo-status"`,
			`role="status"`,
			`aria-live="polite"`,
			`id="petName-error"`,
			`role="alert"`,
			`id="location-error"`,
			`id="reporterEmail-error"`,
			`id="photo-count-badge"`,
			`class="form-control photo-tag-select"`,
			`class="btn btn-secondary btn-sm btn-remove-photo"`,
			`Primary / Face`,
			`Coat Pattern`,
			`Collar & Tags`,
		}
		for _, snippet := range expectedSnippets {
			if !strings.Contains(body, snippet) {
				t.Errorf("report-lost missing expected accessibility snippet: %q", snippet)
			}
		}
	})

	t.Run("Report Found template contains accessible dropzone and live regions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/report-found", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		expectedSnippets := []string{
			`id="found-dropzone"`,
			`aria-label="Upload pet photo"`,
			`tabindex="0"`,
			`aria-describedby="found-dropzone-help"`,
			`id="found-dropzone-help"`,
			`Supports JPG, PNG, WEBP up to 10MB`,
			`id="found-photo-status"`,
			`role="status"`,
			`aria-live="polite"`,
			`id="found-location-error"`,
			`id="finder-email-error"`,
			`id="found-form-status"`,
			`role="alert"`,
			`id="photo-count-badge"`,
			`class="form-control photo-tag-select"`,
			`class="btn btn-secondary btn-sm btn-remove-photo"`,
			`Primary / Face`,
			`Coat Pattern`,
			`Collar & Tags`,
		}
		for _, snippet := range expectedSnippets {
			if !strings.Contains(body, snippet) {
				t.Errorf("report-found missing expected accessibility snippet: %q", snippet)
			}
		}
	})

	t.Run("Match Dashboard contains accessible modal dialogs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/matches", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		expectedSnippets := []string{
			`id="zoom-modal" class="modal-overlay zoom-overlay" role="dialog" aria-modal="true"`,
			`id="contact-modal" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="contact-modal-title"`,
			`id="contact-modal-title">Send Secure Message</h2>`,
			`id="reunion-modal" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="reunion-modal-title"`,
			`id="reunion-modal-title">Confirm Pet Reunion!`,
			`id="match-action-modal" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="action-modal-title"`,
			`id="match-thread-modal" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="match-thread-title"`,
		}
		for _, snippet := range expectedSnippets {
			if !strings.Contains(body, snippet) {
				t.Errorf("matches missing expected accessibility snippet: %q", snippet)
			}
		}
	})

	t.Run("Styles CSS contains focus-visible and reduced-motion media query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/css/styles.css", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		expectedSnippets := []string{
			`:focus-visible`,
			`@media (prefers-reduced-motion: reduce)`,
			`.push-status-banner`,
			`.field-error`,
			`.form-error-banner`,
		}
		for _, snippet := range expectedSnippets {
			if !strings.Contains(body, snippet) {
				t.Errorf("styles.css missing expected accessibility snippet: %q", snippet)
			}
		}
	})
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer()
}

func TestServer_MultiPhotoSubmission(t *testing.T) {
	srv := newTestServer(t)

	t.Run("POST /api/v1/lost-pets with 3 photos succeeds", func(t *testing.T) {
		payload := `{
			"petName": "Rusty",
			"species": "Dog",
			"breed": "Irish Setter",
			"location": "Capitol Hill, Seattle, WA",
			"reporterEmail": "owner@example.com",
			"images": [
				{"object": "images/lost-pets/rust-1/face.jpg", "tag": "primary"},
				{"object": "images/lost-pets/rust-1/coat.jpg", "tag": "coat"},
				{"object": "images/lost-pets/rust-1/collar.jpg", "tag": "collar"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/lost-pets with >3 photos returns 400", func(t *testing.T) {
		payload := `{
			"petName": "Rusty",
			"species": "Dog",
			"location": "Capitol Hill, Seattle, WA",
			"reporterEmail": "owner@example.com",
			"images": [
				{"object": "images/1.jpg"},
				{"object": "images/2.jpg"},
				{"object": "images/3.jpg"},
				{"object": "images/4.jpg"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/found-pets with 3 photos succeeds", func(t *testing.T) {
		payload := `{
			"species": "Dog",
			"breed": "Golden Retriever",
			"location": "Capitol Hill, Seattle, WA",
			"finderEmail": "finder@example.com",
			"images": [
				{"object": "images/found-pets/found-1/face.jpg", "tag": "primary"},
				{"object": "images/found-pets/found-1/coat.jpg", "tag": "coat"},
				{"object": "images/found-pets/found-1/collar.jpg", "tag": "collar"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/found-pets with >3 photos returns 400", func(t *testing.T) {
		payload := `{
			"species": "Dog",
			"location": "Capitol Hill, Seattle, WA",
			"finderEmail": "finder@example.com",
			"images": [
				{"object": "images/1.jpg"},
				{"object": "images/2.jpg"},
				{"object": "images/3.jpg"},
				{"object": "images/4.jpg"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/lost-pets mirrors primary photo to imageObject", func(t *testing.T) {
		payload := `{
			"petName": "Spot",
			"species": "Dog",
			"location": "Capitol Hill, Seattle, WA",
			"reporterEmail": "spot@example.com",
			"images": [
				{"object": "images/spot-face.jpg", "tag": "face"},
				{"object": "images/spot-primary.jpg", "tag": "primary"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Status string `json:"status"`
			PetID  string `json:"petId"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, err := srv.stateStore.GetState(context.Background(), store.LostPetsCollection, resp.PetID)
		if err != nil {
			t.Fatalf("get state: %v", err)
		}
		var record domain.LostPetRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatalf("unmarshal record: %v", err)
		}
		if record.ImageObject != "images/spot-primary.jpg" {
			t.Errorf("expected ImageObject to mirror primary 'images/spot-primary.jpg', got %q", record.ImageObject)
		}
		if len(record.Images) != 2 {
			t.Errorf("expected 2 images, got %d", len(record.Images))
		}
	})

	t.Run("POST /api/v1/found-pets mirrors primary photo to imageObject", func(t *testing.T) {
		payload := `{
			"species": "Cat",
			"location": "Capitol Hill, Seattle, WA",
			"finderEmail": "finder@example.com",
			"images": [
				{"object": "images/found-coat.jpg", "tag": "coat"},
				{"object": "images/found-primary.jpg", "tag": "primary"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Status string `json:"status"`
			PetID  string `json:"petId"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		data, err := srv.stateStore.GetState(context.Background(), store.FoundPetsCollection, resp.PetID)
		if err != nil {
			t.Fatalf("get state: %v", err)
		}
		var record domain.FoundPetRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatalf("unmarshal record: %v", err)
		}
		if record.ImageObject != "images/found-primary.jpg" {
			t.Errorf("expected ImageObject to mirror primary 'images/found-primary.jpg', got %q", record.ImageObject)
		}
		if len(record.Images) != 2 {
			t.Errorf("expected 2 images, got %d", len(record.Images))
		}
	})
}

func TestMatchesTemplate_ContainsMultimodalScoreClasses(t *testing.T) {
	content, err := embeddedFiles.ReadFile("templates/matches.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)
	if !strings.Contains(html, "matches-list-container") {
		t.Error("matches.html missing matches-list-container")
	}

	cssContent, err := embeddedFiles.ReadFile("static/css/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssContent)
	expectedCSSClasses := []string{
		".thumbnail-strip",
		".thumbnail-btn",
		".thumbnail-btn.is-active",
		".thumbnail-img",
		".score-vector",
		".tag-badge",
	}
	for _, class := range expectedCSSClasses {
		if !strings.Contains(css, class) {
			t.Errorf("styles.css missing expected class: %q", class)
		}
	}

	jsContent, err := embeddedFiles.ReadFile("static/js/match-dashboard.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsContent)
	expectedJSSnippets := []string{
		"thumbnail-strip",
		"thumbnail-btn",
		"thumbnail-img",
		"score-vector",
		"✨ Multimodal AI Vector Match:",
		"Discrete Trait Match:",
	}
	for _, snippet := range expectedJSSnippets {
		if !strings.Contains(js, snippet) {
			t.Errorf("match-dashboard.js missing expected snippet: %q", snippet)
		}
	}
}

func TestServer_MatchesAPI_VectorScoreAndMultiImage(t *testing.T) {
	memory := store.NewMemoryStore()
	srv := NewServerWithStore(memory)

	matchRecord := domain.MatchRecord{
		MatchID:      "match-vector-101",
		FoundPetID:   "found-301",
		MatchedPetID: "lost-301",
		Score:        0.95,
		Status:       domain.MatchStatusPendingReview,
		Scores: domain.MatchScoreBreakdown{
			Vector:        0.96,
			Trait:         0.92,
			Visual:        0.90,
			Color:         0.88,
			Spatial:       0.85,
			DistanceMiles: 1.2,
		},
		LostPet: domain.MatchPetDetail{
			PetID:    "lost-301",
			PetName:  "Cooper",
			Breed:    "Golden Retriever",
			ImageURL: "https://storage.petspotr.io/lost-cooper-face.jpg",
			Location: "Capitol Hill, Seattle, WA",
			Images: []domain.PetImage{
				{Object: "images/lost-pets/lost-301/face.jpg", URL: "https://storage.petspotr.io/lost-cooper-face.jpg", Tag: domain.PetImageTagPrimary},
				{Object: "images/lost-pets/lost-301/coat.jpg", URL: "https://storage.petspotr.io/lost-cooper-coat.jpg", Tag: domain.PetImageTagCoat},
			},
		},
		FoundPet: domain.MatchPetDetail{
			PetID:    "found-301",
			Breed:    "Golden Retriever",
			ImageURL: "https://storage.petspotr.io/found-cooper-face.jpg",
			Location: "Capitol Hill, Seattle, WA",
			Images: []domain.PetImage{
				{Object: "images/found-pets/found-301/face.jpg", URL: "https://storage.petspotr.io/found-cooper-face.jpg", Tag: domain.PetImageTagPrimary},
				{Object: "images/found-pets/found-301/collar.jpg", URL: "https://storage.petspotr.io/found-cooper-collar.jpg", Tag: domain.PetImageTagCollar},
			},
		},
	}

	data, err := json.Marshal(matchRecord)
	if err != nil {
		t.Fatalf("marshal match record: %v", err)
	}
	if err := memory.SaveState(context.Background(), store.MatchesCollection, matchRecord.MatchID, data); err != nil {
		t.Fatalf("seed match state: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}

	var matches []domain.MatchRecord
	if err := json.NewDecoder(rec.Body).Decode(&matches); err != nil {
		t.Fatalf("decode matches response: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	m := matches[0]
	if m.MatchID != "match-vector-101" {
		t.Errorf("expected match ID 'match-vector-101', got %q", m.MatchID)
	}
	if m.Scores.Vector != 0.96 {
		t.Errorf("expected Scores.Vector 0.96, got %f", m.Scores.Vector)
	}
	if m.Scores.Trait != 0.92 {
		t.Errorf("expected Scores.Trait 0.92, got %f", m.Scores.Trait)
	}
	if len(m.LostPet.Images) != 2 {
		t.Fatalf("expected 2 lost pet images, got %d", len(m.LostPet.Images))
	}
	if m.LostPet.Images[0].Tag != domain.PetImageTagPrimary {
		t.Errorf("expected lost pet image 0 tag 'primary', got %q", m.LostPet.Images[0].Tag)
	}
	if len(m.FoundPet.Images) != 2 {
		t.Fatalf("expected 2 found pet images, got %d", len(m.FoundPet.Images))
	}
	if m.FoundPet.Images[1].Tag != domain.PetImageTagCollar {
		t.Errorf("expected found pet image 1 tag 'collar', got %q", m.FoundPet.Images[1].Tag)
	}
}

func TestFrontendAccessibility_MultiPhotoAndCarousel(t *testing.T) {
	t.Parallel()

	// 1. styles.css focus-visible for thumbnail-btn
	cssContent, err := embeddedFiles.ReadFile("static/css/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssContent)
	expectedCSS := []string{
		".thumbnail-btn:focus-visible",
		"outline: 2px solid var(--accent-primary)",
		"outline-offset: 2px",
		".dropzone[aria-disabled=\"true\"]",
	}
	for _, snip := range expectedCSS {
		if !strings.Contains(css, snip) {
			t.Errorf("styles.css missing expected accessibility snippet: %q", snip)
		}
	}

	// 2. match-dashboard.js accessible selection state
	dashContent, err := embeddedFiles.ReadFile("static/js/match-dashboard.js")
	if err != nil {
		t.Fatal(err)
	}
	dashJS := string(dashContent)
	expectedDashSnippets := []string{
		"aria-current",
		"btn.setAttribute('aria-current', idx === i ? 'true' : 'false')",
		"btn.setAttribute('aria-label', 'View photo ' + (i + 1) + (img.tag ? ' (' + img.tag + ')' : ''))",
	}
	for _, snip := range expectedDashSnippets {
		if !strings.Contains(dashJS, snip) {
			t.Errorf("match-dashboard.js missing expected accessible selection snippet: %q", snip)
		}
	}

	// 3. lost-wizard.js dropzone aria-disabled and file ignore
	lostContent, err := embeddedFiles.ReadFile("static/js/lost-wizard.js")
	if err != nil {
		t.Fatal(err)
	}
	lostJS := string(lostContent)
	expectedLostSnippets := []string{
		"updatePhotoCount",
		"dropzone.setAttribute('aria-disabled', 'true')",
		"dropzone.setAttribute('aria-disabled', 'false')",
		"stagedImages.length >= 3",
	}
	for _, snip := range expectedLostSnippets {
		if !strings.Contains(lostJS, snip) {
			t.Errorf("lost-wizard.js missing expected snippet: %q", snip)
		}
	}

	// 4. found-report.js dropzone aria-disabled and file ignore
	foundContent, err := embeddedFiles.ReadFile("static/js/found-report.js")
	if err != nil {
		t.Fatal(err)
	}
	foundJS := string(foundContent)
	expectedFoundSnippets := []string{
		"updatePhotoCount",
		"dropzone.setAttribute('aria-disabled', 'true')",
		"dropzone.setAttribute('aria-disabled', 'false')",
		"stagedImages.length >= 3",
	}
	for _, snip := range expectedFoundSnippets {
		if !strings.Contains(foundJS, snip) {
			t.Errorf("found-report.js missing expected snippet: %q", snip)
		}
	}
}

func TestManifestAndOfflinePage(t *testing.T) {
	t.Parallel()
	srv := NewServer()

	t.Run("GET /manifest.webmanifest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/manifest+json") {
			t.Errorf("Content-Type = %q, want application/manifest+json", contentType)
		}
		cacheControl := rec.Header().Get("Cache-Control")
		if cacheControl != "public, max-age=86400" {
			t.Errorf("Cache-Control = %q, want public, max-age=86400", cacheControl)
		}
		var manifest map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
			t.Fatalf("invalid json manifest: %v", err)
		}
		if manifest["name"] != "PetSpotR — Lost & Found Pet Recovery" {
			t.Errorf("manifest name = %v, want PetSpotR — Lost & Found Pet Recovery", manifest["name"])
		}
	})

	t.Run("HEAD /manifest.webmanifest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/manifest.webmanifest", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("POST /manifest.webmanifest returns 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/manifest.webmanifest", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("GET /offline.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/offline.html", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", contentType)
		}
		cacheControl := rec.Header().Get("Cache-Control")
		if cacheControl != "public, max-age=86400" {
			t.Errorf("Cache-Control = %q, want public, max-age=86400", cacheControl)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "You're Offline") {
			t.Errorf("body does not contain 'You're Offline': %s", body)
		}
	})

	t.Run("HEAD /offline.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/offline.html", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("POST /offline.html returns 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/offline.html", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})
}

func TestOfflineFormInterceptionSnippets(t *testing.T) {
	t.Parallel()

	// 1. lost-wizard.js offline interception
	lostWizardContent, err := embeddedFiles.ReadFile("static/js/lost-wizard.js")
	if err != nil {
		t.Fatal(err)
	}
	lostWizardJS := string(lostWizardContent)
	expectedLostWizardSnippets := []string{
		"!navigator.onLine",
		"window.PetSpotROutbox",
		"enqueueReport({ type: 'lost'",
		"showToast('Report saved offline. It will submit automatically when you reconnect.')",
	}
	for _, snip := range expectedLostWizardSnippets {
		if !strings.Contains(lostWizardJS, snip) {
			t.Errorf("lost-wizard.js missing expected snippet: %q", snip)
		}
	}

	// 2. lost-report.js offline interception
	lostReportContent, err := embeddedFiles.ReadFile("static/js/lost-report.js")
	if err != nil {
		t.Fatal(err)
	}
	lostReportJS := string(lostReportContent)
	expectedLostReportSnippets := []string{
		"!navigator.onLine",
		"window.PetSpotROutbox",
		"enqueueReport",
		"Report saved offline",
	}
	for _, snip := range expectedLostReportSnippets {
		if !strings.Contains(lostReportJS, snip) {
			t.Errorf("lost-report.js missing expected snippet: %q", snip)
		}
	}

	// 3. found-report.js offline interception
	foundReportContent, err := embeddedFiles.ReadFile("static/js/found-report.js")
	if err != nil {
		t.Fatal(err)
	}
	foundReportJS := string(foundReportContent)
	expectedFoundSnippets := []string{
		"!navigator.onLine",
		"window.PetSpotROutbox",
		"enqueueReport({ type: 'found'",
		"showToast('Report saved offline. It will submit automatically when you reconnect.')",
	}
	for _, snip := range expectedFoundSnippets {
		if !strings.Contains(foundReportJS, snip) {
			t.Errorf("found-report.js missing expected snippet: %q", snip)
		}
	}
}

func TestMatchesReunionRoomUI(t *testing.T) {
	srv := NewDemoServer()

	t.Run("/matches renders reunion room presence, typing, attachment, and resolution markup", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/matches", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		expectedSnippets := []string{
			`id="match-thread-presence"`,
			`class="presence-badge"`,
			`class="presence-dot`,
			`class="presence-text"`,
			`id="match-thread-typing"`,
			`role="status" aria-live="polite"`,
			`id="match-thread-resolved-banner"`,
			`class="reunion-banner"`,
			`role="alert"`,
			`id="btn-chat-resolve-reunion"`,
			`id="match-thread-attach-btn"`,
			`id="match-thread-file-input"`,
			`accept="image/jpeg,image/png,image/webp"`,
			`multiple`,
			`id="match-thread-staged-tray"`,
			`class="staged-tray"`,
			`class="staged-thumbs"`,
			`compose-row`,
			`btn-attach`,
		}

		for _, snippet := range expectedSnippets {
			if !strings.Contains(body, snippet) {
				t.Errorf("/matches missing expected reunion room snippet: %q", snippet)
			}
		}
	})

	t.Run("styles.css declares reunion room responsive styles and reduced motion overrides", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/css/styles.css", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		expectedSnippets := []string{
			`.presence-badge`,
			`.presence-dot`,
			`.status-online`,
			`.status-typing`,
			`.status-offline`,
			`.typing-indicator`,
			`@keyframes typingBounce`,
			`.reunion-banner`,
			`.compose-row`,
			`.btn-attach`,
			`.staged-tray`,
			`.staged-thumbs`,
		}

		for _, snippet := range expectedSnippets {
			if !strings.Contains(body, snippet) {
				t.Errorf("styles.css missing expected reunion room snippet: %q", snippet)
			}
		}
	})
}

