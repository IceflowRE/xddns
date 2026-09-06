package config

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// Preset represents a configuration preset for notifiers, providers or resolvers.
type Preset struct {
	Type string `yaml:"type,omitempty" comment:"Which type of notifier, provider or resolver to use (e.g. 'dyndns', 'scaleway', 'discord', 'ip_service', etc.)"`
	Use  string `yaml:"use,omitempty" comment:"Reference to a preset defined in the 'presets' section of the configuration"`

	Protocols *Protocols `yaml:"protocols,omitempty" comment:"Protocols for this preset (optional)"`
	Proxy     *Proxy     `yaml:"proxy,omitempty" comment:"Proxy settings for this preset (optional)"`
	Extra     ast.Node   `yaml:"-"`
}

var reservedKeys = map[string]bool{ //nolint:gochecknoglobals
	"protocols": true,
	"proxy":     true,
	"type":      true,
	"use":       true,
}

var (
	ErrCannotUnmarshalIntoNilPreset = errors.New("cannot unmarshal into nil Preset")
	ErrUnexpectedNodeKind           = errors.New("unexpected YAML node kind")
)

// UnmarshalYAML implements yaml.Unmarshaler.
func (preset *Preset) UnmarshalYAML(node ast.Node) error {
	if preset == nil {
		return ErrCannotUnmarshalIntoNilPreset
	}

	if scalarNode, ok := node.(ast.ScalarNode); ok {
		*preset = Preset{}
		preset.Use = fmt.Sprintf("%v", scalarNode.GetValue())

		return nil
	}

	mapNode, ok := node.(*ast.MappingNode)
	if !ok {
		return fmt.Errorf("%w at line %d: %s", ErrUnexpectedNodeKind, node.GetToken().Position.Line, node.Type())
	}

	type alias Preset
	var tmp alias
	err := yaml.NodeToValue(node, &tmp)
	if err != nil {
		return err
	}
	*preset = Preset(tmp)

	var extraValues []*ast.MappingValueNode
	for _, valNode := range mapNode.Values {
		if !reservedKeys[valNode.Key.GetToken().Value] {
			extraValues = append(extraValues, valNode)
		}
	}
	if len(extraValues) > 0 {
		preset.Extra = &ast.MappingNode{
			BaseNode: &ast.BaseNode{},
			Start:    mapNode.Start,
			Values:   extraValues,
		}
	}

	return nil
}
