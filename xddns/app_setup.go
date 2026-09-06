package xddns

import (
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/term"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/registry"
	"github.com/iceflowre/xddns/xddns/resolver"
)

const (
	// MinUpdateInterval is the minimum allowed update interval for the application.
	MinUpdateInterval = 1 * time.Minute
)

var (
	ErrFailedToCreateDriver       = errors.New("failed to create driver")
	ErrFailedToCreateLogger       = errors.New("failed to create logger")
	ErrFailedToDecodeDriverConfig = errors.New("failed to decode driver config")
	ErrInvalidLogLevel            = errors.New("invalid log level")
)

type updater struct {
	Name string

	Provider  namedProtocolDriver[provider.Provider]
	Resolvers []namedProtocolDriver[resolver.Resolver]
	Notifiers []namedDriver[notifier.Notifier]
}

type namedDriver[T any] struct {
	Name   string
	Driver T
}

type namedProtocolDriver[T any] struct {
	namedDriver[T]

	Protocols config.Protocols
}

type getRegistryEntryFn[T any] = func(name string) (registry.Entry[T], bool)

type appOptions struct {
	NotifierGetter getRegistryEntryFn[notifier.Notifier]
	ProviderGetter getRegistryEntryFn[provider.Provider]
	ResolverGetter getRegistryEntryFn[resolver.Resolver]

	OutWriter io.Writer
	LogLevel  string
}

func newAppOptions() *appOptions {
	return &appOptions{
		NotifierGetter: notifier.Get,
		ProviderGetter: provider.Get,
		ResolverGetter: resolver.Get,
	}
}

func (appOpts *appOptions) apply(opts ...AppOption) *appOptions {
	for _, opt := range opts {
		opt(appOpts)
	}

	return appOpts
}

// AppOption is a function that modifies the application options.
type AppOption func(opts *appOptions)

// WithRegistries sets the registries for notifiers, providers, and resolvers in the application options.
func WithRegistries(
	notifiers *registry.Registry[notifier.Notifier],
	providers *registry.Registry[provider.Provider],
	resolvers *registry.Registry[resolver.Resolver],
) AppOption {
	return func(opts *appOptions) {
		if notifiers != nil {
			opts.NotifierGetter = notifiers.Get
		}
		if providers != nil {
			opts.ProviderGetter = providers.Get
		}
		if resolvers != nil {
			opts.ResolverGetter = resolvers.Get
		}
	}
}

// WithOutput sets the output writer for the logger.
func WithOutput(outWriter io.Writer) AppOption {
	return func(opts *appOptions) {
		opts.OutWriter = outWriter
	}
}

// WithLogLevel sets the log level.
func WithLogLevel(level string) AppOption {
	return func(opts *appOptions) {
		opts.LogLevel = level
	}
}

// NewApp creates a new application instance with the provided configuration and options.
func NewApp(cfg config.Config, opts ...AppOption) (*App, error) {
	appOpts := newAppOptions().apply(opts...)

	resolvedConfig, errs := resolveConfig(cfg, appOpts)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	outWriter := appOpts.OutWriter
	if outWriter == nil {
		outWriter = os.Stderr
	}
	logger, err := newLogger(outWriter, resolvedConfig.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToCreateLogger, err)
	}

	updaters, err := createUpdaters(resolvedConfig, appOpts, logger)
	if err != nil {
		return nil, err
	}

	return &App{
		updateInterval: resolvedConfig.Interval,
		updaters:       updaters,
		logger:         logger,
		lastIPs:        make(map[*updater]lib.IPs, len(updaters)),
		updateGuard:    sync.Mutex{},
	}, nil
}

// ValidateConfig validates the provided configuration and returns any errors found.
func ValidateConfig(cfg *config.Config, opts ...AppOption) error {
	appOpts := newAppOptions().apply(opts...)
	_, errs := resolveConfig(*cfg, appOpts)

	return errors.Join(errs...)
}

func getOrCreateNamedDriver[T any](
	resDriver ResolvedDriver,
	instances instanceCache[namedDriver[T]],
	getter func(name string) (registry.Entry[T], bool), logger zerolog.Logger, path string,
) (nDriver namedDriver[T], err error) {
	fingerprint, err := resDriver.fingerprint()
	if err != nil {
		return namedDriver[T]{}, fmt.Errorf("%s: %w: %w", path, ErrFailedToFingerprintPreset, err)
	}
	if nDriver, ok := instances[fingerprint]; ok {
		return nDriver, nil
	}
	driver, err := createDriver(resDriver, getter, logger)
	if err != nil {
		return namedDriver[T]{}, fmt.Errorf("%s: %w: %w", path, ErrFailedToCreateDriver, err)
	}
	nDriver = namedDriver[T]{
		Name:   resDriver.Type,
		Driver: driver,
	}

	instances[fingerprint] = nDriver

	return nDriver, nil
}

