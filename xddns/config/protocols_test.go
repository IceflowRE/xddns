package config_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/config"
)

func TestProtocolsAwareConfig(t *testing.T) {
	t.Parallel()

	t.Run("implements ProtocolAware", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(config.ProtocolAware), new(config.ProtocolAwareConfig))
	})
}

func TestProtocols_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("valid protocols", func(t *testing.T) {
		t.Parallel()

		var protos config.Protocols
		err := yaml.Unmarshal([]byte(`
- ipv4
- ipv6`), &protos)

		assert.NoError(t, err)
		assert.Equal(t, config.Protocols{IPv4: true, IPv6: true}, protos)
	})

	t.Run("invalid protocol", func(t *testing.T) {
		t.Parallel()

		var protos config.Protocols
		err := yaml.Unmarshal([]byte(`
- invalid`), &protos)

		assert.ErrorIs(t, err, config.ErrInvalidProtocol)
	})

	t.Run("string protocol", func(t *testing.T) {
		t.Parallel()

		var protos config.Protocols
		err := yaml.Unmarshal([]byte(`ipv4`), &protos)

		assert.NoError(t, err)
		assert.Equal(t, config.Protocols{IPv4: true, IPv6: false}, protos)
	})
}
