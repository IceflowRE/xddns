package internal

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		urlStr  string
		want    *url.URL
		wantErr error
	}{
		{
			name:   "valid URL",
			urlStr: "https://example.com/path",
			want:   &url.URL{Scheme: "https", Host: "example.com", Path: "/path"},
		},
		{
			name:    "invalid URL",
			urlStr:  "://example.com",
			wantErr: ErrInvalidURL,
		},
		{
			name:    "empty URL",
			urlStr:  "",
			wantErr: ErrEmptyURL,
		},
		{
			name:    "missing scheme",
			urlStr:  "example.com",
			wantErr: ErrInvalidURL,
		},
		{
			name:    "missing host",
			urlStr:  "http://",
			wantErr: ErrInvalidURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ValidateURL(tt.urlStr)
			if tt.wantErr != nil {
				require.ErrorIs(t, gotErr, tt.wantErr)
				return
			}
			require.NoError(t, gotErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
