package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunInitPiMaterializesPackageAndRegistersSettings(t *testing.T) {
	projectRoot := t.TempDir()

	runInitPi(projectRoot)

	packageDir := filepath.Join(projectRoot, ".haft", "pi", "haft-pi")
	for _, relPath := range []string{
		"package.json",
		"extensions/haft/index.ts",
		"extensions/haft/bridge.ts",
		"extensions/haft/tools.ts",
		"prompts/h-reason.md",
		"prompts/h-decide.md",
		"skills/h-method/SKILL.md",
		"skills/fpf-development/SKILL.md",
		"skills/fpf-semiotics/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(packageDir, filepath.FromSlash(relPath))); err != nil {
			t.Fatalf("expected materialized file %s: %v", relPath, err)
		}
	}

	settings := readPiSettingsForTest(t, projectRoot)
	packages, _ := settings["packages"].([]any)
	if len(packages) != 1 || packages[0] != "../.haft/pi/haft-pi" {
		t.Fatalf("expected single local-path package entry, got %#v", packages)
	}
}

func TestRunInitPiIsIdempotentAndPreservesForeignSettings(t *testing.T) {
	projectRoot := t.TempDir()
	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"theme": "dark", "packages": ["npm:@foo/bar@1.0.0"]}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	runInitPi(projectRoot)
	runInitPi(projectRoot)

	settings := readPiSettingsForTest(t, projectRoot)
	if settings["theme"] != "dark" {
		t.Fatalf("expected foreign settings preserved, got %#v", settings)
	}
	packages, _ := settings["packages"].([]any)
	if len(packages) != 2 {
		t.Fatalf("expected exactly two package entries after double init, got %#v", packages)
	}
	if packages[0] != "npm:@foo/bar@1.0.0" || packages[1] != "../.haft/pi/haft-pi" {
		t.Fatalf("unexpected package entries: %#v", packages)
	}
}

func TestRegisterPiPackageMigratesLegacyEntry(t *testing.T) {
	projectRoot := t.TempDir()
	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Entry written by earlier haft builds: resolved relative to .pi/ by pi
	// and silently skipped.
	legacy := `{"packages": ["./.haft/pi/haft-pi"]}`
	if err := os.WriteFile(settingsPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := registerPiPackage(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected legacy entry migration to report a change")
	}

	settings := readPiSettingsForTest(t, projectRoot)
	packages, _ := settings["packages"].([]any)
	if len(packages) != 1 || packages[0] != "../.haft/pi/haft-pi" {
		t.Fatalf("expected legacy entry replaced in place, got %#v", packages)
	}
}

func TestNormalizeInitHostOptionsPiSuppressesClaudeDefault(t *testing.T) {
	normalized := normalizeInitHostOptions(initHostOptions{pi: true})
	if normalized.claude {
		t.Fatal("expected --pi alone not to default to Claude host config")
	}
	if !normalized.pi {
		t.Fatal("expected pi host selection to survive normalization")
	}
}

func TestHasPiEntryRecognizesObjectForm(t *testing.T) {
	packages := []any{
		map[string]any{"source": "../.haft/pi/haft-pi", "skills": []any{}},
	}
	if !hasPiEntry(packages) {
		t.Fatal("expected object-form entry to be recognized")
	}
}

func readPiSettingsForTest(t *testing.T, projectRoot string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, ".pi", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return settings
}
