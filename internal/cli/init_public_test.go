package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/spf13/cobra"
)

func TestPublicInitBareNonTerminalFailsBeforeWrites(t *testing.T) {
	projectRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()

	cmd := newPublicInitTestCommand()
	selectionCalled := false
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return false
		},
		func(
			initplanning.InteractiveSession,
			io.Reader,
			io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			selectionCalled = true
			return initplanning.InteractiveCancelledOutcome{}, nil
		},
		func(*cobra.Command, []string, initplanning.InvocationPolicy) error {
			applyCalled = true
			return nil
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "requires an interactive terminal") {
		t.Fatalf("runPublicInitWith error = %v", err)
	}
	if selectionCalled || applyCalled {
		t.Fatalf(
			"non-terminal path called selection=%t apply=%t",
			selectionCalled,
			applyCalled,
		)
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, ".haft")); !os.IsNotExist(statErr) {
		t.Fatalf("bare non-terminal init wrote .haft: %v", statErr)
	}
}

func TestPublicInitModifierOnlyNonTerminalFailsBeforeWrites(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initLocal = true
	initNoFileInstructions = true

	cmd := newPublicInitTestCommand()
	var local bool
	var omitInstructions bool
	cmd.Flags().BoolVar(&local, "local", false, "")
	cmd.Flags().BoolVar(
		&omitInstructions,
		"no-file-instructions",
		false,
		"",
	)
	if err := cmd.Flags().Set("local", "true"); err != nil {
		t.Fatalf("set local flag: %v", err)
	}
	if err := cmd.Flags().Set(
		"no-file-instructions",
		"true",
	); err != nil {
		t.Fatalf("set no-file-instructions flag: %v", err)
	}
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return false
		},
		func(
			initplanning.InteractiveSession,
			io.Reader,
			io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			t.Fatal("modifier-only invocation opened the TTY menu")
			return nil, nil
		},
		func(
			*cobra.Command,
			[]string,
			initplanning.InvocationPolicy,
		) error {
			applyCalled = true
			return nil
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "require an explicit target") {
		t.Fatalf("modifier-only error = %v", err)
	}
	if applyCalled {
		t.Fatal("modifier-only invocation reached the effect runner")
	}
	if _, statErr := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(statErr) {
		t.Fatalf("modifier-only invocation wrote .haft: %v", statErr)
	}
}

func TestPublicInitProductionExplicitCodexUsesTypedOperation(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	homeRoot, err := filepath.EvalSymlinks(homeRoot)
	if err != nil {
		t.Fatalf("resolve physical home root: %v", err)
	}
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetOut(output)
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("runPublicInit: %v", err)
	}
	if !strings.Contains(output.String(), "Haft initialization complete") ||
		!strings.Contains(output.String(), "Project memory ready") ||
		!strings.Contains(
			output.String(),
			"Project core initialized (schema ",
		) ||
		!strings.Contains(
			output.String(),
			"Codex: ",
		) ||
		!strings.Contains(output.String(), "MCP") ||
		!strings.Contains(output.String(), "skills") ||
		!strings.Contains(output.String(), "instructions") ||
		!strings.Contains(output.String(), "Reload: required") ||
		!strings.Contains(output.String(), "Project ID: ") ||
		strings.Contains(output.String(), "Haft initialization plan") ||
		strings.Contains(output.String(), "Apply this exact plan?") ||
		strings.Contains(output.String(), "Initializing Haft project...") {
		t.Fatalf("typed public output = %q", output.String())
	}
	binding := mustOnboardProjectBinding(t, projectRoot)
	memoryReady, err := projectMemoryReadyReadOnly(
		context.Background(),
		binding,
	)
	if err != nil {
		t.Fatalf("inspect initialized project memory: %v", err)
	}
	if !memoryReady {
		t.Fatal("haft init completed without ready project memory")
	}
	if _, statErr := os.Stat(
		projectTypeEnvGenesisReviewPath(projectRoot),
	); !os.IsNotExist(statErr) {
		t.Fatalf("haft init retained an internal memory review carrier: %v", statErr)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".codex", "config.toml"),
	); err != nil {
		t.Fatalf("typed Codex config missing: %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(
			projectRoot,
			".haft",
			"host-installations",
			"codex.project.json",
		),
	); err != nil {
		t.Fatalf("typed Codex manifest missing: %v", err)
	}
	assertPublicHostManifestAdapterEdition(
		t,
		filepath.Join(
			projectRoot,
			".haft",
			"host-installations",
			"codex.project.json",
		),
		"codex.coherent.v2",
	)
	assertPublicHostManifestAdapterEdition(
		t,
		filepath.Join(
			homeRoot,
			".haft",
			"host-installations",
			"codex.user.json",
		),
		"codex.skills.v1",
	)
	for _, path := range []string{
		filepath.Join(projectRoot, "AGENTS.md"),
		filepath.Join(
			homeRoot,
			".agents",
			"skills",
			"h-reason",
			"SKILL.md",
		),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("typed Codex integration missing %s: %v", path, err)
		}
	}

	currentOutput := &bytes.Buffer{}
	current := newPublicInitTestCommand()
	current.SetOut(currentOutput)
	var currentCodex bool
	current.Flags().BoolVar(&currentCodex, "codex", false, "")
	if err := current.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set current Codex flag: %v", err)
	}
	if err := runPublicInit(current, nil); err != nil {
		t.Fatalf("rerun current public init: %v", err)
	}
	for _, expected := range []string{
		"Haft is already initialized",
		"Project core already current (schema ",
		"Project memory ready",
		"Codex: MCP and instructions already current",
		"Codex (user): skills already current",
		"Project ID: ",
	} {
		if !strings.Contains(currentOutput.String(), expected) {
			t.Fatalf(
				"current typed public output missing %q: %q",
				expected,
				currentOutput.String(),
			)
		}
	}
	if strings.Contains(currentOutput.String(), "Reload: required") {
		t.Fatalf(
			"current init incorrectly requested a reload: %q",
			currentOutput.String(),
		)
	}
}

