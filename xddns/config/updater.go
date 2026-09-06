package config

// Updater represents a configuration for an updater, which includes protocols, proxy settings, provider, resolvers and notifiers.
type Updater struct {
	Protocols *Protocols `yaml:"protocols,omitempty" comment:"Optional protocols for this updater"`
	Proxy     *Proxy     `yaml:"proxy,omitempty" comment:"Optional proxy settings for this updater"`

	Name      string    `yaml:"name" comment:"Name of the updater"`
	Provider  *Preset   `yaml:"provider" comment:"Provider to use for this updater"`
	Resolvers []*Preset `yaml:"resolvers" comment:"List of resolvers to use for this updater"`
	Notifiers []*Preset `yaml:"notifiers,omitempty" comment:"List of notifiers to use for this updater"`
}
