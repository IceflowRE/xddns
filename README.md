# XDDNS

XDDNS is a dynamic DNS client that supports multiple resolvers, providers and notifiers. The application can be run as a command-line tool or as a system service.

## Install

### Archlinux

Package for [xddns](https://github.com/IceflowRE/xddns).

#### AUR

```shell
yay -S xddns
```

```shell
paru -S xddns
```

#### From Source

```shell
sudo pacman -S --needed git base-devel
git clone https://github.com/IceflowRE/xddns.git
cd xddns/packaging/arch
makepkg -si
```

```shell
sudo pacman -S --needed git base-devel && git clone https://github.com/IceflowRE/xddns.git && cd xddns/packaging/arch && makepkg -si
```

## Configure

**Resolver:**

- [`netif`](docs/resolver.md#netif) - network interface resolver
- [`ip_service`](docs/resolver.md#ip_service) - external HTTP IP service resolver

**Provder:**

- [`scaleway`](docs/provider.md#scaleway) - [Scaleway](https://www.scaleway.com)
- [`strato`](docs/provider.md#strato) - [Strato](https://www.strato.de)
- [`dyndns`](docs/provider.md#dyndns) - any DynDNS 2 compatible provider

**Notifier:**

- [`discord`](docs/notifier.md#discord) - Discord webhook notifier

### Example

```yaml
protocols:
  - ipv4
  - ipv6

proxy:
  url: http://proxy.example.com:8080
  username: user
  password: pass

update:
  - name: "my-domain.com"
    provider:
      type: "dyndns"
      domain: "my-domain.com"
      url: "https://example.com/nic/update"
    resolvers:
      - type: "netif"
        interface: "eth0"
      - type: "ip_service"
        url: "https://api.ipify.org"
```

### Deep Dive

The simple configuration is a minimal configuration meant for single domains.

If you have a lot more domains to update you can use presets. To use a global preset in an updater reference it by the key `use`. Presets can be defined for resolvers, providers and notifiers.

```yaml
presets:
  providers:
    scaleway-private-account:
      type: "scaleway"
  notifiers:
    discord-mow-server:
      type: "discord"

presets:
  providers:
    scaleway-private-account:
      type: "scaleway"
  notifiers:
    discord-mow-server:
      type: "discord"
```

## Development

### Configuration

Each configuration type (`notifier`, `provider`, `resolver`) has its own configuration structure, but with reserved keys that cannot be used by implementations.

- `type`
- `notifier`
- `protocols`
- `provider`
- `proxy`
- `resolver`
- `use`

The global `proxy` and `protocols` fields are merged into all presets and made available if the type claims to support them.
