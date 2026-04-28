package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestApplyReadinessReminder_AppendsOnNeedsOnboard(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	result := applyReadinessReminder("ProblemCard framed: ...", "haft_problem", haftDir)

	if !strings.Contains(result, "Project readiness") {
		t.Fatalf("expected readiness reminder appended; got %q", result)
	}
	if !strings.Contains(result, "needs_onboard") {
		t.Fatalf("reminder should explain needs_onboard state; got %q", result)
	}
	if !strings.Contains(result, "/h-onboard") {
		t.Fatalf("reminder should point at /h-onboard; got %q", result)
	}
	// Original result is preserved in front.
	if !strings.HasPrefix(result, "ProblemCard framed: ...") {
		t.Fatalf("reminder should append, not replace; got %q", result)
	}
}

func TestApplyReadinessReminder_SkipsToolsNotInReasoningLoop(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	for _, tool := range []string{"haft_query", "haft_refresh", "haft_commission", "haft_spec_section"} {
		result := applyReadinessReminder("payload", tool, haftDir)
		if strings.Contains(result, "Project readiness") {
			t.Fatalf("tool %q should not receive readiness reminder; got %q", tool, result)
		}
	}
}

func TestApplyReadinessReminder_SkipsMachineJSONResponse(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	jsonResult := `{"id":"prob-20260428-abc","title":"x"}`
	result := applyReadinessReminder(jsonResult, "haft_problem", haftDir)

	if result != jsonResult {
		t.Fatalf("JSON response must not be polluted; got %q", result)
	}
}

func TestApplyReadinessReminder_SkipsReadyProject(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectReady)
	haftDir := filepath.Join(root, ".haft")

	result := applyReadinessReminder("payload", "haft_decision", haftDir)
	if strings.Contains(result, "Project readiness") {
		t.Fatalf("ready project should not receive reminder; got %q", result)
	}
}

func TestApplyReadinessReminder_SkipsNeedsInitProject(t *testing.T) {
	root := t.TempDir() // no .haft at all → needs_init
	haftDir := filepath.Join(root, ".haft")

	result := applyReadinessReminder("payload", "haft_problem", haftDir)
	if strings.Contains(result, "Project readiness") {
		t.Fatalf("needs_init project should not receive needs_onboard reminder; got %q", result)
	}
}

func TestApplyReadinessReminder_AppliesToAllReasoningTools(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	for _, tool := range []string{"haft_problem", "haft_solution", "haft_decision", "haft_note"} {
		result := applyReadinessReminder("payload", tool, haftDir)
		if !strings.Contains(result, "Project readiness") {
			t.Fatalf("tool %q should receive readiness reminder; got %q", tool, result)
		}
	}
}

type readinessTestProjectMode int

const (
	readinessTestProjectInit readinessTestProjectMode = iota
	readinessTestProjectReady
)

// newReadinessTestProject creates a temp directory whose ReadinessFacts.Status
// matches the requested mode:
//   - readinessTestProjectInit: project.yaml present, no workflow.md → needs_onboard.
//   - readinessTestProjectReady: project.yaml + workflow.md with "## Defaults" → ready.
//
// Fixture mirrors what `haft init` produces, just trimmed to the bytes
// readiness inspection actually checks.
func newReadinessTestProject(t *testing.T, mode readinessTestProjectMode) string {
	t.Helper()

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := project.Create(haftDir, root); err != nil {
		t.Fatalf("project.Create: %v", err)
	}

	if mode == readinessTestProjectReady {
		writeReadinessReadyFixture(t, haftDir)
	}

	return root
}

// writeReadinessReadyFixture lays down the minimum carriers
// `project.hasMinimumSpecificationSet` checks: workflow.md with the
// "## Defaults" marker, plus one active target-system section, one
// active enabling-system section, and one term-map entry that pass
// `CheckSpecificationSet` cleanly.
func writeReadinessReadyFixture(t *testing.T, haftDir string) {
	t.Helper()

	specsDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		filepath.Join(haftDir, "workflow.md"): "# workflow\n\n## Defaults\n\nmode: standard\n",
		filepath.Join(specsDir, "target-system.md"): "## TS.environment.001\n\n" +
			"```yaml spec-section\n" +
			"id: TS.environment.001\n" +
			"spec: target-system\n" +
			"kind: environment-change\n" +
			"title: Test environment change\n" +
			"statement_type: definition\n" +
			"claim_layer: object\n" +
			"owner: human\n" +
			"status: active\n" +
			"valid_until: 2099-12-31\n" +
			"```\n",
		filepath.Join(specsDir, "enabling-system.md"): "## ES.creator.001\n\n" +
			"```yaml spec-section\n" +
			"id: ES.creator.001\n" +
			"spec: enabling-system\n" +
			"kind: creator-role\n" +
			"title: Test creator role\n" +
			"statement_type: explanation\n" +
			"claim_layer: carrier\n" +
			"owner: human\n" +
			"status: active\n" +
			"valid_until: 2099-12-31\n" +
			"```\n",
		filepath.Join(specsDir, "term-map.md"): "```yaml term-map\n" +
			"entries:\n" +
			"  - term: TestProject\n" +
			"    domain: target\n" +
			"    definition: A project under readiness test fixture.\n" +
			"```\n",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
