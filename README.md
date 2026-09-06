# XDDNS

XDDNS is a dynamic DNS client for keeping DNS records aligned with your current IP address. It supports multiple IP resolvers, DNS providers, and notification services, and can run once from the command line or continuously as a service.

## Installation

### Using `go`

```shell
go install github.com/iceflowre/xddns/cmd/xddns@latest
```

### Arch Linux

#### AUR

```shell
yay -S xddns
```

or

```shell
paru -S xddns
```

## Quick start

Create `xddns.yaml` in the current directory, or use `--config /path/to/xddns.yaml` to select a file explicitly. The file must be readable and writable only by its owner:

```shell
chmod 600 xddns.yaml
```

The smallest useful configuration has one provider, one resolver and one updater. Replace the example values with credentials and names from your DNS provider:

```yaml
updaters:
  - name: "home.example.com"
    provider:
      type: "dyndns"
      url: "https://example.com/nic/update"
      domain: "home.example.com"
      username: "user"
      password: "secret"
    resolvers:
      - type: "ip_service"
        url: "https://api.ipify.org"
```

Run one update before enabling continuous operation, the update command also supports `--dry-run` to test the configuration without making changes:

```shell
xddns config validate
xddns update
```

Then start the daemon:

```shell
xddns daemon
```

See [Configuration](docs/configuration.md) for file discovery, presets, shared settings, and provider-specific options. See [CLI](docs/cli.md) for all commands.

## Supported drivers

**[Resolvers](docs/resolver.md):**

- [`ip_service`](docs/resolver.md#ip_service) - external HTTP IP service
- [`netif`](docs/resolver.md#netif) - network interface

**[Providers](docs/provider.md):**

- [`dyndns`](docs/provider.md#dyndns) - any DynDNS 2 compatible provider
- [`ionos`](docs/provider.md#ionos) - [IONOS](https://www.ionos.de)
- [`scaleway`](docs/provider.md#scaleway) - [Scaleway](https://www.scaleway.com)
- [`strato`](docs/provider.md#strato) - [Strato](https://www.strato.de)

**[Notifiers](docs/notifier.md):**

- [`discord`](docs/notifier.md#discord) - Discord webhook notifier

### Configuration

Updater names must **not** contain any sensitive information like passwords or API keys. The updater name is used for logging and notifications.

```yaml
updaters:
  - name: "my-domain.com"
    provider:
      type: "dyndns"
      domain: "my-domain.com"
      url: "https://example.com/nic/update"
    resolvers:
      - type: "netif"
        interface: "eth0"
      - type: "ip_service"
        url:
          - "https://api.ipify.org"
          - "https://ifconfig.me/ip"
```

## Development

Each driver type (`notifier`, `provider`, `resolver`) has its own interface that must be implemented. Returned errors should not be logged as they are logged by the caller. The driver should only log errors that are not returned to the caller.

Additionally drivers configuration structs can implement the `ProtocolAware` and/or `ProxyAware` interfaces to support protocol and proxy settings. The most easiest way is to embed `config.Protocols` and `config.Proxy` into the driver configuration struct.

### Driver configurations

Each configuration type (`notifier`, `provider`, `resolver`) has its own configuration structure, but should not use the following reserved keys.

- `protocols`
- `proxy`
- `type`
- `use`

## Disclaimer

All product names, trademarks, service marks, logos, and brands mentioned in this repository are property of their respective owners. This project is an independent open-source tool and is not affiliated, associated, authorized, endorsed by, or in any way officially connected with any of the companies, providers, or services referenced in this project.

## License

MIT License

Copyright (c) 2026-present Iceflower S <iceflower@iceflower.eu>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
