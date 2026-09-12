package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestTokenConsumptionAndBurstCapacity(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	mockTime := now
	clock := func() time.Time { return mockTime }

	limiter := New(WithClock(clock), WithCleanupInterval(0))
	defer limiter.Close()

	limit := Limit{
		Name:     "test-limit",
		Requests: 5,
		Window:   time.Minute,
		Burst:    5,
	}

	clientKey := "client-1"

	// Consume 5 tokens (full burst capacity)
	for i := 1; i <= 5; i++ {
		allowed, retryAfter := limiter.Allow(clientKey, limit)
		if !allowed {
			t.Fatalf("request %d should be allowed within burst capacity", i)
		}
		if retryAfter != 0 {
			t.Fatalf("request %d retryAfter should be 0, got %v", i, retryAfter)
		}
	}

	// 6th request should be refused immediately
	allowed, retryAfter := limiter.Allow(clientKey, limit)
	if allowed {
		t.Fatalf("request 6 should be refused when tokens are depleted")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %v", retryAfter)
	}
}

func TestTokenReplenishmentOverTime(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	mockTime := now
	clock := func() time.Time { return mockTime }

	limiter := New(WithClock(clock), WithCleanupInterval(0))
	defer limiter.Close()

	limit := Limit{
		Name:     "replenish-limit",
		Requests: 5,
		Window:   time.Minute, // 1 token every 12 seconds
		Burst:    5,
	}

	clientKey := "client-replenish"

	// Deplete all 5 tokens
	for i := 0; i < 5; i++ {
		if allowed, _ := limiter.Allow(clientKey, limit); !allowed {
			t.Fatalf("expected request %d to be allowed", i)
		}
	}

	// Immediately refused
	if allowed, _ := limiter.Allow(clientKey, limit); allowed {
		t.Fatalf("expected request to be refused")
	}

	// Advance time by 6 seconds (half a token, still < 1 token)
	mockTime = mockTime.Add(6 * time.Second)
	if allowed, _ := limiter.Allow(clientKey, limit); allowed {
		t.Fatalf("expected request to be refused after 6s (only 0.5 tokens refilled)")
	}

	// Advance time by another 6 seconds (total 12s -> 1 token refilled)
	mockTime = mockTime.Add(6 * time.Second)
	allowed, retryAfter := limiter.Allow(clientKey, limit)
	if !allowed {
		t.Fatalf("expected request to be allowed after 12s replenishment, retryAfter=%v", retryAfter)
	}

	// Consumed the 1 token, next immediate request should be refused
	if allowed, _ := limiter.Allow(clientKey, limit); allowed {
		t.Fatalf("expected immediate request to be refused")
	}

	// Advance time by 2 minutes (tokens should cap at burst capacity 5, not accumulate beyond)
	mockTime = mockTime.Add(2 * time.Minute)
	for i := 1; i <= 5; i++ {
		if allowed, _ := limiter.Allow(clientKey, limit); !allowed {
			t.Fatalf("expected token %d to be available after full replenish", i)
		}
	}
	// 6th request should fail
	if allowed, _ := limiter.Allow(clientKey, limit); allowed {
		t.Fatalf("tokens should be capped at burst capacity of 5")
	}
}

func TestRefusalWhenEmpty(t *testing.T) {
	limiter := New(WithCleanupInterval(0))
	defer limiter.Close()

	limit := Limit{
		Name:     "refusal-test",
		Requests: 1,
		Window:   time.Minute,
		Burst:    1,
	}

	key := "refusal-client"
	allowed, retryAfter := limiter.Allow(key, limit)
	if !allowed || retryAfter != 0 {
		t.Fatalf("first request should be allowed")
	}

	allowed, retryAfter = limiter.Allow(key, limit)
	if allowed {
		t.Fatalf("second request should be refused")
	}
	if retryAfter < 50*time.Second || retryAfter > 60*time.Second {
		t.Fatalf("expected retryAfter around 60s, got %v", retryAfter)
	}
}

