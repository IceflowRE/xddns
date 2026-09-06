package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"golang.org/x/net/publicsuffix"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
)

var (
	ErrFindingAuthoritativeNameserver = errors.New("finding authoritative nameserver")
	ErrNoProtocolsSpecified           = errors.New("no protocols specified")
	ErrNoRecordsReturned              = errors.New("no records returned")
)

// ResolveAuthoritative resolves the given domain name using its authoritative nameserver(s).
// Returns the resolved IP addresses based on the specified protocols (IPv4 and/or IPv6).
func ResolveAuthoritative(ctx context.Context, domain string, protocols config.Protocols) (lib.IPs, error) {
	nameserver, err := findAuthoritativeNameserver(ctx, domain)
	if err != nil {
		return lib.IPs{}, fmt.Errorf("%w for %s: %w", ErrFindingAuthoritativeNameserver, domain, err)
	}

	resolver := nsResolver(nameserver)
	ips, err := lookupIP(ctx, resolver, domain, protocols)
	if err != nil {
		return lib.IPs{}, fmt.Errorf("resolving %s via %s: %w", domain, nameserver, err)
	}

	result := lib.IPs{}
	for _, ip := range ips {
		if ip.Is4() && protocols.IsSet(internal.IPv4) {
			result.IPv4 = ip
		}
		if ip.Is6() && protocols.IsSet(internal.IPv6) {
			result.IPv6 = ip
		}
	}

	return result, nil
}

var ErrNoNSRecordsRound = errors.New("no NS records found")

func findAuthoritativeNameserver(ctx context.Context, domain string) (string, error) {
	name := strings.TrimSuffix(domain, ".")
	boundary, err := publicsuffix.EffectiveTLDPlusOne(name)
	if err != nil {
		return "", fmt.Errorf("determining registrable domain for %s: %w", domain, err)
	}

	for {
		nss, err := net.DefaultResolver.LookupNS(ctx, name)
		if err == nil && len(nss) > 0 {
			return strings.TrimSuffix(nss[0].Host, "."), nil
		}
		dnsErr, isDNSErr := errors.AsType[*net.DNSError](err)
		if err != nil && (isDNSErr || !dnsErr.IsNotFound) {
			return "", fmt.Errorf("looking up NS for %s: %w", name, err)
		}

		if name == boundary {
			return "", fmt.Errorf("%w up to registrable domain %s", ErrNoNSRecordsRound, boundary)
		}

		var found bool
		_, name, found = strings.Cut(name, ".")
		if !found {
			return "", fmt.Errorf("%w up to root for %s", ErrNoNSRecordsRound, domain)
		}
	}
}

func nsResolver(namespace string) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network string, _ string) (net.Conn, error) {
			var d net.Dialer

			return d.DialContext(ctx, network, net.JoinHostPort(namespace, "53"))
		},
	}
}

func lookupIP(ctx context.Context, resolver *net.Resolver, domain string, protocols config.Protocols) ([]netip.Addr, error) {
	var network string
	switch {
	case protocols.IsSet(internal.IPv4) && protocols.IsSet(internal.IPv6):
		network = "ip"
	case protocols.IsSet(internal.IPv4):
		network = "ip4"
	case protocols.IsSet(internal.IPv6):
		network = "ip6"
	default:
		return nil, fmt.Errorf("%w for resolving %s", ErrNoProtocolsSpecified, domain)
	}

	addrs, err := resolver.LookupNetIP(ctx, network, domain)
	if err != nil {
		dnsErr, ok := errors.AsType[*net.DNSError](err)
		if ok && dnsErr.IsNotFound {
			return nil, nil
		}

		return nil, err
	}

	return addrs, nil
}
