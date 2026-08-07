package telemetry

import (
	"context"
	"fmt"
	"time"
)

type spanContextKey struct{}

// StartSpan creates a new trace context span and attaches a generated span ID.
func StartSpan(ctx context.Context, name string) (context.Context, string) {
	spanID := fmt.Sprintf("span-%s-%d", name, time.Now().UnixNano())
	return context.WithValue(ctx, spanContextKey{}, spanID), spanID
}

// GetSpanID extracts the trace span ID from a context if present.
func GetSpanID(ctx context.Context) string {
	if val := ctx.Value(spanContextKey{}); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}
