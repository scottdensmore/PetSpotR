package telemetry_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/telemetry"
)

func TestHandleLiveness(t *testing.T) {
	handler := telemetry.NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.HandleLiveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestHandleReadinessSuccess(t *testing.T) {
	checkers := map[string]telemetry.ReadinessChecker{
		"database": func(ctx context.Context) error {
			return nil
		},
		"cache": func(ctx context.Context) error {
			return nil
		},
	}
	handler := telemetry.NewHealthHandler(checkers)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.HandleReadiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("expected status ready, got %v", body["status"])
	}
	if _, ok := body["errors"]; ok {
		t.Errorf("expected no errors in response, got %v", body["errors"])
	}
}

func TestHandleReadinessEmpty(t *testing.T) {
	handler := telemetry.NewHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.HandleReadiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("expected status ready, got %v", body["status"])
	}
}

func TestHandleReadinessFailure(t *testing.T) {
	checkers := map[string]telemetry.ReadinessChecker{
		"database": func(ctx context.Context) error {
			return errors.New("connection refused")
		},
		"cache": func(ctx context.Context) error {
			return nil
		},
		"storage": func(ctx context.Context) error {
			return errors.New("bucket unreachable")
		},
	}
	handler := telemetry.NewHealthHandler(checkers)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.HandleReadiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 Service Unavailable, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body struct {
		Status string   `json:"status"`
		Errors []string `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if body.Status != "unavailable" {
		t.Errorf("expected status unavailable, got %q", body.Status)
	}

	expectedErrors := []string{
		"database: connection refused",
		"storage: bucket unreachable",
	}
	if !reflect.DeepEqual(body.Errors, expectedErrors) {
		t.Errorf("expected errors %v, got %v", expectedErrors, body.Errors)
	}
}

func TestRegisterHealthRoutes(t *testing.T) {
	mux := http.NewServeMux()
	checkers := map[string]telemetry.ReadinessChecker{
		"db": func(ctx context.Context) error {
			return nil
		},
	}
	telemetry.RegisterHealthRoutes(mux, checkers)

	// Liveness
	reqLive := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recLive := httptest.NewRecorder()
	mux.ServeHTTP(recLive, reqLive)
	if recLive.Code != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", recLive.Code)
	}

	// Readiness
	reqReady := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recReady := httptest.NewRecorder()
	mux.ServeHTTP(recReady, reqReady)
	if recReady.Code != http.StatusOK {
		t.Fatalf("expected /readyz 200, got %d", recReady.Code)
	}
}
