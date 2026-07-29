package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestApplyReadinessReminder_AppendsOnNeedsOnboard(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	result := applyProfileAwareReadinessReminder(
		context.Background(),
		"ProblemCard framed: ...",
		"haft_problem",
		haftDir,
		map[string]any{},
	)

	if !strings.Contains(result, "Project readiness") {
		t.Fatalf("expected readiness reminder appended; got %q", result)
	}
	if !strings.Contains(result, "needs_onboard") {
		t.Fatalf("reminder should explain needs_onboard state; got %q", result)
	}
	if !strings.Contains(result, "/h-spec") {
		t.Fatalf("reminder should point at /h-spec; got %q", result)
	}
	if !strings.Contains(result, "/h-onboard") {
		t.Fatalf("reminder should preserve /h-onboard compatibility; got %q", result)
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
		result := applyProfileAwareReadinessReminder(
			context.Background(),
			"payload",
			tool,
			haftDir,
			map[string]any{},
		)
		if strings.Contains(result, "Project readiness") {
			t.Fatalf("tool %q should not receive readiness reminder; got %q", tool, result)
		}
	}
}

func TestApplyReadinessReminder_SkipsMachineJSONResponse(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	jsonResult := `{"id":"prob-20260428-abc","title":"x"}`
	result := applyProfileAwareReadinessReminder(
		context.Background(),
		jsonResult,
		"haft_problem",
		haftDir,
		map[string]any{},
	)

	if result != jsonResult {
		t.Fatalf("JSON response must not be polluted; got %q", result)
	}
}

func TestApplyReadinessReminder_SkipsReadyProject(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectReady)
	haftDir := filepath.Join(root, ".haft")
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if readiness.facts.Status != project.ReadinessReady {
		applicability, _ := readiness.resolvedApplicability()
		report, reportErr := project.CheckSpecificationSetForScope(
			root,
			applicability,
		)
		t.Fatalf(
			"ready fixture status = %q; report_error=%v; report=%#v",
			readiness.facts.Status,
			reportErr,
			report,
		)
	}

	result := applyProfileAwareReadinessReminder(
		context.Background(),
		"payload",
		"haft_decision",
		haftDir,
		map[string]any{},
	)
	if strings.Contains(result, "Project readiness") {
		t.Fatalf("ready project should not receive reminder; got %q", result)
	}
}

func TestApplyReadinessReminder_SkipsNeedsInitProject(t *testing.T) {
	root := t.TempDir() // no .haft at all → needs_init
	haftDir := filepath.Join(root, ".haft")

	result := applyProfileAwareReadinessReminder(
		context.Background(),
		"payload",
		"haft_problem",
		haftDir,
		map[string]any{},
	)
	if strings.Contains(result, "Project readiness") {
		t.Fatalf("needs_init project should not receive needs_onboard reminder; got %q", result)
	}
}

func TestApplyReadinessReminder_AppliesToAllReasoningTools(t *testing.T) {
	root := newReadinessTestProject(t, readinessTestProjectInit)
	haftDir := filepath.Join(root, ".haft")

	for _, tool := range []string{"haft_problem", "haft_solution", "haft_decision", "haft_note"} {
		result := applyProfileAwareReadinessReminder(
			context.Background(),
			"payload",
			tool,
			haftDir,
			map[string]any{},
		)
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
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevisionWithTargetEntity(
		t,
		"readiness-reminder",
		"entity:readiness-reminder-target",
	)
	root = harness.Root().String()
	haftDir := filepath.Join(root, ".haft")

	if mode == readinessTestProjectReady {
		writeReadinessReadyFixture(t, haftDir)
	}

	return root
}

// writeReadinessReadyFixture lays down the minimum carriers
// `project.hasMinimumSpecificationSet` checks: workflow.md with the
// "## Defaults" marker, the required target and software sections,
// and one term-map entry that pass `CheckSpecificationSet` cleanly.
func writeReadinessReadyFixture(t *testing.T, haftDir string) {
	t.Helper()

	specsDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	targetCarrier := readinessReadyTargetCarrier()
	softwareCarrier := readinessReadySoftwareCarrier()
	files := map[string]string{
		filepath.Join(haftDir, "workflow.md"):         "# workflow\n\n## Defaults\n\nmode: standard\n",
		filepath.Join(specsDir, "target-system.md"):   targetCarrier,
		filepath.Join(specsDir, "software-system.md"): softwareCarrier,
		filepath.Join(specsDir, "term-map.md"): "```yaml\n" +
			"term: TestProject\n" +
			"category: enabling\n" +
			"definition: A project under readiness test fixture.\n" +
			"```\n",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func readinessReadyTargetCarrier() string {
	sections := []string{
		readinessReadySection("TS.environment.001", "target-system", "target.environment", "Test target environment"),
		readinessReadySection("TS.role.001", "target-system", "target.role", "Test target role"),
		readinessReadySection("TS.boundary.001", "target-system", "target.boundary", "Test target boundary"),
	}

	return strings.Join(sections, "")
}

func readinessReadySoftwareCarrier() string {
	sections := []string{
		readinessReadySection("SS.role.001", "software-system", "software.role", "Test software role"),
		readinessReadySection("SS.functional.001", "software-system", "software.functional_behavior", "Test software behavior"),
		readinessReadySection("SS.interfaces.001", "software-system", "software.interfaces", "Test software interfaces"),
		readinessReadySection("SS.constraints.001", "software-system", "software.constraints", "Test software constraints"),
	}

	return strings.Join(sections, "")
}

func readinessReadySection(id string, spec string, kind string, title string) string {
	lines := []string{
		"## " + id,
		"",
		"```yaml spec-section",
		"id: " + id,
		"kind: " + kind,
		"statement_type: definition",
		"claim_layer: object",
		"owner: human",
		"status: active",
		"```",
		"",
	}

	return strings.Join(lines, "\n")
}
