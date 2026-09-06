package config

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresetUnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("scalar node", func(t *testing.T) {
		t.Parallel()

		var preset Preset
		err := yaml.Unmarshal([]byte("my-preset"), &preset)

		require.NoError(t, err)
		assert.Equal(t, "my-preset", preset.Use)
		assert.Empty(t, preset.Type)
		assert.Nil(t, preset.Extra)
	})

	t.Run("mixed list", func(t *testing.T) {
		t.Parallel()

		var data struct {
			List []*Preset `yaml:"list"`
		}
		err := yaml.Unmarshal([]byte(`
list:
  - "my-preset"
  - type: "dummy"
`), &data)

		require.NoError(t, err)
		require.Len(t, data.List, 2)
		item := *data.List[0]
		assert.Equal(t, Preset{
			Use:   "my-preset",
			Type:  "",
			Extra: nil,
		}, item)
		item = *data.List[1]
		assert.Equal(t, Preset{
			Use:   "",
			Type:  "dummy",
			Extra: nil,
		}, item)
	})

	t.Run("mapping node decodes known fields", func(t *testing.T) {
		t.Parallel()

		var preset Preset
		err := yaml.Unmarshal([]byte(`
type: scaleway
use: base-preset
`), &preset)

		require.NoError(t, err)
		assert.Equal(t, "scaleway", preset.Type)
		assert.Equal(t, "base-preset", preset.Use)
	})

	t.Run("mapping node with extra fields", func(t *testing.T) {
		t.Parallel()

		var preset Preset
		err := yaml.Unmarshal([]byte(`
type: discord
webhook_url: https://example.com/hook
username: ddns-bot
`), &preset)

		require.NoError(t, err)
		assert.Equal(t, "discord", preset.Type)
		require.NotNil(t, preset.Extra)
		assert.Equal(t, ast.MappingType, preset.Extra.Type())
		mappingNode, ok := preset.Extra.(*ast.MappingNode)
		require.True(t, ok)
		assert.Len(t, mappingNode.Values, 2)
		assert.Equal(t, "webhook_url", mappingNode.Values[0].Key.String())
		assert.Equal(t, "username", mappingNode.Values[1].Key.String())
	})

	t.Run("mapping node without extra fields", func(t *testing.T) {
		t.Parallel()

		var preset Preset
		err := yaml.Unmarshal([]byte(`
type: dyndns
use: some-preset
`), &preset)

		require.NoError(t, err)
		assert.Nil(t, preset.Extra)
	})

	t.Run("invalid node kind", func(t *testing.T) {
		t.Parallel()

		var preset Preset
		err := yaml.Unmarshal([]byte(`
- one
- two
`), &preset)

		assert.ErrorIs(t, err, ErrUnexpectedNodeKind)
	})

	t.Run("invalid field type", func(t *testing.T) {
		t.Parallel()

		var preset Preset
		err := yaml.Unmarshal([]byte(`
type: dummy
protocols: not-a-protocols-object
`), &preset)

		assert.ErrorIs(t, err, ErrInvalidProtocol)
	})
}
