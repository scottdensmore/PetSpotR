package identity_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/identity"
)

func TestWebClientConfigRejectsValuesThatAreNotCSPOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change func(*identity.WebClientConfig)
	}{
		{
			name: "authentication domain user info",
			change: func(config *identity.WebClientConfig) {
				config.AuthDomain = "attacker@petspotr.example"
			},
		},
		{
			name: "authentication emulator user info",
			change: func(config *identity.WebClientConfig) {
				config.AuthEmulatorURL = "http://attacker@127.0.0.1:9099"
			},
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := identity.WebClientConfig{
				Enabled: true, Provider: identity.ProviderGoogle, APIKey: "public-browser-key",
				AuthDomain: "auth.petspotr.example", ProjectID: "petspotr-production",
			}
			test.change(&config)
			if err := config.Validate(); err == nil {
				t.Fatalf("Validate() accepted %#v", config)
			}
		})
	}
}
