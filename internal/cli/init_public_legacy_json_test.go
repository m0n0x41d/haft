package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestTypedPublicInitMigratesExactHistoricalJSONMCPAliases(
	t *testing.T,
) {
	testCases := []struct {
		name  string
		host  initplanning.HostID
		flag  string
		shape publicLegacyJSONMCPShape
		path  func(string, string) string
	}{
		{
			name:  "claude",
			host:  initplanning.HostClaude,
			flag:  "claude",
			shape: publicLegacyJSONMCPClaude,
			path: func(projectRoot string, _ string) string {
				return filepath.Join(projectRoot, ".mcp.json")
			},
		},
		{
			name:  "cursor",
			host:  initplanning.HostCursor,
			flag:  "cursor",
			shape: publicLegacyJSONMCPCursor,
			path: func(projectRoot string, _ string) string {
				return filepath.Join(
					projectRoot,
					".cursor",
					"mcp.json",
				)
			},
		},
		{
			name:  "gemini",
			host:  initplanning.HostGemini,
			flag:  "gemini",
			shape: publicLegacyJSONMCPGemini,
			path: func(_ string, homeRoot string) string {
				return filepath.Join(
					homeRoot,
					".gemini",
					"settings.json",
				)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := physicalInitTestTempDir(t)
			homeRoot := physicalInitTestTempDir(t)
			restoreDirectory := changeInitTestDirectory(
				t,
				projectRoot,
			)
			defer restoreDirectory()
			t.Setenv("HOME", homeRoot)

			configPath := testCase.path(projectRoot, homeRoot)
			if err := os.MkdirAll(
				filepath.Dir(configPath),
				0o755,
			); err != nil {
				t.Fatalf("create host config root: %v", err)
			}
			legacy := publicLegacyJSONMCPValue(
				testCase.shape,
				"/usr/local/bin/quint-code",
				projectRoot,
			)
			fixture := map[string]any{
				"theme": "dark",
				"mcpServers": map[string]any{
					"operator": map[string]any{
						"command": "operator-server",
						"args":    []string{"start"},
					},
					"quint-code": legacy,
				},
			}
			content, err := json.MarshalIndent(fixture, "", "  ")
			if err != nil {
				t.Fatalf("encode host fixture: %v", err)
			}
			content = append(content, '\n')
			if err := os.WriteFile(
				configPath,
				content,
				0o600,
			); err != nil {
				t.Fatalf("write host fixture: %v", err)
			}
			if err := runTypedPublicHostForTest(
				t,
				testCase.host,
				testCase.flag,
			); err != nil {
				t.Fatalf("migrate historical alias: %v", err)
			}
			first, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read migrated host config: %v", err)
			}
			var migrated map[string]any
			if err := json.Unmarshal(first, &migrated); err != nil {
				t.Fatalf("decode migrated host config: %v", err)
			}
			servers, ok := migrated["mcpServers"].(map[string]any)
			if !ok ||
				servers["operator"] == nil ||
				servers["haft"] == nil {
				t.Fatalf("migrated MCP servers = %#v", migrated["mcpServers"])
			}
			if _, present := servers["quint-code"]; present {
				t.Fatalf(
					"historical alias survived migration: %#v",
					servers,
				)
			}
			if migrated["theme"] != "dark" {
				t.Fatalf("foreign host setting changed: %#v", migrated)
			}
			info, err := os.Stat(configPath)
			if err != nil {
				t.Fatalf("stat migrated host config: %v", err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf(
					"migrated host mode = %o, want 600",
					info.Mode().Perm(),
				)
			}
			if err := runTypedPublicHostForTest(
				t,
				testCase.host,
				testCase.flag,
			); err != nil {
				t.Fatalf("rerun migrated host: %v", err)
			}
			second, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read rerun host config: %v", err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf(
					"host alias migration is not idempotent\nfirst: %s\nsecond: %s",
					first,
					second,
				)
			}
		})
	}
}

