package testutil

import "github.com/iceflowre/xddns/xddns/config"

// ChangeProtocols is a utility function that sets the protocols of a given ProtocolAware configuration and returns the modified configuration.
func ChangeProtocols[T config.ProtocolAware](cfg T, protocols config.Protocols) T { //nolint:ireturn
	cfg.SetProtocols(protocols)

	return cfg
}
