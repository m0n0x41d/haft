package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AgentPreset struct {
	Name      string `json:"name"`
	AgentKind string `json:"agent_kind"`
	Model     string `json:"model"`
	Role      string `json:"role"`
}

const currentDesktopConfigVersion = 1

type DesktopConfig struct {
	ConfigVersion      int           `json:"config_version"`
	DefaultAgent       string        `json:"default_agent"`
	ReviewAgent        string        `json:"review_agent"`
	VerifyAgent        string        `json:"verify_agent"`
	AgentPresets       []AgentPreset `json:"agent_presets"`
	TaskTimeoutMinutes int           `json:"task_timeout_minutes"`
	SoundEnabled       bool          `json:"sound_enabled"`
	NotifyEnabled      bool          `json:"notify_enabled"`
	DefaultIDE         string        `json:"default_ide"`
	DefaultWorktree    bool          `json:"default_worktree"`
	AutoWireMCP        bool          `json:"auto_wire_mcp"`
	DefaultAutoRun     bool          `json:"default_auto_run"` // true = agent runs without pausing, false = stop-and-ask
}

func defaultDesktopConfig() DesktopConfig {
	return DesktopConfig{
		ConfigVersion:      currentDesktopConfigVersion,
		DefaultAgent:       string(AgentClaude),
		ReviewAgent:        string(AgentCodex),
		VerifyAgent:        string(AgentClaude),
		AgentPresets:       defaultAgentPresets(),
		TaskTimeoutMinutes: 300,
		SoundEnabled:       true,
		NotifyEnabled:      true,
		DefaultIDE:         "code",
		DefaultWorktree:    true,
		AutoWireMCP:        true,
	}
}

func defaultAgentPresets() []AgentPreset {
	return []AgentPreset{
		{Name: "Implementation", AgentKind: string(AgentClaude), Role: "implementation"},
		{Name: "Review", AgentKind: string(AgentCodex), Role: "review"},
		{Name: "Verify", AgentKind: string(AgentClaude), Role: "verify"},
	}
}

func desktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(home, ".haft")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(dir, "desktop-config.json"), nil
}

func loadDesktopConfig() (*DesktopConfig, error) {
	path, err := desktopConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := defaultDesktopConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, err
	}

	storedVersion, err := detectDesktopConfigVersion(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg = migrateDesktopConfig(cfg, storedVersion)
	cfg = normalizeDesktopConfig(cfg)
	return &cfg, nil
}

func saveDesktopConfig(cfg DesktopConfig) error {
	path, err := desktopConfigPath()
	if err != nil {
		return err
	}

	cfg = normalizeDesktopConfig(cfg)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return atomicWriteFile(path, append(data, '\n'), 0o644)
}

func detectDesktopConfigVersion(data []byte) (int, error) {
	var payload struct {
		ConfigVersion *int `json:"config_version"`
	}

	err := json.Unmarshal(data, &payload)
	if err != nil {
		return 0, err
	}

	if payload.ConfigVersion == nil {
		return 0, nil
	}

	return *payload.ConfigVersion, nil
}

func migrateDesktopConfig(cfg DesktopConfig, storedVersion int) DesktopConfig {
	if storedVersion >= currentDesktopConfigVersion {
		return cfg
	}

	cfg.ConfigVersion = currentDesktopConfigVersion
	return cfg
}

func normalizeDesktopConfig(cfg DesktopConfig) DesktopConfig {
	defaults := defaultDesktopConfig()

	if cfg.ConfigVersion <= 0 {
		cfg.ConfigVersion = defaults.ConfigVersion
	}

	cfg.DefaultAgent = normalizeAgentKind(cfg.DefaultAgent, defaults.DefaultAgent)
	cfg.ReviewAgent = normalizeAgentKind(cfg.ReviewAgent, defaults.ReviewAgent)
	cfg.VerifyAgent = normalizeAgentKind(cfg.VerifyAgent, defaults.VerifyAgent)

	if cfg.TaskTimeoutMinutes <= 0 {
		cfg.TaskTimeoutMinutes = defaults.TaskTimeoutMinutes
	}

	cfg.DefaultIDE = normalizeIDE(cfg.DefaultIDE, defaults.DefaultIDE)
	cfg.AgentPresets = normalizeAgentPresets(cfg.AgentPresets)

	if len(cfg.AgentPresets) == 0 {
		cfg.AgentPresets = defaults.AgentPresets
	}

	return cfg
}

func normalizeAgentPresets(presets []AgentPreset) []AgentPreset {
	defaults := defaultDesktopConfig()
	normalized := make([]AgentPreset, 0, len(presets))
	seen := make(map[string]bool)

	for _, preset := range presets {
		name := strings.TrimSpace(preset.Name)
		role := strings.TrimSpace(preset.Role)

		if name == "" {
			continue
		}

		preset.Name = name
		preset.Role = role
		preset.AgentKind = normalizeAgentKind(preset.AgentKind, defaults.DefaultAgent)
		preset.Model = strings.TrimSpace(preset.Model)

		key := strings.ToLower(name) + "|" + strings.ToLower(role)
		if seen[key] {
			continue
		}

		seen[key] = true
		normalized = append(normalized, preset)
	}

	return normalized
}

func normalizeAgentKind(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(AgentClaude):
		return string(AgentClaude)
	case string(AgentCodex):
		return string(AgentCodex)
	case string(AgentHaft):
		return string(AgentHaft)
	default:
		return fallback
	}
}

func normalizeIDE(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "code":
		return "code"
	case "zed":
		return "zed"
	case "idea":
		return "idea"
	default:
		return fallback
	}
}

func buildIDECommand(ide string, targetPath string) []string {
	switch normalizeIDE(ide, defaultDesktopConfig().DefaultIDE) {
	case "zed":
		return []string{"zed", targetPath}
	case "idea":
		return []string{"idea", targetPath}
	default:
		return []string{"code", targetPath}
	}
}

func (a *App) GetConfig() (*DesktopConfig, error) {
	return loadDesktopConfig()
}

func (a *App) SaveConfig(cfg DesktopConfig) (*DesktopConfig, error) {
	cfg = normalizeDesktopConfig(cfg)

	if err := saveDesktopConfig(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
