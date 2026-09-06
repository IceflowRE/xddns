package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iceflowre/xddns/xddns"
	"github.com/iceflowre/xddns/xddns/config"
	// Load all drivers to ensure they are registered for testing.
	_ "github.com/iceflowre/xddns/xddns/notifier/all"
	_ "github.com/iceflowre/xddns/xddns/provider/all"
	_ "github.com/iceflowre/xddns/xddns/resolver/all"
)

func loadYAMLSchema(t testing.TB) *jsonschema.Schema {
	t.Helper()

	file, err := os.Open(filepath.Join("..", "..", "docs", "xddns.schema.json"))
	require.NoError(t, err)
	defer file.Close()

	schema, err := jsonschema.UnmarshalJSON(file)
	require.NoError(t, err)

	comp := jsonschema.NewCompiler()
	err = comp.AddResource("xddns.schema.json", schema)
	require.NoError(t, err)
	compiledSchema, err := comp.Compile("xddns.schema.json")
	require.NoError(t, err)

	return compiledSchema
}

func FuzzConfigYAMLSchemaValidation(f *testing.F) {
	schema := loadYAMLSchema(f)

	for _, seed := range [][]byte{
		[]byte(`
updaters:
  - name: home
    provider:
      type: dyndns
      url: https://dyn.example.com/update
      domain: home.example.com
    resolvers:
      - type: ip_service
        url: https://api.ipify.org
`),
		[]byte(`
updaters:
  - name: ionos-domain
    provider:
      type: ionos
      public_prefix: abc123
      secret: shh
      domain: [a.example.com, b.example.com]
    resolvers:
      - type: netif
        interface: eth0
`),
		[]byte(`
updaters:
  - name: ionos-url
    provider:
      type: ionos
      public_prefix: abc123
      secret: shh
      update_url: https://ipv4.ionos.example/update
    resolvers:
      - type: netif
        interface: eth0
`),
		[]byte(`
updaters:
  - name: sw
    provider:
      type: scaleway
      domain: example.com
      project_id: p1
      access_key: ak
      secret_key: sk
      ttl: 300
    resolvers:
      - type: ip_service
        url: [https://a.example, https://b.example]
    notifiers:
      - type: discord
        url: https://discord.com/api/webhooks/x/y
`),
		[]byte(`
updaters:
  - name: st
    provider:
      type: strato
      username: user
      password: pass
      domain: example.com
    resolvers:
      - type: netif
        interface: [eth0, wlan0]
    notifiers:
      - type: discord
        id: "12345"
        token: tok
        footer: bye
`),
		[]byte(`
presets:
  providers:
    myProvider:
      type: dyndns
      url: https://dyn.example.com/update
      domain: home.example.com
  resolvers:
    myResolver:
      type: ip_service
      url: https://api.ipify.org
updaters:
  - name: preset-based
    provider: myProvider
    resolvers:
      - myResolver
      - type: netif
        interface: eth0
`),
		[]byte(`
update_interval: 10m
log_level: debug
protocols: [ipv4, ipv6]
proxy: false
updaters:
  - name: x
    provider:
      type: ip_service
      url: https://x.example
    resolvers:
      - type: netif
        interface: eth0
`),
		[]byte(`
proxy: http://user:pass@proxy.example:8080
updaters:
  - name: x
    provider:
      type: netif
      interface: eth0
    resolvers:
      - type: netif
        interface: eth0
`),
		[]byte(`
proxy:
  url: http://proxy.example:8080
  username: u
  password: p
updaters:
  - name: x
    provider:
      type: netif
      interface: eth0
    resolvers:
      - type: netif
        interface: eth0
`),
		// both type and use present — schema says exactly one via oneOf
		[]byte(`
updaters:
  - name: x
    provider:
      type: dyndns
      use: something
      url: https://x
      domain: d
    resolvers:
      - type: netif
        interface: eth0
`),
		// ionos with BOTH domain and update_url — the oneOf should reject this
		[]byte(`
updaters:
  - name: x
    provider:
      type: ionos
      public_prefix: p
      secret: s
      domain: d.example.com
      update_url: https://x
    resolvers:
      - type: netif
        interface: eth0
`),
		// discord with neither url nor id+token
		[]byte(`
updaters:
  - name: x
    provider:
      type: netif
      interface: eth0
    resolvers:
      - type: netif
        interface: eth0
    notifiers:
      - type: discord
`),
		// empty updaters array — valid per schema (no minItems), make sure your Go side agrees
		[]byte(`updaters: []`),
		// unknown top-level key — additionalProperties: false should reject
		[]byte(`
foo: bar
updaters: []
`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		cfg := &config.Config{}

		err := yaml.Unmarshal(data, cfg)
		if err != nil {
			return
		}

		schemaErr := schema.Validate(cfg)
		structErr := xddns.ValidateConfig(cfg)

		assert.True(t, (schemaErr == nil) == (structErr == nil), "schema/struct validation disagree\ninput: %s\nschemaErr: %v\nstructErr: %v",
			data, schemaErr, structErr)
	})
}