func TestPublicInitProductionStableHostSurfaceMatrix(
	t *testing.T,
) {
	testCases := []struct {
		name            string
		host            initplanning.HostID
		flag            string
		mcpOnly         bool
		adapterEdition  string
		configPath      func(string) string
		instructionPath func(string) string
		skillPath       func(string) string
	}{
		{
			name:           "claude full",
			host:           initplanning.HostClaude,
			flag:           "claude",
			adapterEdition: "claude.coherent.v2",
			configPath: func(root string) string {
				return filepath.Join(root, ".mcp.json")
			},
			instructionPath: func(root string) string {
				return filepath.Join(root, "CLAUDE.md")
			},
			skillPath: func(home string) string {
				return filepath.Join(
					home,
					".claude",
					"skills",
					"h-reason",
					"SKILL.md",
				)
			},
		},
		{
			name:           "claude mcp only",
			host:           initplanning.HostClaude,
			flag:           "claude",
			mcpOnly:        true,
			adapterEdition: "claude.coherent.v2",
			configPath: func(root string) string {
				return filepath.Join(root, ".mcp.json")
			},
			instructionPath: func(root string) string {
				return filepath.Join(root, "CLAUDE.md")
			},
			skillPath: func(home string) string {
				return filepath.Join(home, ".claude", "skills")
			},
		},
		{
			name:           "codex mcp only",
			host:           initplanning.HostCodex,
			flag:           "codex",
			mcpOnly:        true,
			adapterEdition: "codex.coherent.v2",
			configPath: func(root string) string {
				return filepath.Join(
					root,
					".codex",
					"config.toml",
				)
			},
			instructionPath: func(root string) string {
				return filepath.Join(root, "AGENTS.md")
			},
			skillPath: func(home string) string {
				return filepath.Join(home, ".agents", "skills")
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			homeRoot := t.TempDir()
			restoreDirectory := changeInitTestDirectory(
				t,
				projectRoot,
			)
			defer restoreDirectory()
			restoreFlags := captureInitHostFlagState()
			defer restoreFlags.apply()
			clearInitHostFlags()
			if err := selectInitHostFlag(testCase.host); err != nil {
				t.Fatalf("select host: %v", err)
			}
			initMCPOnly = testCase.mcpOnly
			t.Setenv("HOME", homeRoot)

			output := &bytes.Buffer{}
			cmd := newPublicInitTestCommand()
			cmd.SetOut(output)
			var selected bool
			var mcpOnly bool
			cmd.Flags().BoolVar(
				&selected,
				testCase.flag,
				false,
				"",
			)
			cmd.Flags().BoolVar(
				&mcpOnly,
				"mcp-only",
				false,
				"",
			)
			if err := cmd.Flags().Set(
				testCase.flag,
				"true",
			); err != nil {
				t.Fatalf("set host flag: %v", err)
			}
			if testCase.mcpOnly {
				if err := cmd.Flags().Set(
					"mcp-only",
					"true",
				); err != nil {
					t.Fatalf("set mcp-only flag: %v", err)
				}
			}
			if err := runPublicInit(cmd, nil); err != nil {
				t.Fatalf("runPublicInit: %v", err)
			}
			if _, err := os.Stat(
				testCase.configPath(projectRoot),
			); err != nil {
				t.Fatalf("MCP carrier missing: %v", err)
			}
			assertPublicHostManifestAdapterEdition(
				t,
				filepath.Join(
					projectRoot,
					".haft",
					"host-installations",
					string(testCase.host)+".project.json",
				),
				testCase.adapterEdition,
			)
			instructionPath := testCase.instructionPath(
				projectRoot,
			)
			skillPath := testCase.skillPath(homeRoot)
			if testCase.mcpOnly {
				for _, path := range []string{
					instructionPath,
					skillPath,
				} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf(
							"mcp-only created %s: %v",
							path,
							err,
						)
					}
				}
				if !strings.Contains(
					output.String(),
					"Changed: mcp_changed",
				) ||
					strings.Contains(
						output.String(),
						"skills_changed",
					) ||
					strings.Contains(
						output.String(),
						"instructions_changed",
					) {
					t.Fatalf(
						"mcp-only reload receipt = %q",
						output.String(),
					)
				}
				return
			}
			for _, path := range []string{
				instructionPath,
				skillPath,
			} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf(
						"full integration missing %s: %v",
						path,
						err,
					)
				}
			}
			assertPublicHostManifestAdapterEdition(
				t,
				filepath.Join(
					homeRoot,
					".haft",
					"host-installations",
					string(testCase.host)+".user.json",
				),
				string(testCase.host)+".skills.v1",
			)
			for _, reason := range []string{
				"mcp_changed",
				"skills_changed",
				"instructions_changed",
			} {
				if !strings.Contains(
					output.String(),
					reason,
				) {
					t.Fatalf(
						"full reload receipt omits %s: %q",
						reason,
						output.String(),
					)
				}
			}
		})
	}
}

