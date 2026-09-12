// Package ratelimit provides in-memory token-bucket rate limiting with
// route-specific tiers, trusted proxy IP resolution, and HTTP middleware.
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limit defines the rate limit constraints for an endpoint or route tier.
type Limit struct {
	Name     string
	Requests int
	Window   time.Duration
	Burst    int
}

var (
	// StrictLimit is for expensive AI feature extraction (5 requests/minute, burst 5).
	StrictLimit = Limit{
		Name:     "strict",
		Requests: 5,
		Window:   time.Minute,
		Burst:    5,
	}

	// ModerateLimit is for state mutations (15 requests/minute, burst 15).
	ModerateLimit = Limit{
		Name:     "moderate",
		Requests: 15,
		Window:   time.Minute,
		Burst:    15,
	}

	// GenerousLimit is for read queries (120 requests/minute, burst 60).
	GenerousLimit = Limit{
		Name:     "generous",
		Requests: 120,
		Window:   time.Minute,
		Burst:    60,
	}
)

type subjectCtxKey struct{}

// ContextWithSubject returns a new context containing the authenticated user identity subject.
func ContextWithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectCtxKey{}, subject)
}

// SubjectFromContext extracts the authenticated user identity subject if present.
func SubjectFromContext(ctx context.Context) (string, bool) {
	if val, ok := ctx.Value(subjectCtxKey{}).(string); ok && val != "" {
		return val, true
	}
	return "", false
}

// Limiter defines the interface for route-level and client-key rate limiting.
type Limiter interface {
	// Allow checks if the client key is allowed under the given Limit.
	// Returns allowed (bool) and the duration to wait before retrying (retryAfter).
	Allow(key string, limit Limit) (bool, time.Duration)

	// ResolveKey extracts the client key (authenticated subject or client IP) from the request.
	ResolveKey(r *http.Request) string

	// RequireRateLimit returns an HTTP middleware enforcing the specified limit.
	RequireRateLimit(limit Limit) func(http.Handler) http.Handler

	// RequireRateLimitFunc wraps an http.HandlerFunc with rate limiting for the specified limit.
	RequireRateLimitFunc(limit Limit, next http.HandlerFunc) http.HandlerFunc

	// RequireRateLimitByMethod returns an HTTP middleware enforcing limits by HTTP method.
	RequireRateLimitByMethod(methodLimits map[string]Limit, fallback *Limit) func(http.Handler) http.Handler

	// RequireRateLimitByMethodFunc wraps an http.HandlerFunc with method-based rate limiting.
	RequireRateLimitByMethodFunc(methodLimits map[string]Limit, fallback *Limit, next http.HandlerFunc) http.HandlerFunc

	// Close terminates any background workers.
	Close()
}

type tokenBucket struct {
	tokens          float64
	lastReplenished time.Time
	lastAccessed    time.Time
}

// TokenBucketLimiter implements a thread-safe token-bucket rate limiter.
type TokenBucketLimiter struct {
	mu               sync.Mutex
	buckets          map[string]*tokenBucket
	trustedCIDRs     []*net.IPNet
	trustedIPs       []net.IP
	subjectExtractor func(r *http.Request) string
	clock            func() time.Time
	ttl              time.Duration
	cleanupInterval  time.Duration
	stopCh           chan struct{}
	stopOnce         sync.Once
}

// Option configures a TokenBucketLimiter.
type Option func(*TokenBucketLimiter)

// WithTrustedProxies configures trusted proxy IP addresses or CIDR blocks.
func WithTrustedProxies(proxies ...string) Option {
	return func(l *TokenBucketLimiter) {
		for _, p := range proxies {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if strings.Contains(p, "/") {
				_, ipNet, err := net.ParseCIDR(p)
				if err == nil && ipNet != nil {
					l.trustedCIDRs = append(l.trustedCIDRs, ipNet)
				}
			} else {
				parsed := net.ParseIP(p)
				if parsed != nil {
					l.trustedIPs = append(l.trustedIPs, parsed)
				}
			}
		}
	}
}

