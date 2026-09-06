package config_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/config"
)

func TestProxyAwareConfig(t *testing.T) {
	t.Parallel()

	t.Run("implements config.ProxyAware", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(config.ProxyAware), new(config.ProxyAwareConfig))
	})
}

func TestProxy_ValidateProxy(t *testing.T) {
	t.Parallel()

	t.Run("valid proxy", func(t *testing.T) {
		t.Parallel()

		proxy := &config.Proxy{
			URL:      "http://example.com",
			Username: "user",
			Password: "pass",
		}
		assert.NoError(t, proxy.Validate())
	})

	t.Run("invalid proxy", func(t *testing.T) {
		t.Parallel()

		proxy := &config.Proxy{
			URL: "invalid-url",
		}
		err := proxy.Validate()
		assert.Error(t, err)
	})

	t.Run("empty proxy", func(t *testing.T) {
		t.Parallel()

		proxy := &config.Proxy{}
		assert.NoError(t, proxy.Validate())
	})

	t.Run("missing host", func(t *testing.T) {
		t.Parallel()

		proxy := &config.Proxy{
			URL: "http://",
		}
		err := proxy.Validate()
		assert.ErrorIs(t, err, config.ErrMissingHost)
	})
}

func TestProxy_proxyURL(t *testing.T) {
	t.Parallel()

	t.Run("proxy with username and password", func(t *testing.T) {
		t.Parallel()

		proxy := &config.Proxy{
			URL:      "http://example.com",
			Username: "user",
			Password: "pass",
		}
		assert.Equal(t, "http://user:pass@example.com", proxy.FullURL())
	})

	t.Run("proxy without username and password", func(t *testing.T) {
		t.Parallel()

		proxy := &config.Proxy{
			URL: "http://example.com",
		}
		assert.Equal(t, "http://example.com", proxy.FullURL())
	})
}

func TestProxy_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("valid proxy", func(t *testing.T) {
		t.Parallel()

		var proxy config.Proxy
		err := yaml.Unmarshal([]byte(`
url: http://myproxy
username: user
password: pass
`), &proxy)

		assert.NoError(t, err)
		assert.Equal(t, config.Proxy{
			URL:      "http://myproxy",
			Username: "user",
			Password: "pass",
		}, proxy)
	})

	t.Run("proxy as false", func(t *testing.T) {
		t.Parallel()

		var proxy config.Proxy
		err := yaml.Unmarshal([]byte(`false`), &proxy)

		assert.NoError(t, err)
		assert.Equal(t, config.Proxy{
			URL:      "",
			Username: "",
			Password: "",
		}, proxy)
	})

	t.Run("proxy as true", func(t *testing.T) {
		t.Parallel()

		var proxy config.Proxy
		err := yaml.Unmarshal([]byte(`true`), &proxy)

		assert.Error(t, err)
	})
}
