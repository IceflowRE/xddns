package discord_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iceflowre/xddns/xddns/notifier"
	"github.com/iceflowre/xddns/xddns/notifier/discord"
)

func TestNotifier(t *testing.T) {
	t.Parallel()

	t.Run("implements notifier.Notifier", func(t *testing.T) {
		t.Parallel()

		assert.Implements(t, new(notifier.Notifier), new(discord.Notifier))
	})
}

func TestConfig_Prepare(t *testing.T) {
	t.Parallel()

	t.Run("valid config with URL", func(t *testing.T) {
		t.Parallel()

		cfg := discord.Config{
			URL: "https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnopqrstuvwxyz",
		}
		errs := cfg.Prepare()
		assert.Empty(t, errs)
	})

	t.Run("valid config with ID and Token", func(t *testing.T) {
		t.Parallel()

		cfg := discord.Config{
			ID:    "123456789012345678",
			Token: "abcdefghijklmnopqrstuvwxyz",
		}
		errs := cfg.Prepare()
		assert.Empty(t, errs)
	})

	t.Run("invalid config with both URL and ID/Token", func(t *testing.T) {
		t.Parallel()

		cfg := discord.Config{
			URL:   "https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnopqrstuvwxyz",
			ID:    "123456789012345678",
			Token: "abcdefghijklmnopqrstuvwxyz",
		}
		errs := cfg.Prepare()
		assert.Len(t, errs, 1)
	})

	t.Run("invalid config with neither URL nor ID/Token", func(t *testing.T) {
		t.Parallel()

		cfg := discord.Config{}
		errs := cfg.Prepare()
		assert.Len(t, errs, 1)
	})

	t.Run("idompotent id, token", func(t *testing.T) {
		t.Parallel()

		cfg := discord.Config{
			ID:    "123456789012345678",
			Token: "abcdefghijklmnopqrstuvwxyz",
		}
		errs := cfg.Prepare()
		assert.Empty(t, errs)

		errs = cfg.Prepare()
		assert.Empty(t, errs)
	})

	t.Run("idompotent url", func(t *testing.T) {
		t.Parallel()

		cfg := discord.Config{
			URL: "https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnopqrstuvwxyz",
		}
		errs := cfg.Prepare()
		assert.Empty(t, errs)

		errs = cfg.Prepare()
		assert.Empty(t, errs)
	})
}
