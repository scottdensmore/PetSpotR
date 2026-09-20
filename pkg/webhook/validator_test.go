package webhook_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestValidator_SSRF(t *testing.T) {
	t.Parallel()

	invalidURLs := []struct {
		url    string
		reason string
	}{
		// Loopback IPv4
		{url: "http://127.0.0.1/admin", reason: "loopback IPv4 127.0.0.1"},
		{url: "http://127.0.0.2:8080/hook", reason: "loopback IPv4 127.0.0.2"},
		{url: "https://127.255.255.255/api", reason: "loopback IPv4 range end"},

		// Localhost domain
		{url: "http://localhost/test", reason: "localhost domain"},
		{url: "http://localhost:8080/test", reason: "localhost with port"},
		{url: "http://sub.localhost/api", reason: "subdomain of localhost"},

		// RFC 1918 10.0.0.0/8
		{url: "https://10.0.0.5/api", reason: "RFC 1918 10.0.0.0/8"},
		{url: "http://10.255.255.255/webhook", reason: "RFC 1918 10.255.255.255"},

		// RFC 1918 172.16.0.0/12
		{url: "http://172.16.0.1/webhook", reason: "RFC 1918 172.16.0.1"},
		{url: "http://172.31.255.255/hook", reason: "RFC 1918 172.31.255.255"},

		// RFC 1918 192.168.0.0/16
		{url: "http://192.168.0.1/hook", reason: "RFC 1918 192.168.0.1"},
		{url: "http://192.168.1.100:3000/hook", reason: "RFC 1918 192.168.1.100 with port"},

		// Cloud metadata / Link-local (169.254.0.0/16)
		{url: "http://169.254.169.254/latest/meta-data/", reason: "cloud metadata IP 169.254.169.254"},
		{url: "http://169.254.1.1/webhook", reason: "link-local IPv4"},

		// Unspecified IPv4
		{url: "http://0.0.0.0/webhook", reason: "unspecified 0.0.0.0"},

		// IPv6 Loopback, Link-local, Unique-local
		{url: "http://[::1]/webhook", reason: "IPv6 loopback"},
		{url: "http://[fe80::1]/webhook", reason: "IPv6 link-local"},
		{url: "http://[fc00::1]/webhook", reason: "IPv6 unique local"},
		{url: "http://[::]/webhook", reason: "IPv6 unspecified"},

		// IPv4-mapped IPv6
		{url: "http://[::ffff:127.0.0.1]/webhook", reason: "IPv4-mapped loopback"},
		{url: "http://[::ffff:10.0.0.5]/api", reason: "IPv4-mapped RFC 1918"},
		{url: "http://[::ffff:169.254.169.254]/metadata", reason: "IPv4-mapped metadata"},

		// Non-HTTP/HTTPS schemes
		{url: "ftp://example.com/webhook", reason: "ftp scheme"},
		{url: "file:///etc/passwd", reason: "file scheme"},
		{url: "gopher://example.com/", reason: "gopher scheme"},
		{url: "javascript:alert(1)", reason: "javascript scheme"},

		// Malformed or empty
		{url: "", reason: "empty URL"},
		{url: "http:///no-host", reason: "missing host"},
		{url: "://missing-scheme", reason: "missing scheme"},
	}

	for _, tc := range invalidURLs {
		tc := tc
		t.Run(tc.reason, func(t *testing.T) {
			t.Parallel()
			if err := webhook.ValidateURL(tc.url); err == nil {
				t.Errorf("expected SSRF validation error for %s (%s), got nil", tc.url, tc.reason)
			}
		})
	}

	validURLs := []struct {
		url  string
		desc string
	}{
		{url: "https://api.example.com/webhook", desc: "standard HTTPS public domain"},
		{url: "http://api.example.com:8080/events", desc: "HTTP with custom port"},
		{url: "https://hooks.slack.com/services/T00/B00/X00", desc: "Slack webhook path"},
		{url: "https://webhook.site/12345678-1234-1234-1234-123456789abc", desc: "UUID path"},
		{url: "https://8.8.8.8/webhook", desc: "public IPv4"},
		{url: "https://93.184.216.34:443/feed", desc: "public IPv4 with port"},
	}

	for _, tc := range validURLs {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			if err := webhook.ValidateURL(tc.url); err != nil {
				t.Errorf("expected no error for %s (%s), got %v", tc.url, tc.desc, err)
			}
		})
	}
}
