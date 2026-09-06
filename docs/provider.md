# Provider

## `dyndns`

```yaml
# Protocols to use for this provider (optional)
protocols: []
# Proxy settings to use for this provider (optional)
proxy:
  # Proxy URL
  url: ""
  # Username for proxy authentication (optional)
  username: ""
  # Password for proxy authentication (optional)
  password: ""
# DynDNS update URL (e.g. https://example.com/nic/update)
url: ""
# List of domain names to update (e.g. example.com, sub.example.com)
domain: []
# Username for DynDNS authentication
username: ""
# Password for DynDNS authentication
password: ""
```

## `ionos`

```yaml
# Protocols to use (optional)
protocols: []
# Proxy settings to use (optional)
proxy:
  # Proxy URL
  url: ""
  # Username for proxy authentication (optional)
  username: ""
  # Password for proxy authentication (optional)
  password: ""
# List of domain names to update (e.g. example.com, sub.example.com). Provider either the domains OR update_url.
domain: []
# Update URL (e.g. https://ipv4.api.hosting.ionos.com/dns/v1/dyndns?q=...)
update_url: ""
# Public prefix
public_prefix: ""
# Secret
secret: ""
```

## `scaleway`

```yaml
# Protocols to use for this provider (optional)
protocols: []
# Proxy settings to use for this provider (optional)
proxy:
  # Proxy URL
  url: ""
  # Username for proxy authentication (optional)
  username: ""
  # Password for proxy authentication (optional)
  password: ""
# List of domain names to update (e.g. example.com, sub.example.com)
domain: []
# Scaleway project ID
project_id: ""
# Scaleway access key
access_key: ""
# Scaleway secret key
secret_key: ""
# Zone name for the DNS records (optional)
zone: ""
# TTL for the DNS record (default: 150) (optional)
ttl: 0
```

## `strato`

```yaml
# Protocols to use for this provider (optional)
protocols: []
# Proxy settings to use for this provider (optional)
proxy:
  # Proxy URL
  url: ""
  # Username for proxy authentication (optional)
  username: ""
  # Password for proxy authentication (optional)
  password: ""
# List of domain names to update (e.g. example.com, sub.example.com)
domain: []
# Username for Strato authentication
username: ""
# Password for Strato authentication
password: ""
```
