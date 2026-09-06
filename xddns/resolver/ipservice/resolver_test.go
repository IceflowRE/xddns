package ipservice_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/resolver"
	"github.com/iceflowre/xddns/xddns/resolver/ipservice"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	t.Run("implements resolver.Resolver", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(resolver.Resolver), new(ipservice.Resolver))
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal yaml", func(t *testing.T) {
		t.Parallel()

		t.Run("valid config", func(t *testing.T) {
			t.Parallel()

			var cfg ipservice.Config
			err := yaml.Unmarshal([]byte(`
url:
  - https://api.ipify.org
  - https://ifconfig.me/ip`), &cfg)

			require.NoError(t, err)
			assert.Equal(t, lib.StringSlice([]string{"https://api.ipify.org", "https://ifconfig.me/ip"}), cfg.URL)
		})
	})
}
