package xddns

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/resolver"
)

var (
	ErrDriverNoTypeDefined         = errors.New("no type defined")
	ErrFailedToFingerprintPreset   = errors.New("failed to fingerprint preset")
	ErrFailedToMergePresets        = errors.New("failed to merge presets")
	ErrGlobalPresetsCannotUseUse   = errors.New("global presets cannot use 'use' to reference other presets")
	ErrInvalidProxy                = errors.New("invalid proxy")
	ErrNoProtocols                 = errors.New("no protocols specified")
	ErrNoResolversDefined          = errors.New("no resolvers defined")
	ErrNoUpdatersDefined           = errors.New("no updaters defined")
	ErrPresetDoesNotExist          = errors.New("preset does not exist")
	ErrPresetIsNil                 = errors.New("preset is nil")
	ErrProtocolMismatch            = errors.New("protocol mismatch")
	ErrProtocolsNotSupported       = errors.New("protocols not supported")
	ErrProxyNotSupported           = errors.New("proxy not supported")
	ErrTypeAndUseNotSpecified      = errors.New("provider type or use must be specified")
	ErrTypeAndUseSpecifiedTogether = errors.New("provider type and use cannot be specified together")
	ErrTypeDoesNotExist            = errors.New("type does not exist")
	ErrUpdateIntervalTooShort      = fmt.Errorf("update interval too short, must be at least %d minutes", int(MinUpdateInterval.Minutes()))
	ErrUpdaterNoName               = errors.New("no name")
	ErrUpdaterShareName            = errors.New("updaters share the same name")
	ErrUpdaterShareProvider        = errors.New("updaters share the same provider")
)

// ResolvedConfig represents the fully resolved configuration for the application, including all updaters, providers, resolvers and notifiers.
type ResolvedConfig struct {
	Interval time.Duration      `yaml:"interval"`
	LogLevel string             `yaml:"log_level"`
	Updaters []*ResolvedUpdater `yaml:"updaters"`
}

// ResolvedUpdater represents an updater that has been resolved from the configuration, including its provider, resolvers and notifiers.
type ResolvedUpdater struct {
	Name string `yaml:"name"`

	Provider  ResolvedDriver   `yaml:"provider"`
	Resolvers []ResolvedDriver `yaml:"resolvers,omitempty"`
	Notifiers []ResolvedDriver `yaml:"notifiers,omitempty"`
}

// ResolvedDriver represents a driver (provider, resolver or notifier) that has been resolved from a preset or directly from the configuration.
type ResolvedDriver struct {
	Type string `yaml:"type"`
	// Config is a pointer
	Config any `yaml:",omitempty,inline"`
}

type fingerprint = uint64

const zeroFingerprint fingerprint = 0

// fingerprint computes a unique fingerprint for the resolved driver based on its type and configuration.
func (rd *ResolvedDriver) fingerprint() (fingerprint, error) {
	hasher := fnv.New64a()
	writeField(hasher, rd.Type)
	_, _ = hasher.Write([]byte{0xFF})

	if rd.Config != nil {
		canonical, err := json.Marshal(rd.Config, json.Deterministic(true))
		if err != nil {
			return zeroFingerprint, fmt.Errorf("failed to marshal extra config: %w", err)
		}
		_, _ = hasher.Write(canonical)
	}

	return hasher.Sum64(), nil
}

// ResolveConfig resolves the given configuration into a fully resolved configuration, applying any provided options.
func ResolveConfig(cfg config.Config, opts ...AppOption) (*ResolvedConfig, []error) {
	return resolveConfig(cfg, newAppOptions().apply(opts...))
}

func resolveConfig(cfg config.Config, appOpts *appOptions) (resolvedConfig *ResolvedConfig, errs []error) { //nolint:revive
	if appOpts.LogLevel != "" {
		cfg.LogLevel = appOpts.LogLevel
	}

	cfgResolver := &configResolver{
		cfg:            &cfg,
		notifierGetter: appOpts.NotifierGetter,
		providerGetter: appOpts.ProviderGetter,
		resolverGetter: appOpts.ResolverGetter,
	}

	return cfgResolver.ValidateAndResolve()
}

type configResolver struct {
	cfg            *config.Config
	notifierGetter getRegistryEntryFn[notifier.Notifier]
	providerGetter getRegistryEntryFn[provider.Provider]
	resolverGetter getRegistryEntryFn[resolver.Resolver]
}

