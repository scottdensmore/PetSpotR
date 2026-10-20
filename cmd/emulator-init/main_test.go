package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("PUBSUB_EMULATOR_HOST", "pubsub:8086")
	t.Setenv("FIRESTORE_EMULATOR_HOST", "firestore:8085")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")

	cfg := loadConfigFromEnv()
	if cfg.pubsubHost != "pubsub:8086" {
		t.Fatalf("expected pubsub:8086, got %s", cfg.pubsubHost)
	}
	if cfg.firestoreHost != "firestore:8085" {
		t.Fatalf("expected firestore:8085, got %s", cfg.firestoreHost)
	}
	if cfg.projectID != "test-project" {
		t.Fatalf("expected test-project, got %s", cfg.projectID)
	}
}

func TestRunSuccessAndIdempotentConflict(t *testing.T) {
	var createdTopics []string
	var createdSubs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/topics/"):
			createdTopics = append(createdTopics, r.URL.Path)
			if strings.HasSuffix(r.URL.Path, "-dlq") {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/subscriptions/"):
			createdSubs = append(createdSubs, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	cfg := config{
		pubsubHost:    host,
		firestoreHost: host,
		projectID:     "petspotr-local",
		maxRetries:    2,
		retryInterval: time.Millisecond,
	}

	var out bytes.Buffer
	if err := run(context.Background(), cfg, srv.Client(), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(createdTopics) != len(canonicalTopics) {
		t.Fatalf("expected %d topics, got %d", len(canonicalTopics), len(createdTopics))
	}
	if len(createdSubs) != len(canonicalSubscriptions) {
		t.Fatalf("expected %d subscriptions, got %d", len(canonicalSubscriptions), len(createdSubs))
	}
	if !strings.Contains(out.String(), "Emulator initialization completed successfully.") {
		t.Fatalf("expected completion message in output, got: %s", out.String())
	}
}

func TestRunWaitTimeoutAndTopicError(t *testing.T) {
	cfg := config{
		pubsubHost:    "127.0.0.1:1",
		firestoreHost: "127.0.0.1:1",
		projectID:     "petspotr-local",
		maxRetries:    2,
		retryInterval: time.Millisecond,
	}
	var out bytes.Buffer
	if err := run(context.Background(), cfg, &http.Client{Timeout: 10 * time.Millisecond}, &out); err == nil {
		t.Fatal("expected timeout error when emulator is unreachable")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	cfg.pubsubHost = host
	cfg.firestoreHost = host
	if err := run(context.Background(), cfg, srv.Client(), &out); err == nil || !strings.Contains(err.Error(), "failed to create topic") {
		t.Fatalf("expected topic creation error, got: %v", err)
	}
}
