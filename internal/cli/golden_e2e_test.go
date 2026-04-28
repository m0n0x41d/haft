package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/spf13/cobra"
)

const (
	goldenE2ETargetSection  = "TS.environment-change.001"
	goldenE2EEnableSection  = "ES.creator-role.001"
	goldenE2EAffectedFile   = "internal/app/flow.go"
	goldenE2EEvidence       = "test -f internal/app/flow.go"
	goldenE2EPlanID         = "plan-golden-e2e"
	goldenE2EPlanRevision   = "p1"
	goldenE2EStatusFileName = "status.json"
	goldenE2ELogFileName    = "runtime.jsonl"
)

func TestGoldenE2EInitOnboardCommissionRuntimeEvidenceCoverage(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	sourceRoot := goldenE2ESourceRoot(t)
	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()

	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	if err := runInit(&cobra.Command{}, nil); err != nil {
		t.Fatalf("run init: %v", err)
	}

	initialReadiness := goldenE2EReadiness(t, root)
	if initialReadiness.Status != project.ReadinessNeedsOnboard {
		t.Fatalf("initial readiness = %+v, want needs_onboard", initialReadiness)
	}

	err := runHarnessRun(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("harness run before onboarding succeeded, want needs_onboard block")
	}
	if !strings.Contains(err.Error(), "needs_onboard") {
		t.Fatalf("harness block = %q, want needs_onboard", err.Error())
	}

	writeGoldenE2ESpecs(t, root)
	baselineGoldenE2ESpecSections(t, root)
	runGoldenE2ESpecCheck(t, sourceRoot, root)
	uncoveredCoverage := runGoldenE2ESpecCoverage(t, sourceRoot, root)
	assertGoldenE2ESectionStates(t, uncoveredCoverage, project.SpecCoverageUncovered)

	specPlan := runGoldenE2ESpecPlan(t, sourceRoot, root)
	assertGoldenE2EPlanCoversSections(t, specPlan, goldenE2ESectionIDs())

	readyFacts := goldenE2EReadiness(t, root)
	if readyFacts.Status != project.ReadinessReady {
		t.Fatalf("ready facts = %+v, want ready", readyFacts)
	}

	decisionID := createGoldenE2EDecision(t, root)
	reasonedCoverage := goldenE2ESpecCoverageFromCore(t, root)
	assertGoldenE2ESectionStates(t, reasonedCoverage, project.SpecCoverageReasoned)

	runtimeRoot := newGoldenE2ERuntime(t, root)
	configureGoldenE2EHarness(root, runtimeRoot)

	var runOutput bytes.Buffer
	runCmd := &cobra.Command{}
	runCmd.SetOut(&runOutput)
	if err := runHarnessRun(runCmd, []string{decisionID}); err != nil {
		t.Fatalf("harness prepare-only run: %v", err)
	}
	for _, fragment := range []string{
		filepath.Join(".haft", "plans", goldenE2EPlanID+".yaml"),
		"Commissions: created",
	} {
		if !strings.Contains(runOutput.String(), fragment) {
			t.Fatalf("harness output missing %q:\n%s", fragment, runOutput.String())
		}
	}

	commissions := goldenE2ECommissions(t, root)
	if len(commissions) != 1 {
		t.Fatalf("commissions = %#v, want one prepared WorkCommission", commissions)
	}

	commission := commissions[0]
	commissionID := stringField(commission, "id")
	if commissionID == "" {
		t.Fatalf("commission id missing: %#v", commission)
	}
	if stringField(commission, "decision_ref") != decisionID {
		t.Fatalf("decision_ref = %q, want %s", stringField(commission, "decision_ref"), decisionID)
	}
	if stringField(commission, "state") != "queued" {
		t.Fatalf("state = %q, want queued", stringField(commission, "state"))
	}
	if stringField(commission, "implementation_plan_ref") != goldenE2EPlanID {
		t.Fatalf("plan ref = %q, want %s", stringField(commission, "implementation_plan_ref"), goldenE2EPlanID)
	}
	if stringField(commission, "projection_policy") != "local_only" {
		t.Fatalf("projection policy = %q, want local_only", stringField(commission, "projection_policy"))
	}

	scope := mapField(commission, "scope")
	if !containsAnyString(scope["allowed_paths"], goldenE2EAffectedFile) {
		t.Fatalf("allowed_paths = %#v, want %s", scope["allowed_paths"], goldenE2EAffectedFile)
	}
	if !containsAnyString(scope["lockset"], goldenE2EAffectedFile) {
		t.Fatalf("lockset = %#v, want %s", scope["lockset"], goldenE2EAffectedFile)
	}

	commissionedCoverage := goldenE2ESpecCoverageFromCore(t, root)
	assertGoldenE2ESectionStates(t, commissionedCoverage, project.SpecCoverageCommissioned)

	resultOutput := goldenE2EHarnessResult(t, commissionID)
	for _, fragment := range []string{
		"commission: " + commissionID,
		"state: queued",
		"evidence_summary:",
		"requirement: kind=command command=" + goldenE2EEvidence,
	} {
		if !strings.Contains(resultOutput, fragment) {
			t.Fatalf("harness result missing %q:\n%s", fragment, resultOutput)
		}
	}

	runtimeRunID := runGoldenE2EMockRuntime(t, root, commissionID)
	implementedCoverage := goldenE2ESpecCoverageFromCore(t, root)
	assertGoldenE2ESectionStates(t, implementedCoverage, project.SpecCoverageImplemented)

	evidenceID := attachGoldenE2ERuntimeEvidence(t, root, commissionID, runtimeRunID)
	verifiedCoverage := runGoldenE2ESpecCoverage(t, sourceRoot, root)
	assertGoldenE2ESectionStates(t, verifiedCoverage, project.SpecCoverageVerified)
	assertGoldenE2ERuntimeEvidenceEdges(t, verifiedCoverage, runtimeRunID, evidenceID)

	completedResult := goldenE2EHarnessResult(t, commissionID)
	for _, fragment := range []string{
		"commission: " + commissionID,
		"state: completed",
		"last_event: workflow_terminal",
		"evidence_summary:",
	} {
		if !strings.Contains(completedResult, fragment) {
			t.Fatalf("completed harness result missing %q:\n%s", fragment, completedResult)
		}
	}

	status := goldenE2EHarnessStatus(t)
	metadata := mapField(status, "metadata")
	if stringField(metadata, "tracker_kind") != "commission_source:haft" {
		t.Fatalf("tracker_kind = %#v, want local Haft commission source", metadata["tracker_kind"])
	}
	if unavailable, ok := status["status_unavailable"].(bool); !ok || !unavailable {
		t.Fatalf("status_unavailable = %#v, want true for prepare-only missing runtime status", status["status_unavailable"])
	}
}