// ValidateAndResolve validates the configuration and resolves it into a ResolvedConfig.
// It returns the resolved configuration and any validation errors encountered.
func (cr *configResolver) ValidateAndResolve() (resolvedConfig *ResolvedConfig, errs []error) {
	errs = append(errs, cr.validateGlobal()...)

	resolvedUpdaters := make([]*ResolvedUpdater, 0, len(cr.cfg.Updaters))
	seenUpdaterNames := make(map[string][]int)
	seenProviderFingerprints := make(map[fingerprint][]int)

	for uIdx, uCfg := range cr.cfg.Updaters {
		resolvedUpdater, updaterErrs := cr.resolveUpdater(uCfg, uIdx)
		errs = append(errs, updaterErrs...)

		seenUpdaterNames[resolvedUpdater.Name] = append(seenUpdaterNames[resolvedUpdater.Name], uIdx)

		if resolvedUpdater.Provider.Type != "" {
			fp, err := resolvedUpdater.Provider.fingerprint()
			if err != nil {
				errs = append(errs, fmt.Errorf("updaters[%d] (%q) > provider: %w: %w", uIdx, resolvedUpdater.Name, ErrFailedToFingerprintPreset, err))
			} else {
				seenProviderFingerprints[fp] = append(seenProviderFingerprints[fp], uIdx)
			}
		}

		if len(updaterErrs) > 0 {
			continue
		}
		errs = append(errs, validateUpdater(&resolvedUpdater, uIdx)...)

		resolvedUpdaters = append(resolvedUpdaters, &resolvedUpdater)
	}

	errs = append(errs, reportDuplicates(seenUpdaterNames, func(idx int, name string) error {
		return fmt.Errorf("updaters[%d] (%q): %w", idx, name, ErrUpdaterShareName)
	})...)
	errs = append(errs, reportDuplicates(seenProviderFingerprints, func(idx int, fp fingerprint) error {
		return fmt.Errorf("updaters[%d]: %w: %x", idx, ErrUpdaterShareProvider, fp)
	})...)

	return &ResolvedConfig{
		Interval: cr.cfg.Interval,
		LogLevel: cr.cfg.LogLevel,
		Updaters: resolvedUpdaters,
	}, errs
}

// reportDuplicates returns one error per index for every key that maps to
// more than one index, using makeErr to format the message for that key.
func reportDuplicates[K comparable](seen map[K][]int, makeErr func(idx int, key K) error) (errs []error) {
	for key, indices := range seen {
		if len(indices) <= 1 {
			continue
		}
		for _, idx := range indices {
			errs = append(errs, makeErr(idx, key))
		}
	}

	return errs
}

func (cr *configResolver) resolveUpdater(cfg *config.Updater, idx int) (updater ResolvedUpdater, errs []error) {
	updater.Name = cfg.Name

	var err error
	updater.Provider, err = cr.resolveDriver(cfg.Provider, cfg, cr.cfg.Presets.Providers, cr.providerGetter)
	if err != nil {
		errs = appendErrWithPath(errs, fmt.Sprintf("updaters[%d] (%q) > provider", idx, cfg.Name), err)
	}
	for rIdx, rCfg := range cfg.Resolvers {
		res, err := cr.resolveDriver(rCfg, cfg, cr.cfg.Presets.Resolvers, cr.resolverGetter)
		if err != nil {
			errs = appendErrWithPath(errs, fmt.Sprintf("updaters[%d] (%q) > resolvers[%d]", idx, cfg.Name, rIdx), err)
		} else {
			updater.Resolvers = append(updater.Resolvers, res)
		}
	}
	for nIdx, nCfg := range cfg.Notifiers {
		notif, err := cr.resolveDriver(nCfg, cfg, cr.cfg.Presets.Notifiers, cr.notifierGetter)
		if err != nil {
			errs = appendErrWithPath(errs, fmt.Sprintf("updaters[%d] (%q) > notifiers[%d]", idx, cfg.Name, nIdx), err)
		} else {
			updater.Notifiers = append(updater.Notifiers, notif)
		}
	}

	return updater, errs
}

