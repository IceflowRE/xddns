package shell

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/resolver"
)

func init() {
	resolver.MustRegister("shell", New)
}

var (
	ErrNoCommand      = errors.New("no command provided")
	ErrInvalidAddress = errors.New("command returned an invalid address")
	ErrNonPublicIP    = errors.New("command returned a non-public address")
)

// Config configures a command used to resolve the public IP address.
type Config struct {
	config.ProtocolAwareConfig `yaml:",inline"`

	Command string `yaml:"command" comment:"Shell command to execute, its output must contain public IP addresses"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	if strings.TrimSpace(cfg.Command) == "" {
		errs = append(errs, ErrNoCommand)
	}

	errs = append(errs, cfg.ProtocolAwareConfig.Prepare()...)

	return errs
}

// Resolver resolves public IP addresses from command output.
type Resolver struct {
	cfg    Config
	logger zerolog.Logger
}

// New creates a resolver that executes the configured command.
func New(cfg Config, logger zerolog.Logger) (*Resolver, error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &Resolver{cfg: cfg, logger: logger}, nil
}

// Resolve executes the configured command and parses public IP addresses from its output.
func (resol *Resolver) Resolve(ctx context.Context, protocols config.Protocols) (ips lib.IPs, err error) {
	shell, shellArgs := shellCommand(resol.cfg.Command)
	output, err := exec.CommandContext(ctx, shell, shellArgs...).Output() //nolint:gosec
	if err != nil {
		if ctx.Err() != nil {
			return ips, ctx.Err()
		}

		return ips, fmt.Errorf("command %q failed: %w", resol.cfg.Command, err)
	}

	var errs []error
	missingProtos := protocols
	for value := range strings.FieldsSeq(string(output)) {
		ipAddr, proto, isPublic := internal.IsPublicIP(value)
		resol.logger.Debug().Str("output", value).Str("proto", proto).Bool("ispublic", isPublic).Msg("command IP response")
		if !ipAddr.IsValid() {
			errs = append(errs, fmt.Errorf("%w: %q", ErrInvalidAddress, value))

			continue
		}
		if !isPublic {
			errs = append(errs, fmt.Errorf("%w: %q", ErrNonPublicIP, value))

			continue
		}
		if !missingProtos.IsSet(proto) {
			continue
		}

		ips.Set(proto, ipAddr)
		missingProtos.Set(proto, false)
		if missingProtos.IsEmpty() {
			return ips, nil
		}
	}

	return ips, errors.Join(append(errs, fmt.Errorf("%w: %v", resolver.ErrNotAllProtocolsResolved, missingProtos))...)
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", command}
	}

	return "sh", []string{"-c", command}
}
