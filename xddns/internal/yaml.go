package internal

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

var ErrMappingNodeExpected = errors.New("expected a mapping node")

// MergeYaml merges two YAML mapping nodes, with the override node taking precedence over the base node.
// If both nodes have the same key and their corresponding values are also mapping nodes, they will be merged recursively.
// If a key exists in both nodes but their values are not both mapping nodes, the value from the override node will be used.
// The order of keys in the resulting merged node will have keys from the override node first,
// followed by keys from the base node that were not present in the override node.
func MergeYaml(base ast.Node, override ast.Node) (ast.Node, error) { //nolint:gocognit,ireturn
	if base == nil && override == nil {
		return nil, nil //nolint:nilnil
	}

	baseMap, err := asMappingNode(base)
	if err != nil {
		return nil, err
	}
	overrideMap, err := asMappingNode(override)
	if err != nil {
		return nil, err
	}

	merged := ast.Mapping(nil, false)

	// index override keys -> their position in merged.Values
	overrideIndex := map[string]int{}
	if overrideMap != nil {
		for _, v := range overrideMap.Values {
			merged.Values = append(merged.Values, v)
			overrideIndex[v.Key.GetToken().Value] = len(merged.Values) - 1
		}
	}

	if baseMap != nil {
		for _, val := range baseMap.Values {
			key := val.Key.GetToken().Value

			idx, existsInOverride := overrideIndex[key]
			if !existsInOverride {
				// key only in base: keep as-is
				merged.Values = append(merged.Values, val)

				continue
			}

			// key in both: deep-merge only if BOTH sides are mappings.
			// otherwise override fully wins (already in merged.Values).
			baseChild, baseIsMap := val.Value.(*ast.MappingNode)
			overrideChild, overrideIsMap := merged.Values[idx].Value.(*ast.MappingNode)
			if baseIsMap && overrideIsMap {
				mergedChild, err := MergeYaml(baseChild, overrideChild)
				if err != nil {
					return nil, err
				}
				// Rebuild the entry rather than mutating override's node in place.
				original := merged.Values[idx]
				merged.Values[idx] = ast.MappingValue(original.Start, original.Key, mergedChild)
			}
		}
	}

	if len(merged.Values) > 0 {
		merged.Start = merged.Values[0].Key.GetToken()
	}

	return merged, nil
}

func asMappingNode(node ast.Node) (*ast.MappingNode, error) {
	if node == nil {
		return nil, nil //nolint:nilnil
	}
	m, ok := node.(*ast.MappingNode)
	if !ok {
		return nil, ErrMappingNodeExpected
	}

	return m, nil
}

var (
	bytesMarshalerType     = reflect.TypeFor[*yaml.BytesMarshaler]().Elem()     //nolint:gochecknoglobals
	interfaceMarshalerType = reflect.TypeFor[*yaml.InterfaceMarshaler]().Elem() //nolint:gochecknoglobals
)

func implementsYAMLMarshaler(val reflect.Value) bool {
	valType := val.Type()
	pt := reflect.PointerTo(valType)
	if pt.Implements(bytesMarshalerType) || pt.Implements(interfaceMarshalerType) {
		return true
	}

	return valType.Implements(bytesMarshalerType) || valType.Implements(interfaceMarshalerType)
}

// BuildYAMLComments builds a map of YAML comments for the given value, using the "comment" struct tags.
func BuildYAMLComments(val any) (yaml.CommentMap, error) {
	cm := yaml.CommentMap{}
	walkYAMLComments(reflect.ValueOf(val), "$", cm)

	return cm, nil
}

func walkYAMLComments(val reflect.Value, path string, comments yaml.CommentMap) { //nolint:gocognit
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct || implementsYAMLMarshaler(val) {
		return
	}

	typ := val.Type()
	for idx := range val.NumField() {
		fieldType := typ.Field(idx)
		fieldVal := val.Field(idx)

		if !fieldType.IsExported() {
			continue
		}

		yamlTag := fieldType.Tag.Get("yaml")
		var name string
		inline := false
		if yamlTag != "" {
			parts := strings.Split(yamlTag, ",")
			name = parts[0]
			if name == "-" {
				continue
			}
			inline = slices.Contains(parts[1:], "inline")
		}

		isEmbedded := fieldType.Anonymous && name == ""
		if inline || isEmbedded {
			walkYAMLComments(fieldVal, path, comments)

			continue
		}

		fieldName := fieldType.Name
		if name != "" {
			fieldName = name
		}
		fieldPath := path + "." + fieldName

		if comment := fieldType.Tag.Get("comment"); comment != "" {
			comments[fieldPath] = []*yaml.Comment{yaml.HeadComment(" " + comment)}
		}

		walkYAMLComments(fieldVal, fieldPath, comments)
	}
}
