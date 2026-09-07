# Resolver

## `ip_service`

Resolves the public IP address by querying an external IP service.

```yaml
# Protocols to use for this resolver (optional)
protocols: []
# Proxy settings to use for this resolver (optional)
proxy:
  # Proxy URL
  url: ""
  # Username for proxy authentication (optional)
  username: ""
  # Password for proxy authentication (optional)
  password: ""
# IP service URLs to query for the public IP address (e.g. https://api.ipify.org, https://ifconfig.me/ip)
url: []
```

## `netif`

Resolves the public IP address by querying the specified network interface.

```yaml
# Protocols to use for this resolver (optional)
protocols: []
# List of the network interface to use for resolving the public IP address (e.g. eth0, en0)
interface: []
```

## `shell`

Resolves the public IP address from the output of a local shell command.

```yaml
# Protocols to use for this resolver (optional)
protocols: []
# Shell command, output must contain one or more public IP addresses
command: ""
```
