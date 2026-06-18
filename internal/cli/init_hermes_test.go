package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/overseer"
	"gopkg.in/yaml.v3"
)

func TestNormalizeInitHostOptionsHermesSuppressesClaudeDefault(t *testing.T) {
	got := normalizeInitHostOptions(initHostOptions{hermes: true})
	want := initHostOptions{hermes: true}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
}

func TestInstallHermesMaterializesSkillsAndConfig(t *testing.T) {
	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	hermesHome := filepath.Join(homeDir, ".hermes")
	t.Setenv("HOME", homeDir)

	cfg := createInitSmokeProject(t, projectRoot)

	result, err := installHermes(projectRoot, "/opt/haft/bin/haft", hermesHome, "")
	if err != nil {
		t.Fatalf("installHermes: %v", err)
	}

	wantSkillsRoot := filepath.Join(homeDir, ".haft", "hermes", "skills")
	if result.SkillsRoot != wantSkillsRoot {
		t.Fatalf("SkillsRoot = %q, want %q", result.SkillsRoot, wantSkillsRoot)
	}
	if result.SkillCount != len(allSkills) {
		t.Fatalf("SkillCount = %d, want %d", result.SkillCount, len(allSkills))
	}

	frameSkillPath := filepath.Join(result.SkillsRoot, "haft", "h-frame", "SKILL.md")
	frameSkill, err := os.ReadFile(frameSkillPath)
	if err != nil {
		t.Fatalf("read h-frame Hermes skill: %v", err)
	}
	frameText := string(frameSkill)
	if !strings.Contains(frameText, "name: h-frame") {
		t.Fatalf("h-frame skill missing frontmatter name:\n%s", frameText)
	}
	if !strings.Contains(frameText, "haft_problem(") {
		t.Fatalf("h-frame skill did not adapt tool references:\n%s", frameText)
	}
	if strings.Contains(frameText, "mcp__haft__haft_problem") {
		t.Fatalf("h-frame skill still contains Claude-style tool name:\n%s", frameText)
	}

	settings := readHermesConfigForTest(t, result.ConfigPath)
	mcpServers := mapForTest(t, settings, "mcp_servers")
	server := mapForTest(t, mcpServers, "haft")
	if server["command"] != "/opt/haft/bin/haft" {
		t.Fatalf("Hermes command = %#v, want absolute haft binary", server["command"])
	}
	if server["enabled"] != true {
		t.Fatalf("Hermes enabled = %#v, want true", server["enabled"])
	}

	args := listForTest(t, server, "args")
	if len(args) != 1 || args[0] != "serve" {
		t.Fatalf("Hermes args = %#v, want [serve]", args)
	}

	env := mapForTest(t, server, "env")
	if env[envProjectRoot] != projectRoot {
		t.Fatalf("HAFT_PROJECT_ROOT = %#v, want %s", env[envProjectRoot], projectRoot)
	}
	if env[envExpectedProjectID] != cfg.ID {
		t.Fatalf("HAFT_EXPECTED_PROJECT_ID = %#v, want %s", env[envExpectedProjectID], cfg.ID)
	}

	skills := mapForTest(t, settings, "skills")
	externalDirs := listForTest(t, skills, "external_dirs")
	if len(externalDirs) != 1 || externalDirs[0] != result.SkillsRoot {
		t.Fatalf("external_dirs = %#v, want [%s]", externalDirs, result.SkillsRoot)
	}
}

