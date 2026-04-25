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

func TestGoldenE2EInitOnboardCommissionPrepareOnly(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

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
	runGoldenE2ESpecCheck(t)

	readyFacts := goldenE2EReadiness(t, root)
	if readyFacts.Status != project.ReadinessReady {
		t.Fatalf("ready facts = %+v, want ready", readyFacts)
	}

	decisionID := createGoldenE2EDecision(t, root)
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

func runGoldenE2ESpecCheck(t *testing.T) {
	t.Helper()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCheckJSON(t, false)
	defer restoreJSON()

	exitCode := stubSpecCheckExit(t)
	if err := runSpecCheck(cmd, nil); err != nil {
		t.Fatalf("run spec check: %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("spec check exit = %d, want 0\n%s", *exitCode, output.String())
	}
	if !strings.Contains(output.String(), "haft spec check: clean") {
		t.Fatalf("spec check output = %q, want clean", output.String())
	}
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