func newGoldenE2ERepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeGoldenE2EFile(t, filepath.Join(root, "README.md"), "# Golden E2E fixture\n")
	runGoldenE2EGit(t, root, "init")
	runGoldenE2EGit(t, root, "config", "user.email", "test@example.com")
	runGoldenE2EGit(t, root, "config", "user.name", "Test User")
	runGoldenE2EGit(t, root, "add", "README.md")
	runGoldenE2EGit(t, root, "commit", "-m", "initial")

	return root
}

func writeGoldenE2ESpecs(t *testing.T, root string) {
	t.Helper()

	specDir := filepath.Join(root, ".haft", "specs")
	writeGoldenE2EFile(
		t,
		filepath.Join(specDir, "target-system.md"),
		goldenE2ESpecSection(goldenE2ETargetSection, "environment-change", "Golden target loop"),
	)
	writeGoldenE2EFile(
		t,
		filepath.Join(specDir, "enabling-system.md"),
		goldenE2ESpecSection(goldenE2EEnableSection, "creator-role", "Golden enabling loop"),
	)
	writeGoldenE2EFile(
		t,
		filepath.Join(specDir, "term-map.md"),
		strings.Join([]string{
			"# Term Map",
			"",
			"```yaml term-map",
			"entries:",
			"  - term: HarnessableProject",
			"    domain: enabling",
			"    definition: A project with active target and enabling specs, a term map, and a local workflow policy.",
			"    not:",
			"      - tracker ticket",
			"      - README-only project",
			"    aliases:",
			"      - harness-ready project",
			"    owners:",
			"      - haft",
			"```",
			"",
		}, "\n"),
	)
}

