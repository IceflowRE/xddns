package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/rs/zerolog"
)

var (
	ErrInsecureConfigFile = errors.New("config file has insecure permissions")
	ErrNoConfigFound      = errors.New("no configuration file found in standard locations")
)

const (
	// EnvConfigPath is the environment variable that can be used to specify the path to the configuration file.
	EnvConfigPath = "XDDNS_CONFIG"
	// DefaultUpdateInterval is the default interval between automatic updates.
	DefaultUpdateInterval = 10 * time.Minute
)

// DefaultProtocols returns the default protocols to use for all updaters, unless overridden by individual updater settings.
func DefaultProtocols() Protocols {
	return Protocols{IPv4: true, IPv6: true}
}

// Preparer is implemented by driver configurations that prepare and validate their configuration.
type Preparer interface {
	// Prepare mutates a config into its canonical, usable form (defaults applied, values normalized) and validates the result in the same pass.
	// It must be idempotent.
	Prepare() []error
}

// Config is the main configuration struct for the application.
type Config struct {
	Interval  time.Duration `yaml:"update_interval,omitempty" comment:"Interval between automatic updates (default: 10m)"`
	LogLevel  string        `yaml:"log_level,omitempty" comment:"Log level for the application (default: info)"`
	Protocols Protocols     `yaml:"protocols,omitempty" comment:"Protocols to use for all updaters, unless overridden by individual updater settings (default: ipv4, ipv6)"` //nolint:lll
	Proxy     Proxy         `yaml:"proxy,omitempty" comment:"Proxy settings to use for all updaters, unless overridden by individual updater settings (optional)"`

	Presets  Presets    `yaml:"presets,omitempty" comment:"Presets for providers, resolvers, and notifiers"`
	Updaters []*Updater `yaml:"updaters" comment:"List of updaters to run"`
}

// NewConfig creates a new Config instance with default values.
func NewConfig() *Config {
	return &Config{
		Interval:  DefaultUpdateInterval,
		LogLevel:  zerolog.LevelInfoValue,
		Protocols: DefaultProtocols(),
		Presets: Presets{
			Notifiers: map[string]*Preset{},
			Providers: map[string]*Preset{},
			Resolvers: map[string]*Preset{},
		},
		Updaters: []*Updater{},
	}
}

// Presets are the global presets for providers, resolvers, and notifiers that can be referenced by updaters.
type Presets struct {
	Notifiers map[string]*Preset `yaml:"notifiers,omitempty" comment:"Presets for notifiers"`
	Providers map[string]*Preset `yaml:"providers,omitempty" comment:"Presets for providers"`
	Resolvers map[string]*Preset `yaml:"resolvers,omitempty" comment:"Presets for resolvers"`
}

// IsZero implements IsZeroer.
func (p *Presets) IsZero() bool {
	return len(p.Notifiers) == 0 && len(p.Providers) == 0 && len(p.Resolvers) == 0
}

// LoadConfig loads the configuration from the specified path or from standard locations if the path is empty.
// Returns the loaded Config, the path from which it was loaded, and any error encountered.
func LoadConfig(path string) (*Config, string, error) {
	candiates := []string{path}
	if path == "" {
		candiates = configPathCandidates()
	}

	for _, candidate := range candiates {
		err := validateConfigFilePermissions(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, candidate, err
		}

		cfg, err := loadConfigFromFile(candidate)
		if err != nil {
			return nil, candidate, err
		}

		return cfg, candidate, nil
	}

	return nil, "", ErrNoConfigFound
}

// FindConfigPath checks candidate locations in order and returns the first existing path.
func FindConfigPath() string {
	for _, path := range configPathCandidates() {
		if fileExists(path) {
			return path
		}
	}

	return ""
}

func loadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}

	cfg := NewConfig()
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateConfigFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	mode := info.Mode().Perm()

	if isSystemdCredential(path) {
		// systemd credential files are set to 0440 by default.
		if mode&0o0037 != 0 {
			return fmt.Errorf("%w (%04o): %q. Unexpected permissions for a systemd credential file", ErrInsecureConfigFile, mode, path)
		}

		return nil
	}

	if mode&0o0077 != 0 {
		return fmt.Errorf("%w (%04o): %q. It should be readable and writeable only by the owner (0600)", ErrInsecureConfigFile, mode, path)
	}

	return nil
}

func configPathCandidates() []string {
	// if env var is set, use that as the only candidate
	envPath := os.Getenv(EnvConfigPath)
	if envPath != "" {
		return []string{envPath}
	}

	var candidates []string

	sysdConfigDir := os.Getenv("CONFIGURATION_DIRECTORY")
	if sysdConfigDir != "" {
		for _, dir := range filepath.SplitList(sysdConfigDir) {
			candidates = append(candidates,
				filepath.Join(dir, "xddns.yaml"),
				filepath.Join(dir, "xddns.yml"),
			)
		}
	}
	candidates = append(candidates,
		"xddns.yaml",
		"xddns.yml",
	)

	// User-Specific Config Directory
	// Linux/macOS: ~/.config/xddns/config.yaml
	// Windows: %APPDATA%\xddns\config.yaml
	userConfigDir, err := os.UserConfigDir()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(userConfigDir, "xddns.yaml"),
			filepath.Join(userConfigDir, "xddns.yml"),
			filepath.Join(userConfigDir, "xddns/xddns.yaml"),
			filepath.Join(userConfigDir, "xddns/xddns.yml"),
		)
	}

	// System-Wide Directory (OS-Specific)
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData != "" {
			candidates = append(candidates,
				filepath.Join(programData, "xddns/xddns.yaml"),
				filepath.Join(programData, "xddns/xddns.yml"),
			)
		}
	} else {
		// Linux / Unix System Paths
		candidates = append(candidates,
			"/etc/xddns.yaml",
			"/etc/xddns.yml",
		)
	}

	return candidates
}

// isSystemdCredential reports whether path resides under the directory systemd exposes for LoadCredential=/SetCredential=.
func isSystemdCredential(path string) bool {
	credDir := os.Getenv("CREDENTIALS_DIRECTORY")
	if credDir == "" {
		return false
	}

	absCredDir, err := filepath.Abs(credDir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absCredDir, absPath)
	if err != nil {
		return false
	}

	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
