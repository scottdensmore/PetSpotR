package webhook_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestDispatcher_Delivery_Success(t *testing.T) {
	t.Parallel()

	var receivedHeaders http.Header
	var receivedBody []byte
	payload := []byte(`{"id":"pet-123","event":"pet_lost","species":"dog"}`)
	secret := "test-secret-key-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"received"}`))
	}))
	defer server.Close()

	st := store.NewMemoryStore()
	d := webhook.NewDispatcher(st, webhook.WithAllowLocalhost(true))
	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}

	sub := &webhook.WebhookSubscription{
		ID:        "sub-success-1",
		PartnerID: "partner-1",
		TargetURL: server.URL + "/webhook",
		Secret:    secret,
		Active:    true,
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", payload, nil)
	if err != nil {
		t.Fatalf("unexpected Deliver error: %v", err)
	}

	if record == nil {
		t.Fatal("expected non-nil delivery record")
	}
	if !record.Success {
		t.Errorf("record.Success = false, want true")
	}
	if record.StatusCode != http.StatusOK {
		t.Errorf("record.StatusCode = %d, want %d", record.StatusCode, http.StatusOK)
	}
	if record.Attempt != 1 {
		t.Errorf("record.Attempt = %d, want 1", record.Attempt)
	}
	if record.SubscriptionID != sub.ID {
		t.Errorf("record.SubscriptionID = %q, want %q", record.SubscriptionID, sub.ID)
	}
	if record.EventID != "pet-123" {
		t.Errorf("record.EventID = %q, want pet-123", record.EventID)
	}

	// Verify headers received by server
	if contentType := receivedHeaders.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
	sig := receivedHeaders.Get(webhook.SignatureHeader)
	if sig == "" {
		t.Fatalf("missing %s header", webhook.SignatureHeader)
	}
	if !webhook.VerifySignature(payload, secret, sig) {
		t.Errorf("signature verification failed for %q", sig)
	}
	if string(receivedBody) != string(payload) {
		t.Errorf("receivedBody = %q, want %q", string(receivedBody), string(payload))
	}

	// Verify persistence in store
	savedBytes, err := st.GetState(context.Background(), store.WebhookDeliveriesCollection, record.ID)
	if err != nil {
		t.Fatalf("failed to retrieve delivery record from store: %v", err)
	}
	var savedRecord webhook.WebhookDeliveryRecord
	if err := json.Unmarshal(savedBytes, &savedRecord); err != nil {
		t.Fatalf("failed to unmarshal saved delivery record: %v", err)
	}
	if savedRecord.ID != record.ID || !savedRecord.Success || savedRecord.StatusCode != 200 {
		t.Errorf("savedRecord mismatch: %+v", savedRecord)
	}
}

func TestDispatcher_SignatureHeader(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"event":"sighting_reported","sightingId":"sight-999"}`)
	secret := "secret-signing-key"

	var capturedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get(webhook.SignatureHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := webhook.NewDispatcher(nil, webhook.WithAllowLocalhost(true))

	sub := &webhook.WebhookSubscription{
		ID:        "sub-sig-test",
		TargetURL: server.URL,
		Secret:    secret,
		Active:    true,
	}

	_, err := d.Deliver(context.Background(), sub, "sighting_reported", payload, nil)
	if err != nil {
		t.Fatalf("Deliver failed: %v", err)
	}

	expectedSig := webhook.GenerateSignature(payload, secret)
	if capturedSig != expectedSig {
		t.Errorf("capturedSig = %q, want %q", capturedSig, expectedSig)
	}
	if !webhook.VerifySignature(payload, secret, capturedSig) {
		t.Error("VerifySignature returned false for captured header")
	}
}

func TestDispatcher_SSRFRejection(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	// Default dispatcher has allowLocalhost = false, strictly enforcing SSRF blocking
	d := webhook.NewDispatcher(st)

	blockedURLs := []string{
		"http://127.0.0.1:8080/hook",
		"http://localhost:8080/hook",
		"http://169.254.169.254/latest/meta-data",
		"https://10.0.1.5/webhook",
		"http://192.168.1.1/webhook",
		"http://metadata.google.internal/computeMetadata/v1",
	}

	for _, u := range blockedURLs {
		sub := &webhook.WebhookSubscription{
			ID:        "sub-blocked",
			TargetURL: u,
			Secret:    "sec",
			Active:    true,
		}

		record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{}`), nil)
		if err == nil {
			t.Errorf("expected SSRF error for %s, got nil", u)
		}
		if !errors.Is(err, webhook.ErrBlockedAddress) && err != nil && record == nil {
			t.Errorf("expected error wrapping ErrBlockedAddress for %s, got %v", u, err)
		}
		if record != nil && record.Success {
			t.Errorf("expected record.Success = false for %s", u)
		}
	}
}

