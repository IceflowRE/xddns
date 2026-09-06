package xddns

import (
	"errors"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal/testutil"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/registry"
)

func newCR(t *testing.T, cfg *config.Config) *configResolver {
	t.Helper()

	return new(configResolver{
		cfg:            cfg,
		notifierGetter: testutil.NotifierRegistry(t).Get,
		providerGetter: testutil.ProviderRegistry(t).Get,
		resolverGetter: testutil.ResolverRegistry(t).Get,
	})
}

func baseUpdaterCfg(t *testing.T) *config.Updater {
	t.Helper()

	return &config.Updater{
		Name:      "test-updater",
		Provider:  &config.Preset{Type: "dummy"},
		Resolvers: []*config.Preset{{Type: "dummy"}},
		Notifiers: []*config.Preset{{Type: "dummy"}},
	}
}

func baseUpdater(t *testing.T) *ResolvedUpdater {
	t.Helper()

	return &ResolvedUpdater{
		Name: "test-updater",
		Provider: ResolvedDriver{
			Type:   "dummy",
			Config: &testutil.DummyProviderConfig{Protocols: config.DefaultProtocols()},
		},
		Resolvers: []ResolvedDriver{{Type: "dummy", Config: &testutil.DummyResolverConfig{Protocols: config.DefaultProtocols()}}},
		Notifiers: []ResolvedDriver{{Type: "dummy", Config: &testutil.DummyNotifierConfig{}}},
	}
}

func newValidUpdaterCfg(t *testing.T, name string) *config.Updater {
	t.Helper()
	u := baseUpdaterCfg(t)
	u.Name = name
	return u
}

func newValidConfig(t *testing.T, updaters ...*config.Updater) *config.Config {
	t.Helper()
	if len(updaters) == 0 {
		updaters = []*config.Updater{newValidUpdaterCfg(t, "test-updater")}
	}
	return &config.Config{
		Interval:  15 * time.Minute,
		Protocols: config.DefaultProtocols(),
		Updaters:  updaters,
	}
}

func TestResolvedDriver_Fingerprint(t *testing.T) {
	t.Parallel()

	t.Run("same type and config produce the same fingerprint", func(t *testing.T) {
		t.Parallel()

		d1 := &ResolvedDriver{Type: "dummy", Config: &testutil.DummyProviderConfig{Protocols: config.DefaultProtocols()}}
		d2 := &ResolvedDriver{Type: "dummy", Config: &testutil.DummyProviderConfig{Protocols: config.DefaultProtocols()}}

		fp1, err := d1.fingerprint()
		require.NoError(t, err)
		fp2, err := d2.fingerprint()
		require.NoError(t, err)

		assert.Equal(t, fp1, fp2)
		assert.NotEqual(t, zeroFingerprint, fp1)
	})

	t.Run("different type produces different fingerprint", func(t *testing.T) {
		t.Parallel()

		d1 := &ResolvedDriver{Type: "dummy-a"}
		d2 := &ResolvedDriver{Type: "dummy-b"}

		fp1, err := d1.fingerprint()
		require.NoError(t, err)
		fp2, err := d2.fingerprint()
		require.NoError(t, err)

		assert.NotEqual(t, fp1, fp2)
	})

	t.Run("different config produces different fingerprint", func(t *testing.T) {
		t.Parallel()

		d1 := &ResolvedDriver{Type: "dummy", Config: &testutil.DummyProviderConfig{Protocols: config.DefaultProtocols()}}
		d2 := &ResolvedDriver{Type: "dummy", Config: &testutil.DummyProviderConfig{}}

		fp1, err := d1.fingerprint()
		require.NoError(t, err)
		fp2, err := d2.fingerprint()
		require.NoError(t, err)

		assert.NotEqual(t, fp1, fp2)
	})

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()

		d := &ResolvedDriver{Type: "dummy"}

		fp1, err := d.fingerprint()
		require.NoError(t, err)
		fp2, err := d.fingerprint()
		require.NoError(t, err)

		assert.Equal(t, fp1, fp2)
	})

	t.Run("empty type and nil config differ from any populated driver", func(t *testing.T) {
		t.Parallel()

		empty := &ResolvedDriver{}
		fp, err := empty.fingerprint()
		require.NoError(t, err)

		populated := &ResolvedDriver{Type: "dummy"}
		fp2, err := populated.fingerprint()
		require.NoError(t, err)

		assert.NotEqual(t, fp, fp2)
	})
}

