package webhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var defaultRetryDelays = []time.Duration{
	5 * time.Second,
	15 * time.Second,
	45 * time.Second,
}

// DispatcherOption configures a Dispatcher instance.
type DispatcherOption func(*Dispatcher)

// WithRetryDelays sets custom retry backoff durations between attempts.
func WithRetryDelays(delays ...time.Duration) DispatcherOption {
	return func(d *Dispatcher) {
		d.retryDelays = delays
	}
}

// WithHTTPClient sets a custom HTTP client for the dispatcher.
func WithHTTPClient(client *http.Client) DispatcherOption {
	return func(d *Dispatcher) {
		d.httpClient = client
	}
}

// WithAllowLocalhost permits loopback target URLs (useful in integration/unit tests).
func WithAllowLocalhost(allow bool) DispatcherOption {
	return func(d *Dispatcher) {
		d.allowLocalhost = allow
	}
}

// WithTimeout sets the per-attempt HTTP timeout.
func WithTimeout(timeout time.Duration) DispatcherOption {
	return func(d *Dispatcher) {
		d.timeout = timeout
	}
}

// Dispatcher is a background service and delivery worker that securely dispatches
// webhook events with HMAC signatures, geospatial geofencing, SSRF protection,
// and exponential backoff retries.
type Dispatcher struct {
	store          store.StateStore
	httpClient     *http.Client
	retryDelays    []time.Duration
	timeout        time.Duration
	allowLocalhost bool
}

// NewDispatcher creates a new Dispatcher backed by the given StateStore and options.
func NewDispatcher(st store.StateStore, opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		store:          st,
		retryDelays:    defaultRetryDelays,
		timeout:        10 * time.Second,
		allowLocalhost: false,
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.httpClient == nil {
		d.httpClient = newSSRFSafeHTTPClient(d.allowLocalhost, d.timeout)
	}

	return d
}

// Deliver delivers a webhook event to a subscription, adhering to geospatial filters,
// HMAC signing, SSRF validation, and backoff retries.
//
// If the event falls outside the subscription's GeoFence or event filter, or if the
// subscription is inactive, delivery is skipped and (nil, nil) is returned.
func (d *Dispatcher) Deliver(
	ctx context.Context,
	sub *WebhookSubscription,
	eventType string,
	payload []byte,
	eventCoords *domain.LocationPoint,
) (*WebhookDeliveryRecord, error) {
	if sub == nil || !sub.Active {
		return nil, nil
	}

	// Filter by event type if specified on subscription
	if len(sub.FilterEvents) > 0 {
		matched := false
		for _, f := range sub.FilterEvents {
			if f == eventType {
				matched = true
				break
			}
		}
		if !matched {
			return nil, nil
		}
	}

	// Geospatial filtering: if GeoFence is configured, verify event is within radius
	if sub.GeoFence != nil {
		if eventCoords == nil {
			return nil, nil
		}
		center := domain.LocationPoint{
			Latitude:  sub.GeoFence.CenterLat,
			Longitude: sub.GeoFence.CenterLng,
		}
		dist := domain.HaversineDistanceMiles(center, *eventCoords)
		if dist > sub.GeoFence.RadiusMiles {
			return nil, nil
		}
	}

	eventID := extractEventID(payload)

	// Pre-validate target URL for SSRF
	if !d.allowLocalhost {
		if err := ValidateURL(sub.TargetURL); err != nil {
			record := &WebhookDeliveryRecord{
				ID:             generateDeliveryID(),
				SubscriptionID: sub.ID,
				EventID:        eventID,
				EventType:      eventType,
				TargetURL:      sub.TargetURL,
				StatusCode:     0,
				Success:        false,
				Error:          err.Error(),
				Attempt:        0,
				ResponseTimeMs: 0,
				CreatedAt:      time.Now().UTC(),
			}
			d.saveDeliveryRecord(ctx, record)
			return record, fmt.Errorf("invalid webhook target URL: %w", err)
		}
	}

	// Generate HMAC signature header
	sig := GenerateSignature(payload, sub.Secret)

	maxAttempts := 1 + len(d.retryDelays)
	var lastStatusCode int
	var lastErr error
	var totalDuration time.Duration
	attempts := 0

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attempts = attempt

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.TargetURL, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to build webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(SignatureHeader, sig)
		req.Header.Set("User-Agent", "PetSpotR-Webhook-Dispatcher/1.0")

		start := time.Now()
		resp, err := d.httpClient.Do(req)
		duration := time.Since(start)
		totalDuration = duration

		if err != nil {
			lastErr = err
			lastStatusCode = 0

			// Context canceled during HTTP request
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			// If SSRF dialer rejected destination IP, do not retry
			if errors.Is(err, ErrBlockedAddress) {
				break
			}

			// Network error: retry with backoff if attempts remaining
			if attempt < maxAttempts {
				delay := d.retryDelays[attempt-1]
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			break
		}

		lastStatusCode = resp.StatusCode
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()

		// Success: 2xx response
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < 300 {
			record := &WebhookDeliveryRecord{
				ID:             generateDeliveryID(),
				SubscriptionID: sub.ID,
				EventID:        eventID,
				EventType:      eventType,
				TargetURL:      sub.TargetURL,
				StatusCode:     resp.StatusCode,
				Success:        true,
				Attempt:        attempt,
				ResponseTimeMs: duration.Milliseconds(),
				CreatedAt:      time.Now().UTC(),
			}
			d.saveDeliveryRecord(ctx, record)
			return record, nil
		}

		// 5xx Server Error: retry with backoff if attempts remaining
		if resp.StatusCode >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("server responded with status %d", resp.StatusCode)
			if attempt < maxAttempts {
				delay := d.retryDelays[attempt-1]
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			break
		}

		// 3xx / 4xx Client Error: do not retry
		lastErr = fmt.Errorf("server responded with client error status %d", resp.StatusCode)
		break
	}

	// Delivery failure after all attempts or non-retryable error
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	record := &WebhookDeliveryRecord{
		ID:             generateDeliveryID(),
		SubscriptionID: sub.ID,
		EventID:        eventID,
		EventType:      eventType,
		TargetURL:      sub.TargetURL,
		StatusCode:     lastStatusCode,
		Success:        false,
		Error:          errMsg,
		Attempt:        attempts,
		ResponseTimeMs: totalDuration.Milliseconds(),
		CreatedAt:      time.Now().UTC(),
	}
	d.saveDeliveryRecord(ctx, record)

	return record, fmt.Errorf("webhook delivery failed: %w", lastErr)
}