func (cr *configResolver) resolveDriver[T any]( //nolint:funlen,gocognit,gocyclo
	preset *config.Preset,
	uCfg *config.Updater,
	globPresets map[string]*config.Preset,
	regFn getRegistryEntryFn[T],
) (resolved ResolvedDriver, err error) {
	if preset == nil {
		return ResolvedDriver{}, ErrPresetIsNil
	}
	if preset.Type == "" && preset.Use == "" {
		return ResolvedDriver{}, ErrTypeAndUseNotSpecified
	}
	if preset.Type != "" && preset.Use != "" {
		return ResolvedDriver{}, ErrTypeAndUseSpecifiedTogether
	}

	var globPreset *config.Preset
	typeStr := preset.Type
	if preset.Use != "" {
		var ok bool
		globPreset, ok = globPresets[preset.Use]
		if !ok {
			return ResolvedDriver{}, fmt.Errorf("%w: %s", ErrPresetDoesNotExist, preset.Use)
		}
		typeStr = globPreset.Type
	}

	entry, ok := regFn(typeStr)
	if !ok {
		return ResolvedDriver{}, fmt.Errorf("%w: %q", ErrTypeDoesNotExist, typeStr)
	}

	extra := preset.Extra
	if globPreset != nil {
		var err error
		extra, err = internal.MergeYaml(globPreset.Extra, preset.Extra)
		if err != nil {
			return ResolvedDriver{}, fmt.Errorf("%w: %w", ErrFailedToMergePresets, err)
		}
	}

	driverCfg := entry.Config()
	if extra != nil {
		err := yaml.NodeToValue(extra, driverCfg)
		if err != nil {
			return ResolvedDriver{}, fmt.Errorf("%w for %q: %w", ErrFailedToDecodeDriverConfig, typeStr, err)
		}
	}

	if proAw, ok := driverCfg.(config.ProtocolAware); ok {
		switch {
		case preset.Protocols != nil:
			proAw.SetProtocols(*preset.Protocols)
		case uCfg.Protocols != nil:
			proAw.SetProtocols(*uCfg.Protocols)
		case globPreset != nil && globPreset.Protocols != nil:
			proAw.SetProtocols(*globPreset.Protocols)
		case !cr.cfg.Protocols.IsEmpty():
			proAw.SetProtocols(cr.cfg.Protocols)
		default:
			proAw.SetProtocols(config.DefaultProtocols())
		}
	}
	if proAw, ok := driverCfg.(config.ProxyAware); ok {
		switch {
		case preset.Proxy != nil:
			proAw.SetProxy(*preset.Proxy)
		case uCfg.Proxy != nil:
			proAw.SetProxy(*uCfg.Proxy)
		case globPreset != nil && globPreset.Proxy != nil:
			proAw.SetProxy(*globPreset.Proxy)
		case !cr.cfg.Proxy.IsZero():
			proAw.SetProxy(cr.cfg.Proxy)
		default:
			proAw.SetProxy(config.Proxy{})
		}
	}

	return ResolvedDriver{
		Type:   typeStr,
		Config: driverCfg,
	}, nil
}

func validateUpdater(updater *ResolvedUpdater, uIdx int) (errs []error) {
	if updater.Name == "" {
		errs = append(errs, fmt.Errorf("updaters[%d]: %w", uIdx, ErrUpdaterNoName))
	}

	prefix := fmt.Sprintf("updaters[%d] (%q)", uIdx, updater.Name)

	errs = append(errs, validateDriver(&updater.Provider, prefix+" > provider")...)

	if len(updater.Resolvers) == 0 {
		errs = append(errs, fmt.Errorf("%s > resolvers: %w", prefix, ErrNoResolversDefined))
	}
	for rIdx, driver := range updater.Resolvers {
		errs = append(errs, validateDriver(&driver, fmt.Sprintf("%s > resolvers[%d]", prefix, rIdx))...)
	}

	for nIdx, driver := range updater.Notifiers {
		errs = append(errs, validateDriver(&driver, fmt.Sprintf("%s > notifiers[%d]", prefix, nIdx))...)
	}

	errs = append(errs, validateResolverProtocols(updater, prefix)...)

	return errs
}

// validateResolverProtocols checks that if the provider requires IPv4 and/or
// IPv6, at least one resolver supplies each required protocol.
func validateResolverProtocols(updater *ResolvedUpdater, prefix string) (errs []error) {
	protoAware, ok := updater.Provider.Config.(config.ProtocolAware)
	if !ok {
		return nil
	}

	needProtocols := protoAware.GetProtocols()
	if needProtocols.IsEmpty() {
		return nil
	}

	for _, driver := range updater.Resolvers {
		protoAware, ok := driver.Config.(config.ProtocolAware)
		if !ok {
			continue
		}
		needProtocols.IPv4 = needProtocols.IPv4 && !protoAware.GetProtocols().IPv4
		needProtocols.IPv6 = needProtocols.IPv6 && !protoAware.GetProtocols().IPv6
	}

	if needProtocols.IPv4 {
		errs = append(errs, fmt.Errorf("%s: %w: provider requires IPv4", prefix, ErrProtocolMismatch))
	}
	if needProtocols.IPv6 {
		errs = append(errs, fmt.Errorf("%s: %w: provider requires IPv6", prefix, ErrProtocolMismatch))
	}

	return errs
}

func validateDriver(driver *ResolvedDriver, path string) (errs []error) {
	if proAw, ok := driver.Config.(config.ProtocolAware); ok {
		if new(proAw.GetProtocols()).IsEmpty() {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrNoProtocols))
		}
	}
	if proAw, ok := driver.Config.(config.ProxyAware); ok {
		err := new(proAw.GetProxy()).Validate()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w: %w", path, ErrInvalidProxy, err))
		}
	}

	if prep, ok := driver.Config.(config.Preparer); ok {
		errs = appendErrsWithPath(errs, path, prep.Prepare())
	}

	return errs
}

