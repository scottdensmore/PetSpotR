package telemetry

import (
	"context"
	"fmt"
	"time"
)

type spanContextKey struct{}
type traceContextKey struct{}

// ContextWithTrace returns a copy of parent context with the provided trace ID and span ID.
func ContextWithTrace(ctx context.Context, traceID string, spanID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID != "" {
		ctx = context.WithValue(ctx, traceContextKey{}, traceID)
	}
	if spanID != "" {
		ctx = context.WithValue(ctx, spanContextKey{}, spanID)
	}
	return ctx
}

// GetTraceID extracts the trace ID from a context if present.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val := ctx.Value(traceContextKey{}); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

// StartSpan creates a new trace context span and attaches a generated span ID.
func StartSpan(ctx context.Context, name string) (context.Context, string) {
	spanID := fmt.Sprintf("span-%s-%d", name, time.Now().UnixNano())
	return context.WithValue(ctx, spanContextKey{}, spanID), spanID
}

// GetSpanID extracts the trace span ID from a context if present.
func GetSpanID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val := ctx.Value(spanContextKey{}); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}