func TestAppendErrWithPath(t *testing.T) {
	t.Parallel()

	out := appendErrWithPath(nil, "some > path", assert.AnError)

	require.Len(t, out, 1)
	assert.ErrorIs(t, out[0], assert.AnError)
	assert.Contains(t, out[0].Error(), "some > path")
}

func TestAppendErrsWithPath(t *testing.T) {
	t.Parallel()

	t.Run("appends wrapped errors when optscumulator already has entries", func(t *testing.T) {
		t.Parallel()

		e1 := errors.New("first")
		e2 := errors.New("second")

		optsc := []error{assert.AnError}
		out := appendErrsWithPath(optsc, "path", []error{e1, e2})

		require.Len(t, out, 3)
		assert.ErrorIs(t, out[1], e1)
		assert.ErrorIs(t, out[2], e2)
		assert.Contains(t, out[1].Error(), "path")
	})

	t.Run("no-op when newErrs is empty", func(t *testing.T) {
		t.Parallel()

		optsc := []error{assert.AnError}
		out := appendErrsWithPath(optsc, "path", nil)
		assert.Equal(t, optsc, out)
	})
}

func TestConfigResolver_resolveDriver(t *testing.T) {
	t.Parallel()

	t.Run("both type and use set", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Type: "dummy", Use: "something"}
		cr := newCR(t, cfg)

		_, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

		assert.ErrorIs(t, err, ErrTypeAndUseSpecifiedTogether)
	})

	t.Run("both type and use set", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Type: "", Use: ""}
		cr := newCR(t, cfg)

		_, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

		assert.ErrorIs(t, err, ErrTypeAndUseNotSpecified)
	})

	t.Run("both type and use set", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Type: "does-not-exist"}
		cr := newCR(t, cfg)

		_, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

		assert.ErrorIs(t, err, ErrTypeDoesNotExist)
	})

	t.Run("set protocols", func(t *testing.T) {
		t.Parallel()

		newProtocol := func() *config.Protocols {
			return &config.Protocols{IPv4: true, IPv6: false}
		}

		t.Run("preset", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Updaters[0].Provider.Protocols = newProtocol()
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, *newProtocol(), resolved.Config.(config.ProtocolAware).GetProtocols())
		})

		t.Run("updater", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Updaters[0].Protocols = newProtocol()
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, *newProtocol(), resolved.Config.(config.ProtocolAware).GetProtocols())
		})

		t.Run("global", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Protocols = *newProtocol()
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, *newProtocol(), resolved.Config.(config.ProtocolAware).GetProtocols())
		})

		t.Run("default", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Protocols = config.Protocols{}
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, config.DefaultProtocols(), resolved.Config.(config.ProtocolAware).GetProtocols())
		})
	})

	t.Run("set proxy", func(t *testing.T) {
		t.Parallel()

		newProxy := func() *config.Proxy {
			return &config.Proxy{URL: "http://example.com", Username: "user", Password: "pass"}
		}

		t.Run("preset", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Updaters[0].Provider.Proxy = newProxy()
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, *newProxy(), resolved.Config.(config.ProxyAware).GetProxy())
			assert.Equal(t, "dummy", resolved.Type)
		})

		t.Run("updater preset", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Updaters[0].Proxy = newProxy()
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, *newProxy(), resolved.Config.(config.ProxyAware).GetProxy())
		})

		t.Run("global preset", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Proxy = *newProxy()
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, *newProxy(), resolved.Config.(config.ProxyAware).GetProxy())
		})

		t.Run("default", func(t *testing.T) {
			t.Parallel()

			cfg := newValidConfig(t)
			cfg.Proxy = config.Proxy{}
			cr := newCR(t, cfg)

			resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

			require.NoError(t, err)
			assert.Equal(t, config.Proxy{}, resolved.Config.(config.ProxyAware).GetProxy())
		})
	})

	t.Run("merge global preset", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Use: "global-dummy"}
		cfg.Presets.Providers = map[string]*config.Preset{
			"global-dummy": {Type: "dummy", Protocols: new(config.DefaultProtocols())},
		}
		cr := newCR(t, cfg)

		resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

		assert.NoError(t, err)
		assert.Equal(t, "dummy", resolved.Type)
	})

	t.Run("decode extra config", func(t *testing.T) {
		t.Parallel()

		presetExtra, err := yaml.ValueToNode(testutil.ExtraConfig{Value: 42})
		require.NoError(t, err)

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Type: "dummy", Extra: presetExtra}
		cr := newCR(t, cfg)

		resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

		require.NoError(t, err)
		assert.Equal(t, "dummy", resolved.Type)
		assert.Equal(t, 42, resolved.Config.(*testutil.DummyProviderConfig).Value)
		assert.Equal(t, "", resolved.Config.(*testutil.DummyProviderConfig).Value2)
	})

	t.Run("decode and merge extra config", func(t *testing.T) {
		t.Parallel()

		globalExtra, err := yaml.ValueToNode(testutil.ExtraConfig{Value: 42})
		require.NoError(t, err)
		require.NoError(t, err)
		presetExtra, err := yaml.ValueToNode(testutil.ExtraConfig{Value2: "test"})
		require.NoError(t, err)

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Use: "global-dummy", Extra: presetExtra}
		cfg.Presets.Providers = map[string]*config.Preset{
			"global-dummy": {Type: "dummy", Extra: globalExtra},
		}
		cr := newCR(t, cfg)

		resolved, err := cr.resolveDriver(cfg.Updaters[0].Provider, cfg.Updaters[0], cfg.Presets.Providers, cr.providerGetter)

		require.NoError(t, err)
		assert.Equal(t, "dummy", resolved.Type)
		assert.Equal(t, 42, resolved.Config.(*testutil.DummyProviderConfig).Value)
		assert.Equal(t, "test", resolved.Config.(*testutil.DummyProviderConfig).Value2)
	})
}

