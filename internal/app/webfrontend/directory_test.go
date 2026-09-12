package webfrontend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestPublicPetDirectory_HTML(t *testing.T) {
	srv := NewDemoServer()

	req := httptest.NewRequest(http.MethodGet, "/pets", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}

	body := rec.Body.String()
	// Must contain layout & title
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE html in response")
	}
	if !strings.Contains(body, "Public Pet Directory") {
		t.Error("expected Public Pet Directory heading in HTML")
	}
	// Active navigation
	if !strings.Contains(body, `id="nav-directory"`) {
		t.Error("expected nav-directory link in navbar")
	}
	// Must contain active reports
	if !strings.Contains(body, "Buddy") {
		t.Error("expected active lost pet Buddy in directory HTML")
	}
	if !strings.Contains(body, "Golden Retriever") {
		t.Error("expected active found pet breed in directory HTML")
	}

	// Must NOT contain inactive reports (reunited or resolved)
	if strings.Contains(body, "lost-999-reunited") || strings.Contains(body, "Max") {
		t.Error("inactive reunited report leaked into directory HTML")
	}
	if strings.Contains(body, "found-999-resolved") {
		t.Error("inactive resolved report leaked into directory HTML")
	}

	// Must NOT leak private fields
	for _, secret := range []string{
		"owner@example.com",
		"finder@example.com",
		"luna-owner@example.com",
		"catfinder@example.com",
		"555-0101",
		"555-0105",
	} {
		if strings.Contains(body, secret) {
			t.Errorf("private information %q leaked into directory HTML!", secret)
		}
	}
}

func TestPublicPetDirectory_JSONAPI(t *testing.T) {
	srv := NewDemoServer()

	for _, endpoint := range []struct {
		name   string
		path   string
		accept string
	}{
		{name: "api route", path: "/api/v1/pets", accept: ""},
		{name: "pets route with accept json", path: "/pets", accept: "application/json"},
		{name: "pets route with format param", path: "/pets?format=json", accept: ""},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint.path, nil)
			if endpoint.accept != "" {
				req.Header.Set("Accept", endpoint.accept)
			}
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}

			if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
				t.Errorf("expected application/json, got %s", rec.Header().Get("Content-Type"))
			}

			if total := rec.Header().Get("X-Total-Count"); total == "" || total == "0" {
				t.Errorf("expected X-Total-Count header, got %q", total)
			}

			var items []PublicPetDirectoryItem
			if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}

			if len(items) == 0 {
				t.Fatal("expected at least one active pet report")
			}

			// Verify private fields are absent from JSON serialized output
			rawJSON := rec.Body.String()
			for _, forbidden := range []string{
				"owner@example.com",
				"finder@example.com",
				"luna-owner@example.com",
				"catfinder@example.com",
				"555-0101",
				"555-0105",
				"reporterEmail",
				"finderEmail",
				"phone",
				"ownedBy",
			} {
				if strings.Contains(rawJSON, forbidden) {
					t.Errorf("private field %q present in public JSON DTO!", forbidden)
				}
			}
		})
	}
}

