package xddns

import (
	"bytes"
	"encoding/json/v2"
	"hash/fnv"
	"os"
	"reflect"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal/testutil"
	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/registry"
	"github.com/iceflowre/xddns/xddns/resolver"
)

func TestMinUpdateInteral(t *testing.T) {
	t.Parallel()

	assert.LessOrEqual(t, MinUpdateInterval, config.DefaultUpdateInterval)
}

func TestXddns_WriteField(t *testing.T) {
	t.Parallel()

	t.Run("identical field sequences", func(t *testing.T) {
		t.Parallel()

		exp := fnv.New64a()
		writeField(exp, "provider")
		writeField(exp, "cloudflare")

		act := fnv.New64a()
		writeField(act, "provider")
		writeField(act, "cloudflare")

		assert.Equal(t, exp.Sum64(), act.Sum64())
	})

	t.Run("different field sequences", func(t *testing.T) {
		t.Parallel()

		exo := fnv.New64a()
		writeField(exo, "provider")
		writeField(exo, "cloudflare")

		act := fnv.New64a()
		writeField(act, "provider")
		writeField(act, "route53")

		assert.NotEqual(t, exo.Sum64(), act.Sum64())
	})
}

func TestXddns_NewLogger(t *testing.T) {
	t.Parallel()

	t.Run("known level string", func(t *testing.T) {
		t.Parallel()

		levels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic", "disabled"} //nolint:goconst
		for _, level := range levels {
			t.Run(level, func(t *testing.T) {
				t.Parallel()

				expected, parseErr := zerolog.ParseLevel(level)
				require.NoError(t, parseErr)

				logger, err := newLogger(&bytes.Buffer{}, level)
				require.NoError(t, err)
				assert.Equal(t, expected, logger.GetLevel())
			})
		}
	})

	t.Run("empty level string", func(t *testing.T) {
		t.Parallel()

		logger, err := newLogger(&bytes.Buffer{}, "")
		require.NoError(t, err)
		assert.Equal(t, zerolog.NoLevel, logger.GetLevel())
	})

	t.Run("unknown level string", func(t *testing.T) {
		t.Parallel()

		logger, err := newLogger(&bytes.Buffer{}, "not-a-level")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidLogLevel)
		assert.ErrorContains(t, err, "not-a-level")
		assert.Equal(t, zerolog.Logger{}, logger)
	})

	t.Run("write newline-delimited JSON to a plain io.Writer", func(t *testing.T) {
		t.Parallel()

		buf := &bytes.Buffer{}
		logger, err := newLogger(buf, "info")
		require.NoError(t, err)

		logger.Info().Msg("hello")

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
		assert.Equal(t, "hello", payload["message"])
		assert.Contains(t, payload, "time")
	})

	t.Run("write newline-delimited JSON to a non-terminal file", func(t *testing.T) {
		t.Parallel()

		file, err := os.CreateTemp(t.TempDir(), "xddns-log-*.log")
		require.NoError(t, err)
		defer file.Close()

		logger, err := newLogger(file, "info")
		require.NoError(t, err)

		logger.Info().Msg("hello from file")
		require.NoError(t, file.Sync())

		contents, err := os.ReadFile(file.Name())
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(contents, &payload))
		assert.Equal(t, "hello from file", payload["message"])
	})
}

func TestAppOption_WithOutput(t *testing.T) {
	t.Parallel()

	t.Run("set the configured writer", func(t *testing.T) {
		t.Parallel()

		buf := &bytes.Buffer{}
		opts := newAppOptions().apply(WithOutput(buf))

		assert.Same(t, buf, opts.OutWriter)
	})

	t.Run("nil writer is stored as-is", func(t *testing.T) {
		t.Parallel()

		opts := &appOptions{OutWriter: &bytes.Buffer{}}

		WithOutput(nil)(opts)

		assert.Nil(t, opts.OutWriter)
	})
}

func TestAppOption_WithLogLevel(t *testing.T) {
	t.Parallel()

	t.Run("set the configured level", func(t *testing.T) {
		t.Parallel()

		opts := newAppOptions().apply(WithLogLevel("debug"))
		assert.Equal(t, "debug", opts.LogLevel)
	})

	t.Run("empty level overwrites any previous value", func(t *testing.T) {
		t.Parallel()

		opts := &appOptions{LogLevel: "info"}
		WithLogLevel("")(opts)
		assert.Empty(t, opts.LogLevel)
	})
}

