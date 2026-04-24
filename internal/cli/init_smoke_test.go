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

// TestConfigureMCPClaude_CreatesPortableProjectRootMCP verifies the Claude Code
// integration writes .mcp.json at the project root with a portable
// HAFT_PROJECT_ROOT value suitable for committed project-scoped config.
func TestConfigureMCPClaude_CreatesPortableProjectRootMCP(t *testing.T) {
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

	var got MCPConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("produced config not valid JSON: %v\n%s", err, string(data))
	}

	server := got.MCPServers["haft"]
	root := server.Env["HAFT_PROJECT_ROOT"]
	if root != claudeProjectRootEnv {
		t.Errorf("HAFT_PROJECT_ROOT = %q, want portable %q", root, claudeProjectRootEnv)
	}
	if strings.Contains(string(data), tmp) {
		t.Errorf("committed .mcp.json contains absolute temp path %q:\n%s", tmp, string(data))
	}

	first := string(data)
	if err := configureMCPClaude(tmp, "haft"); err != nil {
		t.Fatalf("configureMCPClaude second run: %v", err)
	}
	secondData, err := os.ReadFile(filepath.Join(tmp, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json after second run: %v", err)
	}
	if first != string(secondData) {
		t.Errorf("configureMCPClaude is not idempotent.\nfirst:\n%s\nsecond:\n%s", first, string(secondData))
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

	server := got.MCPServers["haft"]
	root := server.Env["HAFT_PROJECT_ROOT"]
	if root != cursorProjectRootEnv {
		t.Errorf("HAFT_PROJECT_ROOT = %q, want portable %q", root, cursorProjectRootEnv)
	}
	if strings.Contains(string(data), tmp) {
		t.Errorf("committed .cursor/mcp.json contains absolute temp path %q:\n%s", tmp, string(data))
	}
}

// TestConfigureMCPCodex_CreatesPortableProjectRootMCP verifies the Codex/Air
// project-local TOML does not commit the current machine checkout path.
func TestConfigureMCPCodex_CreatesPortableProjectRootMCP(t *testing.T) {
	tmp := t.TempDir()

	if err := configureMCPCodex(tmp, "haft"); err != nil {
		t.Fatalf("configureMCPCodex: %v", err)
	}

	configPath := filepath.Join(tmp, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected .codex/config.toml: %v", err)
	}

	text := string(data)
	for _, fragment := range []string{
		`[mcp_servers.haft]`,
		`command = "haft"`,
		`args = ["serve"]`,
		`[mcp_servers.haft.env]`,
		`HAFT_PROJECT_ROOT = "."`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("codex config missing %q:\n%s", fragment, text)
		}
	}
	if strings.Contains(text, tmp) {
		t.Fatalf("committed .codex/config.toml contains absolute temp path %q:\n%s", tmp, text)
	}

	first := text
	if err := configureMCPCodex(tmp, "haft"); err != nil {
		t.Fatalf("configureMCPCodex second run: %v", err)
	}
	secondData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read .codex/config.toml after second run: %v", err)
	}
	if first != string(secondData) {
		t.Fatalf("configureMCPCodex is not idempotent.\nfirst:\n%s\nsecond:\n%s", first, string(secondData))
	}
}
