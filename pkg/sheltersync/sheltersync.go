// Package sheltersync provides data models, feed adapters, and polling workers
// for municipal animal shelter intake feeds.
package sheltersync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ShelterImageData represents an image attachment from a shelter intake record.
type ShelterImageData struct {
	URL  string `json:"url"`
	View string `json:"view,omitempty"`
}

// ShelterLocationData represents the intake or found location associated with a shelter intake.
type ShelterLocationData struct {
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

// ShelterAnimalData describes the animal associated with a shelter intake.
type ShelterAnimalData struct {
	Species        string             `json:"species"`
	Breed          string             `json:"breed,omitempty"`
	PrimaryColor   string             `json:"primaryColor,omitempty"`
	SecondaryColor string             `json:"secondaryColor,omitempty"`
	Gender         string             `json:"gender,omitempty"`
	Description    string             `json:"description,omitempty"`
	MicrochipID    string             `json:"microchipId,omitempty"`
	Images         []ShelterImageData `json:"images,omitempty"`
}

// ShelterIntakeRequest encapsulates an inbound shelter intake record.
type ShelterIntakeRequest struct {
	ShelterID      string              `json:"shelterId"`
	ShelterName    string              `json:"shelterName"`
	ShelterAddress string              `json:"shelterAddress,omitempty"`
	ShelterPhone   string              `json:"shelterPhone,omitempty"`
	ShelterEmail   string              `json:"shelterEmail,omitempty"`
	IntakeID       string              `json:"intakeId"`
	IntakeDate     time.Time           `json:"intakeDate,omitempty"`
	Animal         ShelterAnimalData   `json:"animal"`
	Location       ShelterLocationData `json:"location"`
}

// ShelterFeedAdapter fetches intake records from an upstream shelter management system or feed.
type ShelterFeedAdapter interface {
	FetchIntakes(ctx context.Context) ([]ShelterIntakeRequest, error)
}

// Ingester accepts and processes a shelter intake record.
type Ingester interface {
	IngestIntake(ctx context.Context, req ShelterIntakeRequest) error
}

// IngestFunc is an adapter allowing a bare function to satisfy the Ingester interface.
type IngestFunc func(ctx context.Context, req ShelterIntakeRequest) error

// IngestIntake calls f(ctx, req).
func (f IngestFunc) IngestIntake(ctx context.Context, req ShelterIntakeRequest) error {
	return f(ctx, req)
}

// HTTPIngester submits shelter intake records to a remote PetSpotR ingest endpoint over HTTP.
type HTTPIngester struct {
	Endpoint   string
	HTTPClient *http.Client
	APIKey     string
}

// NewHTTPIngester constructs an HTTPIngester targeting the given endpoint.
func NewHTTPIngester(endpoint string, client *http.Client) *HTTPIngester {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPIngester{
		Endpoint:   endpoint,
		HTTPClient: client,
	}
}

// IngestIntake sends req as JSON via POST to the configured endpoint.
func (h *HTTPIngester) IngestIntake(ctx context.Context, req ShelterIntakeRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("sheltersync: marshal intake request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sheltersync: create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if h.APIKey != "" {
		httpReq.Header.Set("X-Shelter-API-Key", h.APIKey)
	}

	resp, err := h.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sheltersync: send intake request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("sheltersync: ingest failed with HTTP status %d", resp.StatusCode)
	}
	return nil
}

// MockShelterFeedAdapter provides seeded, realistic shelter intake records for deterministic testing.
type MockShelterFeedAdapter struct {
	Intakes []ShelterIntakeRequest
	Err     error
}

