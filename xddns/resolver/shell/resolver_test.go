package shell_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/resolver"
	"github.com/iceflowre/xddns/xddns/resolver/shell"
)

func TestResolver(t *testing.T) {
	assert.Implements(t, new(resolver.Resolver), new(shell.Resolver))
}

func TestConfig(t *testing.T) {
	var cfg shell.Config
	err := yaml.Unmarshal([]byte(`command: "ip address show dev eno1 | grep 'inet6 .* scope global'"`), &cfg)

	require.NoError(t, err)
	assert.Equal(t, "ip address show dev eno1 | grep 'inet6 .* scope global'", cfg.Command)
}

func TestResolver_Resolve(t *testing.T) {
	newResolver := func(t *testing.T, commandText string) *shell.Resolver {
		t.Helper()

		resol, err := shell.New(shell.Config{
			Protocols: config.Protocols{IPv4: true, IPv6: true},
			Command:   commandText,
		}, zerolog.Nop())
		require.NoError(t, err)

		return resol
	}

	isLinux := runtime.GOOS == "linux"
	t.Run("resolves requested addresses", func(t *testing.T) {
		if !isLinux {
			t.Skip("skipping test on non-Linux OS")
		}
		resol := newResolver(t, "printf '8.8.8.8\\n2001:4860:4860::8888\\n' | grep -E '8\\.8\\.8\\.8|2001:'")
		ips, err := resol.Resolve(context.Background(), config.Protocols{IPv4: true, IPv6: true})

		require.NoError(t, err)
		assert.Equal(t, "8.8.8.8, 2001:4860:4860::8888", ips.String())
	})

	t.Run("returns partial result", func(t *testing.T) {
		if !isLinux {
			t.Skip("skipping test on non-Linux OS")
		}
		resol := newResolver(t, "printf '8.8.8.8\\n'")
		ips, err := resol.Resolve(context.Background(), config.Protocols{IPv4: true, IPv6: true})

		require.ErrorIs(t, err, resolver.ErrNotAllProtocolsResolved)
		assert.Equal(t, "8.8.8.8", ips.String())
	})

	t.Run("rejects non-public output", func(t *testing.T) {
		if !isLinux {
			t.Skip("skipping test on non-Linux OS")
		}
		resol := newResolver(t, "printf '192.168.1.1\\n'")
		_, err := resol.Resolve(context.Background(), config.Protocols{IPv4: true})

		require.ErrorIs(t, err, shell.ErrNonPublicIP)
	})
}
