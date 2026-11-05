package netif

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/assert/yaml"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/resolver"
)

func TestParseInterfaceAddress(t *testing.T) {
	t.Parallel()

	t.Run("parses CIDR-formatted address", func(t *testing.T) {
		t.Parallel()

		parsedAddr, err := parseInterfaceAddress("2001:db8::1/64")

		require.NoError(t, err)
		assert.Equal(t, "2001:db8::1", parsedAddr.ip.String())
		assert.Equal(t, "ipv6", parsedAddr.protocol)
		assert.False(t, parsedAddr.temporary)
	})
}

func TestResolver(t *testing.T) {
	t.Parallel()

	t.Run("implements resolver.Resolver", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(resolver.Resolver), new(Resolver))
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal yaml", func(t *testing.T) {
		t.Parallel()

		t.Run("valid config", func(t *testing.T) {
			t.Parallel()

			var cfg Config
			err := yaml.Unmarshal([]byte(`
interface:
  - eth0
  - en0`), &cfg)

			require.NoError(t, err)
			assert.Equal(t, lib.StringSlice([]string{"eth0", "en0"}), cfg.Interface)
		})
	})
}
