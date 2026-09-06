package dyndns

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/net/idna"
	"resty.dev/v3"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/provider"
)

func init() {
	provider.MustRegister("dyndns", New)
}

var (
	ErrInvalidDomain     = errors.New("invalid domain name")
	ErrInvalidURL        = errors.New("invalid URL")
	ErrRequestFailed     = errors.New("request failed")
	ErrUnexpectedStatus  = errors.New("unexpected status code")
	ErrUnsupportedScheme = errors.New("unsupported scheme, only https is supported")
	ErrUpdateFailed      = errors.New("update failed")
	ErrURLIsRequired     = errors.New("url is required")
)

// the protocol allows 20 requests, but we batch them in groups of 10 as a precaution.
const batchSize = 10

// Config configuration.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	URL      string           `yaml:"url" comment:"DynDNS update URL (e.g. https://example.com/nic/update)"`
	Domain   lib.StringSlice  `yaml:"domain" comment:"List of domain names to update (e.g. example.com, sub.example.com)"`
	Username string           `yaml:"username,omitempty" comment:"Username for DynDNS authentication"`
	Password lib.SecretString `yaml:"password,omitempty" comment:"Password for DynDNS authentication"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	if cfg.URL == "" {
		errs = append(errs, ErrURLIsRequired)
	}
	if !strings.Contains(cfg.URL, "://") {
		cfg.URL = "https://" + cfg.URL
	}

	u, err := internal.ValidateURL(cfg.URL)
	if err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidURL, err))
	} else if u.Scheme != "https" {
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnsupportedScheme, u.Scheme))
	}

	cfg.Domain, err = internal.NormalizeDNSNames(cfg.Domain)
	if err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidDomain, err))
	}

	errs = append(errs, cfg.ProtocolAwareConfig.Prepare()...)
	errs = append(errs, cfg.ProxyAwareConfig.Prepare()...)

	return errs
}

// Provider implements a DynDNS provider.
type Provider struct {
	cfg    Config
	client *resty.Client

	batchDomains []string
	logger       zerolog.Logger
}

// New creates a new DynDNS provider.
func New(cfg Config, logger zerolog.Logger) (*Provider, error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	batchDomains := make([]string, 0, (len(cfg.Domain)+batchSize-1)/batchSize)
	var err error
	for chunk := range slices.Chunk(cfg.Domain, batchSize) {
		chunk = slices.Clone(chunk)
		for idx, domain := range chunk {
			chunk[idx], err = idna.ToASCII(domain)
			if err != nil {
				return nil, fmt.Errorf("%w %q: %w", ErrInvalidDomain, domain, err)
			}
		}

		batchDomains = append(batchDomains, strings.Join(chunk, ","))
	}

	return &Provider{
		cfg: cfg,
		client: internal.RestyClient(
			internal.WithRestyRetry(),
			internal.WithRestyProxy(cfg.Proxy.FullURL()),
		).SetBaseURL(cfg.URL),
		batchDomains: batchDomains,
		logger:       logger,
	}, nil
}

// Update pushes the current IP(s) to a dyndns2-compatible update endpoint,
// e.g. https://<host>/nic/update?hostname=<domain>&myip=<ip>
func (prov *Provider) Update(ctx context.Context, ips lib.IPs) (err error) {
	var myIPs []string
	if ips.IPv4.IsValid() {
		myIPs = append(myIPs, ips.IPv4.String())
	}
	if ips.IPv6.IsValid() {
		myIPs = append(myIPs, ips.IPv6.String())
	}

	ipStr := strings.Join(myIPs, ",")

	for _, domains := range prov.batchDomains {
		resp, err := prov.client.R().
			SetContext(ctx).
			SetBasicAuth(prov.cfg.Username, prov.cfg.Password.Expose()).
			SetQueryParams(map[string]string{
				"hostname": domains,
				"myip":     ipStr,
			}).
			Get("")
		if err != nil {
			return fmt.Errorf("%w for [%s]: %w", ErrRequestFailed, domains, err)
		}

		if resp.IsStatusFailure() {
			return fmt.Errorf("%w %d for [%s]: %s", ErrUnexpectedStatus, resp.StatusCode(), domains, strings.TrimSpace(resp.String()))
		}

		err = parseDynDNSResponse(resp.String())
		if err != nil {
			return fmt.Errorf("%w for [%s]: %w", ErrUpdateFailed, domains, err)
		}
	}

	return nil
}

var (
	Err911           = errors.New("server-side error, try again later") //nolint:errname
	ErrAbuse         = errors.New("hostname is blocked for update abuse")
	ErrBadAgent      = errors.New("client disabled (bad user agent)")
	ErrBadAuth       = errors.New("invalid username or password")
	ErrDNSError      = errors.New("server-side DNS error")
	ErrEmptyResponse = errors.New("empty response")
	ErrNoHost        = errors.New("hostname does not exist for this account")
	ErrNotFQDN       = errors.New("hostname is not a valid fully-qualified domain name")
	ErrUnrecognized  = errors.New("unrecognized response")
)

// parseDynDNSResponse interprets the standard dyndns2 response codes.
// See: https://help.dyn.com/remote-access-api/perform-update/ and https://noip.com/integrate/response
func parseDynDNSResponse(body string) error {
	line := strings.TrimSpace(body)
	if line == "" {
		return ErrEmptyResponse
	}

	code := strings.Fields(line)[0]

	switch code {
	case "good", "nochg":
		return nil
	case "badauth":
		return ErrBadAuth
	case "notfqdn":
		return ErrNotFQDN
	case "nohost":
		return ErrNoHost
	case "abuse":
		return ErrAbuse
	case "badagent":
		return ErrBadAgent
	case "dnserr":
		return ErrDNSError
	case "911":
		return Err911
	default:
		return fmt.Errorf("%w: %s", ErrUnrecognized, line)
	}
}