func TestPublicPetDirectory_Filters(t *testing.T) {
	srv := NewDemoServer()

	t.Run("Species filter dog", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?species=dog", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(items) == 0 {
			t.Fatal("expected dog items")
		}
		for _, it := range items {
			if !strings.EqualFold(it.Species, "dog") {
				t.Errorf("expected dog species, got %s (id: %s)", it.Species, it.PetID)
			}
		}
	})

	t.Run("Species filter cat", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?species=cat", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(items) == 0 {
			t.Fatal("expected cat items")
		}
		for _, it := range items {
			if !strings.EqualFold(it.Species, "cat") {
				t.Errorf("expected cat species, got %s (id: %s)", it.Species, it.PetID)
			}
		}
	})

	t.Run("Species filter other with no other animals returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?species=other", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("expected 0 other animals, got %d", len(items))
		}
	})

	t.Run("Status filter lost", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?status=lost", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(items) == 0 {
			t.Fatal("expected lost pet items")
		}
		for _, it := range items {
			if it.Status != "lost" || it.ReportType != "lost" {
				t.Errorf("expected status lost, got status=%s reportType=%s (id: %s)", it.Status, it.ReportType, it.PetID)
			}
		}
	})

	t.Run("Status filter found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?status=found", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(items) == 0 {
			t.Fatal("expected found pet items")
		}
		for _, it := range items {
			if it.Status != "found" || it.ReportType != "found" {
				t.Errorf("expected status found, got status=%s reportType=%s (id: %s)", it.Status, it.ReportType, it.PetID)
			}
		}
	})

	t.Run("Text query filter matches breed and description", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?q=Golden", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(items) == 0 {
			t.Fatal("expected Golden Retriever reports")
		}
		for _, it := range items {
			if !strings.Contains(it.Breed, "Golden") && !strings.Contains(it.PetName, "Golden") {
				t.Errorf("unexpected item in Golden query results: %+v", it)
			}
		}
	})

	t.Run("Text query with no results returns 200 with empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?q=UnicornNonexistent", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		var items []PublicPetDirectoryItem
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})
}

func TestPublicPetDirectory_DeterministicPagination(t *testing.T) {
	memStore := store.NewMemoryStore()
	srv := NewServerWithStore(memStore)
	ctx := context.Background()

	// Seed 9 items with distinct timestamps
	baseTime := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 9; i++ {
		record := domain.NormalizeLostPetRecord(domain.LostPetRecord{
			PetID:      fmt.Sprintf("lost-det-%02d", i),
			PetName:    fmt.Sprintf("Pet%02d", i),
			Species:    "Dog",
			Breed:      "Terrier",
			Location:   "Seattle, WA",
			ReportedAt: baseTime.Add(time.Duration(i) * time.Hour),
			Status:     domain.LostPetStatusLost,
		})
		data, _ := json.Marshal(record)
		if err := memStore.SaveState(ctx, store.LostPetsCollection, record.PetID, data); err != nil {
			t.Fatalf("failed to seed pet: %v", err)
		}
	}

	t.Run("Deterministic sort order across multiple invocations", func(t *testing.T) {
		var firstRunIDs []string
		for run := 0; run < 5; run++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/pets?limit=10", nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d", rec.Code)
			}

			var items []PublicPetDirectoryItem
			if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			var ids []string
			for _, item := range items {
				ids = append(ids, item.PetID)
			}

			if run == 0 {
				firstRunIDs = ids
				// Verify strictly descending by ReportedAt
				for k := 1; k < len(items); k++ {
					if items[k].ReportedAt.After(items[k-1].ReportedAt) {
						t.Errorf("items not in descending ReportedAt order at index %d", k)
					}
				}
			} else {
				if len(ids) != len(firstRunIDs) {
					t.Fatalf("run %d length %d != first run length %d", run, len(ids), len(firstRunIDs))
				}
				for k := range ids {
					if ids[k] != firstRunIDs[k] {
						t.Fatalf("run %d index %d: id %s != %s (non-deterministic sort)", run, k, ids[k], firstRunIDs[k])
					}
				}
			}
		}
	})

	t.Run("Offset pagination guarantees no duplicates or omissions across adjacent pages", func(t *testing.T) {
		// Page 1: offset 0, limit 3
		req1 := httptest.NewRequest(http.MethodGet, "/api/v1/pets?limit=3&offset=0", nil)
		rec1 := httptest.NewRecorder()
		srv.ServeHTTP(rec1, req1)
		var page1 []PublicPetDirectoryItem
		_ = json.Unmarshal(rec1.Body.Bytes(), &page1)

		// Page 2: offset 3, limit 3
		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/pets?limit=3&offset=3", nil)
		rec2 := httptest.NewRecorder()
		srv.ServeHTTP(rec2, req2)
		var page2 []PublicPetDirectoryItem
		_ = json.Unmarshal(rec2.Body.Bytes(), &page2)

		// Page 3: offset 6, limit 3
		req3 := httptest.NewRequest(http.MethodGet, "/api/v1/pets?limit=3&offset=6", nil)
		rec3 := httptest.NewRecorder()
		srv.ServeHTTP(rec3, req3)
		var page3 []PublicPetDirectoryItem
		_ = json.Unmarshal(rec3.Body.Bytes(), &page3)

		if len(page1) != 3 || len(page2) != 3 || len(page3) != 3 {
			t.Fatalf("expected 3 items per page, got %d, %d, %d", len(page1), len(page2), len(page3))
		}

		seen := make(map[string]bool)
		for _, page := range [][]PublicPetDirectoryItem{page1, page2, page3} {
			for _, item := range page {
				if seen[item.PetID] {
					t.Fatalf("duplicate item across pages: %s", item.PetID)
				}
				seen[item.PetID] = true
			}
		}

		if len(seen) != 9 {
			t.Fatalf("expected 9 distinct items across 3 pages, got %d", len(seen))
		}
	})

	t.Run("Cursor pagination produces stable traversal", func(t *testing.T) {
		// First page
		req1 := httptest.NewRequest(http.MethodGet, "/api/v1/pets?limit=4", nil)
		rec1 := httptest.NewRecorder()
		srv.ServeHTTP(rec1, req1)

		var page1 []PublicPetDirectoryItem
		_ = json.Unmarshal(rec1.Body.Bytes(), &page1)
		if len(page1) != 4 {
			t.Fatalf("expected 4 items on page 1, got %d", len(page1))
		}

		nextCursor := rec1.Header().Get("X-Next-Cursor")
		if nextCursor == "" {
			t.Fatal("expected non-empty X-Next-Cursor header")
		}

		// Second page via cursor
		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/pets?limit=4&cursor="+nextCursor, nil)
		rec2 := httptest.NewRecorder()
		srv.ServeHTTP(rec2, req2)

		var page2 []PublicPetDirectoryItem
		_ = json.Unmarshal(rec2.Body.Bytes(), &page2)
		if len(page2) != 4 {
			t.Fatalf("expected 4 items on page 2, got %d", len(page2))
		}

		// Ensure zero overlap between cursor pages
		p1Map := make(map[string]bool)
		for _, item := range page1 {
			p1Map[item.PetID] = true
		}
		for _, item := range page2 {
			if p1Map[item.PetID] {
				t.Fatalf("item %s duplicated in cursor page 2", item.PetID)
			}
		}
	})
}

