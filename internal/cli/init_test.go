package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
