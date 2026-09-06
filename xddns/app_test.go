package xddns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal/testutil"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/resolver"
)

func baseAppConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg := config.NewConfig()
	cfg.Updaters = []*config.Updater{
		{
			Name:     "test-updater",
			Provider: &config.Preset{Type: "dummy"},
			Resolvers: []*config.Preset{
				{Type: "dummy"},
			},
		},
	}

	return cfg
}

func newTestApp(t *testing.T) (*App, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	cfg := baseAppConfig(t)
	app, err := NewApp(*cfg, WithRegistries(testutil.NotifierRegistry(t), testutil.ProviderRegistry(t), testutil.ResolverRegistry(t)))
	require.NoError(t, err)
	app.logger = zerolog.New(&buf)

	return app, &buf
}

func newDummyProtocolDriver[T any](t *testing.T, name string, driver T) namedProtocolDriver[T] {
	t.Helper()

	named := namedProtocolDriver[T]{
		Name:   name,
		Driver: driver,
	}

	// set protocols from the config if they are not already defined
	switch driverT := any(driver).(type) {
	case *testutil.DummyProvider:
		if named.Protocols.IsEmpty() {
			named.Protocols = driverT.Cfg.Protocols
		}
		if named.Protocols.IsEmpty() {
			named.Protocols = config.DefaultProtocols()
		}
	case *testutil.DummyResolver:
		if named.Protocols.IsEmpty() {
			named.Protocols = driverT.Cfg.Protocols
			named.Protocols.IPv4 = driverT.Cfg.IPv4.IsValid()
			named.Protocols.IPv6 = driverT.Cfg.IPv6.IsValid()
		}
		if named.Protocols.IsEmpty() {
			named.Protocols = config.DefaultProtocols()
		}
	default:
		t.Fatalf("test method only supports dummies")
	}

	return named
}

