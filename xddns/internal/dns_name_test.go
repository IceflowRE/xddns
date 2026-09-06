package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDNSName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "already normalized",
			input: "example.com",
			want:  "example.com",
		},
		{
			name:  "uppercase is lowercased",
			input: "EXAMPLE.COM",
			want:  "example.com",
		},
		{
			name:  "mixed case with subdomain",
			input: "Www.Example.COM",
			want:  "www.example.com",
		},
		{
			name:  "surrounding whitespace trimmed",
			input: "   example.com   ",
			want:  "example.com",
		},
		{
			name:  "trailing dot removed",
			input: "example.com.",
			want:  "example.com",
		},
		{
			name:  "whitespace, case, and trailing dot combined",
			input: "  Example.COM.  ",
			want:  "example.com",
		},
		{
			name:  "unicode domain",
			input: "münchen.de",
			want:  "münchen.de",
		},
		{
			name:  "wildcard domain",
			input: "*.münchen.de",
			want:  "*.münchen.de",
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: ErrEmptyDomain,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			want:    "",
			wantErr: ErrEmptyDomain,
		},
		{
			name:    "empty label",
			input:   "example..com",
			want:    "",
			wantErr: ErrInvalidDomain,
		},
		{
			name:    "leading dot",
			input:   ".example.com",
			want:    "",
			wantErr: ErrInvalidDomain,
		},
		{
			name:    "invalid characters",
			input:   "exa mple.com",
			want:    "",
			wantErr: ErrInvalidDomain,
		},
		{
			name:    "invalid characters underscore-adjacent symbols",
			input:   "exa!mple.com",
			want:    "",
			wantErr: ErrInvalidDomain,
		},
		{
			name:    "label too long (>63 chars)",
			input:   "a123456789012345678901234567890123456789012345678901234567890123.com",
			want:    "",
			wantErr: ErrInvalidDomain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeDNSName(tt.input)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeDNSNames(t *testing.T) {
	t.Parallel()

	t.Run("normalizes and ignores empty strings", func(t *testing.T) {
		t.Parallel()

		_, err := NormalizeDNSNames([]string{
			"Example.COM.",
			"",
			"   ",
			"Sub.Example.com",
		})
		require.ErrorIs(t, err, ErrEmptyDomain)
	})

	t.Run("single valid name", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizeDNSNames([]string{"example.com"})
		require.NoError(t, err)
		assert.Equal(t, []string{"example.com"}, got)
	})

	t.Run("nil slice returns error", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizeDNSNames(nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEmptyDomain)
		assert.Empty(t, got)
	})

	t.Run("empty slice returns error", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizeDNSNames([]string{})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEmptyDomain)
		assert.Empty(t, got)
	})

	t.Run("all entries empty returns error", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizeDNSNames([]string{"", "   ", ""})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEmptyDomain)
		assert.Empty(t, got)
	})

	t.Run("invalid non-empty entry propagates error", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizeDNSNames([]string{"example.com", "exa mple.com"})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidDomain)
		assert.Empty(t, got)
	})
}

func TestGetDNSNameRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "already a root domain",
			input: "example.com",
			want:  "example.com",
		},
		{
			name:  "single subdomain",
			input: "www.example.com",
			want:  "example.com",
		},
		{
			name:  "deeply nested subdomain",
			input: "a.b.c.example.com",
			want:  "example.com",
		},
		{
			name:    "empty dns name",
			input:   "",
			want:    "",
			wantErr: ErrEmptyDomain,
		},
		{
			name:    "single label has no root",
			input:   "localhost",
			want:    "",
			wantErr: ErrInvalidDNSRoot,
		},
		{
			name:    "malformed dns name",
			input:   "exa mple.com",
			want:    "",
			wantErr: ErrInvalidDomain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetDNSNameRoot(tt.input)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidDNSZone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		zone    string
		dnsName string
		wantErr error
	}{
		{
			name:    "zone equals dns name root",
			zone:    "example.com",
			dnsName: "www.example.com",
			wantErr: nil,
		},
		{
			name:    "zone equals dns name exactly",
			zone:    "example.com",
			dnsName: "example.com",
			wantErr: nil,
		},
		{
			name:    "zone is deeper than root but still a label-aligned ancestor",
			zone:    "sub.example.com",
			dnsName: "www.sub.example.com",
			wantErr: nil,
		},
		{
			name:    "empty zone",
			zone:    "",
			dnsName: "www.example.com",
			wantErr: ErrEmptyZone,
		},
		{
			name:    "malformed dns name",
			zone:    "example.com",
			dnsName: "exa mple.com",
			wantErr: ErrInvalidDomain,
		},
		{
			name:    "zone not label-aligned suffix (partial label match)",
			zone:    "ple.com",
			dnsName: "example.com",
			wantErr: ErrDomainNotInZone,
		},
		{
			name:    "zone well-formed but unrelated domain",
			zone:    "other.com",
			dnsName: "www.example.com",
			wantErr: ErrDomainNotInZone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := IsValidDNSZone(tt.zone, tt.dnsName)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestIsValidDNSName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "valid dns name",
			input:   "example.com",
			wantErr: nil,
		},
		{
			name:    "invalid dns name",
			input:   "exa mple.com",
			wantErr: ErrInvalidDomain,
		},
		{
			name:    "not normalized dns name",
			input:   "Example.COM",
			wantErr: ErrNotNormalized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := IsValidDNSName(tt.input)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestGetRecordName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dnsName string
		zone    string
		want    string
		wantErr error
	}{
		{
			name:    "record at zone apex returns DNSZoneRoot",
			dnsName: "example.com",
			zone:    "example.com",
			want:    DNSZoneRoot,
		},
		{
			name:    "single label record",
			dnsName: "www.example.com",
			zone:    "example.com",
			want:    "www",
		},
		{
			name:    "multi label record",
			dnsName: "a.b.example.com",
			zone:    "example.com",
			want:    "a.b",
		},
		{
			name:    "record under a non-root zone",
			dnsName: "www.sub.example.com",
			zone:    "sub.example.com",
			want:    "www",
		},
		{
			name:    "empty zone",
			dnsName: "www.example.com",
			zone:    "",
			want:    "",
			wantErr: ErrEmptyZone,
		},
		{
			name:    "dns name not part of zone",
			dnsName: "www.other.com",
			zone:    "example.com",
			want:    "",
			wantErr: ErrDomainNotInZone,
		},
		{
			name:    "use @ as zone",
			dnsName: "www.example.com",
			zone:    "@",
			want:    "www",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetRecordName(tt.dnsName, tt.zone)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
