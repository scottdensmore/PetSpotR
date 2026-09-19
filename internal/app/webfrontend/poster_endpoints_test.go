package webfrontend_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestPosterEndpoints(t *testing.T) {
	t.Parallel()
	st := store.NewMemoryStore()
	pet := domain.LostPetRecord{
		PetID:        "lost-pet-404",
		PetName:      "Rusty",
		Species:      "Dog",
		Breed:        "Golden Retriever",
		PrimaryColor: "Golden",
		Location:     "Capitol Hill, Seattle, WA",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.625,
			Longitude: -122.320,
		},
		ReportedAt:  time.Now().UTC().Add(-24 * time.Hour),
		ImageObject: "images/lost/rusty.jpg",
		Status:      domain.LostPetStatusLost,
	}
	data, err := json.Marshal(pet)
	if err != nil {
		t.Fatalf("failed to marshal pet: %v", err)
	}
	_ = st.SaveState(context.Background(), store.LostPetsCollection, pet.PetID, data)

	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	t.Run("GET /api/v1/pets/:id/qr.svg returns 200 and image/svg+xml", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-pet-404/qr.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Fatalf("expected Content-Type image/svg+xml, got %q", ct)
		}
		if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=86400, immutable" {
			t.Fatalf("expected Cache-Control public, max-age=86400, immutable, got %q", cc)
		}
		body := w.Body.String()
		if !strings.Contains(body, "<svg") || !strings.Contains(body, "</svg>") {
			t.Fatal("expected SVG body")
		}
		if !strings.Contains(body, "<path") {
			t.Fatal("expected SVG path in qr code")
		}
	})

	t.Run("GET /api/v1/pets/:id/qr.svg supports custom size query param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-pet-404/qr.svg?size=64", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, `width="64"`) || !strings.Contains(body, `height="64"`) {
			t.Fatalf("expected width and height 64, got body:\n%s", body)
		}
	})

	t.Run("GET /api/v1/pets/:id/share-card.svg returns 200 and 1200x630 card", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-pet-404/share-card.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Fatalf("expected Content-Type image/svg+xml, got %q", ct)
		}
		if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
			t.Fatalf("expected Cache-Control public, max-age=3600, got %q", cc)
		}
		body := w.Body.String()
		if !strings.Contains(body, `viewBox="0 0 1200 630"`) {
			t.Fatal("expected 1200x630 viewBox in social card")
		}
		if !strings.Contains(body, "Rusty") {
			t.Fatal("expected pet name Rusty in social card")
		}
		if !strings.Contains(body, "Golden Retriever") {
			t.Fatal("expected breed Golden Retriever in social card")
		}
		if !strings.Contains(body, "Capitol Hill, Seattle, WA") {
			t.Fatal("expected location Capitol Hill, Seattle, WA in social card")
		}
		if !strings.Contains(body, "REWARD") {
			t.Fatal("expected reward badge in social card")
		}
	})

	t.Run("GET /api/v1/pets/:id/share-card.svg supports custom reward query param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-pet-404/share-card.svg?reward=$1,000", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "$1,000") {
			t.Fatalf("expected custom reward $1,000 in social card, got:\n%s", body)
		}
	})

	t.Run("GET /api/v1/pets/:id/share-card.svg properly XML escapes pet data", func(t *testing.T) {
		xPet := domain.LostPetRecord{
			PetID:        "lost-special-char",
			PetName:      "Jack & Jill <Special>",
			Species:      "Dog",
			Breed:        "Shepherd & Husky",
			PrimaryColor: "Black & Tan",
			Location:     "5th & Main <Downtown>",
			Status:       domain.LostPetStatusLost,
		}
		xData, _ := json.Marshal(xPet)
		_ = st.SaveState(context.Background(), store.LostPetsCollection, xPet.PetID, xData)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/lost-special-char/share-card.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		body := w.Body.String()
		if strings.Contains(body, "<Special>") {
			t.Fatal("expected '<Special>' to be escaped, found unescaped tag in SVG")
		}
		if !strings.Contains(body, "Jack &amp; Jill &lt;Special&gt;") {
			t.Fatalf("expected escaped pet name in social card, got:\n%s", body)
		}
		if !strings.Contains(body, "Shepherd &amp; Husky") {
			t.Fatalf("expected escaped breed in social card, got:\n%s", body)
		}
	})

	t.Run("Returns 404 for nonexistent pet", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets/unknown-pet/qr.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 NotFound, got %d", w.Code)
		}

		reqCard := httptest.NewRequest(http.MethodGet, "/api/v1/pets/unknown-pet/share-card.svg", nil)
		wCard := httptest.NewRecorder()
		srv.ServeHTTP(wCard, reqCard)

		if wCard.Code != http.StatusNotFound {
			t.Fatalf("expected 404 NotFound for share-card, got %d", wCard.Code)
		}
	})

	t.Run("Returns 405 for non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/pets/lost-pet-404/qr.svg", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 MethodNotAllowed, got %d", w.Code)
		}

		reqCard := httptest.NewRequest(http.MethodPost, "/api/v1/pets/lost-pet-404/share-card.svg", nil)
		wCard := httptest.NewRecorder()
		srv.ServeHTTP(wCard, reqCard)

		if wCard.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 MethodNotAllowed for share-card, got %d", wCard.Code)
		}
	})
}
