package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/method"
	"github.com/m0n0x41d/haft/internal/project"
	"gopkg.in/yaml.v3"
)

func TestNormalizeInitHostOptionsDefaultsToClaude(t *testing.T) {
	got := normalizeInitHostOptions(initHostOptions{})
	want := initHostOptions{claude: true}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
}

func TestNormalizeInitHostOptionsAllMeansSupportedHostsOnly(t *testing.T) {
	got := normalizeInitHostOptions(initHostOptions{all: true})
	want := initHostOptions{claude: true, codex: true, all: true}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %+v, want %+v", got, want)
	}
}

func TestRunInit_GeminiKeepsLegacyCommands(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	oldInitClaude := initClaude
	oldInitCursor := initCursor
	oldInitGemini := initGemini
	oldInitCodex := initCodex
	oldInitAir := initAir
	oldInitOpencode := initOpencode
	oldInitAll := initAll
	oldInitLocal := initLocal
	oldInitNoClaudeMD := initNoClaudeMD
	defer func() {
		initClaude = oldInitClaude
		initCursor = oldInitCursor
		initGemini = oldInitGemini
		initCodex = oldInitCodex
		initAir = oldInitAir
		initOpencode = oldInitOpencode
		initAll = oldInitAll
		initLocal = oldInitLocal
		initNoClaudeMD = oldInitNoClaudeMD
	}()

	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	commandsDir := filepath.Join(homeDir, ".gemini", "commands")
	legacyCommand := filepath.Join(commandsDir, "h-frame.toml")

	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyCommand, []byte("description = \"legacy haft command\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	initClaude = false
	initCursor = false
	initGemini = true
	initCodex = false
	initAir = false
	initOpencode = false
	initAll = false
	initLocal = false
	initNoClaudeMD = true

	if err := runInit(nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyCommand); err != nil {
		t.Fatalf("Gemini init removed legacy command without replacement: %v", err)
	}
}

func TestCreateDirectoryStructure_CreatesWorkflowExample(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(haftDir, "workflow.md"))
	if err != nil {
		t.Fatalf("read workflow example: %v", err)
	}

	if !strings.Contains(string(data), "## Defaults") {
		t.Fatalf("workflow example missing Defaults section:\n%s", string(data))
	}
}

func TestCreateDirectoryStructure_CreatesOnboardingSpecCarriers(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure returned error: %v", err)
	}

	for _, carrier := range project.MinimumSpecCarriers() {
		path := filepath.Join(haftDir, carrier.RelativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read onboarding carrier %s: %v", path, err)
		}
		if string(data) != carrier.Content {
			t.Fatalf("onboarding carrier %s content mismatch:\n%s", path, string(data))
		}
	}
}

func TestCreateDirectoryStructure_OnboardingSpecCarriersAreIdempotent(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("first createDirectoryStructure returned error: %v", err)
	}
	first := readSpecCarrierContents(t, haftDir)

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("second createDirectoryStructure returned error: %v", err)
	}
	second := readSpecCarrierContents(t, haftDir)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("spec carriers changed on second init:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestCreateDirectoryStructure_CreatesDefaultMethodCatalogIdempotently(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("first createDirectoryStructure returned error: %v", err)
	}

	methodDir := filepath.Join(haftDir, "methods", method.CatalogID)
	manifestPath := filepath.Join(methodDir, "manifest.yaml")
	firstManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read method manifest: %v", err)
	}
	if !strings.Contains(string(firstManifest), "catalog_id: swe-core") {
		t.Fatalf("method manifest missing swe-core catalog:\n%s", string(firstManifest))
	}

	for _, definition := range method.BuiltinCatalog().Methods {
		path := filepath.Join(methodDir, definition.ID+".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read method file %s: %v", path, err)
		}
		if !strings.Contains(string(data), "id: "+definition.ID) {
			t.Fatalf("method file %s missing id:\n%s", path, string(data))
		}
	}

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("second createDirectoryStructure returned error: %v", err)
	}
	secondManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read method manifest after second run: %v", err)
	}
	if string(firstManifest) != string(secondManifest) {
		t.Fatalf("method manifest changed on second init:\nfirst:\n%s\nsecond:\n%s", firstManifest, secondManifest)
	}
}