func goldenE2ESpecSection(id string, kind string, title string) string {
	return strings.Join([]string{
		"# " + title,
		"",
		"## " + id + " " + title,
		"",
		"```yaml spec-section",
		"id: " + id,
		"kind: " + kind,
		"title: " + title,
		"statement_type: definition",
		"claim_layer: object",
		"owner: human",
		"status: active",
		"valid_until: 2099-01-01",
		"depends_on: []",
		"supersedes: []",
		"terms:",
		"  - HarnessableProject",
		"target_refs: []",
		"evidence_required:",
		"  - kind: review",
		"    description: Golden E2E fixture verifies this section can govern a local WorkCommission.",
		"```",
		"",
		"The golden fixture states the minimal active section needed for deterministic CLI readiness.",
		"",
	}, "\n")
}

func runGoldenE2ESpecCheck(t *testing.T, sourceRoot string, root string) project.SpecCheckReport {
	t.Helper()

	output := runGoldenE2EHaftCLI(t, sourceRoot, root, "spec", "check", "--json")
	report := project.SpecCheckReport{}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode spec check JSON: %v\n%s", err, string(output))
	}
	if report.Summary.TotalFindings != 0 {
		t.Fatalf("spec check findings = %#v, want clean", report.Findings)
	}
	if report.Summary.ActiveSpecSections != len(goldenE2ESectionIDs()) {
		t.Fatalf("active sections = %d, want %d", report.Summary.ActiveSpecSections, len(goldenE2ESectionIDs()))
	}

	return report
}

func createGoldenE2EDecision(t *testing.T, root string) string {
	t.Helper()

	writeGoldenE2EFile(t, filepath.Join(root, goldenE2EAffectedFile), "package app\n\nfunc Flow() string {\n\treturn \"ready\"\n}\n")

	database, store := openGoldenE2EStore(t, root)
	defer database.Close()

	ctx := context.Background()
	haftDir := filepath.Join(root, ".haft")
	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Golden E2E harness proof",
		Signal:     "The product loop needs one deterministic init-to-commission proof.",
		Acceptance: "A local-only WorkCommission can be prepared from a narrow DecisionRecord without external trackers.",
	})
	if err != nil {
		t.Fatalf("frame problem: %v", err)
	}
	if err := store.AddLink(ctx, problem.Meta.ID, goldenE2ETargetSection, "describes"); err != nil {
		t.Fatalf("link problem to target section: %v", err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Prepare a local-only golden WorkCommission",
		WhySelected:     "The golden path needs a bounded execution authorization that can be inspected offline.",
		SelectionPolicy: "Prefer the smallest deterministic path through init, spec check, DecisionRecord, and WorkCommission preparation.",
		CounterArgument: "A unit-only proof is faster to maintain.",
		WeakestLink:     "The fixture can become unrepresentative if it stops using real project storage and CLI seams.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Unit-only proof",
			Reason:  "It would not prove project readiness, spec check, and harness preparation compose.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Golden E2E fixture becomes flaky or requires network."},
		},
		Invariants: []string{
			"WorkCommission scope remains limited to internal/app/flow.go.",
			"Projection policy remains local_only.",
		},
		EvidenceReqs: []string{goldenE2EEvidence},
		AffectedFiles: []string{
			goldenE2EAffectedFile,
		},
		SectionRefs: []string{
			goldenE2ETargetSection,
			goldenE2EEnableSection,
		},
		ValidUntil:     "2099-01-01T00:00:00Z",
		GovernanceMode: "exact",
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}

	for _, sectionID := range []string{goldenE2ETargetSection, goldenE2EEnableSection} {
		if err := store.AddLink(ctx, decision.Meta.ID, sectionID, "governs"); err != nil {
			t.Fatalf("link decision to spec section %s: %v", sectionID, err)
		}
	}

	return decision.Meta.ID
}

