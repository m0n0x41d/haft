package project

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents .haft/project.yaml — the stable project identity.
type Config struct {
	ID   string `yaml:"id"`   // immutable, generated on init (e.g., "qnt_a7f3b2c1")
	Name string `yaml:"name"` // human-readable, from directory name
}

const configFile = "project.yaml"

// Load reads project config from .haft/project.yaml.
// Returns nil if file doesn't exist (pre-migration project).
func Load(haftDir string) (*Config, error) {
	path := filepath.Join(haftDir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	if cfg.ID == "" {
		return nil, fmt.Errorf("project config has empty ID")
	}

	return &cfg, nil
}

// Create generates a new project config and writes it to .haft/project.yaml.
// The ID is immutable — if project.yaml already exists, returns the existing config.
func Create(haftDir string, projectRoot string) (*Config, error) {
	// If already exists, return existing
	existing, err := Load(haftDir)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// Generate new ID
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate project ID: %w", err)
	}

	cfg := &Config{
		ID:   id,
		Name: filepath.Base(projectRoot),
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal project config: %w", err)
	}

	path := filepath.Join(haftDir, configFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("write project config: %w", err)
	}

	return cfg, nil
}

// DBDir returns the path to this project's DB directory in the unified store.
// Creates the directory if it doesn't exist.
func (c *Config) DBDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(homeDir, ".haft", "projects", c.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create project DB dir: %w", err)
	}

	return dir, nil
}

// DBPath returns the full path to this project's SQLite database.
func (c *Config) DBPath() (string, error) {
	dir, err := c.DBDir()
	if err != nil {
		return "", err
	}
	// Check for haft.db first, then legacy quint.db (migration)
	haftDB := filepath.Join(dir, "haft.db")
	if _, err := os.Stat(haftDB); err == nil {
		return haftDB, nil
	}
	legacyDB := filepath.Join(dir, "quint.db")
	if _, err := os.Stat(legacyDB); err == nil {
		// Rename legacy DB in place
		_ = os.Rename(legacyDB, haftDB)
		return haftDB, nil
	}
	return haftDB, nil // new DB will be created here
}

// IndexDBPath returns the path to the global cross-project index.
func IndexDBPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(homeDir, ".haft")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create haft dir: %w", err)
	}

	return filepath.Join(dir, "index.db"), nil
}

func generateID() (string, error) {
	bytes := make([]byte, 4) // 4 bytes = 8 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "qnt_" + hex.EncodeToString(bytes), nil
}