// DispatchEvent looks up all active subscriptions from the StateStore, applies filters,
// and delivers the webhook to matching partners.
func (d *Dispatcher) DispatchEvent(
	ctx context.Context,
	eventType string,
	payload []byte,
	eventCoords *domain.LocationPoint,
) ([]*WebhookDeliveryRecord, error) {
	if d.store == nil {
		return nil, nil
	}

	rawSubs, err := d.store.ListState(ctx, store.WebhooksCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	var (
		mu      sync.Mutex
		records []*WebhookDeliveryRecord
		wg      sync.WaitGroup
	)

	for _, raw := range rawSubs {
		var sub WebhookSubscription
		if err := json.Unmarshal(raw, &sub); err != nil {
			continue
		}
		if !sub.Active {
			continue
		}

		wg.Add(1)
		subCopy := sub
		go func(s WebhookSubscription) {
			defer wg.Done()
			rec, _ := d.Deliver(ctx, &s, eventType, payload, eventCoords)
			if rec != nil {
				mu.Lock()
				records = append(records, rec)
				mu.Unlock()
			}
		}(subCopy)
	}

	wg.Wait()
	return records, nil
}

func (d *Dispatcher) saveDeliveryRecord(ctx context.Context, record *WebhookDeliveryRecord) {
	if d.store == nil || record == nil {
		return
	}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	_ = d.store.SaveState(ctx, store.WebhookDeliveriesCollection, record.ID, data)
}

// SSRFSafeDialerControl returns a net.Dialer Control callback that prevents outbound
// connections to private, loopback, or metadata addresses at socket connection time.
func SSRFSafeDialerControl(allowLocalhost bool) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		if allowLocalhost {
			return nil
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("failed to split host and port %q: %w", address, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("invalid socket IP %q", host)
		}
		if err := ValidateIP(ip); err != nil {
			return fmt.Errorf("%w: target IP %s is restricted", ErrBlockedAddress, ip.String())
		}
		return nil
	}
}

func newSSRFSafeHTTPClient(allowLocalhost bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control:   SSRFSafeDialerControl(allowLocalhost),
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func extractEventID(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var meta struct {
		ID      string `json:"id"`
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(payload, &meta); err == nil {
		if meta.EventID != "" {
			return meta.EventID
		}
		if meta.ID != "" {
			return meta.ID
		}
	}
	return ""
}

func generateDeliveryID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("del-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("del-%s", hex.EncodeToString(random[:]))
}
