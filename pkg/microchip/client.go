package microchip

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RegistryLookupResult represents the result of a clearinghouse or registry lookup.
type RegistryLookupResult struct {
	MicrochipID    string       `json:"microchipId"`
	Status         string       `json:"status"` // e.g. "registered", "unregistered", "not_found"
	Registry       RegistryInfo `json:"registry"`
	LastUpdated    time.Time    `json:"lastUpdated"`
	LookupProvider string       `json:"lookupProvider"`
}

// RegistryLookupClient defines the contract for looking up microchip registration status.
type RegistryLookupClient interface {
	Lookup(ctx context.Context, microchipID string) (*RegistryLookupResult, error)
}

// MockRegistryLookupClient is an in-memory mock client implementing RegistryLookupClient.
type MockRegistryLookupClient struct {
	mu        sync.RWMutex
	responses map[string]*RegistryLookupResult
}

// NewMockRegistryLookupClient returns a new simulated registry lookup client.
func NewMockRegistryLookupClient() RegistryLookupClient {
	return &MockRegistryLookupClient{
		responses: make(map[string]*RegistryLookupResult),
	}
}

// SetResponse registers or overrides a mock response for a specific microchip ID.
func (m *MockRegistryLookupClient) SetResponse(microchipID string, result *RegistryLookupResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val := ValidateAndNormalize(microchipID)
	key := microchipID
	if val.Valid {
		key = val.NormalizedID
	}
	m.responses[key] = result
}

// Lookup queries the simulated registry clearinghouse for the provided microchip.
func (m *MockRegistryLookupClient) Lookup(ctx context.Context, microchipID string) (*RegistryLookupResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	val := ValidateAndNormalize(microchipID)
	if !val.Valid {
		return nil, fmt.Errorf("microchip: invalid transponder format: %s", val.ErrorMessage)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if custom, ok := m.responses[val.NormalizedID]; ok {
		return custom, nil
	}

	reg := IdentifyIssuingRegistry(val.NormalizedID)
	return &RegistryLookupResult{
		MicrochipID:    val.NormalizedID,
		Status:         "registered",
		Registry:       reg,
		LastUpdated:    time.Now().UTC(),
		LookupProvider: "AAHA Universal Pet Microchip Lookup (Simulated)",
	}, nil
}