func TestConfigResolver_ResolveUpdater(t *testing.T) {
	t.Parallel()

	t.Run("resolve a fully valid updater", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cr := newCR(t, cfg)

		resolved, errs := cr.resolveUpdater(cfg.Updaters[0], 0)

		require.Empty(t, errs)
		assert.Equal(t, "test-updater", resolved.Name)
		assert.Equal(t, "dummy", resolved.Provider.Type)
		require.Len(t, resolved.Resolvers, 1)
		assert.Equal(t, "dummy", resolved.Resolvers[0].Type)
		require.Len(t, resolved.Notifiers, 1)
		assert.Equal(t, "dummy", resolved.Notifiers[0].Type)
	})

	t.Run("provider preset with neither type nor use errors", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{}
		cr := newCR(t, cfg)

		_, errs := cr.resolveUpdater(cfg.Updaters[0], 0)

		require.NotEmpty(t, errs)
		assert.ErrorIs(t, errors.Join(errs...), ErrTypeAndUseNotSpecified)
	})

	t.Run("provider preset referencing a missing global preset errors", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Use: "does-not-exist"}
		cr := newCR(t, cfg)

		_, errs := cr.resolveUpdater(cfg.Updaters[0], 0)

		require.NotEmpty(t, errs)
		assert.ErrorIs(t, errors.Join(errs...), ErrPresetDoesNotExist)
	})

	t.Run("provider preset with unregistered type errors", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Provider = &config.Preset{Type: "does-not-exist"}
		cr := newCR(t, cfg)

		_, errs := cr.resolveUpdater(cfg.Updaters[0], 0)

		require.NotEmpty(t, errs)
		assert.ErrorIs(t, errors.Join(errs...), ErrTypeDoesNotExist)
	})

	t.Run("missing resolvers/notifiers are omitted", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Resolvers = nil
		cfg.Updaters[0].Notifiers = nil
		cr := newCR(t, cfg)

		resolved, errs := cr.resolveUpdater(cfg.Updaters[0], 0)

		assert.Empty(t, errs)
		assert.Empty(t, resolved.Resolvers)
		assert.Empty(t, resolved.Notifiers)
	})

	t.Run("bad resolver preset", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Resolvers = []*config.Preset{{}}
		cr := newCR(t, cfg)

		resolved, errs := cr.resolveUpdater(cfg.Updaters[0], 0)

		require.NotEmpty(t, errs)
		assert.Equal(t, "dummy", resolved.Provider.Type)
		assert.Empty(t, resolved.Resolvers)
	})
}