func TestDispatcher_SSRF_DialerControl(t *testing.T) {
	t.Parallel()

	// Direct test of dialer control validation logic
	control := webhook.SSRFSafeDialerControl(false)

	// Blocked socket addresses
	blockedAddresses := []string{
		"127.0.0.1:80",
		"10.0.0.1:443",
		"169.254.169.254:80",
		"192.168.0.1:8080",
		"[::1]:80",
		"[fe80::1]:80",
	}

	for _, addr := range blockedAddresses {
		err := control("tcp", addr, &fakeRawConn{})
		if err == nil {
			t.Errorf("expected control to block %s, got nil", addr)
		}
		if !errors.Is(err, webhook.ErrBlockedAddress) {
			t.Errorf("expected ErrBlockedAddress for %s, got %v", addr, err)
		}
	}

	// Allowed public addresses
	allowedAddresses := []string{
		"8.8.8.8:53",
		"93.184.216.34:443",
		"1.1.1.1:80",
	}

	for _, addr := range allowedAddresses {
		err := control("tcp", addr, &fakeRawConn{})
		if err != nil {
			t.Errorf("expected control to allow %s, got %v", addr, err)
		}
	}
}

type fakeRawConn struct{}

func (f *fakeRawConn) Control(func(fd uintptr)) error {
	return nil
}
func (f *fakeRawConn) Read(func(fd uintptr) (done bool)) error {
	return nil
}
func (f *fakeRawConn) Write(func(fd uintptr) (done bool)) error {
	return nil
}

func TestDispatcher_GeospatialFilter(t *testing.T) {
	t.Parallel()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := webhook.NewDispatcher(nil, webhook.WithAllowLocalhost(true))

	// Geofence centered in Downtown Seattle with 5 mile radius
	sub := &webhook.WebhookSubscription{
		ID:        "sub-geo-1",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    true,
		GeoFence: &webhook.GeoFence{
			CenterLat:   47.6062,
			CenterLng:   -122.3321,
			RadiusMiles: 5.0,
		},
	}

	// 1. Inside fence: Capitol Hill (~1.2 miles away)
	insideCoords := &domain.LocationPoint{
		Latitude:  47.6150,
		Longitude: -122.3200,
	}
	rec1, err1 := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p1"}`), insideCoords)
	if err1 != nil {
		t.Fatalf("unexpected error for inside coords: %v", err1)
	}
	if rec1 == nil || !rec1.Success {
		t.Errorf("expected successful delivery for coords inside fence, got rec: %+v", rec1)
	}
	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("requestCount = %d, want 1", count)
	}

	// 2. Outside fence: Tacoma (~25 miles away)
	outsideCoords := &domain.LocationPoint{
		Latitude:  47.2529,
		Longitude: -122.4443,
	}
	rec2, err2 := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p2"}`), outsideCoords)
	if err2 != nil {
		t.Fatalf("unexpected error for outside coords: %v", err2)
	}
	if rec2 != nil {
		t.Errorf("expected delivery to be skipped (nil record) for outside coords, got %+v", rec2)
	}
	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("requestCount = %d, want 1 (should not have incremented)", count)
	}

	// 3. Nil coords with geofence set -> skipped
	rec3, err3 := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p3"}`), nil)
	if err3 != nil {
		t.Fatalf("unexpected error for nil coords: %v", err3)
	}
	if rec3 != nil {
		t.Errorf("expected delivery to be skipped (nil record) for nil coords, got %+v", rec3)
	}
	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("requestCount = %d, want 1", count)
	}

	// 4. Sub with nil GeoFence -> delivered even if outside Seattle
	subNoFence := &webhook.WebhookSubscription{
		ID:        "sub-no-fence",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    true,
		GeoFence:  nil,
	}
	rec4, err4 := d.Deliver(context.Background(), subNoFence, "pet_lost", []byte(`{"id":"p4"}`), outsideCoords)
	if err4 != nil {
		t.Fatalf("unexpected error for sub without geofence: %v", err4)
	}
	if rec4 == nil || !rec4.Success {
		t.Errorf("expected successful delivery for sub without geofence, got %+v", rec4)
	}
	if count := atomic.LoadInt32(&requestCount); count != 2 {
		t.Errorf("requestCount = %d, want 2", count)
	}
}