func TestAppOption_WithRegistries(t *testing.T) {
	t.Parallel()

	t.Run("override the default notifier, provider and resolver getters", func(t *testing.T) {
		t.Parallel()

		var notifiers registry.Registry[notifier.Notifier]
		var providers registry.Registry[provider.Provider]
		var resolvers registry.Registry[resolver.Resolver]

		opts := &appOptions{
			NotifierGetter: notifier.Get,
			ProviderGetter: provider.Get,
			ResolverGetter: resolver.Get,
		}

		WithRegistries(&notifiers, &providers, &resolvers)(opts)

		require.NotNil(t, opts.NotifierGetter)
		require.NotNil(t, opts.ProviderGetter)
		require.NotNil(t, opts.ResolverGetter)

		assert.NotEqual(t, reflect.ValueOf(notifier.Get).Pointer(), reflect.ValueOf(opts.NotifierGetter).Pointer())
		assert.NotEqual(t, reflect.ValueOf(provider.Get).Pointer(), reflect.ValueOf(opts.ProviderGetter).Pointer())
		assert.NotEqual(t, reflect.ValueOf(resolver.Get).Pointer(), reflect.ValueOf(opts.ResolverGetter).Pointer())
	})
}

func TestXddns_CreateDriver(t *testing.T) {
	t.Parallel()

	t.Run("valid driver", func(t *testing.T) {
		t.Parallel()

		driver, err := createDriver(ResolvedDriver{
			Type:   "dummy",
			Config: new(testutil.NewDefaultDummyProviderConfig(t)),
		}, testutil.ProviderRegistry(t).Get, zerolog.Nop())

		require.NoError(t, err)
		assert.NotNil(t, driver)
	})

	t.Run("unregistered type", func(t *testing.T) {
		t.Parallel()

		resDriver := ResolvedDriver{Type: "dummy", Config: &testutil.DummyProviderConfig{Protocols: config.DefaultProtocols()}}
		resDriver.Type = "does-not-exist"

		_, err := createDriver(resDriver, testutil.ProviderRegistry(t).Get, zerolog.Nop())

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		assert.ErrorContains(t, err, "does-not-exist")
	})
}

