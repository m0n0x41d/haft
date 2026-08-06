package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditProjectPathCarriersCountsExactSupportedReferences(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	projectRoot := filepath.Join(root, "new", "haft")
	oldRoot := filepath.Join(root, "old", "haft")

	writePathCarrierFixture(t, filepath.Join(home, ".codex", "config.toml"), strings.Join([]string{
		codexProjectTableHeader(oldRoot),
		`trust_level = "trusted"`,
		"",
	}, "\n"))
	writePathCarrierFixture(t, filepath.Join(home, ".claude.json"), `{"projects":{"`+oldRoot+`":{},"`+oldRoot+`/.claude/worktrees/tmp":{}}}`)
	writePathCarrierFixture(t, filepath.Join(projectRoot, ".mcp.json"), `{"env":{"HAFT_PROJECT_ROOT":"`+oldRoot+`"}}`)

	carriers := projectPathCarrierCandidates(home, projectRoot)
	results, err := auditProjectPathCarriers(carriers, oldRoot)
	if err != nil {
		t.Fatalf("auditProjectPathCarriers: %v", err)
	}

	occurrences := pathCarrierOccurrences(results)
	if occurrences["Codex user project trust"] != 1 {
		t.Fatalf("codex occurrences = %d, want 1", occurrences["Codex user project trust"])
	}
	if occurrences["Claude user project state"] != 1 {
		t.Fatalf("claude exact JSON occurrences = %d, want 1", occurrences["Claude user project state"])
	}
	if occurrences["Claude project MCP"] != 1 {
		t.Fatalf("project mcp occurrences = %d, want 1", occurrences["Claude project MCP"])
	}
}

func TestRepairProjectPathCarriersRemovesStaleCodexSectionWhenCurrentRootExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	oldRoot := filepath.Join(root, "old", "haft")
	newRoot := filepath.Join(root, "new", "haft")
	content := strings.Join([]string{
		`model = "gpt-5.4"`,
		"",
		codexProjectTableHeader(oldRoot),
		`trust_level = "trusted"`,
		"",
		codexProjectTableHeader(newRoot),
		`trust_level = "trusted"`,
		"",
		`[plugins."github@openai-curated"]`,
		`enabled = true`,
		"",
	}, "\n")
	writePathCarrierFixture(t, configPath, content)

	changed, message, err := repairCodexProjectRoot(configPath, oldRoot, newRoot)
	if err != nil {
		t.Fatalf("repairCodexProjectRoot: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, message=%q", message)
	}

	data := readPathCarrierFixture(t, configPath)
	if strings.Contains(data, codexProjectTableHeader(oldRoot)) {
		t.Fatalf("stale codex section remains:\n%s", data)
	}
	if strings.Count(data, codexProjectTableHeader(newRoot)) != 1 {
		t.Fatalf("current codex section count wrong:\n%s", data)
	}
	if !strings.Contains(data, `[plugins."github@openai-curated"]`) {
		t.Fatalf("section after stale project was lost:\n%s", data)
	}
}

func TestRepairProjectPathCarriersRenamesCodexSectionWhenCurrentRootMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	oldRoot := filepath.Join(root, "old", "haft")
	newRoot := filepath.Join(root, "new", "haft")
	content := strings.Join([]string{
		codexProjectTableHeader(oldRoot),
		`trust_level = "trusted"`,
		"",
	}, "\n")
	writePathCarrierFixture(t, configPath, content)

	changed, _, err := repairCodexProjectRoot(configPath, oldRoot, newRoot)
	if err != nil {
		t.Fatalf("repairCodexProjectRoot: %v", err)
	}
	if !changed {
		t.Fatal("changed = false")
	}

	data := readPathCarrierFixture(t, configPath)
	if strings.Contains(data, codexProjectTableHeader(oldRoot)) {
		t.Fatalf("old section remains:\n%s", data)
	}
	if !strings.Contains(data, codexProjectTableHeader(newRoot)) {
		t.Fatalf("new section missing:\n%s", data)
	}
}

func TestRepairJSONStringProjectRootOnlyRewritesExactStringLiteral(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	statePath := filepath.Join(root, ".claude.json")
	oldRoot := filepath.Join(root, "old", "haft")
	newRoot := filepath.Join(root, "new", "haft")
	worktree := oldRoot + "/.claude/worktrees/tmp"
	content := `{"repos":{"m0n0x41d/haft":["` + oldRoot + `","` + worktree + `"]},"projects":{"` + oldRoot + `":{"allowedTools":[]}}}`
	writePathCarrierFixture(t, statePath, content)

	changed, _, err := repairJSONStringProjectRoot(statePath, oldRoot, newRoot)
	if err != nil {
		t.Fatalf("repairJSONStringProjectRoot: %v", err)
	}
	if !changed {
		t.Fatal("changed = false")
	}

	data := readPathCarrierFixture(t, statePath)
	if strings.Contains(data, `"`+oldRoot+`"`) {
		t.Fatalf("exact old root remains:\n%s", data)
	}
	if !strings.Contains(data, `"`+newRoot+`"`) {
		t.Fatalf("new exact root missing:\n%s", data)
	}
	if !strings.Contains(data, `"`+worktree+`"`) {
		t.Fatalf("prefixed worktree path should not be rewritten:\n%s", data)
	}
}

func writePathCarrierFixture(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func readPathCarrierFixture(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func pathCarrierOccurrences(results []projectPathCarrierResult) map[string]int {
	occurrences := make(map[string]int, len(results))
	for _, result := range results {
		occurrences[result.Label] = result.Occurrences
	}
	return occurrences
}
