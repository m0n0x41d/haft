// Package config manages haft's persistent configuration.
//
// Config file: ~/.haft/config.yaml
//
// Supports multiple providers with separate credentials.
// One model is the default; --model flag overrides at runtime.
//
// Architecture:
//
//	L0: Config, ProviderAuth — pure data types
//	L1: IsConfigured, GetAuth, ProviderForModel — pure functions
//	L2: Load, Save — file I/O boundary
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// L0: Pure data types
// ---------------------------------------------------------------------------

// Config is the top-level haft configuration.
//
// Example ~/.haft/config.yaml:
//
//	model: gpt-5.4
//	providers:
//	  openai:
//	    api_key: sk-...
//	  anthropic:
//	    api_key: sk-ant-...
//	  deepseek:
//	    api_key: sk-...
type Config struct {
	Model     string                  `yaml:"model" json:"model"`         // default model ID
	Providers map[string]ProviderAuth `yaml:"providers" json:"providers"` // provider ID → auth
}

// ProviderAuth stores credentials for one provider.
type ProviderAuth struct {
	AuthType     string `yaml:"auth_type,omitempty" json:"auth_type,omitempty"` // "api_key", "codex_oauth"
	APIKey       string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	AccessToken  string `yaml:"access_token,omitempty" json:"access_token,omitempty"`
	RefreshToken string `yaml:"refresh_token,omitempty" json:"refresh_token,omitempty"`
	ExpiresAt    int64  `yaml:"expires_at,omitempty" json:"expires_at,omitempty"`
	AccountID    string `yaml:"account_id,omitempty" json:"account_id,omitempty"`
}

// ---------------------------------------------------------------------------
// L1: Pure functions
// ---------------------------------------------------------------------------

// IsConfigured returns true if at least one provider has credentials and a default model is set.
func (c *Config) IsConfigured() bool {
	if c.Model == "" {
		return false
	}
	for _, auth := range c.Providers {
		if auth.APIKey != "" || auth.AccessToken != "" {
			return true
		}
	}
	return false
}

// GetAuth returns credentials for a provider, or zero value if not configured.
func (c *Config) GetAuth(providerID string) ProviderAuth {
	if c.Providers == nil {
		return ProviderAuth{}
	}
	return c.Providers[providerID]
}

// SetAuth sets credentials for a provider.
func (c *Config) SetAuth(providerID string, auth ProviderAuth) {
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderAuth)
	}
	c.Providers[providerID] = auth
}

// ConfiguredProviders returns the list of provider IDs with credentials.
func (c *Config) ConfiguredProviders() []string {
	var ids []string
	for id, auth := range c.Providers {
		if auth.APIKey != "" || auth.AccessToken != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// ProviderForModel guesses which provider a model belongs to based on ID prefix.
// Returns empty string if unknown.
func ProviderForModel(modelID string) string {
	prefixes := map[string]string{
		"gpt-":      "openai",
		"o1":        "openai",
		"o3":        "openai",
		"o4":        "openai",
		"claude-":   "anthropic",
		"gemini-":   "google",
		"deepseek-": "deepseek",
		"llama-":    "groq",
		"mistral":   "mistral",
	}
	for prefix, provider := range prefixes {
		if len(modelID) >= len(prefix) && modelID[:len(prefix)] == prefix {
			return provider
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// L2: File I/O
// ---------------------------------------------------------------------------

const (
	haftDir    = ".haft"
	configFile = "config.yaml"
)

// HaftDir returns the global haft config directory path.
func HaftDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", haftDir)
	}
	return filepath.Join(home, haftDir)
}

// ConfigPath returns the full path to config.yaml.
func ConfigPath() string {
	return filepath.Join(HaftDir(), configFile)
}

// Load reads config from ~/.haft/config.yaml. Returns zero config if not found.
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Providers: make(map[string]ProviderAuth)}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderAuth)
	}
	return &cfg, nil
}

// Save writes config to ~/.haft/config.yaml.
func Save(cfg *Config) error {
	dir := HaftDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

// MigrateOldAuth migrates from the old separate auth.json to the unified config.
// Called once on startup. No-op if already migrated.
func MigrateOldAuth(cfg *Config) bool {
	oldAuthPath := filepath.Join(HaftDir(), "auth.json")
	data, err := os.ReadFile(oldAuthPath)
	if err != nil {
		return false
	}
	var old struct {
		Provider string `json:"provider"`
		AuthType string `json:"auth_type"`
		APIKey   string `json:"api_key"`
	}
	if json.Unmarshal(data, &old) != nil || old.APIKey == "" {
		return false
	}
	cfg.SetAuth(old.Provider, ProviderAuth{
		AuthType: old.AuthType,
		APIKey:   old.APIKey,
	})
	_ = os.Remove(oldAuthPath) // clean up old file
	return true
}
