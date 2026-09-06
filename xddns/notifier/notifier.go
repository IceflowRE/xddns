package notifier

import (
	"context"
	"iter"

	"github.com/rs/zerolog"

	"github.com/iceflowre/xddns/xddns/registry"
)

// Notifier is an interface for notification services.
type Notifier interface {
	Notify(ctx context.Context, notification Notification) error
}

var defaultRegistry = registry.NewRegistry[Notifier]() //nolint:gochecknoglobals

// Register registers a notifier by name. Returns an error if registration fails.
func Register[T any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item T, err error)) error {
	return defaultRegistry.Register(name, newFn)
}

// MustRegister registers a notifier by name and panics if registration fails.
func MustRegister[T any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item T, err error)) {
	err := Register(name, newFn)
	if err != nil {
		panic(err)
	}
}

// Get retrieves a registered notifier by name. Returns the notifier and true if found, or nil and false if not found.
func Get(name string) (registry.Entry[Notifier], bool) {
	return defaultRegistry.Get(name)
}

// List returns a sequence of all registered notifier names and their corresponding registry entries.
func List() iter.Seq2[string, registry.Entry[Notifier]] {
	return defaultRegistry.Keys()
}
