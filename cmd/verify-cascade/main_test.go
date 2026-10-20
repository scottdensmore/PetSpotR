package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("LOSTPET_URL", "http://lostpet:8080")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "cascade-test")

	cfg := loadConfigFromEnv()
	if cfg.lostPetURL != "http://lostpet:8080" {
		t.Fatalf("expected http://lostpet:8080, got %s", cfg.lostPetURL)
	}
	if cfg.projectID != "cascade-test" {
		t.Fatalf("expected cascade-test, got %s", cfg.projectID)
	}
}

func TestRunCascadeSuccess(t *testing.T) {
	var lastLostID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/healthz"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		case r.Method == http.MethodPost && r.URL.Path == "/lostPet":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if id, ok := body["petId"].(string); ok {
				lastLostID = id
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"created"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/foundPet":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"created"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/documents/lost_pets/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"lost_pet"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/documents/found_pets/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"found_pet"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/matches":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"lostPetId":"` + lastLostID + `"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	cfg := cascadeConfig{
		lostPetURL:      srv.URL,
		foundPetURL:     srv.URL,
		webURL:          srv.URL,
		matcherURL:      srv.URL,
		notificationURL: srv.URL,
		firestoreHost:   host,
		pubsubHost:      host,
		projectID:       "petspotr-local",
		healthRetries:   2,
		cascadeRetries:  2,
		retryInterval:   time.Millisecond,
	}

	var out bytes.Buffer
	if err := runCascade(context.Background(), cfg, srv.Client(), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Cascade Verification Completed Successfully!") {
		t.Fatalf("expected completion banner in output, got: %s", out.String())
	}
}

func TestRunCascadeFallbackAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/healthz"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/documents/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/matches":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	cfg := cascadeConfig{
		lostPetURL:      srv.URL,
		foundPetURL:     srv.URL,
		webURL:          srv.URL,
		matcherURL:      srv.URL,
		notificationURL: srv.URL,
		firestoreHost:   host,
		pubsubHost:      host,
		projectID:       "petspotr-local",
		healthRetries:   2,
		cascadeRetries:  1,
		retryInterval:   time.Millisecond,
	}

	var out bytes.Buffer
	if err := runCascade(context.Background(), cfg, srv.Client(), &out); err != nil {
		t.Fatalf("unexpected error on fallback path: %v", err)
	}
	if !strings.Contains(out.String(), "NOTE: AI Matcher requires Ollama model inference") {
		t.Fatalf("expected fallback note, got: %s", out.String())
	}
}
