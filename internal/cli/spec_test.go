package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestRunSpecCheckCommandSmokeCleanProject(t *testing.T) {
	root := newSpecCheckCLIProject(t, validCLITermMapCarrier())
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCheckJSON(t, false)
	defer restoreJSON()

	exitCode := stubSpecCheckExit(t)

	err := runSpecCheck(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCheck returned error: %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", *exitCode)
	}
	if !strings.Contains(output.String(), "haft spec check: clean (L0/L1/L1.5)") {
		t.Fatalf("output = %q, want clean summary", output.String())
	}
}

func TestRunSpecCheckJSONExitsOneOnFindings(t *testing.T) {
	root := newSpecCheckCLIProject(t, invalidCLITermMapCarrier())
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCheckJSON(t, true)
	defer restoreJSON()

	exitCode := stubSpecCheckExit(t)

	err := runSpecCheck(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCheck returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}

	var report project.SpecCheckReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if report.Summary.TotalFindings == 0 {
		t.Fatalf("total_findings = 0, want findings")
	}
	if report.Level != "L0/L1/L1.5" {
		t.Fatalf("level = %q, want L0/L1/L1.5", report.Level)
	}
	if report.Findings[0].FieldPath == "" {
		t.Fatalf("first finding field_path is empty: %+v", report.Findings[0])
	}
}

func TestRunSpecCoverageJSONReportsDerivedSectionStates(t *testing.T) {
	fixture := newCheckTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)

	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Cover checkout spec",
		WhySelected:     "The checkout section needs one governing decision for coverage derivation.",
		SelectionPolicy: "Prefer direct section linkage over manual status fields.",
		CounterArgument: "A fixture could overfit to one section id.",
		WeakestLink:     "Coverage depends on explicit section refs.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual coverage status",
			Reason:  "Manual status would violate the derived coverage contract.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Coverage no longer derives from section refs."},
		},
		AffectedFiles: []string{"internal/checkout/flow.go"},
	})
	if err := fixture.store.AddLink(context.Background(), decision.Meta.ID, "TS.covered.001", "governs"); err != nil {
		t.Fatalf("link decision to spec section: %v", err)
	}
	_, err := artifact.AttachEvidence(context.Background(), fixture.store, artifact.EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "Checkout spec measurement passed.",
		Type:            "measurement",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  2,
	})
	if err != nil {
		t.Fatalf("attach evidence: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()

	err = runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	var report project.SpecCoverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	states := map[string]project.SpecCoverageState{}
	for _, section := range report.Sections {
		states[section.SectionID] = section.State
		if len(section.Why) == 0 {
			t.Fatalf("section %s has empty why", section.SectionID)
		}
		if section.NextAction == "" {
			t.Fatalf("section %s has empty next_action", section.SectionID)
		}
	}

	if got := states["TS.covered.001"]; got != project.SpecCoverageVerified {
		t.Fatalf("covered state = %q, want verified", got)
	}
	if got := states["TS.uncovered.001"]; got != project.SpecCoverageUncovered {
		t.Fatalf("uncovered state = %q, want uncovered", got)
	}
	if strings.Contains(output.String(), "percent") {
		t.Fatalf("JSON output contains percentage scalar: %s", output.String())
	}
}

func TestRunSpecCoverageHumanSummaryIncludesWhyAndNextAction(t *testing.T) {
	fixture := newCheckTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, false)
	defer restoreJSON()

	err := runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "haft spec coverage: 2 active section(s)") {
		t.Fatalf("output = %q, want section count summary", result)
	}
	if !strings.Contains(result, "why:") {
		t.Fatalf("output = %q, want per-section why", result)
	}
	if !strings.Contains(result, "next_action:") {
		t.Fatalf("output = %q, want per-section next_action", result)
	}
	if strings.Contains(result, "%") {
		t.Fatalf("output contains percentage scalar: %s", result)
	}
}

func TestRunSpecCoverageBlocksWhenSpecCheckHasFindings(t *testing.T) {
	root := newSpecCheckCLIProject(t, invalidCLITermMapCarrier())
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecCoverage(cmd, nil)
	if err == nil {
		t.Fatal("runSpecCoverage returned nil, want spec-check block")
	}
	if !strings.Contains(err.Error(), "spec coverage blocked") {
		t.Fatalf("error = %q, want spec coverage block", err.Error())
	}
	if !strings.Contains(err.Error(), "haft spec check") {
		t.Fatalf("error = %q, want spec check next action", err.Error())
	}
}

func newSpecCheckCLIProject(t *testing.T, termMap string) string {
	t.Helper()

	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), validCLISpecSectionCarrier("TS.use.001", "environment-change"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), validCLISpecSectionCarrier("ES.creator.001", "creator-role"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), termMap)

	return root
}

func validCLISpecSectionCarrier(id string, kind string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"```\n"
}

func validCLITermMapCarrier() string {
	return "```yaml term-map\n" +
		"entries:\n" +
		"  - term: HarnessableProject\n" +
		"    domain: enabling\n" +
		"    definition: A project with active specs.\n" +
		"```\n"
}

func invalidCLITermMapCarrier() string {
	return "```yaml term-map\n" +
		"entries:\n" +
		"  - domain: enabling\n" +
		"    definition: A project with active specs.\n" +
		"```\n"
}

func writeSpecCoverageCLICarriers(t *testing.T, root string) {
	t.Helper()

	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), strings.Join([]string{
		coverageCLISpecSection("TS.covered.001", "Covered section"),
		coverageCLISpecSection("TS.uncovered.001", "Uncovered section"),
	}, "\n"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), coverageCLIDraftSpecSection("ES.coverage.001", "Coverage draft"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())
}

func coverageCLISpecSection(id string, title string) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: environment-change\n" +
		"title: " + title + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"```\n"
}

func coverageCLIDraftSpecSection(id string, title string) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: creator-role\n" +
		"title: " + title + "\n" +
		"statement_type: explanation\n" +
		"claim_layer: carrier\n" +
		"owner: human\n" +
		"status: draft\n" +
		"```\n"
}

func writeSpecCheckCLIFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stubSpecCheckJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specCheckJSON
	specCheckJSON = value

	return func() {
		specCheckJSON = previous
	}
}

func stubSpecCheckExit(t *testing.T) *int {
	t.Helper()

	exitCode := new(int)
	previous := specCheckExit
	specCheckExit = func(code int) {
		*exitCode = code
	}
	t.Cleanup(func() {
		specCheckExit = previous
	})

	return exitCode
}

func stubSpecCoverageJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specCoverageJSON
	specCoverageJSON = value

	return func() {
		specCoverageJSON = previous
	}
}
