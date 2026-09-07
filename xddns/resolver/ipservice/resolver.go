package ipservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"resty.dev/v3"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/resolver"
)

func init() {
	resolver.MustRegister("ip_service", New)
}

var (
	ErrInvalidAddress           = errors.New("invalid address")
	ErrNoIPServiceNamesProvided = errors.New("no IP service names provided")
	ErrNonPublicAddress         = errors.New("non-public address")
	ErrUnexpectedStatusCode     = errors.New("unexpected status code")
)

// Config configuration.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`
	config.ProxyAwareConfig    `yaml:",inline"`

	URL lib.StringSlice `yaml:"url" comment:"IP service URLs to query for the public IP address (e.g. https://api.ipify.org, https://ifconfig.me/ip)"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	if len(cfg.URL) == 0 {
		errs = append(errs, ErrNoIPServiceNamesProvided)
	}

	for idx, url := range cfg.URL {
		if !strings.Contains(url, "://") {
			url = "https://" + url
		}

		_, err := internal.ValidateURL(url)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid url %q: %w", url, err))
		}
		cfg.URL[idx] = url
	}

	errs = append(errs, cfg.ProtocolAwareConfig.Prepare()...)
	errs = append(errs, cfg.ProxyAwareConfig.Prepare()...)

	return errs
}

// Resolver is a resolver that resolves the public IP address using the specified IP service(s).
type Resolver struct {
	cfg      Config
	clientv4 *resty.Client
	clientv6 *resty.Client

	logger zerolog.Logger
}

// New creates a new IP resolver with using IP services.
func New(cfg Config, logger zerolog.Logger) (*Resolver, error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &Resolver{
		cfg: cfg,
		clientv4: internal.RestyClient(
			internal.WithRestyRetry(),
			internal.WithRestyProxy(cfg.Proxy.FullURL()),
			internal.WithTCP("tcp4"),
		).SetRedirectPolicy(resty.RedirectNoPolicy()),
		clientv6: internal.RestyClient(
			internal.WithRestyRetry(),
			internal.WithRestyProxy(cfg.Proxy.FullURL()),
			internal.WithTCP("tcp6"),
		).SetRedirectPolicy(resty.RedirectNoPolicy()),
		logger: logger,
	}, nil
}

// Resolve resolves the public IP address using the configured IP services.
func (resol *Resolver) Resolve(ctx context.Context, protocols config.Protocols) (ips lib.IPs, err error) { //nolint:gocognit
	missingProtos := protocols

	var reqErrs []error
	for _, serviceURL := range resol.cfg.URL {
		for proto := range missingProtos.Seq() {
			client := resol.clientv4
			if proto == internal.IPv6 {
				client = resol.clientv6
			}

			resp, err := client.R().SetContext(ctx).Get(serviceURL)
			if err != nil {
				if ctx.Err() != nil {
					return ips, errors.Join(append(reqErrs, ctx.Err())...)
				}

				reqErrs = append(reqErrs, fmt.Errorf("%s (%s) request failed: %w", serviceURL, proto, err))

				continue
			}

			if resp.StatusCode() != http.StatusOK {
				reqErrs = append(reqErrs, fmt.Errorf("%w returned by %s (%s): %d", ErrUnexpectedStatusCode, serviceURL, proto, resp.StatusCode()))

				continue
			}

			respStr := resp.String()
			ipAddr, proto, isPublic := internal.IsPublicIP(respStr)
			resol.logger.Debug().Str("response", respStr).Str("url", serviceURL).Str("proto", proto).Bool("ispublic", isPublic).Msg("IP response")
			if !ipAddr.IsValid() {
				reqErrs = append(reqErrs, fmt.Errorf("%w returned by %s (%s): %q", ErrInvalidAddress, serviceURL, proto, respStr))

				continue
			}
			if !isPublic {
				reqErrs = append(reqErrs, fmt.Errorf("%w returned by %s (%s): %q", ErrNonPublicAddress, serviceURL, proto, ipAddr.String()))

				continue
			}

			ips.Set(proto, ipAddr)
			missingProtos.Set(proto, false)

			if missingProtos.IsEmpty() {
				return ips, nil
			}
		}
	}

	return ips, errors.Join(append(reqErrs, fmt.Errorf("%w: %v", resolver.ErrNotAllProtocolsResolved, missingProtos))...)
}