func TestDispatcher_RetryBackoff_500(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	st := store.NewMemoryStore()
	// 3 retries with 1ms intervals for instant test execution
	d := webhook.NewDispatcher(
		st,
		webhook.WithAllowLocalhost(true),
		webhook.WithRetryDelays(time.Millisecond, time.Millisecond, time.Millisecond),
	)

	sub := &webhook.WebhookSubscription{
		ID:        "sub-retry-500",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    true,
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p-retry"}`), nil)
	if err == nil {
		t.Fatal("expected error on 500 failure after retries, got nil")
	}
	if record == nil {
		t.Fatal("expected delivery record to be returned on failure")
	}

	// Initial attempt (1) + 3 retries = 4 attempts total
	if count := atomic.LoadInt32(&attempts); count != 4 {
		t.Errorf("server received %d attempts, want 4", count)
	}
	if record.Attempt != 4 {
		t.Errorf("record.Attempt = %d, want 4", record.Attempt)
	}
	if record.Success {
		t.Errorf("record.Success = true, want false")
	}
	if record.StatusCode != http.StatusInternalServerError {
		t.Errorf("record.StatusCode = %d, want 500", record.StatusCode)
	}
	if record.Error == "" {
		t.Error("record.Error is empty, want error message")
	}

	// Verify persistence of failure record
	savedBytes, err := st.GetState(context.Background(), store.WebhookDeliveriesCollection, record.ID)
	if err != nil {
		t.Fatalf("failed to get state from store: %v", err)
	}
	var saved webhook.WebhookDeliveryRecord
	if err := json.Unmarshal(savedBytes, &saved); err != nil {
		t.Fatalf("failed to unmarshal saved record: %v", err)
	}
	if saved.Attempt != 4 || saved.Success != false || saved.StatusCode != 500 {
		t.Errorf("saved record mismatch: %+v", saved)
	}
}

func TestDispatcher_RetryBackoff_NetworkError(t *testing.T) {
	t.Parallel()

	var attempts int32
	mockTransport := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&attempts, 1)
			return nil, errors.New("network connection reset by peer")
		},
	}

	client := &http.Client{Transport: mockTransport}
	d := webhook.NewDispatcher(
		nil,
		webhook.WithHTTPClient(client),
		webhook.WithRetryDelays(time.Millisecond, time.Millisecond, time.Millisecond),
	)

	sub := &webhook.WebhookSubscription{
		ID:        "sub-net-err",
		TargetURL: "https://api.example.com/webhook",
		Secret:    "secret",
		Active:    true,
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p-net"}`), nil)
	if err == nil {
		t.Fatal("expected error on network failure, got nil")
	}
	if record == nil {
		t.Fatal("expected delivery record on failure")
	}
	if count := atomic.LoadInt32(&attempts); count != 4 {
		t.Errorf("attempts = %d, want 4", count)
	}
	if record.Attempt != 4 {
		t.Errorf("record.Attempt = %d, want 4", record.Attempt)
	}
	if record.Success {
		t.Errorf("record.Success = true, want false")
	}
	if record.StatusCode != 0 {
		t.Errorf("record.StatusCode = %d, want 0", record.StatusCode)
	}
}

