package testutil

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
)

// ExtraConfig is a struct that holds additional configuration values for the DummyProvider.
type ExtraConfig struct {
	Value  int    `yaml:"value,omitempty"`
	Value2 string `yaml:"value2,omitempty"`
}

// DummyProviderConfig is a configuration struct for the DummyProvider, used for testing purposes.
type DummyProviderConfig struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	ExtraConfig `yaml:",inline"`

	Err error
}

// DummyProvider is a test implementation of a provider that simulates updating IP addresses.
type DummyProvider struct {
	Cfg DummyProviderConfig

	// how often Update is called
	Calls uint32
}

// NewDefaultDummyProviderConfig creates a default configuration for DummyProvider.
func NewDefaultDummyProviderConfig(t *testing.T) DummyProviderConfig {
	t.Helper()

	return DummyProviderConfig{
		Protocols: config.DefaultProtocols(),
	}
}

// NewDummyProvider creates a new instance of DummyProvider with the provided configuration.
func NewDummyProvider(cfg DummyProviderConfig) (*DummyProvider, error) {
	return &DummyProvider{
		Cfg: cfg,
	}, nil
}

// Update simulates the update of IP addresses for the DummyProvider.
func (d *DummyProvider) Update(_ctx context.Context, _ips lib.IPs) error {
	atomic.AddUint32(&d.Calls, 1)

	return d.Cfg.Err
}