func TestTrustedProxyVsSpoofedXForwardedFor(t *testing.T) {
	limiter := New(
		WithTrustedProxies("10.0.0.0/8", "192.168.1.100"),
		WithCleanupInterval(0),
	)
	defer limiter.Close()

	tests := []struct {
		name        string
		remoteAddr  string
		xff         string
		expectedKey string
	}{
		{
			name:        "untrusted remote IP attempting spoofed XFF",
			remoteAddr:  "203.0.113.195:54321",
			xff:         "198.51.100.1",
			expectedKey: "203.0.113.195", // Spoofed header ignored
		},
		{
			name:        "trusted loopback IPv4 remote IP with XFF",
			remoteAddr:  "127.0.0.1:40000",
			xff:         "198.51.100.22",
			expectedKey: "198.51.100.22", // Trusted proxy honored
		},
		{
			name:        "trusted loopback IPv6 remote IP with XFF",
			remoteAddr:  "[::1]:40000",
			xff:         "198.51.100.33",
			expectedKey: "198.51.100.33",
		},
		{
			name:        "trusted CIDR remote IP with multiple XFF addresses",
			remoteAddr:  "10.2.3.4:50000",
			xff:         "198.51.100.44, 10.0.0.1",
			expectedKey: "198.51.100.44", // Leftmost address selected
		},
		{
			name:        "trusted explicit IP with XFF",
			remoteAddr:  "192.168.1.100:50000",
			xff:         "198.51.100.55",
			expectedKey: "198.51.100.55",
		},
		{
			name:        "trusted proxy with invalid XFF falls back to remote IP",
			remoteAddr:  "127.0.0.1:40000",
			xff:         "invalid-ip-address",
			expectedKey: "127.0.0.1",
		},
		{
			name:        "trusted proxy without XFF uses remote IP",
			remoteAddr:  "127.0.0.1:40000",
			xff:         "",
			expectedKey: "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			resolved := limiter.ResolveKey(req)
			if resolved != tt.expectedKey {
				t.Errorf("ResolveKey() = %q, want %q", resolved, tt.expectedKey)
			}
		})
	}
}

func TestIndependentTrackingAcrossDistinctClientIPs(t *testing.T) {
	limiter := New(WithCleanupInterval(0))
	defer limiter.Close()

	limit := Limit{
		Name:     "ip-isolation-limit",
		Requests: 3,
		Window:   time.Minute,
		Burst:    3,
	}

	clientA := "192.0.2.1"
	clientB := "192.0.2.2"

	// Client A consumes all tokens
	for i := 0; i < 3; i++ {
		if allowed, _ := limiter.Allow(clientA, limit); !allowed {
			t.Fatalf("client A request %d should be allowed", i)
		}
	}
	// Client A is blocked
	if allowed, _ := limiter.Allow(clientA, limit); allowed {
		t.Fatalf("client A should be blocked")
	}

	// Client B should still have full quota
	for i := 0; i < 3; i++ {
		if allowed, _ := limiter.Allow(clientB, limit); !allowed {
			t.Fatalf("client B request %d should be allowed independently", i)
		}
	}
	// Client B is blocked
	if allowed, _ := limiter.Allow(clientB, limit); allowed {
		t.Fatalf("client B should be blocked now")
	}
}