func newGoldenE2ERuntime(t *testing.T, root string) string {
	t.Helper()

	runtimeRoot := filepath.Join(root, ".fake-open-sleigh")
	executable := filepath.Join(runtimeRoot, "bin", "open_sleigh")
	writeGoldenE2EFile(t, executable, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(executable, 0o755); err != nil {
		t.Fatalf("chmod fake runtime: %v", err)
	}

	return runtimeRoot
}

func configureGoldenE2EHarness(root string, runtimeRoot string) {
	harnessPlanID = goldenE2EPlanID
	harnessPlanRevision = goldenE2EPlanRevision
	harnessRunPrepareOnly = true
	harnessRunMock = true
	harnessRunRuntimePath = runtimeRoot
	harnessRunStatusPath = filepath.Join(root, ".haft", "runtime", goldenE2EStatusFileName)
	harnessRunLogPath = filepath.Join(root, ".haft", "runtime", goldenE2ELogFileName)
	harnessRunWorkspaceRoot = filepath.Join(root, ".haft", "workspaces")
	harnessStatusPath = harnessRunStatusPath
	harnessStatusLogPath = harnessRunLogPath
}

func goldenE2EHarnessResult(t *testing.T, commissionID string) string {
	t.Helper()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runHarnessResult(cmd, []string{commissionID}); err != nil {
		t.Fatalf("harness result: %v", err)
	}

	return output.String()
}

func goldenE2EHarnessStatus(t *testing.T) map[string]any {
	t.Helper()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	previous := harnessStatusJSON
	harnessStatusJSON = true
	defer func() {
		harnessStatusJSON = previous
	}()

	if err := runHarnessStatus(cmd, nil); err != nil {
		t.Fatalf("harness status: %v", err)
	}

	status := map[string]any{}
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatalf("decode harness status: %v\n%s", err, output.String())
	}

	return status
}

func goldenE2ESourceRoot(t *testing.T) string {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve source root: %v", err)
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		t.Fatal("source root is empty")
	}

	return root
}

func runGoldenE2EHaftCLI(t *testing.T, sourceRoot string, root string, args ...string) []byte {
	t.Helper()

	commandArgs := append([]string{"run", "./cmd/haft"}, args...)
	cmd := exec.Command("go", commandArgs...)
	cmd.Dir = sourceRoot
	cmd.Env = append(
		os.Environ(),
		"HAFT_PROJECT_ROOT="+root,
		"GOCACHE="+filepath.Join(os.TempDir(), "haft-golden-go-build"),
		"GOMODCACHE="+filepath.Join(os.TempDir(), "haft-golden-go-mod"),
		"GOFLAGS=-modcacherw",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf(
			"go run ./cmd/haft %s failed: %v\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "),
			err,
			stdout.String(),
			stderr.String(),
		)
	}

	return append([]byte(nil), stdout.Bytes()...)
}

func runGoldenE2ESpecCoverage(t *testing.T, sourceRoot string, root string) project.SpecCoverageReport {
	t.Helper()

	output := runGoldenE2EHaftCLI(t, sourceRoot, root, "spec", "coverage", "--json")
	report := project.SpecCoverageReport{}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode spec coverage JSON: %v\n%s", err, string(output))
	}

	return report
}

func runGoldenE2ESpecPlan(t *testing.T, sourceRoot string, root string) project.SpecPlanReport {
	t.Helper()

	output := runGoldenE2EHaftCLI(t, sourceRoot, root, "spec", "plan", "--json")
	report := project.SpecPlanReport{}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode spec plan JSON: %v\n%s", err, string(output))
	}

	return report
}

func goldenE2ESpecCoverageFromCore(t *testing.T, root string) project.SpecCoverageReport {
	t.Helper()

	report, err := buildSpecCoverageReport(context.Background(), root)
	if err != nil {
		t.Fatalf("build spec coverage: %v", err)
	}

	return report
}

