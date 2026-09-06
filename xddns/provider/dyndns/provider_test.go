package dyndns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"resty.dev/v3"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/provider"
)

func TestProvider(t *testing.T) {
	t.Parallel()

	t.Run("implements provider.Provider", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(provider.Provider), new(Provider))
	})
}

func TestParseDynDNSResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{
			name:    "good response",
			body:    "good 1.2.3.4",
			wantErr: nil,
		},
		{
			name:    "good response without ip",
			body:    "good",
			wantErr: nil,
		},
		{
			name:    "nochg response",
			body:    "nochg 1.2.3.4",
			wantErr: nil,
		},
		{
			name:    "badauth response",
			body:    "badauth",
			wantErr: ErrBadAuth,
		},
		{
			name:    "notfqdn response",
			body:    "notfqdn",
			wantErr: ErrNotFQDN,
		},
		{
			name:    "nohost response",
			body:    "nohost",
			wantErr: ErrNoHost,
		},
		{
			name:    "abuse response",
			body:    "abuse",
			wantErr: ErrAbuse,
		},
		{
			name:    "badagent response",
			body:    "badagent",
			wantErr: ErrBadAgent,
		},
		{
			name:    "dnserr response",
			body:    "dnserr",
			wantErr: ErrDNSError,
		},
		{
			name:    "911 response",
			body:    "911",
			wantErr: Err911,
		},
		{
			name:    "unrecognized response",
			body:    "wat happened here",
			wantErr: ErrUnrecognized,
		},
		{
			name:    "empty response",
			body:    "",
			wantErr: ErrEmptyResponse,
		},
		{
			name:    "whitespace only response",
			body:    "   \n\t  ",
			wantErr: ErrEmptyResponse,
		},
		{
			name:    "response with surrounding whitespace",
			body:    "  good 1.2.3.4  \n",
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := parseDynDNSResponse(tc.body)

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestConfig_Prepare(t *testing.T) {
	t.Run("url missing", func(t *testing.T) {
		t.Parallel()

		errs := new(Config{}).Prepare()

		err := errors.Join(errs...)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrURLIsRequired)
	})

	t.Run("missing https scheme", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Protocols: config.DefaultProtocols(),
			URL:       "example.com/nic/update",
			Domain:    lib.StringSlice{"host.example.com"},
			Username:  "user",
			Password:  lib.SecretString("pass"),
		}

		errs := cfg.Prepare()

		err := errors.Join(errs...)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(cfg.URL, "https://"))
	})

	t.Run("reject non https", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			URL:      "http://example.com/nic/update",
			Domain:   lib.StringSlice{"host.example.com"},
			Username: "user",
			Password: lib.SecretString("pass"),
		}

		errs := cfg.Prepare()

		err := errors.Join(errs...)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrUnsupportedScheme)
	})

	t.Run("idomptent", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			Protocols: config.DefaultProtocols(),
			URL:       "https://example.com/nic/update",
			Domain:    lib.StringSlice{"host.example.com"},
			Username:  "user",
			Password:  lib.SecretString("pass"),
		}

		errs := cfg.Prepare()
		assert.Empty(t, errs)

		errs = cfg.Prepare()
		assert.Empty(t, errs)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("batch domains", func(t *testing.T) {
		t.Parallel()

		domains := make(lib.StringSlice, 0, 12)
		for idx := 0; idx < 12; idx++ {
			domains = append(domains, "host"+string(rune('a'+idx))+".example.com")
		}

		cfg := Config{
			Protocols: config.DefaultProtocols(),
			URL:       "https://example.com/nic/update",
			Domain:    domains,
			Username:  "user",
			Password:  lib.SecretString("pass"),
		}

		prov, err := New(cfg, zerolog.Nop())
		require.NoError(t, err)
		require.NotNil(t, prov)

		// 12 domains, batch size 10 -> 2 batches (10 + 2)
		require.Len(t, prov.batchDomains, 2)
		assert.Len(t, strings.Split(prov.batchDomains[0], ","), 10)
		assert.Len(t, strings.Split(prov.batchDomains[1], ","), 2)
	})
}

