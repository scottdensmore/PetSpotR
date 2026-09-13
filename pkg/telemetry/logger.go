package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// NewJSONLogger initializes a structured log/slog JSON logger for a named microservice.
func NewJSONLogger(serviceName string) *slog.Logger {
	return NewJSONLoggerWithWriter(serviceName, os.Stdout)
}

// NewJSONLoggerWithWriter constructs a JSON logger writing to the provided io.Writer.
func NewJSONLoggerWithWriter(serviceName string, w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler).With("service", serviceName)
}

// WithTraceContext extracts trace_id and span_id from ctx and attaches them to log entries.
func WithTraceContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	if ctx == nil {
		return logger
	}
	var attrs []any
	if traceID := GetTraceID(ctx); traceID != "" {
		attrs = append(attrs, "trace_id", traceID)
	}
	if spanID := GetSpanID(ctx); spanID != "" {
		attrs = append(attrs, "span_id", spanID)
	}
	if len(attrs) > 0 {
		return logger.With(attrs...)
	}
	return logger
}

// ExtractTraceparent parses a W3C traceparent header (00-<32-hex-trace-id>-<16-hex-span-id>-<flags>).
func ExtractTraceparent(header string) (traceID string, spanID string, ok bool) {
	header = strings.TrimSpace(header)
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return "", "", false
	}
	version := parts[0]
	traceID = parts[1]
	spanID = parts[2]
	flags := parts[3]

	if version != "00" {
		return "", "", false
	}
	if len(traceID) != 32 || len(spanID) != 16 || len(flags) != 2 {
		return "", "", false
	}
	if !isHex(traceID) || !isHex(spanID) || !isHex(flags) {
		return "", "", false
	}
	if traceID == "00000000000000000000000000000000" || spanID == "0000000000000000" {
		return "", "", false
	}
	return strings.ToLower(traceID), strings.ToLower(spanID), true
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func generateTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil || isAllZero(b[:]) {
		b[0] = 1
	}
	return hex.EncodeToString(b[:])
}

func generateSpanID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil || isAllZero(b[:]) {
		b[0] = 1
	}
	return hex.EncodeToString(b[:])
}

func isAllZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// TraceContextMiddleware reads incoming W3C traceparent (or generates trace/span IDs if absent),
// injects them into the request context, and writes outgoing traceparent header.
func TraceContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceparent := r.Header.Get("traceparent")
		traceID, spanID, ok := ExtractTraceparent(traceparent)
		if !ok {
			traceID = generateTraceID()
			spanID = generateSpanID()
		}
		ctx := ContextWithTrace(r.Context(), traceID, spanID)
		w.Header().Set("traceparent", fmt.Sprintf("00-%s-%s-01", traceID, spanID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
