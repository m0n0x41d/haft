package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	initHermesHome    string
	initHermesProfile string
)

const hermesSkillsRelDir = "packages/haft-hermes/skills"

func init() {
	initCmd.Flags().BoolVar(&initHermes, "hermes", false, "Configure for Hermes MCP and external skills")
	initCmd.Flags().StringVar(&initHermesHome, "hermes-home", "", "Hermes home directory (default: $HERMES_HOME or ~/.hermes)")
	initCmd.Flags().StringVar(&initHermesProfile, "profile", "", "Hermes profile name for --hermes (uses <home>/profiles/<profile>)")
}

func cleanHermesProfile(profileInput string) (string, error) {
	profile := strings.TrimSpace(profileInput)
	if profile == "" {
		return "", nil
	}
	if filepath.Base(profile) != profile {
		return "", fmt.Errorf("hermes profile must be a name, got %q", profileInput)
	}
	if profile == "." || profile == ".." {
		return "", fmt.Errorf("hermes profile must be a name, got %q", profileInput)
	}
	return profile, nil
}

func expandHermesPath(rawPath string) (string, error) {
	expanded := os.ExpandEnv(strings.TrimSpace(rawPath))
	if expanded == "" {
		return "", fmt.Errorf("empty Hermes path")
	}

	if expanded == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return homeDir, nil
	}

	homePrefix := "~" + string(os.PathSeparator)
	if strings.HasPrefix(expanded, homePrefix) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		relativePath := strings.TrimPrefix(expanded, homePrefix)
		expanded = filepath.Join(homeDir, relativePath)
	}

	return filepath.Abs(expanded)
}

func isHaftSourceRoot(projectRoot string) bool {
	modulePath := goModulePath(projectRoot)
	return modulePath == "github.com/m0n0x41d/haft"
}

func goModulePath(projectRoot string) string {
	data, err := os.ReadFile(filepath.Join(projectRoot, "go.mod"))
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}

func readHermesConfig(configPath string) (map[string]any, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}

	settings := map[string]any{}
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	return settings, nil
}

func withHermesExternalDir(settings map[string]any, skillsRoot string) {
	skills := hermesMapField(settings, "skills")
	externalDirs := hermesListField(skills, "external_dirs")
	if !hasStringListEntry(externalDirs, skillsRoot) {
		externalDirs = append(externalDirs, skillsRoot)
	}

	skills["external_dirs"] = externalDirs
	settings["skills"] = skills
}

func hermesMapField(parent map[string]any, key string) map[string]any {
	value, ok := parent[key]
	if !ok {
		return map[string]any{}
	}

	typed, ok := value.(map[string]any)
	if ok {
		return typed
	}

	return map[string]any{}
}

func hermesListField(parent map[string]any, key string) []any {
	value, ok := parent[key]
	if !ok {
		return []any{}
	}

	typed, ok := value.([]any)
	if ok {
		return typed
	}

	stringsValue, ok := value.([]string)
	if !ok {
		return []any{}
	}

	result := make([]any, 0, len(stringsValue))
	for _, item := range stringsValue {
		result = append(result, item)
	}
	return result
}

func hasStringListEntry(list []any, target string) bool {
	for _, item := range list {
		text, ok := item.(string)
		if ok && text == target {
			return true
		}
	}
	return false
}

func resolveHermesCommand(binaryPath string) string {
	command := strings.TrimSpace(binaryPath)
	if command == "" {
		command = "haft"
	}
	if filepath.IsAbs(command) {
		return command
	}

	resolved, err := exec.LookPath(command)
	if err != nil {
		return command
	}

	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return resolved
	}
	return absPath
}
