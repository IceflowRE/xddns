package ionos_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/provider/ionos"
)

func TestProvider(t *testing.T) {
	t.Parallel()

	t.Run("implements provider.Provider", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(provider.Provider), new(ionos.Provider))
	})
}
