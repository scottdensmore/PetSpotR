// Command emulator-init initializes local Pub/Sub and Firestore emulators with
// canonical topics and push subscriptions for PetSpotR microservices, then seeds
// demo data into Firestore unless SEED_DEMO_DATA is false.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
)

type config struct {
	pubsubHost    string
	firestoreHost string
	projectID     string
	maxRetries    int
	retryInterval time.Duration
	seedDemoData  bool
}

type subscriptionSpec struct {
	name     string
	topic    string
	endpoint string
}

var canonicalTopics = []string{
	"lostPet",
	"foundPet",
	"matchFound",
	"petStatusChanged",
	"lostPet-dlq",
	"foundPet-dlq",
	"matchFound-dlq",
	"petStatusChanged-dlq",
}

var canonicalSubscriptions = []subscriptionSpec{
	{
		name:     "lost-pet-matcher-analysis",
		topic:    "lostPet",
		endpoint: "http://pet-matcher:8083/events/lost-pets",
	},
	{
		name:     "found-pet-matcher",
		topic:    "foundPet",
		endpoint: "http://pet-matcher:8083/events/found-pets",
	},
	{
		name:     "match-found-notification-backlog",
		topic:    "matchFound",
		endpoint: "http://notification-service:8084/events/matches",
	},
	{
		name:     "lost-pet-notification",
		topic:    "lostPet",
		endpoint: "http://notification-service:8084/events/lost-pets",
	},
}

func loadConfigFromEnv() (config, error) {
	seedDemoData, err := strconv.ParseBool(envOrDefault("SEED_DEMO_DATA", "true"))
	if err != nil {
		return config{}, fmt.Errorf("invalid SEED_DEMO_DATA: %w", err)
	}
	return config{
		pubsubHost:    envOrDefault("PUBSUB_EMULATOR_HOST", "localhost:8086"),
		firestoreHost: envOrDefault("FIRESTORE_EMULATOR_HOST", "localhost:8085"),
		projectID:     envOrDefault("GOOGLE_CLOUD_PROJECT", "petspotr-local"),
		maxRetries:    30,
		retryInterval: time.Second,
		seedDemoData:  seedDemoData,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func waitForEmulator(ctx context.Context, client *http.Client, name, host string, maxRetries int, interval time.Duration, out io.Writer) error {
	url := fmt.Sprintf("http://%s/", host)
	_, _ = fmt.Fprintf(out, "==> Waiting for %s emulator to accept connections at %s...\n", name, url)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, reqErr := client.Do(req)
			if reqErr == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					_, _ = fmt.Fprintf(out, "    %s emulator is ready.\n", name)
					return nil
				}
			}
		}
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
			}
		}
	}
	return fmt.Errorf("timed out waiting for %s emulator at %s", name, host)
}

func run(ctx context.Context, cfg config, client *http.Client, out io.Writer) error {
	_, _ = fmt.Fprintf(out, "==> Initializing PetSpotR emulators for project '%s'...\n", cfg.projectID)
	_, _ = fmt.Fprintf(out, "    Pub/Sub emulator:   %s\n", cfg.pubsubHost)
	_, _ = fmt.Fprintf(out, "    Firestore emulator: %s\n", cfg.firestoreHost)

	if err := waitForEmulator(ctx, client, "Pub/Sub", cfg.pubsubHost, cfg.maxRetries, cfg.retryInterval, out); err != nil {
		return err
	}
	if err := waitForEmulator(ctx, client, "Firestore", cfg.firestoreHost, cfg.maxRetries, cfg.retryInterval, out); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(out, "==> Creating canonical Pub/Sub topics...")
	for _, topic := range canonicalTopics {
		topicPath := fmt.Sprintf("projects/%s/topics/%s", cfg.projectID, topic)
		url := fmt.Sprintf("http://%s/v1/%s", cfg.pubsubHost, topicPath)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
		if err != nil {
			return fmt.Errorf("create request for topic %s: %w", topicPath, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("create topic %s: %w", topicPath, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("failed to create topic %s (status: %d): %s", topicPath, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		_, _ = fmt.Fprintf(out, "    Topic %s initialized (status: %d).\n", topicPath, resp.StatusCode)
	}

	_, _ = fmt.Fprintln(out, "==> Creating canonical Pub/Sub push subscriptions...")
	for _, sub := range canonicalSubscriptions {
		subPath := fmt.Sprintf("projects/%s/subscriptions/%s", cfg.projectID, sub.name)
		topicPath := fmt.Sprintf("projects/%s/topics/%s", cfg.projectID, sub.topic)
		payload, err := json.Marshal(map[string]any{
			"topic": topicPath,
			"pushConfig": map[string]string{
				"pushEndpoint": sub.endpoint,
			},
		})
		if err != nil {
			return fmt.Errorf("marshal subscription %s: %w", subPath, err)
		}

		url := fmt.Sprintf("http://%s/v1/%s", cfg.pubsubHost, subPath)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create request for subscription %s: %w", subPath, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("create subscription %s: %w", subPath, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("failed to create subscription %s (status: %d): %s", subPath, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		_, _ = fmt.Fprintf(out, "    Subscription %s -> %s initialized (status: %d).\n", subPath, sub.endpoint, resp.StatusCode)
	}

	_, _ = fmt.Fprintln(out, "==> Emulator initialization completed successfully.")
	return nil
}

func seedDemoData(ctx context.Context, cfg config, out io.Writer) error {
	_, _ = fmt.Fprintln(out, "==> Seeding demo data in Firestore emulator...")
	runtime, err := runtimeconfig.NewStateRuntime(ctx, runtimeconfig.StateConfig{
		Mode:                  runtimeconfig.ModeLocalEmulator,
		ProjectID:             cfg.projectID,
		FirestoreEmulatorHost: cfg.firestoreHost,
	})
	if err != nil {
		return fmt.Errorf("initialize state runtime: %w", err)
	}
	defer func() { _ = runtime.Close() }()

	if err := webfrontend.SeedDemoData(ctx, runtime.Store); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, "    Seeded demo matches and pets.")
	return nil
}

func main() {
	ctx := context.Background()
	cfg, err := loadConfigFromEnv()
	if err == nil {
		err = run(ctx, cfg, &http.Client{Timeout: 5 * time.Second}, os.Stdout)
	}
	if err == nil && cfg.seedDemoData {
		err = seedDemoData(ctx, cfg, os.Stdout)
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
