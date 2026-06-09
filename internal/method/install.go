package method

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	CatalogID           string   `yaml:"catalog_id"`
	CatalogVersion      string   `yaml:"catalog_version"`
	HaftFeature         string   `yaml:"haft_feature"`
	MaterializedAt      string   `yaml:"materialized_at"`
	LocalOverridePolicy string   `yaml:"local_override_policy"`
	Methods             []string `yaml:"methods"`
}

func InstallDefaultCatalog(haftDir string) error {
	catalog := BuiltinCatalog()
	if err := ValidateCatalog(catalog); err != nil {
		return err
	}

	dir := filepath.Join(haftDir, "methods", catalog.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create method catalog dir: %w", err)
	}

	manifest := Manifest{
		CatalogID:           catalog.ID,
		CatalogVersion:      catalog.Version,
		HaftFeature:         "methodpack-v1",
		MaterializedAt:      time.Now().UTC().Format(time.RFC3339),
		LocalOverridePolicy: "preserve_existing_files",
		Methods:             methodIDs(catalog.Methods),
	}
	if err := writeYAMLIfMissing(filepath.Join(dir, "manifest.yaml"), manifest); err != nil {
		return err
	}

	for _, definition := range catalog.Methods {
		path := filepath.Join(dir, definition.ID+".yaml")
		if err := writeYAMLIfMissing(path, definition); err != nil {
			return err
		}
	}

	return nil
}

func methodIDs(definitions []Definition) []string {
	ids := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		ids = append(ids, definition.ID)
	}
	return ids
}

func writeYAMLIfMissing(path string, value any) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
