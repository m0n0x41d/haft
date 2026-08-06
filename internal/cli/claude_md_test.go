package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testHaftSectionStart = "<!-- haft:start -->"
	testHaftSectionEnd   = "<!-- haft:end -->"
)

// TestClaudeMDTemplateInSyncWithRepoRoot guards against drift between the
// embedded template and the showcase copy in repo-root CLAUDE.md. Both must
// stay identical so that haft maintainers reading repo-root CLAUDE.md see
// the same content end-users get from `haft init`.
func TestClaudeMDTemplateInSyncWithRepoRoot(t *testing.T) {
	t.Parallel()

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Skipf("skipping sync check outside repo: %v", err)
	}
	rootData, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read repo CLAUDE.md: %v", err)
	}
	rootContent := string(rootData)

	startIdx := strings.Index(rootContent, testHaftSectionStart)
	if startIdx < 0 {
		t.Fatal("repo CLAUDE.md missing haft:start marker")
	}
	bodyStart := startIdx + len(testHaftSectionStart)
	rel := strings.Index(rootContent[bodyStart:], testHaftSectionEnd)
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

func TestInstructionCarriersExposeSourceFirstFPFContract(t *testing.T) {
	t.Parallel()

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
			"Current question first",
			`action="fpf"`,
			`mode="concern"`,
			`mode="lookup"`,
			`mode="inspect"`,
			"README practical-use cards",
			"Table of Contents",
			"full pattern body is the authority",
			"not phases",
			"Materialize only the records that receiving use needs",
			"context-heavy, multi-session, or reliance-bearing",
			`projection_profile_ref="agent_orientation.v2"`,
			"`known_absent` is an identity result, not permission to persist",
			`haft_entity(action="establish", ...)`,
			`haft_onboard(action="status")`,
			"Never persist merely because memory is empty or a read failed",
			"operator-named or agent-inferred",
			"preserve its exact request provenance",
			"FPF is relation-first at the framework-navigation level",
			"Status signals are attention, not authorization gates",
			"Never ask for bare `OK`, `yes`, or `да`",
			"Unrelated already-authorized Work continues",
			"Description is not Work",
		} {
			assertContains(t, carrier, content, want)
		}
		for _, banned := range []string{
			"should_use_pattern",
			"suggested_haft_surface",
			"recommended_pattern_use",
			"required_next_action",
			"matched_route_id",
			"Naming/terminology requests should route",
			"Architecture requests should route",
			"SoTA/current-practice requests should route",
			"haft memory typeenv",
			"Never persist automatically",
		} {
			if strings.Contains(content, banned) {
				t.Fatalf("%s contains retired router fragment %q", carrier, banned)
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
