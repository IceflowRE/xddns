package lib_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/lib"
)

func TestSecretString(t *testing.T) {
	t.Parallel()

	t.Run("round trip preserves secret", func(t *testing.T) {
		t.Parallel()

		type TestStruct struct {
			Secret lib.SecretString `yaml:"secret"`
		}

		original := TestStruct{
			Secret: lib.SecretString("my-secret"),
		}

		data, err := yaml.Marshal(original)
		require.NoError(t, err)

		var decoded TestStruct
		err = yaml.Unmarshal([]byte("secret: my-secret\n"), &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.Secret.Expose(), decoded.Secret.Expose())
		assert.NotContains(t, string(data), original.Secret.Expose())
	})
}

func TestSecretString_Expose(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var secret lib.SecretString

		assert.Empty(t, secret.Expose())
	})

	t.Run("expose", func(t *testing.T) {
		t.Parallel()

		secret := lib.SecretString("my-secret")

		assert.Equal(t, "my-secret", secret.Expose())
		assert.Equal(t, lib.SecretString("my-secret"), secret)
	})

	t.Run("different secrets are independently exposed", func(t *testing.T) {
		t.Parallel()

		first := lib.SecretString("first-secret")
		second := lib.SecretString("second-secret")

		assert.Equal(t, "first-secret", first.Expose())
		assert.Equal(t, "second-secret", second.Expose())
		assert.NotEqual(t, first.Expose(), second.Expose())
	})
}

func TestSecretString_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("unmarshal yaml", func(t *testing.T) {
		t.Parallel()

		var secret lib.SecretString
		err := yaml.Unmarshal([]byte("my-secret"), &secret)

		require.NoError(t, err)
		assert.Equal(t, "my-secret", secret.Expose())
	})

	t.Run("unmarshal yaml empty", func(t *testing.T) {
		t.Parallel()

		var secret lib.SecretString
		err := yaml.Unmarshal([]byte(""), &secret)

		require.NoError(t, err)
		assert.Empty(t, secret.Expose())
	})
}

func TestSecretString_MarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("marshal yaml", func(t *testing.T) {
		t.Parallel()

		secret := lib.SecretString("my-secret")

		data, err := yaml.Marshal(secret)

		require.NoError(t, err)
		assert.Equal(t, "\""+lib.RedactedText+"\"\n", string(data))
		assert.NotContains(t, string(data), "my-secret")
	})

	t.Run("marshal yaml in struct", func(t *testing.T) {
		t.Parallel()

		type TestStruct struct {
			Secret lib.SecretString `yaml:"secret"`
		}

		testStruct := TestStruct{
			Secret: lib.SecretString("my-secret"),
		}

		data, err := yaml.Marshal(testStruct)

		require.NoError(t, err)
		assert.Equal(t, "secret: \""+lib.RedactedText+"\"\n", string(data))
		assert.NotContains(t, string(data), "my-secret")
	})

	t.Run("marshal yaml nested in struct", func(t *testing.T) {
		t.Parallel()

		type Credentials struct {
			Username string           `yaml:"username"`
			Password lib.SecretString `yaml:"password"`
		}

		type Config struct {
			Credentials Credentials `yaml:"credentials"`
		}

		config := Config{
			Credentials: Credentials{
				Username: "admin",
				Password: lib.SecretString("super-secret"),
			},
		}

		data, err := yaml.Marshal(config)

		require.NoError(t, err)
		assert.Equal(t, "credentials:\n  username: admin\n  password: \""+lib.RedactedText+"\"\n", string(data))
		assert.NotContains(t, string(data), "super-secret")
	})

	t.Run("redacts secret regardless of value", func(t *testing.T) {
		t.Parallel()

		testCases := []string{
			"",
			"my-secret",
			"secret with spaces",
			"secret\nwith\nnewlines",
			"special: value #123",
		}

		for _, secretValue := range testCases {
			t.Run(secretValue, func(t *testing.T) {
				t.Parallel()

				secret := lib.SecretString(secretValue)

				data, err := yaml.Marshal(secret)

				require.NoError(t, err)
				assert.Equal(t, "\""+lib.RedactedText+"\"\n", string(data))

				if secretValue != "" {
					assert.NotContains(t, string(data), secretValue)
				}
			})
		}
	})
}