func TestDispatcher_NoRetry_ClientError400(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	d := webhook.NewDispatcher(
		nil,
		webhook.WithAllowLocalhost(true),
		webhook.WithRetryDelays(time.Millisecond, time.Millisecond, time.Millisecond),
	)

	sub := &webhook.WebhookSubscription{
		ID:        "sub-client-err",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    true,
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p-400"}`), nil)
	if err == nil {
		t.Fatal("expected error on 400 Bad Request, got nil")
	}
	if record == nil {
		t.Fatal("expected delivery record")
	}
	// Should NOT retry on 4xx
	if count := atomic.LoadInt32(&attempts); count != 1 {
		t.Errorf("attempts = %d, want 1 (client error must not be retried)", count)
	}
	if record.Attempt != 1 {
		t.Errorf("record.Attempt = %d, want 1", record.Attempt)
	}
	if record.StatusCode != http.StatusBadRequest {
		t.Errorf("record.StatusCode = %d, want 400", record.StatusCode)
	}
}

func TestDispatcher_RetrySuccess_OnSecondAttempt(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current == 1 {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	d := webhook.NewDispatcher(
		nil,
		webhook.WithAllowLocalhost(true),
		webhook.WithRetryDelays(time.Millisecond, time.Millisecond, time.Millisecond),
	)

	sub := &webhook.WebhookSubscription{
		ID:        "sub-retry-success",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    true,
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{"id":"p-ok-2"}`), nil)
	if err != nil {
		t.Fatalf("unexpected Deliver error: %v", err)
	}
	if record == nil {
		t.Fatal("expected non-nil record")
	}
	if count := atomic.LoadInt32(&attempts); count != 2 {
		t.Errorf("attempts = %d, want 2", count)
	}
	if record.Attempt != 2 {
		t.Errorf("record.Attempt = %d, want 2", record.Attempt)
	}
	if !record.Success {
		t.Errorf("record.Success = false, want true")
	}
	if record.StatusCode != http.StatusOK {
		t.Errorf("record.StatusCode = %d, want 200", record.StatusCode)
	}
}

func TestDispatcher_InactiveSubscription(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
	}))
	defer server.Close()

	d := webhook.NewDispatcher(nil, webhook.WithAllowLocalhost(true))

	sub := &webhook.WebhookSubscription{
		ID:        "sub-inactive",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    false, // Inactive
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record != nil {
		t.Errorf("expected nil record for inactive subscription, got %+v", record)
	}
	if count := atomic.LoadInt32(&attempts); count != 0 {
		t.Errorf("attempts = %d, want 0", count)
	}
}

func TestDispatcher_FilterEvents(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := webhook.NewDispatcher(nil, webhook.WithAllowLocalhost(true))

	sub := &webhook.WebhookSubscription{
		ID:           "sub-filtered",
		TargetURL:    server.URL,
		Secret:       "secret",
		FilterEvents: []string{"pet_lost", "pet_found"},
		Active:       true,
	}

	// Event not in filter -> skipped
	rec1, err1 := d.Deliver(context.Background(), sub, "sighting_reported", []byte(`{}`), nil)
	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}
	if rec1 != nil {
		t.Errorf("expected nil record for non-matching event, got %+v", rec1)
	}
	if count := atomic.LoadInt32(&attempts); count != 0 {
		t.Errorf("attempts = %d, want 0", count)
	}

	// Event in filter -> delivered
	rec2, err2 := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{}`), nil)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if rec2 == nil || !rec2.Success {
		t.Errorf("expected delivery for matching event, got %+v", rec2)
	}
	if count := atomic.LoadInt32(&attempts); count != 1 {
		t.Errorf("attempts = %d, want 1", count)
	}
}

func TestDispatcher_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	// Delays longer than cancel timeout
	d := webhook.NewDispatcher(
		nil,
		webhook.WithAllowLocalhost(true),
		webhook.WithRetryDelays(500*time.Millisecond, 500*time.Millisecond),
	)

	sub := &webhook.WebhookSubscription{
		ID:        "sub-cancel",
		TargetURL: server.URL,
		Secret:    "secret",
		Active:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := d.Deliver(ctx, sub, "pet_lost", []byte(`{}`), nil)
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context cancellation error, got %v", err)
	}
}

