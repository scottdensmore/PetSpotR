package webhook

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	// ErrEmptyURL is returned when the provided webhook URL is empty.
	ErrEmptyURL = errors.New("webhook URL cannot be empty")
	// ErrInvalidScheme is returned when the URL scheme is not http or https.
	ErrInvalidScheme = errors.New("webhook URL scheme must be http or https")
	// ErrMissingHost is returned when the URL is missing a host.
	ErrMissingHost = errors.New("webhook URL must include a host")
	// ErrBlockedAddress is returned when the URL targets a restricted, private, or loopback address.
	ErrBlockedAddress = errors.New("webhook URL targets a restricted or private address")
)

var (
	blockedIPv4CIDRs = []*net.IPNet{
		mustParseCIDR("0.0.0.0/8"),          // Current network (RFC 791)
		mustParseCIDR("10.0.0.0/8"),         // Private-use (RFC 1918)
		mustParseCIDR("100.64.0.0/10"),      // Carrier-grade NAT (RFC 6598)
		mustParseCIDR("127.0.0.0/8"),        // Loopback (RFC 1122)
		mustParseCIDR("169.254.0.0/16"),     // Link-local / Cloud metadata (RFC 3927)
		mustParseCIDR("172.16.0.0/12"),      // Private-use (RFC 1918)
		mustParseCIDR("192.0.0.0/24"),       // IETF Protocol Assignments (RFC 6890)
		mustParseCIDR("192.0.2.0/24"),       // Documentation / TEST-NET-1 (RFC 5737)
		mustParseCIDR("192.168.0.0/16"),     // Private-use (RFC 1918)
		mustParseCIDR("198.18.0.0/15"),      // Benchmark testing (RFC 2544)
		mustParseCIDR("198.51.100.0/24"),    // Documentation / TEST-NET-2 (RFC 5737)
		mustParseCIDR("203.0.113.0/24"),     // Documentation / TEST-NET-3 (RFC 5737)
		mustParseCIDR("224.0.0.0/4"),        // Multicast (RFC 5771)
		mustParseCIDR("240.0.0.0/4"),        // Reserved for future use (RFC 1112)
		mustParseCIDR("255.255.255.255/32"), // Broadcast (RFC 919)
	}

	blockedIPv6CIDRs = []*net.IPNet{
		mustParseCIDR("::/128"),        // Unspecified
		mustParseCIDR("::1/128"),       // Loopback
		mustParseCIDR("fc00::/7"),      // Unique Local Address (ULA) (RFC 4193)
		mustParseCIDR("fe80::/10"),     // Link-Local Unicast (RFC 4291)
		mustParseCIDR("ff00::/8"),      // Multicast (RFC 4291)
		mustParseCIDR("2001:db8::/32"), // Documentation (RFC 3849)
	}
)

func mustParseCIDR(cidr string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("invalid static CIDR %q: %v", cidr, err))
	}
	return ipNet
}

// ValidateURL validates that the target URL uses http or https, has a valid host,
// and does not point to loopback, RFC 1918 private, link-local / cloud metadata, or other restricted IP ranges.
func ValidateURL(rawURL string) error {
	return ValidateURLWithOptions(rawURL, false)
}

// ValidateURLWithOptions validates a target URL, with an option to permit loopback addresses for local testing.
func ValidateURLWithOptions(rawURL string, allowLocalhost bool) error {
	if rawURL == "" {
		return ErrEmptyURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidScheme
	}

	host := parsed.Hostname()
	if host == "" {
		return ErrMissingHost
	}

	// Trim brackets for IPv6 host representation
	host = strings.Trim(host, "[]")
	hostLower := strings.ToLower(host)
	hostLower = strings.TrimSuffix(hostLower, ".")

	return validateHost(hostLower, allowLocalhost)
}

func validateHost(hostLower string, allowLocalhost bool) error {
	if allowLocalhost {
		return nil
	}

	// Block loopback domain names
	if hostLower == "localhost" || strings.HasSuffix(hostLower, ".localhost") {
		return ErrBlockedAddress
	}

	// Block cloud metadata hostnames
	if hostLower == "metadata.google.internal" || strings.HasSuffix(hostLower, ".metadata.google.internal") {
		return ErrBlockedAddress
	}

	// Block numeric integer IP representation (e.g., http://2130706433)
	if isAllDigits(hostLower) {
		return ErrBlockedAddress
	}

	// If the host is an IP literal, validate it against restricted CIDR blocks.
	ip := net.ParseIP(hostLower)
	if ip != nil {
		if err := ValidateIP(ip); err != nil {
			return err
		}
	}

	return nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ValidateIP checks if an IP address belongs to any restricted, loopback, or private CIDR range.
func ValidateIP(ip net.IP) error {
	// If IPv4 or IPv4-mapped IPv6, check IPv4 restricted ranges.
	if ip4 := ip.To4(); ip4 != nil {
		if ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast() || ip4.IsUnspecified() || ip4.IsMulticast() {
			return ErrBlockedAddress
		}
		for _, blocked := range blockedIPv4CIDRs {
			if blocked.Contains(ip4) {
				return ErrBlockedAddress
			}
		}
		return nil
	}

	// Check IPv6 restricted ranges.
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return ErrBlockedAddress
	}
	for _, blocked := range blockedIPv6CIDRs {
		if blocked.Contains(ip) {
			return ErrBlockedAddress
		}
	}

	return nil
}

// IsBlockedIP returns true if the IP belongs to any restricted or private CIDR range.
func IsBlockedIP(ip net.IP) bool {
	return ValidateIP(ip) != nil
}
