package webfrontend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/ratelimit"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestRateLimitStrict_ExtractFeatures(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	payload := `{"imageUrl":"https://storage.petspotr.io/images/found-dog.jpg"}`
	clientIP := "192.0.2.10:1234"

	// StrictLimit allows 5 requests in burst
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d expected 200 OK, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// 6th request must be rejected with 429 Too Many Requests
	req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 6 expected 429 Too Many Requests, got %d", rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	retryAfterHeader := rr.Header().Get("Retry-After")
	if retryAfterHeader == "" {
		t.Fatal("expected Retry-After header to be present")
	}
	retrySec, err := strconv.Atoi(retryAfterHeader)
	if err != nil || retrySec < 1 {
		t.Errorf("invalid Retry-After value: %q", retryAfterHeader)
	}

	var errResp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if errResp["error"] != "Rate limit exceeded, please retry later" {
		t.Errorf("unexpected error message: %q", errResp["error"])
	}
}

func TestRateLimitModerate_LostPetReport(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	clientIP := "192.0.2.20:1234"

	// ModerateLimit allows 15 requests in burst
	for i := 1; i <= 15; i++ {
		payload, _ := json.Marshal(map[string]any{
			"petName":       "Buddy",
			"species":       "dog",
			"breed":         "Labrador",
			"reporterEmail": "owner@example.com",
			"location":      "Seattle, WA",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("request %d expected 201 Created, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// 16th request must be rate limited
	payload, _ := json.Marshal(map[string]any{
		"petName":       "Buddy",
		"species":       "dog",
		"reporterEmail": "owner@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 16 expected 429 Too Many Requests, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}

func TestRateLimitModerate_FoundPetReport(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	clientIP := "192.0.2.30:1234"

	// ModerateLimit allows 15 requests in burst
	for i := 1; i <= 15; i++ {
		payload, _ := json.Marshal(map[string]any{
			"imageUrl":    "https://storage.petspotr.io/images/found.jpg",
			"location":    "Seattle, WA",
			"finderEmail": "finder@example.com",
			"species":     "dog",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("request %d expected 201 Created, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// 16th request must be rate limited
	payload, _ := json.Marshal(map[string]any{
		"imageUrl":    "https://storage.petspotr.io/images/found.jpg",
		"location":    "Seattle, WA",
		"finderEmail": "finder@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 16 expected 429 Too Many Requests, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}

func TestRateLimitModerate_ReunionsContact(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	clientIP := "192.0.2.40:1234"

	// ModerateLimit allows 15 requests in burst
	for i := 1; i <= 15; i++ {
		payload, _ := json.Marshal(map[string]string{
			"matchId":     "match-101",
			"senderEmail": "test@example.com",
			"message":     "Hello!",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/contact", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d expected 200 OK, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// 16th request must be rate limited
	payload, _ := json.Marshal(map[string]string{
		"matchId":     "match-101",
		"senderEmail": "test@example.com",
		"message":     "Hello!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/contact", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 16 expected 429 Too Many Requests, got %d", rr.Code)
	}
}

func TestRateLimitGenerous_PetDirectory(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	clientIP := "192.0.2.50:1234"

	// GenerousLimit allows burst 60
	for i := 1; i <= 60; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pets", nil)
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d expected 200 OK, got %d", i, rr.Code)
		}
	}

	// 61st request must be rate limited
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pets", nil)
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 61 expected 429 Too Many Requests, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}

func TestRateLimitGenerous_Matches(t *testing.T) {
	srv := NewDemoServer()
	defer srv.Close()

	clientIP := "192.0.2.60:1234"

	// GenerousLimit allows burst 60
	for i := 1; i <= 60; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d expected 200 OK, got %d", i, rr.Code)
		}
	}

	// 61st request must be rate limited
	req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 61 expected 429 Too Many Requests, got %d", rr.Code)
	}
}

func TestRateLimitClientIsolationInServer(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	payload := `{"imageUrl":"https://storage.petspotr.io/images/found.jpg"}`

	// Client A exhausts strict limit (5)
	clientA := "192.0.2.70:1234"
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
		req.RemoteAddr = clientA
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("client A request %d failed: %d", i, rr.Code)
		}
	}

	// Client A is 429'd
	reqA := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
	reqA.RemoteAddr = clientA
	rrA := httptest.NewRecorder()
	srv.ServeHTTP(rrA, reqA)
	if rrA.Code != http.StatusTooManyRequests {
		t.Fatalf("client A should be rate limited, got %d", rrA.Code)
	}

	// Client B should be allowed
	clientB := "192.0.2.71:1234"
	reqB := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
	reqB.RemoteAddr = clientB
	rrB := httptest.NewRecorder()
	srv.ServeHTTP(rrB, reqB)
	if rrB.Code != http.StatusOK {
		t.Fatalf("client B should be allowed, got %d", rrB.Code)
	}
}

func TestRateLimitTrustedProxyAndSpoofingInServer(t *testing.T) {
	limiter := ratelimit.New(
		ratelimit.WithTrustedProxies("10.0.0.1"),
	)
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		AllowPrivilegedMutations: true,
		RateLimiter:              limiter,
	})
	defer srv.Close()

	payload := `{"imageUrl":"https://storage.petspotr.io/images/found.jpg"}`

	// Untrusted client attempts to spoof X-Forwarded-For
	untrustedRemote := "203.0.113.100:5432"
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
		req.RemoteAddr = untrustedRemote
		req.Header.Set("X-Forwarded-For", "198.51.100.1")
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d failed", i)
		}
	}

	// Next request from untrustedRemote with a DIFFERENT spoofed IP is still blocked
	// because rate limiter keys by untrusted RemoteAddr (203.0.113.100)
	reqSpoofed := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
	reqSpoofed.RemoteAddr = untrustedRemote
	reqSpoofed.Header.Set("X-Forwarded-For", "198.51.100.2")
	rrSpoofed := httptest.NewRecorder()
	srv.ServeHTTP(rrSpoofed, reqSpoofed)
	if rrSpoofed.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed request should be rate limited under actual client IP, got %d", rrSpoofed.Code)
	}

	// Requests via trusted proxy (10.0.0.1) track the XFF client IP
	trustedProxyRemote := "10.0.0.1:1234"
	clientViaProxy := "198.51.100.77"
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
		req.RemoteAddr = trustedProxyRemote
		req.Header.Set("X-Forwarded-For", clientViaProxy)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("proxied request %d failed: %d", i, rr.Code)
		}
	}

	// 6th request for clientViaProxy via proxy is blocked
	reqProxiedBlocked := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
	reqProxiedBlocked.RemoteAddr = trustedProxyRemote
	reqProxiedBlocked.Header.Set("X-Forwarded-For", clientViaProxy)
	rrBlocked := httptest.NewRecorder()
	srv.ServeHTTP(rrBlocked, reqProxiedBlocked)
	if rrBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("proxied client should be rate limited, got %d", rrBlocked.Code)
	}

	// Another client through the same proxy is allowed
	reqProxiedOther := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
	reqProxiedOther.RemoteAddr = trustedProxyRemote
	reqProxiedOther.Header.Set("X-Forwarded-For", "198.51.100.88")
	rrOther := httptest.NewRecorder()
	srv.ServeHTTP(rrOther, reqProxiedOther)
	if rrOther.Code != http.StatusOK {
		t.Fatalf("other proxied client should be allowed, got %d", rrOther.Code)
	}
}

func TestDisableRateLimitingOption(t *testing.T) {
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
	})
	defer srv.Close()

	payload := `{"imageUrl":"https://storage.petspotr.io/images/found.jpg"}`
	clientIP := "192.0.2.80:1234"

	// Strict limit would block after 5 requests; here we send 10
	for i := 1; i <= 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets/extract-features", strings.NewReader(payload))
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d should succeed when rate limiting is disabled, got %d", i, rr.Code)
		}
	}
}

func TestRateLimitModerate_ImageEndpoints(t *testing.T) {
	srv := NewServer()
	defer srv.Close()

	clientIP := "192.0.2.110:1234"

	// ModerateLimit allows 15 requests in burst
	for i := 1; i <= 15; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/images/extract-metadata", strings.NewReader("invalid"))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP
		rr := httptest.NewRecorder()

		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("request %d expected 400 Bad Request, got %d", i, rr.Code)
		}
	}

	// 16th request must be rate limited with 429
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/extract-metadata", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP
	rr := httptest.NewRecorder()

	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 16 expected 429 Too Many Requests, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}