func TestResolveIPs(t *testing.T) {
	t.Parallel()

	t.Run("first success wins", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		failing, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{Err: assert.AnError})
		require.NoError(t, err)
		succeeding, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("1.2.3.4")})
		require.NoError(t, err)
		neverCalled, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("9.9.9.9")})
		require.NoError(t, err)

		resolvers := []namedProtocolDriver[resolver.Resolver]{
			newDummyProtocolDriver[resolver.Resolver](t, "failing", failing),
			newDummyProtocolDriver[resolver.Resolver](t, "succeeding", succeeding),
			newDummyProtocolDriver[resolver.Resolver](t, "never-called", neverCalled),
		}
		cache := map[resolver.Resolver]resolverResult{}

		ips, err := app.resolveIPs(context.Background(), resolvers, config.Protocols{IPv4: true}, cache)

		require.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("1.2.3.4"), ips.IPv4)
		assert.Equal(t, uint32(1), failing.Calls.Load())
		assert.Equal(t, uint32(1), succeeding.Calls.Load())
		assert.Equal(t, uint32(0), neverCalled.Calls.Load(), "resolver after the first success should never be tried")
	})

	t.Run("all fail", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		r1, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{Err: assert.AnError})
		require.NoError(t, err)
		r2, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{Err: assert.AnError})
		require.NoError(t, err)

		resolvers := []namedProtocolDriver[resolver.Resolver]{
			newDummyProtocolDriver[resolver.Resolver](t, "r1", r1),
			newDummyProtocolDriver[resolver.Resolver](t, "r2", r2),
		}
		cache := map[resolver.Resolver]resolverResult{}
		ips, err := app.resolveIPs(context.Background(), resolvers, config.Protocols{IPv4: true, IPv6: true}, cache)

		assert.Error(t, err)
		assert.Equal(t, lib.IPs{}, ips)
		assert.Equal(t, uint32(1), r1.Calls.Load())
		assert.Equal(t, uint32(1), r2.Calls.Load())
	})

	t.Run("use cache instead of recalling", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		shared, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{Err: assert.AnError})
		require.NoError(t, err)
		final, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("5.6.7.8")})
		require.NoError(t, err)

		resolvers := []namedProtocolDriver[resolver.Resolver]{
			newDummyProtocolDriver[resolver.Resolver](t, "first-try", shared),
			newDummyProtocolDriver[resolver.Resolver](t, "second-try-same-driver", shared), // should hit cache
			newDummyProtocolDriver[resolver.Resolver](t, "third", final),
		}
		cache := map[resolver.Resolver]resolverResult{}

		ips, err := app.resolveIPs(context.Background(), resolvers, config.Protocols{IPv4: true}, cache)

		require.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("5.6.7.8"), ips.IPv4)
		assert.Equal(t, uint32(1), shared.Calls.Load(), "second reference to the same resolver should be served from cache, not re-invoked")
		assert.Equal(t, uint32(1), final.Calls.Load())
	})

	t.Run("cache resuses prior failure", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		shared, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{Err: assert.AnError})
		require.NoError(t, err)

		cache := map[resolver.Resolver]resolverResult{
			shared: {ips: lib.IPs{}, err: assert.AnError},
		}

		succeeding, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("10.0.0.1")})
		require.NoError(t, err)

		resolvers := []namedProtocolDriver[resolver.Resolver]{
			newDummyProtocolDriver[resolver.Resolver](t, "already-tried", shared),
			newDummyProtocolDriver[resolver.Resolver](t, "fresh", succeeding),
		}

		ips, err := app.resolveIPs(context.Background(), resolvers, config.Protocols{IPv4: true}, cache)
		require.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("10.0.0.1"), ips.IPv4)

		assert.Equal(t, uint32(0), shared.Calls.Load(), "pre-cached failure should not trigger a Resolve call at all")
	})

	t.Run("only ipv6 set", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		r, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv6: netip.MustParseAddr("2001:db8::1")})
		require.NoError(t, err)

		resolvers := []namedProtocolDriver[resolver.Resolver]{
			newDummyProtocolDriver[resolver.Resolver](t, "ipv4-only", r),
		}
		cache := map[resolver.Resolver]resolverResult{}

		ips, err := app.resolveIPs(context.Background(), resolvers, config.Protocols{IPv6: true}, cache)
		require.NoError(t, err)
		assert.Equal(t, netip.MustParseAddr("2001:db8::1"), ips.IPv6)
		assert.False(t, ips.IPv4.IsValid(), "IPv4 should remain zero-value when only IPv6 is configured")
	})
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	require.NoError(t, err)
	return addr
}

