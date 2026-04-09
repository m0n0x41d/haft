package main

import (
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

	if _, err := os.Stat(filepath.Join(home, ".haft", "desktop-config.json")); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}
