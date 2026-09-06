package registry

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testInterface interface {
	Value() string
}

type testImpl struct {
	V string
}

func (i *testImpl) Value() string { return i.V }

type testImplConfig struct {
	V string
}

func newTestImpl(cfg testImplConfig, logger zerolog.Logger) (*testImpl, error) {
	return &testImpl{V: cfg.V}, nil
}

func newTestImplErr(cfg testImplConfig, logger zerolog.Logger) (*testImpl, error) {
	return nil, errors.New("constructor error")
}

type notImpl struct{}

type notImplConfig struct{}

func newNotImpl(cfg notImplConfig, logger zerolog.Logger) (*notImpl, error) {
	return &notImpl{}, nil
}

func TestRegistry_New(t *testing.T) {
	t.Parallel()

	t.Run("creates empty registry", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()

		require.NotNil(t, r)
		assert.NotNil(t, r.items)
		assert.Empty(t, r.items)
	})
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()

		err := r.Register("test", newTestImpl)
		require.NoError(t, err)
		entry, ok := r.Get("test")

		require.True(t, ok)
		assert.NotNil(t, entry.Config)
		assert.NotNil(t, entry.New)
	})

	t.Run("duplicate name", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		err := r.Register("test", newTestImpl)
		require.NoError(t, err)
		err = r.Register("test", newTestImpl)

		assert.ErrorIs(t, err, ErrAlreadyRegistered)
	})

	t.Run("type constraint violation", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		err := r.Register("bad", newNotImpl)

		assert.ErrorIs(t, err, ErrDoesNotImplement)
	})

	t.Run("registry is using pointer type", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[*testImpl]()
		err := r.Register("test", newTestImpl)

		assert.NoError(t, err)
	})

	t.Run("register interface", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		err := r.Register("test", func(cfg testImplConfig, logger zerolog.Logger) (testInterface, error) {
			return newTestImpl(cfg, logger)
		})

		assert.NoError(t, err)
	})

	t.Run("nil constructor", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		var fn func(cfg testImplConfig, logger zerolog.Logger) (testInterface, error)
		err := r.Register("test", fn)

		assert.ErrorIs(t, err, ErrNilConstructor)
	})

	t.Run("config is pointer", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		err := r.Register("test", func(cfg *testImplConfig, logger zerolog.Logger) (testInterface, error) {
			return newTestImpl(*cfg, logger)
		})

		assert.ErrorIs(t, err, ErrInvalidConfigType)
	})
}

func TestRegistry_Get(t *testing.T) {
	t.Parallel()

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		err := r.Register("test", newTestImpl)
		require.NoError(t, err)
		entry, ok := r.Get("test")

		require.True(t, ok)
		assert.NotNil(t, entry.Config)
		assert.NotNil(t, entry.New)
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		entry, ok := r.Get("missing")

		require.False(t, ok)
		assert.Zero(t, entry)
	})
}

func TestRegistry_Keys(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()

		count := 0
		for range r.Keys() {
			count++
		}

		assert.Zero(t, count)
	})

	t.Run("multiple entries", func(t *testing.T) {
		t.Parallel()

		r := NewRegistry[testInterface]()
		err := r.Register("a", newTestImpl)
		require.NoError(t, err)
		err = r.Register("b", newTestImpl)
		require.NoError(t, err)

		collected := make(map[string]Entry[testInterface])
		for name, entry := range r.Keys() {
			collected[name] = entry
		}

		assert.Len(t, collected, 2)
		assert.Contains(t, collected, "a")
		assert.Contains(t, collected, "b")
	})
}

func TestEntry_Config(t *testing.T) {
	t.Parallel()

	t.Run("return new config", func(t *testing.T) {
		t.Parallel()

		entry := newEntry[testInterface](newTestImpl)
		cfg1 := entry.Config()
		cfg2 := entry.Config()

		require.IsType(t, &testImplConfig{}, cfg1)
		require.IsType(t, &testImplConfig{}, cfg2)
		assert.NotSame(t, cfg1, cfg2, "should return distinct pointers")
	})
}

func TestEntry_New(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		entry := newEntry[testInterface](newTestImpl)
		item, err := entry.New(&testImplConfig{V: "hello"}, zerolog.Nop())

		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, "hello", item.Value())
	})

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()

		entry := newEntry[testInterface](newTestImpl)
		_, err := entry.New(nil, zerolog.Nop())

		require.NoError(t, err)
	})

	t.Run("constructor error", func(t *testing.T) {
		t.Parallel()

		entry := newEntry[testInterface](newTestImplErr)
		_, err := entry.New(&testImplConfig{V: "whatever"}, zerolog.Nop())

		require.Error(t, err)
		assert.Equal(t, "constructor error", err.Error())
	})

	t.Run("wrong config type", func(t *testing.T) {
		t.Parallel()

		entry := newEntry[testInterface](newTestImpl)
		_, err := entry.New(string("string"), zerolog.Nop())

		assert.ErrorIs(t, err, ErrInvalidConfigType)
	})
}
