package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"resty.dev/v3"
)

const (
	defaultRetryCount   = 3
	defaultRetryTimeout = 1 * time.Second
	defaultTimeout      = 10 * time.Second
	maxRetryTimeout     = 10 * time.Second
)

// RestyClientOptions defines a function type that modifies a Resty client.
type RestyClientOptions func(*resty.Client)

// WithRestyRetry configures the Resty client to retry requests on certain conditions.
func WithRestyRetry() RestyClientOptions {
	return func(client *resty.Client) {
		AddRestyRetry(client)
	}
}

// WithRestyProxy configures the Resty client to use the specified proxy URL for its requests.
func WithRestyProxy(proxyURL string) RestyClientOptions {
	return func(client *resty.Client) {
		if proxyURL == "" {
			return
		}

		client.SetProxy(proxyURL)
	}
}

const (
	// TCPv4 represents the TCP version 4.
	TCPv4 = "tcp4"
	// TCPv6 represents the TCP version 6.
	TCPv6 = "tcp6"
)

// WithTCP configures the Resty client to use the specified TCP version (TCPv4 or TCPv6) for its connections.
func WithTCP(tcpVersion string) RestyClientOptions {
	return func(client *resty.Client) {
		transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert

		transport.DialContext = func(ctx context.Context, _network string, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout: 5 * time.Second, //nolint:mnd
			}

			return dialer.DialContext(ctx, tcpVersion, addr)
		}

		client.SetTransport(transport)
	}
}

// RestyClient creates a new Resty client with the provided options.
func RestyClient(opts ...RestyClientOptions) *resty.Client {
	client := resty.New().
		SetTimeout(defaultTimeout).
		SetHeaders(map[string]string{
			"User-Agent": "xddns/" + Version(),
		})
	for _, opt := range opts {
		opt(client)
	}

	return client
}

var ErrRetryTimeoutExceeded = errors.New("Retry-After header value exceeds maximum allowed timeout")

// AddRestyRetry configures the provided Resty client to retry requests on certain conditions.
func AddRestyRetry(client *resty.Client) *resty.Client {
	return client.
		SetRetryCount(defaultRetryCount).
		SetRetryMaxWaitTime(maxRetryTimeout).
		SetRetryWaitTime(defaultRetryTimeout).
		AddRetryConditions(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			if r.StatusCode() == http.StatusTooManyRequests || r.StatusCode() >= http.StatusInternalServerError {
				return true
			}

			return false
		}).SetRetryDelayStrategy(retryDelayFn(client))
}

func retryDelayFn(client *resty.Client) func(resp *resty.Response, err error) (time.Duration, error) {
	return func(resp *resty.Response, _ error) (time.Duration, error) {
		if resp.StatusCode() != http.StatusTooManyRequests {
			return client.RetryWaitTime(), nil
		}

		retryAfter := resp.Header().Get("Retry-After")
		if retryAfter == "" {
			return client.RetryWaitTime(), nil
		}

		timeout, parseErr := strconv.ParseInt(retryAfter, 10, 64)
		if parseErr != nil {
			return client.RetryWaitTime(), nil //nolint:nilerr
		}

		if timeout > int64(client.RetryMaxWaitTime().Seconds()) {
			return client.RetryMaxWaitTime(), fmt.Errorf("%w: (%d/%d)", ErrRetryTimeoutExceeded, timeout, int64(maxRetryTimeout.Seconds()))
		}

		if timeout < 0 {
			return client.RetryWaitTime(), nil
		}

		return time.Duration(timeout) * time.Second, nil
	}
}
