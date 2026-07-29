// Package config reads Haft's global optional-runtime configuration.
//
// Config file: ~/.haft/config.yaml
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the retained global configuration consumed by optional embedding
// compatibility. Unknown historical fields remain readable because the YAML
// decoder ignores them.
type Config struct {
	Embedding EmbeddingConfig `yaml:"embedding,omitempty" json:"embedding,omitempty"`
}

// EmbeddingConfig controls the optional hybrid-recall embedding layer. An empty
// Provider means "auto": use the local EmbeddingGemma sidecar when installed,
// otherwise FTS5+PPR only. Set Provider to "none" to disable embeddings even
// when the sidecar is present, or "openai" to use the API backend.
type EmbeddingConfig struct {
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"` // local (default) | openai | none
	Model    string `yaml:"model,omitempty" json:"model,omitempty"`       // provider-specific model id
	Dim      int    `yaml:"dim,omitempty" json:"dim,omitempty"`           // MRL-truncated dimension (0 = native)
}

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
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