func TestAuthenticatedSubjectResolution(t *testing.T) {
	limiter := New(
		WithSubjectExtractor(func(r *http.Request) string {
			return r.Header.Get("X-User-Subject")
		}),
		WithCleanupInterval(0),
	)
	defer limiter.Close()

	limit := Limit{
		Name:     "user-sub-limit",
		Requests: 2,
		Window:   time.Minute,
		Burst:    2,
	}

	// Two requests from different remote addrs but same user subject
	req1 := httptest.NewRequest(http.MethodPost, "/api", nil)
	req1.RemoteAddr = "192.0.2.1:1111"
	req1.Header.Set("X-User-Subject", "subject-user-abc")

	req2 := httptest.NewRequest(http.MethodPost, "/api", nil)
	req2.RemoteAddr = "198.51.100.99:2222"
	req2.Header.Set("X-User-Subject", "subject-user-abc")

	key1 := limiter.ResolveKey(req1)
	key2 := limiter.ResolveKey(req2)
	if key1 != "subject-user-abc" || key2 != "subject-user-abc" {
		t.Fatalf("expected keys to match subject, got %q and %q", key1, key2)
	}

	// Use tokens under subject
	if allowed, _ := limiter.Allow(key1, limit); !allowed {
		t.Fatalf("request 1 should be allowed")
	}
	if allowed, _ := limiter.Allow(key2, limit); !allowed {
		t.Fatalf("request 2 should be allowed")
	}
	// 3rd request should be blocked for this subject
	if allowed, _ := limiter.Allow(key1, limit); allowed {
		t.Fatalf("request 3 should be blocked for subject")
	}

	// Request with context subject
	ctx := ContextWithSubject(context.Background(), "ctx-user-xyz")
	reqCtx := httptest.NewRequest(http.MethodGet, "/api", nil).WithContext(ctx)
	reqCtx.RemoteAddr = "192.0.2.5:3333"

	// Limiter without header subject extractor should pick up context
	limiterCtx := New(WithCleanupInterval(0))
	defer limiterCtx.Close()

	keyCtx := limiterCtx.ResolveKey(reqCtx)
	if keyCtx != "ctx-user-xyz" {
		t.Fatalf("expected context subject %q, got %q", "ctx-user-xyz", keyCtx)
	}
}

func TestRouteTierIsolation(t *testing.T) {
	limiter := New(WithCleanupInterval(0))
	defer limiter.Close()

	client := "192.0.2.1"

	// Exhaust StrictLimit (burst 5)
	for i := 0; i < 5; i++ {
		if allowed, _ := limiter.Allow(client, StrictLimit); !allowed {
			t.Fatalf("strict request %d should be allowed", i)
		}
	}
	if allowed, _ := limiter.Allow(client, StrictLimit); allowed {
		t.Fatalf("strict limit should be exhausted")
	}

	// ModerateLimit for the same client should NOT be affected
	for i := 0; i < 15; i++ {
		if allowed, _ := limiter.Allow(client, ModerateLimit); !allowed {
			t.Fatalf("moderate request %d should be allowed independently", i)
		}
	}
	if allowed, _ := limiter.Allow(client, ModerateLimit); allowed {
		t.Fatalf("moderate limit should be exhausted")
	}

	// GenerousLimit for the same client should also be independent
	if allowed, _ := limiter.Allow(client, GenerousLimit); !allowed {
		t.Fatalf("generous limit request should be allowed")
	}
}

func TestHTTPMiddleware(t *testing.T) {
	limiter := New(WithCleanupInterval(0))
	defer limiter.Close()

	limit := Limit{
		Name:     "mw-test",
		Requests: 2,
		Window:   time.Minute,
		Burst:    2,
	}

	handler := limiter.RequireRateLimitFunc(limit, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// 1st request -> 200
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.0.2.10:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr1.Code)
	}

	// 2nd request -> 200
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.0.2.10:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr2.Code)
	}

	// 3rd request -> 429 Too Many Requests
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.0.2.10:1234"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rr3.Code)
	}

	contentType := rr3.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", contentType)
	}

	retryAfterHeader := rr3.Header().Get("Retry-After")
	if retryAfterHeader == "" {
		t.Fatalf("expected Retry-After header to be set")
	}
	retrySec, err := strconv.Atoi(retryAfterHeader)
	if err != nil || retrySec < 1 {
		t.Fatalf("invalid Retry-After header: %q", retryAfterHeader)
	}

	var errResp map[string]string
	if err := json.Unmarshal(rr3.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if errResp["error"] != "Rate limit exceeded, please retry later" {
		t.Errorf("unexpected error message: %q", errResp["error"])
	}
}

