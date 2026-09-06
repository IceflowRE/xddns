package lib_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
)

func TestIPs(t *testing.T) {
	t.Parallel()

	t.Run("is equal", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, lib.IPs{
			IPv4: netip.MustParseAddr("192.168.1.1"),
			IPv6: netip.MustParseAddr("2001:db8::1"),
		}, lib.IPs{
			IPv4: netip.MustParseAddr("192.168.1.1"),
			IPv6: netip.MustParseAddr("2001:db8::1"),
		})
	})
}

func TestIPs_Set(t *testing.T) {
	t.Parallel()

	v4Addr := netip.MustParseAddr("192.168.1.1")
	v6Addr := netip.MustParseAddr("2001:db8::1")

	tests := []struct {
		name         string
		initialState *lib.IPs
		proto        string
		ip           netip.Addr
		expectedIPv4 netip.Addr
		expectedIPv6 netip.Addr
	}{
		{
			name:         "set IPv4 on empty struct",
			initialState: &lib.IPs{},
			proto:        internal.IPv4,
			ip:           v4Addr,
			expectedIPv4: v4Addr,
			expectedIPv6: netip.Addr{},
		},
		{
			name:         "set IPv6 on empty struct",
			initialState: &lib.IPs{},
			proto:        internal.IPv6,
			ip:           v6Addr,
			expectedIPv4: netip.Addr{},
			expectedIPv6: v6Addr,
		},
		{
			name:         "overwrite existing IPv4 address",
			initialState: &lib.IPs{IPv4: netip.MustParseAddr("10.0.0.1")},
			proto:        internal.IPv4,
			ip:           v4Addr,
			expectedIPv4: v4Addr,
			expectedIPv6: netip.Addr{},
		},
		{
			name:         "invalid protocol does not alter struct",
			initialState: &lib.IPs{IPv4: v4Addr},
			proto:        "invalid_proto",
			ip:           v6Addr,
			expectedIPv4: v4Addr,
			expectedIPv6: netip.Addr{},
		},
		{
			name:         "set zero-value IP clears field",
			initialState: &lib.IPs{IPv4: v4Addr, IPv6: v6Addr},
			proto:        internal.IPv4,
			ip:           netip.Addr{},
			expectedIPv4: netip.Addr{},
			expectedIPv6: v6Addr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ips := tt.initialState
			ips.Set(tt.proto, tt.ip)

			assert.Equal(t, tt.expectedIPv4, ips.IPv4)
			assert.Equal(t, tt.expectedIPv6, ips.IPv6)
		})
	}
}
