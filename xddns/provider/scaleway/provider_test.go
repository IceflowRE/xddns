package scaleway_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/provider/scaleway"
)

func TestProvider(t *testing.T) {
	t.Parallel()

	t.Run("implements provider.Provider", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(provider.Provider), new(scaleway.Provider))
	})
}

func TestConfig_Prepare(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()

		cfg := scaleway.Config{
			Protocols: config.DefaultProtocols(),
			Domain:    []string{"example.com"},
			ProjectID: "project-id",
			AccessKey: "access-key",
			SecretKey: "secret-key",
		}
		errs := cfg.Prepare()
		assert.Empty(t, errs)
	})

	t.Run("missing required fields", func(t *testing.T) {
		t.Parallel()

		cfg := scaleway.Config{Protocols: config.DefaultProtocols()}
		errs := cfg.Prepare()
		assert.Len(t, errs,
			4)
	})

	t.Run("idompotent", func(t *testing.T) {
		t.Parallel()

		cfg := scaleway.Config{
			Protocols: config.DefaultProtocols(),
			Domain:    []string{"example.com"},
			ProjectID: "project-id",
			AccessKey: "access-key",
			SecretKey: "secret-key",
		}
		errs := cfg.Prepare()
		assert.Empty(t, errs)

		errs = cfg.Prepare()
		assert.Empty(t, errs)
	})
}