func assertGoldenE2ESectionStates(
	t *testing.T,
	report project.SpecCoverageReport,
	state project.SpecCoverageState,
) {
	t.Helper()

	for _, sectionID := range goldenE2ESectionIDs() {
		section := goldenE2ECoverageSection(report, sectionID)
		if section.SectionID == "" {
			t.Fatalf("coverage missing section %s in %#v", sectionID, report.Sections)
		}
		if section.State != state {
			t.Fatalf("section %s state = %s, want %s; section = %#v", sectionID, section.State, state, section)
		}
	}
}

func assertGoldenE2EPlanCoversSections(
	t *testing.T,
	report project.SpecPlanReport,
	sectionIDs []string,
) {
	t.Helper()

	if report.Summary.TotalCandidates != len(sectionIDs) {
		t.Fatalf("spec plan candidates = %d, want %d", report.Summary.TotalCandidates, len(sectionIDs))
	}

	planned := map[string]bool{}
	for _, proposal := range report.Proposals {
		for _, sectionID := range proposal.SectionRefs {
			planned[sectionID] = true
		}
	}
	for _, sectionID := range sectionIDs {
		if planned[sectionID] {
			continue
		}
		t.Fatalf("spec plan proposals = %#v, want section %s", report.Proposals, sectionID)
	}
}

func runGoldenE2EMockRuntime(t *testing.T, root string, commissionID string) string {
	t.Helper()

	database, store := openGoldenE2EStore(t, root)
	defer database.Close()

	runtimeRunID := commissionID + "#runtime-run-golden"
	ctx := context.Background()
	events := []map[string]any{
		{
			"action":        "claim_for_preflight",
			"commission_id": commissionID,
			"runner_id":     "golden-e2e-runtime",
		},
		{
			"action":        "record_preflight",
			"commission_id": commissionID,
			"runner_id":     "golden-e2e-runtime",
			"event":         "preflight_passed",
			"verdict":       "pass",
			"payload": map[string]any{
				"phase":          "preflight",
				"runtime_run_id": runtimeRunID,
				"section_refs":   stringsToAnySlice(goldenE2ESectionIDs()),
			},
		},
		{
			"action":        "start_after_preflight",
			"commission_id": commissionID,
			"runner_id":     "golden-e2e-runtime",
			"event":         "runtime_started",
			"verdict":       "pass",
			"project_root":  root,
			"payload": map[string]any{
				"phase":          "execute",
				"runtime_run_id": runtimeRunID,
				"section_refs":   stringsToAnySlice(goldenE2ESectionIDs()),
			},
		},
		{
			"action":        "record_run_event",
			"commission_id": commissionID,
			"runner_id":     "golden-e2e-runtime",
			"event":         "phase_outcome",
			"verdict":       "pass",
			"payload": map[string]any{
				"phase":          "execute",
				"runtime_run_id": runtimeRunID,
				"section_refs":   stringsToAnySlice(goldenE2ESectionIDs()),
			},
		},
		{
			"action":        "complete_or_block",
			"commission_id": commissionID,
			"runner_id":     "golden-e2e-runtime",
			"event":         "workflow_terminal",
			"verdict":       "pass",
			"payload": map[string]any{
				"phase":          "measure",
				"runtime_run_id": runtimeRunID,
				"section_refs":   stringsToAnySlice(goldenE2ESectionIDs()),
			},
		},
	}

	for _, event := range events {
		if _, err := handleHaftCommission(ctx, store, event); err != nil {
			t.Fatalf("mock runtime %s: %v", stringArg(event, "action"), err)
		}
	}

	return runtimeRunID
}

