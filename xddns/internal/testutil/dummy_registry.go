package testutil

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/registry"
	"github.com/iceflowre/xddns/xddns/resolver"
)

// NotifierGetter is a test utility function that returns a DummyNotifier based on the provided configuration.
func NotifierGetter(cfg DummyNotifierConfig, _logger zerolog.Logger) (*DummyNotifier, error) {
	return NewDummyNotifier(cfg)
}

// ProviderGetter is a test utility function that returns a DummyProvider based on the provided configuration.
func ProviderGetter(cfg DummyProviderConfig, _logger zerolog.Logger) (*DummyProvider, error) {
	return NewDummyProvider(cfg)
}

// ResolverGetter is a test utility function that returns a DummyResolver based on the provided configuration.
func ResolverGetter(cfg DummyResolverConfig, _logger zerolog.Logger) (*DummyResolver, error) {
	return NewDummyResolver(cfg)
}

// NotifierRegistry creates a new registry for DummyNotifier and registers the NotifierGetter function.
func NotifierRegistry(t *testing.T) *registry.Registry[notifier.Notifier] {
	t.Helper()

	notifierRegistry := registry.NewRegistry[notifier.Notifier]()
	err := notifierRegistry.Register("dummy", NotifierGetter)
	require.NoError(t, err)

	return notifierRegistry
}

// ProviderRegistry creates a new registry for DummyProvider and registers the ProviderGetter function.
func ProviderRegistry(t *testing.T) *registry.Registry[provider.Provider] {
	t.Helper()

	providerRegistry := registry.NewRegistry[provider.Provider]()
	err := providerRegistry.Register("dummy", ProviderGetter)
	require.NoError(t, err)

	return providerRegistry
}

// ResolverRegistry creates a new registry for DummyResolver and registers the ResolverGetter function.
func ResolverRegistry(t *testing.T) *registry.Registry[resolver.Resolver] {
	t.Helper()

	resolverRegistry := registry.NewRegistry[resolver.Resolver]()
	err := resolverRegistry.Register("dummy", ResolverGetter)
	require.NoError(t, err)

	return resolverRegistry
}
