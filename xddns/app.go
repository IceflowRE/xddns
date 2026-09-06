package xddns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/resolver"
)

const (
	shortUpdateInterval = 10 * time.Second
)

var (
	ErrFailedToResolveAllProtocols = errors.New("failed to resolve all requested protocols")
	ErrUpdateInProgress            = errors.New("update already in progress")
)

// App represents the main application that manages updaters, resolvers and notifiers.
type App struct {
	updateInterval time.Duration
	updaters       []*updater
	logger         zerolog.Logger

	lastIPs     map[*updater]lib.IPs
	updateGuard sync.Mutex
}

// UpdateOption is an option that configures the behavior of the Update method.
type UpdateOption func(*updateOptions)

type updateOptions struct {
	DryRun       bool
	DryRunNotify bool
}

// WithDryRun configures whether the update should be a dry run (no actual updates will be performed).
func WithDryRun(dryRun bool) UpdateOption {
	return func(cfg *updateOptions) {
		cfg.DryRun = dryRun
	}
}

// WithDryRunNotify configures whether notifications should be sent during a dry run.
func WithDryRunNotify(notify bool) UpdateOption {
	return func(cfg *updateOptions) {
		cfg.DryRunNotify = notify
	}
}

// Update resolves current IPs and pushes them to every configured updater's provider.
// Each updater's resolvers are tried in order, so a later resolver acts as a fallback if an earlier one fails.
// Resolve results are cached for the duration of this call, so updaters sharing an identical resolver config only resolve it once.
// Every updater runs even if others fail; their errors are combined and returned.
// If an update is already running, this call returns ErrUpdateInProgress immediately without doing anything.
// Returned errors are already logged, except for ErrUpdateInProgress or context cancellation, which are not logged.
func (app *App) Update(ctx context.Context, opts ...UpdateOption) error {
	if !app.updateGuard.TryLock() {
		return ErrUpdateInProgress
	}
	defer app.updateGuard.Unlock()

	cfg := &updateOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	resolverCache := make(map[resolver.Resolver]resolverResult)
	var errs []error
	for _, upd := range app.updaters {
		err := ctx.Err()
		if err != nil {
			errs = append(errs, err)

			break
		}

		err = app.runUpdater(ctx, cfg, upd, resolverCache)
		if err != nil {
			errs = append(errs, fmt.Errorf("updater %q: %w", upd.Name, err))
		}
	}

	return errors.Join(errs...)
}

// RunDaemon starts the application in daemon mode, running updates at the configured interval until the provided context is canceled.
func (app *App) RunDaemon(ctx context.Context) error {
	app.logger.Info().Dur("interval", app.updateInterval).Msg("daemon started")

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			err := app.daemonUpdateRun(ctx)
			if isNetworkError(err) {
				timer.Reset(shortUpdateInterval)
			} else {
				timer.Reset(app.updateInterval)
			}
		}
	}
}

// Returned error is already logged, except for context cancellation, which is not logged.
func (app *App) runUpdater(ctx context.Context, opts *updateOptions, upd *updater, cache map[resolver.Resolver]resolverResult) error {
	ips, err := app.resolveIPs(ctx, upd.Resolvers, upd.Provider.Protocols, cache)
	if err != nil {
		app.logger.Error().Err(err).Str("updater", upd.Name).Msg("failed to resolve IPs")
		if !opts.DryRun || opts.DryRunNotify {
			app.notify(ctx, upd.Notifiers, notifier.Notification{
				Reason:      notifier.ReasonFailedToResolve,
				UpdaterName: upd.Name,
				Error:       err,
			})
		}

		return err
	}

	last, ok := app.lastIPs[upd]
	if ok && last == ips {
		app.logger.Debug().Str("updater", upd.Name).Msg("no IP change, skipping update")

		return nil
	}

	if opts.DryRun {
		app.logger.Info().Str("updater", upd.Name).Str("ipv4", ips.IPv4.String()).Str("ipv6", ips.IPv6.String()).Msg("dry run, skipping update")
	} else {
		err = upd.Provider.Driver.Update(ctx, ips)
		if err != nil {
			app.logger.Error().Err(err).Str("updater", upd.Name).Msgf("failed to update")
			if !opts.DryRun || opts.DryRunNotify {
				app.notify(ctx, upd.Notifiers, notifier.Notification{
					Reason:      notifier.ReasonUpdateFailed,
					UpdaterName: upd.Name,
					Error:       err,
					IPs:         ips,
				})
			}

			return err
		}
		app.lastIPs[upd] = ips
		app.logger.Info().Str("updater", upd.Name).Str("ipv4", ips.IPv4.String()).Str("ipv6", ips.IPv6.String()).Msg("successfully updated")
	}

	if !opts.DryRun || opts.DryRunNotify {
		app.notify(ctx, upd.Notifiers, notifier.Notification{
			Reason:      notifier.ReasonUpdateSucceeded,
			UpdaterName: upd.Name,
			IPs:         ips,
		})
	}

	return nil
}

