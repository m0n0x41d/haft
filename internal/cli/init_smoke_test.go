package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeMCPConfig_FreshProject is the minimum smoke check that haft init
// still produces a valid MCP config after the schema/dispatcher refactors of
// 6.2 (#62 parity_plan exposure, #63 ID format, governance_mode field, the
// dispatchTool signature change). If init regresses, MCP-mode users (Claude
// Code, Cursor, Gemini CLI, Codex) silently lose access to haft on next
// install.
func TestMergeMCPConfig_FreshProject(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, ".mcp.json")

	if err := mergeMCPConfig(configPath, "haft", tmp, nil); err != nil {
		t.Fatalf("mergeMCPConfig: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read produced config: %v", err)
	}

	var got MCPConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("produced config is not valid JSON: %v\n%s", err, string(data))
	}

	server, ok := got.MCPServers["haft"]
	if !ok {
		t.Fatalf("expected 'haft' entry in mcpServers, got: %+v", got.MCPServers)
	}
	if server.Command != "haft" {
		t.Errorf("server.Command = %q, want %q (bare name — see commit 88510ba9)", server.Command, "haft")
	}
	if len(server.Args) != 1 || server.Args[0] != "serve" {
		t.Errorf("server.Args = %v, want [serve]", server.Args)
	}
}

// TestMergeMCPConfig_MigratesLegacyKey verifies the quint-code → haft rename
// migration on an existing config. Users with stale .mcp.json from pre-rename
// installs should not end up with two registered MCP servers.
func TestMergeMCPConfig_MigratesLegacyKey(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, ".mcp.json")

	legacy := `{"mcpServers":{"quint-code":{"command":"quint-code","args":["serve"]}}}`
	if err := os.WriteFile(configPath, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	if err := mergeMCPConfig(configPath, "haft", tmp, nil); err != nil {
		t.Fatalf("mergeMCPConfig on legacy file: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var got MCPConfig
	_ = json.Unmarshal(data, &got)

	if _, stillThere := got.MCPServers["quint-code"]; stillThere {
		t.Errorf("legacy quint-code key not migrated; should have been deleted: %+v", got.MCPServers)
	}
	if _, ok := got.MCPServers["haft"]; !ok {
		t.Errorf("haft entry missing after legacy migration: %+v", got.MCPServers)
	}
}

// TestMergeMCPConfig_Idempotent verifies that running haft init twice
// produces the same config — running init is safe to repeat.
func TestMergeMCPConfig_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, ".mcp.json")

	if err := mergeMCPConfig(configPath, "haft", tmp, nil); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, _ := os.ReadFile(configPath)

	if err := mergeMCPConfig(configPath, "haft", tmp, nil); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, _ := os.ReadFile(configPath)

	if string(first) != string(second) {
		t.Errorf("haft init is not idempotent.\nfirst:\n%s\nsecond:\n%s", string(first), string(second))
	}
}

// TestConfigureMCPClaude_CreatesProjectRootMCP verifies the Claude Code
// integration writes .mcp.json at the project root with the HAFT_PROJECT_ROOT
// env var so the spawned haft serve picks up the correct project context.
func TestConfigureMCPClaude_CreatesProjectRootMCP(t *testing.T) {
	tmp := t.TempDir()

	if err := configureMCPClaude(tmp, "haft"); err != nil {
		t.Fatalf("configureMCPClaude: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".mcp.json"))
	if err != nil {
		t.Fatalf("expected .mcp.json at project root: %v", err)
	}

	if !strings.Contains(string(data), `"haft"`) {
		t.Errorf("expected haft MCP entry, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), `"HAFT_PROJECT_ROOT"`) {
		t.Errorf("expected HAFT_PROJECT_ROOT env var to be set:\n%s", string(data))
	}
	if !strings.Contains(string(data), tmp) {
		t.Errorf("expected project root %q in env, got:\n%s", tmp, string(data))
	}
}

// TestConfigureMCPCursor_CreatesProjectScopedMCP verifies the Cursor
// integration writes .cursor/mcp.json (not the user-level location).
func TestConfigureMCPCursor_CreatesProjectScopedMCP(t *testing.T) {
	tmp := t.TempDir()

	if err := configureMCPCursor(tmp, "haft"); err != nil {
		t.Fatalf("configureMCPCursor: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("expected .cursor/mcp.json: %v", err)
	}

	var got MCPConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("produced config not valid JSON: %v\n%s", err, string(data))
	}
	if _, ok := got.MCPServers["haft"]; !ok {
		t.Fatalf("expected haft entry in cursor mcp.json: %+v", got.MCPServers)
	}
}
