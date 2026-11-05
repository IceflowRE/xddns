package netif

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"strings"

	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/resolver"
)

func init() {
	resolver.MustRegister("netif", New)
}

var (
	ErrFailedToListInterfaces     = errors.New("failed to list system interfaces")
	ErrInvalidIP                  = errors.New("interface returned an invalid address")
	ErrNoInterfaceAvailable       = errors.New("no network interface available")
	ErrNoNetworkInterfaceProvided = errors.New("no network interface name provided")
)

// Config configuration.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`

	Interface lib.StringSlice `yaml:"interface" comment:"List of the network interface to use for resolving the public IP address (e.g. eth0, en0)"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	if len(cfg.Interface) == 0 {
		errs = append(errs, ErrNoNetworkInterfaceProvided)
	}
	iterIfaces, err := iterInterfaces(cfg.Interface)
	if err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrFailedToListInterfaces, err))
	}
	foundOne := false
	for range iterIfaces {
		foundOne = true

		break
	}
	if !foundOne {
		errs = append(errs, fmt.Errorf("%w [%s]", ErrNoInterfaceAvailable, strings.Join(cfg.Interface, ", ")))
	}

	errs = append(errs, cfg.ProtocolAwareConfig.Prepare()...)

	return errs
}

// Resolver is a resolver that resolves the public IP address using the specified network interface(s).
type Resolver struct {
	cfg Config

	logger zerolog.Logger
}

// New creates a new IP resolver with using network interfaces.
func New(cfg Config, logger zerolog.Logger) (*Resolver, error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &Resolver{
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Resolve resolves the public IP address.
func (resol *Resolver) Resolve(ctx context.Context, protocols config.Protocols) (ips lib.IPs, err error) { //nolint:gocognit
	missingProtos := protocols

	iterIfaces, err := iterInterfaces(resol.cfg.Interface)
	if err != nil {
		return ips, err
	}

	var errs []error
	for iface := range iterIfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return ips, fmt.Errorf("failed to list addresses for interface %q: %w", resol.cfg.Interface, err)
		}

		for _, addr := range addrs {
			if ctx.Err() != nil {
				return ips, errors.Join(append(errs, ctx.Err())...)
			}

			parsedAddr, err := parseInterfaceAddress(addr.String())
			if !parsedAddr.ip.IsValid() {
				errs = append(errs, fmt.Errorf("%w: (%q): %q", ErrInvalidIP, resol.cfg.Interface, addr.String()))

				continue
			}
			if err != nil {
				errs = append(errs, err)

				continue
			}
			if parsedAddr.temporary {
				continue
			}

			_, _, isPublic := internal.IsPublicIP(parsedAddr.ip.String())
			if !missingProtos.IsSet(parsedAddr.protocol) || !isPublic {
				continue
			}

			ips.Set(parsedAddr.protocol, parsedAddr.ip)
			missingProtos.Set(parsedAddr.protocol, false)

			if missingProtos.IsEmpty() {
				return ips, nil
			}
		}
	}

	return ips, errors.Join(append(errs, fmt.Errorf("%w: %v", resolver.ErrNotAllProtocolsResolved, missingProtos))...)
}

type interfaceAddress struct {
	ip        netip.Addr
	protocol  string
	temporary bool
}

func parseInterfaceAddress(value string) (interfaceAddress, error) {
	value = strings.TrimSpace(value)
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		ip, err := netip.ParseAddr(value)
		if err != nil {
			return interfaceAddress{}, err
		}

		if ip.Is4() {
			return interfaceAddress{ip: ip, protocol: internal.IPv4}, nil
		}

		return interfaceAddress{ip: ip, protocol: internal.IPv6}, nil
	}

	ip := prefix.Addr()
	if ip.Is4() {
		return interfaceAddress{ip: ip, protocol: internal.IPv4}, nil
	}

	return interfaceAddress{
		ip:        ip,
		protocol:  internal.IPv6,
		temporary: runtime.GOOS == "windows" && prefix.Bits() == 128,
	}, nil
}

func iterInterfaces(names lib.StringSlice) (iter.Seq[net.Interface], error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list system interfaces: %w", err)
	}

	return func(yield func(net.Interface) bool) {
		for _, iface := range ifaces {
			if !slices.Contains(names, iface.Name) {
				continue
			}
			if !yield(iface) {
				return
			}
		}
	}, nil
}
