package telemetry

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RequestKey uniquely identifies an HTTP request pattern.
type RequestKey struct {
	Method string
	Path   string
	Status int
}

// MetricsRegistry records HTTP request metrics and renders Prometheus format.
type MetricsRegistry struct {
	mu          sync.Mutex
	serviceName string
	requestMap  map[RequestKey]int
	durationSum map[RequestKey]time.Duration
}

// NewMetricsRegistry constructs a MetricsRegistry.
func NewMetricsRegistry(serviceName string) *MetricsRegistry {
	return &MetricsRegistry{
		serviceName: serviceName,
		requestMap:  make(map[RequestKey]int),
		durationSum: make(map[RequestKey]time.Duration),
	}
}

// RecordRequest records status code and duration for an HTTP invocation.
func (m *MetricsRegistry) RecordRequest(method, path string, status int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := RequestKey{Method: method, Path: path, Status: status}
	m.requestMap[key]++
	m.durationSum[key] += duration
}

// MetricsHandler returns an http.Handler that renders Prometheus text format metrics.
func (m *MetricsRegistry) MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		var sb strings.Builder
		sb.WriteString("# HELP http_requests_total Total number of HTTP requests processed.\n")
		sb.WriteString("# TYPE http_requests_total counter\n")

		for key, count := range m.requestMap {
			fmt.Fprintf(&sb,
				"http_requests_total{service=\"%s\",method=\"%s\",path=\"%s\",status=\"%d\"} %d\n",
				m.serviceName, key.Method, key.Path, key.Status, count,
			)
		}

		sb.WriteString("\n# HELP http_request_duration_seconds_sum Total request latency in seconds.\n")
		sb.WriteString("# TYPE http_request_duration_seconds_sum counter\n")

		for key, dur := range m.durationSum {
			fmt.Fprintf(&sb,
				"http_request_duration_seconds_sum{service=\"%s\",method=\"%s\",path=\"%s\",status=\"%d\"} %.6f\n",
				m.serviceName, key.Method, key.Path, key.Status, dur.Seconds(),
			)
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	})
}
