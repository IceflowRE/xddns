package cli

import (
	"github.com/goccy/go-yaml"

	"github.com/iceflowre/xddns/xddns/internal"
)

func marshalYAMLWithComments(val any) ([]byte, error) {
	cm, err := internal.BuildYAMLComments(val)
	if err != nil {
		return nil, err
	}

	return yaml.MarshalWithOptions(val, yaml.WithComment(cm))
}