func TestCreateDirectoryStructure_DoesNotOverwriteExistingSpecCarriers(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")
	customContents := map[string]string{
		filepath.Join("specs", "target-system.md"):   "# Existing Target Spec\n\nHuman-authored content.\n",
		filepath.Join("specs", "enabling-system.md"): "# Existing Enabling Spec\n\nHuman-authored content.\n",
		filepath.Join("specs", "term-map.md"):        "# Existing Term Map\n\nHuman-authored content.\n",
	}
	for relativePath, content := range customContents {
		path := filepath.Join(haftDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure returned error: %v", err)
	}

	for relativePath, content := range customContents {
		path := filepath.Join(haftDir, relativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read spec carrier %s: %v", path, err)
		}
		if string(data) != content {
			t.Fatalf("spec carrier %s overwritten:\n%s", path, string(data))
		}
	}
}

func TestCreateDirectoryStructure_CreatesParseableDraftPlaceholders(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure returned error: %v", err)
	}

	target, err := os.ReadFile(filepath.Join(haftDir, "specs", "target-system.md"))
	if err != nil {
		t.Fatalf("read target carrier: %v", err)
	}
	enabling, err := os.ReadFile(filepath.Join(haftDir, "specs", "enabling-system.md"))
	if err != nil {
		t.Fatalf("read enabling carrier: %v", err)
	}
	termMap, err := os.ReadFile(filepath.Join(haftDir, "specs", "term-map.md"))
	if err != nil {
		t.Fatalf("read term-map carrier: %v", err)
	}

	for name, content := range map[string]string{
		"target":   string(target),
		"enabling": string(enabling),
	} {
		parsed := parseFirstYAMLBlock(t, content)
		if !strings.Contains(content, "```yaml spec-section") {
			t.Fatalf("%s placeholder missing spec-section fence:\n%s", name, content)
		}
		if parsed["status"] != "draft" {
			t.Fatalf("%s placeholder status = %#v, want draft", name, parsed["status"])
		}
		if parsed["claim_layer"] != "carrier" {
			t.Fatalf("%s placeholder claim_layer = %#v, want carrier", name, parsed["claim_layer"])
		}
	}
	if !strings.Contains(string(termMap), "```yaml term-map") {
		t.Fatalf("term-map placeholder missing parseable fence:\n%s", string(termMap))
	}
	parsedTermMap := parseFirstYAMLBlock(t, string(termMap))
	if parsedTermMap["status"] != "draft" {
		t.Fatalf("term-map placeholder status = %#v, want draft", parsedTermMap["status"])
	}
	if entries, ok := parsedTermMap["entries"].([]any); !ok || len(entries) != 0 {
		t.Fatalf("term-map placeholder entries = %#v, want empty list", parsedTermMap["entries"])
	}
}

func readSpecCarrierContents(t *testing.T, haftDir string) map[string]string {
	t.Helper()

	contents := make(map[string]string)
	for _, carrier := range project.MinimumSpecCarriers() {
		path := filepath.Join(haftDir, carrier.RelativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read spec carrier %s: %v", path, err)
		}
		contents[carrier.RelativePath] = string(data)
	}

	return contents
}

func parseFirstYAMLBlock(t *testing.T, content string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inBlock := false
	var block strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```yaml") {
			inBlock = true
			continue
		}
		if inBlock && strings.HasPrefix(trimmed, "```") {
			break
		}
		if inBlock {
			block.WriteString(line)
			block.WriteString("\n")
		}
	}

	payload := strings.TrimSpace(block.String())
	if payload == "" {
		t.Fatalf("YAML block missing in:\n%s", content)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("parse YAML block: %v\n%s", err, payload)
	}

	return parsed
}