func TestTypedPublicInitRejectsModifiedHistoricalJSONMCPAliases(
	t *testing.T,
) {
	testCases := []struct {
		name  string
		host  initplanning.HostID
		flag  string
		shape publicLegacyJSONMCPShape
		path  func(string, string) string
	}{
		{
			name:  "claude",
			host:  initplanning.HostClaude,
			flag:  "claude",
			shape: publicLegacyJSONMCPClaude,
			path: func(projectRoot string, _ string) string {
				return filepath.Join(projectRoot, ".mcp.json")
			},
		},
		{
			name:  "cursor",
			host:  initplanning.HostCursor,
			flag:  "cursor",
			shape: publicLegacyJSONMCPCursor,
			path: func(projectRoot string, _ string) string {
				return filepath.Join(
					projectRoot,
					".cursor",
					"mcp.json",
				)
			},
		},
		{
			name:  "gemini",
			host:  initplanning.HostGemini,
			flag:  "gemini",
			shape: publicLegacyJSONMCPGemini,
			path: func(_ string, homeRoot string) string {
				return filepath.Join(
					homeRoot,
					".gemini",
					"settings.json",
				)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := physicalInitTestTempDir(t)
			homeRoot := physicalInitTestTempDir(t)
			restoreDirectory := changeInitTestDirectory(
				t,
				projectRoot,
			)
			defer restoreDirectory()
			t.Setenv("HOME", homeRoot)

			configPath := testCase.path(projectRoot, homeRoot)
			if err := os.MkdirAll(
				filepath.Dir(configPath),
				0o755,
			); err != nil {
				t.Fatalf("create host config root: %v", err)
			}
			foreign := publicLegacyJSONMCPValue(
				testCase.shape,
				"operator-wrapper",
				projectRoot,
			)
			fixture, err := json.Marshal(
				map[string]any{
					"mcpServers": map[string]any{
						"quint-code": foreign,
					},
				},
			)
			if err != nil {
				t.Fatalf("encode foreign host fixture: %v", err)
			}
			if err := os.WriteFile(
				configPath,
				fixture,
				0o600,
			); err != nil {
				t.Fatalf("write foreign host fixture: %v", err)
			}
			err = runTypedPublicHostForTest(
				t,
				testCase.host,
				testCase.flag,
			)
			if err == nil {
				t.Fatal("modified historical alias was claimed")
			}
			after, readErr := os.ReadFile(configPath)
			if readErr != nil {
				t.Fatalf("read foreign host fixture: %v", readErr)
			}
			if !bytes.Equal(after, fixture) {
				t.Fatalf("foreign host alias changed: %s", after)
			}
			if _, err := os.Stat(
				filepath.Join(
					projectRoot,
					".haft",
					"config.yaml",
				),
			); !os.IsNotExist(err) {
				t.Fatalf("foreign host alias wrote project core: %v", err)
			}
		})
	}
}

func TestTypedPublicAirMigratesSharedHistoricalCodexAlias(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	configPath := filepath.Join(
		projectRoot,
		".codex",
		"config.toml",
	)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create Air Codex-compatible root: %v", err)
	}
	fixture := append(
		[]byte("model = \"gpt-5\"\n\n"),
		publicLegacyCodexQuintContent(
			"quint-code",
			projectRoot,
		)...,
	)
	fixture = append(
		fixture,
		[]byte(
			"\n[mcp_servers.private]\n"+
				"command = \"operator-server\"\n",
		)...,
	)
	if err := os.WriteFile(configPath, fixture, 0o600); err != nil {
		t.Fatalf("write historical Air config: %v", err)
	}
	if err := runTypedPublicHostForTest(
		t,
		initplanning.HostAir,
		"air",
	); err != nil {
		t.Fatalf("migrate historical Air config: %v", err)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated Air config: %v", err)
	}
	if bytes.Contains(
		content,
		[]byte("[mcp_servers.quint-code]"),
	) ||
		!bytes.Contains(
			content,
			[]byte("[mcp_servers.haft]"),
		) ||
		!bytes.Contains(
			content,
			[]byte("[mcp_servers.private]"),
		) {
		t.Fatalf("migrated Air config:\n%s", content)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat migrated Air config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("migrated Air mode = %o, want 600", info.Mode().Perm())
	}
}

