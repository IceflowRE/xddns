package strato_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/provider"
	"github.com/iceflowre/xddns/xddns/provider/strato"
)

func TestProvider(t *testing.T) {
	t.Parallel()

	t.Run("implements provider.Provider", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(provider.Provider), new(strato.Provider))
	})
}
