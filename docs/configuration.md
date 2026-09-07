# Configuration

XDDNS reads YAML configuration. Keep credentials out of updater names because names appear in logs and notifications. Protect the file with mode `0600`, other modes are rejected.

## Selecting a file

Use `--config` (or `-c`) with any command:

```shell
xddns --config /etc/xddns.yaml update
```

Without that flag, set `XDDNS_CONFIG` to use one specific file. Otherwise XDDNS checks the current directory, the user configuration directory and system locations. The usual filenames are `xddns.yaml` and `xddns.yml`. `config path` prints the file that would be selected.

## Main settings

```yaml
# daemon interval; minimum is 1m
update_interval: 10m
# debug, info, warn, or error
log_level: info

protocols:
  - ipv4
  - ipv6

proxy:
  url: "http://proxy.example.com:8080"
  username: "user"
  password: "secret"
```

These settings apply to all updaters by default. A provider, resolver, notifier or individual updater can override the applicable settings. IPv4 and IPv6 are enabled by default. Use only the protocol needed by a record when appropriate.

## Presets

Presets avoid repeating credentials or connection details. Define them under the matching section, then reference them with `use`. A bare string in a resolver or notifier list is also treated as a preset name.

```yaml
presets:
  providers:
    dns-account:
      type: "scaleway"
      project_id: "project-id"
      access_key: "access-key"
      secret_key: "secret-key"
      domain: "example.com"
  resolvers:
    public-ip:
      type: "ip_service"
      url: "https://api.ipify.org"
  notifiers:
    ops-discord:
      type: "discord"
      url: "https://discord.com/api/webhooks/id/token"

updaters:
  - name: "home.example.com"
    provider:
      use: "dns-account"
      domain: "home.example.com"
    resolvers:
      - public-ip
    notifiers:
      - use: "ops-discord"
```

An inline mapping can override a preset for one updater. For example, add `proxy` or `protocols` beside `use`. The updater-level value takes precedence over the global value and the inline driver value takes precedence for that driver.

## Drivers

Choose one provider and at least one resolver in every updater. Optional notifiers run after update activity. The available drivers and their complete fields are documented in:

**[Resolvers](resolver.md):**

- [`ip_service`](resolver.md#ip_service) - external HTTP IP service
- [`netif`](resolver.md#netif) - network interface
- [`shell`](resolver.md#shell) - shell command

**[Providers](provider.md):**

- [`dyndns`](provider.md#dyndns) - any DynDNS 2 compatible provider
- [`ionos`](provider.md#ionos) - [IONOS](https://www.ionos.de)
- [`scaleway`](provider.md#scaleway) - [Scaleway](https://www.scaleway.com)
- [`strato`](provider.md#strato) - [Strato](https://www.strato.de)

**[Notifiers](notifier.md):**

- [`discord`](notifier.md#discord) - Discord webhook notifier

## Checking a configuration

Validate before starting the daemon:

```shell
xddns config validate
```

To inspect the fully resolved configuration, use `xddns config show`. Secrets are redacted by default,  `--show-secrets` prints them and should be used carefully.