func assertPublicHostManifestAdapterEdition(
	t *testing.T,
	path string,
	want string,
) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read host installation manifest %s: %v", path, err)
	}
	var manifest struct {
		AdapterEdition string `json:"adapter_edition"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("decode host installation manifest %s: %v", path, err)
	}
	if manifest.AdapterEdition != want {
		t.Fatalf(
			"host manifest %s adapter_edition = %q, want %q\n%s",
			path,
			manifest.AdapterEdition,
			want,
			content,
		)
	}
}

func TestPublicInitProductionAppliesEveryExplicitHostFlag(
	t *testing.T,
) {
	testCases := []struct {
		host initplanning.HostID
		flag string
	}{
		{initplanning.HostClaude, "claude"},
		{initplanning.HostCursor, "cursor"},
		{initplanning.HostGemini, "gemini"},
		{initplanning.HostCodex, "codex"},
		{initplanning.HostAir, "air"},
		{initplanning.HostOpenCode, "opencode"},
		{initplanning.HostHermes, "hermes"},
		{initplanning.HostZed, "zed"},
		{initplanning.HostAntigravity, "agy"},
		{initplanning.HostPi, "pi"},
		{initplanning.HostGrok, "grok"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.flag, func(t *testing.T) {
			projectRoot := t.TempDir()
			homeRoot := t.TempDir()
			restoreDirectory := changeInitTestDirectory(
				t,
				projectRoot,
			)
			defer restoreDirectory()
			restoreFlags := captureInitHostFlagState()
			defer restoreFlags.apply()
			clearInitHostFlags()
			if err := selectInitHostFlag(testCase.host); err != nil {
				t.Fatalf("selectInitHostFlag: %v", err)
			}
			t.Setenv("HOME", homeRoot)
			output := &bytes.Buffer{}
			cmd := newPublicInitTestCommand()
			cmd.SetOut(output)
			var selected bool
			cmd.Flags().BoolVar(
				&selected,
				testCase.flag,
				false,
				"",
			)
			if err := cmd.Flags().Set(
				testCase.flag,
				"true",
			); err != nil {
				t.Fatalf("set %s flag: %v", testCase.flag, err)
			}
			if err := runPublicInit(cmd, nil); err != nil {
				t.Fatalf("runPublicInit: %v", err)
			}
			if !strings.Contains(
				output.String(),
				"Haft initialization complete",
			) {
				t.Fatalf(
					"typed public output = %q",
					output.String(),
				)
			}
			if _, err := os.Stat(
				filepath.Join(projectRoot, ".haft", "project.yaml"),
			); err != nil {
				t.Fatalf("typed project identity missing: %v", err)
			}
		})
	}
}

func TestPublicInitProductionAppliesExplicitSurfaceCombinations(
	t *testing.T,
) {
	testCases := []struct {
		name    string
		globals func()
		flags   []string
		check   func(*testing.T, string, string)
	}{
		{
			name: "multiple hosts",
			globals: func() {
				initClaude = true
				initCodex = true
			},
			flags: []string{"claude", "codex"},
			check: func(
				t *testing.T,
				projectRoot string,
				_ string,
			) {
				t.Helper()
				for _, path := range []string{
					filepath.Join(projectRoot, ".mcp.json"),
					filepath.Join(
						projectRoot,
						".codex",
						"config.toml",
					),
				} {
					if _, err := os.Stat(path); err != nil {
						t.Fatalf("missing selected surface %s: %v", path, err)
					}
				}
			},
		},
		{
			name: "agents alone",
			globals: func() {
				initAgents = true
			},
			flags: []string{"agents"},
			check: func(
				t *testing.T,
				_ string,
				homeRoot string,
			) {
				t.Helper()
				if _, err := os.Stat(filepath.Join(
					homeRoot,
					".agents",
					"skills",
					"h-reason",
					"SKILL.md",
				)); err != nil {
					t.Fatalf("agent skills missing: %v", err)
				}
			},
		},
		{
			name: "all",
			globals: func() {
				initAll = true
			},
			flags: []string{"all"},
			check: func(
				t *testing.T,
				projectRoot string,
				homeRoot string,
			) {
				t.Helper()
				for _, path := range []string{
					filepath.Join(projectRoot, ".mcp.json"),
					filepath.Join(
						projectRoot,
						".codex",
						"config.toml",
					),
					filepath.Join(projectRoot, "CLAUDE.md"),
					filepath.Join(projectRoot, "AGENTS.md"),
					filepath.Join(
						homeRoot,
						".claude",
						"skills",
						"h-reason",
						"SKILL.md",
					),
					filepath.Join(
						homeRoot,
						".agents",
						"skills",
						"h-reason",
						"SKILL.md",
					),
				} {
					if _, err := os.Stat(path); err != nil {
						t.Fatalf(
							"--all stable surface missing %s: %v",
							path,
							err,
						)
					}
				}
				for _, path := range []string{
					filepath.Join(projectRoot, ".cursor"),
					filepath.Join(homeRoot, ".gemini"),
				} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf(
							"--all selected experimental host path %s: %v",
							path,
							err,
						)
					}
				}
			},
		},
		{
			name: "core only",
			globals: func() {
				initCoreOnly = true
			},
			flags: []string{"core-only"},
			check: func(
				t *testing.T,
				projectRoot string,
				_ string,
			) {
				t.Helper()
				if _, err := os.Stat(filepath.Join(
					projectRoot,
					".haft",
					"project.yaml",
				)); err != nil {
					t.Fatalf("core identity missing: %v", err)
				}
				if _, err := os.Stat(filepath.Join(
					projectRoot,
					".codex",
				)); !os.IsNotExist(err) {
					t.Fatalf("core-only implied host surface: %v", err)
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			homeRoot := t.TempDir()
			restoreDirectory := changeInitTestDirectory(
				t,
				projectRoot,
			)
			defer restoreDirectory()
			restoreFlags := captureInitHostFlagState()
			defer restoreFlags.apply()
			clearInitHostFlags()
			testCase.globals()
			t.Setenv("HOME", homeRoot)
			output := &bytes.Buffer{}
			cmd := newPublicInitTestCommand()
			cmd.SetOut(output)
			for _, flag := range testCase.flags {
				var selected bool
				cmd.Flags().BoolVar(&selected, flag, false, "")
				if err := cmd.Flags().Set(flag, "true"); err != nil {
					t.Fatalf("set %s flag: %v", flag, err)
				}
			}
			if err := runPublicInit(cmd, nil); err != nil {
				t.Fatalf("runPublicInit: %v", err)
			}
			if !strings.Contains(
				output.String(),
				"Haft initialization complete",
			) {
				t.Fatalf(
					"typed public output = %q",
					output.String(),
				)
			}
			testCase.check(t, projectRoot, homeRoot)
		})
	}
}

func TestPublicInitProductionPreservesContentOutsideManagedMarkers(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initClaude = true
	t.Setenv("HOME", homeRoot)
	instructionPath := filepath.Join(projectRoot, "CLAUDE.md")
	operatorContent := "# Operator prelude\n\nKeep this text.\n"
	if err := os.WriteFile(
		instructionPath,
		[]byte(operatorContent),
		0o640,
	); err != nil {
		t.Fatalf("write operator instructions: %v", err)
	}
	cmd := newPublicInitTestCommand()
	var claude bool
	cmd.Flags().BoolVar(&claude, "claude", false, "")
	if err := cmd.Flags().Set("claude", "true"); err != nil {
		t.Fatalf("set Claude flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("runPublicInit: %v", err)
	}
	content, err := os.ReadFile(instructionPath)
	if err != nil {
		t.Fatalf("read merged instructions: %v", err)
	}
	info, err := os.Stat(instructionPath)
	if err != nil {
		t.Fatalf("stat merged instructions: %v", err)
	}
	if !strings.Contains(string(content), operatorContent) ||
		!strings.Contains(string(content), "<!-- haft:start -->") ||
		!strings.Contains(string(content), "<!-- haft:end -->") ||
		info.Mode().Perm() != 0o640 {
		t.Fatalf(
			"merged instructions mode=%o content=%q",
			info.Mode().Perm(),
			content,
		)
	}
}

func TestPublicInitCodexPreservesAGENTSBytesOutsideManagedMarkers(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)
	instructionPath := filepath.Join(projectRoot, "AGENTS.md")
	operatorContent := []byte(
		"# Operator prelude\r\n\r\nKeep these exact CRLF bytes.\r\n",
	)
	if err := os.WriteFile(
		instructionPath,
		operatorContent,
		0o600,
	); err != nil {
		t.Fatalf("write operator instructions: %v", err)
	}
	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("runPublicInit: %v", err)
	}
	content, err := os.ReadFile(instructionPath)
	if err != nil {
		t.Fatalf("read merged instructions: %v", err)
	}
	info, err := os.Stat(instructionPath)
	if err != nil {
		t.Fatalf("stat merged instructions: %v", err)
	}
	if !bytes.HasPrefix(content, operatorContent) ||
		!bytes.Contains(content, []byte("<!-- haft:start -->")) ||
		!bytes.Contains(content, []byte("<!-- haft:end -->")) ||
		info.Mode().Perm() != 0o600 {
		t.Fatalf(
			"merged AGENTS mode=%o content=%q",
			info.Mode().Perm(),
			content,
		)
	}
}

func TestPublicInitCodexReplacesPreManifestHaftInstructions(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)
	instructionPath := filepath.Join(projectRoot, "AGENTS.md")
	prefix := []byte("# Operator prelude\n\n")
	legacy := []byte(
		"<!-- haft:start -->\n" +
			"# Legacy Haft instructions\n" +
			"<!-- haft:end -->",
	)
	suffix := []byte("\n\n## Local project rules\n")
	content := make([]byte, 0)
	content = append(content, prefix...)
	content = append(content, legacy...)
	content = append(content, suffix...)
	if err := os.WriteFile(instructionPath, content, 0o640); err != nil {
		t.Fatalf("write legacy AGENTS instructions: %v", err)
	}
	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("runPublicInit: %v", err)
	}
	updated, err := os.ReadFile(instructionPath)
	if err != nil {
		t.Fatalf("read updated AGENTS instructions: %v", err)
	}
	if !bytes.HasPrefix(updated, prefix) ||
		!bytes.HasSuffix(updated, suffix) ||
		bytes.Contains(updated, []byte("Legacy Haft instructions")) ||
		!bytes.Contains(updated, []byte(embeddedClaudeMDTemplate)) {
		t.Fatalf("pre-manifest AGENTS update = %q", updated)
	}
	manifestPath := filepath.Join(
		projectRoot,
		".haft",
		"host-installations",
		"codex.project.json",
	)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("Codex project manifest missing: %v", err)
	}
}

func TestPublicInitProductionDoesNotAdoptForeignManagedSurface(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)
	foreignPath := filepath.Join(
		projectRoot,
		".codex",
		"config.toml",
	)
	if err := os.MkdirAll(filepath.Dir(foreignPath), 0o755); err != nil {
		t.Fatalf("create foreign Codex root: %v", err)
	}
	foreign := []byte(
		"[mcp_servers.haft]\ncommand = \"foreign\"\n",
	)
	if err := os.WriteFile(foreignPath, foreign, 0o644); err != nil {
		t.Fatalf("write foreign Codex surface: %v", err)
	}
	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetOut(output)
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	err := runPublicInit(cmd, nil)
	if err == nil ||
		!strings.Contains(err.Error(), "cannot safely update") {
		t.Fatalf("foreign surface error = %v", err)
	}
	content, err := os.ReadFile(foreignPath)
	if err != nil {
		t.Fatalf("read foreign Codex surface: %v", err)
	}
	if string(content) != string(foreign) {
		t.Fatalf("foreign Codex surface changed: %q", content)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(err) {
		t.Fatalf("blocked foreign surface wrote project core: %v", err)
	}
}

func TestPublicInitCodexReplacesKnownQuintMCPAndPreservesForeignTOML(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	configPath := filepath.Join(
		projectRoot,
		".codex",
		"config.toml",
	)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create Codex config root: %v", err)
	}
	operatorPrefix := []byte(
		"# operator-prefix\nmodel = \"gpt-5\"\n\n",
	)
	operatorSuffix := []byte(
		"\n[mcp_servers.private]\ncommand = \"private-server\"\n",
	)
	legacy := publicLegacyCodexQuintContent(
		"quint-code",
		projectRoot,
	)
	fixture := append([]byte{}, operatorPrefix...)
	fixture = append(fixture, legacy...)
	fixture = append(fixture, operatorSuffix...)
	if err := os.WriteFile(configPath, fixture, 0o640); err != nil {
		t.Fatalf("write known Quint config: %v", err)
	}

	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("replace known Quint config: %v", err)
	}
	applied, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read applied Codex config: %v", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat applied Codex config: %v", err)
	}
	if bytes.Contains(
		applied,
		[]byte("[mcp_servers.quint-code]"),
	) ||
		!bytes.Contains(applied, []byte("[mcp_servers.haft]")) ||
		!bytes.Contains(applied, operatorPrefix) ||
		!bytes.Contains(applied, operatorSuffix) ||
		info.Mode().Perm() != 0o640 {
		t.Fatalf(
			"known Quint replacement mode=%o content:\n%s",
			info.Mode().Perm(),
			applied,
		)
	}
}

func TestPublicInitCodexReplacesKnownPortableHaftMCPAndPreservesForeignTOML(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	configPath := filepath.Join(
		projectRoot,
		".codex",
		"config.toml",
	)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create Codex config root: %v", err)
	}
	operatorPrefix := []byte(
		"approval_policy = \"never\"\n" +
			"sandbox_mode = \"danger-full-access\"\n\n",
	)
	legacy := publicLegacyPortableCodexHaftContent()
	operatorSuffix := []byte(
		"\n[mcp_servers.haft.tools.haft_query]\n" +
			"approval_mode = \"approve\"\n\n" +
			"[mcp_servers.haft.tools.haft_method]\n" +
			"approval_mode = \"approve\"\n\n" +
			"[mcp_servers.private]\n" +
			"command = \"private-server\"\n" +
			"env = { PRIVATE_MODE = \"fixture\" }\n \t\n\t ",
	)
	fixture := append([]byte{}, operatorPrefix...)
	fixture = append(fixture, legacy...)
	fixture = append(fixture, operatorSuffix...)
	if err := os.WriteFile(configPath, fixture, 0o640); err != nil {
		t.Fatalf("write known portable Haft config: %v", err)
	}

	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("replace known portable Haft config: %v", err)
	}
	applied, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read applied Codex config: %v", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat applied Codex config: %v", err)
	}
	config, err := currentPublicProjectConfig(projectRoot)
	if err != nil {
		t.Fatalf("read applied project identity: %v", err)
	}
	expectedManaged, err := currentCodexTOMLFragmentContent(
		currentCoherentHostContext{
			projectRoot: projectRoot,
			projectID:   config.ID,
		},
	)
	if err != nil {
		t.Fatalf("render expected Codex Haft tables: %v", err)
	}
	expected := append([]byte{}, operatorPrefix...)
	expected = append(expected, expectedManaged...)
	expected = append(expected, operatorSuffix...)
	if !bytes.Equal(applied, expected) ||
		info.Mode().Perm() != 0o640 {
		t.Fatalf(
			"known portable Haft replacement mode=%o\n"+
				"want bytes:\n%q\n"+
				"got bytes:\n%q",
			info.Mode().Perm(),
			expected,
			applied,
		)
	}
}

func TestPublicInitCodexReplacesPublishedV8HaftMCPAndPreservesForeignTOML(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	configPath := filepath.Join(projectRoot, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create Codex config root: %v", err)
	}
	operatorPrefix := []byte("approval_policy = \"never\"\n\n")
	legacy := []byte(`[mcp_servers.haft]
command = "haft"
args = ["serve"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
`)
	operatorSuffix := []byte(
		"\n[mcp_servers.haft.tools.haft_query]\n" +
			"approval_mode = \"approve\"\n\n" +
			"[mcp_servers.private]\n" +
			"command = \"private-server\"\n",
	)
	fixture := append([]byte{}, operatorPrefix...)
	fixture = append(fixture, legacy...)
	fixture = append(fixture, operatorSuffix...)
	if err := os.WriteFile(configPath, fixture, 0o640); err != nil {
		t.Fatalf("write published v8 Codex config: %v", err)
	}

	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("replace published v8 Codex config: %v", err)
	}
	applied, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated Codex config: %v", err)
	}
	config, err := currentPublicProjectConfig(projectRoot)
	if err != nil {
		t.Fatalf("read applied project identity: %v", err)
	}
	expectedManaged, err := currentCodexTOMLFragmentContent(
		currentCoherentHostContext{
			projectRoot: projectRoot,
			projectID:   config.ID,
		},
	)
	if err != nil {
		t.Fatalf("render expected Codex Haft tables: %v", err)
	}
	expected := append([]byte{}, operatorPrefix...)
	expected = append(expected, expectedManaged...)
	expected = append(expected, operatorSuffix...)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat migrated Codex config: %v", err)
	}
	if !bytes.Equal(applied, expected) || info.Mode().Perm() != 0o640 {
		t.Fatalf(
			"published v8 Codex migration mode=%o\nwant:\n%q\ngot:\n%q",
			info.Mode().Perm(),
			expected,
			applied,
		)
	}
}

func TestPublicLegacyPublishedV8CodexHaftTablesRequireExactBinaryShape(
	t *testing.T,
) {
	t.Parallel()

	valid := []byte(`[mcp_servers.haft]
command = "/usr/local/bin/haft"
args = ["serve"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
`)
	if !isPublicLegacyPublishedV8CodexHaftTables(valid) {
		t.Fatal("exact published v8 Codex Haft tables were rejected")
	}
	for name, nearMiss := range map[string][]byte{
		"operator wrapper": bytes.Replace(
			valid,
			[]byte(`/usr/local/bin/haft`),
			[]byte(`/usr/local/bin/operator-wrapper`),
			1,
		),
		"different project root": bytes.Replace(
			valid,
			[]byte(`HAFT_PROJECT_ROOT = "."`),
			[]byte(`HAFT_PROJECT_ROOT = ".."`),
			1,
		),
		"extra authority field": bytes.Replace(
			valid,
			[]byte("tool_timeout_sec = 60\n"),
			[]byte("tool_timeout_sec = 60\ndefault_tools_approval_mode = \"prompt\"\n"),
			1,
		),
	} {
		t.Run(name, func(t *testing.T) {
			if isPublicLegacyPublishedV8CodexHaftTables(nearMiss) {
				t.Fatal("near-miss published v8 shape was accepted")
			}
		})
	}
}

func TestPublicInitCodexRejectsNearMissPortableHaftMCPBeforeAnyWrite(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	configPath := filepath.Join(
		projectRoot,
		".codex",
		"config.toml",
	)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create Codex config root: %v", err)
	}
	foreign := bytes.Replace(
		publicLegacyPortableCodexHaftContent(),
		[]byte(`command = "haft"`),
		[]byte(`command = "operator-wrapper"`),
		1,
	)
	if err := os.WriteFile(configPath, foreign, 0o644); err != nil {
		t.Fatalf("write near-miss portable Haft config: %v", err)
	}

	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	err := runPublicInit(cmd, nil)
	if err == nil ||
		!strings.Contains(err.Error(), "cannot safely update") {
		t.Fatalf("near-miss portable Haft error = %v", err)
	}
	preserved, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read preserved near-miss config: %v", err)
	}
	if !bytes.Equal(preserved, foreign) {
		t.Fatalf("near-miss portable Haft config changed:\n%s", preserved)
	}
	for _, path := range []string{
		filepath.Join(projectRoot, ".haft"),
		filepath.Join(projectRoot, "AGENTS.md"),
		filepath.Join(homeRoot, ".agents"),
		filepath.Join(homeRoot, ".haft"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf(
				"blocked near-miss portable Haft init wrote %s: %v",
				path,
				err,
			)
		}
	}
}

func TestPublicInitCodexRejectsForeignQuintMCPBeforeAnyWrite(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	configPath := filepath.Join(
		projectRoot,
		".codex",
		"config.toml",
	)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create Codex config root: %v", err)
	}
	foreign := publicLegacyCodexQuintContent(
		"operator-owned-wrapper",
		projectRoot,
	)
	if err := os.WriteFile(configPath, foreign, 0o644); err != nil {
		t.Fatalf("write foreign Quint config: %v", err)
	}

	cmd := newPublicInitTestCommand()
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	err := runPublicInit(cmd, nil)
	if err == nil ||
		!strings.Contains(err.Error(), "cannot safely update") {
		t.Fatalf("foreign Quint error = %v", err)
	}
	preserved, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read preserved foreign config: %v", err)
	}
	if !bytes.Equal(preserved, foreign) {
		t.Fatalf("foreign Quint config changed:\n%s", preserved)
	}
	for _, path := range []string{
		filepath.Join(projectRoot, ".haft"),
		filepath.Join(projectRoot, "AGENTS.md"),
		filepath.Join(homeRoot, ".agents"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("blocked foreign Quint init wrote %s: %v", path, err)
		}
	}
}

func TestPublicInitCodexAdoptsExistingConfigAndPreservesToolApprovals(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	workspaceRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve test workspace: %v", err)
	}
	homeRoot, err := os.MkdirTemp(
		workspaceRoot,
		".test-init-home-*",
	)
	if err != nil {
		t.Fatalf("create physical test home: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(homeRoot) })
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	projectRoot, err = os.Getwd()
	if err != nil {
		t.Fatalf("resolve physical project root: %v", err)
	}
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", homeRoot)

	haftRoot := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftRoot, 0o755); err != nil {
		t.Fatalf("create existing Haft root: %v", err)
	}
	projectConfig := "id: qnt_e3149c17\nname: " +
		filepath.Base(projectRoot) + "\n"
	if err := os.WriteFile(
		filepath.Join(haftRoot, "project.yaml"),
		[]byte(projectConfig),
		0o644,
	); err != nil {
		t.Fatalf("write existing project identity: %v", err)
	}
	config, err := currentPublicProjectConfig(projectRoot)
	if err != nil {
		t.Fatalf("propose project config: %v", err)
	}
	haftTables, err := currentCodexTOMLFragmentContent(
		currentCoherentHostContext{
			projectRoot: projectRoot,
			projectID:   config.ID,
		},
	)
	if err != nil {
		t.Fatalf("render existing Codex config: %v", err)
	}
	configPath := filepath.Join(projectRoot, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create existing Codex config root: %v", err)
	}
	userOwned := `
[mcp_servers.haft.tools.haft_query]
approval_mode = "approve"

[mcp_servers.haft.tools.haft_method]
approval_mode = "approve"
`
	if err := os.WriteFile(
		configPath,
		append(haftTables, []byte(userOwned)...),
		0o644,
	); err != nil {
		t.Fatalf("write existing Codex config: %v", err)
	}

	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetOut(output)
	var codex bool
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	request, err := currentPublicInitRequest(
		cmd,
		initplanning.InvocationExplicit,
	)
	if err != nil {
		t.Fatalf("compile Codex init request: %v", err)
	}
	if request.projectID != config.ID {
		t.Fatalf(
			"prepared project id = %s, existing config id = %s",
			request.projectID,
			config.ID,
		)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("adopt existing Codex config: %v", err)
	}
	if strings.Contains(output.String(), "Haft initialization plan") ||
		strings.Contains(output.String(), "Apply this exact plan?") ||
		!strings.Contains(
			output.String(),
			"registered existing MCP configuration (.codex/config.toml)",
		) {
		t.Fatalf("unexpected Codex adoption output: %q", output.String())
	}

	appliedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read applied Codex config: %v", err)
	}
	if !bytes.Contains(appliedConfig, []byte(userOwned)) {
		t.Fatalf(
			"Codex tool approvals were not preserved:\n%s",
			appliedConfig,
		)
	}
	manifestPath := filepath.Join(
		projectRoot,
		".haft",
		"host-installations",
		"codex.project.json",
	)
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read Codex manifest: %v", err)
	}
	for _, expected := range []string{
		`"kind":"toml_table_set"`,
		`"toml_tables":["mcp_servers.haft","mcp_servers.haft.env"]`,
	} {
		if !bytes.Contains(manifest, []byte(expected)) {
			t.Fatalf(
				"Codex manifest omits %s:\n%s",
				expected,
				manifest,
			)
		}
	}

	legacyManifest := bytes.Replace(
		manifest,
		[]byte(`"kind":"toml_table_set"`),
		[]byte(`"kind":"toml_table_family"`),
		1,
	)
	legacyManifest = bytes.Replace(
		legacyManifest,
		[]byte(`,"toml_tables":["mcp_servers.haft","mcp_servers.haft.env"]`),
		nil,
		1,
	)
	if bytes.Equal(legacyManifest, manifest) {
		t.Fatal("failed to construct broad legacy Codex manifest")
	}
	if err := os.WriteFile(manifestPath, legacyManifest, 0o644); err != nil {
		t.Fatalf("write broad legacy Codex manifest: %v", err)
	}

	secondOutput := &bytes.Buffer{}
	second := newPublicInitTestCommand()
	second.SetOut(secondOutput)
	var secondCodex bool
	second.Flags().BoolVar(&secondCodex, "codex", false, "")
	if err := second.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set second Codex flag: %v", err)
	}
	if err := runPublicInit(second, nil); err != nil {
		t.Fatalf("migrate broad legacy Codex manifest: %v", err)
	}
	if strings.Contains(secondOutput.String(), "Haft initialization plan") ||
		strings.Contains(secondOutput.String(), "Apply this exact plan?") {
		t.Fatalf(
			"legacy migration exposed internal plan: %q",
			secondOutput.String(),
		)
	}
	migratedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated Codex config: %v", err)
	}
	if !bytes.Contains(migratedConfig, []byte(userOwned)) {
		t.Fatalf(
			"legacy migration changed Codex tool approvals:\n%s",
			migratedConfig,
		)
	}
	migratedManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read migrated Codex manifest: %v", err)
	}
	if !bytes.Contains(
		migratedManifest,
		[]byte(`"kind":"toml_table_set"`),
	) {
		t.Fatalf(
			"legacy Codex manifest did not migrate:\n%s",
			migratedManifest,
		)
	}
}

func TestPublicInitInteractiveSelectionAppliesWithoutSecondConfirmation(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	t.Setenv("HOME", homeRoot)

	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(output)
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return true
		},
		func(
			session initplanning.InteractiveSession,
			_ io.Reader,
			_ io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			selected, err := toggleInitTestHost(
				session,
				initplanning.HostCodex,
			)
			if err != nil {
				return nil, err
			}
			confirmed, err := selected.Reduce(
				initplanning.ConfirmSelectionEvent{},
			)
			if err != nil {
				return nil, err
			}
			return confirmed.Outcome(), nil
		},
		runTypedPublicInit,
	)
	if err != nil {
		t.Fatalf("interactive typed init: %v", err)
	}
	if !strings.Contains(
		output.String(),
		"Haft initialization complete",
	) ||
		strings.Contains(
			output.String(),
			"Apply this exact plan?",
		) ||
		strings.Contains(
			output.String(),
			"Haft initialization plan",
		) {
		t.Fatalf("interactive typed output = %q", output.String())
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".codex", "config.toml"),
	); err != nil {
		t.Fatalf("interactive Codex config missing: %v", err)
	}
}

func TestPublicInitInteractiveCancellationLeavesProjectUntouched(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	t.Setenv("HOME", homeRoot)

	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetOut(output)
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return true
		},
		func(
			_ initplanning.InteractiveSession,
			_ io.Reader,
			_ io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			return initplanning.InteractiveCancelledOutcome{}, nil
		},
		runTypedPublicInit,
	)
	if err != nil {
		t.Fatalf("interactive typed cancellation: %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(err) {
		t.Fatalf("cancelled selection wrote .haft: %v", err)
	}
	if !strings.Contains(
		output.String(),
		"Initialization cancelled; no files were changed.",
	) {
		t.Fatalf("cancellation output = %q", output.String())
	}
}

func TestPublicInitExplicitFlagBypassesMenu(t *testing.T) {
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true

	cmd := newPublicInitTestCommand()
	var explicit bool
	cmd.Flags().BoolVar(&explicit, "codex", false, "")
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set explicit flag: %v", err)
	}

	selectionCalled := false
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return false
		},
		func(
			initplanning.InteractiveSession,
			io.Reader,
			io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			selectionCalled = true
			return initplanning.InteractiveCancelledOutcome{}, nil
		},
		func(
			_ *cobra.Command,
			_ []string,
			policy initplanning.InvocationPolicy,
		) error {
			applyCalled = true
			if policy != initplanning.InvocationExplicit {
				t.Fatalf("explicit invocation policy = %q", policy)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runPublicInitWith: %v", err)
	}
	if selectionCalled || !applyCalled {
		t.Fatalf(
			"explicit path called selection=%t apply=%t",
			selectionCalled,
			applyCalled,
		)
	}
}

func TestPublicInitInteractiveSelectionLowersToExistingHostFlags(t *testing.T) {
	projectRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()

	cmd := newPublicInitTestCommand()
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return true
		},
		func(
			session initplanning.InteractiveSession,
			_ io.Reader,
			_ io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			next, err := toggleInitTestHost(
				session,
				initplanning.HostClaude,
			)
			if err != nil {
				return nil, err
			}
			next, err = toggleInitTestHost(
				next,
				initplanning.HostCodex,
			)
			if err != nil {
				return nil, err
			}
			confirmed, err := next.Reduce(
				initplanning.ConfirmSelectionEvent{},
			)
			if err != nil {
				return nil, err
			}
			return confirmed.Outcome(), nil
		},
		func(
			_ *cobra.Command,
			_ []string,
			policy initplanning.InvocationPolicy,
		) error {
			applyCalled = true
			if policy != initplanning.InvocationInteractive {
				t.Fatalf("interactive invocation policy = %q", policy)
			}
			if !initClaude || !initCodex {
				t.Fatalf(
					"interactive flags claude=%t codex=%t",
					initClaude,
					initCodex,
				)
			}
			if initCoreOnly || initAll || initGemini {
				t.Fatalf(
					"unexpected flags core=%t all=%t gemini=%t",
					initCoreOnly,
					initAll,
					initGemini,
				)
			}
			if initAgents {
				t.Fatal("interactive host selection changed independent --agents state")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runPublicInitWith: %v", err)
	}
	if !applyCalled {
		t.Fatal("interactive selection did not reach init effects")
	}
}

func TestPublicInitInteractiveEmptySelectionIsCoreOnly(t *testing.T) {
	projectRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()

	cmd := newPublicInitTestCommand()
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return true
		},
		func(
			session initplanning.InteractiveSession,
			_ io.Reader,
			_ io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			confirmed, err := session.Reduce(
				initplanning.ConfirmSelectionEvent{},
			)
			if err != nil {
				return nil, err
			}
			return confirmed.Outcome(), nil
		},
		func(*cobra.Command, []string, initplanning.InvocationPolicy) error {
			if !initCoreOnly {
				t.Fatal("empty interactive selection did not select core-only")
			}
			if hasRequestedInitHost(requestedInitHostOptions()) {
				t.Fatal("empty interactive selection implied a host")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runPublicInitWith: %v", err)
	}
}

func TestPublicInitCancellationDoesNotApply(t *testing.T) {
	projectRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()

	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetOut(output)
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return true
		},
		func(
			initplanning.InteractiveSession,
			io.Reader,
			io.Writer,
		) (initplanning.InteractiveOutcome, error) {
			return initplanning.InteractiveCancelledOutcome{}, nil
		},
		func(*cobra.Command, []string, initplanning.InvocationPolicy) error {
			applyCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runPublicInitWith: %v", err)
	}
	if applyCalled {
		t.Fatal("cancelled selection reached init effects")
	}
	if !strings.Contains(output.String(), "no files were changed") {
		t.Fatalf("cancellation output = %q", output.String())
	}
}

func TestPublicInitCoreOnlyRejectsHostCombinationBeforeApply(t *testing.T) {
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCoreOnly = true
	initCodex = true

	cmd := newPublicInitTestCommand()
	var coreOnly bool
	cmd.Flags().BoolVar(&coreOnly, "core-only", false, "")
	if err := cmd.Flags().Set("core-only", "true"); err != nil {
		t.Fatalf("set core-only flag: %v", err)
	}
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return false
		},
		nil,
		func(*cobra.Command, []string, initplanning.InvocationPolicy) error {
			applyCalled = true
			return nil
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("runPublicInitWith error = %v", err)
	}
	if applyCalled {
		t.Fatal("invalid core-only combination reached init effects")
	}
}

func TestPublicInitCoreOnlyRejectsOverseerConfigurationBeforeApply(
	t *testing.T,
) {
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCoreOnly = true

	cmd := newPublicInitTestCommand()
	var coreOnly bool
	var reviewer string
	cmd.Flags().BoolVar(&coreOnly, "core-only", false, "")
	cmd.Flags().StringVar(&reviewer, "overseer-reviewer", "auto", "")
	if err := cmd.Flags().Set("core-only", "true"); err != nil {
		t.Fatalf("set core-only flag: %v", err)
	}
	if err := cmd.Flags().Set("overseer-reviewer", "codex"); err != nil {
		t.Fatalf("set overseer reviewer: %v", err)
	}
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return false
		},
		nil,
		func(*cobra.Command, []string, initplanning.InvocationPolicy) error {
			applyCalled = true
			return nil
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "overseer installation flags") {
		t.Fatalf("runPublicInitWith error = %v", err)
	}
	if applyCalled {
		t.Fatal("invalid core-only overseer combination reached init effects")
	}
}

func TestPublicInitCoreOnlyRejectsAgentsBeforeApply(t *testing.T) {
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCoreOnly = true
	initAgents = true

	cmd := newPublicInitTestCommand()
	var coreOnly bool
	cmd.Flags().BoolVar(&coreOnly, "core-only", false, "")
	if err := cmd.Flags().Set("core-only", "true"); err != nil {
		t.Fatalf("set core-only flag: %v", err)
	}
	applyCalled := false
	err := runPublicInitWithTypedEffect(
		cmd,
		nil,
		func(*cobra.Command) bool {
			return false
		},
		nil,
		func(*cobra.Command, []string, initplanning.InvocationPolicy) error {
			applyCalled = true
			return nil
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "cannot be combined with --agents") {
		t.Fatalf("runPublicInitWith error = %v", err)
	}
	if applyCalled {
		t.Fatal("invalid core-only agents combination reached init effects")
	}
}

func TestPublicInitMenuCoversCoherentHostRegistry(t *testing.T) {
	t.Parallel()

	registry, err := currentCoherentHostApplicabilityRegistry()
	if err != nil {
		t.Fatalf("currentCoherentHostApplicabilityRegistry: %v", err)
	}
	_, choices, err := publicInitAdapterCatalog()
	if err != nil {
		t.Fatalf("publicInitAdapterCatalog: %v", err)
	}
	if len(choices) != len(registry) {
		t.Fatalf(
			"menu choices = %d, coherent hosts = %d",
			len(choices),
			len(registry),
		)
	}
	seen := make(map[string]struct{}, len(choices))
	for _, choice := range choices {
		if len(choice.Components) == 0 {
			t.Fatalf("menu host %s has no components", choice.Host)
		}
		if _, duplicate := seen[choice.Host]; duplicate {
			t.Fatalf("menu repeats host %s", choice.Host)
		}
		seen[choice.Host] = struct{}{}
		if choice.Host != string(initplanning.HostCodex) {
			continue
		}
		if !slices.Contains(
			choice.Components,
			string(initplanning.ComponentMCP),
		) {
			t.Fatalf("Codex menu components = %v, want MCP", choice.Components)
		}
		if !slices.Contains(
			choice.Components,
			string(initplanning.ComponentSkills),
		) {
			t.Fatalf(
				"Codex menu components = %v, want full skills effect",
				choice.Components,
			)
		}
		if !slices.Contains(
			choice.Components,
			string(initplanning.ComponentInstructions),
		) {
			t.Fatalf(
				"Codex menu components = %v, want AGENTS.md instructions",
				choice.Components,
			)
		}
	}
}

func newPublicInitTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(io.Discard)
	return cmd
}

func toggleInitTestHost(
	session initplanning.InteractiveSession,
	host initplanning.HostID,
) (initplanning.InteractiveSession, error) {
	event, err := initplanning.NewToggleHostEvent(host)
	if err != nil {
		return initplanning.InteractiveSession{}, err
	}
	return session.Reduce(event)
}

func changeInitTestDirectory(
	t *testing.T,
	target string,
) func() {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(target); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	return func() {
		if err := os.Chdir(current); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}
}