func TestApp_runUpdater(t *testing.T) {
	t.Parallel()

	const (
		ipv4 = "203.0.113.10"
		ipv6 = "2001:db8::1"
	)

	buildUpdater := func(resolverErr error, providerErr error) (upd *updater, prov *testutil.DummyProvider, ipv4Res *testutil.DummyResolver, ipv6Res *testutil.DummyResolver, notif *testutil.DummyNotifier) {
		prov, err := testutil.NewDummyProvider(testutil.DummyProviderConfig{Err: providerErr})
		require.NoError(t, err)
		notif, err = testutil.NewDummyNotifier(testutil.DummyNotifierConfig{})
		require.NoError(t, err)

		ipv4Res, err = testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr(ipv4), Err: resolverErr})
		require.NoError(t, err)
		ipv6Res, err = testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv6: netip.MustParseAddr(ipv6), Err: resolverErr})
		require.NoError(t, err)
		upd = &updater{
			Name: "test-updater",
			Resolvers: []namedProtocolDriver[resolver.Resolver]{
				newDummyProtocolDriver[resolver.Resolver](t, "v4", ipv4Res),
				newDummyProtocolDriver[resolver.Resolver](t, "v6", ipv6Res),
			},
			Provider: newDummyProtocolDriver[provider.Provider](t, "test-provider", prov),
			Notifiers: []namedDriver[notifier.Notifier]{
				{Name: "test-notifier", Driver: notif},
			},
		}

		return upd, prov, ipv4Res, ipv6Res, notif
	}

	t.Run("resolve failure", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(assert.AnError, nil)
		opts := &updateOptions{DryRun: false}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.ErrorIs(t, err, assert.AnError)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Len(t, notif.SentNotifications, 1)
		assert.Equal(t, notifier.ReasonFailedToResolve, notif.SentNotifications[len(notif.SentNotifications)-1].Reason)
	})

	t.Run("resolve failure dry run", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(assert.AnError, nil)
		opts := &updateOptions{DryRun: true, DryRunNotify: false}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.ErrorIs(t, err, assert.AnError)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Empty(t, notif.SentNotifications)
	})

	t.Run("resolve failure dry with DryRunNotify set", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(assert.AnError, nil)
		opts := &updateOptions{DryRun: true, DryRunNotify: true}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.ErrorIs(t, err, assert.AnError)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Len(t, notif.SentNotifications, 1)
		assert.Equal(t, notifier.ReasonFailedToResolve, notif.SentNotifications[len(notif.SentNotifications)-1].Reason)
	})

	t.Run("propagates a canceled context without special-casing it", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(context.Canceled, nil)
		opts := &updateOptions{DryRun: false}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Len(t, notif.SentNotifications, 1)
	})

	t.Run("skip update when resolved IPs is newest", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(nil, nil)
		app.lastIPs[upd] = lib.IPs{IPv4: mustAddr(t, ipv4), IPv6: mustAddr(t, ipv6)}
		opts := &updateOptions{DryRun: false}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.NoError(t, err)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Len(t, notif.SentNotifications, 0)
	})

	t.Run("dry run never calls the provider or notifier even when IPs changed", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(nil, nil)
		opts := &updateOptions{DryRun: true}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.NoError(t, err)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Len(t, notif.SentNotifications, 0)
		_, stored := app.lastIPs[upd]
		assert.False(t, stored, "lastIPs should not be updated on a dry run")
	})

	t.Run("update failure notifies and returns the error without storing lastIPs", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(nil, assert.AnError)
		opts := &updateOptions{DryRun: false}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.ErrorIs(t, err, assert.AnError)
		assert.Equal(t, uint32(1), prov.Calls)
		assert.Len(t, notif.SentNotifications, 1)
		assert.Equal(t, notifier.ReasonUpdateFailed, notif.SentNotifications[len(notif.SentNotifications)-1].Reason)
		require.NotContains(t, app.lastIPs, upd)
	})

	t.Run("successful update stores lastIPs and notifies success", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := buildUpdater(nil, nil)
		opts := &updateOptions{DryRun: false}

		err := app.runUpdater(context.Background(), opts, upd, map[resolver.Resolver]resolverResult{})

		assert.NoError(t, err)
		assert.Equal(t, uint32(1), prov.Calls)
		assert.Len(t, notif.SentNotifications, 1)
		assert.Equal(t, notifier.ReasonUpdateSucceeded, notif.SentNotifications[len(notif.SentNotifications)-1].Reason)
		assert.Equal(t, lib.IPs{IPv4: mustAddr(t, ipv4), IPv6: mustAddr(t, ipv6)}, app.lastIPs[upd])
	})
}

