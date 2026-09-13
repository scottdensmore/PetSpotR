package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// ReadinessChecker checks whether a service dependency is ready.
type ReadinessChecker func(ctx context.Context) error

// HealthHandler manages liveness and readiness HTTP handlers.
type HealthHandler struct {
	checkers map[string]ReadinessChecker
}

// NewHealthHandler initializes a HealthHandler with the provided readiness checkers.
func NewHealthHandler(checkers map[string]ReadinessChecker) *HealthHandler {
	copied := make(map[string]ReadinessChecker)
	for k, v := range checkers {
		copied[k] = v
	}
	return &HealthHandler{checkers: copied}
}

type livenessResponse struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string   `json:"status"`
	Errors []string `json:"errors,omitempty"`
}

// HandleLiveness responds with HTTP 200 {"status":"ok"}.
func (h *HealthHandler) HandleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(livenessResponse{Status: "ok"})
}

// HandleReadiness runs all registered ReadinessChecker functions.
// If all pass: returns HTTP 200 {"status":"ready"}.
// If any fails: returns HTTP 503 {"status":"unavailable","errors":[...]}.
func (h *HealthHandler) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h == nil || len(h.checkers) == 0 {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(readinessResponse{Status: "ready"})
		return
	}

	keys := make([]string, 0, len(h.checkers))
	for k := range h.checkers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var errMsgs []string
	for _, name := range keys {
		checker := h.checkers[name]
		if checker != nil {
			if err := checker(r.Context()); err != nil {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}

	if len(errMsgs) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(readinessResponse{
			Status: "unavailable",
			Errors: errMsgs,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(readinessResponse{Status: "ready"})
}

// RegisterHealthRoutes registers standardized /healthz and /readyz endpoints on the given ServeMux.
func RegisterHealthRoutes(mux *http.ServeMux, checkers map[string]ReadinessChecker) {
	handler := NewHealthHandler(checkers)
	mux.HandleFunc("/healthz", handler.HandleLiveness)
	mux.HandleFunc("/readyz", handler.HandleReadiness)
}