func TestXddns_CreateUpdaters(t *testing.T) {
	t.Parallel()

	newProtocols := func(t *testing.T) config.Protocols {
		t.Helper()

		return config.Protocols{
			IPv4: true,
			IPv6: false,
		}
	}

	newDummyappOpts := func(t *testing.T) *appOptions {
		t.Helper()

		opts := newAppOptions().apply(WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		return opts
	}

	t.Run("create updater", func(t *testing.T) {
		t.Parallel()

		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{
					Name:      "test-updater",
					Provider:  ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))},
					Resolvers: []ResolvedDriver{{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyResolverConfig(t)), newProtocols(t))}},
				},
			},
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.NoError(t, err)
		require.Len(t, updaters, 1)
		assert.Equal(t, "test-updater", updaters[0].Name)
		require.NotNil(t, updaters[0].Provider)
		assert.Equal(t, newProtocols(t), updaters[0].Provider.Protocols)
		require.Len(t, updaters[0].Resolvers, 1)
		assert.Empty(t, updaters[0].Notifiers)
	})

	t.Run("return an empty, non-nil slice for no resolved updaters", func(t *testing.T) {
		t.Parallel()

		updaters, err := createUpdaters(nil, newDummyappOpts(t), zerolog.Nop())

		require.NoError(t, err)
		assert.NotNil(t, updaters)
		assert.Empty(t, updaters)
	})

	t.Run("reuse a single provider driver instance acrross updaters", func(t *testing.T) {
		t.Parallel()

		resolved := []*ResolvedUpdater{
			{Name: "updater-1", Provider: ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}},
			{Name: "updater-2", Provider: ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}},
		}
		cfg := &ResolvedConfig{
			Updaters: resolved,
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.NoError(t, err)
		require.Len(t, updaters, 2)
		assert.Same(t, updaters[0].Provider.Driver, updaters[1].Provider.Driver)
	})

	t.Run("reuse shared resolver and notifier driver instances across updaters", func(t *testing.T) {
		t.Parallel()

		provider1Preset := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}
		provider2Preset := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}
		sharedResolverPreset := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyResolverConfig(t)), newProtocols(t))}
		sharedNotifierPreset := ResolvedDriver{Type: "dummy", Config: new(testutil.NewDefaultDummyNotifierConfig(t))}
		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{Name: "updater-1", Provider: provider1Preset, Resolvers: []ResolvedDriver{sharedResolverPreset}, Notifiers: []ResolvedDriver{sharedNotifierPreset}},
				{Name: "updater-2", Provider: provider2Preset, Resolvers: []ResolvedDriver{sharedResolverPreset}, Notifiers: []ResolvedDriver{sharedNotifierPreset}},
			},
		}
		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.NoError(t, err)
		require.Len(t, updaters, 2)
		require.Len(t, updaters[0].Resolvers, 1)
		require.Len(t, updaters[1].Resolvers, 1)
		assert.Same(t, updaters[0].Resolvers[0].Driver, updaters[1].Resolvers[0].Driver)
		require.Len(t, updaters[0].Notifiers, 1)
		require.Len(t, updaters[1].Notifiers, 1)
		assert.Same(t, updaters[0].Notifiers[0].Driver, updaters[1].Notifiers[0].Driver)
	})

	t.Run("unknown provider type", func(t *testing.T) {
		t.Parallel()

		providerDriver := ResolvedDriver{Type: "unregistered-provider-type", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}
		resolverPreset := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyResolverConfig(t)), newProtocols(t))}
		notifierPreset := ResolvedDriver{Type: "dummy", Config: new(testutil.NewDefaultDummyNotifierConfig(t))}
		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{Name: "broken-updater", Provider: providerDriver, Resolvers: []ResolvedDriver{resolverPreset}, Notifiers: []ResolvedDriver{notifierPreset}},
			},
		}

		_, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFailedToCreateDriver)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		assert.ErrorContains(t, err, "broken-updater")
	})

	t.Run("unregistered resolver type", func(t *testing.T) {
		t.Parallel()

		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{
					Name:      "resolver-broken",
					Provider:  ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))},
					Resolvers: []ResolvedDriver{{Type: "unregistered-resolver-type", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyResolverConfig(t)), newProtocols(t))}},
				},
			},
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFailedToCreateDriver)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		assert.ErrorContains(t, err, "resolver-broken")
		require.Len(t, updaters, 1, "a resolver-only failure should not prevent the updater from being created")
		assert.NotNil(t, updaters[0].Provider)
		assert.Empty(t, updaters[0].Resolvers)
	})

	t.Run("unregistered notifier type", func(t *testing.T) {
		t.Parallel()

		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{
					Name:      "notifier-broken",
					Provider:  ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))},
					Notifiers: []ResolvedDriver{{Type: "unregistered-notifier-type", Config: new(testutil.NewDefaultDummyNotifierConfig(t))}},
				},
			},
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFailedToCreateDriver)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		assert.ErrorContains(t, err, "notifier-broken")
		require.Len(t, updaters, 1)
		assert.NotNil(t, updaters[0].Provider)
		assert.Empty(t, updaters[0].Notifiers)
	})

	t.Run("continue wiring subsequent resolvers after an earlier one fails with an unregistered type", func(t *testing.T) {
		t.Parallel()

		providerPreset := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}
		badResolver := ResolvedDriver{Type: "unregistered-resolver-type", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyResolverConfig(t)), newProtocols(t))}
		goodResolver := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyResolverConfig(t)), newProtocols(t))}

		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{Name: "partial-resolvers", Provider: providerPreset, Resolvers: []ResolvedDriver{badResolver, goodResolver}},
			},
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		require.Len(t, updaters, 1)
		require.Len(t, updaters[0].Resolvers, 1)
	})

	t.Run("continue wiring subsequent notifiers after an earlier one fails with an unregistered type", func(t *testing.T) {
		t.Parallel()

		providerPreset := ResolvedDriver{Type: "dummy", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}
		badNotifier := ResolvedDriver{Type: "unregistered-notifier-type", Config: new(testutil.NewDefaultDummyNotifierConfig(t))}
		goodNotifier := ResolvedDriver{Type: "dummy", Config: new(testutil.NewDefaultDummyNotifierConfig(t))}

		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{Name: "partial-notifiers", Provider: providerPreset, Notifiers: []ResolvedDriver{badNotifier, goodNotifier}},
			},
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		require.Len(t, updaters, 1)
		require.Len(t, updaters[0].Notifiers, 1)
	})

	t.Run("multiple failing updaters", func(t *testing.T) {
		t.Parallel()

		cfg := &ResolvedConfig{
			Updaters: []*ResolvedUpdater{
				{Name: "broken-1", Provider: ResolvedDriver{Type: "unregistered-a", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}},
				{Name: "broken-2", Provider: ResolvedDriver{Type: "unregistered-b", Config: testutil.ChangeProtocols(new(testutil.NewDefaultDummyProviderConfig(t)), newProtocols(t))}},
			},
		}

		updaters, err := createUpdaters(cfg, newDummyappOpts(t), zerolog.Nop())

		require.Error(t, err)
		assert.ErrorContains(t, err, "broken-1")
		assert.ErrorContains(t, err, "broken-2")
		assert.Empty(t, updaters)
	})
}