func TestConfigResolver_ValidateUpdater(t *testing.T) {
	t.Parallel()

	t.Run("valid updater", func(t *testing.T) {
		t.Parallel()

		updater := baseUpdater(t)

		errs := validateUpdater(updater, 0)

		assert.Empty(t, errs)
	})

	t.Run("missing name", func(t *testing.T) {
		t.Parallel()

		updater := baseUpdater(t)
		updater.Name = ""

		errs := validateUpdater(updater, 0)

		require.NotEmpty(t, errs)
		assert.ErrorIs(t, errors.Join(errs...), ErrUpdaterNoName)
	})

	t.Run("no resolvers", func(t *testing.T) {
		t.Parallel()

		updater := baseUpdater(t)
		updater.Resolvers = nil

		errs := validateUpdater(updater, 0)

		require.NotEmpty(t, errs)
		assert.ErrorIs(t, errors.Join(errs...), ErrNoResolversDefined)
	})

	t.Run("empty protocols on a protocol-aware driver", func(t *testing.T) {
		t.Parallel()

		updater := baseUpdater(t)
		updater.Provider.Config = &testutil.DummyProviderConfig{} // zero-value Protocols

		errs := validateUpdater(updater, 0)

		require.NotEmpty(t, errs)
		assert.ErrorIs(t, errors.Join(errs...), ErrNoProtocols)
	})

	t.Run("provider reqire protocol", func(t *testing.T) {
		t.Parallel()

		for _, tt := range []struct {
			ProviderProto config.Protocols
			ResolverProto []config.Protocols
			Error         error
		}{
			{ProviderProto: config.Protocols{IPv4: true, IPv6: false}, ResolverProto: []config.Protocols{{IPv4: false, IPv6: true}}, Error: ErrNoProtocols},
			{ProviderProto: config.Protocols{IPv4: false, IPv6: true}, ResolverProto: []config.Protocols{{IPv4: true, IPv6: false}}, Error: ErrNoProtocols},
			{ProviderProto: config.Protocols{IPv4: true, IPv6: true}, ResolverProto: []config.Protocols{{IPv4: false, IPv6: false}}, Error: ErrNoProtocols},
			{ProviderProto: config.Protocols{IPv4: true, IPv6: false}, ResolverProto: []config.Protocols{{IPv4: true, IPv6: false}}, Error: nil},
			{ProviderProto: config.Protocols{IPv4: false, IPv6: true}, ResolverProto: []config.Protocols{{IPv4: false, IPv6: true}}, Error: nil},
			{ProviderProto: config.Protocols{IPv4: true, IPv6: true}, ResolverProto: []config.Protocols{{IPv4: false, IPv6: true}, {IPv4: true, IPv6: false}}, Error: nil},
		} {
			t.Run(tt.ProviderProto.String()+" vs "+tt.ResolverProto[0].String(), func(t *testing.T) {
				t.Parallel()

				updater := baseUpdater(t)
				updater.Provider.Config.(*testutil.DummyProviderConfig).Protocols = tt.ProviderProto
				updater.Resolvers = make([]ResolvedDriver, 0, len(tt.ResolverProto))
				for _, proto := range tt.ResolverProto {
					updater.Resolvers = append(updater.Resolvers, ResolvedDriver{
						Type:   "dummy",
						Config: &testutil.DummyResolverConfig{Protocols: proto},
					})
				}

				errs := validateUpdater(updater, 0)

				if tt.Error == nil {
					assert.Empty(t, errs)
				} else {
					require.NotEmpty(t, errs)
					assert.ErrorIs(t, errors.Join(errs...), ErrProtocolMismatch)
				}
			})
		}
	})
}

func TestConfigResolver_ValidateGlobal(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()

		cr := newCR(t, newValidConfig(t))
		errs := cr.validateGlobal()
		assert.Empty(t, errs)
	})

	t.Run("interval below the minimum", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Interval = 1 * time.Second
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrUpdateIntervalTooShort)
	})

	t.Run("empty global protocols", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Protocols = config.Protocols{}
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrNoProtocols)
	})

	t.Run("no updaters", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters = nil
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrNoUpdatersDefined)
	})

	t.Run("global preset using 'use'", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Presets.Providers = map[string]*config.Preset{
			"bad": {Use: "other", Protocols: new(config.DefaultProtocols())},
		}
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrGlobalPresetsCannotUseUse)
	})

	t.Run("global preset without type", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Presets.Resolvers = map[string]*config.Preset{
			"bad": {},
		}
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrDriverNoTypeDefined)
	})

	t.Run("global preset with an unregistered type", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Presets.Notifiers = map[string]*config.Preset{
			"bad": {Type: "does-not-exist", Protocols: new(config.DefaultProtocols())},
		}
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrTypeDoesNotExist)
	})

	t.Run("nil global preset entry", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Presets.Providers = map[string]*config.Preset{
			"bad": nil,
		}
		cr := newCR(t, cfg)

		errs := cr.validateGlobal()

		assert.ErrorIs(t, errors.Join(errs...), ErrPresetIsNil)
	})

	t.Run("protocols not supported", func(t *testing.T) {
		t.Parallel()

		preset := &config.Preset{Type: "dummy", Protocols: &config.Protocols{IPv4: true, IPv6: false}}
		type tConfig struct{}

		errs := validateUpdaterPreset(preset, "path", func(name string) (registry.Entry[provider.Provider], bool) {
			return registry.Entry[provider.Provider]{
				Config: func() any {
					return &tConfig{}
				},
			}, true
		})

		assert.ErrorIs(t, errors.Join(errs...), ErrProtocolsNotSupported)
	})

	t.Run("proxy not supported", func(t *testing.T) {
		t.Parallel()

		preset := &config.Preset{Type: "dummy", Proxy: &config.Proxy{URL: "http://example.com"}}
		type tConfig struct{}

		errs := validateUpdaterPreset(preset, "path", func(name string) (registry.Entry[provider.Provider], bool) {
			return registry.Entry[provider.Provider]{
				Config: func() any {
					return &tConfig{}
				},
			}, true
		})

		assert.ErrorIs(t, errors.Join(errs...), ErrProxyNotSupported)
	})
}