// WithSubjectExtractor configures a custom function to extract an authenticated identity subject from requests.
func WithSubjectExtractor(fn func(r *http.Request) string) Option {
	return func(l *TokenBucketLimiter) {
		l.subjectExtractor = fn
	}
}

// WithClock overrides the default time.Now function (useful in testing).
func WithClock(clock func() time.Time) Option {
	return func(l *TokenBucketLimiter) {
		l.clock = clock
	}
}

// WithTTL sets the duration after which inactive client buckets are pruned.
func WithTTL(ttl time.Duration) Option {
	return func(l *TokenBucketLimiter) {
		if ttl > 0 {
			l.ttl = ttl
		}
	}
}

// WithCleanupInterval sets how often the background pruner runs.
// Pass 0 or negative to disable the background goroutine.
func WithCleanupInterval(interval time.Duration) Option {
	return func(l *TokenBucketLimiter) {
		l.cleanupInterval = interval
	}
}

// New creates and initializes a new TokenBucketLimiter.
func New(opts ...Option) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		buckets:         make(map[string]*tokenBucket),
		clock:           time.Now,
		ttl:             5 * time.Minute,
		cleanupInterval: 1 * time.Minute,
		stopCh:          make(chan struct{}),
	}
	for _, opt := range opts {
		opt(limiter)
	}

	if limiter.cleanupInterval > 0 {
		go limiter.cleanupLoop()
	}

	return limiter
}

// NewDefault returns a Limiter initialized with default production settings.
func NewDefault() Limiter {
	return New()
}

func (l *TokenBucketLimiter) now() time.Time {
	if l.clock != nil {
		return l.clock()
	}
	return time.Now()
}

func (l *TokenBucketLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.CleanupStale(l.now())
		}
	}
}

// CleanupStale removes client buckets that have not been accessed within the TTL.
func (l *TokenBucketLimiter) CleanupStale(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if now.Sub(b.lastAccessed) > l.ttl {
			delete(l.buckets, k)
		}
	}
}

func bucketKey(clientKey string, limit Limit) string {
	limitName := limit.Name
	if limitName == "" {
		limitName = fmt.Sprintf("%d/%s/b%d", limit.Requests, limit.Window, limit.Burst)
	}
	return clientKey + "\x00" + limitName
}

// Allow checks if clientKey is permitted to consume a token under the given Limit.
func (l *TokenBucketLimiter) Allow(key string, limit Limit) (bool, time.Duration) {
	if key == "" {
		key = "unknown"
	}

	burst := limit.Burst
	if burst <= 0 {
		burst = limit.Requests
	}
	if burst <= 0 {
		burst = 1
	}
	capacity := float64(burst)

	window := limit.Window
	if window <= 0 {
		window = time.Minute
	}
	rate := float64(limit.Requests) / window.Seconds()

	now := l.now()
	bKey := bucketKey(key, limit)

	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.buckets[bKey]
	if !exists {
		b = &tokenBucket{
			tokens:          capacity,
			lastReplenished: now,
			lastAccessed:    now,
		}
		l.buckets[bKey] = b
	}

	elapsed := now.Sub(b.lastReplenished).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(capacity, b.tokens+elapsed*rate)
		b.lastReplenished = now
	}
	b.lastAccessed = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, 0
	}

	if rate <= 0 {
		return false, time.Hour
	}

	missing := 1.0 - b.tokens
	secondsWait := missing / rate
	retryAfterSec := int(math.Ceil(secondsWait))
	if retryAfterSec < 1 {
		retryAfterSec = 1
	}
	return false, time.Duration(retryAfterSec) * time.Second
}

// isTrustedProxy checks if an IP address is localhost or among configured trusted proxies.
func (l *TokenBucketLimiter) isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, trusted := range l.trustedIPs {
		if trusted.Equal(ip) {
			return true
		}
	}
	for _, cidr := range l.trustedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// extractClientIP retrieves the client IP address safely from the request.