type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestDispatcher_DispatchEvent(t *testing.T) {
	t.Parallel()

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewMemoryStore()
	d := webhook.NewDispatcher(st, webhook.WithAllowLocalhost(true))

	// Save two subscriptions: one active, one inactive
	sub1 := webhook.WebhookSubscription{
		ID:           "sub-active-1",
		PartnerID:    "p1",
		TargetURL:    server.URL + "/sub1",
		Secret:       "sec1",
		FilterEvents: []string{"pet_lost"},
		Active:       true,
		CreatedAt:    time.Now().UTC(),
	}
	sub2 := webhook.WebhookSubscription{
		ID:           "sub-inactive-2",
		PartnerID:    "p2",
		TargetURL:    server.URL + "/sub2",
		Secret:       "sec2",
		FilterEvents: []string{"pet_lost"},
		Active:       false,
		CreatedAt:    time.Now().UTC(),
	}
	sub3 := webhook.WebhookSubscription{
		ID:           "sub-other-event-3",
		PartnerID:    "p3",
		TargetURL:    server.URL + "/sub3",
		Secret:       "sec3",
		FilterEvents: []string{"sighting_reported"},
		Active:       true,
		CreatedAt:    time.Now().UTC(),
	}

	data1, _ := json.Marshal(sub1)
	data2, _ := json.Marshal(sub2)
	data3, _ := json.Marshal(sub3)

	_ = st.SaveState(context.Background(), store.WebhooksCollection, sub1.ID, data1)
	_ = st.SaveState(context.Background(), store.WebhooksCollection, sub2.ID, data2)
	_ = st.SaveState(context.Background(), store.WebhooksCollection, sub3.ID, data3)

	records, err := d.DispatchEvent(context.Background(), "pet_lost", []byte(`{"id":"lost-10"}`), nil)
	if err != nil {
		t.Fatalf("DispatchEvent failed: %v", err)
	}

	// Only sub1 should have matched and been delivered
	if len(records) != 1 {
		t.Fatalf("expected 1 record returned, got %d", len(records))
	}
	if records[0].SubscriptionID != sub1.ID {
		t.Errorf("SubscriptionID = %q, want %q", records[0].SubscriptionID, sub1.ID)
	}
	if !records[0].Success {
		t.Errorf("record.Success = false, want true")
	}
	if count := atomic.LoadInt32(&hits); count != 1 {
		t.Errorf("server received %d hits, want 1", count)
	}
}

func TestDispatcher_FilterEvents_EmptyEventTypeNotBypassed(t *testing.T) {
	t.Parallel()

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewMemoryStore()
	d := webhook.NewDispatcher(st, webhook.WithAllowLocalhost(true))

	sub := &webhook.WebhookSubscription{
		ID:           "sub-filtered",
		PartnerID:    "p1",
		TargetURL:    server.URL + "/webhook",
		FilterEvents: []string{"pet_lost"},
		Active:       true,
	}

	// Delivering with empty eventType should NOT bypass filter
	rec, err := d.Deliver(context.Background(), sub, "", []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec != nil {
		t.Errorf("expected delivery to be skipped, got non-nil record: %+v", rec)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Errorf("server received %d hits, expected 0", atomic.LoadInt32(&hits))
	}
}

func TestDispatcher_DispatchEvent_Concurrent(t *testing.T) {
	t.Parallel()

	// Simulate slow partner server
	delay := 100 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	st := store.NewMemoryStore()
	d := webhook.NewDispatcher(st, webhook.WithAllowLocalhost(true))

	// Register 3 subscriptions to the same endpoint
	for i := 1; i <= 3; i++ {
		sub := webhook.WebhookSubscription{
			ID:           fmt.Sprintf("sub-conc-%d", i),
			PartnerID:    fmt.Sprintf("partner-%d", i),
			TargetURL:    server.URL,
			FilterEvents: []string{"pet_lost"},
			Active:       true,
			CreatedAt:    time.Now().UTC(),
		}
		data, _ := json.Marshal(sub)
		_ = st.SaveState(context.Background(), store.WebhooksCollection, sub.ID, data)
	}

	start := time.Now()
	records, err := d.DispatchEvent(context.Background(), "pet_lost", []byte(`{"id":"lost-conc"}`), nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DispatchEvent failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// If concurrent, total time should be close to 100ms, well below 300ms sequential time
	if elapsed >= 250*time.Millisecond {
		t.Errorf("DispatchEvent took %v, expected concurrent execution < 250ms", elapsed)
	}
}

func TestDispatcher_RedirectNotFollowed(t *testing.T) {
	t.Parallel()

	var redirectHits int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&redirectHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer server.Close()

	st := store.NewMemoryStore()
	d := webhook.NewDispatcher(st, webhook.WithAllowLocalhost(true))

	sub := &webhook.WebhookSubscription{
		ID:        "sub-redirect",
		PartnerID: "partner-redir",
		TargetURL: server.URL,
		Active:    true,
	}

	record, err := d.Deliver(context.Background(), sub, "pet_lost", []byte(`{}`), nil)
	// Delivery to a 302 endpoint returns client error status (non-2xx) and does not follow redirect
	if record == nil {
		t.Fatal("expected non-nil delivery record")
	}
	if record.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, record.StatusCode)
	}
	if atomic.LoadInt32(&redirectHits) != 0 {
		t.Errorf("redirect target was visited %d times; redirect was unexpectedly followed", atomic.LoadInt32(&redirectHits))
	}
	if err == nil {
		t.Error("expected error for non-2xx status code")
	}
}