func TestHTTPMiddlewareByMethod(t *testing.T) {
	limiter := New(WithCleanupInterval(0))
	defer limiter.Close()

	methodLimits := map[string]Limit{
		http.MethodPost: {
			Name:     "post-limit",
			Requests: 1,
			Window:   time.Minute,
			Burst:    1,
		},
	}

	handler := limiter.RequireRateLimitByMethodFunc(methodLimits, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// POST 1 -> 200
	reqPost1 := httptest.NewRequest(http.MethodPost, "/item", nil)
	reqPost1.RemoteAddr = "192.0.2.50:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, reqPost1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr1.Code)
	}

	// POST 2 -> 429
	reqPost2 := httptest.NewRequest(http.MethodPost, "/item", nil)
	reqPost2.RemoteAddr = "192.0.2.50:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, reqPost2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rr2.Code)
	}

	// GET -> not limited by methodLimits, should return 200
	reqGet := httptest.NewRequest(http.MethodGet, "/item", nil)
	reqGet.RemoteAddr = "192.0.2.50:1234"
	rrGet := httptest.NewRecorder()
	handler.ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET, got %d", rrGet.Code)
	}
}

func TestStaleKeyCleanup(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	mockTime := now
	clock := func() time.Time { return mockTime }

	limiter := New(
		WithClock(clock),
		WithTTL(10*time.Second),
		WithCleanupInterval(0),
	)
	defer limiter.Close()

	limit := Limit{Name: "ttl-test", Requests: 5, Window: time.Minute, Burst: 5}

	limiter.Allow("stale-client", limit)
	limiter.Allow("active-client", limit)

	// Advance time by 5 seconds (below TTL)
	mockTime = mockTime.Add(5 * time.Second)
	limiter.Allow("active-client", limit) // Refresh active-client

	// Run cleanup at t = 5s
	limiter.CleanupStale(mockTime)

	limiter.mu.Lock()
	if len(limiter.buckets) != 2 {
		t.Errorf("expected 2 buckets at 5s, got %d", len(limiter.buckets))
	}
	limiter.mu.Unlock()

	// Advance time to t = 12s (stale-client was last accessed at t=0s, so >10s TTL; active was at 5s)
	mockTime = mockTime.Add(7 * time.Second)
	limiter.CleanupStale(mockTime)

	limiter.mu.Lock()
	if len(limiter.buckets) != 1 {
		t.Errorf("expected 1 bucket at 12s, got %d", len(limiter.buckets))
	}
	_, hasActive := limiter.buckets[bucketKey("active-client", limit)]
	if !hasActive {
		t.Errorf("expected active-client bucket to be preserved")
	}
	limiter.mu.Unlock()
}

func TestConcurrentAccess(t *testing.T) {
	limiter := New(WithCleanupInterval(0))
	defer limiter.Close()

	limit := Limit{
		Name:     "concurrent-limit",
		Requests: 1000,
		Window:   time.Minute,
		Burst:    1000,
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			key := "client-" + strconv.Itoa(workerID%5)
			for j := 0; j < 50; j++ {
				limiter.Allow(key, limit)
			}
		}(i)
	}
	wg.Wait()
}

func TestNoopLimiter(t *testing.T) {
	noop := NewNoop()
	defer noop.Close()

	limit := Limit{Name: "strict", Requests: 1, Window: time.Minute, Burst: 1}
	for i := 0; i < 10; i++ {
		allowed, retry := noop.Allow("any-key", limit)
		if !allowed || retry != 0 {
			t.Fatalf("noop limiter must always allow")
		}
	}

	handler := noop.RequireRateLimitFunc(limit, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("noop middleware must allow")
	}
}
