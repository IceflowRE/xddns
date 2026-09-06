package netif_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/assert/yaml"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/resolver"
	"github.com/iceflowre/xddns/xddns/resolver/netif"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	t.Run("implements resolver.Resolver", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(resolver.Resolver), new(netif.Resolver))
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal yaml", func(t *testing.T) {
		t.Parallel()

		t.Run("valid config", func(t *testing.T) {
			t.Parallel()

			var cfg netif.Config
			err := yaml.Unmarshal([]byte(`
interface:
  - eth0
  - en0`), &cfg)

			require.NoError(t, err)
			assert.Equal(t, lib.StringSlice([]string{"eth0", "en0"}), cfg.Interface)
		})
	})
}
