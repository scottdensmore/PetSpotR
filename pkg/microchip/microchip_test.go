package microchip_test

import (
	"context"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/microchip"
)

func TestValidateAndNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantValid      bool
		wantStandard   microchip.Standard
		wantNormalized string
	}{
		{
			name:           "Valid ISO 15-digit FDX-B",
			input:          " 985141000123456 ",
			wantValid:      true,
			wantStandard:   microchip.StandardISO15,
			wantNormalized: "985141000123456",
		},
		{
			name:           "Valid Avid 9-digit with asterisks",
			input:          "123*456*789",
			wantValid:      true,
			wantStandard:   microchip.StandardAvid9,
			wantNormalized: "123456789",
		},
		{
			name:           "Valid Euro 10-digit alphanumeric",
			input:          "00064a12b3",
			wantValid:      true,
			wantStandard:   microchip.StandardEuro10,
			wantNormalized: "00064A12B3",
		},
		{
			name:         "Invalid length",
			input:        "12345",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
		{
			name:         "Invalid characters in ISO",
			input:        "98514100012345X",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
		{
			name:         "Empty input",
			input:        "",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
		{
			name:         "Whitespace only",
			input:        "   ",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
		{
			name:           "Valid ISO with hyphens and spaces",
			input:          "985-141-000 123 456",
			wantValid:      true,
			wantStandard:   microchip.StandardISO15,
			wantNormalized: "985141000123456",
		},
		{
			name:         "Invalid characters in Avid length",
			input:        "123*456*78A",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
		{
			name:         "Invalid characters in Euro length",
			input:        "00064Z12B3",
			wantValid:    false,
			wantStandard: microchip.StandardUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := microchip.ValidateAndNormalize(tt.input)
			if res.Valid != tt.wantValid {
				t.Fatalf("expected valid=%v, got %v (err=%s)", tt.wantValid, res.Valid, res.ErrorMessage)
			}
			if res.Standard != tt.wantStandard {
				t.Errorf("expected standard=%v, got %v", tt.wantStandard, res.Standard)
			}
			if tt.wantValid && res.NormalizedID != tt.wantNormalized {
				t.Errorf("expected normalized=%q, got %q", tt.wantNormalized, res.NormalizedID)
			}
		})
	}
}

func TestIdentifyIssuingRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		chip         string
		wantRegistry string
		wantPhone    string
		wantWebsite  string
	}{
		{"985141000123456", "HomeAgain", "1-888-466-3242", "homeagain.com"},
		{"981010000123456", "AKC Reunite", "1-800-252-7894", "akcreunite.org"},
		{"977200000123456", "PetLink", "1-877-738-5465", "petlink.net"},
		{"982000000123456", "24Petwatch", "1-866-597-2424", "24petwatch.com"},
		{"965000000123456", "BuddyID", "1-800-434-2843", "buddyid.com"},
		{"123456789", "Avid Identification Systems", "1-800-336-2843", "avidid.com"},
		{"00064A12B3", "Euro / Trovan Microchip Registry", "", "trovan.com"},
		{"900123000123456", "Universal Clearinghouse", "", "aaha.org/petmicrochiplookup"},
		{"unknown-format", "Universal Clearinghouse", "", "aaha.org/petmicrochiplookup"},
	}

	for _, tt := range tests {
		t.Run(tt.chip, func(t *testing.T) {
			reg := microchip.IdentifyIssuingRegistry(tt.chip)
			if reg.RegistryName != tt.wantRegistry {
				t.Errorf("expected registry %q, got %q", tt.wantRegistry, reg.RegistryName)
			}
			if tt.wantPhone != "" && reg.Phone != tt.wantPhone {
				t.Errorf("expected phone %q, got %q", tt.wantPhone, reg.Phone)
			}
			if tt.wantWebsite != "" && reg.Website != tt.wantWebsite {
				t.Errorf("expected website %q, got %q", tt.wantWebsite, reg.Website)
			}
			if reg.Clearinghouse == "" {
				t.Errorf("expected clearinghouse to be populated")
			}
		})
	}
}

func TestMaskMicrochip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		chip     string
		wantMask string
	}{
		{"985141000123456", "HomeAgain ••••3456"},
		{"123456789", "Avid ••••6789"},
		{"00064A12B3", "Euro ••••12B3"},
		{"", ""},
		{"12345", ""},
		{"981010000123456", "AKC Reunite ••••3456"},
		{"977200000123456", "PetLink ••••3456"},
		{"982000000123456", "24Petwatch ••••3456"},
		{"965000000123456", "BuddyID ••••3456"},
		{"900123000123456", "ISO ••••3456"},
	}

	for _, tt := range tests {
		t.Run(tt.chip, func(t *testing.T) {
			got := microchip.MaskMicrochip(tt.chip)
			if got != tt.wantMask {
				t.Errorf("expected mask %q, got %q", tt.wantMask, got)
			}
		})
	}
}

func TestMockRegistryLookupClient(t *testing.T) {
	t.Parallel()

	client := microchip.NewMockRegistryLookupClient()

	t.Run("Valid HomeAgain lookup", func(t *testing.T) {
		res, err := client.Lookup(context.Background(), "985141000123456")
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}
		if res.Registry.RegistryName != "HomeAgain" {
			t.Errorf("expected HomeAgain, got %s", res.Registry.RegistryName)
		}
		if res.Status != "registered" {
			t.Errorf("expected registered status, got %s", res.Status)
		}
		if res.MicrochipID != "985141000123456" {
			t.Errorf("expected normalized ID %q, got %q", "985141000123456", res.MicrochipID)
		}
		if res.LookupProvider == "" {
			t.Errorf("expected non-empty lookup provider")
		}
		if res.LastUpdated.IsZero() {
			t.Errorf("expected populated LastUpdated timestamp")
		}
	})

	t.Run("Invalid microchip lookup error", func(t *testing.T) {
		_, err := client.Lookup(context.Background(), "invalid-chip")
		if err == nil {
			t.Fatalf("expected error for invalid chip, got nil")
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Lookup(ctx, "985141000123456")
		if err == nil {
			t.Fatalf("expected error on cancelled context, got nil")
		}
	})

	t.Run("Custom response override", func(t *testing.T) {
		mockClient, ok := client.(*microchip.MockRegistryLookupClient)
		if !ok {
			t.Fatalf("expected *microchip.MockRegistryLookupClient")
		}
		custom := &microchip.RegistryLookupResult{
			MicrochipID: "985141000123456",
			Status:      "unregistered",
			Registry: microchip.RegistryInfo{
				RegistryName:  "HomeAgain",
				Clearinghouse: "AAHA Clearinghouse",
			},
			LastUpdated:    time.Now().UTC(),
			LookupProvider: "Custom Mock Provider",
		}
		mockClient.SetResponse("985141000123456", custom)

		res, err := client.Lookup(context.Background(), "985141000123456")
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}
		if res.Status != "unregistered" {
			t.Errorf("expected status 'unregistered', got %s", res.Status)
		}
	})
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"985141000123456", "985141000123456"},
		{" 985-141-000 123 456 ", "985141000123456"},
		{"123*456*789", "123456789"},
		{"00064a12b3", "00064A12B3"},
		{"invalid", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := microchip.Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
