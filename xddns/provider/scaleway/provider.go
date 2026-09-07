package scaleway

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	scwdomain "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/provider"
)

func init() {
	provider.MustRegister("scaleway", New)
}

const (
	defaultTTL = uint32(150)
)

var (
	ErrInvalidDomain        = errors.New("invalid domain name")
	ErrMissingRequiredField = errors.New("is required")
	ErrNoValidRecordNames   = errors.New("no valid record names found for domains")
)

// Config configuration.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	Domain    lib.StringSlice  `yaml:"domain" comment:"List of domain names to update (e.g. example.com, sub.example.com)"`
	ProjectID string           `yaml:"project_id" comment:"Scaleway project ID"`
	AccessKey lib.SecretString `yaml:"access_key" comment:"Scaleway access key"`
	SecretKey lib.SecretString `yaml:"secret_key" comment:"Scaleway secret key"`

	// Optional zone name for the DNS records. If not provided, the root domain will be used.
	Zone string `yaml:"zone" comment:"Zone name for the DNS records (optional)"`
	// Optional TTL for the DNS record. If not provided, the default TTL (150) will be used.
	TTL *uint32 `yaml:"ttl" comment:"TTL for the DNS record (default: 150) (optional)"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	if len(cfg.Domain) == 0 {
		errs = append(errs, fmt.Errorf("domain %w", ErrMissingRequiredField))
	} else {
		errs = append(errs, cfg.prepareDomain()...)
	}

	errs = append(errs, requireField("project_id", cfg.ProjectID)...)
	errs = append(errs, requireField("access_key", cfg.AccessKey.Expose())...)
	errs = append(errs, requireField("secret_key", cfg.SecretKey.Expose())...)

	if cfg.TTL == nil {
		cfg.TTL = new(defaultTTL)
	}

	errs = append(errs, cfg.ProtocolAwareConfig.Prepare()...)
	errs = append(errs, cfg.ProxyAwareConfig.Prepare()...)

	return errs
}

// prepareDomain normalizes cfg.Domain and cfg.Zone and validates that every
// domain belongs to the resolved zone. cfg.Domain is assumed non-empty.
func (cfg *Config) prepareDomain() (errs []error) {
	var err error
	cfg.Domain, err = internal.NormalizeDNSNames(cfg.Domain)
	if err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidDomain, err))
	}

	switch cfg.Zone {
	case "", internal.DNSZoneRoot:
		cfg.Zone, err = internal.GetDNSNameRoot(cfg.Domain[0])
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to determine DNS root: %w", err))
		}
	default:
		cfg.Zone, err = internal.NormalizeDNSName(cfg.Zone)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid zone: %w", err))
		}
	}

	for _, domain := range cfg.Domain {
		err := internal.IsValidDNSZone(cfg.Zone, domain)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// requireField returns a MissingRequiredField error for name if val is empty.
func requireField(name string, val string) []error {
	if val == "" {
		return []error{fmt.Errorf("%s %w", name, ErrMissingRequiredField)}
	}

	return nil
}

// Provider is a Scaleway provider.
type Provider struct {
	cfg Config
	api *scwdomain.API

	recordNames []string

	logger zerolog.Logger
}

// New creates a new Scaleway provider.
func New(cfg Config, logger zerolog.Logger) (sw *Provider, err error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	rClient := internal.RestyClient(internal.WithRestyProxy(cfg.Proxy.FullURL()))
	client, err := scw.NewClient(
		scw.WithAuth(cfg.AccessKey.Expose(), cfg.SecretKey.Expose()),
		scw.WithDefaultProjectID(cfg.ProjectID),
		scw.WithHTTPClient(rClient.Client()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Scaleway client: %w", err)
	}

	recordNames := make([]string, 0, 2*len(cfg.Domain)) //nolint:mnd
	for _, domain := range cfg.Domain {
		recordName, err := internal.GetRecordName(domain, cfg.Zone)
		if err != nil {
			return nil, fmt.Errorf("failed to get record name for domain %s: %w", domain, err)
		}

		recordNames = append(recordNames, recordName)
	}

	if len(recordNames) == 0 {
		return nil, fmt.Errorf("%w: %v", ErrNoValidRecordNames, cfg.Domain)
	}

	return &Provider{
		cfg:         cfg,
		api:         scwdomain.NewAPI(client),
		recordNames: recordNames,
		logger:      logger,
	}, nil
}

// Update updates the DNS records for the configured domains with the provided IPs.
func (prov *Provider) Update(ctx context.Context, ips lib.IPs) error {
	changes := make([]*scwdomain.RecordChange, 0, 2*len(prov.cfg.Domain)) //nolint:mnd
	for _, recordName := range prov.recordNames {
		if ips.IPv4.IsValid() {
			changes = append(changes, &scwdomain.RecordChange{
				Set: &scwdomain.RecordChangeSet{
					IDFields: &scwdomain.RecordIdentifier{
						Name: recordName,
						Type: scwdomain.RecordTypeA,
					},
					Records: []*scwdomain.Record{{
						Name: recordName,
						Type: scwdomain.RecordTypeA,
						Data: ips.IPv4.String(),
						TTL:  *prov.cfg.TTL,
					}},
				},
			})
		}
		if ips.IPv6.IsValid() {
			changes = append(changes, &scwdomain.RecordChange{
				Set: &scwdomain.RecordChangeSet{
					IDFields: &scwdomain.RecordIdentifier{
						Name: recordName,
						Type: scwdomain.RecordTypeAAAA,
					},
					Records: []*scwdomain.Record{{
						Name: recordName,
						Type: scwdomain.RecordTypeAAAA,
						Data: ips.IPv6.String(),
						TTL:  *prov.cfg.TTL,
					}},
				},
			})
		}
	}

	if len(changes) == 0 {
		return nil
	}

	_, err := prov.api.UpdateDNSZoneRecords(&scwdomain.UpdateDNSZoneRecordsRequest{
		DNSZone:                 prov.cfg.Zone,
		Changes:                 changes,
		ReturnAllRecords:        new(false),
		DisallowNewZoneCreation: true,
	}, scw.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to update Scaleway DNS records: %w", err)
	}

	return nil
}
