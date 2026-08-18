package identity

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const ProviderGoogle = "google.com"

// WebClientConfig contains only public Firebase browser settings. It never
// contains provider secrets, service credentials, or session material.
type WebClientConfig struct {
	Enabled         bool   `json:"enabled"`
	Provider        string `json:"provider,omitempty"`
	APIKey          string `json:"apiKey,omitempty"`
	AuthDomain      string `json:"authDomain,omitempty"`
	ProjectID       string `json:"projectId,omitempty"`
	AuthEmulatorURL string `json:"authEmulatorUrl,omitempty"`
}

// Validate ensures public configuration cannot widen browser policy through
// malformed provider-controlled values.
func (c WebClientConfig) Validate() error {
	if !c.Enabled {
		if c != (WebClientConfig{}) {
			return errors.New("identity: disabled browser configuration must be empty")
		}
		return nil
	}
	if c.Provider != ProviderGoogle || !validPublicConfigValue(c.APIKey, 512) ||
		!validPublicConfigValue(c.ProjectID, 128) {
		return errors.New("identity: invalid Google browser configuration")
	}
	if !validPublicConfigValue(c.AuthDomain, 253) || strings.Contains(c.AuthDomain, "://") ||
		strings.ContainsAny(c.AuthDomain, "/?#;") {
		return errors.New("identity: invalid authentication domain")
	}
	parsedDomain, err := url.Parse("https://" + c.AuthDomain)
	if err != nil || parsedDomain.Hostname() == "" || parsedDomain.Port() != "" ||
		parsedDomain.User != nil || parsedDomain.Host != c.AuthDomain {
		return errors.New("identity: invalid authentication domain")
	}
	if c.AuthEmulatorURL == "" {
		return nil
	}
	parsedEmulator, err := url.Parse(c.AuthEmulatorURL)
	if err != nil || parsedEmulator.Scheme != "http" || parsedEmulator.Host == "" || parsedEmulator.User != nil ||
		parsedEmulator.Path != "" || parsedEmulator.RawQuery != "" || parsedEmulator.Fragment != "" {
		return errors.New("identity: invalid browser authentication emulator origin")
	}
	host := parsedEmulator.Hostname()
	if host == "localhost" {
		return nil
	}
	address := net.ParseIP(host)
	if address == nil || !address.IsLoopback() {
		return errors.New("identity: browser authentication emulator must use loopback")
	}
	return nil
}

func validPublicConfigValue(value string, maxRunes int) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value || value == "" ||
		utf8.RuneCountInString(value) > maxRunes {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return false
		}
	}
	return true
}
