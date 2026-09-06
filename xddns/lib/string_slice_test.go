package lib_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/lib"
)

type testStringSlice struct {
	Tags lib.StringSlice `yaml:"tags"`
}

func TestStringSlice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input lib.StringSlice
	}{
		{"single", lib.StringSlice{"production"}},
		{"multiple", lib.StringSlice{"production", "us-east", "tier-1"}},
		{"empty", lib.StringSlice{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := yaml.Marshal(testStringSlice{Tags: tc.input})
			require.NoError(t, err)

			var got testStringSlice
			require.NoError(t, yaml.Unmarshal(data, &got))

			assert.Equal(t, tc.input, got.Tags)
		})
	}
}

func TestStringSlice_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("single string", func(t *testing.T) {
		t.Parallel()

		var cfg testStringSlice
		err := yaml.Unmarshal([]byte(`tags: "production"`), &cfg)

		require.NoError(t, err)
		assert.Equal(t, lib.StringSlice{"production"}, cfg.Tags)
	})

	t.Run("string array", func(t *testing.T) {
		t.Parallel()

		var cfg testStringSlice
		err := yaml.Unmarshal([]byte(`
tags:
  - "production"
  - "us-east"
`), &cfg)

		require.NoError(t, err)
		assert.Equal(t, lib.StringSlice{"production", "us-east"}, cfg.Tags)
	})

	t.Run("empty array", func(t *testing.T) {
		t.Parallel()

		var cfg testStringSlice
		err := yaml.Unmarshal([]byte("tags: []"), &cfg)

		require.NoError(t, err)
		assert.Empty(t, cfg.Tags)
	})

	t.Run("inline array", func(t *testing.T) {
		t.Parallel()

		var cfg testStringSlice
		err := yaml.Unmarshal([]byte(`tags: ["prod", "staging"]`), &cfg)

		require.NoError(t, err)
		assert.Equal(t, lib.StringSlice{"prod", "staging"}, cfg.Tags)
	})

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()

		var cfg testStringSlice
		err := yaml.Unmarshal([]byte(`
tags:
  key: value
`), &cfg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "StringSlice: expected string or array")
	})
}

func TestStringSlice_MarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("single element", func(t *testing.T) {
		t.Parallel()

		out, err := yaml.Marshal(testStringSlice{Tags: lib.StringSlice{"production"}})

		require.NoError(t, err)
		assert.Equal(t, "tags: production\n", string(out))
	})

	t.Run("multiple elements", func(t *testing.T) {
		t.Parallel()

		input := testStringSlice{
			Tags: lib.StringSlice{"production", "us-east"},
		}

		out, err := yaml.Marshal(input)
		require.NoError(t, err)

		var got testStringSlice
		require.NoError(t, yaml.Unmarshal(out, &got))

		assert.Equal(t, input, got)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out, err := yaml.Marshal(testStringSlice{Tags: lib.StringSlice{}})
		require.NoError(t, err)

		assert.Equal(t, "tags: []\n", string(out))
	})
}
