package strato

import (
	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/provider/dyndns"
)

func init() {
	provider.MustRegister("strato", New)
}

// Config configuration.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	Domain   lib.StringSlice  `yaml:"domain" comment:"List of domain names to update (e.g. example.com, sub.example.com)"`
	Username string           `yaml:"username" comment:"Username for Strato authentication"`
	Password lib.SecretString `yaml:"password" comment:"Password for Strato authentication"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	return toDynDNSConfig(*cfg).Prepare()
}

// Provider implements a Strato provider.
type Provider struct {
	*dyndns.Provider
}

// New creates a new Strato provider.
func New(cfg Config, logger zerolog.Logger) (*Provider, error) {
	dyndnsUpdater, err := dyndns.New(*toDynDNSConfig(cfg), logger)
	if err != nil {
		return nil, err
	}

	return &Provider{
		Provider: dyndnsUpdater,
	}, nil
}

func toDynDNSConfig(cfg Config) *dyndns.Config {
	return &dyndns.Config{
		Protocols: cfg.Protocols,
		Proxy:     cfg.Proxy,
		URL:       "https://dyndns.strato.com/nic/update",
		Username:  cfg.Username,
		Password:  cfg.Password,
		Domain:    cfg.Domain,
	}
}
