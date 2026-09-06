package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name string `yaml:"name" comment:"The name of the person"`
	Age  int    `yaml:"age" comment:"The age of the person"`
}

func TestMarshalYAMLWithComments(t *testing.T) {
	t.Parallel()

	out, err := marshalYAMLWithComments(testStruct{Name: "Alice", Age: 30})
	require.NoError(t, err)
	assert.Equal(t, `# The name of the person
name: Alice
# The age of the person
age: 30
`, string(out))
}
