package resolver

import (
	"context"
	"errors"
	"iter"

	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/registry"
)

// Resolver is an interface for IP resolvers.
type Resolver interface {
	// Resolve resolves the current public IPs.
	// The passed protocols will never contradict the resolver's protocol configuration (if specified).
	// If an error is returned, this does not mean that no IPs were resolved; the returned IPs may be partially valid. Therefore, all returned IPs must be valid.
	Resolve(ctx context.Context, protocols config.Protocols) (ips lib.IPs, err error)
}

var ErrNotAllProtocolsResolved = errors.New("not all protocols resolved")

var defaultRegistry = registry.NewRegistry[Resolver]() //nolint:gochecknoglobals

// Register registers a resolver by name. Returns an error if registration fails.
func Register[T any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item T, err error)) error {
	return defaultRegistry.Register(name, newFn)
}

// MustRegister registers a resolver by name and panics if registration fails.
func MustRegister[T any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item T, err error)) {
	err := Register(name, newFn)
	if err != nil {
		panic(err)
	}
}

// Get retrieves a registered resolver by name. Returns the resolver and true if found, or nil and false if not found.
func Get(name string) (registry.Entry[Resolver], bool) {
	return defaultRegistry.Get(name)
}

// List returns a sequence of all registered resolver names and their corresponding registry entries.
func List() iter.Seq2[string, registry.Entry[Resolver]] {
	return defaultRegistry.Keys()
}
