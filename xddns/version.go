package xddns

import (
	"github.com/iceflowre/xddns/xddns/internal"
)

// Version returns the current version of the xddns package.
func Version() string {
	return internal.Version()
}