// Only trusts X-Forwarded-For if RemoteAddr is a trusted proxy or localhost.
func (l *TokenBucketLimiter) extractClientIP(r *http.Request) string {
	remoteIPStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIPStr = strings.TrimSpace(r.RemoteAddr)
	}
	if remoteIPStr == "" {
		return "127.0.0.1"
	}

	remoteIP := net.ParseIP(remoteIPStr)
	if remoteIP != nil && l.isTrustedProxy(remoteIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(parts[0])
			if host, _, err := net.SplitHostPort(clientIP); err == nil {
				clientIP = host
			}
			if parsed := net.ParseIP(clientIP); parsed != nil {
				return clientIP
			}
		}
	}

	return remoteIPStr
}

// ResolveKey returns the rate limiting key for an HTTP request.
// Prefers authenticated user identity subject if available, falling back to client IP.
func (l *TokenBucketLimiter) ResolveKey(r *http.Request) string {
	if l.subjectExtractor != nil {
		if sub := l.subjectExtractor(r); sub != "" {
			return sub
		}
	}
	if sub, ok := SubjectFromContext(r.Context()); ok && sub != "" {
		return sub
	}
	return l.extractClientIP(r)
}

func writeRateLimitExceeded(w http.ResponseWriter, retryAfter time.Duration) {
	retryAfterSec := int(math.Ceil(retryAfter.Seconds()))
	if retryAfterSec < 1 {
		retryAfterSec = 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Rate limit exceeded, please retry later",
	})
}

// RequireRateLimit returns an HTTP middleware enforcing the specified limit.
func (l *TokenBucketLimiter) RequireRateLimit(limit Limit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := l.ResolveKey(r)
			allowed, retryAfter := l.Allow(key, limit)
			if !allowed {
				writeRateLimitExceeded(w, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRateLimitFunc wraps an http.HandlerFunc with rate limiting.
func (l *TokenBucketLimiter) RequireRateLimitFunc(limit Limit, next http.HandlerFunc) http.HandlerFunc {
	return l.RequireRateLimit(limit)(next).ServeHTTP
}

// RequireRateLimitByMethod returns an HTTP middleware enforcing limits based on HTTP method.
func (l *TokenBucketLimiter) RequireRateLimitByMethod(methodLimits map[string]Limit, fallback *Limit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit, ok := methodLimits[r.Method]
			if !ok {
				if fallback != nil {
					limit = *fallback
				} else {
					next.ServeHTTP(w, r)
					return
				}
			}
			key := l.ResolveKey(r)
			allowed, retryAfter := l.Allow(key, limit)
			if !allowed {
				writeRateLimitExceeded(w, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRateLimitByMethodFunc wraps an http.HandlerFunc with method-based rate limiting.
func (l *TokenBucketLimiter) RequireRateLimitByMethodFunc(methodLimits map[string]Limit, fallback *Limit, next http.HandlerFunc) http.HandlerFunc {
	return l.RequireRateLimitByMethod(methodLimits, fallback)(next).ServeHTTP
}

// Close terminates the background cleanup goroutine.
func (l *TokenBucketLimiter) Close() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
}

// noopLimiter provides an inert Limiter implementation for testing.
type noopLimiter struct{}

// NewNoop creates a Limiter that performs no rate limiting.
func NewNoop() Limiter {
	return &noopLimiter{}
}

func (n *noopLimiter) Allow(string, Limit) (bool, time.Duration) {
	return true, 0
}

func (n *noopLimiter) ResolveKey(r *http.Request) string {
	remoteIPStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIPStr = strings.TrimSpace(r.RemoteAddr)
	}
	return remoteIPStr
}

func (n *noopLimiter) RequireRateLimit(Limit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func (n *noopLimiter) RequireRateLimitFunc(_ Limit, next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (n *noopLimiter) RequireRateLimitByMethod(map[string]Limit, *Limit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func (n *noopLimiter) RequireRateLimitByMethodFunc(_ map[string]Limit, _ *Limit, next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (n *noopLimiter) Close() {}