func newFixtureUpdater(t *testing.T, name string, resolverErr error, providerErr error) (upd *updater, prov *testutil.DummyProvider, ipv4Res *testutil.DummyResolver, ipv6Res *testutil.DummyResolver, notif *testutil.DummyNotifier) {
	t.Helper()

	prov, err := testutil.NewDummyProvider(testutil.DummyProviderConfig{Err: providerErr})
	require.NoError(t, err)
	notif, err = testutil.NewDummyNotifier(testutil.DummyNotifierConfig{})
	require.NoError(t, err)

	ipv4Res, err = testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("203.0.113.10"), Err: resolverErr})
	require.NoError(t, err)
	ipv6Res, err = testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv6: netip.MustParseAddr("2001:db8::1"), Err: resolverErr})
	require.NoError(t, err)

	upd = &updater{
		Name: name,
		Resolvers: []namedProtocolDriver[resolver.Resolver]{
			newDummyProtocolDriver[resolver.Resolver](t, name+"-v4", ipv4Res),
			newDummyProtocolDriver[resolver.Resolver](t, name+"-v6", ipv6Res),
		},
		Provider: newDummyProtocolDriver[provider.Provider](t, name+"-provider", prov),
		Notifiers: []namedDriver[notifier.Notifier]{
			{Name: name + "-notifier", Driver: notif},
		},
	}

	return upd, prov, ipv4Res, ipv6Res, notif
}

func TestApp_Update(t *testing.T) {
	t.Parallel()

	t.Run("update is already running", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		require.True(t, app.updateGuard.TryLock())
		defer app.updateGuard.Unlock()

		err := app.Update(context.Background())

		assert.ErrorIs(t, err, ErrUpdateInProgress)
	})

	t.Run("run every updater", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)

		err1 := errors.New("first updater boom")
		err2 := errors.New("second updater boom")
		upd1, prov1, _, _, _ := newFixtureUpdater(t, "u1", nil, err1)
		upd2, prov2, _, _, _ := newFixtureUpdater(t, "u2", nil, err2)
		upd3, prov3, _, _, _ := newFixtureUpdater(t, "u3", nil, nil)
		app.updaters = []*updater{upd1, upd2, upd3}

		err := app.Update(context.Background())

		require.Error(t, err)
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
		assert.Equal(t, uint32(1), prov1.Calls)
		assert.Equal(t, uint32(1), prov2.Calls)
		assert.Equal(t, uint32(1), prov3.Calls)
	})

	t.Run("stop when the context is already canceled", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, _ := newFixtureUpdater(t, "u1", nil, nil)
		app.updaters = []*updater{upd}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := app.Update(ctx)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, uint32(0), prov.Calls)
	})

	t.Run("pass DryRun through to every updater", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := newFixtureUpdater(t, "u1", nil, nil)
		app.updaters = []*updater{upd}

		err := app.Update(context.Background(), WithDryRun(true))

		require.NoError(t, err)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.Empty(t, notif.SentNotifications)
	})

	t.Run("WithDryRunNotify send notifications during a dry run", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, notif := newFixtureUpdater(t, "u1", nil, nil)
		app.updaters = []*updater{upd}

		err := app.Update(context.Background(), WithDryRun(true), WithDryRunNotify(true))

		require.NoError(t, err)
		assert.Equal(t, uint32(0), prov.Calls)
		assert.NotEmpty(t, notif.SentNotifications)
	})
}

func TestApp_daemonUpdateRun(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		app, buf := newTestApp(t)
		app.updaters[0].Resolvers[0].Driver.(*testutil.DummyResolver).Cfg.IPv4 = mustAddr(t, "1.2.3.4")
		app.updaters[0].Resolvers[0].Driver.(*testutil.DummyResolver).Cfg.IPv6 = mustAddr(t, "2001:db8::1")

		err := app.daemonUpdateRun(context.Background())

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "update completed successfully")
	})

	t.Run("log at debug", func(t *testing.T) {
		t.Parallel()

		app, buf := newTestApp(t)
		require.True(t, app.updateGuard.TryLock())
		defer app.updateGuard.Unlock()

		err := app.daemonUpdateRun(context.Background())

		assert.ErrorIs(t, err, ErrUpdateInProgress)
		assert.Contains(t, buf.String(), "skipping update, already in progress")
		assert.NotContains(t, buf.String(), `"level":"error"`)
	})

	t.Run("updater failure", func(t *testing.T) {
		t.Parallel()

		app, buf := newTestApp(t)
		upd, _, _, _, _ := newFixtureUpdater(t, "failing-updater", nil, assert.AnError)
		app.updaters = []*updater{upd}

		err := app.daemonUpdateRun(context.Background())

		assert.Error(t, err)
		assert.Contains(t, buf.String(), "update failed")
	})
}

