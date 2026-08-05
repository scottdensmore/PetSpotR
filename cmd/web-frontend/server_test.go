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
}
