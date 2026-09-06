package internal

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"
)

var (
	ErrEmptyURL   = errors.New("url is empty")
	ErrInvalidURL = errors.New("url is invalid")
)

// ValidateURL validates the given URL string and returns a parsed URL object if valid.
func ValidateURL(urlStr string) (*url.URL, error) {
	if urlStr == "" {
		return nil, ErrEmptyURL
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if parsedURL.Host == "" {
		return parsedURL, ErrInvalidURL
	}

	return parsedURL, nil
}

var nonPublicPrefixes = []netip.Prefix{ //nolint:gochecknoglobals
	// IPv4
	// RFC 791 - "this" network
	netip.MustParsePrefix("0.0.0.0/8"),
	// RFC 6598 - Carrier-Grade NAT (CGNAT)
	netip.MustParsePrefix("100.64.0.0/10"),
	// RFC 6890 - IETF Protocol Assignments
	netip.MustParsePrefix("192.0.0.0/24"),
	// RFC 5737 - TEST-NET-1 (documentation)
	netip.MustParsePrefix("192.0.2.0/24"),
	// RFC 2544 - Benchmark testing
	netip.MustParsePrefix("198.18.0.0/15"),
	// RFC 5737 - TEST-NET-2 (documentation)
	netip.MustParsePrefix("198.51.100.0/24"),
	// RFC 5737 - TEST-NET-3 (documentation)
	netip.MustParsePrefix("203.0.113.0/24"),
	// RFC 1112 - Reserved for future use (Class E)
	netip.MustParsePrefix("240.0.0.0/4"),
	// RFC 8190 - Limited broadcast
	netip.MustParsePrefix("255.255.255.255/32"),

	// IPv6
	// RFC 6666 - Discard-Only address block
	netip.MustParsePrefix("100::/64"),
	// RFC 4380 - Teredo tunneling
	netip.MustParsePrefix("2001::/32"),
	// RFC 5180 - Benchmarking
	netip.MustParsePrefix("2001:2::/48"),
	// RFC 3849 - Documentation
	netip.MustParsePrefix("2001:db8::/32"),
	// RFC 3056 - 6to4 (relies on unverifiable relays)
	netip.MustParsePrefix("2002::/16"),
	// RFC 6052 - NAT64 well-known prefix
	netip.MustParsePrefix("64:ff9b::/96"),
}

const (
	// IPv4 represents the IPv4 protocol.
	IPv4 string = "ipv4"
	// IPv6 represents the IPv6 protocol.
	IPv6 string = "ipv6"
)

// IsPublicIP checks if the given IP address is a public IP address.
func IsPublicIP(ip string) (ipAddr netip.Addr, protocol string, isPublic bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return netip.Addr{}, "", false
	}

	version := IPv6
	if addr.Is4() {
		version = IPv4
	}

	if addr.Is4In6() || addr.Zone() != "" || !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return addr, version, false
	}

	if slices.ContainsFunc(nonPublicPrefixes, func(prefix netip.Prefix) bool {
		return prefix.Contains(addr)
	}) {
		return addr, version, false
	}

	return addr, version, true
}
