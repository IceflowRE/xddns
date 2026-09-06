package internal

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// DNSZoneRoot represents the root of a DNS zone, which is denoted by "@".
const DNSZoneRoot = "@"

var (
	ErrDomainTooLong   = errors.New("domain exceeds maximum length")
	ErrEmptyDomain     = errors.New("domain is empty")
	ErrInvalidDomain   = errors.New("invalid domain name")
	ErrInvalidWildcard = errors.New("invalid wildcard domain")
)

// dnsLabel matches a single DNS label.
var dnsLabel = regexp.MustCompile(`^[a-zA-Z0-9_](?:[a-zA-Z0-9_-]*[a-zA-Z0-9_])?$`)

// smoothZoneOrName lowercases and strips a trailing dot.
func smoothZoneOrName(s string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "."))
}

// NormalizeDNSName normalizes and validates a single DNS name.
// It returns the normalized name or an error if the name is invalid.
func NormalizeDNSName(domain string) (string, error) {
	domain = smoothZoneOrName(domain)
	if domain == "" {
		return "", ErrEmptyDomain
	}

	hasWildcard := strings.HasPrefix(domain, "*.")
	if hasWildcard {
		domain = strings.TrimPrefix(domain, "*.")
	}
	if domain == "" || strings.Contains(domain, "*") {
		return "", ErrInvalidWildcard
	}

	ascii, err := idna.ToASCII(domain)
	if err != nil {
		return "", err
	}

	maxLen := 253
	if hasWildcard {
		maxLen -= 2
	}
	if len(ascii) > maxLen {
		return "", ErrDomainTooLong
	}

	for label := range strings.SplitSeq(ascii, ".") {
		if len(label) == 0 || len(label) > 63 || !dnsLabel.MatchString(label) {
			return "", ErrInvalidDomain
		}
	}

	unicode, err := idna.ToUnicode(ascii)
	if err != nil {
		return "", err
	}

	if hasWildcard {
		unicode = "*." + unicode
	}

	return unicode, nil
}

// NormalizeDNSNames normalizes and validates a slice of DNS names.
// Invalid entries are rejected.
func NormalizeDNSNames(dnsNames []string) ([]string, error) {
	if len(dnsNames) == 0 {
		return nil, ErrEmptyDomain
	}

	normedDomains := make([]string, 0, len(dnsNames))
	for _, domain := range dnsNames {
		normedDomain, err := NormalizeDNSName(domain)
		if err != nil {
			return nil, fmt.Errorf("%w %q: %w", ErrInvalidDomain, domain, err)
		}
		normedDomains = append(normedDomains, normedDomain)
	}

	return normedDomains, nil
}

var (
	ErrDomainNotInZone = errors.New("domain is not part of the zone")
	ErrEmptyZone       = errors.New("zone name is empty")
	ErrInvalidDNSRoot  = errors.New("DNS root is invalid")
	ErrInvalidZone     = errors.New("invalid zone name")
	ErrNotNormalized   = errors.New("zone or domain is not normalized")
)

// GetDNSNameRoot returns the root domain for a given DNS name.
func GetDNSNameRoot(dnsName string) (string, error) {
	err := IsValidDNSName(dnsName)
	if err != nil {
		return "", err
	}
	// remove wildcard
	dnsName = strings.TrimPrefix(dnsName, "*.")

	ascii, err := idna.ToASCII(dnsName)
	if err != nil {
		return "", fmt.Errorf("%w %q: %w", ErrInvalidDomain, dnsName, err)
	}

	rootASCII, err := publicsuffix.EffectiveTLDPlusOne(ascii)
	if err != nil {
		return "", fmt.Errorf("%w %q: %w", ErrInvalidDNSRoot, dnsName, err)
	}

	return idna.ToUnicode(rootASCII)
}

// IsValidDNSName checks if the provided DNS name is valid and normalized.
func IsValidDNSName(dnsName string) error {
	norm, err := NormalizeDNSName(dnsName)
	if err != nil {
		return err
	}
	if norm != dnsName {
		return ErrNotNormalized
	}

	return nil
}

// IsValidDNSZone checks if the provided zone is a valid zone for the given DNS name.
func IsValidDNSZone(zone string, dnsName string) error {
	err := IsValidDNSName(dnsName)
	if err != nil {
		return err
	}
	// no need to norm the zone it has to be part of the dns name which is already valid

	normZone := smoothZoneOrName(zone)
	if zone == "" {
		return ErrEmptyZone
	}
	if normZone != zone {
		return ErrNotNormalized
	}
	if zone == DNSZoneRoot {
		return nil
	}

	if dnsName != zone && !strings.HasSuffix(dnsName, "."+zone) {
		return fmt.Errorf("%w: %q - %q", ErrDomainNotInZone, dnsName, zone)
	}

	return nil
}

// GetRecordName returns the record name for a given DNS name and zone.
// It is the part of the DNS name that is not part of the zone.
func GetRecordName(dnsName string, zone string) (string, error) {
	err := IsValidDNSName(dnsName)
	if err != nil {
		return "", err
	}

	zone, err = resolveZone(dnsName, zone)
	if err != nil {
		return "", err
	}

	if dnsName == zone {
		return DNSZoneRoot, nil
	}

	suffix := "." + zone
	if !strings.HasSuffix(dnsName, suffix) {
		return "", fmt.Errorf("%w: %q - %q", ErrDomainNotInZone, dnsName, zone)
	}

	return strings.TrimSuffix(dnsName, suffix), nil
}

// resolveZone validates an explicit zone against dnsName or if zone is the
// root, derives the effective zone from dnsName itself.
func resolveZone(dnsName string, zone string) (string, error) {
	if zone == DNSZoneRoot {
		zone, err := publicsuffix.EffectiveTLDPlusOne(dnsName)
		if err != nil {
			return "", fmt.Errorf("%w %q: %w", ErrInvalidDNSRoot, dnsName, err)
		}

		return zone, nil
	}

	err := IsValidDNSName(zone)
	if errors.Is(err, ErrEmptyDomain) {
		return "", ErrEmptyZone
	}
	if err != nil {
		return "", err
	}

	err = IsValidDNSZone(zone, dnsName)
	if err != nil {
		return "", err
	}

	return zone, nil
}