func (cr *configResolver) validateGlobal() (errs []error) {
	if cr.cfg.Interval < MinUpdateInterval {
		errs = append(errs, ErrUpdateIntervalTooShort)
	}
	if cr.cfg.LogLevel != "" {
		_, err := zerolog.ParseLevel(cr.cfg.LogLevel)
		if err != nil {
			errs = append(errs, fmt.Errorf("log_level: %w: %w", ErrInvalidLogLevel, err))
		}
	}
	if cr.cfg.Protocols.IsEmpty() {
		errs = append(errs, fmt.Errorf("protocols: %w", ErrNoProtocols))
	}

	errs = append(errs, validateGlobalPresets("notifiers", cr.cfg.Presets.Notifiers, cr.notifierGetter)...)
	errs = append(errs, validateGlobalPresets("providers", cr.cfg.Presets.Providers, cr.providerGetter)...)
	errs = append(errs, validateGlobalPresets("resolvers", cr.cfg.Presets.Resolvers, cr.resolverGetter)...)

	if len(cr.cfg.Updaters) == 0 {
		errs = append(errs, fmt.Errorf("updaters: %w", ErrNoUpdatersDefined))
	}

	for uIdx, uCfg := range cr.cfg.Updaters {
		errs = append(errs, cr.validateUpdaterConfig(uCfg, uIdx)...)
	}

	return errs
}

func (cr *configResolver) validateUpdaterConfig(uCfg *config.Updater, uIdx int) (errs []error) {
	if uCfg.Protocols.IsEmpty() {
		errs = append(errs, fmt.Errorf("updaters[%d] (%q): %w", uIdx, uCfg.Name, ErrNoProtocols))
	}

	errs = append(errs, validateUpdaterPreset(uCfg.Provider, fmt.Sprintf("updaters[%d] (%q) > provider", uIdx, uCfg.Name), cr.providerGetter)...)
	for rIdx, preset := range uCfg.Resolvers {
		errs = append(errs, validateUpdaterPreset(preset, fmt.Sprintf("updaters[%d] (%q) > resolvers[%d]", uIdx, uCfg.Name, rIdx), cr.resolverGetter)...)
	}
	for nIdx, preset := range uCfg.Notifiers {
		errs = append(errs, validateUpdaterPreset(preset, fmt.Sprintf("updaters[%d] (%q) > notifiers[%d]", uIdx, uCfg.Name, nIdx), cr.notifierGetter)...)
	}

	return errs
}

func validateUpdaterPreset[T any](preset *config.Preset, path string, getEntry getRegistryEntryFn[T]) (errs []error) {
	if preset == nil {
		return errs
	}

	if preset.Protocols.IsEmpty() {
		errs = append(errs, fmt.Errorf("%s: %w", path, ErrNoProtocols))
	}
	if preset.Type != "" { //nolint:nestif
		if entry, ok := getEntry(preset.Type); ok {
			iCfg := entry.Config()
			if _, ok = iCfg.(config.ProtocolAware); !ok && preset.Protocols != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, ErrProtocolsNotSupported))
			}
			if _, ok = iCfg.(config.ProxyAware); !ok && preset.Proxy != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, ErrProxyNotSupported))
			}
		}
	}

	return errs
}

func validateGlobalPresets[T any](kind string, presets map[string]*config.Preset, getEntry getRegistryEntryFn[T]) []error { //nolint:gocognit
	var errs []error
	for name, preset := range presets {
		path := fmt.Sprintf("presets > %s > %q", kind, name)
		if preset == nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrPresetIsNil))

			continue
		}
		if preset.Use != "" {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrGlobalPresetsCannotUseUse))
		}
		if preset.Type == "" {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrDriverNoTypeDefined))

			continue
		}
		if preset.Protocols.IsEmpty() {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrNoProtocols))
		}

		entry, ok := getEntry(preset.Type)
		if !ok {
			errs = append(errs, fmt.Errorf("%s: %w: %q", path, ErrTypeDoesNotExist, preset.Type))

			continue
		}
		iCfg := entry.Config()
		_, ok = iCfg.(config.ProtocolAware)
		if !ok && preset.Protocols != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrProtocolsNotSupported))
		}

		_, ok = iCfg.(config.ProxyAware)
		if !ok && preset.Proxy != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, ErrProxyNotSupported))
		}
	}

	return errs
}

func appendErrWithPath(errs []error, path string, err error) []error {
	return append(errs, fmt.Errorf("%s: %w", path, err))
}

func appendErrsWithPath(errs []error, path string, newErrs []error) []error {
	if len(newErrs) == 0 {
		return errs
	}
	for _, err := range newErrs {
		errs = appendErrWithPath(errs, path, err)
	}

	return errs
}
