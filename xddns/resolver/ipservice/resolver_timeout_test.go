package ipservice

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"resty.dev/v3"

	"github.com/iceflowre/xddns/xddns/config"
)

type blockingRoundTripper struct{}

func (blockingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	<-request.Context().Done()

	return nil, request.Context().Err()
}

func TestResolverTotalTimeout(t *testing.T) {
	resolver := &Resolver{
		cfg:      Config{URL: []string{"http://example.test"}},
		clientv4: resty.New().SetTransport(blockingRoundTripper{}),
		logger:   zerolog.Nop(),
	}

	synctest.Test(t, func(t *testing.T) {
		done := make(chan error, 1)
		go func() {
			_, err := resolver.Resolve(context.Background(), config.Protocols{IPv4: true})
			done <- err
		}()

		synctest.Wait()
		time.Sleep(ipResolutionTimeout)
		synctest.Wait()

		require.ErrorIs(t, <-done, context.DeadlineExceeded)
	})
}