func TestApp_RunDaemon(t *testing.T) {
	t.Parallel()

	t.Run("immediate update and stop on context cancellation", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, _, _, _ := newFixtureUpdater(t, "daemon-updater", nil, nil)
		app.updaters = []*updater{upd}
		app.updateInterval = time.Hour

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			done := make(chan error, 1)
			go func() {
				done <- app.RunDaemon(ctx)
			}()

			synctest.Wait()

			assert.Equal(t, uint32(1), prov.Calls)

			cancel()

			err := <-done
			require.ErrorIs(t, err, context.Canceled)

			assert.Equal(t, uint32(1), prov.Calls)
		})
	})

	t.Run("context already canceled", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			done := make(chan error, 1)
			go func() {
				done <- app.RunDaemon(ctx)
			}()

			err := <-done
			assert.ErrorIs(t, err, context.Canceled)
		})
	})

	t.Run("running as daemon", func(t *testing.T) {
		t.Parallel()

		app, _ := newTestApp(t)
		upd, prov, ipv4Res, _, _ := newFixtureUpdater(t, "daemon-updater", nil, nil)
		app.updaters = []*updater{upd}
		const interval = 1 * time.Second
		app.updateInterval = interval

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- app.RunDaemon(ctx)
			}()

			synctest.Wait()
			require.Equal(t, uint32(1), ipv4Res.Calls.Load())
			require.Equal(t, uint32(1), prov.Calls)

			for want := uint32(2); want <= 4; want++ {
				time.Sleep(interval)
				synctest.Wait()
				require.Equal(t, want, ipv4Res.Calls.Load())
			}

			assert.Equal(t, uint32(1), prov.Calls)

			cancel()
			err := <-done
			assert.ErrorIs(t, err, context.Canceled)
		})
	})
}

func TestIsNetworkError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped deadline exceeded", fmt.Errorf("wrapped: %w", context.DeadlineExceeded), true},
		{"net.Error", &net.DNSError{IsTimeout: true}, true},
		{"context canceled is not a network error", context.Canceled, false},
		{"generic error", assert.AnError, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isNetworkError(tt.err))
		})
	}
}

func TestResolveIPs_ContextCanceledMidLoop(t *testing.T) {
	t.Parallel()

	app, _ := newTestApp(t)
	r1, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("1.1.1.1")})
	require.NoError(t, err)
	r2, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{IPv4: netip.MustParseAddr("2.2.2.2")})
	require.NoError(t, err)

	resolvers := []namedProtocolDriver[resolver.Resolver]{
		newDummyProtocolDriver[resolver.Resolver](t, "r1", r1),
		newDummyProtocolDriver[resolver.Resolver](t, "r2", r2),
	}
	cache := map[resolver.Resolver]resolverResult{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = app.resolveIPs(ctx, resolvers, config.Protocols{IPv4: true}, cache)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, uint32(0), r1.Calls.Load())
	assert.Equal(t, uint32(0), r2.Calls.Load())
}

func TestResolveIPs_LogsCorrectLevelForFreshFailure(t *testing.T) {
	t.Parallel()

	app, buf := newTestApp(t)
	failing, err := testutil.NewDummyResolver(testutil.DummyResolverConfig{Err: assert.AnError})
	require.NoError(t, err)

	resolvers := []namedProtocolDriver[resolver.Resolver]{
		newDummyProtocolDriver[resolver.Resolver](t, "failing", failing),
	}
	cache := map[resolver.Resolver]resolverResult{}

	_, err = app.resolveIPs(context.Background(), resolvers, config.Protocols{IPv4: true}, cache)
	require.Error(t, err)

	assert.Contains(t, buf.String(), `"level":"error"`)
}
