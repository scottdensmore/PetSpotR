package telemetry

import (
	"io"
	"log/slog"
	"os"
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
