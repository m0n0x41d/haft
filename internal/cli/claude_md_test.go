package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallClaudeMD_CreatesWhenAbsent(t *testing.T) {
	dir := t.TempDir()

	path, action, err := installClaudeMD(dir)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if action != claudeMDCreated {
		t.Errorf("action = %q, want %q", action, claudeMDCreated)
	}
	if path != filepath.Join(dir, "CLAUDE.md") {
		t.Errorf("path = %q", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, haftSectionStart) {
		t.Error("missing start marker")
	}
	if !strings.Contains(content, haftSectionEnd) {
		t.Error("missing end marker")
	}
	if !strings.Contains(content, "Description ≠ Work") {
		t.Error("template content not present")
	}
}

func TestInstallClaudeMD_ReplacesBetweenMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")

	userPrefix := "# My Project\n\nSome project-specific rules.\n\n"
	userSuffix := "\n\n## Local conventions\n\nWe use tabs.\n"
	staleHaft := haftSectionStart + "\nOLD STALE haft content that should be replaced\n" + haftSectionEnd
	initial := userPrefix + staleHaft + userSuffix

	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	_, action, err := installClaudeMD(dir)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if action != claudeMDUpdated {
		t.Errorf("action = %q, want %q", action, claudeMDUpdated)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.HasPrefix(content, "# My Project") {
		t.Error("user prefix lost")
	}
	if !strings.Contains(content, "## Local conventions") {
		t.Error("user suffix lost")
	}
	if !strings.Contains(content, "We use tabs.") {
		t.Error("user content after markers lost")
	}
	if strings.Contains(content, "OLD STALE") {
		t.Error("stale haft content was not replaced")
	}
	if !strings.Contains(content, "Description ≠ Work") {
		t.Error("new template content not installed")
	}
}

func TestInstallClaudeMD_AppendsWhenNoMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	initial := "# My Project\n\nNo haft markers yet.\n"

	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	_, action, err := installClaudeMD(dir)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if action != claudeMDAppended {
		t.Errorf("action = %q, want %q", action, claudeMDAppended)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.HasPrefix(content, "# My Project") {
		t.Error("existing prefix lost")
	}
	if !strings.Contains(content, haftSectionStart) {
		t.Error("haft section not appended")
	}
	if !strings.Contains(content, haftSectionEnd) {
		t.Error("haft end marker not appended")
	}
}

func TestInstallClaudeMD_IdempotentWhenUnchanged(t *testing.T) {
	dir := t.TempDir()

	if _, _, err := installClaudeMD(dir); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	_, action, err := installClaudeMD(dir)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if action != claudeMDUnchanged {
		t.Errorf("second action = %q, want %q (idempotent)", action, claudeMDUnchanged)
	}
}

// TestClaudeMDTemplateInSyncWithRepoRoot guards against drift between the
// embedded template and the showcase copy in repo-root CLAUDE.md. Both must
// stay identical so that haft maintainers reading repo-root CLAUDE.md see
// the same content end-users get from `haft init`.
func TestClaudeMDTemplateInSyncWithRepoRoot(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Skipf("skipping sync check outside repo: %v", err)
	}
	rootData, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read repo CLAUDE.md: %v", err)
	}
	rootContent := string(rootData)

	startIdx := strings.Index(rootContent, haftSectionStart)
	if startIdx < 0 {
		t.Fatal("repo CLAUDE.md missing haft:start marker")
	}
	bodyStart := startIdx + len(haftSectionStart)
	rel := strings.Index(rootContent[bodyStart:], haftSectionEnd)
	if rel < 0 {
		t.Fatal("repo CLAUDE.md missing haft:end marker")
	}
	repoSection := strings.TrimSpace(rootContent[bodyStart : bodyStart+rel])
	embedded := strings.TrimSpace(embeddedClaudeMDTemplate)

	if repoSection != embedded {
		t.Errorf("repo-root CLAUDE.md haft section drifted from internal/cli/claude_md_template.md.\n"+
			"Lengths: repo=%d embedded=%d. Sync them: copy template into the haft section of repo CLAUDE.md, or vice versa.",
			len(repoSection), len(embedded))
	}
}

