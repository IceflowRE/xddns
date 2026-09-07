package config

import (
	"errors"
	"fmt"
	"iter"

	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
)

var (
	ErrCannotUnmarshalIntoNil = errors.New("cannot unmarshal into nil Protocols")
	ErrInvalidProtocol        = errors.New("invalid protocol specified")
	ErrNoProtocols            = errors.New("no protocols specified")
)

// ProtocolAware is an interface that defines methods for setting and getting protocol configurations.
type ProtocolAware interface {
	SetProtocols(protocols Protocols)
	GetProtocols() Protocols
}

// ProtocolAwareConfig is a configuration struct that can be embedded in other configuration structs to provide protocol settings.
type ProtocolAwareConfig struct {
	Protocols Protocols `yaml:"protocols" comment:"Protocols to use (optional)"`
}

// SetProtocols sets the protocol configuration.
func (pac *ProtocolAwareConfig) SetProtocols(protocols Protocols) {
	pac.Protocols = protocols
}

// GetProtocols returns the protocol configuration.
func (pac *ProtocolAwareConfig) GetProtocols() Protocols {
	return pac.Protocols
}

// Prepare validates the protocol configuration and returns any errors encountered.
func (pac *ProtocolAwareConfig) Prepare() (errs []error) {
	if pac.Protocols.IsEmpty() {
		errs = append(errs, fmt.Errorf("protocols: %w", ErrNoProtocols))
	}

	return errs
}

// IsZero implements IsZeroer.
func (pac *ProtocolAwareConfig) IsZero() bool {
	return pac.Protocols.IsEmpty()
}

// Protocols represents the protocols to use (IPv4 and/or IPv6).
type Protocols struct { //nolint:recvcheck
	IPv4 bool
	IPv6 bool
}

// ProtocolsFromStrings creates a Protocols struct from a slice of protocol strings.
// Ignores invalid protocols.
func ProtocolsFromStrings(protocols ...string) (*Protocols, error) {
	resProto := Protocols{}
	for _, proto := range protocols {
		err := resProto.set(proto, true)
		if err != nil {
			return nil, err
		}
	}

	return &resProto, nil
}

// Conjunct returns a new Protocols struct that is the conjunction of the two Protocols structs.
func (p *Protocols) Conjunct(other Protocols) Protocols {
	return Protocols{
		IPv4: p.IPv4 && other.IPv4,
		IPv6: p.IPv6 && other.IPv6,
	}
}

// String returns a string representation of the Protocols struct.
func (p Protocols) String() string {
	return fmt.Sprint(p.Slice())
}

// IsZero implements IsZeroer.
func (p Protocols) IsZero() bool {
	return p.IPv4 && p.IPv6
}

// IsEmpty is equal to len(Slice()) == 0.
func (p *Protocols) IsEmpty() bool {
	return p != nil && !p.IPv4 && !p.IPv6
}

// IsSet checks if the specified protocol is set to true.
func (p *Protocols) IsSet(protocol string) bool {
	if p == nil {
		return true
	}

	switch protocol {
	case internal.IPv4:
		return p.IPv4
	case internal.IPv6:
		return p.IPv6
	default:
		return false
	}
}

// Set sets the specified protocol to the given value.
func (p *Protocols) Set(protocol string, value bool) {
	_ = p.set(protocol, value)
}

// Seq returns a sequence of protocol strings that are set to true.
func (p *Protocols) Seq() iter.Seq[string] {
	return func(yield func(val string) bool) {
		if p.IPv4 {
			if !yield(internal.IPv4) {
				return
			}
		}
		if p.IPv6 {
			if !yield(internal.IPv6) {
				return
			}
		}
	}
}

// Slice returns a slice of protocol strings that are set to true.
func (p *Protocols) Slice() []string {
	if p == nil {
		return new(DefaultProtocols()).Slice()
	}

	protos := make([]string, 0, 2) //nolint:mnd
	if p.IPv4 {
		protos = append(protos, internal.IPv4)
	}
	if p.IPv6 {
		protos = append(protos, internal.IPv6)
	}

	return protos
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (p *Protocols) UnmarshalYAML(buf []byte) error {
	if p == nil {
		return ErrCannotUnmarshalIntoNil
	}

	protos := lib.StringSlice{}
	err := protos.UnmarshalYAML(buf)
	if err != nil {
		return err
	}

	if len(protos) == 0 {
		*p = Protocols{}

		return nil
	}

	newP, err := ProtocolsFromStrings(protos...)
	if err != nil {
		return err
	}
	*p = *newP

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (p Protocols) MarshalYAML() (any, error) {
	return lib.StringSlice(p.Slice()), nil
}

func (p *Protocols) set(protocol string, value bool) error { //nolint:revive
	if p == nil {
		return nil
	}
	switch protocol {
	case internal.IPv4:
		p.IPv4 = value
	case internal.IPv6:
		p.IPv6 = value
	default:
		return fmt.Errorf("%w: %s", ErrInvalidProtocol, protocol)
	}

	return nil
}
