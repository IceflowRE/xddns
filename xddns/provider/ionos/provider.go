package ionos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"resty.dev/v3"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/provider"
)

func init() {
	provider.MustRegister("ionos", New)
}

var (
	ErrEitherDomainOrUpdateURLRequired        = errors.New("either domain or update_url is required")
	ErrEitherDomainOrUpdateURLRequiredNotBoth = errors.New("either domain or update_url is required, not both")
	ErrInvalidDomain                          = errors.New("invalid domain name")
	ErrPublicPrefixRequired                   = errors.New("public_prefix is required")
	ErrRequestFailed                          = errors.New("request failed")
	ErrSecretRequired                         = errors.New("secret is required")
	ErrUnexpectedStatus                       = errors.New("unexpected status code")
	ErrUpdateFailed                           = errors.New("update failed")
)

// Config configuration.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	Domain       lib.StringSlice  `yaml:"domain,omitempty" comment:"List of domain names to update (e.g. example.com, sub.example.com). Provider either the domains OR update_url."` //nolint:lll
	UpdateURL    lib.SecretString `yaml:"update_url,omitempty" comment:"Update URL (e.g. https://ipv4.api.hosting.ionos.com/dns/v1/dyndns?q=...)"`
	PublicPrefix string           `yaml:"public_prefix" comment:"Public prefix"`
	Secret       lib.SecretString `yaml:"secret" comment:"Secret"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	var err error

	switch {
	case len(cfg.Domain) == 0 && cfg.UpdateURL == "":
		errs = append(errs, ErrEitherDomainOrUpdateURLRequired)
	case len(cfg.Domain) > 0 && cfg.UpdateURL != "":
		errs = append(errs, ErrEitherDomainOrUpdateURLRequiredNotBoth)
	case len(cfg.Domain) > 0:
		cfg.Domain, err = internal.NormalizeDNSNames(cfg.Domain)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidDomain, err))
		}
	case cfg.UpdateURL != "":
		_, err := internal.ValidateURL(cfg.UpdateURL.Expose())
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid update_url: %w", err))
		}
	default:
	}

	if cfg.PublicPrefix == "" {
		errs = append(errs, ErrPublicPrefixRequired)
	}
	if cfg.Secret == "" {
		errs = append(errs, ErrSecretRequired)
	}

	errs = append(errs, cfg.ProtocolAwareConfig.Prepare()...)
	errs = append(errs, cfg.ProxyAwareConfig.Prepare()...)

	return errs
}

// Provider is the IONOS provider.
type Provider struct {
	cfg    Config
	client *resty.Client

	logger zerolog.Logger
}

// New creates a new IONOS provider.
func New(cfg Config, logger zerolog.Logger) (sw *Provider, err error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	prov := &Provider{
		cfg: cfg,
		client: internal.RestyClient(
			internal.WithRestyRetry(),
			internal.WithRestyProxy(cfg.Proxy.FullURL()),
		).
			SetHeader("X-API-Key", fmt.Sprintf("%s.%s", cfg.PublicPrefix, cfg.Secret.Expose())).
			SetBaseURL("https://api.hosting.ionos.com/dns"),
		logger: logger,
	}

	if len(cfg.Domain) > 0 {
		// quota of 2 requests per minute
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute) //nolint:mnd
		defer cancel()
		tmpClient := prov.client.Clone(ctx).
			SetRetryMaxWaitTime(3 * time.Minute). //nolint:mnd
			SetRetryWaitTime(1 * time.Minute)

		res, err := postDYNDNS(ctx, tmpClient, cfg.Domain, "xddns")
		if err != nil {
			return nil, fmt.Errorf("failed to create dyndns update URL: %w", err)
		}
		logger.Debug().Strs("domains", prov.cfg.Domain).Msg("Created IONOS dyndns update URL")

		prov.cfg.UpdateURL = lib.SecretString(res.UpdateURL)
		prov.cfg.Domain = nil
		cancel()
	}

	return prov, nil
}

// Update updates the IP addresses for the configured domains using the IONOS dyndns update URL.
func (prov *Provider) Update(ctx context.Context, ips lib.IPs) error {
	req := prov.client.R().SetContext(ctx)
	if ips.IPv4.IsValid() {
		req.SetQueryParam("ipv4", ips.IPv4.String())
	}
	if ips.IPv6.IsValid() {
		req.SetQueryParam("ipv6", ips.IPv6.String())
	}

	resp, err := req.Get(prov.cfg.UpdateURL.Expose())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	if resp.IsStatusFailure() {
		return fmt.Errorf("%w %d: %s", ErrUnexpectedStatus, resp.StatusCode(), strings.TrimSpace(resp.String()))
	}

	return nil
}

type postDYNDNSBody struct {
	Domains     []string `json:"domains"`
	Description string   `json:"description"`
}

type postDYNDNSResponse struct {
	BulkID      string   `json:"bulkId"`
	UpdateURL   string   `json:"updateUrl"`
	Domains     []string `json:"domains"`
	Description string   `json:"description"`
}

type postDYNDNSErrorResponse []postDYNDNSErrorResponseItem

type postDYNDNSErrorResponseItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var ErrFailedToCreateUpdateURL = errors.New("failed to create dyndns update URL")

// https://developer.hosting.ionos.com/docs/dns
func postDYNDNS(ctx context.Context, client *resty.Client, domains []string, description string) (resp *postDYNDNSResponse, err error) {
	resp = &postDYNDNSResponse{}
	errResp := postDYNDNSErrorResponse{}

	client.SetDebug(true)
	httpResp, err := client.R().SetBody(postDYNDNSBody{
		Domains:     domains,
		Description: description,
	}).SetContext(ctx).SetResult(resp).SetResultError(&errResp).Post("/v1/dyndns")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}

	if httpResp.IsStatusFailure() {
		msgs := make([]string, 0, len(errResp))
		for _, item := range errResp {
			msgs = append(msgs, fmt.Sprintf("%s: %s", item.Code, item.Message))
		}

		return nil, fmt.Errorf("%w: %s", ErrFailedToCreateUpdateURL, strings.Join(msgs, "; "))
	}

	return resp, nil
}
