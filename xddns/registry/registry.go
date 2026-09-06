package registry

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"reflect"

	"github.com/rs/zerolog"
)

var (
	ErrAlreadyRegistered = errors.New("already registered")
	ErrDoesNotImplement  = errors.New("does not implement required type")
	ErrInvalidConfigType = errors.New("invalid config type")
	ErrNilConstructor    = errors.New("nil constructor")
)

// Entry represents a registered entry in the registry.
type Entry[T any] struct {
	// Config returns a pointer to a new config struct for this entry.
	Config func() any
	// New creates a new instance from the provided config.
	New func(config any, logger zerolog.Logger) (T, error)
}

// Registry is a generic registry for any type T.
type Registry[T any] struct {
	items map[string]Entry[T]
}

// NewRegistry creates a new registry for the specified type.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		items: map[string]Entry[T]{},
	}
}

// Register registers a new entry in the registry.
func (registry *Registry[T]) Register[U any, C any](name string, newFn func(cfg C, logger zerolog.Logger) (item U, err error)) error {
	if _, ok := registry.items[name]; ok {
		return fmt.Errorf("%q %w", name, ErrAlreadyRegistered)
	}

	uType := reflect.TypeFor[U]()
	tType := reflect.TypeFor[T]()
	if !uType.AssignableTo(tType) {
		return fmt.Errorf("%s %w %s", uType.String(), ErrDoesNotImplement, tType.String())
	}
	cType := reflect.TypeFor[C]()
	if cType.Kind() == reflect.Pointer {
		return fmt.Errorf("%s %w", cType.String(), ErrInvalidConfigType)
	}

	if newFn == nil {
		return fmt.Errorf("%w for %q", ErrNilConstructor, name)
	}
	registry.items[name] = newEntry[T](newFn)

	return nil
}

// Get retrieves a registered entry by name. Returns the entry and true if found, or nil and false if not found.
func (registry *Registry[T]) Get(name string) (Entry[T], bool) {
	entry, ok := registry.items[name]

	return entry, ok
}

// Keys returns a sequence of all registered names and their corresponding registry entries.
func (registry *Registry[T]) Keys() iter.Seq2[string, Entry[T]] {
	return maps.All(registry.items)
}

func newEntry[U any, T any, C any](newFn func(cfg C, logger zerolog.Logger) (T, error)) Entry[U] {
	return Entry[U]{
		Config: func() any {
			return new(C)
		},
		New: func(config any, logger zerolog.Logger) (obj U, err error) {
			var objT T
			if config == nil {
				objT, err = newFn(*new(C), logger)
			} else {
				cfg, ok := config.(*C)
				if !ok {
					return *new(U), fmt.Errorf("%w: expected %T, got %T", ErrInvalidConfigType, new(C), config)
				}
				objT, err = newFn(*cfg, logger)
			}
			if err != nil {
				return *new(U), err
			}

			return any(objT).(U), nil //nolint:forcetypeassert
		},
	}
}