func TestConfigureMCPHermesPreservesExistingConfigAndDedupe(t *testing.T) {
	projectRoot := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	skillsRoot := filepath.Join(t.TempDir(), "skills")

	existing := strings.Join([]string{
		"theme: dark",
		"skills:",
		"  external_dirs:",
		"    - /foreign/skills",
		"    - " + skillsRoot,
		"mcp_servers:",
		"  other:",
		"    command: other-server",
		"  quint-code:",
		"    command: old-haft",
		"",
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := configureMCPHermes(projectRoot, "/bin/haft", configPath, skillsRoot); err != nil {
		t.Fatalf("first configureMCPHermes: %v", err)
	}
	if err := configureMCPHermes(projectRoot, "/bin/haft", configPath, skillsRoot); err != nil {
		t.Fatalf("second configureMCPHermes: %v", err)
	}

	settings := readHermesConfigForTest(t, configPath)
	if settings["theme"] != "dark" {
		t.Fatalf("theme = %#v, want preserved dark", settings["theme"])
	}

	mcpServers := mapForTest(t, settings, "mcp_servers")
	if _, ok := mcpServers["other"]; !ok {
		t.Fatal("other MCP server was clobbered")
	}
	if _, ok := mcpServers["quint-code"]; ok {
		t.Fatal("legacy quint-code MCP server was not removed")
	}
	if _, ok := mcpServers["haft"]; !ok {
		t.Fatal("haft MCP server was not installed")
	}

	skills := mapForTest(t, settings, "skills")
	externalDirs := listForTest(t, skills, "external_dirs")
	want := []any{"/foreign/skills", skillsRoot}
	if !reflect.DeepEqual(externalDirs, want) {
		t.Fatalf("external_dirs = %#v, want %#v", externalDirs, want)
	}
}

func TestResolveHermesHomeUsesProfileWhenHomeIsImplicit(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	got, err := resolveHermesHome("", "ku")
	if err != nil {
		t.Fatalf("resolveHermesHome: %v", err)
	}

	want := filepath.Join(homeDir, ".hermes", "profiles", "ku")
	if got != want {
		t.Fatalf("home = %q, want %q", got, want)
	}
}

func TestRunInitHermesDoesNotCreateClaudeInstructionFileByDefault(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	restore := snapshotInitFlags()
	defer restore()

	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := os.Chdir(projectRoot); err != nil {
		t.Fatal(err)
	}

	initClaude = false
	initCursor = false
	initGemini = false
	initCodex = false
	initAir = false
	initOpencode = false
	initHermes = true
	initPi = false
	initAll = false
	initLocal = false
	initNoFileInstructions = false
	initHermesHome = filepath.Join(homeDir, ".hermes")
	initHermesProfile = ""
	initOverseer = false
	initOverseerReviewer = overseer.ReviewerAuto
	initOverseerReviewerCommand = ""
	initOverseerReviewOnHook = false
	initOverseerReviewTimeout = 180

	if err := runInit(nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("Hermes init should not create CLAUDE.md by default; err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".hermes", "config.yaml")); err != nil {
		t.Fatalf("Hermes config was not created: %v", err)
	}
}

func snapshotInitFlags() func() {
	oldInitClaude := initClaude
	oldInitCursor := initCursor
	oldInitGemini := initGemini
	oldInitCodex := initCodex
	oldInitAir := initAir
	oldInitOpencode := initOpencode
	oldInitHermes := initHermes
	oldInitPi := initPi
	oldInitAll := initAll
	oldInitLocal := initLocal
	oldInitNoFileInstructions := initNoFileInstructions
	oldInitHermesHome := initHermesHome
	oldInitHermesProfile := initHermesProfile
	oldInitOverseer := initOverseer
	oldInitOverseerReviewer := initOverseerReviewer
	oldInitOverseerReviewerCommand := initOverseerReviewerCommand
	oldInitOverseerReviewOnHook := initOverseerReviewOnHook
	oldInitOverseerReviewTimeout := initOverseerReviewTimeout

	return func() {
		initClaude = oldInitClaude
		initCursor = oldInitCursor
		initGemini = oldInitGemini
		initCodex = oldInitCodex
		initAir = oldInitAir
		initOpencode = oldInitOpencode
		initHermes = oldInitHermes
		initPi = oldInitPi
		initAll = oldInitAll
		initLocal = oldInitLocal
		initNoFileInstructions = oldInitNoFileInstructions
		initHermesHome = oldInitHermesHome
		initHermesProfile = oldInitHermesProfile
		initOverseer = oldInitOverseer
		initOverseerReviewer = oldInitOverseerReviewer
		initOverseerReviewerCommand = oldInitOverseerReviewerCommand
		initOverseerReviewOnHook = oldInitOverseerReviewOnHook
		initOverseerReviewTimeout = oldInitOverseerReviewTimeout
	}
}

func readHermesConfigForTest(t *testing.T, configPath string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read Hermes config: %v", err)
	}

	settings := map[string]any{}
	if err := yaml.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse Hermes config: %v\n%s", err, string(data))
	}
	return settings
}

func mapForTest(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("%s missing in %#v", key, parent)
	}

	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want map", key, value)
	}
	return typed
}

func listForTest(t *testing.T, parent map[string]any, key string) []any {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("%s missing in %#v", key, parent)
	}

	typed, ok := value.([]any)
	if !ok {
		t.Fatalf("%s = %#v, want list", key, value)
	}
	return typed
}
