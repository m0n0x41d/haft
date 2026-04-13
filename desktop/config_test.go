package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDesktopConfigAppliesDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := desktopConfigPath()
	if err != nil {
		t.Fatalf("desktopConfigPath: %v", err)
	}

	data := []byte(`{"default_agent":"codex","notify_enabled":false}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := loadDesktopConfig()
	if err != nil {
		t.Fatalf("loadDesktopConfig: %v", err)
	}

	if cfg.DefaultAgent != string(AgentCodex) {
		t.Fatalf("expected default agent codex, got %q", cfg.DefaultAgent)
	}

	if cfg.NotifyEnabled {
		t.Fatalf("expected notify_enabled=false")
	}

	if cfg.ConfigVersion != currentDesktopConfigVersion {
		t.Fatalf("expected config version %d, got %d", currentDesktopConfigVersion, cfg.ConfigVersion)
	}

	if cfg.TaskTimeoutMinutes != 300 {
		t.Fatalf("expected default timeout 300, got %d", cfg.TaskTimeoutMinutes)
	}

	if len(cfg.AgentPresets) == 0 {
		t.Fatalf("expected default presets to be populated")
	}
}

func TestSaveDesktopConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := defaultDesktopConfig()
	cfg.DefaultAgent = string(AgentHaft)
	cfg.TaskTimeoutMinutes = 42
	cfg.DefaultIDE = "zed"
	cfg.AgentPresets = []AgentPreset{
		{Name: "Implement", AgentKind: string(AgentClaude), Role: "implementation"},
		{Name: "Review", AgentKind: string(AgentCodex), Role: "review", Model: "gpt-5"},
	}

	if err := saveDesktopConfig(cfg); err != nil {
		t.Fatalf("saveDesktopConfig: %v", err)
	}

	loaded, err := loadDesktopConfig()
	if err != nil {
		t.Fatalf("loadDesktopConfig: %v", err)
	}

	if loaded.DefaultAgent != string(AgentHaft) {
		t.Fatalf("expected default agent haft, got %q", loaded.DefaultAgent)
	}

	if loaded.TaskTimeoutMinutes != 42 {
		t.Fatalf("expected timeout 42, got %d", loaded.TaskTimeoutMinutes)
	}

	if loaded.DefaultIDE != "zed" {
		t.Fatalf("expected IDE zed, got %q", loaded.DefaultIDE)
	}

	if len(loaded.AgentPresets) != 2 {
		t.Fatalf("expected 2 presets, got %d", len(loaded.AgentPresets))
	}

	configPath := filepath.Join(home, ".haft", "desktop-config.json")

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var payload map[string]any

	err = json.Unmarshal(data, &payload)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	versionValue, ok := payload["config_version"].(float64)
	if !ok {
		t.Fatalf("expected config_version to be written")
	}

	if int(versionValue) != currentDesktopConfigVersion {
		t.Fatalf("expected written config version %d, got %d", currentDesktopConfigVersion, int(versionValue))
	}
}

func TestLoadDesktopConfigMigratesLegacyConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := desktopConfigPath()
	if err != nil {
		t.Fatalf("desktopConfigPath: %v", err)
	}

	data := []byte(`{
  "default_agent": "codex",
  "notify_enabled": false,
  "future_field": "ignored"
}`)
	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := loadDesktopConfig()
	if err != nil {
		t.Fatalf("loadDesktopConfig: %v", err)
	}

	if cfg.ConfigVersion != currentDesktopConfigVersion {
		t.Fatalf("expected migrated config version %d, got %d", currentDesktopConfigVersion, cfg.ConfigVersion)
	}

	if cfg.DefaultAgent != string(AgentCodex) {
		t.Fatalf("expected default agent codex, got %q", cfg.DefaultAgent)
	}

	if cfg.TaskTimeoutMinutes != defaultDesktopConfig().TaskTimeoutMinutes {
		t.Fatalf(
			"expected timeout %d, got %d",
			defaultDesktopConfig().TaskTimeoutMinutes,
			cfg.TaskTimeoutMinutes,
		)
	}

	if cfg.AutoWireMCP != defaultDesktopConfig().AutoWireMCP {
		t.Fatalf("expected auto_wire_mcp default %t, got %t", defaultDesktopConfig().AutoWireMCP, cfg.AutoWireMCP)
	}
}

func TestBuildIDECommandUsesNormalizedIDE(t *testing.T) {
	testCases := []struct {
		name     string
		ide      string
		expected []string
	}{
		{name: "default to vscode", ide: "", expected: []string{"code", "/tmp/project"}},
		{name: "normalize zed", ide: "ZeD", expected: []string{"zed", "/tmp/project"}},
		{name: "normalize idea", ide: "idea", expected: []string{"idea", "/tmp/project"}},
		{name: "fallback on unknown", ide: "custom", expected: []string{"code", "/tmp/project"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command := buildIDECommand(tc.ide, "/tmp/project")

			if len(command) != len(tc.expected) {
				t.Fatalf("expected %d command parts, got %d", len(tc.expected), len(command))
			}

			for index := range tc.expected {
				if command[index] != tc.expected[index] {
					t.Fatalf(
						"expected command[%d]=%q, got %q",
						index,
						tc.expected[index],
						command[index],
					)
				}
			}
		})
	}
}
