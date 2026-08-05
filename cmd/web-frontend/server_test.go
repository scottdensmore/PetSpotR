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
}
