package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServer_Routes(t *testing.T) {
	srv := NewServer()

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
}
