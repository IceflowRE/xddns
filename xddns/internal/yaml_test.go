package internal

import (
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseMapping(t *testing.T, s string) ast.Node {
	t.Helper()

	f, err := parser.ParseBytes([]byte(s), 0)
	require.NoError(t, err)
	if len(f.Docs) == 0 || f.Docs[0].Body == nil {
		return nil
	}
	return f.Docs[0].Body
}

func nodeToYAML(t *testing.T, n ast.Node) string {
	t.Helper()

	if n == nil {
		return ""
	}
	b, err := n.MarshalYAML()
	require.NoError(t, err)
	return string(b) + "\n"
}

func mapFromNode(t *testing.T, n ast.Node) map[string]string {
	t.Helper()

	m, ok := n.(*ast.MappingNode)
	require.True(t, ok, "expected mapping node, got %T", n)

	values := map[string]string{}
	for _, v := range m.Values {
		values[v.Key.GetToken().Value] = v.Value.GetToken().Value
	}
	return values
}

func keysFromNode(t *testing.T, n ast.Node) []string {
	t.Helper()

	m, ok := n.(*ast.MappingNode)
	require.True(t, ok, "expected mapping node, got %T", n)

	keys := make([]string, 0, len(m.Values))
	for _, v := range m.Values {
		keys = append(keys, v.Key.GetToken().Value)
	}
	return keys
}

func TestMergeYAML(t *testing.T) {
	t.Parallel()

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()

		merged, err := MergeYaml(nil, nil)
		require.NoError(t, err)
		require.Nil(t, merged)
	})

	t.Run("base nil", func(t *testing.T) {
		t.Parallel()

		override := mustParseMapping(t, `a: 1
b: 2
`)
		merged, err := MergeYaml(nil, override)
		require.NoError(t, err)
		assert.Equal(t, "a: 1\nb: 2\n", nodeToYAML(t, merged))
	})

	t.Run("override nil", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, `a: 1
b: 2
`)
		merged, err := MergeYaml(base, nil)
		require.NoError(t, err)
		assert.Equal(t, "a: 1\nb: 2\n", nodeToYAML(t, merged))
	})

	t.Run("override takes precedence", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, `a: 1
b: 2
c: 3
`)
		override := mustParseMapping(t, `b: 20
d: 4
`)
		merged, err := MergeYaml(base, override)
		require.NoError(t, err)

		assert.Equal(t, map[string]string{
			"a": "1",
			"b": "20",
			"c": "3",
			"d": "4",
		}, mapFromNode(t, merged))
	})

	t.Run("override keys come first", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, `a: 1
b: 2
`)
		override := mustParseMapping(t, `b: 20
c: 3
`)
		merged, err := MergeYaml(base, override)
		require.NoError(t, err)

		assert.Equal(t, []string{"b", "c", "a"}, keysFromNode(t, merged))
	})

	t.Run("base not mapping", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, "- a\n- b\n")
		override := mustParseMapping(t, "a: 1\n")

		merged, err := MergeYaml(base, override)
		assert.Nil(t, merged)
		assert.ErrorIs(t, err, ErrMappingNodeExpected)
	})

	t.Run("override not mapping", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, "a: 1\n")
		override := mustParseMapping(t, "not-a-map\n")

		merged, err := MergeYaml(base, override)
		assert.Nil(t, merged)
		assert.ErrorIs(t, err, ErrMappingNodeExpected)
	})

	t.Run("empty mappings", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, "{}\n")
		override := mustParseMapping(t, "{}\n")

		merged, err := MergeYaml(base, override)
		require.NoError(t, err)

		mapNode, ok := merged.(*ast.MappingNode)
		require.True(t, ok)
		assert.Empty(t, mapNode.Values)
	})

	t.Run("duplicate key in base skipped when overridden", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, `a: 1
b: 2
`)
		override := mustParseMapping(t, `a: 100
`)

		merged, err := MergeYaml(base, override)
		require.NoError(t, err)

		values := mapFromNode(t, merged)
		assert.Equal(t, "100", values["a"])
		assert.Equal(t, "2", values["b"])
		assert.Len(t, values, 2)
	})

	t.Run("deep merge on objects", func(t *testing.T) {
		t.Parallel()

		t.Run("nested mapping keys deep merge", func(t *testing.T) {
			t.Parallel()

			base := mustParseMapping(t, `obj:
  a: "foo"
  b: "boo"
`)
			override := mustParseMapping(t, `obj:
  b: "newboo"
`)

			merged, err := MergeYaml(base, override)
			require.NoError(t, err)

			mapNode, ok := merged.(*ast.MappingNode)
			require.True(t, ok)
			require.Len(t, mapNode.Values, 1)
			assert.Equal(t, "obj", mapNode.Values[0].Key.GetToken().Value)

			objNode := mapNode.Values[0].Value
			_, isMapping := objNode.(*ast.MappingNode)
			require.True(t, isMapping)

			// "a" survives from base since override didn't touch it,
			// "b" is overridden.
			assert.Equal(t, map[string]string{
				"a": "foo",
				"b": "newboo",
			}, mapFromNode(t, objNode))
		})

		t.Run("scalar override", func(t *testing.T) {
			t.Parallel()

			base := mustParseMapping(t, `obj:
  a: "foo"
  b: "boo"
`)
			override := mustParseMapping(t, `obj: false
`)

			merged, err := MergeYaml(base, override)
			require.NoError(t, err)

			mapNode, ok := merged.(*ast.MappingNode)
			require.True(t, ok)
			require.Len(t, mapNode.Values, 1)
			assert.Equal(t, "obj", mapNode.Values[0].Key.GetToken().Value)

			objNode := mapNode.Values[0].Value
			boolNode, ok := objNode.(*ast.BoolNode)
			require.True(t, ok, "expected bool node, got %T", objNode)
			assert.False(t, boolNode.Value)
		})

		t.Run("deeply nested objects merge recursively", func(t *testing.T) {
			t.Parallel()

			base := mustParseMapping(t, `obj:
  nested:
    x: "1"
    y: "2"
  top: "unchanged"
`)
			override := mustParseMapping(t, `obj:
  nested:
    y: "overridden"
    z: "3"
`)

			merged, err := MergeYaml(base, override)
			require.NoError(t, err)

			mapNode, ok := merged.(*ast.MappingNode)
			require.True(t, ok)
			objNode, ok := mapNode.Values[0].Value.(*ast.MappingNode)
			require.True(t, ok)

			objVals := map[string]ast.Node{}
			for _, v := range objNode.Values {
				objVals[v.Key.GetToken().Value] = v.Value
			}

			require.Contains(t, objVals, "top")
			assert.Equal(t, "unchanged", objVals["top"].GetToken().Value)

			require.Contains(t, objVals, "nested")
			nestedNode, ok := objVals["nested"].(*ast.MappingNode)
			require.True(t, ok)
			assert.Equal(t, map[string]string{
				"x": "1",
				"y": "overridden",
				"z": "3",
			}, mapFromNode(t, nestedNode))
		})

		t.Run("object override", func(t *testing.T) {
			t.Parallel()

			base := mustParseMapping(t, "obj: false\n")
			override := mustParseMapping(t, `obj:
  a: "foo"
`)

			merged, err := MergeYaml(base, override)
			require.NoError(t, err)

			mapNode, ok := merged.(*ast.MappingNode)
			require.True(t, ok)
			objNode, ok := mapNode.Values[0].Value.(*ast.MappingNode)
			require.True(t, ok)
			assert.Equal(t, map[string]string{
				"a": "foo",
			}, mapFromNode(t, objNode))
		})
	})

	t.Run("lists are replaced wholesale not merged", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, `items:
  - "a"
  - "b"
  - "c"
`)
		override := mustParseMapping(t, `items:
  - "x"
`)

		merged, err := MergeYaml(base, override)
		require.NoError(t, err)

		mapNode, ok := merged.(*ast.MappingNode)
		require.True(t, ok)
		require.Len(t, mapNode.Values, 1)
		assert.Equal(t, "items", mapNode.Values[0].Key.GetToken().Value)

		itemsNode, ok := mapNode.Values[0].Value.(*ast.SequenceNode)
		require.True(t, ok)

		var values []string
		for _, n := range itemsNode.Values {
			values = append(values, n.GetToken().Value)
		}

		// override's list wins entirely — base's "a", "b", "c" are gone, not appended/concatenated/merged by index.
		assert.Equal(t, []string{"x"}, values)
	})

	t.Run("does not mutate inputs", func(t *testing.T) {
		t.Parallel()

		base := mustParseMapping(t, `obj:
  a: "foo"
  b: "boo"
shared_scalar: "base-val"
base_only: "keep-me"
items:
  - "a"
  - "b"
`)
		override := mustParseMapping(t, `obj:
  b: "newboo"
  c: "newc"
shared_scalar: "override-val"
override_only: "also-keep-me"
items:
  - "x"
`)
		baseBefore := nodeToYAML(t, base)
		overrideBefore := nodeToYAML(t, override)

		merged, err := MergeYaml(base, override)
		require.NoError(t, err)
		require.NotNil(t, merged)

		require.NotEqual(t, baseBefore, nodeToYAML(t, merged))
		require.NotEqual(t, overrideBefore, nodeToYAML(t, merged))

		baseAfter := nodeToYAML(t, base)
		overrideAfter := nodeToYAML(t, override)

		assert.Equal(t, baseBefore, baseAfter, "base node must not be mutated by MergeYAML")
		assert.Equal(t, overrideBefore, overrideAfter, "override node must not be mutated by MergeYAML")
	})
}