func TestConfigResolver_ValidateGlobalPresets(t *testing.T) {
	t.Parallel()

	t.Run("no protocols", func(t *testing.T) {
		t.Parallel()

		errs := validateGlobalPresets(
			"providers",
			map[string]*config.Preset{
				"bad": {Type: "dummy", Protocols: new(config.Protocols{})},
			},
			testutil.ProviderRegistry(t).Get,
		)

		assert.ErrorIs(t, errors.Join(errs...), ErrNoProtocols)
	})

	t.Run("protocols not supported", func(t *testing.T) {
		t.Parallel()

		type tConfig struct{}

		errs := validateGlobalPresets(
			"providers",
			map[string]*config.Preset{
				"bad": {Type: "dummy", Protocols: new(config.Protocols{})},
			},
			func(name string) (registry.Entry[provider.Provider], bool) {
				return registry.Entry[provider.Provider]{
					Config: func() any {
						return &tConfig{}
					},
				}, true
			},
		)

		assert.ErrorIs(t, errors.Join(errs...), ErrProtocolsNotSupported)
	})

	t.Run("proxy not supported", func(t *testing.T) {
		t.Parallel()

		type tConfig struct{}

		errs := validateGlobalPresets(
			"providers",
			map[string]*config.Preset{
				"bad": {Type: "dummy", Proxy: &config.Proxy{URL: "http://example.com"}},
			},
			func(name string) (registry.Entry[provider.Provider], bool) {
				return registry.Entry[provider.Provider]{
					Config: func() any {
						return &tConfig{}
					},
				}, true
			},
		)

		assert.ErrorIs(t, errors.Join(errs...), ErrProxyNotSupported)
	})
}

func TestConfigResolver_ValidateUpdaterConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid updater config", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cr := newCR(t, cfg)

		errs := cr.validateUpdaterConfig(cfg.Updaters[0], 0)
		assert.Empty(t, errs)
	})

	t.Run("empty updater protocols", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Updaters[0].Protocols = new(config.Protocols{})
		cr := newCR(t, cfg)

		errs := cr.validateUpdaterConfig(cfg.Updaters[0], 0)

		assert.ErrorIs(t, errors.Join(errs...), ErrNoProtocols)
	})
}

func TestConfigResolver_ValidateUpdaterPreset(t *testing.T) {
	t.Parallel()

	t.Run("valid preset", func(t *testing.T) {
		t.Parallel()

		preset := &config.Preset{Type: "dummy"}

		errs := validateUpdaterPreset(preset, "path", testutil.ProviderRegistry(t).Get)
		assert.Empty(t, errs)
	})

	t.Run("empty protocols on the preset", func(t *testing.T) {
		t.Parallel()

		preset := &config.Preset{Type: "dummy", Protocols: new(config.Protocols{})}

		errs := validateUpdaterPreset(preset, "path", testutil.ProviderRegistry(t).Get)

		assert.ErrorIs(t, errors.Join(errs...), ErrNoProtocols)
	})

	t.Run("protocols not supported", func(t *testing.T) {
		t.Parallel()

		preset := &config.Preset{Type: "dummy", Protocols: &config.Protocols{IPv4: true, IPv6: false}}

		type tConfig struct{}

		errs := validateUpdaterPreset(preset, "path", func(name string) (registry.Entry[provider.Provider], bool) {
			return registry.Entry[provider.Provider]{
				Config: func() any {
					return &tConfig{}
				},
			}, true
		})

		assert.ErrorIs(t, errors.Join(errs...), ErrProtocolsNotSupported)
	})

	t.Run("proxy not supported", func(t *testing.T) {
		t.Parallel()

		preset := &config.Preset{Type: "dummy", Proxy: &config.Proxy{URL: "http://example.com"}}

		type tConfig struct{}

		errs := validateUpdaterPreset(preset, "path", func(name string) (registry.Entry[provider.Provider], bool) {
			return registry.Entry[provider.Provider]{
				Config: func() any {
					return &tConfig{}
				},
			}, true
		})

		assert.ErrorIs(t, errors.Join(errs...), ErrProxyNotSupported)
	})
}

