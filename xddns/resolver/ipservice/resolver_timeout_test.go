package ipservice

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
)

type blockingRoundTripper struct {
	requests atomic.Int32
}

func (transport *blockingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests.Add(1)
	<-request.Context().Done()

	return nil, request.Context().Err()
}

func TestResolverRequestTimeoutRetries(t *testing.T) {
	transport := &blockingRoundTripper{}
	resolver := &Resolver{
		cfg:      Config{URL: []string{"http://example.test"}},
		clientv4: internal.RestyClient(internal.WithRestyRetry()).SetTimeout(ipServiceRequestTimeout).SetTransport(transport),
		logger:   zerolog.Nop(),
	}

	synctest.Test(t, func(t *testing.T) {
		done := make(chan error, 1)
		go func() {
			_, err := resolver.Resolve(context.Background(), config.Protocols{IPv4: true})
			done <- err
		}()

		synctest.Wait()
		time.Sleep(1 * time.Second)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("request completed before the retry budget was exhausted: %v", err)
		default:
		}
		require.LessOrEqual(t, transport.requests.Load(), int32(1))

		synctest.Wait()
		require.ErrorIs(t, <-done, context.DeadlineExceeded)
		require.Equal(t, int32(4), transport.requests.Load())
	})
}