func TestInstructionCarriersPreservePeerEngineeringStyle(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Skipf("skipping sync check outside repo: %v", err)
	}

	claudeData, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read repo CLAUDE.md: %v", err)
	}
	agentsData, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read repo AGENTS.md: %v", err)
	}

	claudeContent := string(claudeData)
	agentsContent := string(agentsData)

	assertContains(t, "embedded template", embeddedClaudeMDTemplate, "Be a peer engineer, not a cheerleader")
	assertContains(t, "embedded template", embeddedClaudeMDTemplate, "Use dry, technical humor when appropriate")
	assertContains(t, "embedded template", embeddedClaudeMDTemplate, "Talk like you're pairing with a staff engineer, not pitching to a VP")
	assertContains(t, "repo CLAUDE.md", claudeContent, "Be a peer engineer, not a cheerleader")
	assertContains(t, "repo CLAUDE.md", claudeContent, "Use dry, technical humor when appropriate")
	assertContains(t, "repo CLAUDE.md", claudeContent, "Talk like you're pairing with a staff engineer, not pitching to a VP")
	assertContains(t, "repo AGENTS.md", agentsContent, "Be a peer engineer, not a cheerleader")
	assertContains(t, "repo AGENTS.md", agentsContent, "Use dry, technical humor when appropriate")
	assertContains(t, "repo AGENTS.md", agentsContent, "Talk like you're pairing with a staff engineer, not pitching to a VP")
	assertContains(t, "embedded template", embeddedClaudeMDTemplate, "unpersisted reasoning, not durable project governance, evidence, or authority")
	assertContains(t, "repo CLAUDE.md", claudeContent, "unpersisted reasoning, not durable project governance, evidence, or authority")
	assertContains(t, "repo AGENTS.md", agentsContent, "unpersisted reasoning, not durable project governance, evidence, or authority")
	assertContains(t, "embedded template", embeddedClaudeMDTemplate, "Derive changed files, commands, test output")
	assertContains(t, "repo CLAUDE.md", claudeContent, "Derive changed files, commands, test output")
	assertContains(t, "repo AGENTS.md", agentsContent, "Derive changed files, commands, test output")
}

func TestInstructionCarriersExposePatternUseGatewayWithoutCatalog(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Skipf("skipping sync check outside repo: %v", err)
	}

	claudeData, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read repo CLAUDE.md: %v", err)
	}
	agentsData, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read repo AGENTS.md: %v", err)
	}

	for carrier, content := range map[string]string{
		"embedded template": embeddedClaudeMDTemplate,
		"repo CLAUDE.md":    string(claudeData),
		"repo AGENTS.md":    string(agentsData),
	} {
		for _, want := range []string{
			"PatternUse Gateway",
			`action="pattern_use"`,
			`mode="compact"`,
			`mode="full"`,
			"should_use_pattern",
			"Do not inline the FPF catalog",
			"PatternUse is advisory and read-only",
			"not approval",
			"not MethodPack",
		} {
			assertContains(t, carrier, content, want)
		}
		for _, banned := range []string{
			"Naming/terminology requests should route",
			"Architecture requests should route",
			"SoTA/current-practice requests should route",
		} {
			if strings.Contains(content, banned) {
				t.Fatalf("%s inlines stale PatternUse catalog cue %q", carrier, banned)
			}
		}
	}
}

func assertContains(t *testing.T, carrier string, content string, fragment string) {
	t.Helper()
	if !strings.Contains(content, fragment) {
		t.Fatalf("%s missing %q", carrier, fragment)
	}
}

// findRepoRoot walks up from cwd looking for a go.mod file. Returns the
// directory containing it, or an error if none found.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func TestInstallClaudeMD_PreservesUserEditsOutsideMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")

	// First install to seed markers.
	if _, _, err := installClaudeMD(dir); err != nil {
		t.Fatalf("seed install failed: %v", err)
	}

	// User adds content before and after the haft section.
	initial, _ := os.ReadFile(path)
	userContent := "# My Custom Rules\n\nUse 4 spaces.\n\n"
	suffix := "\n## My Other Rules\n\nNo trailing whitespace.\n"
	edited := userContent + string(initial) + suffix
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	// Re-install — must preserve user content.
	if _, _, err := installClaudeMD(dir); err != nil {
		t.Fatalf("re-install failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "Use 4 spaces.") {
		t.Error("user content before markers lost")
	}
	if !strings.Contains(content, "No trailing whitespace.") {
		t.Error("user content after markers lost")
	}
}
