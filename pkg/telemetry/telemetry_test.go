package telemetry_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/telemetry"
)

func TestStructuredJSONLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := telemetry.NewJSONLoggerWithWriter("test-service", &buf)

	logger.Info("processed pet event", "petId", "lost-101", "score", 0.95)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"service":"test-service"`) {
		t.Errorf("expected service tag in log output, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"petId":"lost-101"`) {
		t.Errorf("expected petId tag in log output, got %s", logOutput)
	}
}

func TestMetricsRegistry(t *testing.T) {
	registry := telemetry.NewMetricsRegistry("web-frontend")

	t.Run("records request metrics and renders prometheus metrics output", func(t *testing.T) {
		registry.RecordRequest(http.MethodGet, "/matches", http.StatusOK, 45*time.Millisecond)
		registry.RecordRequest(http.MethodPost, "/api/v1/lost-pets", http.StatusCreated, 120*time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		handler := registry.MetricsHandler()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK for /metrics, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "http_requests_total") {
			t.Errorf("expected body to contain http_requests_total metric, got %s", body)
		}
		if !strings.Contains(body, `service="web-frontend"`) {
			t.Errorf("expected body to contain service label, got %s", body)
		}
	})
}

func TestTraceContextPropagation(t *testing.T) {
	ctx := context.Background()
	ctxWithTrace, spanID := telemetry.StartSpan(ctx, "ProcessFoundPet")

	if spanID == "" {
		t.Errorf("expected non-empty span ID")
	}

	retrievedSpan := telemetry.GetSpanID(ctxWithTrace)
	if retrievedSpan != spanID {
		t.Errorf("got span ID %s, want %s", retrievedSpan, spanID)
	}
}