func TestConfigResolver_ValidateAndResolve(t *testing.T) {
	t.Parallel()

	t.Run("valid single-updater", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cr := newCR(t, cfg)

		resolved, errs := cr.ValidateAndResolve()

		require.Empty(t, errs)
		require.NotNil(t, resolved)
		require.Len(t, resolved.Updaters, 1)
		assert.Equal(t, "test-updater", resolved.Updaters[0].Name)
		assert.Equal(t, cfg.Interval, resolved.Interval)
	})

	t.Run("duplicate updater names", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t,
			newValidUpdaterCfg(t, "dup"),
			newValidUpdaterCfg(t, "dup"),
		)
		cfg.Updaters[1].Provider.Extra = nil
		cr := newCR(t, cfg)

		_, errs := cr.ValidateAndResolve()

		count := 0
		for _, err := range errs {
			if errors.Is(err, ErrUpdaterShareName) {
				count++
			}
		}
		assert.Equal(t, 2, count, "expected both duplicate updaters to be flagged, got errs: %v", errs)
	})

	t.Run("updater share an identical provider", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t,
			newValidUpdaterCfg(t, "updater-a"),
			newValidUpdaterCfg(t, "updater-b"),
		)
		cr := newCR(t, cfg)

		_, errs := cr.ValidateAndResolve()

		count := 0
		for _, err := range errs {
			if errors.Is(err, ErrUpdaterShareProvider) {
				count++
			}
		}
		assert.Equal(t, 2, count, "expected both updaters sharing a provider fingerprint to be flagged, got errs: %v", errs)
	})

	t.Run("global validation errors are surfoptsed", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.Interval = 1 * time.Second
		cr := newCR(t, cfg)

		_, errs := cr.ValidateAndResolve()

		assert.ErrorIs(t, errors.Join(errs...), ErrUpdateIntervalTooShort)
	})

	t.Run("one updater fails to resolve", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t,
			newValidUpdaterCfg(t, "good"),
			newValidUpdaterCfg(t, "bad"),
		)
		cfg.Updaters[1].Provider = &config.Preset{Type: "does-not-exist"}
		cr := newCR(t, cfg)

		resolved, _ := cr.ValidateAndResolve()

		require.NotNil(t, resolved)
		require.Len(t, resolved.Updaters, 1)
		assert.Equal(t, "good", resolved.Updaters[0].Name)
	})
}

func TestResolveConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		opts := newAppOptions().apply(WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))

		resolved, errs := resolveConfig(*cfg, opts)

		require.Empty(t, errs)
		require.NotNil(t, resolved)
		require.Len(t, resolved.Updaters, 1)
	})

	t.Run("valid opts.LogLevel override", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		opts := &appOptions{LogLevel: "debug"}
		WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t))(opts)

		resolved, errs := resolveConfig(*cfg, opts)

		require.Empty(t, errs)
		require.NotNil(t, resolved)
		assert.Equal(t, "debug", resolved.LogLevel)
	})

	t.Run("invalid opts.LogLevel override", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		opts := &appOptions{LogLevel: "not-a-real-level"}
		WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t))(opts)

		_, errs := resolveConfig(*cfg, opts)

		assert.ErrorIs(t, errors.Join(errs...), ErrInvalidLogLevel)
	})

	t.Run("not mutating original config", func(t *testing.T) {
		t.Parallel()

		cfg := newValidConfig(t)
		cfg.LogLevel = "info"
		opts := &appOptions{LogLevel: "debug"}
		WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t))(opts)

		resolved, errs := resolveConfig(*cfg, opts)

		require.Empty(t, errs)
		assert.Equal(t, "info", cfg.LogLevel)
		assert.Equal(t, "debug", resolved.LogLevel)
	})
}
