// Package config parses sift's YAML configuration. The per-provider `config`
// block is kept as a raw yaml.Node so each provider can decode it into its own
// typed structure — keeping configuration and provider code colocated.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default file locations, matching the user-local layout used by sibling CLIs:
// ~/.sift/config.yaml and ~/.sift/sift.db.
const (
	AppDirName        = ".sift"
	DefaultConfigName = "config.yaml"
	DefaultDBName     = "sift.db"
)

// DefaultDir returns sift's user-local configuration/data directory.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, AppDirName), nil
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultConfigName), nil
}

// DefaultDBPath returns the default SQLite cache path.
func DefaultDBPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultDBName), nil
}

// Config is the top-level configuration.
type Config struct {
	Cache     CacheConfig      `yaml:"cache"`
	Providers []ProviderConfig `yaml:"providers"`
}

// CacheConfig configures persistence.
type CacheConfig struct {
	DBPath string `yaml:"db_path"`
}

// ProviderConfig activates and parameterizes one provider instance. The
// provider-specific settings live under Config (decoded by the provider).
type ProviderConfig struct {
	Name    string    `yaml:"name"`
	Enabled bool      `yaml:"enabled"`
	Config  yaml.Node `yaml:"config"`
}

// Decode unmarshals the provider's raw config block into dst (a pointer to the
// provider's typed config struct). A missing/empty block is a no-op so that
// providers can rely on their own zero-value defaults.
func (p ProviderConfig) Decode(dst any) error {
	if p.Config.Kind == 0 {
		return nil
	}
	if err := p.Config.Decode(dst); err != nil {
		return fmt.Errorf("provider %q: invalid config: %w", p.Name, err)
	}
	return nil
}

// Load reads and parses the config file at path, applying defaults and
// validating the result.
func Load(path string) (*Config, error) {
	if path == "" {
		defaultPath, err := DefaultConfigPath()
		if err != nil {
			return nil, err
		}
		path = defaultPath
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() error {
	if c.Cache.DBPath == "" {
		path, err := DefaultDBPath()
		if err != nil {
			return err
		}
		c.Cache.DBPath = path
	}
	return nil
}

// Validate checks the configuration for obvious mistakes.
func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("config: no providers configured")
	}
	seen := map[string]bool{}
	for i, p := range c.Providers {
		if p.Name == "" {
			return fmt.Errorf("config: providers[%d] has no name", i)
		}
		if seen[p.Name] {
			return fmt.Errorf("config: duplicate provider %q", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// EnabledProviders returns only the provider configs with enabled: true.
func (c *Config) EnabledProviders() []ProviderConfig {
	var out []ProviderConfig
	for _, p := range c.Providers {
		if p.Enabled {
			out = append(out, p)
		}
	}
	return out
}

// EnsureDirs creates the parent directory of the cache DB if it doesn't exist.
func (c *Config) EnsureDirs() error {
	if dir := filepath.Dir(c.Cache.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create cache dir: %w", err)
		}
	}
	return nil
}
