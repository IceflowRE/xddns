package config

import (
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/goccy/go-yaml"

	"github.com/iceflowre/xddns/xddns/lib"
)

var (
	ErrCannotMarshalIntoNil = errors.New("cannot unmarshal into nil Proxy")
	ErrInvalidProxy         = errors.New("invalid proxy configuration")
)

// ProxyAware is an interface that defines methods for setting and getting proxy configurations.
type ProxyAware interface {
	SetProxy(proxy Proxy)
	GetProxy() Proxy
}

// ProxyAwareConfig is a configuration struct that can be embedded in other configuration structs to provide proxy settings.
type ProxyAwareConfig struct {
	Proxy Proxy `yaml:"proxy,omitempty" comment:"Proxy settings to use (optional)"`
}

// SetProxy sets the proxy configuration.
func (pac *ProxyAwareConfig) SetProxy(proxy Proxy) {
	pac.Proxy = proxy
}

// GetProxy returns the proxy configuration.
func (pac *ProxyAwareConfig) GetProxy() Proxy {
	return pac.Proxy
}

// Prepare validates the proxy configuration and returns any errors encountered.
func (pac *ProxyAwareConfig) Prepare() (errs []error) {
	err := pac.Proxy.Validate()
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid proxy: %w", err))
	}

	return errs
}

// Proxy represents a proxy configuration.
type Proxy struct {
	URL      lib.SecretString `yaml:"url,omitempty" comment:"Proxy URL"`
	Username string           `yaml:"username,omitempty" comment:"Username for proxy authentication (optional)"`
	Password lib.SecretString `yaml:"password,omitempty" comment:"Password for proxy authentication (optional)"`
}

// IsZero implements IsZeroer.
func (proxy *Proxy) IsZero() bool {
	return proxy.URL == "" && proxy.Username == "" && proxy.Password == ""
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (proxy *Proxy) UnmarshalYAML(buf []byte) error {
	if proxy == nil {
		return ErrCannotMarshalIntoNil
	}

	var disabled bool
	err := yaml.Unmarshal(buf, &disabled)
	if err == nil {
		if disabled {
			return fmt.Errorf("%w: 'true' is not allowed (use 'false' to disable)", ErrInvalidProxy)
		}
		*proxy = Proxy{}

		return nil
	}

	var proxyURL lib.SecretString
	err = yaml.Unmarshal(buf, &proxyURL)
	if err == nil {
		*proxy = Proxy{URL: proxyURL}

		return nil
	}

	type alias Proxy
	var tmp alias
	err = yaml.Unmarshal(buf, &tmp)
	if err != nil {
		return fmt.Errorf("%w: expected boolean false, string, or a proxy object: %w", ErrInvalidProxy, err)
	}
	*proxy = Proxy(tmp)

	return nil
}

var (
	ErrInvalidURL        = errors.New("invalid proxy URL")
	ErrMissingHost       = errors.New("missing host in proxy URL")
	ErrUnsupportedScheme = errors.New("unsupported proxy scheme")
)

// Validate checks the validity of the proxy configuration.
func (proxy *Proxy) Validate() error {
	if proxy == nil {
		return nil
	}
	if proxy.URL == "" {
		return nil
	}

	proxyURL, err := url.Parse(proxy.URL.Expose())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if !slices.Contains([]string{"http", "https", "socks5"}, proxyURL.Scheme) {
		return fmt.Errorf("%w %q: proxy requires HTTP/HTTPS or SOCKS5", ErrUnsupportedScheme, proxyURL.Scheme)
	}
	if proxyURL.Host == "" {
		return ErrMissingHost
	}

	return nil
}

// FullURL returns the full proxy URL including authentication credentials if provided.
func (proxy *Proxy) FullURL() string {
	if proxy.URL == "" {
		return ""
	}

	if proxy.Username == "" && proxy.Password == "" {
		return proxy.URL.Expose()
	}

	proxyURL, err := url.Parse(proxy.URL.Expose())
	if err != nil {
		return ""
	}

	if proxy.Password != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password.Expose())
	} else {
		proxyURL.User = url.User(proxy.Username)
	}

	return proxyURL.String()
}