func attachGoldenE2ERuntimeEvidence(
	t *testing.T,
	root string,
	commissionID string,
	runtimeRunID string,
) string {
	t.Helper()

	database, store := openGoldenE2EStore(t, root)
	defer database.Close()

	evidence, err := artifact.AttachEvidence(context.Background(), store, artifact.EvidenceInput{
		ArtifactRef:     commissionID,
		Content:         "Golden E2E mock runtime completed the required local evidence command: " + goldenE2EEvidence,
		Type:            "test",
		Verdict:         "supports",
		CarrierRef:      runtimeRunID,
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ClaimScope: []string{
			goldenE2ETargetSection,
			goldenE2EEnableSection,
			goldenE2EAffectedFile,
			"internal/cli/golden_e2e_test.go",
		},
		ValidUntil: "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("attach runtime evidence: %v", err)
	}

	return evidence.ID
}

func assertGoldenE2ERuntimeEvidenceEdges(
	t *testing.T,
	report project.SpecCoverageReport,
	runtimeRunID string,
	evidenceID string,
) {
	t.Helper()

	for _, sectionID := range goldenE2ESectionIDs() {
		section := goldenE2ECoverageSection(report, sectionID)
		if !goldenE2EEdgeExists(section.Edges, project.SpecCoverageEdgeRuntimeRun, runtimeRunID) {
			t.Fatalf("section %s edges = %#v, want RuntimeRun %s", sectionID, section.Edges, runtimeRunID)
		}
		if !goldenE2EEdgeExists(section.Edges, project.SpecCoverageEdgeRuntimeEvidence, evidenceID) {
			t.Fatalf("section %s edges = %#v, want RuntimeRun evidence %s", sectionID, section.Edges, evidenceID)
		}
	}
}

func goldenE2ECoverageSection(
	report project.SpecCoverageReport,
	sectionID string,
) project.SpecCoverageSection {
	for _, section := range report.Sections {
		if section.SectionID == sectionID {
			return section
		}
	}

	return project.SpecCoverageSection{}
}

func goldenE2EEdgeExists(
	edges []project.SpecCoverageEdge,
	edgeType project.SpecCoverageEdgeType,
	target string,
) bool {
	for _, edge := range edges {
		if edge.Type != edgeType {
			continue
		}
		if edge.Target == target {
			return true
		}
	}

	return false
}

func goldenE2ESectionIDs() []string {
	return []string{
		goldenE2ETargetSection,
		goldenE2EEnableSection,
	}
}

func goldenE2ECommissions(t *testing.T, root string) []map[string]any {
	t.Helper()

	database, store := openGoldenE2EStore(t, root)
	defer database.Close()

	commissions, err := loadWorkCommissionPayloads(context.Background(), store)
	if err != nil {
		t.Fatalf("load commissions: %v", err)
	}

	return commissions
}

func goldenE2EReadiness(t *testing.T, root string) project.ReadinessFacts {
	t.Helper()

	facts, err := project.InspectReadiness(root)
	if err != nil {
		t.Fatalf("inspect readiness: %v", err)
	}

	return facts
}

// baselineGoldenE2ESpecSections records SpecSection baselines for every
// active section in the golden fixture, mirroring what the slice 3c
// `haft_spec_section(action="approve", ...)` will do once it ships.
// Without baselines, slice 3b drift detection rightfully reports
// `spec_section_needs_baseline` for active sections; the golden test
// asserts a clean spec check, so it must approve its own fixture.
func baselineGoldenE2ESpecSections(t *testing.T, root string) {
	t.Helper()

	specSet, err := project.LoadProjectSpecificationSet(root)
	if err != nil {
		t.Fatalf("load spec set: %v", err)
	}

	cfg, err := project.Load(filepath.Join(root, ".haft"))
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if cfg == nil {
		t.Fatal("project config missing")
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	store := specflow.NewSQLiteBaselineStore(database.GetRawDB())
	for _, section := range specSet.Sections {
		if !strings.EqualFold(strings.TrimSpace(section.Status), string(project.SpecSectionStateActive)) {
			continue
		}
		baseline := specflow.SectionBaseline{
			ProjectID:  cfg.ID,
			SectionID:  section.ID,
			Hash:       specflow.HashSection(section),
			ApprovedBy: "golden-fixture",
		}
		if err := store.Put(baseline); err != nil {
			t.Fatalf("put baseline %s: %v", section.ID, err)
		}
	}
}

func openGoldenE2EStore(t *testing.T, root string) (*db.Store, *artifact.Store) {
	t.Helper()

	cfg, err := project.Load(filepath.Join(root, ".haft"))
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if cfg == nil {
		t.Fatal("project config missing after init")
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	return database, artifact.NewStore(database.GetRawDB())
}

func writeGoldenE2EFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGoldenE2EGit(t *testing.T, root string, args ...string) {
	t.Helper()

	gitArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", gitArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func overrideGoldenE2EFlags(t *testing.T) func() {
	t.Helper()

	restoreHarness := overrideHarnessTestFlags()

	oldInitClaude := initClaude
	oldInitCursor := initCursor
	oldInitGemini := initGemini
	oldInitCodex := initCodex
	oldInitAir := initAir
	oldInitAll := initAll
	oldInitLocal := initLocal

	oldHarnessRunPrepareOnly := harnessRunPrepareOnly
	oldHarnessRunForceCreate := harnessRunForceCreate
	oldHarnessRunOnce := harnessRunOnce
	oldHarnessRunOnceTimeoutMS := harnessRunOnceTimeoutMS
	oldHarnessRunMock := harnessRunMock
	oldHarnessRunMockAgent := harnessRunMockAgent
	oldHarnessRunMockJudge := harnessRunMockJudge
	oldHarnessRunStatusPath := harnessRunStatusPath
	oldHarnessRunLogPath := harnessRunLogPath
	oldHarnessRunWorkspaceRoot := harnessRunWorkspaceRoot
	oldHarnessRunRuntimePath := harnessRunRuntimePath
	oldHarnessRunGeneratedPlanPath := harnessRunGeneratedPlanPath
	oldHarnessStatusPath := harnessStatusPath
	oldHarnessStatusLogPath := harnessStatusLogPath
	oldHarnessStatusJSON := harnessStatusJSON

	initClaude = false
	initCursor = false
	initGemini = false
	initCodex = false
	initAir = false
	initAll = false
	initLocal = false

	harnessRunPrepareOnly = false
	harnessRunForceCreate = false
	harnessRunOnce = false
	harnessRunOnceTimeoutMS = 8000
	harnessRunMock = false
	harnessRunMockAgent = false
	harnessRunMockJudge = false
	harnessRunStatusPath = ""
	harnessRunLogPath = ""
	harnessRunWorkspaceRoot = ""
	harnessRunRuntimePath = ""
	harnessRunGeneratedPlanPath = ""
	harnessStatusPath = ""
	harnessStatusLogPath = ""
	harnessStatusJSON = false

	return func() {
		restoreHarness()

		initClaude = oldInitClaude
		initCursor = oldInitCursor
		initGemini = oldInitGemini
		initCodex = oldInitCodex
		initAir = oldInitAir
		initAll = oldInitAll
		initLocal = oldInitLocal

		harnessRunPrepareOnly = oldHarnessRunPrepareOnly
		harnessRunForceCreate = oldHarnessRunForceCreate
		harnessRunOnce = oldHarnessRunOnce
		harnessRunOnceTimeoutMS = oldHarnessRunOnceTimeoutMS
		harnessRunMock = oldHarnessRunMock
		harnessRunMockAgent = oldHarnessRunMockAgent
		harnessRunMockJudge = oldHarnessRunMockJudge
		harnessRunStatusPath = oldHarnessRunStatusPath
		harnessRunLogPath = oldHarnessRunLogPath
		harnessRunWorkspaceRoot = oldHarnessRunWorkspaceRoot
		harnessRunRuntimePath = oldHarnessRunRuntimePath
		harnessRunGeneratedPlanPath = oldHarnessRunGeneratedPlanPath
		harnessStatusPath = oldHarnessStatusPath
		harnessStatusLogPath = oldHarnessStatusLogPath
		harnessStatusJSON = oldHarnessStatusJSON
	}
}