func TestApp_NewApp(t *testing.T) {
	t.Parallel()

	t.Run("wire a valid config end to end", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		app, err := NewApp(*cfg, WithRegistries(
			testutil.NotifierRegistry(t),
			testutil.ProviderRegistry(t),
			testutil.ResolverRegistry(t),
		), WithOutput(&bytes.Buffer{}))

		require.NoError(t, err)
		require.NotNil(t, app)
		require.Len(t, app.updaters, 1)
		assert.Equal(t, "test-updater", app.updaters[0].Name)
		assert.NotNil(t, app.updaters[0].Provider)
		assert.Len(t, app.updaters[0].Resolvers, 1)
		assert.NotNil(t, app.lastIPs)
		assert.Empty(t, app.lastIPs)
	})

	t.Run("invalid WithLogLevel option", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		app, err := NewApp(*cfg, WithLogLevel("not-a-real-level"))

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidLogLevel)
		assert.ErrorContains(t, err, "not-a-real-level")
		assert.Nil(t, app)
	})

	t.Run("WithLogLevel override the config's own log level", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		cfg.LogLevel = "info"

		app, err := NewApp(*cfg, WithRegistries(
			testutil.NotifierRegistry(t),
			testutil.ProviderRegistry(t),
			testutil.ResolverRegistry(t),
		), WithLogLevel("debug"), WithOutput(&bytes.Buffer{}))

		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, zerolog.DebugLevel, app.logger.GetLevel())
	})

	t.Run("default log level from the config", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		cfg.LogLevel = "warn"

		app, err := NewApp(*cfg, WithRegistries(
			testutil.NotifierRegistry(t),
			testutil.ProviderRegistry(t),
			testutil.ResolverRegistry(t),
		), WithOutput(&bytes.Buffer{}))

		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, zerolog.WarnLevel, app.logger.GetLevel())
	})

	t.Run("no updaters defined", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		cfg.Updaters = []*config.Updater{}

		app, err := NewApp(*cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoUpdatersDefined)
		assert.Nil(t, app)
	})

	t.Run("unregistered resolver type", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		cfg.Updaters[0].Resolvers[0] = &config.Preset{Type: "unregistered-resolver-type"}

		app, err := NewApp(*cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		assert.Nil(t, app)
	})

	t.Run("accumulates errors from multiple broken updaters", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)

		second := *cfg.Updaters[0]
		second.Name = "second-updater"
		second.Resolvers = append([]*config.Preset(nil), cfg.Updaters[0].Resolvers...)
		cfg.Updaters = append(cfg.Updaters, &second)

		cfg.Updaters[0].Resolvers[0] = &config.Preset{Type: "unregistered-resolver-type-1"}
		cfg.Updaters[1].Resolvers[0] = &config.Preset{Type: "unregistered-resolver-type-2"}

		app, err := NewApp(*cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
		assert.ErrorContains(t, err, "unregistered-resolver-type-1")
		assert.ErrorContains(t, err, "unregistered-resolver-type-2")
		assert.Nil(t, app)
	})

	t.Run("os.Stderr as the default output writer", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)

		app, err := NewApp(*cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		require.NoError(t, err)
		require.NotNil(t, app)
	})
}

func TestApp_ValidateConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)

		err := ValidateConfig(cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		assert.NoError(t, err)
	})

	t.Run("no updaters defined", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		cfg.Updaters = []*config.Updater{}

		err := ValidateConfig(cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoUpdatersDefined)
	})

	t.Run("unregistered resolver type", func(t *testing.T) {
		t.Parallel()

		cfg := baseAppConfig(t)
		cfg.Updaters[0].Resolvers[0] = &config.Preset{Type: "unregistered-resolver-type"}

		err := ValidateConfig(cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
	})
}
