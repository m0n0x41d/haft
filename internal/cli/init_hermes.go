package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	haftconfig "github.com/m0n0x41d/haft/internal/config"
	"gopkg.in/yaml.v3"
)

var (
	initHermesHome    string
	initHermesProfile string
)

const hermesSkillsRelDir = "packages/haft-hermes/skills"

type hermesInstallResult struct {
	ConfigPath string
	SkillsRoot string
	SkillCount int
}

func init() {
	initCmd.Flags().BoolVar(&initHermes, "hermes", false, "Configure for Hermes MCP and external skills")
	initCmd.Flags().StringVar(&initHermesHome, "hermes-home", "", "Hermes home directory (default: $HERMES_HOME or ~/.hermes)")
	initCmd.Flags().StringVar(&initHermesProfile, "profile", "", "Hermes profile name for --hermes (uses <home>/profiles/<profile>)")
}

func runInitHermes(projectRoot string, binaryPath string) {
	result, err := installHermes(projectRoot, binaryPath, initHermesHome, initHermesProfile)
	if err != nil {
		fmt.Printf("  ⚠ Failed to configure Hermes: %v\n", err)
		return
	}

	homeDir, _ := os.UserHomeDir()
	configPath := displayHomePath(result.ConfigPath, homeDir)
	skillsRoot := displayHomePath(result.SkillsRoot, homeDir)

	fmt.Printf("  ✓ Materialized %d Hermes skills (%s)\n", result.SkillCount, skillsRoot)
	fmt.Printf("  ✓ Configured MCP for Hermes (%s)\n", configPath)
	fmt.Println("    Note: reload Hermes skills/MCP or restart the Hermes gateway/session")
}

func installHermes(
	projectRoot string,
	binaryPath string,
	homeInput string,
	profileInput string,
) (hermesInstallResult, error) {
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return hermesInstallResult{}, err
	}

	hermesHome, err := resolveHermesHome(homeInput, profileInput)
	if err != nil {
		return hermesInstallResult{}, err
	}

	skillsRoot, err := defaultHermesSkillsRoot(absProjectRoot)
	if err != nil {
		return hermesInstallResult{}, err
	}

	count, err := materializeHermesSkills(skillsRoot)
	if err != nil {
		return hermesInstallResult{}, err
	}

	configPath := filepath.Join(hermesHome, "config.yaml")
	err = configureMCPHermes(absProjectRoot, binaryPath, configPath, skillsRoot)
	if err != nil {
		return hermesInstallResult{}, err
	}

	return hermesInstallResult{
		ConfigPath: configPath,
		SkillsRoot: skillsRoot,
		SkillCount: count,
	}, nil
}

func resolveHermesHome(homeInput string, profileInput string) (string, error) {
	profile, err := cleanHermesProfile(profileInput)
	if err != nil {
		return "", err
	}

	explicitHome := strings.TrimSpace(homeInput)
	rawHome := explicitHome
	if rawHome == "" {
		rawHome = strings.TrimSpace(os.Getenv("HERMES_HOME"))
	}
	if rawHome == "" {
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", homeErr
		}
		rawHome = filepath.Join(homeDir, ".hermes")
	}

	home, err := expandHermesPath(rawHome)
	if err != nil {
		return "", err
	}
	if profile == "" || explicitHome != "" {
		return home, nil
	}

	return filepath.Join(home, "profiles", profile), nil
}

func cleanHermesProfile(profileInput string) (string, error) {
	profile := strings.TrimSpace(profileInput)
	if profile == "" {
		return "", nil
	}
	if filepath.Base(profile) != profile {
		return "", fmt.Errorf("Hermes profile must be a name, got %q", profileInput)
	}
	if profile == "." || profile == ".." {
		return "", fmt.Errorf("Hermes profile must be a name, got %q", profileInput)
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

func defaultHermesSkillsRoot(projectRoot string) (string, error) {
	if isHaftSourceRoot(projectRoot) {
		relPath := filepath.FromSlash(hermesSkillsRelDir)
		return filepath.Join(projectRoot, relPath), nil
	}

	skillsRoot := filepath.Join(haftconfig.HaftDir(), "hermes", "skills")
	return filepath.Abs(skillsRoot)
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

func materializeHermesSkills(skillsRoot string) (int, error) {
	categoryRoot := filepath.Join(skillsRoot, "haft")
	if err := os.RemoveAll(categoryRoot); err != nil {
		return 0, err
	}

	installed := 0
	for _, sk := range allSkills {
		skillDir := filepath.Join(categoryRoot, sk.Name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return 0, err
		}

		content := transformHermesSkillReferences(string(sk.Content))
		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
			return 0, err
		}
		installed++
	}

	return installed, nil
}

func transformHermesSkillReferences(content string) string {
	replacer := strings.NewReplacer(
		"mcp__haft__haft_problem", "haft_problem",
		"mcp__haft__haft_solution", "haft_solution",
		"mcp__haft__haft_decision", "haft_decision",
		"mcp__haft__haft_query", "haft_query",
		"mcp__haft__haft_note", "haft_note",
		"mcp__haft__haft_refresh", "haft_refresh",
		"mcp__haft__haft_commission", "haft_commission",
		"mcp__haft__haft_spec_section", "haft_spec_section",
	)
	return replacer.Replace(content)
}

func configureMCPHermes(
	projectRoot string,
	binaryPath string,
	configPath string,
	skillsRoot string,
) error {
	settings, err := readHermesConfig(configPath)
	if err != nil {
		return err
	}

	withHermesMCP(settings, projectRoot, binaryPath)
	withHermesExternalDir(settings, skillsRoot)

	return writeHermesConfig(configPath, settings)
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

func withHermesMCP(settings map[string]any, projectRoot string, binaryPath string) {
	mcpServers := hermesMapField(settings, "mcp_servers")
	delete(mcpServers, "quint-code")

	mcpServers["haft"] = map[string]any{
		"command": resolveHermesCommand(binaryPath),
		"args":    []string{"serve"},
		"env":     projectEnvForRoot(projectRoot, projectRoot),
		"enabled": true,
	}

	settings["mcp_servers"] = mcpServers
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

func writeHermesConfig(configPath string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o644)
}
