package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypedPublicZedAcceptsJSONCAndPreservesForeignSemantics(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	settingsPath := filepath.Join(
		homeRoot,
		".config",
		"zed",
		"settings.json",
	)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create Zed settings root: %v", err)
	}
	fixture := []byte(`// Zed settings
{
  "theme": "dark",
  "agent_servers": {
    "codex-acp": {
      "type": "registry",
    },
  },
  "context_servers": {
    "Pieces": {
      "command": "/opt/pieces",
      "args": ["mcp", "start"],
    },
    "haft": {
      "command": "operator-wrapper",
      "args": ["serve"],
    },
    "quint-code": {
      "command": "operator-quint",
      "args": ["custom"],
    },
  },
}
`)
	if err := os.WriteFile(settingsPath, fixture, 0o600); err != nil {
		t.Fatalf("write Zed JSONC settings: %v", err)
	}
	if err := runTypedPublicZedForTest(t); err != nil {
		t.Fatalf("first typed Zed init: %v", err)
	}
	first, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read typed Zed settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(first, &settings); err != nil {
		t.Fatalf("typed Zed output is not strict JSON: %v\n%s", err, first)
	}
	if settings["theme"] != "dark" {
		t.Fatalf("Zed theme changed: %#v", settings["theme"])
	}
	agents, ok := settings["agent_servers"].(map[string]any)
	if !ok || agents["codex-acp"] == nil {
		t.Fatalf("Zed agent_servers changed: %#v", settings["agent_servers"])
	}
	servers, ok := settings["context_servers"].(map[string]any)
	if !ok ||
		servers["Pieces"] == nil ||
		servers["haft"] == nil ||
		servers["quint-code"] == nil ||
		servers[zedContextServerName] == nil {
		t.Fatalf("Zed context_servers changed: %#v", settings["context_servers"])
	}
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat Zed settings: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("Zed settings mode = %o, want 600", info.Mode().Perm())
	}
	if err := runTypedPublicZedForTest(t); err != nil {
		t.Fatalf("second typed Zed init: %v", err)
	}
	second, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read second Zed settings: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf(
			"typed Zed init is not idempotent\nfirst: %s\nsecond: %s",
			first,
			second,
		)
	}
}

func TestTypedPublicZedRejectsMalformedJSONCBeforeWrites(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	settingsPath := filepath.Join(
		homeRoot,
		".config",
		"zed",
		"settings.json",
	)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create Zed settings root: %v", err)
	}
	fixture := []byte(
		"{ /* unfinished\n  \"context_servers\": {}\n}\n",
	)
	if err := os.WriteFile(settingsPath, fixture, 0o600); err != nil {
		t.Fatalf("write malformed Zed settings: %v", err)
	}
	err := runTypedPublicZedForTest(t)
	if err == nil || !strings.Contains(
		err.Error(),
		"unterminated block comment",
	) {
		t.Fatalf("malformed Zed settings error = %v", err)
	}
	after, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatalf("read malformed Zed settings: %v", readErr)
	}
	if !bytes.Equal(after, fixture) {
		t.Fatalf("malformed Zed settings changed: %s", after)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft", "config.yaml"),
	); !os.IsNotExist(err) {
		t.Fatalf("malformed Zed init wrote project core: %v", err)
	}
}

func runTypedPublicZedForTest(t *testing.T) error {
	t.Helper()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initZed = true
	cmd := newPublicInitTestCommand()
	var selected bool
	cmd.Flags().BoolVar(&selected, "zed", false, "")
	if err := cmd.Flags().Set("zed", "true"); err != nil {
		t.Fatalf("set Zed flag: %v", err)
	}
	return runPublicInit(cmd, nil)
}
