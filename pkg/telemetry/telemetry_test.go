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

func TestContextWithTraceAndGetTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"

	ctxWithTrace := telemetry.ContextWithTrace(ctx, traceID, spanID)

	if got := telemetry.GetTraceID(ctxWithTrace); got != traceID {
		t.Errorf("GetTraceID() = %q, want %q", got, traceID)
	}
	if got := telemetry.GetSpanID(ctxWithTrace); got != spanID {
		t.Errorf("GetSpanID() = %q, want %q", got, spanID)
	}
}

func TestExtractTraceparent(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		wantTraceID string
		wantSpanID  string
		wantOK      bool
	}{
		{
			name:        "valid W3C traceparent",
			header:      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantTraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
			wantSpanID:  "00f067aa0ba902b7",
			wantOK:      true,
		},
		{
			name:        "valid with whitespace",
			header:      "  00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00  ",
			wantTraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
			wantSpanID:  "00f067aa0ba902b7",
			wantOK:      true,
		},
		{
			name:        "empty header",
			header:      "",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "unsupported version",
			header:      "01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "version ff invalid",
			header:      "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "all zeros trace id",
			header:      "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "all zeros span id",
			header:      "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "trace ID too short",
			header:      "00-4bf92f3577b34da6a3ce929d0e0e47-00f067aa0ba902b7-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "span ID too short",
			header:      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "non-hex characters",
			header:      "00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
		{
			name:        "invalid parts count",
			header:      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
			wantTraceID: "",
			wantSpanID:  "",
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTraceID, gotSpanID, gotOK := telemetry.ExtractTraceparent(tt.header)
			if gotOK != tt.wantOK {
				t.Fatalf("ExtractTraceparent() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotTraceID != tt.wantTraceID {
				t.Errorf("ExtractTraceparent() traceID = %q, want %q", gotTraceID, tt.wantTraceID)
			}
			if gotSpanID != tt.wantSpanID {
				t.Errorf("ExtractTraceparent() spanID = %q, want %q", gotSpanID, tt.wantSpanID)
			}
		})
	}
}

func TestTraceContextMiddleware(t *testing.T) {
	t.Run("propagates incoming traceparent", func(t *testing.T) {
		incomingHeader := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

		var recordedTraceID, recordedSpanID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recordedTraceID = telemetry.GetTraceID(r.Context())
			recordedSpanID = telemetry.GetSpanID(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := telemetry.TraceContextMiddleware(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("traceparent", incomingHeader)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if recordedTraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Errorf("recorded trace ID = %q, want %q", recordedTraceID, "4bf92f3577b34da6a3ce929d0e0e4736")
		}
		if recordedSpanID != "00f067aa0ba902b7" {
			t.Errorf("recorded span ID = %q, want %q", recordedSpanID, "00f067aa0ba902b7")
		}

		outgoing := rec.Header().Get("traceparent")
		if !strings.HasPrefix(outgoing, "00-4bf92f3577b34da6a3ce929d0e0e4736-") {
			t.Errorf("outgoing traceparent header = %q, expected trace ID match", outgoing)
		}
	})

	t.Run("generates trace and span IDs if absent", func(t *testing.T) {
		var recordedTraceID, recordedSpanID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recordedTraceID = telemetry.GetTraceID(r.Context())
			recordedSpanID = telemetry.GetSpanID(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := telemetry.TraceContextMiddleware(next)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if len(recordedTraceID) != 32 {
			t.Errorf("expected 32-char hex trace ID, got %q", recordedTraceID)
		}
		if len(recordedSpanID) != 16 {
			t.Errorf("expected 16-char hex span ID, got %q", recordedSpanID)
		}

		outgoing := rec.Header().Get("traceparent")
		expectedPrefix := "00-" + recordedTraceID + "-" + recordedSpanID
		if !strings.HasPrefix(outgoing, expectedPrefix) {
			t.Errorf("outgoing traceparent = %q, want prefix %q", outgoing, expectedPrefix)
		}
	})
}

func TestWithTraceContext(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := telemetry.NewJSONLoggerWithWriter("test-service", &buf)

	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	ctx := telemetry.ContextWithTrace(context.Background(), traceID, spanID)

	logger := telemetry.WithTraceContext(ctx, baseLogger)
	logger.Info("event with trace")

	output := buf.String()
	if !strings.Contains(output, `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`) {
		t.Errorf("expected trace_id in log output, got %s", output)
	}
	if !strings.Contains(output, `"span_id":"00f067aa0ba902b7"`) {
		t.Errorf("expected span_id in log output, got %s", output)
	}
}