func newTestProvider(t *testing.T, baseURL string, batchDomains []string) *Provider {
	t.Helper()

	return &Provider{
		cfg: Config{
			Username: "testuser",
			Password: lib.SecretString("testpass"),
		},
		client:       resty.New().SetBaseURL(baseURL),
		batchDomains: batchDomains,
		logger:       zerolog.Nop(),
	}
}

func TestProvider_Update(t *testing.T) {
	t.Parallel()

	t.Run("successful update with good response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("good 1.2.3.4"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		ips := lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")}
		err := prov.Update(context.Background(), ips)
		assert.NoError(t, err)
	})

	t.Run("successful update with nochg response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("nochg 1.2.3.4"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		ips := lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")}
		err := prov.Update(context.Background(), ips)
		assert.NoError(t, err)
	})

	t.Run("sends basic auth credentials", func(t *testing.T) {
		t.Parallel()

		var gotUser, gotPass string
		var gotOK bool

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser, gotPass, gotOK = r.BasicAuth()
			_, _ = w.Write([]byte("good"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		err := prov.Update(context.Background(), lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")})
		require.NoError(t, err)

		assert.True(t, gotOK)
		assert.Equal(t, "testuser", gotUser)
		assert.Equal(t, "testpass", gotPass)
	})

	t.Run("sends hostname and myip query params", func(t *testing.T) {
		t.Parallel()

		var gotHostname, gotMyIP string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotHostname = r.URL.Query().Get("hostname")
			gotMyIP = r.URL.Query().Get("myip")
			_, _ = w.Write([]byte("good"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"a.example.com,b.example.com"})

		ips := lib.IPs{
			IPv4: netip.MustParseAddr("1.2.3.4"),
			IPv6: netip.MustParseAddr("2001:db8::1"),
		}
		err := prov.Update(context.Background(), ips)
		require.NoError(t, err)

		assert.Equal(t, "a.example.com,b.example.com", gotHostname)
		assert.Equal(t, "1.2.3.4,2001:db8::1", gotMyIP)
	})

	t.Run("uses only ipv4 when ipv6 is not valid", func(t *testing.T) {
		t.Parallel()

		var gotMyIP string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMyIP = r.URL.Query().Get("myip")
			_, _ = w.Write([]byte("good"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		ips := lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")}
		err := prov.Update(context.Background(), ips)
		require.NoError(t, err)

		assert.Equal(t, "1.2.3.4", gotMyIP)
	})

	t.Run("calls server once per batch", func(t *testing.T) {
		t.Parallel()

		var callCount int
		var seenHostnames []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			seenHostnames = append(seenHostnames, r.URL.Query().Get("hostname"))
			_, _ = w.Write([]byte("good"))
		}))
		defer srv.Close()

		batches := []string{"a.example.com", "b.example.com,c.example.com"}
		prov := newTestProvider(t, srv.URL, batches)

		err := prov.Update(context.Background(), lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")})
		require.NoError(t, err)

		assert.Equal(t, 2, callCount)
		assert.ElementsMatch(t, batches, seenHostnames)
	})

	t.Run("returns error on badauth response", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("badauth"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		err := prov.Update(context.Background(), lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrUpdateFailed)
		assert.ErrorIs(t, err, ErrBadAuth)
	})

	t.Run("returns error on non-2xx status", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		err := prov.Update(context.Background(), lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.ErrorContains(t, err, "host.example.com")
	})

	t.Run("stops on first failing batch", func(t *testing.T) {
		t.Parallel()

		var callCount int

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			_, _ = w.Write([]byte("badauth"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"a.example.com", "b.example.com"})

		err := prov.Update(context.Background(), lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")})
		require.Error(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("returns error when context is already cancelled", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("good"))
		}))
		defer srv.Close()

		prov := newTestProvider(t, srv.URL, []string{"host.example.com"})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := prov.Update(ctx, lib.IPs{IPv4: netip.MustParseAddr("1.2.3.4")})
		assert.Error(t, err)
	})
}
