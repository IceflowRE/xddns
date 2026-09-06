package lib

import "context"

// RedactedText is the string that will be used to represent a redacted SecretString.
const RedactedText = "[REDACTED]"

// SecretString is a string type that redacts its value when marshaled to YAML or printed, but can be exposed when needed.
type SecretString string

// String implements the fmt.Stringer interface, returning a redacted representation of the SecretString.
func (SecretString) String() string {
	return RedactedText
}

// GoString implements the fmt.GoStringer interface, returning a redacted representation of the SecretString.
func (SecretString) GoString() string {
	return "types.SecretString(" + RedactedText + ")"
}

// Expose returns the underlying string value of the SecretString, bypassing the redaction.
func (s SecretString) Expose() string {
	return string(s)
}

// MarshalText implements encoding.TextMarshaler.
func (s SecretString) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// MarshalYAML implements yaml.Marshaler.
func (s SecretString) MarshalYAML(ctx context.Context) (any, error) {
	if unredact, _ := ctx.Value(unredactSecretsKey{}).(bool); unredact {
		return string(s), nil
	}

	return s.String(), nil
}

type unredactSecretsKey struct{}

// WithUnredactedSecrets returns a new context that indicates that secrets should be unredacted when marshaling.
func WithUnredactedSecrets(ctx context.Context) context.Context {
	return context.WithValue(ctx, unredactSecretsKey{}, true)
}