func TestPublicPetDirectory_QueryValidation(t *testing.T) {
	srv := NewDemoServer()

	invalidURLs := []string{
		"/api/v1/pets?limit=-1",
		"/api/v1/pets?limit=xyz",
		"/api/v1/pets?offset=-5",
		"/api/v1/pets?offset=xyz",
		"/api/v1/pets?status=banana",
		"/api/v1/pets?cursor=invalid_base64!!!",
		"/api/v1/pets?lat=47.6",                           // missing lng
		"/api/v1/pets?lng=-122.3",                         // missing lat
		"/api/v1/pets?lat=999&lng=-122.3",                 // out of bounds lat
		"/api/v1/pets?lat=47.6&lng=-999",                  // out of bounds lng
		"/api/v1/pets?lat=47.6&lng=-122.3&radiusMiles=-5", // negative radius
		"/pets?limit=-1",
		"/pets?status=banana",
	}

	for _, u := range invalidURLs {
		t.Run(u, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, u, nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("for URL %s: expected status 400 Bad Request, got %d (body: %s)", u, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestExistingPetAPIs_StrictValidationAndDeterminism(t *testing.T) {
	srv := NewDemoServer()

	t.Run("GET /api/v1/lost-pets rejects malformed limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets?limit=-5", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("GET /api/v1/found-pets rejects malformed offset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/found-pets?offset=xyz", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("GET /api/v1/lost-pets rejects incomplete geo parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets?lat=47.6", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("GET /api/v1/found-pets rejects invalid status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/found-pets?status=banana", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}
