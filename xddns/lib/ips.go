package lib

import (
	"net/netip"
	"strings"

	"github.com/iceflowre/xddns/xddns/internal"
)

// IPs represents a pair of IPv4 and IPv6 addresses.
type IPs struct { //nolint:recvcheck
	IPv4 netip.Addr
	IPv6 netip.Addr
}

// IsEmpty checks if both IPv4 and IPv6 addresses are invalid or if the IPs struct is nil.
func (ips *IPs) IsEmpty() bool {
	return ips == nil || !ips.IPv4.IsValid() && !ips.IPv6.IsValid()
}

// IsEqual checks if two IPs structs are equal, considering both IPv4 and IPv6 addresses.
func (ips IPs) IsEqual(other IPs) bool {
	return ips.IPv4.Compare(other.IPv4) == 0 && ips.IPv6.Compare(other.IPv6) == 0
}

// IsOneValid checks if exactly one of the IPs (IPv4 or IPv6) is valid.
func (ips *IPs) IsOneValid() bool {
	return ips != nil && (ips.IPv4.IsValid() != ips.IPv6.IsValid())
}

// Set assigns an IP address to the specified protocol, invalid protocols are ignored.
func (ips *IPs) Set(proto string, ip netip.Addr) {
	switch proto { //nolint:revive
	case internal.IPv4:
		ips.IPv4 = ip
	case internal.IPv6:
		ips.IPv6 = ip
	}
}

func (ips IPs) String() string {
	strs := make([]string, 0, 2) //nolint:mnd
	if ips.IPv4.IsValid() {
		strs = append(strs, ips.IPv4.String())
	}
	if ips.IPv6.IsValid() {
		strs = append(strs, ips.IPv6.String())
	}

	return strings.Join(strs, ", ")
}
