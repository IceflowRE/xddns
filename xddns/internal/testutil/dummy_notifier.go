package testutil

import (
	"context"
	"sync"
	"testing"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/notifier"
)

// DummyNotifierConfig is a configuration struct for the DummyNotifier, used for testing purposes.
type DummyNotifierConfig struct {
	config.ProxyAwareConfig `yaml:",inline"`

	Err error
}

// DummyNotifier is a test implementation of a notifier that simulates sending notifications.
type DummyNotifier struct {
	Cfg DummyNotifierConfig

	SentNotifications []notifier.Notification
	notifyGuard       sync.Mutex
}

// NewDefaultDummyNotifierConfig creates a default configuration for DummyNotifier.
func NewDefaultDummyNotifierConfig(t *testing.T) DummyNotifierConfig {
	t.Helper()

	return DummyNotifierConfig{}
}

// NewDummyNotifier creates a new instance of DummyNotifier with the provided configuration.
func NewDummyNotifier(cfg DummyNotifierConfig) (*DummyNotifier, error) {
	return &DummyNotifier{
		Cfg: cfg,
	}, nil
}

// Notify simulates sending a notification for the DummyNotifier.
func (d *DummyNotifier) Notify(_ctx context.Context, notification notifier.Notification) error {
	d.notifyGuard.Lock()
	defer d.notifyGuard.Unlock()
	d.SentNotifications = append(d.SentNotifications, notification)

	return d.Cfg.Err
}
