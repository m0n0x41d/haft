package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	haftpi "github.com/m0n0x41d/haft/packages/haft-pi"
)

// haft init --pi materializes the embedded @haft/pi package into the project
// and registers it as a local-path Pi package. No npm step is needed: the
// extension's only runtime import (typebox) is a Pi-bundled core package
// declared via peerDependencies, and Pi loads local-path packages as-is.

var initPi bool

const (
	piPackageRelDir = ".haft/pi/haft-pi"
	piSettingsEntry = "./" + piPackageRelDir
)

func init() {
	initCmd.Flags().BoolVar(&initPi, "pi", false, "Configure for Pi — materializes the bundled @haft/pi package and registers it in .pi/settings.json")
}

func runInitPi(projectRoot string) {
	packageDir := filepath.Join(projectRoot, filepath.FromSlash(piPackageRelDir))
	if err := materializePiPackage(packageDir); err != nil {
		fmt.Printf("  ⚠ Failed to materialize @haft/pi package: %v\n", err)
		return
	}
	fmt.Printf("  ✓ Materialized @haft/pi package (%s)\n", piPackageRelDir)

	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	added, err := registerPiPackage(settingsPath)
	if err != nil {
		fmt.Printf("  ⚠ Failed to update .pi/settings.json: %v\n", err)
		return
	}
	if added {
		fmt.Println("  ✓ Registered package in .pi/settings.json (loads after project trust)")
	} else {
		fmt.Println("  ✓ Package already registered in .pi/settings.json")
	}
	fmt.Println("    Note: native haft_* tools, /h-* prompts, and FPF skills load on next Pi session")
}

// materializePiPackage writes the embedded package fresh on every init so the
// carriers stay in lockstep with the kernel inside this binary.
func materializePiPackage(packageDir string) error {
	return fs.WalkDir(haftpi.Assets, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(packageDir, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, readErr := haftpi.Assets.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// registerPiPackage merges the local-path package entry into .pi/settings.json,
// preserving everything else in the file. Returns whether the entry was added.
func registerPiPackage(settingsPath string) (bool, error) {
	settings, err := readPiSettings(settingsPath)
	if err != nil {
		return false, err
	}

	packages := piPackagesList(settings)
	if hasPiEntry(packages) {
		return false, nil
	}

	settings["packages"] = append(packages, piSettingsEntry)
	return true, writePiSettings(settingsPath, settings)
}

func readPiSettings(settingsPath string) (map[string]any, error) {
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}

	settings := map[string]any{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", settingsPath, err)
	}
	return settings, nil
}

func piPackagesList(settings map[string]any) []any {
	packages, _ := settings["packages"].([]any)
	return packages
}

// hasPiEntry recognizes both the plain string form and the object form with
// resource filters ({"source": "...", ...}) documented by Pi.
func hasPiEntry(packages []any) bool {
	for _, item := range packages {
		if entry, ok := item.(string); ok && entry == piSettingsEntry {
			return true
		}
		if obj, ok := item.(map[string]any); ok {
			if source, _ := obj["source"].(string); source == piSettingsEntry {
				return true
			}
		}
	}
	return false
}

func writePiSettings(settingsPath string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}
