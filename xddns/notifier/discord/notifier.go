package discord

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog"
	"resty.dev/v3"

	"github.com/iceflowre/xddns/xddns/config"
	"github.com/iceflowre/xddns/xddns/internal"
	"github.com/iceflowre/xddns/xddns/lib"
	"github.com/iceflowre/xddns/xddns/notifier"
)

func init() {
	notifier.MustRegister("discord", New)
}

var (
	ErrConfigMustEitherProvideURLOrIDAndToken = errors.New("config must either provide url or id and token, not a combination")
	ErrConfigMustProvideURLOrIDAndToken       = errors.New("config must provide url or id and token")
	ErrRateLimited                            = errors.New("rate limited")
	ErrRequestFailed                          = errors.New("request failed")
	ErrUnsupportedScheme                      = errors.New("unsupported scheme")
)

// Config configuration.
type Config struct {
	config.ProxyAwareConfig `yaml:",inline"`

	URL lib.SecretString `yaml:"url,omitempty" comment:"Discord webhook URL (optional, can be provided as ID and Token instead)"`
	// Optionally provide ID and Token instead of URL. If all are provided, an error will be returned.
	ID lib.SecretString `yaml:"id,omitempty" comment:"Discord webhook ID (optional, can be provided as URL instead)"`
	// Optionally provide ID and Token instead of URL. If all are provided, an error will be returned.
	Token  lib.SecretString `yaml:"token,omitempty" comment:"Discord webhook token (optional, can be provided as URL instead)"`
	Footer string           `yaml:"footer,omitempty" comment:"Footer text to include in the notification (optional)"`
}

// Prepare validates the configuration and prepares it for use.
func (cfg *Config) Prepare() (errs []error) {
	if cfg.URL != "" && (cfg.ID != "" || cfg.Token != "") {
		errs = append(errs, ErrConfigMustEitherProvideURLOrIDAndToken)
	}
	if cfg.URL == "" && (cfg.ID == "" || cfg.Token == "") {
		errs = append(errs, ErrConfigMustProvideURLOrIDAndToken)
	}

	if cfg.URL == "" {
		cfg.URL = lib.SecretString(fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", url.PathEscape(cfg.ID.Expose()), url.PathEscape(cfg.Token.Expose())))
		cfg.ID = ""
		cfg.Token = ""
	}
	u, err := internal.ValidateURL(cfg.URL.Expose())
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid url: %w", err))
	}
	if u.Scheme != "https" {
		errs = append(errs, fmt.Errorf("%w %q: Discord webhooks require HTTPS", ErrUnsupportedScheme, u.Scheme))
	}

	errs = append(errs, cfg.ProxyAwareConfig.Prepare()...)

	return errs
}

// Notifier for Discord.
type Notifier struct {
	cfg    Config
	client *resty.Client

	logger zerolog.Logger
}

type discordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Footer      *discordEmbedFooter `json:"footer,omitempty"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

type webhookMessage struct {
	Embeds []*discordEmbed `json:"embeds"`
}

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// New creates a new Discord notifier.
func New(cfg Config, logger zerolog.Logger) (notif *Notifier, err error) {
	errs := cfg.Prepare()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	notif = &Notifier{
		cfg: cfg,
		client: internal.RestyClient(
			internal.WithRestyRetry(),
			internal.WithRestyProxy(cfg.Proxy.FullURL()),
		).
			SetHeader("Content-Type", "application/json").
			SetBaseURL(cfg.URL.Expose()),
		logger: logger,
	}

	return notif, nil
}

// Notify sends a notification to the configured Discord webhook.
func (notif *Notifier) Notify(ctx context.Context, notification notifier.Notification) error {
	var color int
	switch {
	case notification.Reason == notifier.ReasonUpdateSucceeded:
		color = 0x00FF00
	case notification.Reason.IsEquivalent(notifier.ReasonInfo):
		color = 0x3498DB
	case notification.Reason.IsEquivalent(notifier.ReasonError):
		color = 0xFF0000
	default:
		color = 0x000000
	}

	var descParts []string
	if notification.IPs.IPv4.IsValid() {
		descParts = append(descParts, "IPv4: `"+notification.IPs.IPv4.String()+"`")
	}
	if notification.IPs.IPv6.IsValid() {
		descParts = append(descParts, "IPv6: `"+notification.IPs.IPv6.String()+"`")
	}
	if notification.Error != nil {
		descParts = append(descParts, "Error: "+notification.Error.Error())
	}
	desc := strings.Join(descParts, "\n")

	embed := &discordEmbed{
		Title:       "`" + notification.UpdaterName + "` " + notifier.ReasonText(notification.Reason),
		Description: desc,
		Color:       color,
	}
	if notif.cfg.Footer != "" {
		embed.Footer = &discordEmbedFooter{
			Text: notif.cfg.Footer,
		}
	}

	resp, err := notif.client.R().SetContext(ctx).SetBody(&webhookMessage{
		Embeds: []*discordEmbed{embed},
	}).Post("")
	if err != nil {
		return err
	}

	if resp.IsStatusSuccess() {
		return nil
	}
	if resp.StatusCode() == http.StatusTooManyRequests {
		return fmt.Errorf("%w, %w, retry after %s", ErrRequestFailed, ErrRateLimited, resp.Header().Get("Retry-After"))
	}

	apiResp := apiResponse{}
	_ = json.Unmarshal(resp.Bytes(), &apiResp)

	return apiError(resp.StatusCode(), apiResp)
}

func apiError(statusCode int, response apiResponse) error {
	if response.Message == "" {
		return fmt.Errorf("%w: HTTP status %d, code %d", ErrRequestFailed, statusCode, response.Code)
	}

	return fmt.Errorf("%w: HTTP status %d, code %d, message %q", ErrRequestFailed, statusCode, response.Code, response.Message)
}