func (app *App) daemonUpdateRun(ctx context.Context) error {
	err := app.Update(ctx)
	if err != nil {
		if errors.Is(err, ErrUpdateInProgress) {
			app.logger.Debug().Err(err).Msg("skipping update, already in progress")
		} else {
			app.logger.Error().Err(err).Msg("update failed")
		}
	} else {
		app.logger.Debug().Msg("update completed successfully")
	}

	return err
}

type resolverResult struct {
	ips lib.IPs
	err error
}

// resolveIPs tries each resolver in turn and returns the first successful result,
// using cache to avoid re-resolving a driver already tried earlier in this Update call (whether it succeeded or failed).
// resolvedIPs will never return partially resolved IPs.
func (app *App) resolveIPs( //nolint:gocognit,funlen
	ctx context.Context,
	resolvers []namedProtocolDriver[resolver.Resolver],
	protocols config.Protocols,
	cache map[resolver.Resolver]resolverResult,
) (resolvedIPs lib.IPs, err error) {
	var errs []error
	missingProtos := protocols

	for _, res := range resolvers {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())

			break
		}
		resolveProtos := missingProtos.Conjunct(res.Protocols)
		if resolveProtos.IsEmpty() {
			continue
		}

		result, ok := cache[res.Driver]
		if !ok {
			ips, err := res.Driver.Resolve(ctx, resolveProtos)
			result = resolverResult{ips: ips, err: err}
			cache[res.Driver] = result
		}
		partialResolved := result.ips.IsOneValid() && result.err != nil

		var logEvent *zerolog.Event
		switch {
		case partialResolved:
			logEvent = app.logger.Warn()
		case result.err != nil:
			logEvent = app.logger.Error()
		default:
			logEvent = app.logger.Debug()
		}

		logEvent = logEvent.Str("resolver", res.Name).Bool("cached", ok)
		if result.ips.IPv4.IsValid() {
			resolvedIPs.IPv4 = result.ips.IPv4
			missingProtos.IPv4 = false
			logEvent = logEvent.Str("ipv4", result.ips.IPv4.String())
		}
		if result.ips.IPv6.IsValid() {
			resolvedIPs.IPv6 = result.ips.IPv6
			missingProtos.IPv6 = false
			logEvent = logEvent.Str("ipv6", result.ips.IPv6.String())
		}
		if result.err != nil {
			errs = append(errs, result.err)
			logEvent = logEvent.Err(result.err)
		}

		switch {
		case partialResolved:
			logEvent.Msg("partially resolved IPs")
		case result.err != nil:
			logEvent.Msg("failed to resolve IPs")
		default:
			logEvent.Msg("successfully resolved IPs")
		}

		if missingProtos.IsEmpty() {
			return resolvedIPs, nil
		}
	}

	errs = append(errs, fmt.Errorf("%w: %s", ErrFailedToResolveAllProtocols, missingProtos.String()))

	return lib.IPs{}, errors.Join(errs...)
}

func (app *App) notify(ctx context.Context, notifiers []namedDriver[notifier.Notifier], notification notifier.Notification) {
	for _, not := range notifiers {
		err := not.Driver.Notify(ctx, notification)
		if err != nil {
			app.logger.Error().Err(err).Str("notifier", not.Name).Msg("failed to send notification")
		}
	}
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}

	return false
}