func TestPublicLegacyJSONMCPValidatorAcceptsOnlyBoundEmittedShapes(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	commandArgs := func() map[string]any {
		return map[string]any{
			"command": "/usr/local/bin/quint-code",
			"args":    []string{"serve"},
		}
	}
	withEnv := func(value map[string]any) map[string]any {
		value["env"] = map[string]any{
			legacyQuintProjectRootEnv: projectRoot,
		}
		return value
	}
	withCWD := func(value map[string]any) map[string]any {
		value["cwd"] = projectRoot
		return value
	}
	withTimeout := func(value map[string]any) map[string]any {
		value["timeout"] = 30000
		return value
	}
	testCases := []struct {
		name  string
		shape publicLegacyJSONMCPShape
		value map[string]any
		want  bool
	}{
		{
			name:  "claude env",
			shape: publicLegacyJSONMCPClaude,
			value: withEnv(commandArgs()),
			want:  true,
		},
		{
			name:  "claude cwd",
			shape: publicLegacyJSONMCPClaude,
			value: withCWD(commandArgs()),
			want:  true,
		},
		{
			name:  "claude cwd and env",
			shape: publicLegacyJSONMCPClaude,
			value: withEnv(withCWD(commandArgs())),
			want:  true,
		},
		{
			name:  "claude unbound",
			shape: publicLegacyJSONMCPClaude,
			value: commandArgs(),
			want:  false,
		},
		{
			name:  "cursor unbound project carrier",
			shape: publicLegacyJSONMCPCursor,
			value: commandArgs(),
			want:  true,
		},
		{
			name:  "cursor cwd and env",
			shape: publicLegacyJSONMCPCursor,
			value: withEnv(withCWD(commandArgs())),
			want:  true,
		},
		{
			name:  "gemini cwd timeout",
			shape: publicLegacyJSONMCPGemini,
			value: withTimeout(withCWD(commandArgs())),
			want:  true,
		},
		{
			name:  "gemini fully bound",
			shape: publicLegacyJSONMCPGemini,
			value: withTimeout(withEnv(withCWD(commandArgs()))),
			want:  true,
		},
		{
			name:  "gemini timeout only",
			shape: publicLegacyJSONMCPGemini,
			value: withTimeout(commandArgs()),
			want:  false,
		},
		{
			name:  "wrong root",
			shape: publicLegacyJSONMCPClaude,
			value: func() map[string]any {
				value := commandArgs()
				value["env"] = map[string]any{
					legacyQuintProjectRootEnv: t.TempDir(),
				}
				return value
			}(),
			want: false,
		},
		{
			name:  "extra field",
			shape: publicLegacyJSONMCPClaude,
			value: func() map[string]any {
				value := withEnv(commandArgs())
				value["enabled"] = true
				return value
			}(),
			want: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			content, err := json.Marshal(testCase.value)
			if err != nil {
				t.Fatalf("encode validator fixture: %v", err)
			}
			got := isPublicLegacyJSONMCPValue(
				content,
				testCase.shape,
				projectRoot,
			)
			if got != testCase.want {
				t.Fatalf(
					"validator = %t, want %t for %s",
					got,
					testCase.want,
					content,
				)
			}
		})
	}
}

func runTypedPublicHostForTest(
	t *testing.T,
	host initplanning.HostID,
	flag string,
) error {
	t.Helper()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	if err := selectInitHostFlag(host); err != nil {
		t.Fatalf("select %s host: %v", host, err)
	}
	cmd := newPublicInitTestCommand()
	var selected bool
	cmd.Flags().BoolVar(&selected, flag, false, "")
	if err := cmd.Flags().Set(flag, "true"); err != nil {
		t.Fatalf("set %s flag: %v", flag, err)
	}
	return runPublicInit(cmd, nil)
}