// SeededMockIntakes returns realistic test records for Seattle, Bellevue, and King County animal shelters.
func SeededMockIntakes() []ShelterIntakeRequest {
	baseTime := time.Date(2026, time.September, 19, 8, 0, 0, 0, time.UTC)
	return []ShelterIntakeRequest{
		{
			ShelterID:      "shelter-sea-01",
			ShelterName:    "Seattle Animal Shelter",
			ShelterAddress: "2061 15th Ave W, Seattle, WA 98119",
			ShelterPhone:   "(206) 386-7387",
			ShelterEmail:   "shelter@seattle.gov",
			IntakeID:       "INT-2026-8819",
			IntakeDate:     baseTime,
			Animal: ShelterAnimalData{
				Species:        "dog",
				Breed:          "Golden Retriever",
				PrimaryColor:   "Golden",
				Gender:         "male",
				Description:    "Found stray near Interbay. Friendly, scanned microchip on intake.",
				MicrochipID:    "985141000123456",
				Images: []ShelterImageData{
					{
						URL:  "https://storage.petspotr.io/shelters/intake-8819.jpg",
						View: "primary",
					},
				},
			},
			Location: ShelterLocationData{
				Address:   "Interbay, Seattle, WA",
				Latitude:  47.648,
				Longitude: -122.378,
			},
		},
		{
			ShelterID:      "shelter-bel-02",
			ShelterName:    "Bellevue Humane Society",
			ShelterAddress: "13212 SE Eastgate Way, Bellevue, WA 98005",
			ShelterPhone:   "(425) 641-0080",
			ShelterEmail:   "intake@bellevuehumane.org",
			IntakeID:       "INT-2026-9901",
			IntakeDate:     baseTime.Add(1 * time.Hour),
			Animal: ShelterAnimalData{
				Species:        "cat",
				Breed:          "Domestic Shorthair",
				PrimaryColor:   "Black",
				Gender:         "female",
				Description:    "Found near Crossroads Park. Friendly adult cat.",
				MicrochipID:    "981010000999999",
				Images: []ShelterImageData{
					{
						URL:  "https://storage.petspotr.io/shelters/intake-9901.jpg",
						View: "primary",
					},
				},
			},
			Location: ShelterLocationData{
				Address:   "Crossroads, Bellevue, WA",
				Latitude:  47.620,
				Longitude: -122.130,
			},
		},
		{
			ShelterID:      "shelter-kcras-03",
			ShelterName:    "King County Regional Animal Services",
			ShelterAddress: "21615 64th Ave S, Kent, WA 98032",
			ShelterPhone:   "(206) 296-7387",
			ShelterEmail:   "pets@kingcounty.gov",
			IntakeID:       "INT-2026-7732",
			IntakeDate:     baseTime.Add(2 * time.Hour),
			Animal: ShelterAnimalData{
				Species:        "dog",
				Breed:          "German Shepherd",
				PrimaryColor:   "Black",
				SecondaryColor: "Tan",
				Gender:         "male",
				Description:    "Found near Kent Station.",
				MicrochipID:    "977110000555123",
				Images: []ShelterImageData{
					{
						URL:  "https://storage.petspotr.io/shelters/intake-7732.jpg",
						View: "primary",
					},
				},
			},
			Location: ShelterLocationData{
				Address:   "Kent Station, Kent, WA",
				Latitude:  47.380,
				Longitude: -122.230,
			},
		},
	}
}

// NewMockShelterFeedAdapter constructs a MockShelterFeedAdapter. If no intakes are passed,
// SeededMockIntakes() is used by default.
func NewMockShelterFeedAdapter(intakes ...ShelterIntakeRequest) *MockShelterFeedAdapter {
	if len(intakes) == 0 {
		return &MockShelterFeedAdapter{
			Intakes: SeededMockIntakes(),
		}
	}
	return &MockShelterFeedAdapter{
		Intakes: intakes,
	}
}

// FetchIntakes returns configured intake records or Err if set.
func (m *MockShelterFeedAdapter) FetchIntakes(ctx context.Context) ([]ShelterIntakeRequest, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Intakes, nil
}

// SyncWorker periodically fetches shelter intake records from an adapter and dispatches them to an ingester.
type SyncWorker struct {
	adapter  ShelterFeedAdapter
	ingester Ingester
}

// SyncEngine is an alias for SyncWorker.
type SyncEngine = SyncWorker

// ScheduledSyncWorker is an alias for SyncWorker.
type ScheduledSyncWorker = SyncWorker

// NewSyncWorker constructs a SyncWorker with the given adapter and ingester.
func NewSyncWorker(adapter ShelterFeedAdapter, ingester Ingester) *SyncWorker {
	return &SyncWorker{
		adapter:  adapter,
		ingester: ingester,
	}
}

// SyncOnce fetches all available intake records from the adapter and forwards them to the ingester.
// Returns the count of successfully ingested records and any error encountered.
func (w *SyncWorker) SyncOnce(ctx context.Context) (int, error) {
	intakes, err := w.adapter.FetchIntakes(ctx)
	if err != nil {
		return 0, fmt.Errorf("sheltersync: fetch intakes: %w", err)
	}

	ingested := 0
	for _, req := range intakes {
		if err := w.ingester.IngestIntake(ctx, req); err != nil {
			return ingested, fmt.Errorf("sheltersync: ingest intake %s: %w", req.IntakeID, err)
		}
		ingested++
	}
	return ingested, nil
}

// Start initiates periodic polling at the specified interval until the context is canceled.
func (w *SyncWorker) Start(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial execution
	if _, err := w.SyncOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		// Log or retain error, do not abort worker loop
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.SyncOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				// Continue polling on transient errors
			}
		}
	}
}