func attachProtocols[T any](nDriver namedDriver[T], cfg any) namedProtocolDriver[T] {
	npDriver := namedProtocolDriver[T]{
		namedDriver: nDriver,
		Protocols:   config.DefaultProtocols(),
	}
	if proAw, ok := cfg.(config.ProtocolAware); ok {
		npDriver.Protocols = proAw.GetProtocols()
	}

	return npDriver
}

type instanceCache[T any] map[fingerprint]T

func createUpdaters(cfg *ResolvedConfig, opts *appOptions, logger zerolog.Logger) ([]*updater, error) {
	if cfg == nil || len(cfg.Updaters) == 0 {
		return []*updater{}, nil
	}

	notifierInstances := instanceCache[namedDriver[notifier.Notifier]]{}
	providerInstances := instanceCache[namedDriver[provider.Provider]]{}
	resolverInstances := instanceCache[namedDriver[resolver.Resolver]]{}
	updaters := make([]*updater, 0, len(cfg.Updaters))
	var errs []error
	for uIdx, uCfg := range cfg.Updaters {
		upd := updater{
			Name: uCfg.Name,
		}

		nDriver, err := getOrCreateNamedDriver(
			uCfg.Provider,
			providerInstances,
			opts.ProviderGetter,
			logger,
			fmt.Sprintf("updaters[%d] (%q) > provider", uIdx, uCfg.Name),
		)
		if err != nil {
			errs = append(errs, err)

			continue
		}
		upd.Provider = attachProtocols(nDriver, uCfg.Provider.Config)

		for nIdx, resDriver := range uCfg.Notifiers {
			nDriver, err := getOrCreateNamedDriver(
				resDriver,
				notifierInstances,
				opts.NotifierGetter,
				logger,
				fmt.Sprintf("updaters[%d] (%q) > notifiers[%d]", uIdx, uCfg.Name, nIdx),
			)
			if err != nil {
				errs = append(errs, err)

				continue
			}

			upd.Notifiers = append(upd.Notifiers, nDriver)
		}

		for rIdx, resDriver := range uCfg.Resolvers {
			nDriver, err := getOrCreateNamedDriver(
				resDriver,
				resolverInstances,
				opts.ResolverGetter,
				logger,
				fmt.Sprintf("updaters[%d] (%q) > resolvers[%d]", uIdx, uCfg.Name, rIdx),
			)
			if err != nil {
				errs = append(errs, err)

				continue
			}
			npDriver := attachProtocols(nDriver, resDriver.Config)

			upd.Resolvers = append(upd.Resolvers, npDriver)
		}

		updaters = append(updaters, &upd)
	}

	return updaters, errors.Join(errs...)
}

// createDriver resolves a driver and returns a driver instance, reusing an existing instance if an identical resolved config was already instantiated.
func createDriver[T any]( //nolint:ireturn
	preset ResolvedDriver,
	getConstructor func(name string) (registry.Entry[T], bool),
	parentLogger zerolog.Logger,
) (driver T, err error) {
	entry, ok := getConstructor(preset.Type)
	if !ok {
		return *new(T), fmt.Errorf("%w: %q not found", ErrTypeDoesNotExist, preset.Type)
	}

	driver, err = entry.New(preset.Config, parentLogger.With().Str("driver", preset.Type).Logger())
	if err != nil {
		return *new(T), fmt.Errorf("%w %q: %w", ErrFailedToCreateDriver, preset.Type, err)
	}

	return driver, nil
}

func writeField(h hash.Hash64, s string) {
	_, _ = h.Write([]byte(s))
	_, _ = h.Write([]byte{0})
}

func newLogger(out io.Writer, level string) (zerolog.Logger, error) {
	writer := out
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("%w %q: %w", ErrInvalidLogLevel, level, err)
	}
	if file, ok := out.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		writer = zerolog.ConsoleWriter{
			Out:        out,
			TimeFormat: time.RFC3339,
		}
	}

	return zerolog.New(writer).
		With().
		Timestamp().
		Logger().
		Level(lvl), nil
}
