package project

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/projectidentity"
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
		return Repair(haftDir, projectRoot, existing)
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

	if err := writeConfig(haftDir, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func ProposeConfig(projectRoot string) (Config, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve project root: %w", err)
	}
	root = filepath.Clean(root)
	id, err := generateID()
	if err != nil {
		return Config{}, fmt.Errorf("generate project ID: %w", err)
	}
	return Config{
		ID:   id,
		Name: filepath.Base(root),
	}, nil
}

func PersistExactConfig(
	haftDir string,
	projectRoot string,
	proposal Config,
) (*Config, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	root = filepath.Clean(root)
	if _, err := projectidentity.ParseProjectID(proposal.ID); err != nil {
		return nil, fmt.Errorf("exact project identity: %w", err)
	}
	if proposal.Name != filepath.Base(root) {
		return nil, fmt.Errorf(
			"exact project name %q differs from root name %q",
			proposal.Name,
			filepath.Base(root),
		)
	}
	existing, err := Load(haftDir)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != proposal.ID {
		return nil, fmt.Errorf(
			"project already carries project identity %s; refusing replacement with %s",
			existing.ID,
			proposal.ID,
		)
	}
	if existing != nil && existing.Name == proposal.Name {
		return existing, nil
	}
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		return nil, fmt.Errorf("create project identity directory: %w", err)
	}
	if err := writeConfig(haftDir, &proposal); err != nil {
		return nil, err
	}
	return &proposal, nil
}

// Repair keeps the mutable project metadata aligned with the actual project
// root. The ID remains immutable; the human-readable name is derived from the
// directory name and may be repaired after a moved/copied .haft/ carrier.
func Repair(haftDir string, projectRoot string, cfg *Config) (*Config, error) {
	expectedName := filepath.Base(projectRoot)
	if cfg.Name == expectedName {
		return cfg, nil
	}

	repaired := &Config{
		ID:   cfg.ID,
		Name: expectedName,
	}
	if err := writeConfig(haftDir, repaired); err != nil {
		return nil, err
	}

	return repaired, nil
}

func writeConfig(haftDir string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}

	path := filepath.Join(haftDir, configFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	return nil
}

// DBDir returns the path to this project's DB directory in the unified store.
// Creates the directory if it doesn't exist.
func (c *Config) DBDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	path, err := CanonicalDBPath(homeDir, c.ID)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
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

func CanonicalDBPath(
	userHomeRoot string,
	projectID string,
) (string, error) {
	homeRoot, err := filepath.Abs(userHomeRoot)
	if err != nil {
		return "", fmt.Errorf("resolve user home root: %w", err)
	}
	homeRoot = filepath.Clean(homeRoot)
	project, err := projectidentity.ParseProjectID(projectID)
	if err != nil {
		return "", fmt.Errorf("canonical database project identity: %w", err)
	}
	return filepath.Join(
		homeRoot,
		".haft",
		"projects",
		project.String(),
		"haft.db",
	), nil
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