func TestRunInit_OpencodeWritesMcpConfigAndCommands(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := configureMCPOpencode(tmpDir, "haft"); err != nil {
		t.Fatalf("configureMCPOpencode: %v", err)
	}

	configPath := filepath.Join(tmpDir, "opencode.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("opencode.json not created: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("opencode.json invalid JSON: %v", err)
	}

	if got := config["$schema"]; got != "https://opencode.ai/config.json" {
		t.Errorf("$schema = %v, want https://opencode.ai/config.json", got)
	}

	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp section missing or wrong type: %v", config["mcp"])
	}
	haft, ok := mcp["haft"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.haft missing or wrong type: %v", mcp["haft"])
	}
	if haft["type"] != "local" {
		t.Errorf("mcp.haft.type = %v, want local", haft["type"])
	}
	if got := haft["command"]; got == nil {
		t.Errorf("mcp.haft.command missing")
	}
	if haft["enabled"] != true {
		t.Errorf("mcp.haft.enabled = %v, want true", haft["enabled"])
	}
	env, ok := haft["environment"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.haft.environment missing: %v", haft["environment"])
	}
	if env["HAFT_PROJECT_ROOT"] != tmpDir {
		t.Errorf("HAFT_PROJECT_ROOT = %v, want %s", env["HAFT_PROJECT_ROOT"], tmpDir)
	}

	// Skill install for opencode lands in .opencode/skills (root); each
	// v8 governance-substrate skill lands as <root>/<name>/SKILL.md.
	skillPath, count, err := installSkill("opencode", true, tmpDir)
	if err != nil {
		t.Fatalf("installSkill opencode: %v", err)
	}
	if count != len(allSkills) {
		t.Errorf("installSkill installed %d skills, expected %d", count, len(allSkills))
	}
	wantSkillRoot := filepath.Join(tmpDir, ".opencode", "skills")
	if skillPath != wantSkillRoot {
		t.Errorf("skillPath = %q, want %q", skillPath, wantSkillRoot)
	}
	// h-reason (umbrella entry point, carries full FPF reasoning palette)
	// must land.
	if _, err := os.Stat(filepath.Join(wantSkillRoot, "h-reason", "SKILL.md")); err != nil {
		t.Errorf("h-reason SKILL.md not installed: %v", err)
	}
	// h-decide (manual-only, Transformer Mandate) must land.
	if _, err := os.Stat(filepath.Join(wantSkillRoot, "h-decide", "SKILL.md")); err != nil {
		t.Errorf("h-decide SKILL.md not installed: %v", err)
	}
	// Deprecated h-fpf must be removed by deprecation cleanup (renamed
	// to h-reason and expanded into the full umbrella).
	if _, err := os.Stat(filepath.Join(wantSkillRoot, "h-fpf")); !os.IsNotExist(err) {
		t.Errorf("h-fpf should be removed; got err=%v", err)
	}
}

func TestConfigureMCPOpencode_PreservesExistingKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	existing := map[string]any{
		"$schema":  "https://opencode.ai/config.json",
		"theme":    "tokyonight",
		"username": "test-user",
		"mcp": map[string]any{
			"some-other-server": map[string]any{
				"type":    "local",
				"command": []any{"echo", "hi"},
			},
			"quint-code": map[string]any{
				"type":    "local",
				"command": []any{"haft", "serve"},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := configureMCPOpencode(tmpDir, "haft"); err != nil {
		t.Fatal(err)
	}

	rewritten, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(rewritten, &config); err != nil {
		t.Fatal(err)
	}

	if config["theme"] != "tokyonight" {
		t.Errorf("theme key clobbered: %v", config["theme"])
	}
	if config["username"] != "test-user" {
		t.Errorf("username key clobbered: %v", config["username"])
	}
	mcp := config["mcp"].(map[string]any)
	if _, ok := mcp["some-other-server"]; !ok {
		t.Errorf("other MCP server clobbered")
	}
	if _, ok := mcp["quint-code"]; ok {
		t.Errorf("legacy quint-code key not removed")
	}
	if _, ok := mcp["haft"]; !ok {
		t.Errorf("haft key not added")
	}
}
