package testutil

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
)

// DummyResolverConfig is a configuration struct for the DummyResolver, used for testing purposes.
type DummyResolverConfig struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	IPv4 netip.Addr
	IPv6 netip.Addr
	Err  error
}

// DummyResolver is a test utility that simulates a DNS resolver for testing purposes.
type DummyResolver struct {
	Cfg DummyResolverConfig

	// how often Resolve is called
	Calls atomic.Uint32
}

// NewDefaultDummyResolverConfig creates a new DummyResolverConfig with default values for testing.
func NewDefaultDummyResolverConfig(t *testing.T) DummyResolverConfig {
	t.Helper()

	return DummyResolverConfig{
		Protocols: config.DefaultProtocols(),
	}
}

// NewDummyResolver creates a new DummyResolver with the given configuration.
func NewDummyResolver(cfg DummyResolverConfig) (*DummyResolver, error) {
	return &DummyResolver{
		Cfg: cfg,
	}, nil
}

var (
	ErrNoIPv4 = errors.New("no IPv4 address available")
	ErrNoIPv6 = errors.New("no IPv6 address available")
)

// Resolve simulates the resolution of IP addresses based on the provided protocols.
func (d *DummyResolver) Resolve(_ctx context.Context, protocols config.Protocols) (lib.IPs, error) {
	d.Calls.Store(d.Calls.Add(1))
	if d.Cfg.Err != nil {
		return lib.IPs{}, d.Cfg.Err
	}

	// requested protocols should never be requested if they are not marked for use
	// this does not indicate whether the resolver supports the protocol
	if protocols.IPv4 && !d.Cfg.IPv4.IsValid() {
		return lib.IPs{}, ErrNoIPv4
	}
	if protocols.IPv6 && !d.Cfg.IPv6.IsValid() {
		return lib.IPs{}, ErrNoIPv6
	}

	var ips lib.IPs
	if d.Cfg.IPv4.IsValid() {
		ips.IPv4 = d.Cfg.IPv4
	}
	if d.Cfg.IPv6.IsValid() {
		ips.IPv6 = d.Cfg.IPv6
	}

	return ips, nil
}
