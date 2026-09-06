package provider

import (
	"context"
	"iter"

	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/registry"
)

// Provider is an interface for DNS provider.
type Provider interface {
	// Update updates the DNS records with the provided IPs.
	// The IPs may be used without validation; at least one IP must be set.
	Update(ctx context.Context, ips lib.IPs) (err error)
}

var defaultRegistry = registry.NewRegistry[Provider]() //nolint:gochecknoglobals

// Register registers a provider by name. Returns an error if registration fails.
func Register[T any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item T, err error)) error {
	return defaultRegistry.Register(name, newFn)
}

// MustRegister registers a provider by name and panics if registration fails.
func MustRegister[T any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item T, err error)) {
	err := Register(name, newFn)
	if err != nil {
		panic(err)
	}
}

// Get retrieves a registered provider by name. Returns the provider and true if found, or nil and false if not found.
func Get(name string) (registry.Entry[Provider], bool) {
	return defaultRegistry.Get(name)
}

// List returns a sequence of all registered provider names and their corresponding registry entries.
func List() iter.Seq2[string, registry.Entry[Provider]] {
	return defaultRegistry.Keys()
}
