package lib

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
)

var ErrCannotMarshalIntoNil = errors.New("cannot unmarshal into nil StringSlice")

// StringSlice is a []string that can be unmarshaled from either a single YAML string or a YAML string array.
//
//	# Single string
//	tags: "production"
//
//	# String array
//	tags:
//	  - "production"
//	  - "us-east"
type StringSlice []string //nolint:recvcheck

// IsZero implements IsZeroer.
func (s *StringSlice) IsZero() bool {
	return s == nil || len(*s) == 0
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *StringSlice) UnmarshalYAML(buf []byte) error {
	if s == nil {
		return ErrCannotMarshalIntoNil
	}

	var single string
	err := yaml.Unmarshal(buf, &single)
	if err == nil {
		*s = StringSlice{single}

		return nil
	}

	var result []string
	err = yaml.Unmarshal(buf, &result)
	if err != nil {
		return fmt.Errorf("StringSlice: expected string or array: %w", err)
	}
	*s = result

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (s StringSlice) MarshalYAML() (any, error) {
	if len(s) == 1 {
		return s[0], nil
	}

	return []string(s), nil
}
